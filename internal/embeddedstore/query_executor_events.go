package embeddedstore

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/models"
)

func (qe *QueryExecutor) collectK8sEventsForResources(resources []filteredResource, startTimeNs, endTimeNs int64) map[string][]models.K8sEvent {
	if len(resources) == 0 {
		return nil
	}

	k8sEventsByResource := make(map[string][]models.K8sEvent)
	for _, resource := range resources {
		events := qe.projection.k8sRawEventsByInvolvedUID[resource.uid]
		if len(events) == 0 {
			continue
		}
		for _, event := range events {
			if event.Timestamp < startTimeNs || event.Timestamp > endTimeNs {
				continue
			}
			k8sEvent, ok := convertToK8sEvent(event)
			if !ok {
				continue
			}
			k8sEventsByResource[resource.uid] = append(k8sEventsByResource[resource.uid], k8sEvent)
		}
	}

	if len(k8sEventsByResource) == 0 {
		return nil
	}

	return k8sEventsByResource
}

func convertToK8sEvent(event models.Event) (models.K8sEvent, bool) {
	if event.Resource.Kind != "Event" {
		return models.K8sEvent{}, false
	}

	var eventData map[string]any
	if len(event.Data) > 0 {
		if err := json.Unmarshal(event.Data, &eventData); err != nil {
			return models.K8sEvent{}, false
		}
	}

	k8sEvent := models.K8sEvent{
		ID:        event.ID,
		Timestamp: event.Timestamp,
		Reason:    getString(eventData, "reason"),
		Message:   getString(eventData, "message"),
		Type:      getString(eventData, "type"),
		Count:     1,
	}
	if k8sEvent.Type == "" {
		k8sEvent.Type = "Normal"
	}

	if count, ok := eventData["count"].(float64); ok {
		k8sEvent.Count = int32(count)
	}
	if source, ok := eventData["source"].(map[string]any); ok {
		if component, ok := source["component"].(string); ok {
			k8sEvent.Source = component
		}
	}

	return k8sEvent, true
}

func (qe *QueryExecutor) executeEventQuery(startTimeNs, endTimeNs int64, filters models.QueryFilters, pagination *models.PaginationRequest, pageSize int, start time.Time) (*models.QueryResult, *models.PaginationResponse, error) {
	byUID := make(map[string][]models.Event)
	metaByUID := make(map[string]models.ResourceMetadata)

	for _, events := range qe.projection.k8sRawEventsByInvolvedUID {
		for _, event := range events {
			if event.Timestamp < startTimeNs || event.Timestamp > endTimeNs {
				continue
			}
			if !filters.Matches(event.Resource) {
				continue
			}
			uid := event.Resource.UID
			if uid == "" {
				continue
			}
			byUID[uid] = append(byUID[uid], cloneEvent(event))
			if _, ok := metaByUID[uid]; !ok {
				metaByUID[uid] = event.Resource
			}
		}
	}

	eventResources := make([]filteredResource, 0, len(metaByUID))
	for uid, meta := range metaByUID {
		events := byUID[uid]
		sort.Slice(events, func(i, j int) bool {
			return compareEventOrder(events[i], events[j]) < 0
		})
		eventResources = append(eventResources, filteredResource{
			orderedResourceKey: orderedResourceKey{
				kind:      meta.Kind,
				namespace: meta.Namespace,
				name:      meta.Name,
				uid:       uid,
			},
			events: events,
		})
	}

	sort.Slice(eventResources, func(i, j int) bool {
		return compareOrderedResourceKey(eventResources[i].orderedResourceKey, eventResources[j].orderedResourceKey) < 0
	})

	startIdx := qe.cursorStartIndex(eventResources, pagination)
	endIdx, hasMore, nextCursor := qe.pageBoundsWithCursor(eventResources, startIdx, pageSize)

	var resultEvents []models.Event
	for _, resource := range eventResources[startIdx:endIdx] {
		resultEvents = append(resultEvents, resource.events...)
	}

	executionTime := time.Since(start)
	queryResult := &models.QueryResult{
		Events:          resultEvents,
		Count:           int32(len(resultEvents)),            // #nosec G115 -- bounded by page size
		ExecutionTimeMs: int32(executionTime.Milliseconds()), // #nosec G115 -- bounded by duration
		QueryStartTime:  startTimeNs,
		QueryEndTime:    endTimeNs,
	}

	paginationResp := &models.PaginationResponse{
		NextCursor: nextCursor,
		HasMore:    hasMore,
		PageSize:   pageSize,
	}

	qe.recordQueryMetrics(queryFamilyResourceEvents, queryPlanStats{projectionUsed: true}, start, nil)
	return queryResult, paginationResp, nil
}
