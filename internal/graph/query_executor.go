package graph

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// QueryExecutor executes timeline queries against the graph database
type QueryExecutor struct {
	client Client
	logger *logging.Logger
}

// NewQueryExecutor creates a new graph-based query executor
func NewQueryExecutor(client Client) *QueryExecutor {
	return &QueryExecutor{
		client: client,
		logger: logging.GetLogger("graph.query"),
	}
}

// Execute executes a timeline query against the graph database
// This implements the same interface as storage.QueryExecutor
func (qe *QueryExecutor) Execute(ctx context.Context, query *models.QueryRequest) (*models.QueryResult, error) {
	// Use ExecutePaginated with nil pagination for backward compatibility
	result, _, err := qe.ExecutePaginated(ctx, query, nil)
	return result, err
}

// ExecutePaginated executes a paginated timeline query against the graph database
// Returns the query result and pagination response (cursor, hasMore)
func (qe *QueryExecutor) ExecutePaginated(ctx context.Context, query *models.QueryRequest, pagination *models.PaginationRequest) (*models.QueryResult, *models.PaginationResponse, error) {
	start := time.Now()

	if err := query.Validate(); err != nil {
		return nil, nil, fmt.Errorf("invalid query: %w", err)
	}

	qe.logger.DebugWithFields("Executing graph query",
		logging.Field("start_timestamp", query.StartTimestamp),
		logging.Field("end_timestamp", query.EndTimestamp),
		logging.Field("filters", fmt.Sprintf("%v", query.Filters)))

	// Convert timestamps to nanoseconds
	startTimeNs := query.StartTimestamp * 1e9
	endTimeNs := query.EndTimestamp * 1e9

	// Determine page size
	pageSize := models.DefaultPageSize
	if pagination != nil {
		pageSize = pagination.GetPageSize()
	}

	// Build Cypher query with pagination
	cypherQuery := qe.buildTimelineQuery(startTimeNs, endTimeNs, query.Filters, pagination)

	// Execute query
	result, err := qe.client.ExecuteQuery(ctx, cypherQuery)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute graph query: %w", err)
	}

	// Convert graph results to events
	allEvents, err := qe.parseTimelineResults(result)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse graph results: %w", err)
	}

	// Group events by resource UID to get actual resource count
	// This is what BuildResourcesFromEventsWithQueryTime does, but we need to do it here
	// to count resources, not events, for proper pagination
	resourceUIDs := make(map[string]bool)
	eventsByResource := make(map[string][]models.Event)

	for _, event := range allEvents {
		uid := event.Resource.UID
		if uid == "" {
			continue
		}
		resourceUIDs[uid] = true
		eventsByResource[uid] = append(eventsByResource[uid], event)
	}

	actualResourceCount := len(resourceUIDs)
	qe.logger.Debug("Resource-based pagination: %d events from %d ResourceIdentity nodes grouped into %d unique resources (pageSize=%d)",
		len(allEvents), actualResourceCount, actualResourceCount, pageSize)

	// Determine if there are more resources
	// We fetched (pageSize * 2) + 1 ResourceIdentity nodes
	// If we got that many or more unique resources, there are more pages
	hasMore := actualResourceCount > (pageSize * 2)

	// Apply resource-level pagination: limit to pageSize unique resources
	// Maintain sort order (kind, namespace, name) from the query
	var limitedEvents []models.Event
	var seenResources int
	seenUIDs := make(map[string]bool)

	// Sort resource UIDs by (kind, namespace, name) to maintain order
	// We'll use the first event of each resource to determine sort order
	type resourceInfo struct {
		uid    string
		kind   string
		ns     string
		name   string
		events []models.Event
	}

	resources := make([]resourceInfo, 0, actualResourceCount)
	for uid, events := range eventsByResource {
		if len(events) == 0 {
			continue
		}
		firstEvent := events[0]
		resources = append(resources, resourceInfo{
			uid:    uid,
			kind:   firstEvent.Resource.Kind,
			ns:     firstEvent.Resource.Namespace,
			name:   firstEvent.Resource.Name,
			events: events,
		})
	}

	// Sort by (kind, namespace, name) to match query ORDER BY
	// The query already applies cursor filtering, so all resources here are after the cursor
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].kind != resources[j].kind {
			return resources[i].kind < resources[j].kind
		}
		if resources[i].ns != resources[j].ns {
			return resources[i].ns < resources[j].ns
		}
		return resources[i].name < resources[j].name
	})

	// Take first pageSize resources (cursor filtering already done in query)
	var lastResourceIdx int = -1
	for i := 0; i < len(resources) && seenResources < pageSize; i++ {
		res := resources[i]
		limitedEvents = append(limitedEvents, res.events...)
		seenResources++
		seenUIDs[res.uid] = true
		lastResourceIdx = i // Track the index of the last resource we're including
	}

	// Check if there are more resources after the page
	// We fetched (pageSize * 2) + 1 resources, so if we have more than pageSize, there are more
	hasMore = len(resources) > pageSize

	// Build next cursor from last resource in the page
	var nextCursor string
	if hasMore && lastResourceIdx >= 0 && lastResourceIdx < len(resources) {
		lastRes := resources[lastResourceIdx]
		cursor := models.NewResourceCursor(lastRes.kind, lastRes.ns, lastRes.name)
		nextCursor = cursor.Encode()
		qe.logger.Debug("Generated nextCursor from last resource: kind=%s, ns=%s, name=%s",
			lastRes.kind, lastRes.ns, lastRes.name)
	}

	qe.logger.Debug("Resource pagination result: %d events from %d resources, hasMore=%v, nextCursor=%q",
		len(limitedEvents), seenResources, hasMore, nextCursor)

	executionTime := time.Since(start)

	queryResult := &models.QueryResult{
		Events:          limitedEvents,
		Count:           int32(len(limitedEvents)),
		ExecutionTimeMs: int32(executionTime.Milliseconds()),
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

// SetSharedCache is a no-op for graph executor (storage-only feature)
// This method exists to satisfy the QueryExecutor interface
func (qe *QueryExecutor) SetSharedCache(cache interface{}) {
	// Graph executor doesn't use shared cache
	// This is a storage-specific optimization
}

// buildTimelineQuery constructs a Cypher query for timeline data
// Supports multi-value filters and cursor-based pagination
func (qe *QueryExecutor) buildTimelineQuery(startNs, endNs int64, filters models.QueryFilters, pagination *models.PaginationRequest) GraphQuery {
	// Base query structure:
	// 1. Match resources with filters
	// 2. Optionally match change events in time range
	// 3. Optionally match K8s events
	// 4. Return aggregated data

	// Filter out resources that were deleted before the time window start
	// This prevents showing empty rows for resources that no longer exist
	// Also filter out resources deleted outside the window end (shouldn't happen but defensive)
	whereConditions := []string{
		"(NOT r.deleted OR (r.deletedAt >= $startNs AND r.deletedAt <= $endNs))",
	}

	params := map[string]interface{}{
		"startNs": startNs,
		"endNs":   endNs,
	}

	qe.logger.Debug("Timeline query filter: deleted resources outside window [%d, %d]", startNs, endNs)

	// Add multi-value filter conditions (use IN operator for arrays)
	kinds := filters.GetKinds()
	if len(kinds) > 0 {
		whereConditions = append(whereConditions, "r.kind IN $kinds")
		params["kinds"] = kinds
	}

	namespaces := filters.GetNamespaces()
	if len(namespaces) > 0 {
		whereConditions = append(whereConditions, "r.namespace IN $namespaces")
		params["namespaces"] = namespaces
	}

	if filters.Group != "" {
		whereConditions = append(whereConditions, "r.apiGroup = $apiGroup")
		params["apiGroup"] = filters.Group
	}

	// Add cursor-based pagination condition
	// The cursor encodes the last seen (kind, namespace, name)
	// We fetch resources AFTER this position in sort order
	if pagination != nil && pagination.Cursor != "" {
		cursor, err := models.DecodeCursor(pagination.Cursor)
		if err == nil && cursor != nil {
			// Composite comparison for stable cursor pagination
			// Resources must be "after" cursor in (kind, namespace, name) order
			cursorCondition := `(
				r.kind > $cursorKind OR
				(r.kind = $cursorKind AND r.namespace > $cursorNamespace) OR
				(r.kind = $cursorKind AND r.namespace = $cursorNamespace AND r.name > $cursorName)
			)`
			whereConditions = append(whereConditions, cursorCondition)
			params["cursorKind"] = cursor.Kind
			params["cursorNamespace"] = cursor.Namespace
			params["cursorName"] = cursor.Name
		}
	}

	// Build WHERE clause
	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = "WHERE " + whereConditions[0]
		for i := 1; i < len(whereConditions); i++ {
			whereClause += " AND " + whereConditions[i]
		}
	}

	// Determine page size and add LIMIT clause
	// Fetch more resources than requested to account for:
	// 1. Resources that might be filtered out during event-to-resource conversion
	// 2. Multiple events per resource (we need enough resources, not events)
	// Use a multiplier of 2-3x to ensure we get enough resources after conversion
	// The +1 is for hasMore detection
	pageSize := models.DefaultPageSize
	if pagination != nil {
		pageSize = pagination.GetPageSize()
	}
	// Fetch 4x more ResourceIdentity nodes to account for filtering during conversion
	// This ensures we get enough resources after grouping events by resource UID
	resourceLimit := (pageSize * 4) + 1
	params["limit"] = resourceLimit

	// Fetch events in time range PLUS one event before the start time for context
	// This helps show the resource state at the beginning of the time window
	// IMPORTANT: Only return resources that have at least one event in the time range
	//            OR have a previous event (to show resource state at window start)
	// Simplified query structure for better FalkorDB compatibility
	// The previous query with CALL subqueries and ORDER BY/LIMIT was causing timeouts
	// This version avoids complex subqueries and uses simple collect/head patterns
	query := fmt.Sprintf(`
		MATCH (r:ResourceIdentity)
		%s
		OPTIONAL MATCH (r)-[:CHANGED]->(e:ChangeEvent)
		WHERE e.timestamp >= $startNs AND e.timestamp <= $endNs
		WITH r, collect(e) as inRangeEvents
		OPTIONAL MATCH (r)-[:CHANGED]->(prev:ChangeEvent)
		WHERE prev.timestamp < $startNs
		WITH r, inRangeEvents, head(collect(prev)) as prev
		OPTIONAL MATCH (r)-[:EMITTED_EVENT]->(k:K8sEvent)
		WHERE k.timestamp >= $startNs AND k.timestamp <= $endNs
		WITH r, inRangeEvents, prev, collect(DISTINCT k) as k8sEvents
		WHERE size(inRangeEvents) > 0
		   OR (prev IS NOT NULL AND (NOT r.deleted OR r.deletedAt >= $startNs))
		RETURN r,
		       CASE WHEN prev IS NOT NULL THEN [prev] + inRangeEvents ELSE inRangeEvents END as events,
		       k8sEvents,
		       prev IS NOT NULL as hasPreExisting
		ORDER BY r.kind, r.namespace, r.name
		LIMIT $limit
	`, whereClause)

	qe.logger.Debug("Timeline Cypher query: %s", query)
	qe.logger.Debug("Timeline query params: startNs=%d, endNs=%d, kinds=%v, namespaces=%v, pageSize=%d, resourceLimit=%d",
		startNs, endNs, kinds, namespaces, pageSize, resourceLimit)

	return GraphQuery{
		Query:      query,
		Parameters: params,
		Timeout:    15000, // 15 seconds - timeline queries can be complex with many resources
	}
}

