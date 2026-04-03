package falkor

import (
	"context"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
)

func (s *Store) fetchNamespacedResources(
	ctx context.Context,
	namespace string,
	timestamp int64,
	limit int,
	cursor string,
) ([]resourceResult, bool, string, error) {
	var lastKind, lastName string
	if cursor != "" {
		pc, err := decodeCursor(cursor)
		if err == nil {
			lastKind = pc.LastKind
			lastName = pc.LastName
		}
	}

	params := map[string]interface{}{
		"namespace": namespace,
		"timestamp": timestamp,
		"limit":     limit + 1,
	}

	query := `
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

	if lastKind != "" || lastName != "" {
		query = `
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
	}

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Timeout:    queryTimeoutMs,
		Query:      query,
		Parameters: params,
	})
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
		nextCursor = encodeCursor(paginationCursor{
			LastKind: lastResource.Kind,
			LastName: lastResource.Name,
		})
	}

	return resources, hasMore, nextCursor, nil
}

func (s *Store) fetchClusterScopedResources(
	ctx context.Context,
	namespacedUIDs []string,
	timestamp int64,
	maxDepth int,
) ([]resourceResult, error) {
	if len(namespacedUIDs) == 0 {
		return nil, nil
	}

	query := `
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

	if maxDepth > 1 {
		query = fmt.Sprintf(`
			MATCH (r:ResourceIdentity)-[*1..%d]-(cs:ResourceIdentity)-[:CHANGED]->(:ChangeEvent)
			WHERE r.uid IN $uids
			  AND (cs.namespace = '' OR cs.namespace IS NULL)
			  AND cs.firstSeen <= $timestamp
			  AND (cs.deleted = false OR cs.deleted IS NULL OR cs.deletedAt > $timestamp)
			RETURN DISTINCT cs.uid as uid, cs.kind as kind, cs.apiGroup as apiGroup, cs.namespace as namespace,
			       cs.name as name, cs.labels as labels, cs.firstSeen as firstSeen, cs.lastSeen as lastSeen,
			       cs.deleted as deleted, cs.deletedAt as deletedAt
			LIMIT 100
		`, maxDepth)
	}

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Timeout: queryTimeoutMs,
		Query:   query,
		Parameters: map[string]interface{}{
			"uids":      namespacedUIDs,
			"timestamp": timestamp,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch cluster-scoped resources: %w", err)
	}

	return parseResourceResults(result), nil
}

func (s *Store) fetchRelationships(ctx context.Context, resourceUIDs []string) ([]edgeResult, error) {
	if len(resourceUIDs) == 0 {
		return nil, nil
	}

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Timeout: queryTimeoutMs,
		Query: `
			MATCH (r:ResourceIdentity)-[rel]->(target:ResourceIdentity)
			WHERE r.uid IN $uids
			  AND target.uid IN $uids
			  AND NOT type(rel) IN ['CHANGED', 'EMITTED_EVENT']
			RETURN DISTINCT r.uid as source, target.uid as target, type(rel) as relType, id(rel) as edgeId
		`,
		Parameters: map[string]interface{}{
			"uids": resourceUIDs,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch relationships: %w", err)
	}

	edges := make([]edgeResult, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) < 4 {
			continue
		}

		edge := edgeResult{}
		if source, ok := row[0].(string); ok {
			edge.SourceUID = source
		}
		if target, ok := row[1].(string); ok {
			edge.TargetUID = target
		}
		if relType, ok := row[2].(string); ok {
			edge.RelationshipType = relType
		}
		switch edgeID := row[3].(type) {
		case int64:
			edge.EdgeID = fmt.Sprintf("%d", edgeID)
		case float64:
			edge.EdgeID = fmt.Sprintf("%.0f", edgeID)
		case string:
			edge.EdgeID = edgeID
		}

		if edge.SourceUID != "" && edge.TargetUID != "" && edge.RelationshipType != "" {
			edges = append(edges, edge)
		}
	}

	return edges, nil
}
