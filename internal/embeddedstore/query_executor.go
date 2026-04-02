package embeddedstore

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

type filteredResource struct {
	orderedResourceKey
	events []models.Event
}

// QueryExecutor executes embedded timeline queries against a shared projection.
type QueryExecutor struct {
	logger     *logging.Logger
	projection *Projection
}

var _ api.QueryExecutor = (*QueryExecutor)(nil)

func NewQueryExecutor(projection *Projection) *QueryExecutor {
	if projection == nil {
		projection = NewProjection()
	}
	return &QueryExecutor{
		logger:     logging.GetLogger("embedded.query"),
		projection: projection,
	}
}

func (qe *QueryExecutor) Execute(ctx context.Context, query *models.QueryRequest) (*models.QueryResult, error) {
	result, _, err := qe.ExecutePaginated(ctx, query, query.Pagination)
	return result, err
}

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

	qe.projection.mu.RLock()
	defer qe.projection.mu.RUnlock()

	if qe.isEventQuery(query.Filters) {
		return qe.executeEventQuery(startTimeNs, endTimeNs, query.Filters, pagination, pageSize, start)
	}

	filteredResources := qe.collectFilteredResources(startTimeNs, endTimeNs, query.Filters)
	startIdx := qe.cursorStartIndex(filteredResources, pagination)
	endIdx, hasMore, nextCursor := qe.pageBoundsWithCursor(filteredResources, startIdx, pageSize)

	var resultEvents []models.Event
	k8sEventsByResource := qe.collectK8sEventsForResources(filteredResources[startIdx:endIdx], startTimeNs, endTimeNs)
	for _, resource := range filteredResources[startIdx:endIdx] {
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

	paginationResp := &models.PaginationResponse{
		NextCursor: nextCursor,
		HasMore:    hasMore,
		PageSize:   pageSize,
	}

	return queryResult, paginationResp, nil
}

func (qe *QueryExecutor) SetSharedCache(cache interface{}) {
}

func (qe *QueryExecutor) QueryDistinctMetadata(ctx context.Context, startTimeNs, endTimeNs int64) (namespaces []string, kinds []string, minTime int64, maxTime int64, err error) {
	qe.projection.mu.RLock()
	defer qe.projection.mu.RUnlock()

	namespacesSet := make(map[string]struct{})
	kindsSet := make(map[string]struct{})
	minTime = -1
	maxTime = -1

	for _, key := range qe.projection.orderedResources {
		meta, ok := qe.projection.resourceMetaByUID[key.uid]
		if !ok {
			continue
		}

		resourceEvents := qe.collectResourceEvents(qe.projection.eventsByResourceUID[key.uid], startTimeNs, endTimeNs)
		if len(resourceEvents) == 0 {
			continue
		}

		namespacesSet[meta.Namespace] = struct{}{}
		kindsSet[meta.Kind] = struct{}{}
		for _, event := range resourceEvents {
			if minTime < 0 || event.Timestamp < minTime {
				minTime = event.Timestamp
			}
			if maxTime < 0 || event.Timestamp > maxTime {
				maxTime = event.Timestamp
			}
		}
	}

	if minTime < 0 {
		minTime = 0
	}
	if maxTime < 0 {
		maxTime = 0
	}

	for ns := range namespacesSet {
		namespaces = append(namespaces, ns)
	}
	for kind := range kindsSet {
		kinds = append(kinds, kind)
	}

	sort.Strings(namespaces)
	sort.Strings(kinds)

	return namespaces, kinds, minTime, maxTime, nil
}

func (qe *QueryExecutor) collectFilteredResources(startTimeNs, endTimeNs int64, filters models.QueryFilters) []filteredResource {
	filtered := make([]filteredResource, 0, len(qe.projection.orderedResources))

	for _, key := range qe.projection.orderedResources {
		meta, ok := qe.projection.resourceMetaByUID[key.uid]
		if !ok {
			continue
		}
		if !filters.Matches(meta) {
			continue
		}

		resourceEvents := qe.collectResourceEvents(qe.projection.eventsByResourceUID[key.uid], startTimeNs, endTimeNs)
		if len(resourceEvents) == 0 {
			continue
		}

		filtered = append(filtered, filteredResource{
			orderedResourceKey: key,
			events:             resourceEvents,
		})
	}

	return filtered
}

func (qe *QueryExecutor) collectResourceEvents(events []models.Event, startTimeNs, endTimeNs int64) []models.Event {
	if len(events) == 0 {
		return nil
	}

	var inRange []models.Event
	var lastBefore models.Event
	var hasLastBefore bool

	for i := range events {
		event := events[i]
		if event.Timestamp < startTimeNs {
			lastBefore = cloneEvent(event)
			hasLastBefore = true
			continue
		}
		if event.Timestamp > endTimeNs {
			break
		}
		inRange = append(inRange, cloneEvent(event))
	}

	var result []models.Event
	if hasLastBefore && lastBefore.Type != models.EventTypeDelete {
		lastBefore.PreExisting = true
		result = append(result, lastBefore)
	}

	if len(inRange) == 0 && len(result) == 0 {
		return nil
	}

	result = append(result, inRange...)
	return result
}

func (qe *QueryExecutor) cursorStartIndex(resources []filteredResource, pagination *models.PaginationRequest) int {
	if pagination == nil || pagination.Cursor == "" {
		return 0
	}

	cursor, err := models.DecodeCursor(pagination.Cursor)
	if err != nil || cursor == nil {
		return 0
	}

	for i, resource := range resources {
		if compareCursorKey(resource, cursor) > 0 {
			return i
		}
	}

	return len(resources)
}

func (qe *QueryExecutor) pageBoundsWithCursor(resources []filteredResource, startIdx, pageSize int) (endIdx int, hasMore bool, nextCursor string) {
	if startIdx > len(resources) {
		startIdx = len(resources)
	}

	endIdx = startIdx + pageSize
	if endIdx > len(resources) {
		endIdx = len(resources)
	}

	if endIdx == 0 || endIdx == len(resources) {
		return endIdx, endIdx < len(resources), ""
	}

	lastKey := resources[endIdx-1]
	for endIdx < len(resources) {
		nextKey := resources[endIdx]
		if compareCursorKey(nextKey, &models.ResourceCursor{
			Kind:      lastKey.kind,
			Namespace: lastKey.namespace,
			Name:      lastKey.name,
		}) != 0 {
			break
		}
		endIdx++
	}

	hasMore = endIdx < len(resources)
	if hasMore && endIdx > 0 {
		last := resources[endIdx-1]
		nextCursor = models.NewResourceCursor(last.kind, last.namespace, last.name).Encode()
	}

	return endIdx, hasMore, nextCursor
}

func compareCursorKey(resource filteredResource, cursor *models.ResourceCursor) int {
	if resource.kind != cursor.Kind {
		if resource.kind < cursor.Kind {
			return -1
		}
		return 1
	}
	if resource.namespace != cursor.Namespace {
		if resource.namespace < cursor.Namespace {
			return -1
		}
		return 1
	}
	if resource.name != cursor.Name {
		if resource.name < cursor.Name {
			return -1
		}
		return 1
	}
	return 0
}

func (qe *QueryExecutor) isEventQuery(filters models.QueryFilters) bool {
	kinds := filters.GetKinds()
	return len(kinds) == 1 && kinds[0] == "Event"
}

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

	return queryResult, paginationResp, nil
}
