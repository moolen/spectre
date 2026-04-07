package embeddedstore

import (
	"context"
	"fmt"
	"sort"
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
	if resources, paginationResp, stats, ok, err := qe.collectPaginatedRecentWindowResources(
		ctx,
		startTimeNs,
		endTimeNs,
		filters,
		pagination,
		pageSize,
	); ok || err != nil {
		return resources, paginationResp, stats, err
	}

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

func (qe *QueryExecutor) collectPaginatedRecentWindowResources(
	ctx context.Context,
	startTimeNs, endTimeNs int64,
	filters models.QueryFilters,
	pagination *models.PaginationRequest,
	pageSize int,
) ([]filteredResource, *models.PaginationResponse, queryPlanStats, bool, error) {
	if qe == nil || qe.projection == nil {
		return nil, nil, queryPlanStats{}, false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, queryPlanStats{}, true, err
	}

	cursor := decodePaginationCursor(pagination)

	qe.projection.mu.RLock()
	defer qe.projection.mu.RUnlock()

	changedUIDs, ok := qe.projection.recentWindowChangedUIDsLocked(startTimeNs)
	if !ok {
		return nil, nil, queryPlanStats{}, false, nil
	}

	changedResources := make([]filteredResource, 0, len(changedUIDs))
	for uid := range changedUIDs {
		record := qe.projection.resourcesByUID[uid]
		if record == nil {
			continue
		}

		meta := resourceRecordLatestMeta(record)
		if !filters.Matches(meta) {
			continue
		}

		events := resourceRecordTimelineEvents(record, startTimeNs, endTimeNs)
		if len(events) == 0 {
			continue
		}

		changedResources = append(changedResources, filteredResource{
			orderedResourceKey: resourceRecordCurrentKey(record),
			events:             events,
		})
	}

	sort.SliceStable(changedResources, func(i, j int) bool {
		return compareOrderedResourceKey(changedResources[i].orderedResourceKey, changedResources[j].orderedResourceKey) < 0
	})

	activeIdx := 0
	if cursor != nil {
		activeIdx = sort.Search(len(qe.projection.activeOrderedResources), func(i int) bool {
			return compareOrderedResourceKeyToCursor(qe.projection.activeOrderedResources[i], cursor) > 0
		})
	}
	changedIdx := 0
	if cursor != nil {
		changedIdx = sort.Search(len(changedResources), func(i int) bool {
			return compareOrderedResourceKeyToCursor(changedResources[i].orderedResourceKey, cursor) > 0
		})
	}

	nextUnchangedActive := func() (filteredResource, bool) {
		for activeIdx < len(qe.projection.activeOrderedResources) {
			key := qe.projection.activeOrderedResources[activeIdx]
			activeIdx++
			if _, changed := changedUIDs[key.uid]; changed {
				continue
			}

			record := qe.projection.resourcesByUID[key.uid]
			if record == nil {
				continue
			}
			meta := resourceRecordLatestMeta(record)
			if !filters.Matches(meta) {
				continue
			}
			preExisting, exists := resourceRecordPreExistingEvent(record)
			if !exists {
				continue
			}
			return filteredResource{
				orderedResourceKey: key,
				events:             []models.Event{preExisting},
			}, true
		}
		return filteredResource{}, false
	}

	var (
		activeNext   filteredResource
		activeLoaded bool
		changedNext  filteredResource
		changedReady bool
	)
	loadNextActive := func() {
		if activeLoaded {
			return
		}
		activeNext, activeLoaded = nextUnchangedActive()
	}
	loadNextChanged := func() {
		if changedReady {
			return
		}
		if changedIdx >= len(changedResources) {
			changedReady = false
			return
		}
		changedNext = changedResources[changedIdx]
		changedIdx++
		changedReady = true
	}
	popNext := func() (filteredResource, bool) {
		loadNextActive()
		loadNextChanged()
		switch {
		case activeLoaded && changedReady:
			if compareOrderedResourceKey(activeNext.orderedResourceKey, changedNext.orderedResourceKey) <= 0 {
				item := activeNext
				activeLoaded = false
				return item, true
			}
			item := changedNext
			changedReady = false
			return item, true
		case activeLoaded:
			item := activeNext
			activeLoaded = false
			return item, true
		case changedReady:
			item := changedNext
			changedReady = false
			return item, true
		default:
			return filteredResource{}, false
		}
	}

	filtered := make([]filteredResource, 0, pageSize)
	for {
		if err := ctx.Err(); err != nil {
			return nil, nil, queryPlanStats{projectionUsed: true}, true, err
		}

		resource, exists := popNext()
		if !exists {
			break
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
		}, queryPlanStats{projectionUsed: true}, true, nil
	}

	return filtered, &models.PaginationResponse{
		HasMore:  false,
		PageSize: pageSize,
	}, queryPlanStats{projectionUsed: true}, true, nil
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