// parseTimelineResults converts graph query results into Event objects
func (qe *QueryExecutor) parseTimelineResults(result *QueryResult) ([]models.Event, error) {
	var events []models.Event

	// Parse result rows
	for _, row := range result.Rows {
		if len(row) < 4 {
			qe.logger.Warn("Unexpected row length: %d, expected 4 (r, events, k8sEvents, hasPreExisting)", len(row))
			continue
		}

		// Extract resource identity using proper parser
		resourceProps, err := ParseNodeFromResult(row[0])
		if err != nil {
			qe.logger.Warn("Failed to parse resource node: %v", err)
			continue
		}

		resourceMeta := qe.parseResourceMetadata(resourceProps)

		// Extract change events (this is a collection)
		changeEvents, ok := row[1].([]interface{})
		if !ok {
			qe.logger.Debug("No change events for resource %s", resourceMeta.UID)
			continue
		}

		// Extract hasPreExisting flag
		hasPreExisting := false
		if len(row) >= 4 {
			if preExisting, ok := row[3].(bool); ok {
				hasPreExisting = preExisting
			}
		}

		if hasPreExisting && len(changeEvents) > 0 {
			qe.logger.DebugWithFields("Resource has pre-existing event",
				logging.Field("resource", resourceMeta.UID),
				logging.Field("total_events", len(changeEvents)))
		}

		// Convert each ChangeEvent to a models.Event
		for i, eventData := range changeEvents {
			// Parse event node using proper parser
			eventProps, err := ParseNodeFromResult(eventData)
			if err != nil {
				qe.logger.Debug("Failed to parse change event node: %v", err)
				continue
			}

			event := qe.parseChangeEvent(eventProps, resourceMeta)
			if event != nil {
				// Mark first event as pre-existing if applicable
				if i == 0 && hasPreExisting {
					event.PreExisting = true
				}
				events = append(events, *event)
			}
		}

		// TODO: Attach K8s events (row[2] if present)
	}

	qe.logger.Info("Parsed %d events from graph query", len(events))
	return events, nil
}

