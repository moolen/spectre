package namespacegraph

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// ResourceFetcher handles querying resources from the graph database
type ResourceFetcher struct {
	graphClient graph.Client
	logger      *logging.Logger
}

// NewResourceFetcher creates a new ResourceFetcher
func NewResourceFetcher(graphClient graph.Client) *ResourceFetcher {
	return &ResourceFetcher{
		graphClient: graphClient,
		logger:      logging.GetLogger("namespacegraph.fetcher"),
	}
}

// resourceResult holds the raw data from a resource query
type resourceResult struct {
	UID       string
	Kind      string
	APIGroup  string
	Namespace string
	Name      string
	Labels    map[string]string
	FirstSeen int64
	LastSeen  int64
	Deleted   bool
	DeletedAt int64
}

// FetchNamespacedResources fetches resources in the given namespace at the specified timestamp
func (f *ResourceFetcher) FetchNamespacedResources(
	ctx context.Context,
	namespace string,
	timestamp int64,
	limit int,
	cursor string,
) ([]resourceResult, bool, string, error) {
	// Parse cursor if provided
	var lastKind, lastName string
	if cursor != "" {
		pc, err := decodeCursor(cursor)
		if err != nil {
			f.logger.Warn("Failed to decode cursor, starting from beginning: %v", err)
		} else {
			lastKind = pc.LastKind
			lastName = pc.LastName
		}
	}

	// Build the query with cursor-based pagination
	var cypherQuery string
	params := map[string]interface{}{
		"namespace": namespace,
		"timestamp": timestamp,
		"limit":     limit + 1, // Fetch one extra to check if there are more
	}

	if lastKind != "" || lastName != "" {
		// Continue from cursor position
		// Note: Event kind is excluded as it clutters the graph and doesn't provide useful visualization
		// Note: We require at least one ChangeEvent to filter out "stub" nodes created from K8s Event
		// involvedObject references - these stubs may reference deleted resources we never saw lifecycle events for
		cypherQuery = `
			MATCH (r:ResourceIdentity)-[:CHANGED]->(:ChangeEvent)
			WHERE r.namespace = $namespace
			  AND r.firstSeen <= $timestamp
			  AND (r.deleted = false OR r.deleted IS NULL OR r.deletedAt > $timestamp)
			  AND r.kind <> 'Event'
			  AND ((r.kind > $lastKind) OR (r.kind = $lastKind AND r.name > $lastName))
			RETURN DISTINCT r.uid as uid, r.kind as kind, r.apiGroup as apiGroup, r.namespace as namespace, 
			       r.name as name, r.labels as labels, r.firstSeen as firstSeen, r.lastSeen as lastSeen,
			       r.deleted as deleted, r.deletedAt as deletedAt
			ORDER BY r.kind, r.name
			LIMIT $limit
		`
		params["lastKind"] = lastKind
		params["lastName"] = lastName
	} else {
		// Start from beginning
		// Note: Event kind is excluded as it clutters the graph and doesn't provide useful visualization
		// Note: We require at least one ChangeEvent to filter out "stub" nodes created from K8s Event
		// involvedObject references - these stubs may reference deleted resources we never saw lifecycle events for
		cypherQuery = `
			MATCH (r:ResourceIdentity)-[:CHANGED]->(:ChangeEvent)
			WHERE r.namespace = $namespace
			  AND r.firstSeen <= $timestamp
			  AND (r.deleted = false OR r.deleted IS NULL OR r.deletedAt > $timestamp)
			  AND r.kind <> 'Event'
			RETURN DISTINCT r.uid as uid, r.kind as kind, r.apiGroup as apiGroup, r.namespace as namespace, 
			       r.name as name, r.labels as labels, r.firstSeen as firstSeen, r.lastSeen as lastSeen,
			       r.deleted as deleted, r.deletedAt as deletedAt
			ORDER BY r.kind, r.name
			LIMIT $limit
		`
	}

	query := graph.GraphQuery{
		Timeout:    QueryTimeoutMs,
		Query:      cypherQuery,
		Parameters: params,
	}

	result, err := f.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, false, "", fmt.Errorf("failed to fetch namespaced resources: %w", err)
	}

	resources := parseResourceResults(result)

	// Check if there are more results
	hasMore := len(resources) > limit
	if hasMore {
		resources = resources[:limit] // Remove the extra one we fetched
	}

	// Generate next cursor if there are more results
	var nextCursor string
	if hasMore && len(resources) > 0 {
		lastResource := resources[len(resources)-1]
		nextCursor = encodeCursor(PaginationCursor{
			LastKind: lastResource.Kind,
			LastName: lastResource.Name,
		})
	}

	return resources, hasMore, nextCursor, nil
}

