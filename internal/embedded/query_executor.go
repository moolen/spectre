package embedded

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

type resourceKey struct {
	kind      string
	namespace string
	name      string
	uid       string
}

// QueryExecutor executes timeline queries against in-memory events.
type QueryExecutor struct {
	logger                 *logging.Logger
	eventsByResourceUID    map[string][]models.Event
	resourceMetaByUID      map[string]models.ResourceMetadata
	k8sEventsByInvolvedUID map[string][]models.Event
	orderedResources       []resourceKey
	minTimestampNs         int64
	maxTimestampNs         int64
}

// NewQueryExecutor builds an in-memory executor from raw events.
func NewQueryExecutor(events []models.Event) (*QueryExecutor, error) {
	executor := &QueryExecutor{
		logger:                 logging.GetLogger("embedded.query"),
		eventsByResourceUID:    make(map[string][]models.Event),
		resourceMetaByUID:      make(map[string]models.ResourceMetadata),
		k8sEventsByInvolvedUID: make(map[string][]models.Event),
		minTimestampNs:         -1,
		maxTimestampNs:         -1,
	}

	latestMetaTimestamp := make(map[string]int64)

	for _, event := range events {
		uid := event.Resource.UID
		if uid == "" {
			executor.logger.Warn("Skipping event with missing resource UID (id=%s kind=%s)", event.ID, event.Resource.Kind)
			continue
		}

		if event.Resource.Kind == "Event" {
			if event.Resource.InvolvedObjectUID == "" {
				executor.logger.Warn("Skipping Event resource missing involvedObjectUid (id=%s uid=%s)", event.ID, uid)
				continue
			}
			executor.k8sEventsByInvolvedUID[event.Resource.InvolvedObjectUID] = append(
				executor.k8sEventsByInvolvedUID[event.Resource.InvolvedObjectUID],
				event,
			)
			continue
		}

		executor.eventsByResourceUID[uid] = append(executor.eventsByResourceUID[uid], event)
		if ts, ok := latestMetaTimestamp[uid]; !ok || event.Timestamp >= ts {
			executor.resourceMetaByUID[uid] = event.Resource
			latestMetaTimestamp[uid] = event.Timestamp
		}

		if executor.minTimestampNs < 0 || event.Timestamp < executor.minTimestampNs {
			executor.minTimestampNs = event.Timestamp
		}
		if executor.maxTimestampNs < 0 || event.Timestamp > executor.maxTimestampNs {
			executor.maxTimestampNs = event.Timestamp
		}
	}

	for uid, events := range executor.eventsByResourceUID {
		sort.Slice(events, func(i, j int) bool {
			return events[i].Timestamp < events[j].Timestamp
		})
		executor.eventsByResourceUID[uid] = events
	}

	executor.orderedResources = make([]resourceKey, 0, len(executor.resourceMetaByUID))
	for uid, meta := range executor.resourceMetaByUID {
		executor.orderedResources = append(executor.orderedResources, resourceKey{
			kind:      meta.Kind,
			namespace: meta.Namespace,
			name:      meta.Name,
			uid:       uid,
		})
	}

	sort.Slice(executor.orderedResources, func(i, j int) bool {
		if executor.orderedResources[i].kind != executor.orderedResources[j].kind {
			return executor.orderedResources[i].kind < executor.orderedResources[j].kind
		}
		if executor.orderedResources[i].namespace != executor.orderedResources[j].namespace {
			return executor.orderedResources[i].namespace < executor.orderedResources[j].namespace
		}
		if executor.orderedResources[i].name != executor.orderedResources[j].name {
			return executor.orderedResources[i].name < executor.orderedResources[j].name
		}
		return executor.orderedResources[i].uid < executor.orderedResources[j].uid
	})

	return executor, nil
}

// Execute executes a timeline query against the in-memory dataset.
func (qe *QueryExecutor) Execute(ctx context.Context, query *models.QueryRequest) (*models.QueryResult, error) {
	result, _, err := qe.ExecutePaginated(ctx, query, query.Pagination)
	return result, err
}

// ExecutePaginated executes a paginated timeline query against the in-memory dataset.
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
		return qe.executeEventQuery(startTimeNs, endTimeNs, query.Filters, pagination, pageSize, start)
	}

	filteredResources := qe.collectFilteredResources(startTimeNs, endTimeNs, query.Filters)

	startIdx := qe.cursorStartIndex(filteredResources, pagination)
	endIdx := startIdx + pageSize
	if endIdx > len(filteredResources) {
		endIdx = len(filteredResources)
	}

	hasMore := endIdx < len(filteredResources)
	var nextCursor string
	if hasMore && endIdx > 0 {
		last := filteredResources[endIdx-1]
		nextCursor = models.NewResourceCursor(last.kind, last.namespace, last.name).Encode()
	}

	var resultEvents []models.Event
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
		K8sEventsByResource: nil,
	}

	paginationResp := &models.PaginationResponse{
		NextCursor: nextCursor,
		HasMore:    hasMore,
		PageSize:   pageSize,
	}

	return queryResult, paginationResp, nil
}

