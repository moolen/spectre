package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// EdgeRevalidator periodically revalidates inferred edges to ensure they remain valid
type EdgeRevalidator struct {
	client   graph.Client
	interval time.Duration
	logger   *logging.Logger

	// Configuration
	maxAge           time.Duration // Maximum age before revalidation required
	staleThreshold   time.Duration // Age after which edges are marked as stale
	decayEnabled     bool
	decayInterval6h  time.Duration
	decayInterval24h time.Duration
	decayFactor6h    float64
	decayFactor24h   float64
}

// Config holds configuration for the EdgeRevalidator
type Config struct {
	// Interval between revalidation runs
	Interval time.Duration

	// MaxAge is the maximum time since last validation before an edge needs revalidation
	MaxAge time.Duration

	// StaleThreshold is the age after which edges are marked as stale
	StaleThreshold time.Duration

	// DecayEnabled controls whether confidence decay is applied
	DecayEnabled bool

	// Decay settings
	DecayInterval6h  time.Duration
	DecayInterval24h time.Duration
	DecayFactor6h    float64
	DecayFactor24h   float64
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		Interval:         5 * time.Minute,    // Run every 5 minutes
		MaxAge:           1 * time.Hour,      // Revalidate after 1 hour
		StaleThreshold:   7 * 24 * time.Hour, // Mark as stale after 7 days
		DecayEnabled:     true,
		DecayInterval6h:  6 * time.Hour,
		DecayInterval24h: 24 * time.Hour,
		DecayFactor6h:    0.9, // 10% decay
		DecayFactor24h:   0.7, // 30% decay
	}
}

// NewEdgeRevalidator creates a new edge revalidator
func NewEdgeRevalidator(client graph.Client, config Config) *EdgeRevalidator {
	return &EdgeRevalidator{
		client:           client,
		interval:         config.Interval,
		maxAge:           config.MaxAge,
		staleThreshold:   config.StaleThreshold,
		decayEnabled:     config.DecayEnabled,
		decayInterval6h:  config.DecayInterval6h,
		decayInterval24h: config.DecayInterval24h,
		decayFactor6h:    config.DecayFactor6h,
		decayFactor24h:   config.DecayFactor24h,
		logger:           logging.GetLogger("graph.validation"),
	}
}

// Run starts the revalidation background job
func (r *EdgeRevalidator) Run(ctx context.Context) error {
	r.logger.Info("Starting edge revalidator with interval %v", r.interval)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// Run immediately on start
	if err := r.revalidateEdges(ctx); err != nil {
		r.logger.Warn("Initial revalidation failed: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := r.revalidateEdges(ctx); err != nil {
				r.logger.Warn("Revalidation failed: %v", err)
			}
		case <-ctx.Done():
			r.logger.Info("Edge revalidator stopped")
			return ctx.Err()
		}
	}
}

// revalidateEdges performs a revalidation cycle
func (r *EdgeRevalidator) revalidateEdges(ctx context.Context) error {
	now := time.Now().UnixNano()
	maxAgeNs := r.maxAge.Nanoseconds()
	staleThresholdNs := r.staleThreshold.Nanoseconds()

	r.logger.Debug("Starting revalidation cycle")

	// Query edges that need revalidation
	// Focus on MANAGES and CREATES_OBSERVED edges with evidence
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

		// Parse edge properties
		edgeProps, err := r.parseEdgeProperties(edgeData)
		if err != nil {
			stats.ErrorCount++
			continue
		}

		stats.TotalEdges++

		// Check if edge needs revalidation
		age := now - edgeProps.LastValidated

		// Apply confidence decay if enabled
		if r.decayEnabled {
			decayed, newConfidence := r.applyConfidenceDecay(edgeProps, age)
			if decayed {
				edgeProps.Confidence = newConfidence
				stats.DecayedEdges++
			}
		}

		// Mark as stale if too old
		if age > staleThresholdNs {
			if edgeProps.ValidationState != graph.ValidationStateStale {
				edgeProps.ValidationState = graph.ValidationStateStale
				stats.StaleEdges++
			}
		} else if age > maxAgeNs {
			// Revalidate edge
			valid := r.validateEdge(ctx, sourceNode, targetNode, edgeProps)

			edgeProps.LastValidated = now

			if valid {
				edgeProps.ValidationState = graph.ValidationStateValid
				stats.RevalidatedEdges++
			} else {
				edgeProps.ValidationState = graph.ValidationStateInvalid
				stats.InvalidatedEdges++
			}
		}

		// Update edge if changed
		if stats.DecayedEdges > 0 || stats.StaleEdges > 0 || stats.RevalidatedEdges > 0 || stats.InvalidatedEdges > 0 {
			if err := r.updateEdge(ctx, edgeData, edgeProps); err != nil {
				r.logger.Warn("Failed to update edge: %v", err)
				stats.ErrorCount++
			} else {
				stats.UpdatedEdges++
			}
		}
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
	newConfidence := originalConfidence

	// Don't decay edges with 100% confidence (explicit relationships)
	if originalConfidence >= 1.0 {
		return false, originalConfidence
	}

	// Apply 24-hour decay
	if ageNs > r.decayInterval24h.Nanoseconds() {
		newConfidence = originalConfidence * r.decayFactor24h
	} else if ageNs > r.decayInterval6h.Nanoseconds() {
		// Apply 6-hour decay
		newConfidence = originalConfidence * r.decayFactor6h
	} else {
		return false, originalConfidence
	}

	// Minimum confidence threshold
	if newConfidence < 0.1 {
		newConfidence = 0.1
	}

	return newConfidence != originalConfidence, newConfidence
}