// FetchClusterScopedResources fetches cluster-scoped resources related to the given namespaced resources
// This uses direct relationship matching for better performance
func (f *ResourceFetcher) FetchClusterScopedResources(
	ctx context.Context,
	namespacedUIDs []string,
	timestamp int64,
	maxDepth int,
) ([]resourceResult, error) {
	if len(namespacedUIDs) == 0 {
		return nil, nil
	}

	// For depth 1, use a simple direct match which is much faster
	// For deeper traversals, we still use variable-length paths but with limits
	// Note: We require ChangeEvent to filter out stub nodes from K8s Event involvedObject references
	var cypherQuery string
	if maxDepth <= 1 {
		// Optimized single-hop query - much faster than variable-length paths
		cypherQuery = `
			UNWIND $uids AS uid
			MATCH (r:ResourceIdentity {uid: uid})-[]-(cs:ResourceIdentity)-[:CHANGED]->(:ChangeEvent)
			WHERE (cs.namespace = '' OR cs.namespace IS NULL)
			  AND cs.firstSeen <= $timestamp
			  AND (cs.deleted = false OR cs.deleted IS NULL OR cs.deletedAt > $timestamp)
			RETURN DISTINCT cs.uid as uid, cs.kind as kind, cs.apiGroup as apiGroup, cs.namespace as namespace,
			       cs.name as name, cs.labels as labels, cs.firstSeen as firstSeen, cs.lastSeen as lastSeen,
			       cs.deleted as deleted, cs.deletedAt as deletedAt
			LIMIT 100
		`
	} else {
		// Variable-length path for deeper traversals (use sparingly)
		cypherQuery = `
			UNWIND $uids AS uid
			MATCH (r:ResourceIdentity {uid: uid})-[*1..` + fmt.Sprintf("%d", maxDepth) + `]-(cs:ResourceIdentity)-[:CHANGED]->(:ChangeEvent)
			WHERE (cs.namespace = '' OR cs.namespace IS NULL)
			  AND cs.firstSeen <= $timestamp
			  AND (cs.deleted = false OR cs.deleted IS NULL OR cs.deletedAt > $timestamp)
			RETURN DISTINCT cs.uid as uid, cs.kind as kind, cs.apiGroup as apiGroup, cs.namespace as namespace,
			       cs.name as name, cs.labels as labels, cs.firstSeen as firstSeen, cs.lastSeen as lastSeen,
			       cs.deleted as deleted, cs.deletedAt as deletedAt
			LIMIT 100
		`
	}

	query := graph.GraphQuery{
		Timeout: QueryTimeoutMs,
		Query:   cypherQuery,
		Parameters: map[string]interface{}{
			"uids":      namespacedUIDs,
			"timestamp": timestamp,
		},
	}

	result, err := f.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch cluster-scoped resources: %w", err)
	}

	return parseResourceResults(result), nil
}

