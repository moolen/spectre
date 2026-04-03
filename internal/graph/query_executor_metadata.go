package graph

import (
	"context"
	"fmt"
)

// QueryDistinctMetadata queries for distinct namespaces and kinds in a time range.
func (qe *QueryExecutor) QueryDistinctMetadata(
	ctx context.Context,
	startTimeNs, endTimeNs int64,
) (namespaces, kinds []string, minTime, maxTime int64, err error) {
	graphQuery := GraphQuery{
		Query: `
		MATCH (r:ResourceIdentity)
		WHERE (NOT COALESCE(r.deleted, false) OR (r.deletedAt >= $startNs AND r.deletedAt <= $endNs))
		OPTIONAL MATCH (r)-[:CHANGED]->(e:ChangeEvent)
		WHERE e.timestamp >= $startNs AND e.timestamp <= $endNs
		WITH DISTINCT r.namespace as namespace, r.kind as kind, e.timestamp as timestamp
		RETURN collect(DISTINCT namespace) as namespaces,
		       collect(DISTINCT kind) as kinds,
		       min(timestamp) as minTime,
		       max(timestamp) as maxTime
	`,
		Parameters: map[string]interface{}{
			"startNs": startTimeNs,
			"endNs":   endTimeNs,
		},
		Timeout: 10000,
	}

	result, err := qe.client.ExecuteQuery(ctx, graphQuery)
	if err != nil {
		return nil, nil, 0, 0, fmt.Errorf("failed to execute metadata query: %w", err)
	}
	if len(result.Rows) == 0 {
		return []string{}, []string{}, 0, 0, nil
	}

	row := result.Rows[0]
	if len(row) < 4 {
		return nil, nil, 0, 0, fmt.Errorf("unexpected result format: got %d columns, expected 4", len(row))
	}

	if nsList, ok := row[0].([]interface{}); ok {
		namespaces = make([]string, 0, len(nsList))
		for _, ns := range nsList {
			if nsStr, ok := ns.(string); ok {
				namespaces = append(namespaces, nsStr)
			}
		}
	}

	if kindsList, ok := row[1].([]interface{}); ok {
		kinds = make([]string, 0, len(kindsList))
		for _, k := range kindsList {
			if kStr, ok := k.(string); ok {
				kinds = append(kinds, kStr)
			}
		}
	}

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