// validateEdge checks if an edge is still valid by verifying:
// 1. Both source and target nodes exist and aren't deleted
// 2. All evidence items are still valid (strict mode - all evidence must pass)
func (r *EdgeRevalidator) validateEdge(
	ctx context.Context,
	source, target map[string]interface{},
	edge *graph.ManagesEdge,
) bool {
	// Check if nodes exist and aren't deleted
	sourceDeleted, _ := source["deleted"].(bool)
	targetDeleted, _ := target["deleted"].(bool)

	if sourceDeleted || targetDeleted {
		r.logger.Debug("Edge invalid: source or target deleted (sourceDeleted=%v, targetDeleted=%v)",
			sourceDeleted, targetDeleted)
		return false
	}

	// Strict evidence validation: all evidence must still be valid
	for i, evidence := range edge.Evidence {
		if !r.validateEvidenceItem(ctx, source, target, evidence) {
			r.logger.Debug("Edge invalid: evidence item %d failed validation (type=%s, value=%s)",
				i, evidence.Type, evidence.Value)
			return false
		}
	}

	return true
}

// validateEvidenceItem checks if a specific piece of evidence is still valid
func (r *EdgeRevalidator) validateEvidenceItem(
	ctx context.Context,
	source, target map[string]interface{},
	evidence graph.EvidenceItem,
) bool {
	switch evidence.Type {
	case graph.EvidenceTypeLabel:
		return r.validateLabelEvidence(target, evidence)
	case graph.EvidenceTypeAnnotation:
		return r.validateAnnotationEvidence(target, evidence)
	case graph.EvidenceTypeOwnership:
		return r.validateOwnershipEvidence(ctx, source, target, evidence)
	case graph.EvidenceTypeNamespace:
		return r.validateNamespaceEvidence(source, target, evidence)
	case graph.EvidenceTypeTemporal:
		// Temporal evidence is historical - was valid at creation time
		// No revalidation needed
		return true
	case graph.EvidenceTypeReconcile:
		// Reconcile correlation is historical - no revalidation needed
		return true
	default:
		// Unknown evidence types pass by default (forward compatibility)
		r.logger.Debug("Unknown evidence type %s, assuming valid", evidence.Type)
		return true
	}
}

// validateLabelEvidence checks if the label referenced in evidence still exists on the target
func (r *EdgeRevalidator) validateLabelEvidence(
	target map[string]interface{},
	evidence graph.EvidenceItem,
) bool {
	// If structured fields are available, use them
	if evidence.Key != "" {
		labels := r.parseLabelsFromNode(target)
		if labels == nil {
			r.logger.Debug("Label evidence invalid: no labels on target node")
			return false
		}

		actualValue, exists := labels[evidence.Key]
		if !exists {
			r.logger.Debug("Label evidence invalid: key %s not found on target", evidence.Key)
			return false
		}

		// If MatchValue is set, check exact match
		if evidence.MatchValue != "" && actualValue != evidence.MatchValue {
			r.logger.Debug("Label evidence invalid: key %s has value %s, expected %s",
				evidence.Key, actualValue, evidence.MatchValue)
			return false
		}

		return true
	}

	// Fallback: if no structured fields, assume valid (backward compatibility)
	// This handles old evidence items that only have the Value field
	r.logger.Debug("Label evidence has no structured key, assuming valid for backward compatibility")
	return true
}

