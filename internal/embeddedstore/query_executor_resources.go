package embeddedstore

import (
	"context"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/models"
)

func (qe *QueryExecutor) ExecutePaginated(ctx context.Context, query *models.QueryRequest, pagination *models.PaginationRequest) (*models.QueryResult, *models.PaginationResponse, error) {
	start := time.Now()

	if err := query.Validate(); err != nil {
		return nil, nil, fmt.Errorf("invalid query: %w", err)
	}

	startTimeNs := query.StartTimestamp * 1e9
	endTimeNs := query.EndTimestamp * 1e9

	pageSize := models.DefaultPageSize
	if pagination != nil {
		pageSize = pagination.GetPageSize()
	}

	if qe.isEventQuery(query.Filters) {
		return qe.executeEventQuery(ctx, startTimeNs, endTimeNs, query.Filters, pagination, pageSize, start)
	}

	filteredResources, paginationResp, stats, err := qe.collectPaginatedResources(ctx, startTimeNs, endTimeNs, query.Filters, pagination, pageSize)
	if err != nil {
		qe.recordQueryMetrics(queryFamilyResourceEvents, stats, start, err)
		return nil, nil, err
	}

	var resultEvents []models.Event
	k8sEventsByResource, k8sEventStats, err := qe.collectK8sEventsForResources(ctx, filteredResources, startTimeNs, endTimeNs)
	stats = stats.merge(k8sEventStats)
	if err != nil {
		qe.recordQueryMetrics(queryFamilyResourceEvents, stats, start, err)
		return nil, nil, err
	}
	for _, resource := range filteredResources {
		resultEvents = append(resultEvents, resource.events...)
	}

	executionTime := time.Since(start)
	queryResult := &models.QueryResult{
		Events:              resultEvents,
		Count:               int32(len(resultEvents)),            // #nosec G115 -- bounded by page size
		ExecutionTimeMs:     int32(executionTime.Milliseconds()), // #nosec G115 -- bounded by duration
		QueryStartTime:      startTimeNs,
		QueryEndTime:        endTimeNs,
		K8sEventsByResource: k8sEventsByResource,
	}

	qe.recordQueryMetrics(queryFamilyResourceEvents, stats, start, nil)
	return queryResult, paginationResp, nil
}

func (qe *QueryExecutor) collectPaginatedResources(
	ctx context.Context,
	startTimeNs, endTimeNs int64,
	filters models.QueryFilters,
	pagination *models.PaginationRequest,
	pageSize int,
) ([]filteredResource, *models.PaginationResponse, queryPlanStats, error) {
	orderedResources, metaByUID, _, _ := qe.snapshotResourceMetadata()
	filtered := make([]filteredResource, 0, pageSize)
	stats := queryPlanStats{}
	cursor := decodePaginationCursor(pagination)

	for _, key := range orderedResources {
		if cursor != nil && compareOrderedResourceKeyToCursor(key, cursor) <= 0 {
			continue
		}

		meta, ok := metaByUID[key.uid]
		if !ok {
			continue
		}
		if !filters.Matches(meta) {
			continue
		}

		resourceEvents, resourceStats, err := qe.resourceEvents(ctx, key.uid, meta, startTimeNs, endTimeNs)
		if err != nil {
			stats = stats.merge(resourceStats)
			return nil, nil, stats, err
		}
		stats = stats.merge(resourceStats)
		if len(resourceEvents) == 0 {
			continue
		}

		resource := filteredResource{
			orderedResourceKey: key,
			events:             resourceEvents,
		}

		if len(filtered) < pageSize {
			filtered = append(filtered, resource)
			continue
		}

		lastResource := filtered[len(filtered)-1]
		lastCursor := models.NewResourceCursor(lastResource.kind, lastResource.namespace, lastResource.name)
		if compareCursorKey(resource, lastCursor) == 0 {
			filtered = append(filtered, resource)
			continue
		}

		return filtered, &models.PaginationResponse{
			NextCursor: lastCursor.Encode(),
			HasMore:    true,
			PageSize:   pageSize,
		}, stats, nil
	}

	return filtered, &models.PaginationResponse{
		HasMore:  false,
		PageSize: pageSize,
	}, stats, nil
}

func (qe *QueryExecutor) collectResourceEvents(events []models.Event, startTimeNs, endTimeNs int64) []models.Event {
	return resourceEventsInWindow(events, startTimeNs, endTimeNs)
}

func (qe *QueryExecutor) resourceEvents(
	ctx context.Context,
	uid string,
	meta models.ResourceMetadata,
	startTimeNs, endTimeNs int64,
) ([]models.Event, queryPlanStats, error) {
	if qe.planner == nil {
		if !qe.projectionHistoryFallbackEnabled {
			return nil, queryPlanStats{}, fmt.Errorf("projection history fallback disabled")
		}

		qe.projection.mu.RLock()
		projectionEvents := qe.projection.resourceEventsForUID(uid)
		qe.projection.mu.RUnlock()

		return qe.collectResourceEvents(projectionEvents, startTimeNs, endTimeNs), queryPlanStats{projectionUsed: true}, nil
	}

	return qe.planner.planResourceEvents(ctx, uid, meta, startTimeNs, endTimeNs)
}

func (qe *QueryExecutor) isEventQuery(filters models.QueryFilters) bool {
	kinds := filters.GetKinds()
	return len(kinds) == 1 && kinds[0] == "Event"
}