// parseResourceMetadata extracts ResourceMetadata from a ResourceIdentity node
func (qe *QueryExecutor) parseResourceMetadata(node map[string]interface{}) models.ResourceMetadata {
	return models.ResourceMetadata{
		UID:       getStringField(node, "uid"),
		Kind:      getStringField(node, "kind"),
		Group:     getStringField(node, "apiGroup"),
		Version:   getStringField(node, "version"),
		Namespace: getStringField(node, "namespace"),
		Name:      getStringField(node, "name"),
		// Labels would need to be parsed from JSON string
	}
}

// parseChangeEvent converts a ChangeEvent node to a models.Event
func (qe *QueryExecutor) parseChangeEvent(node map[string]interface{}, resourceMeta models.ResourceMetadata) *models.Event {
	eventID := getStringField(node, "id")
	if eventID == "" {
		return nil
	}

	timestamp := getInt64Field(node, "timestamp")
	eventType := getStringField(node, "eventType")

	// Map graph eventType to models.EventType
	var evtType models.EventType
	switch eventType {
	case "CREATE":
		evtType = models.EventTypeCreate
	case "UPDATE":
		evtType = models.EventTypeUpdate
	case "DELETE":
		evtType = models.EventTypeDelete
	default:
		evtType = models.EventTypeUpdate
	}

	// Get resource data from ChangeEvent node
	dataStr := getStringField(node, "data")
	var data []byte
	if dataStr != "" {
		data = []byte(dataStr)
	}

	return &models.Event{
		ID:        eventID,
		Timestamp: timestamp,
		Type:      evtType,
		Resource:  resourceMeta,
		Data:      data, // Now populated from graph
	}
}