// validateAnnotationEvidence checks if the annotation referenced in evidence still exists
func (r *EdgeRevalidator) validateAnnotationEvidence(
	target map[string]interface{},
	evidence graph.EvidenceItem,
) bool {
	// If structured fields are available, use them
	if evidence.Key != "" {
		// Note: In ResourceIdentity, annotations may be stored in labels field
		labels := r.parseLabelsFromNode(target)
		if labels == nil {
			r.logger.Debug("Annotation evidence invalid: no labels/annotations on target node")
			return false
		}

		actualValue, exists := labels[evidence.Key]
		if !exists {
			r.logger.Debug("Annotation evidence invalid: key %s not found on target", evidence.Key)
			return false
		}

		if evidence.MatchValue != "" && actualValue != evidence.MatchValue {
			r.logger.Debug("Annotation evidence invalid: key %s has value %s, expected %s",
				evidence.Key, actualValue, evidence.MatchValue)
			return false
		}

		return true
	}

	// Fallback: if no structured fields, assume valid (backward compatibility)
	r.logger.Debug("Annotation evidence has no structured key, assuming valid for backward compatibility")
	return true
}

// validateOwnershipEvidence checks if OWNS edge still exists between source and target
func (r *EdgeRevalidator) validateOwnershipEvidence(
	ctx context.Context,
	source, target map[string]interface{},
	evidence graph.EvidenceItem,
) bool {
	// Get UIDs from evidence or from node data
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

	// Query for OWNS edge
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
	}

	return false
}

// validateNamespaceEvidence checks if both resources are still in the expected namespace
func (r *EdgeRevalidator) validateNamespaceEvidence(
	source, target map[string]interface{},
	evidence graph.EvidenceItem,
) bool {
	targetNS, _ := target["namespace"].(string)

	// If structured field is available, verify against it
	if evidence.Namespace != "" {
		if targetNS != evidence.Namespace {
			r.logger.Debug("Namespace evidence invalid: target namespace is %s, expected %s",
				targetNS, evidence.Namespace)
			return false
		}
		return true
	}

	// Fallback: check source and target are in the same namespace
	sourceNS, _ := source["namespace"].(string)
	return sourceNS == targetNS
}

// parseLabelsFromNode extracts labels from a node's properties
func (r *EdgeRevalidator) parseLabelsFromNode(node map[string]interface{}) map[string]string {
	labelsJSON, ok := node["labels"].(string)
	if !ok || labelsJSON == "" {
		return nil
	}

	var labels map[string]string
	if err := json.Unmarshal([]byte(labelsJSON), &labels); err != nil {
		r.logger.Debug("Failed to parse labels JSON: %v", err)
		return nil
	}

	return labels
}

// updateEdge updates an edge in the graph
func (r *EdgeRevalidator) updateEdge(ctx context.Context, edgeData map[string]interface{}, props *graph.ManagesEdge) error {
	// Get edge identifiers
	fromUID, _ := edgeData["fromUID"].(string)
	toUID, _ := edgeData["toUID"].(string)
	edgeType, _ := edgeData["type"].(string)

	if fromUID == "" || toUID == "" || edgeType == "" {
		return fmt.Errorf("missing edge identifiers")
	}

	// Marshal updated properties
	propsJSON, err := json.Marshal(props)
	if err != nil {
		return fmt.Errorf("failed to marshal properties: %w", err)
	}

	// Update edge in graph
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

// logStats logs revalidation statistics
func (r *EdgeRevalidator) logStats(stats *RevalidationStats) {
	duration := stats.EndTime.Sub(stats.StartTime)

	r.logger.Info(
		"Revalidation complete: total=%d, revalidated=%d, invalidated=%d, decayed=%d, stale=%d, updated=%d, errors=%d, duration=%v",
		stats.TotalEdges,
		stats.RevalidatedEdges,
		stats.InvalidatedEdges,
		stats.DecayedEdges,
		stats.StaleEdges,
		stats.UpdatedEdges,
		stats.ErrorCount,
		duration,
	)
}

// RevalidationStats tracks statistics for a revalidation cycle
type RevalidationStats struct {
	StartTime        time.Time
	EndTime          time.Time
	TotalEdges       int
	RevalidatedEdges int
	InvalidatedEdges int
	DecayedEdges     int
	StaleEdges       int
	UpdatedEdges     int
	ErrorCount       int
}

// GetStats returns the current revalidation statistics
func (r *EdgeRevalidator) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"interval":       r.interval.String(),
		"maxAge":         r.maxAge.String(),
		"staleThreshold": r.staleThreshold.String(),
		"decayEnabled":   r.decayEnabled,
		"decayFactor6h":  r.decayFactor6h,
		"decayFactor24h": r.decayFactor24h,
	}
}
