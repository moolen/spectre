package extractors

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
)

// SecretRelationshipScorerConfig configures the Secret relationship scoring algorithm.
// Different resource types (Certificate, ExternalSecret, etc.) use different evidence
// weights and patterns to determine if they created a Secret.
type SecretRelationshipScorerConfig struct {
	// Evidence weights (0.0 = disabled, weights can sum beyond 1.0 but score is capped)
	OwnerReferenceWeight  float64 // Weight for ownerReference evidence (typically 1.0 = perfect match)
	NameMatchWeight       float64 // Weight for exact Secret name match
	AnnotationMatchWeight float64 // Weight for annotation match (e.g., cert-manager.io/certificate-name)
	TemporalWeight        float64 // Weight for temporal proximity (Secret created near source reconcile)
	LabelMatchWeight      float64 // Weight for label match (e.g., external-secrets.io/name)
	NamespaceWeight       float64 // Weight for namespace match

	// Temporal proximity configuration
	TemporalWindowMs int64 // Max milliseconds between source event and Secret observation for correlation

	// Feature flags for evidence checking
	CheckOwnerReferences bool // Whether to check ownerReferences (currently stub - needs full resource data)
	CheckAnnotations     bool // Whether to check annotations
	CheckLabels          bool // Whether to check labels
	CheckReadyCondition  bool // Whether to verify source resource has Ready=True condition

	// Resource-specific keys
	AnnotationKey string   // Annotation key to check (e.g., "cert-manager.io/certificate-name")
	LabelKey      string   // Label key to check (e.g., "external-secrets.io/name")
	NamePatterns  []string // Name patterns to match (e.g., ["%s-tls"] for Certificate)
}

// SecretRelationshipScorer calculates confidence scores for CREATES_OBSERVED relationships
// between resources (Certificate, ExternalSecret) and the Secrets they create.
//
// Scoring algorithm:
// 1. Check for perfect OwnerReference match → 1.0 confidence (short-circuit)
// 2. Fall back to accumulating heuristic evidence:
//   - Name match (exact or pattern-based)
//   - Annotation/Label match
//   - Temporal proximity (Secret created shortly after source reconcile)
//   - Namespace match
type SecretRelationshipScorer struct {
	config SecretRelationshipScorerConfig
	lookup ResourceLookup
	logger func(format string, args ...interface{}) // Logger function for debug output
}

// NewSecretRelationshipScorer creates a new SecretRelationshipScorer with the given configuration.
func NewSecretRelationshipScorer(
	config SecretRelationshipScorerConfig,
	lookup ResourceLookup,
	logger func(format string, args ...interface{}),
) *SecretRelationshipScorer {
	return &SecretRelationshipScorer{
		config: config,
		lookup: lookup,
		logger: logger,
	}
}