// FetchLatestEvents fetches the latest change event for each resource
func (f *ResourceFetcher) FetchLatestEvents(
	ctx context.Context,
	resourceUIDs []string,
	timestamp int64,
) (map[string]*ChangeEventInfo, error) {
	if len(resourceUIDs) == 0 {
		return make(map[string]*ChangeEventInfo), nil
	}

	cypherQuery := `
		UNWIND $uids AS uid
		MATCH (r:ResourceIdentity {uid: uid})-[:CHANGED]->(e:ChangeEvent)
		WHERE e.timestamp <= $timestamp
		WITH r.uid as resourceUID, e
		ORDER BY e.timestamp DESC
		WITH resourceUID, collect(e)[0] as latestEvent
		WHERE latestEvent IS NOT NULL
		RETURN resourceUID, 
		       latestEvent.timestamp as timestamp, 
		       latestEvent.eventType as eventType,
		       latestEvent.status as status,
		       latestEvent.errorMessage as errorMessage,
		       latestEvent.containerIssues as containerIssues,
		       latestEvent.impactScore as impactScore
	`

	query := graph.GraphQuery{
		Timeout: QueryTimeoutMs,
		Query:   cypherQuery,
		Parameters: map[string]interface{}{
			"uids":      resourceUIDs,
			"timestamp": timestamp,
		},
	}

	result, err := f.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest events: %w", err)
	}

	events := make(map[string]*ChangeEventInfo)
	for _, row := range result.Rows {
		if len(row) < 7 {
			continue
		}

		resourceUID, _ := row[0].(string)
		if resourceUID == "" {
			continue
		}

		event := &ChangeEventInfo{}

		// Parse timestamp
		if ts, ok := row[1].(int64); ok {
			event.Timestamp = ts
		} else if ts, ok := row[1].(float64); ok {
			event.Timestamp = int64(ts)
		}

		// Parse eventType
		if et, ok := row[2].(string); ok {
			event.EventType = et
		}

		// Parse status
		if status, ok := row[3].(string); ok {
			event.Status = status
		}

		// Parse errorMessage
		if errMsg, ok := row[4].(string); ok {
			event.ErrorMessage = errMsg
		}

		// Parse containerIssues (stored as JSON array string or native array)
		if issues, ok := row[5].([]interface{}); ok {
			for _, issue := range issues {
				if s, ok := issue.(string); ok {
					event.ContainerIssues = append(event.ContainerIssues, s)
				}
			}
		} else if issuesStr, ok := row[5].(string); ok && issuesStr != "" {
			// Try parsing as JSON array if stored as string
			var issues []string
			if err := json.Unmarshal([]byte(issuesStr), &issues); err == nil {
				event.ContainerIssues = issues
			}
		}

		// Parse impactScore
		if score, ok := row[6].(float64); ok {
			event.ImpactScore = score
		}

		events[resourceUID] = event
	}

	return events, nil
}

// parseResourceResults parses the query result into resourceResult structs
func parseResourceResults(result *graph.QueryResult) []resourceResult {
	resources := make([]resourceResult, 0, len(result.Rows))

	for _, row := range result.Rows {
		if len(row) < 10 {
			continue
		}

		r := resourceResult{}

		if uid, ok := row[0].(string); ok {
			r.UID = uid
		}
		if kind, ok := row[1].(string); ok {
			r.Kind = kind
		}
		if apiGroup, ok := row[2].(string); ok {
			r.APIGroup = apiGroup
		}
		if namespace, ok := row[3].(string); ok {
			r.Namespace = namespace
		}
		if name, ok := row[4].(string); ok {
			r.Name = name
		}
		if labels, ok := row[5].(map[string]interface{}); ok {
			r.Labels = make(map[string]string)
			for k, v := range labels {
				if vs, ok := v.(string); ok {
					r.Labels[k] = vs
				}
			}
		}
		if firstSeen, ok := row[6].(int64); ok {
			r.FirstSeen = firstSeen
		} else if firstSeen, ok := row[6].(float64); ok {
			r.FirstSeen = int64(firstSeen)
		}
		if lastSeen, ok := row[7].(int64); ok {
			r.LastSeen = lastSeen
		} else if lastSeen, ok := row[7].(float64); ok {
			r.LastSeen = int64(lastSeen)
		}
		if deleted, ok := row[8].(bool); ok {
			r.Deleted = deleted
		}
		if deletedAt, ok := row[9].(int64); ok {
			r.DeletedAt = deletedAt
		} else if deletedAt, ok := row[9].(float64); ok {
			r.DeletedAt = int64(deletedAt)
		}

		if r.UID != "" {
			resources = append(resources, r)
		}
	}

	return resources
}

// decodeCursor decodes a base64-encoded pagination cursor
func decodeCursor(cursor string) (*PaginationCursor, error) {
	data, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return nil, fmt.Errorf("failed to decode cursor: %w", err)
	}

	var pc PaginationCursor
	if err := json.Unmarshal(data, &pc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cursor: %w", err)
	}

	return &pc, nil
}

// encodeCursor encodes a pagination cursor to base64
func encodeCursor(pc PaginationCursor) string {
	data, _ := json.Marshal(pc)
	return base64.StdEncoding.EncodeToString(data)
}