// SetSharedCache is a no-op for embedded executor.
func (qe *QueryExecutor) SetSharedCache(cache interface{}) {
}

// QueryDistinctMetadata returns distinct namespaces/kinds and min/max timestamps within a time range.
func (qe *QueryExecutor) QueryDistinctMetadata(ctx context.Context, startTimeNs, endTimeNs int64) (namespaces []string, kinds []string, minTime int64, maxTime int64, err error) {
	namespacesSet := make(map[string]struct{})
	kindsSet := make(map[string]struct{})
	minTime = -1
	maxTime = -1

	for _, events := range qe.eventsByResourceUID {
		for _, event := range events {
			if event.Timestamp < startTimeNs || event.Timestamp > endTimeNs {
				continue
			}
			namespacesSet[event.Resource.Namespace] = struct{}{}
			kindsSet[event.Resource.Kind] = struct{}{}
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

type filteredResource struct {
	resourceKey
	events []models.Event
}

func (qe *QueryExecutor) collectFilteredResources(startTimeNs, endTimeNs int64, filters models.QueryFilters) []filteredResource {
	filtered := make([]filteredResource, 0, len(qe.orderedResources))

	for _, key := range qe.orderedResources {
		meta, ok := qe.resourceMetaByUID[key.uid]
		if !ok {
			continue
		}
		if !filters.Matches(meta) {
			continue
		}

		events := qe.eventsByResourceUID[key.uid]
		resourceEvents := qe.collectResourceEvents(events, startTimeNs, endTimeNs)
		if len(resourceEvents) == 0 {
			continue
		}

		filtered = append(filtered, filteredResource{
			resourceKey: key,
			events:      resourceEvents,
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
			lastBefore = event
			hasLastBefore = true
			continue
		}
		if event.Timestamp > endTimeNs {
			break
		}
		inRange = append(inRange, event)
	}

	var result []models.Event
	if hasLastBefore && lastBefore.Type != models.EventTypeDelete {
		preExisting := lastBefore
		preExisting.PreExisting = true
		result = append(result, preExisting)
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
		if resource.kind > cursor.Kind {
			return i
		}
		if resource.kind < cursor.Kind {
			continue
		}
		if resource.namespace > cursor.Namespace {
			return i
		}
		if resource.namespace < cursor.Namespace {
			continue
		}
		if resource.name > cursor.Name {
			return i
		}
	}

	return len(resources)
}

func (qe *QueryExecutor) isEventQuery(filters models.QueryFilters) bool {
	kinds := filters.GetKinds()
	return len(kinds) == 1 && kinds[0] == "Event"
}

func (qe *QueryExecutor) executeEventQuery(startTimeNs, endTimeNs int64, filters models.QueryFilters, pagination *models.PaginationRequest, pageSize int, start time.Time) (*models.QueryResult, *models.PaginationResponse, error) {
	byUID := make(map[string][]models.Event)
	metaByUID := make(map[string]models.ResourceMetadata)

	for _, events := range qe.k8sEventsByInvolvedUID {
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
			byUID[uid] = append(byUID[uid], event)
			if _, ok := metaByUID[uid]; !ok {
				metaByUID[uid] = event.Resource
			}
		}
	}

	eventResources := make([]filteredResource, 0, len(metaByUID))
	for uid, meta := range metaByUID {
		events := byUID[uid]
		sort.Slice(events, func(i, j int) bool {
			return events[i].Timestamp < events[j].Timestamp
		})
		eventResources = append(eventResources, filteredResource{
			resourceKey: resourceKey{
				kind:      meta.Kind,
				namespace: meta.Namespace,
				name:      meta.Name,
				uid:       uid,
			},
			events: events,
		})
	}

	sort.Slice(eventResources, func(i, j int) bool {
		if eventResources[i].kind != eventResources[j].kind {
			return eventResources[i].kind < eventResources[j].kind
		}
		if eventResources[i].namespace != eventResources[j].namespace {
			return eventResources[i].namespace < eventResources[j].namespace
		}
		if eventResources[i].name != eventResources[j].name {
			return eventResources[i].name < eventResources[j].name
		}
		return eventResources[i].uid < eventResources[j].uid
	})

	startIdx := qe.cursorStartIndex(eventResources, pagination)
	endIdx := startIdx + pageSize
	if endIdx > len(eventResources) {
		endIdx = len(eventResources)
	}

	hasMore := endIdx < len(eventResources)
	var nextCursor string
	if hasMore && endIdx > 0 {
		last := eventResources[endIdx-1]
		nextCursor = models.NewResourceCursor(last.kind, last.namespace, last.name).Encode()
	}

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