// ScoreRelationship calculates the confidence score for a CREATES_OBSERVED relationship.
//
// Parameters:
//   - ctx: Context for cancellation
//   - sourceEvent: The source resource event (Certificate, ExternalSecret, etc.)
//   - sourceData: Parsed source resource data (for checking Ready condition)
//   - secret: The candidate Secret resource
//   - targetSecretName: Expected Secret name from source's spec
//
// Returns:
//   - confidence: Score from 0.0 to 1.0 (1.0 = perfect match)
//   - evidence: List of evidence items explaining the score
func (s *SecretRelationshipScorer) ScoreRelationship(
	ctx context.Context,
	sourceEvent models.Event,
	sourceData map[string]interface{},
	secret *graph.ResourceIdentity,
	targetSecretName string,
) (float64, []graph.EvidenceItem) {
	evidence := []graph.EvidenceItem{}

	// STAGE 1: Check for perfect OwnerReference match (100% confidence)
	if s.config.CheckOwnerReferences {
		if s.checkOwnerReference(ctx, secret, sourceEvent.Resource.UID) {
			evidence = append(evidence, graph.EvidenceItem{
				Type:      graph.EvidenceTypeOwnership,
				Value:     "Secret has ownerReference to source resource",
				Weight:    1.0,
				Timestamp: time.Now().UnixNano(),
				SourceUID: sourceEvent.Resource.UID,
				TargetUID: secret.UID,
			})
			s.logger("Found perfect OwnerReference match - 100%% confidence: secretUID=%s", secret.UID)
			return 1.0, evidence
		}
	}

	// STAGE 2: Accumulate heuristic evidence
	totalScore := 0.0

	// Evidence 1: Exact name match
	if s.config.NameMatchWeight > 0 && secret.Name == targetSecretName {
		totalScore += s.config.NameMatchWeight
		evidence = append(evidence, graph.EvidenceItem{
			Type:       graph.EvidenceTypeLabel,
			Value:      fmt.Sprintf("Secret name matches: %s", targetSecretName),
			Weight:     s.config.NameMatchWeight,
			Timestamp:  time.Now().UnixNano(),
			Key:        "name",
			MatchValue: targetSecretName,
		})
	}

	// Evidence 2: Pattern-based name match
	// Example: Certificate "my-cert" creates Secret "my-cert-tls" via pattern "%s-tls"
	if s.config.NameMatchWeight > 0 && totalScore == 0 && len(s.config.NamePatterns) > 0 {
		for _, pattern := range s.config.NamePatterns {
			expectedName := fmt.Sprintf(pattern, sourceEvent.Resource.Name)
			if secret.Name == expectedName {
				totalScore += s.config.NameMatchWeight
				evidence = append(evidence, graph.EvidenceItem{
					Type:       graph.EvidenceTypeLabel,
					Value:      fmt.Sprintf("Secret name matches pattern %s: %s", pattern, expectedName),
					Weight:     s.config.NameMatchWeight,
					Timestamp:  time.Now().UnixNano(),
					Key:        "name",
					MatchValue: expectedName,
				})
				break
			}
		}
	}

	// Evidence 3: Annotation match
	if s.config.CheckAnnotations && s.config.AnnotationMatchWeight > 0 && s.config.AnnotationKey != "" {
		if secret.Labels != nil {
			// Note: Some systems store annotations in labels field in graph
			if annotationValue, ok := secret.Labels[s.config.AnnotationKey]; ok && annotationValue == sourceEvent.Resource.Name {
				totalScore += s.config.AnnotationMatchWeight
				evidence = append(evidence, graph.EvidenceItem{
					Type:       graph.EvidenceTypeAnnotation,
					Value:      fmt.Sprintf("Annotation %s=%s", s.config.AnnotationKey, annotationValue),
					Weight:     s.config.AnnotationMatchWeight,
					Timestamp:  time.Now().UnixNano(),
					Key:        s.config.AnnotationKey,
					MatchValue: annotationValue,
				})
			}
		}
	}

	// Evidence 4: Label match
	if s.config.CheckLabels && s.config.LabelMatchWeight > 0 && s.config.LabelKey != "" {
		if secret.Labels != nil {
			if labelValue, ok := secret.Labels[s.config.LabelKey]; ok && labelValue == sourceEvent.Resource.Name {
				totalScore += s.config.LabelMatchWeight
				evidence = append(evidence, graph.EvidenceItem{
					Type:       graph.EvidenceTypeLabel,
					Value:      fmt.Sprintf("Label %s=%s", s.config.LabelKey, labelValue),
					Weight:     s.config.LabelMatchWeight,
					Timestamp:  time.Now().UnixNano(),
					Key:        s.config.LabelKey,
					MatchValue: labelValue,
				})
			}
		}
	}

	// Evidence 5: Temporal proximity
	if s.config.TemporalWeight > 0 && s.config.TemporalWindowMs > 0 {
		// If CheckReadyCondition is enabled, only check temporal proximity if source is Ready
		shouldCheckTemporal := true
		if s.config.CheckReadyCondition {
			shouldCheckTemporal = HasReadyCondition(sourceData)
			if !shouldCheckTemporal {
				s.logger("Source resource not Ready, skipping temporal check")
			}
		}

		if shouldCheckTemporal {
			lagMs := (secret.LastSeen - sourceEvent.Timestamp) / 1_000_000

			// Allow negative lag if CheckReadyCondition is true (Secret might be observed before event processed)
			minLag := int64(0)
			if s.config.CheckReadyCondition {
				minLag = -s.config.TemporalWindowMs
			}

			if lagMs >= minLag && lagMs <= s.config.TemporalWindowMs {
				proximityScore := s.calculateTemporalProximity(lagMs, s.config.TemporalWindowMs)
				weight := s.config.TemporalWeight * proximityScore
				totalScore += weight

				evidence = append(evidence, graph.EvidenceItem{
					Type:      graph.EvidenceTypeTemporal,
					Value:     fmt.Sprintf("Secret observed within %dms of source event", AbsInt64(lagMs)),
					Weight:    weight,
					Timestamp: time.Now().UnixNano(),
					LagMs:     lagMs,
					WindowMs:  s.config.TemporalWindowMs,
				})
			}
		}
	}

	// Evidence 6: Namespace match
	if s.config.NamespaceWeight > 0 && secret.Namespace == sourceEvent.Resource.Namespace {
		totalScore += s.config.NamespaceWeight
		evidence = append(evidence, graph.EvidenceItem{
			Type:      graph.EvidenceTypeNamespace,
			Value:     secret.Namespace,
			Weight:    s.config.NamespaceWeight,
			Timestamp: time.Now().UnixNano(),
			Namespace: secret.Namespace,
		})
	}

	// Cap confidence at 1.0
	confidence := totalScore
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence, evidence
}

