package namespacegraph

import (
	"context"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
)

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
	lastKind, lastName := f.decodePaginationCursor(cursor)

	query := graph.GraphQuery{
		Timeout:    QueryTimeoutMs,
		Query:      buildNamespacedResourcesQuery(lastKind, lastName),
		Parameters: buildNamespacedResourcesParams(namespace, timestamp, limit, lastKind, lastName),
	}

	result, err := f.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, false, "", fmt.Errorf("failed to fetch namespaced resources: %w", err)
	}

	resources := parseResourceResults(result)
	hasMore := len(resources) > limit
	if hasMore {
		resources = resources[:limit]
	}

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

// FetchClusterScopedResources fetches cluster-scoped resources related to the given namespaced resources.
func (f *ResourceFetcher) FetchClusterScopedResources(
	ctx context.Context,
	namespacedUIDs []string,
	timestamp int64,
	maxDepth int,
) ([]resourceResult, error) {
	if len(namespacedUIDs) == 0 {
		return nil, nil
	}

	query := graph.GraphQuery{
		Timeout: QueryTimeoutMs,
		Query:   buildClusterScopedResourcesQuery(maxDepth),
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

func (f *ResourceFetcher) decodePaginationCursor(cursor string) (string, string) {
	if cursor == "" {
		return "", ""
	}

	pc, err := decodeCursor(cursor)
	if err != nil {
		f.logger.Warn("Failed to decode cursor, starting from beginning: %v", err)
		return "", ""
	}

	return pc.LastKind, pc.LastName
}

func buildNamespacedResourcesParams(
	namespace string,
	timestamp int64,
	limit int,
	lastKind string,
	lastName string,
) map[string]interface{} {
	params := map[string]interface{}{
		"namespace": namespace,
		"timestamp": timestamp,
		"limit":     limit + 1,
	}

	if lastKind != "" || lastName != "" {
		params["lastKind"] = lastKind
		params["lastName"] = lastName
	}

	return params
}

func buildNamespacedResourcesQuery(lastKind string, lastName string) string {
	baseQuery := `
		MATCH (r:ResourceIdentity)-[:CHANGED]->(:ChangeEvent)
		WHERE r.namespace = $namespace
		  AND r.firstSeen <= $timestamp
		  AND (r.deleted = false OR r.deleted IS NULL OR r.deletedAt > $timestamp)
		  AND r.kind <> 'Event'
	`

	if lastKind != "" || lastName != "" {
		baseQuery += `
		  AND ((r.kind > $lastKind) OR (r.kind = $lastKind AND r.name > $lastName))
		`
	}

	baseQuery += `
		RETURN DISTINCT r.uid as uid, r.kind as kind, r.apiGroup as apiGroup, r.namespace as namespace,
		       r.name as name, r.labels as labels, r.firstSeen as firstSeen, r.lastSeen as lastSeen,
		       r.deleted as deleted, r.deletedAt as deletedAt
		ORDER BY r.kind, r.name
		LIMIT $limit
	`

	return baseQuery
}

func buildClusterScopedResourcesQuery(maxDepth int) string {
	if maxDepth <= 1 {
		return `
			MATCH (r:ResourceIdentity)-[]-(cs:ResourceIdentity)-[:CHANGED]->(:ChangeEvent)
			WHERE r.uid IN $uids
			  AND (cs.namespace = '' OR cs.namespace IS NULL)
			  AND cs.firstSeen <= $timestamp
			  AND (cs.deleted = false OR cs.deleted IS NULL OR cs.deletedAt > $timestamp)
			RETURN DISTINCT cs.uid as uid, cs.kind as kind, cs.apiGroup as apiGroup, cs.namespace as namespace,
			       cs.name as name, cs.labels as labels, cs.firstSeen as firstSeen, cs.lastSeen as lastSeen,
			       cs.deleted as deleted, cs.deletedAt as deletedAt
			LIMIT 100
		`
	}

	return `
		MATCH (r:ResourceIdentity)-[*1..` + fmt.Sprintf("%d", maxDepth) + `]-(cs:ResourceIdentity)-[:CHANGED]->(:ChangeEvent)
		WHERE r.uid IN $uids
		  AND (cs.namespace = '' OR cs.namespace IS NULL)
		  AND cs.firstSeen <= $timestamp
		  AND (cs.deleted = false OR cs.deleted IS NULL OR cs.deletedAt > $timestamp)
		RETURN DISTINCT cs.uid as uid, cs.kind as kind, cs.apiGroup as apiGroup, cs.namespace as namespace,
		       cs.name as name, cs.labels as labels, cs.firstSeen as firstSeen, cs.lastSeen as lastSeen,
		       cs.deleted as deleted, cs.deletedAt as deletedAt
		LIMIT 100
	`
}
