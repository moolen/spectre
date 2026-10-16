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

// collectPaginatedResources selects one page of resources in (kind, namespace, name, uid)
// order. It walks the ordered resource index from the cursor and checks each resource's
// window against in-memory version metadata, so the cost depends on the page size and
// the resources skipped, not on how much history the window covers. Payloads are read
// only for the resources that make it onto the page.
func (qe *QueryExecutor) collectPaginatedResources(
	ctx context.Context,
	startTimeNs, endTimeNs int64,
	filters models.QueryFilters,
	pagination *models.PaginationRequest,
	pageSize int,
) ([]filteredResource, *models.PaginationResponse, queryPlanStats, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, queryPlanStats{}, err
	}
	stats := queryPlanStats{projectionUsed: true}
	cursor := decodePaginationCursor(pagination)

	projection := qe.projection
	projection.mu.RLock()
	defer projection.mu.RUnlock()

	type pageEntry struct {
		key    orderedResourceKey
		record *resourceRecord
	}
	page := make([]pageEntry, 0, pageSize)
	var paginationResp *models.PaginationResponse
	var walkErr error
	checked := 0
	projection.orderedResources.AscendAfterCursor(cursor, func(key orderedResourceKey) bool {
		if checked++; checked%4096 == 0 {
			if err := ctx.Err(); err != nil {
				walkErr = err
				return false
			}
		}
		record := projection.resourcesByUID[key.uid]
		if record == nil || !resourceRecordHasWindowEvents(record, startTimeNs, endTimeNs) {
			return true
		}
		if !filters.Matches(resourceRecordLatestMeta(record)) {
			return true
		}
		if len(page) >= pageSize {
			last := page[len(page)-1].key
			lastCursor := models.NewResourceCursor(last.kind, last.namespace, last.name)
			if compareOrderedResourceKeyToCursor(key, lastCursor) != 0 {
				paginationResp = &models.PaginationResponse{
					NextCursor: lastCursor.Encode(),
					HasMore:    true,
					PageSize:   pageSize,
				}
				return false
			}
		}
		page = append(page, pageEntry{key: key, record: record})
		return true
	})
	if walkErr != nil {
		return nil, nil, stats, walkErr
	}
	if paginationResp == nil {
		paginationResp = &models.PaginationResponse{HasMore: false, PageSize: pageSize}
	}

	filtered := make([]filteredResource, 0, len(page))
	for i := range page {
		events := resourceRecordTimelineEvents(page[i].record, startTimeNs, endTimeNs, maxTimelineVersionsPerResource)
		if len(events) == 0 {
			continue
		}
		filtered = append(filtered, filteredResource{orderedResourceKey: page[i].key, events: events})
	}
	return filtered, paginationResp, stats, nil
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
	if qe.projection != nil {
		return qe.projection.resourceTimelineEvents(uid, startTimeNs, endTimeNs), queryPlanStats{projectionUsed: true}, nil
	}

	planner := qe.sharedPlanner()
	if planner == nil {
		if !qe.projectionHistoryFallbackEnabled {
			return nil, queryPlanStats{}, fmt.Errorf("projection history fallback disabled")
		}

		qe.projection.mu.RLock()
		projectionEvents := qe.projection.resourceEventsForUID(uid)
		qe.projection.mu.RUnlock()

		return qe.collectResourceEvents(projectionEvents, startTimeNs, endTimeNs), queryPlanStats{projectionUsed: true}, nil
	}

	return planner.planResourceEvents(ctx, uid, meta, startTimeNs, endTimeNs)
}

func (qe *QueryExecutor) isEventQuery(filters models.QueryFilters) bool {
	kinds := filters.GetKinds()
	return len(kinds) == 1 && kinds[0] == "Event"
}
