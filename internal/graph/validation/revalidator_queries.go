package validation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
)

// validateOwnershipEvidence checks if OWNS edge still exists between source and target
func (r *EdgeRevalidator) validateOwnershipEvidence(
	ctx context.Context,
	source, target map[string]interface{},
	evidence graph.EvidenceItem,
) bool {
	sourceUID := evidence.SourceUID
	targetUID := evidence.TargetUID
	if sourceUID == "" {
		sourceUID, _ = source["uid"].(string)
	}
	if targetUID == "" {
		targetUID, _ = target["uid"].(string)
	}

	if sourceUID == "" || targetUID == "" {
		r.logger.Debug("Ownership evidence invalid: missing UIDs")
		return false
	}

	query := graph.GraphQuery{
		Query: `
			MATCH (owner:ResourceIdentity {uid: $sourceUID})-[:OWNS]->(owned:ResourceIdentity {uid: $targetUID})
			RETURN count(*) > 0 as hasOwnership
		`,
		Parameters: map[string]interface{}{
			"sourceUID": sourceUID,
			"targetUID": targetUID,
		},
	}

	result, err := r.client.ExecuteQuery(ctx, query)
	if err != nil {
		r.logger.Warn("Failed to validate ownership evidence: %v", err)
		return false
	}

	if result == nil || len(result.Rows) == 0 || len(result.Rows[0]) == 0 {
		return false
	}

	switch v := result.Rows[0][0].(type) {
	case bool:
		return v
	case int64:
		return v > 0
	case float64:
		return v > 0
	default:
		return false
	}
}

// updateEdge updates an edge in the graph
func (r *EdgeRevalidator) updateEdge(ctx context.Context, edgeData map[string]interface{}, props *graph.ManagesEdge) error {
	fromUID, _ := edgeData["fromUID"].(string)
	toUID, _ := edgeData["toUID"].(string)
	edgeType, _ := edgeData["type"].(string)
	if fromUID == "" || toUID == "" || edgeType == "" {
		return fmt.Errorf("missing edge identifiers")
	}

	propsJSON, err := json.Marshal(props)
	if err != nil {
		return fmt.Errorf("failed to marshal properties: %w", err)
	}

	updateQuery := graph.GraphQuery{
		Query: `
			MATCH (source:ResourceIdentity {uid: $fromUID})
			MATCH (target:ResourceIdentity {uid: $toUID})
			MATCH (source)-[edge]->(target)
			WHERE type(edge) = $edgeType
			SET edge.properties = $properties
		`,
		Parameters: map[string]interface{}{
			"fromUID":    fromUID,
			"toUID":      toUID,
			"edgeType":   edgeType,
			"properties": string(propsJSON),
		},
	}

	_, err = r.client.ExecuteQuery(ctx, updateQuery)
	return err
}