// checkOwnerReference checks if the secret has an ownerReference to the source resource
// by querying the existing OWNS edges in the graph. When resources are processed,
// ownerReferences are converted to OWNS edges, so we can check if one exists.
func (s *SecretRelationshipScorer) checkOwnerReference(
	ctx context.Context,
	secret *graph.ResourceIdentity,
	sourceUID string,
) bool {
	if s.lookup == nil {
		return false
	}

	// Query for existing OWNS edge from source to secret
	query := graph.GraphQuery{
		Query: `
			MATCH (owner:ResourceIdentity {uid: $ownerUID})-[:OWNS]->(owned:ResourceIdentity {uid: $secretUID})
			RETURN count(*) > 0 as hasOwnership
		`,
		Parameters: map[string]interface{}{
			"ownerUID":  sourceUID,
			"secretUID": secret.UID,
		},
	}

	result, err := s.lookup.QueryGraph(ctx, query)
	if err != nil {
		s.logger("Failed to check ownerReference via OWNS edge: %v", err)
		return false
	}

	if result == nil || len(result.Rows) == 0 || len(result.Rows[0]) == 0 {
		return false
	}

	// FalkorDB returns boolean results in various formats
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

// calculateTemporalProximity calculates a proximity score based on time difference.
// Returns 1.0 for immediate proximity (lagMs=0), decreasing linearly to 0.0 at window boundary.
func (s *SecretRelationshipScorer) calculateTemporalProximity(lagMs, windowMs int64) float64 {
	if windowMs == 0 {
		return 0.0
	}
	absLag := AbsInt64(lagMs)
	if absLag > windowMs {
		return 0.0
	}
	return 1.0 - (float64(absLag) / float64(windowMs))
}

// CreateCertificateSecretScorerConfig creates a configuration for Certificate→Secret scoring.
// This mirrors the weights and configuration from the original certificate.go implementation.
func CreateCertificateSecretScorerConfig() SecretRelationshipScorerConfig {
	return SecretRelationshipScorerConfig{
		// Evidence weights (from original certificate.go scoreSecretRelationship)
		OwnerReferenceWeight:  1.0, // Perfect match
		NameMatchWeight:       0.5, // Name match (direct or pattern)
		AnnotationMatchWeight: 0.9, // cert-manager.io/certificate-name
		TemporalWeight:        0.8, // Within 60 seconds
		LabelMatchWeight:      0.0, // Not used by Certificate
		NamespaceWeight:       0.3, // Same namespace

		// Configuration
		TemporalWindowMs:     60000, // 60 seconds
		CheckOwnerReferences: true,  // Stub for now
		CheckAnnotations:     true,  // Check cert-manager annotation
		CheckLabels:          false, // Not used
		CheckReadyCondition:  true,  // Check Certificate is Ready

		// Keys
		AnnotationKey: "cert-manager.io/certificate-name",
		LabelKey:      "",

		// Name patterns (common: "my-cert" → "my-cert-tls")
		NamePatterns: []string{"%s-tls"},
	}
}

// CreateExternalSecretScorerConfig creates a configuration for ExternalSecret→Secret scoring.
// This mirrors the weights and configuration from the original externalsecret.go implementation.
func CreateExternalSecretScorerConfig() SecretRelationshipScorerConfig {
	return SecretRelationshipScorerConfig{
		// Evidence weights (from original externalsecret.go scoreSecretRelationship)
		OwnerReferenceWeight:  1.0, // Perfect match
		NameMatchWeight:       0.9, // spec.target.name
		AnnotationMatchWeight: 0.0, // Not used by ExternalSecret
		TemporalWeight:        0.7, // Within 2 minutes
		LabelMatchWeight:      0.5, // external-secrets.io/name
		NamespaceWeight:       0.3, // Same namespace

		// Configuration
		TemporalWindowMs:     120000, // 2 minutes (120 seconds)
		CheckOwnerReferences: true,   // Stub for now
		CheckAnnotations:     false,  // Not used
		CheckLabels:          true,   // Check external-secrets label
		CheckReadyCondition:  true,   // Check ExternalSecret is Ready

		// Keys
		AnnotationKey: "",
		LabelKey:      "external-secrets.io/name",

		// Name patterns (exact match only, no patterns)
		NamePatterns: nil,
	}
}

// ScorerConfigDebugString returns a human-readable summary of scorer configuration.
// Useful for logging and debugging scoring decisions.
func ScorerConfigDebugString(config SecretRelationshipScorerConfig) string {
	var parts []string

	if config.OwnerReferenceWeight > 0 {
		parts = append(parts, fmt.Sprintf("OwnerRef=%.1f", config.OwnerReferenceWeight))
	}
	if config.NameMatchWeight > 0 {
		parts = append(parts, fmt.Sprintf("Name=%.1f", config.NameMatchWeight))
	}
	if config.AnnotationMatchWeight > 0 {
		parts = append(parts, fmt.Sprintf("Annotation=%.1f", config.AnnotationMatchWeight))
	}
	if config.TemporalWeight > 0 {
		parts = append(parts, fmt.Sprintf("Temporal=%.1f(%dms)", config.TemporalWeight, config.TemporalWindowMs))
	}
	if config.LabelMatchWeight > 0 {
		parts = append(parts, fmt.Sprintf("Label=%.1f", config.LabelMatchWeight))
	}
	if config.NamespaceWeight > 0 {
		parts = append(parts, fmt.Sprintf("Namespace=%.1f", config.NamespaceWeight))
	}

	return strings.Join(parts, ", ")
}
