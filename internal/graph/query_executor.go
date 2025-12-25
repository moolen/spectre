package graph

import (
	"context"
	"fmt"
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
	start := time.Now()

	if err := query.Validate(); err != nil {
		return nil, fmt.Errorf("invalid query: %w", err)
	}

	qe.logger.DebugWithFields("Executing graph query",
		logging.Field("start_timestamp", query.StartTimestamp),
		logging.Field("end_timestamp", query.EndTimestamp),
		logging.Field("filters", fmt.Sprintf("%v", query.Filters)))

	// Convert timestamps to nanoseconds
	startTimeNs := query.StartTimestamp * 1e9
	endTimeNs := query.EndTimestamp * 1e9

	// Build Cypher query
	cypherQuery := qe.buildTimelineQuery(startTimeNs, endTimeNs, query.Filters)

	// Execute query
	result, err := qe.client.ExecuteQuery(ctx, cypherQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to execute graph query: %w", err)
	}

	// Convert graph results to events
	events, err := qe.parseTimelineResults(result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse graph results: %w", err)
	}

	executionTime := time.Since(start)

	return &models.QueryResult{
		Events:          events,
		Count:           int32(len(events)),
		ExecutionTimeMs: int32(executionTime.Milliseconds()),
		QueryStartTime:  startTimeNs,
		QueryEndTime:    endTimeNs,
	}, nil
}

// SetSharedCache is a no-op for graph executor (storage-only feature)
// This method exists to satisfy the QueryExecutor interface
func (qe *QueryExecutor) SetSharedCache(cache interface{}) {
	// Graph executor doesn't use shared cache
	// This is a storage-specific optimization
}

// buildTimelineQuery constructs a Cypher query for timeline data
func (qe *QueryExecutor) buildTimelineQuery(startNs, endNs int64, filters models.QueryFilters) GraphQuery {
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

	// Add filter conditions
	if filters.Kind != "" {
		whereConditions = append(whereConditions, "r.kind = $kind")
		params["kind"] = filters.Kind
	}

	if filters.Namespace != "" {
		whereConditions = append(whereConditions, "r.namespace = $namespace")
		params["namespace"] = filters.Namespace
	}

	if filters.Group != "" {
		whereConditions = append(whereConditions, "r.apiGroup = $apiGroup")
		params["apiGroup"] = filters.Group
	}

	// Build WHERE clause
	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = "WHERE " + whereConditions[0]
		for i := 1; i < len(whereConditions); i++ {
			whereClause += " AND " + whereConditions[i]
		}
	}

	// Fetch events in time range PLUS one event before the start time for context
	// This helps show the resource state at the beginning of the time window
	// IMPORTANT: Only return resources that have at least one event in the time range
	//            OR have a previous event (to show resource state at window start)
	query := fmt.Sprintf(`
		MATCH (r:ResourceIdentity)
		%s
		CALL {
			WITH r
			OPTIONAL MATCH (r)-[:CHANGED]->(e:ChangeEvent)
			WHERE e.timestamp >= $startNs AND e.timestamp <= $endNs
			RETURN collect(e) as inRangeEvents
		}
		CALL {
			WITH r
			OPTIONAL MATCH (r)-[:CHANGED]->(prev:ChangeEvent)
			WHERE prev.timestamp < $startNs
			RETURN prev
			ORDER BY prev.timestamp DESC
			LIMIT 1
		}
		OPTIONAL MATCH (r)-[:EMITTED_EVENT]->(k:K8sEvent)
		WHERE k.timestamp >= $startNs AND k.timestamp <= $endNs
		WITH r, inRangeEvents, prev, collect(DISTINCT k) as k8sEvents
		// Only include resources that have events in the time range OR a pre-existing event
		// BUT exclude resources where the ONLY event is a deletion outside the window
		WHERE size(inRangeEvents) > 0
		   OR (prev IS NOT NULL AND (NOT r.deleted OR r.deletedAt >= $startNs))
		RETURN r,
		       CASE WHEN prev IS NOT NULL THEN [prev] + inRangeEvents ELSE inRangeEvents END as events,
		       k8sEvents,
		       prev IS NOT NULL as hasPreExisting
		ORDER BY r.kind, r.namespace, r.name
	`, whereClause)

	qe.logger.Debug("Timeline Cypher query: %s", query)
	qe.logger.Debug("Timeline query params: startNs=%d, endNs=%d, kind=%v", startNs, endNs, filters.Kind)

	return GraphQuery{
		Query:      query,
		Parameters: params,
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
