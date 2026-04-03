package graph

import (
	"fmt"

	"github.com/moolen/spectre/internal/models"
)

// buildTimelineQuery constructs a Cypher query for timeline data.
func (qe *QueryExecutor) buildTimelineQuery(
	startNs, endNs int64,
	filters models.QueryFilters,
	pagination *models.PaginationRequest,
) GraphQuery {
	whereConditions := []string{
		"(NOT COALESCE(r.deleted, false) OR (r.deletedAt >= $startNs AND r.deletedAt <= $endNs))",
	}

	params := map[string]interface{}{
		"startNs": startNs,
		"endNs":   endNs,
	}

	qe.logger.Debug("Timeline query filter: deleted resources outside window [%d, %d]", startNs, endNs)

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

	if pagination != nil && pagination.Cursor != "" {
		cursor, err := models.DecodeCursor(pagination.Cursor)
		if err == nil && cursor != nil {
			whereConditions = append(whereConditions, `(
				r.kind > $cursorKind OR
				(r.kind = $cursorKind AND r.namespace > $cursorNamespace) OR
				(r.kind = $cursorKind AND r.namespace = $cursorNamespace AND r.name > $cursorName)
			)`)
			params["cursorKind"] = cursor.Kind
			params["cursorNamespace"] = cursor.Namespace
			params["cursorName"] = cursor.Name
		}
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = "WHERE " + whereConditions[0]
		for i := 1; i < len(whereConditions); i++ {
			whereClause += " AND " + whereConditions[i]
		}
	}

	pageSize := models.DefaultPageSize
	if pagination != nil {
		pageSize = pagination.GetPageSize()
	}
	resourceLimit := (pageSize * 4) + 1
	params["limit"] = resourceLimit

	query := fmt.Sprintf(`
		MATCH (r:ResourceIdentity)
		%s
		OPTIONAL MATCH (r)-[:CHANGED]->(e:ChangeEvent)
		WHERE e.timestamp >= $startNs AND e.timestamp <= $endNs
		WITH r, collect(e) as inRangeEvents
		OPTIONAL MATCH (r)-[:CHANGED]->(prevCandidate:ChangeEvent)
		WHERE prevCandidate.timestamp < $startNs
		WITH r, inRangeEvents, max(prevCandidate.timestamp) as prevTimestamp
		OPTIONAL MATCH (r)-[:CHANGED]->(prev:ChangeEvent)
		WHERE prev.timestamp = prevTimestamp
		WITH r, inRangeEvents, prev
		ORDER BY prev.id
		WITH r, inRangeEvents, head(collect(prev)) as prev
		OPTIONAL MATCH (r)-[:EMITTED_EVENT]->(k:K8sEvent)
		WHERE k.timestamp >= $startNs AND k.timestamp <= $endNs
		WITH r, inRangeEvents, prev, collect(DISTINCT k) as k8sEvents
		WHERE size(inRangeEvents) > 0
		   OR (prev IS NOT NULL AND (NOT COALESCE(r.deleted, false) OR r.deletedAt >= $startNs))
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
		Timeout:    15000,
	}
}
