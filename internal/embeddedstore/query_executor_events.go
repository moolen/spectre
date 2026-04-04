package embeddedstore

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/models"
)

func (qe *QueryExecutor) collectK8sEventsForResources(
	ctx context.Context,
	resources []filteredResource,
	startTimeNs, endTimeNs int64,
) (map[string][]models.K8sEvent, queryPlanStats, error) {
	if len(resources) == 0 {
		return nil, queryPlanStats{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, queryPlanStats{}, err
	}

	involvedUIDs, namespaces := resourceLookupInputs(resources)
	if len(involvedUIDs) == 0 {
		return nil, queryPlanStats{}, nil
	}

	if qe.planner != nil {
		associatedEvents, stats, err := qe.planner.collectAssociatedEvents(ctx, involvedUIDs, namespaces, startTimeNs, endTimeNs)
		if err != nil {
			return nil, stats, err
		}

		return convertAssociatedEventsToK8sEvents(associatedEvents), stats, nil
	}
	if !qe.projectionHistoryFallbackEnabled {
		return nil, queryPlanStats{}, fmt.Errorf("projection history fallback disabled")
	}

	return qe.collectAssociatedK8sEventsFromProjection(involvedUIDs, startTimeNs, endTimeNs), queryPlanStats{projectionUsed: true}, nil
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

func (qe *QueryExecutor) executeEventQuery(
	ctx context.Context,
	startTimeNs, endTimeNs int64,
	filters models.QueryFilters,
	pagination *models.PaginationRequest,
	pageSize int,
	start time.Time,
) (*models.QueryResult, *models.PaginationResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		qe.recordQueryMetrics(queryFamilyResourceEvents, queryPlanStats{}, start, err)
		return nil, nil, err
	}

	eventTimeline, stats, err := qe.eventTimelineEvents(ctx, startTimeNs, endTimeNs, filters)
	if err != nil {
		qe.recordQueryMetrics(queryFamilyResourceEvents, stats, start, err)
		return nil, nil, err
	}

	byUID := make(map[string][]models.Event)
	metaByUID := make(map[string]models.ResourceMetadata)

	for i := range eventTimeline {
		event := eventTimeline[i]
		uid := event.Resource.UID
		if uid == "" {
			continue
		}

		byUID[uid] = append(byUID[uid], cloneEvent(event))
		if _, ok := metaByUID[uid]; !ok {
			metaByUID[uid] = event.Resource
		}
	}

	eventResources := make([]filteredResource, 0, len(metaByUID))
	for uid, meta := range metaByUID {
		events := byUID[uid]
		sort.SliceStable(events, func(i, j int) bool {
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

	sort.SliceStable(eventResources, func(i, j int) bool {
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

	qe.recordQueryMetrics(queryFamilyResourceEvents, stats, start, nil)
	return queryResult, paginationResp, nil
}

func (qe *QueryExecutor) eventTimelineEvents(
	ctx context.Context,
	startTimeNs, endTimeNs int64,
	filters models.QueryFilters,
) ([]models.Event, queryPlanStats, error) {
	if qe.planner != nil {
		return qe.planner.exportTimeRange(ctx, startTimeNs, endTimeNs, filters)
	}
	if !qe.projectionHistoryFallbackEnabled {
		return nil, queryPlanStats{}, fmt.Errorf("projection history fallback disabled")
	}

	projectionEvents := qe.snapshotProjectionEvents()
	var timeline []models.Event
	for i := range projectionEvents {
		event := projectionEvents[i]
		if event.Timestamp < startTimeNs || event.Timestamp > endTimeNs {
			continue
		}
		if !filters.Matches(event.Resource) {
			continue
		}
		timeline = append(timeline, cloneEvent(event))
	}

	if len(timeline) == 0 {
		return nil, queryPlanStats{projectionUsed: true}, nil
	}

	sort.SliceStable(timeline, func(i, j int) bool {
		return compareEventOrder(timeline[i], timeline[j]) < 0
	})

	return dedupeEventsByID(timeline), queryPlanStats{projectionUsed: true}, nil
}

func resourceLookupInputs(resources []filteredResource) ([]string, []string) {
	uidSet := make(map[string]struct{}, len(resources))
	namespaceSet := make(map[string]struct{}, len(resources))
	disableNamespaceFilter := false

	for i := range resources {
		if resources[i].uid != "" {
			uidSet[resources[i].uid] = struct{}{}
		}
		if resources[i].namespace == "" {
			disableNamespaceFilter = true
			continue
		}
		namespaceSet[resources[i].namespace] = struct{}{}
	}

	involvedUIDs := make([]string, 0, len(uidSet))
	for uid := range uidSet {
		involvedUIDs = append(involvedUIDs, uid)
	}

	var namespaces []string
	if !disableNamespaceFilter {
		namespaces = make([]string, 0, len(namespaceSet))
		for namespace := range namespaceSet {
			namespaces = append(namespaces, namespace)
		}
	}

	return involvedUIDs, namespaces
}

func (qe *QueryExecutor) collectAssociatedEventsFromProjection(
	involvedUIDs []string,
	namespaces []string,
	startTimeNs, endTimeNs int64,
) map[string][]models.Event {
	if len(involvedUIDs) == 0 || endTimeNs < startTimeNs {
		return nil
	}

	targetUIDs := make(map[string]struct{}, len(involvedUIDs))
	for i := range involvedUIDs {
		targetUIDs[involvedUIDs[i]] = struct{}{}
	}

	namespaceSet := make(map[string]struct{}, len(namespaces))
	for i := range namespaces {
		namespaceSet[namespaces[i]] = struct{}{}
	}

	projectionEvents := qe.snapshotProjectionEvents()
	eventsByUID := make(map[string][]models.Event, len(targetUIDs))
	for i := range projectionEvents {
		event := projectionEvents[i]
		if event.Timestamp < startTimeNs || event.Timestamp > endTimeNs {
			continue
		}
		if event.Resource.Kind != "Event" {
			continue
		}
		if len(namespaceSet) > 0 {
			if _, ok := namespaceSet[event.Resource.Namespace]; !ok {
				continue
			}
		}

		involvedUID := event.Resource.InvolvedObjectUID
		if _, ok := targetUIDs[involvedUID]; !ok {
			continue
		}

		eventsByUID[involvedUID] = append(eventsByUID[involvedUID], cloneEvent(event))
	}

	if len(eventsByUID) == 0 {
		return nil
	}

	for uid := range eventsByUID {
		sort.SliceStable(eventsByUID[uid], func(i, j int) bool {
			return compareEventOrder(eventsByUID[uid][i], eventsByUID[uid][j]) < 0
		})
		eventsByUID[uid] = dedupeEventsByID(eventsByUID[uid])
	}

	return eventsByUID
}

func (qe *QueryExecutor) snapshotProjectionEvents() []models.Event {
	return qe.projection.SnapshotEvents()
}

func convertAssociatedEventsToK8sEvents(eventsByUID map[string][]models.Event) map[string][]models.K8sEvent {
	if len(eventsByUID) == 0 {
		return nil
	}

	k8sEventsByResource := make(map[string][]models.K8sEvent, len(eventsByUID))
	for uid, events := range eventsByUID {
		for i := range events {
			k8sEvent, ok := convertToK8sEvent(events[i])
			if !ok {
				continue
			}
			k8sEventsByResource[uid] = append(k8sEventsByResource[uid], k8sEvent)
		}
		if len(k8sEventsByResource[uid]) == 0 {
			delete(k8sEventsByResource, uid)
		}
	}

	if len(k8sEventsByResource) == 0 {
		return nil
	}

	return k8sEventsByResource
}

func (qe *QueryExecutor) collectAssociatedK8sEventsFromProjection(
	involvedUIDs []string,
	startTimeNs, endTimeNs int64,
) map[string][]models.K8sEvent {
	if len(involvedUIDs) == 0 || endTimeNs < startTimeNs {
		return nil
	}

	qe.projection.mu.RLock()
	defer qe.projection.mu.RUnlock()

	eventsByUID := make(map[string][]models.K8sEvent, len(involvedUIDs))
	for i := range involvedUIDs {
		uid := involvedUIDs[i]
		events := qe.projection.k8sEventsByInvolvedUID[uid]
		if len(events) == 0 {
			continue
		}

		for j := len(events) - 1; j >= 0; j-- {
			event := events[j]
			timestampNs := event.Timestamp.UnixNano()
			if timestampNs < startTimeNs || timestampNs > endTimeNs {
				continue
			}
			eventsByUID[uid] = append(eventsByUID[uid], models.K8sEvent{
				ID:        event.EventID,
				Timestamp: timestampNs,
				Reason:    event.Reason,
				Message:   event.Message,
				Type:      event.Type,
				Count:     int32(event.Count),
				Source:    event.Source,
			})
		}
	}

	if len(eventsByUID) == 0 {
		return nil
	}

	return eventsByUID
}
