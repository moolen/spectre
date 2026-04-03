package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/graph"
)

// revalidateEdges performs a revalidation cycle
func (r *EdgeRevalidator) revalidateEdges(ctx context.Context) error {
	now := time.Now().UnixNano()
	maxAgeNs := r.maxAge.Nanoseconds()
	staleThresholdNs := r.staleThreshold.Nanoseconds()

	r.logger.Debug("Starting revalidation cycle")

	query := graph.GraphQuery{
		Query: `
			MATCH (source:ResourceIdentity)-[edge]->(target:ResourceIdentity)
			WHERE (type(edge) = 'MANAGES' OR type(edge) = 'CREATES_OBSERVED')
			  AND NOT source.deleted
			  AND NOT target.deleted
			RETURN source, edge, target
			LIMIT 1000
		`,
	}

	result, err := r.client.ExecuteQuery(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query edges: %w", err)
	}

	stats := &RevalidationStats{
		StartTime: time.Now(),
	}

	for _, row := range result.Rows {
		if len(row) < 3 {
			continue
		}

		sourceNode, ok := row[0].(map[string]interface{})
		if !ok {
			continue
		}

		edgeData, ok := row[1].(map[string]interface{})
		if !ok {
			continue
		}

		targetNode, ok := row[2].(map[string]interface{})
		if !ok {
			continue
		}

		edgeProps, err := r.parseEdgeProperties(edgeData)
		if err != nil {
			stats.ErrorCount++
			continue
		}

		stats.TotalEdges++
		edgeChanged := false
		age := now - edgeProps.LastValidated

		if r.decayEnabled {
			decayed, newConfidence := r.applyConfidenceDecay(edgeProps, age)
			if decayed {
				edgeProps.Confidence = newConfidence
				stats.DecayedEdges++
				edgeChanged = true
			}
		}

		if age > staleThresholdNs {
			if edgeProps.ValidationState != graph.ValidationStateStale {
				edgeProps.ValidationState = graph.ValidationStateStale
				stats.StaleEdges++
				edgeChanged = true
			}
		} else if age > maxAgeNs {
			valid := r.validateEdge(ctx, sourceNode, targetNode, edgeProps)
			edgeProps.LastValidated = now
			edgeChanged = true

			if valid {
				edgeProps.ValidationState = graph.ValidationStateValid
				stats.RevalidatedEdges++
			} else {
				edgeProps.ValidationState = graph.ValidationStateInvalid
				stats.InvalidatedEdges++
			}
		}

		if !edgeChanged {
			continue
		}

		if err := r.updateEdge(ctx, edgeData, edgeProps); err != nil {
			r.logger.Warn("Failed to update edge: %v", err)
			stats.ErrorCount++
			continue
		}
		stats.UpdatedEdges++
	}

	stats.EndTime = time.Now()
	r.logStats(stats)

	return nil
}

// parseEdgeProperties extracts edge properties from the edge data
func (r *EdgeRevalidator) parseEdgeProperties(edgeData map[string]interface{}) (*graph.ManagesEdge, error) {
	propsJSON, ok := edgeData["properties"].(string)
	if !ok {
		return nil, fmt.Errorf("missing properties field")
	}

	var props graph.ManagesEdge
	if err := json.Unmarshal([]byte(propsJSON), &props); err != nil {
		return nil, fmt.Errorf("failed to unmarshal properties: %w", err)
	}

	return &props, nil
}

// applyConfidenceDecay applies time-based confidence decay
func (r *EdgeRevalidator) applyConfidenceDecay(edge *graph.ManagesEdge, ageNs int64) (bool, float64) {
	originalConfidence := edge.Confidence
	if originalConfidence >= 1.0 {
		return false, originalConfidence
	}

	var newConfidence float64
	if ageNs > r.decayInterval24h.Nanoseconds() {
		newConfidence = originalConfidence * r.decayFactor24h
	} else if ageNs > r.decayInterval6h.Nanoseconds() {
		newConfidence = originalConfidence * r.decayFactor6h
	} else {
		return false, originalConfidence
	}

	if newConfidence < 0.1 {
		newConfidence = 0.1
	}

	return newConfidence != originalConfidence, newConfidence
}
