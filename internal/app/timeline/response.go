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

type TimelineResourceEntry struct {
	ID           string
	Group        string
	Version      string
	Kind         string
	Namespace    string
	Name         string
	UID          string
	PreExisting  bool
	Events       []models.Event
	K8sEvents    []models.K8sEvent
	RawK8sEvents []models.Event
}

type TimelineIndex struct {
	queryStartTime  int64
	queryEndTime    int64
	executionTimeMs int64
	entries         []*TimelineResourceEntry
}

func (i *TimelineIndex) Count() int {
	if i == nil {
		return 0
	}
	return len(i.entries)
}

func (i *TimelineIndex) Entries() []*TimelineResourceEntry {
	if i == nil {
		return nil
	}
	return i.entries
}

func (i *TimelineIndex) ExecutionTimeMs() int64 {
	if i == nil {
		return 0
	}
	return i.executionTimeMs
}

func (s *Service) BuildTimelineIndex(queryResult, eventResult *models.QueryResult) *TimelineIndex {
	index := &TimelineIndex{}
	if queryResult != nil {
		index.executionTimeMs = int64(queryResult.ExecutionTimeMs)
	}
	if queryResult == nil || len(queryResult.Events) == 0 {
		return index
	}

	index.queryStartTime, index.queryEndTime = deriveTimelineQueryBounds(queryResult)

	entryByUID := make(map[string]*TimelineResourceEntry)
	entries := make([]*TimelineResourceEntry, 0)
	for _, event := range queryResult.Events {
		uid := event.Resource.UID
		if uid == "" {
			continue
		}

		entry, exists := entryByUID[uid]
		if !exists {
			entry = &TimelineResourceEntry{
				ID:          buildTimelineResourceID(event),
				Group:       event.Resource.Group,
				Version:     event.Resource.Version,
				Kind:        event.Resource.Kind,
				Namespace:   event.Resource.Namespace,
				Name:        event.Resource.Name,
				UID:         uid,
				PreExisting: event.PreExisting,
			}
			entryByUID[uid] = entry
			entries = append(entries, entry)
		}
		entry.Events = append(entry.Events, event)
	}

	for _, entry := range entries {
		sort.Slice(entry.Events, func(i, j int) bool {
			return entry.Events[i].Timestamp < entry.Events[j].Timestamp
		})
		if len(entry.Events) > 0 {
			entry.PreExisting = entry.Events[0].PreExisting
		}
	}

	if len(queryResult.K8sEventsByResource) > 0 {
		s.logger.Debug("Using K8sEventsByResource from graph executor: %d resources have events", len(queryResult.K8sEventsByResource))
		for _, entry := range entries {
			if events, ok := queryResult.K8sEventsByResource[entry.UID]; ok {
				entry.K8sEvents = append(entry.K8sEvents, events...)
			}
		}
	} else if eventResult != nil {
		for _, event := range eventResult.Events {
			if event.Resource.Kind != "Event" || event.Resource.InvolvedObjectUID == "" {
				continue
			}
			entry, ok := entryByUID[event.Resource.InvolvedObjectUID]
			if !ok {
				continue
			}
			entry.RawK8sEvents = append(entry.RawK8sEvents, event)
		}
	}

	index.entries = entries
	return index
}

func (s *Service) BuildTimelineResources(index *TimelineIndex, entries []*TimelineResourceEntry) []models.Resource {
	if len(entries) == 0 {
		return []models.Resource{}
	}

	resources := make([]models.Resource, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		resources = append(resources, s.buildTimelineResource(index, entry))
	}
	return resources
}

func (s *Service) BuildTimelineResponse(queryResult, eventResult *models.QueryResult) *models.SearchResponse {
	index := s.BuildTimelineIndex(queryResult, eventResult)
	resources := s.BuildTimelineResources(index, index.Entries())

	return &models.SearchResponse{
		Resources:       resources,
		Count:           len(resources),
		ExecutionTimeMs: index.ExecutionTimeMs(),
	}
}

func deriveTimelineQueryBounds(queryResult *models.QueryResult) (int64, int64) {
	queryStartTime := queryResult.QueryStartTime
	queryEndTime := queryResult.QueryEndTime
	if queryStartTime != 0 && queryEndTime != 0 {
		return queryStartTime, queryEndTime
	}

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

	return queryStartTime, queryEndTime
}

func buildTimelineResourceID(event models.Event) string {
	return fmt.Sprintf("%s/%s/%s/%s", event.Resource.Group, event.Resource.Version, event.Resource.Kind, event.Resource.UID)
}

func (s *Service) buildTimelineResource(index *TimelineIndex, entry *TimelineResourceEntry) models.Resource {
	resource := models.Resource{
		ID:          entry.ID,
		Group:       entry.Group,
		Version:     entry.Version,
		Kind:        entry.Kind,
		Namespace:   entry.Namespace,
		Name:        entry.Name,
		Events:      []models.K8sEvent{},
		PreExisting: entry.PreExisting,
	}

	for i, event := range entry.Events {
		status := analyzer.InferStatusFromResource(event.Resource.Kind, event.Data, string(event.Type))

		var endTime int64
		if i < len(entry.Events)-1 {
			endTime = entry.Events[i+1].Timestamp
		} else if index != nil {
			endTime = index.queryEndTime
		}

		startTime := event.Timestamp
		if index != nil && event.PreExisting && event.Type != models.EventTypeDelete && index.queryStartTime > 0 && event.Timestamp < index.queryStartTime {
			startTime = index.queryStartTime
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
				event.Resource.Namespace, event.Resource.Name, event.ID, len(entry.Events))
		}

		resource.StatusSegments = append(resource.StatusSegments, segment)
	}

	if len(entry.K8sEvents) > 0 {
		resource.Events = append(resource.Events, entry.K8sEvents...)
	}
	for _, rawEvent := range entry.RawK8sEvents {
		k8sEvent, ok := s.parseRawK8sEvent(rawEvent)
		if ok {
			resource.Events = append(resource.Events, k8sEvent)
		}
	}

	return resource
}

func (s *Service) parseRawK8sEvent(event models.Event) (models.K8sEvent, bool) {
	var eventData map[string]interface{}
	if len(event.Data) > 0 {
		if err := json.Unmarshal(event.Data, &eventData); err != nil {
			s.logger.Warn("Failed to parse event data: %v", err)
			return models.K8sEvent{}, false
		}
	}

	k8sEvent := models.K8sEvent{
		ID:        event.ID,
		Timestamp: event.Timestamp,
		Reason:    getTimelineStringField(eventData, "reason", ""),
		Message:   getTimelineStringField(eventData, "message", ""),
		Type:      getTimelineStringField(eventData, "type", "Normal"),
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

	return k8sEvent, true
}

func getTimelineStringField(m map[string]interface{}, key, defaultValue string) string {
	if m == nil {
		return defaultValue
	}
	if val, ok := m[key].(string); ok {
		return val
	}
	return defaultValue
}
