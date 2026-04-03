package validation

import (
	"context"
	"encoding/json"

	"github.com/moolen/spectre/internal/graph"
)

// validateEdge checks if an edge is still valid by verifying:
// 1. Both source and target nodes exist and aren't deleted
// 2. All evidence items are still valid (strict mode - all evidence must pass)
func (r *EdgeRevalidator) validateEdge(
	ctx context.Context,
	source, target map[string]interface{},
	edge *graph.ManagesEdge,
) bool {
	sourceDeleted, _ := source["deleted"].(bool)
	targetDeleted, _ := target["deleted"].(bool)
	if sourceDeleted || targetDeleted {
		r.logger.Debug("Edge invalid: source or target deleted (sourceDeleted=%v, targetDeleted=%v)",
			sourceDeleted, targetDeleted)
		return false
	}

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
	case graph.EvidenceTypeTemporal, graph.EvidenceTypeReconcile:
		return true
	default:
		r.logger.Debug("Unknown evidence type %s, assuming valid", evidence.Type)
		return true
	}
}

// validateLabelEvidence checks if the label referenced in evidence still exists on the target
func (r *EdgeRevalidator) validateLabelEvidence(
	target map[string]interface{},
	evidence graph.EvidenceItem,
) bool {
	if evidence.Key == "" {
		r.logger.Debug("Label evidence has no structured key, assuming valid for backward compatibility")
		return true
	}

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

	if evidence.MatchValue != "" && actualValue != evidence.MatchValue {
		r.logger.Debug("Label evidence invalid: key %s has value %s, expected %s",
			evidence.Key, actualValue, evidence.MatchValue)
		return false
	}

	return true
}

// validateAnnotationEvidence checks if the annotation referenced in evidence still exists
func (r *EdgeRevalidator) validateAnnotationEvidence(
	target map[string]interface{},
	evidence graph.EvidenceItem,
) bool {
	if evidence.Key == "" {
		r.logger.Debug("Annotation evidence has no structured key, assuming valid for backward compatibility")
		return true
	}

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

// validateNamespaceEvidence checks if both resources are still in the expected namespace
func (r *EdgeRevalidator) validateNamespaceEvidence(
	source, target map[string]interface{},
	evidence graph.EvidenceItem,
) bool {
	targetNS, _ := target["namespace"].(string)
	if evidence.Namespace != "" {
		if targetNS != evidence.Namespace {
			r.logger.Debug("Namespace evidence invalid: target namespace is %s, expected %s",
				targetNS, evidence.Namespace)
			return false
		}
		return true
	}

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