// Helper functions to safely extract fields from graph nodes

func getStringField(node map[string]interface{}, key string) string {
	if val, ok := node[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getInt64Field(node map[string]interface{}, key string) int64 {
	if val, ok := node[key]; ok {
		switch v := val.(type) {
		case int64:
			return v
		case int:
			return int64(v)
		case float64:
			return int64(v)
		}
	}
	return 0
}

func getBoolField(node map[string]interface{}, key string) bool {
	if val, ok := node[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// QueryDistinctMetadata queries for distinct namespaces and kinds in a time range
// without any pagination limits. This is specifically for the metadata endpoint.
func (qe *QueryExecutor) QueryDistinctMetadata(ctx context.Context, startTimeNs, endTimeNs int64) (namespaces []string, kinds []string, minTime int64, maxTime int64, err error) {
	// Build query to get distinct values
	query := `
		MATCH (r:ResourceIdentity)
		WHERE (NOT r.deleted OR (r.deletedAt >= $startNs AND r.deletedAt <= $endNs))
		OPTIONAL MATCH (r)-[:CHANGED]->(e:ChangeEvent)
		WHERE e.timestamp >= $startNs AND e.timestamp <= $endNs
		WITH DISTINCT r.namespace as namespace, r.kind as kind, e.timestamp as timestamp
		RETURN collect(DISTINCT namespace) as namespaces,
		       collect(DISTINCT kind) as kinds,
		       min(timestamp) as minTime,
		       max(timestamp) as maxTime
	`

	params := map[string]interface{}{
		"startNs": startTimeNs,
		"endNs":   endTimeNs,
	}

	graphQuery := GraphQuery{
		Query:      query,
		Parameters: params,
		Timeout:    10000, // 10 seconds
	}

	result, err := qe.client.ExecuteQuery(ctx, graphQuery)
	if err != nil {
		return nil, nil, 0, 0, fmt.Errorf("failed to execute metadata query: %w", err)
	}

	// Parse results
	if len(result.Rows) == 0 {
		return []string{}, []string{}, 0, 0, nil
	}

	row := result.Rows[0]
	if len(row) < 4 {
		return nil, nil, 0, 0, fmt.Errorf("unexpected result format: got %d columns, expected 4", len(row))
	}

	// Parse namespaces
	if nsList, ok := row[0].([]interface{}); ok {
		namespaces = make([]string, 0, len(nsList))
		for _, ns := range nsList {
			if nsStr, ok := ns.(string); ok {
				namespaces = append(namespaces, nsStr)
			}
		}
	}

	// Parse kinds
	if kindsList, ok := row[1].([]interface{}); ok {
		kinds = make([]string, 0, len(kindsList))
		for _, k := range kindsList {
			if kStr, ok := k.(string); ok {
				kinds = append(kinds, kStr)
			}
		}
	}

	// Parse min/max times
	if row[2] != nil {
		if mt, ok := row[2].(int64); ok {
			minTime = mt
		}
	}
	if row[3] != nil {
		if mt, ok := row[3].(int64); ok {
			maxTime = mt
		}
	}

	return namespaces, kinds, minTime, maxTime, nil
}
