package timeline

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/analyzer"
	"github.com/moolen/spectre/internal/models"
)

func (s *Service) BuildTimelineResponse(queryResult, eventResult *models.QueryResult) *models.SearchResponse {
	if queryResult == nil || len(queryResult.Events) == 0 {
		var executionTimeMs int64
		if queryResult != nil {
			executionTimeMs = int64(queryResult.ExecutionTimeMs)
		}

		return &models.SearchResponse{
			Resources:       []models.Resource{},
			Count:           0,
			ExecutionTimeMs: executionTimeMs,
		}
	}

	eventsByResource := make(map[string][]models.Event)
	queryStartTime := queryResult.QueryStartTime
	queryEndTime := queryResult.QueryEndTime

	for _, event := range queryResult.Events {
		uid := event.Resource.UID
		if uid == "" {
			continue
		}
		eventsByResource[uid] = append(eventsByResource[uid], event)
	}

	if queryStartTime == 0 || queryEndTime == 0 {
		queryStartTime = queryResult.Events[0].Timestamp
		queryEndTime = queryResult.Events[0].Timestamp
		for _, event := range queryResult.Events {
			if event.Timestamp < queryStartTime {
				queryStartTime = event.Timestamp
			}
			if event.Timestamp > queryEndTime {
				queryEndTime = event.Timestamp
			}
		}
	}

	resourceMap := make(map[string]*models.Resource)

	for uid, events := range eventsByResource {
		if len(events) == 0 {
			continue
		}

		sort.Slice(events, func(i, j int) bool {
			return events[i].Timestamp < events[j].Timestamp
		})

		firstEvent := events[0]
		resourceID := fmt.Sprintf("%s/%s/%s/%s", firstEvent.Resource.Group, firstEvent.Resource.Version, firstEvent.Resource.Kind, uid)

		resource := &models.Resource{
			ID:          resourceID,
			Group:       firstEvent.Resource.Group,
			Version:     firstEvent.Resource.Version,
			Kind:        firstEvent.Resource.Kind,
			Namespace:   firstEvent.Resource.Namespace,
			Name:        firstEvent.Resource.Name,
			Events:      []models.K8sEvent{},
			PreExisting: firstEvent.PreExisting,
		}

		var segments []models.StatusSegment
		for i, event := range events {
			status := analyzer.InferStatusFromResource(event.Resource.Kind, event.Data, string(event.Type))

			var endTime int64
			if i < len(events)-1 {
				endTime = events[i+1].Timestamp
			} else {
				endTime = queryEndTime
			}

			startTime := event.Timestamp
			if event.PreExisting && event.Type != models.EventTypeDelete && queryStartTime > 0 && event.Timestamp < queryStartTime {
				startTime = queryStartTime
			}

			segment := models.StatusSegment{
				StartTime:    startTime,
				EndTime:      endTime,
				Status:       status,
				ResourceData: event.Data,
			}

			if len(event.Data) > 0 {
				errorMessages := analyzer.InferErrorMessages(event.Resource.Kind, event.Data, status)
				if len(errorMessages) > 0 {
					segment.Message = strings.Join(errorMessages, "; ")
				}
			} else if strings.EqualFold(event.Resource.Kind, "Pod") {
				s.logger.Warn("Pod event missing ResourceData in timeline service: %s/%s (event ID: %s, has %d events total)",
					event.Resource.Namespace, event.Resource.Name, event.ID, len(events))
			}

			segments = append(segments, segment)
		}

		resource.StatusSegments = segments
		resourceMap[resourceID] = resource
	}

	getString := func(m map[string]interface{}, key, defaultValue string) string {
		if m == nil {
			return defaultValue
		}
		if val, ok := m[key].(string); ok {
			return val
		}
		return defaultValue
	}

	if len(queryResult.K8sEventsByResource) > 0 {
		s.logger.Debug("Using K8sEventsByResource from graph executor: %d resources have events", len(queryResult.K8sEventsByResource))
		for _, resource := range resourceMap {
			parts := strings.Split(resource.ID, "/")
			if len(parts) >= 4 {
				resourceUID := parts[3]
				if events, ok := queryResult.K8sEventsByResource[resourceUID]; ok {
					resource.Events = append(resource.Events, events...)
				}
			}
		}
	} else if eventResult != nil {
		for _, event := range eventResult.Events {
			if event.Resource.Kind != "Event" || event.Resource.InvolvedObjectUID == "" {
				continue
			}

			var targetResource *models.Resource
			for _, resource := range resourceMap {
				parts := strings.Split(resource.ID, "/")
				if len(parts) >= 4 {
					resourceUID := parts[3]
					if resourceUID == event.Resource.InvolvedObjectUID {
						targetResource = resource
						break
					}
				}
			}

			if targetResource == nil {
				continue
			}

			var eventData map[string]interface{}
			if len(event.Data) > 0 {
				if err := json.Unmarshal(event.Data, &eventData); err != nil {
					s.logger.Warn("Failed to parse event data: %v", err)
					continue
				}
			}

			k8sEvent := models.K8sEvent{
				ID:        event.ID,
				Timestamp: event.Timestamp,
				Reason:    getString(eventData, "reason", ""),
				Message:   getString(eventData, "message", ""),
				Type:      getString(eventData, "type", "Normal"),
				Count:     1,
			}

			if count, ok := eventData["count"].(float64); ok {
				k8sEvent.Count = int32(count)
			}
			if source, ok := eventData["source"].(map[string]interface{}); ok {
				if component, ok := source["component"].(string); ok {
					k8sEvent.Source = component
				}
			}
			if firstTimestamp, ok := eventData["firstTimestamp"].(string); ok {
				if t, err := time.Parse(time.RFC3339, firstTimestamp); err == nil {
					k8sEvent.FirstTimestamp = t.UnixNano()
				}
			}
			if lastTimestamp, ok := eventData["lastTimestamp"].(string); ok {
				if t, err := time.Parse(time.RFC3339, lastTimestamp); err == nil {
					k8sEvent.LastTimestamp = t.UnixNano()
				}
			}

			targetResource.Events = append(targetResource.Events, k8sEvent)
		}
	}

	resources := make([]models.Resource, 0, len(resourceMap))
	for _, resource := range resourceMap {
		resources = append(resources, *resource)
	}

	return &models.SearchResponse{
		Resources:       resources,
		Count:           len(resources),
		ExecutionTimeMs: int64(queryResult.ExecutionTimeMs),
	}
}
