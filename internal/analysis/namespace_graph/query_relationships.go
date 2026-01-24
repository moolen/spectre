package namespacegraph

import (
	"context"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// RelationshipFetcher handles querying relationships from the graph database
type RelationshipFetcher struct {
	graphClient graph.Client
	logger      *logging.Logger
}

// NewRelationshipFetcher creates a new RelationshipFetcher
func NewRelationshipFetcher(graphClient graph.Client) *RelationshipFetcher {
	return &RelationshipFetcher{
		graphClient: graphClient,
		logger:      logging.GetLogger("namespacegraph.relationships"),
	}
}

// edgeResult holds the raw data from a relationship query
type edgeResult struct {
	SourceUID        string
	TargetUID        string
	RelationshipType string
	EdgeID           string
}

// FetchRelationships fetches all relationships between the given resources
func (f *RelationshipFetcher) FetchRelationships(
	ctx context.Context,
	resourceUIDs []string,
) ([]edgeResult, error) {
	if len(resourceUIDs) == 0 {
		return nil, nil
	}

	// Query to find all relationship edges between the given resources
	// Excludes structural edges (CHANGED, EMITTED_EVENT) that connect to event nodes
	// Only includes edges where both source and target are ResourceIdentity nodes in our set
	//
	// Optimized: Match relationships directly without UNWIND to avoid O(n²) complexity.
	// This query finds all edges where both endpoints are in the UID set in a single pass.
	cypherQuery := `
		MATCH (r:ResourceIdentity)-[rel]->(target:ResourceIdentity)
		WHERE r.uid IN $uids
		  AND target.uid IN $uids
		  AND NOT type(rel) IN ['CHANGED', 'EMITTED_EVENT']
		RETURN DISTINCT r.uid as source, target.uid as target, type(rel) as relType, id(rel) as edgeId
	`

	query := graph.GraphQuery{
		Timeout: QueryTimeoutMs,
		Query:   cypherQuery,
		Parameters: map[string]interface{}{
			"uids": resourceUIDs,
		},
	}

	result, err := f.graphClient.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch relationships: %w", err)
	}

	f.logger.Debug("Fetched %d relationship edges for %d resources", len(result.Rows), len(resourceUIDs))

	return parseEdgeResults(result), nil
}

// parseEdgeResults parses the query result into edgeResult structs
func parseEdgeResults(result *graph.QueryResult) []edgeResult {
	edges := make([]edgeResult, 0, len(result.Rows))

	for _, row := range result.Rows {
		if len(row) < 4 {
			continue
		}

		e := edgeResult{}

		if source, ok := row[0].(string); ok {
			e.SourceUID = source
		}
		if target, ok := row[1].(string); ok {
			e.TargetUID = target
		}
		if relType, ok := row[2].(string); ok {
			e.RelationshipType = relType
		}
		switch edgeID := row[3].(type) {
		case int64:
			e.EdgeID = fmt.Sprintf("%d", edgeID)
		case float64:
			e.EdgeID = fmt.Sprintf("%.0f", edgeID)
		case string:
			e.EdgeID = edgeID
		}

		// Only include edges with valid source and target
		if e.SourceUID != "" && e.TargetUID != "" && e.RelationshipType != "" {
			edges = append(edges, e)
		}
	}

	return edges
}
