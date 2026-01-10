package extractors

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
)

// ManagementScorerConfig configures the management relationship scoring algorithm.
// Different GitOps tools (Flux, ArgoCD, etc.) use different labels and scoring weights.
type ManagementScorerConfig struct {
	// LabelTemplates maps label purpose to label key template.
	// Templates support {{name}} and {{namespace}} placeholders.
	// Example: {"name": "helm.toolkit.fluxcd.io/name", "namespace": "helm.toolkit.fluxcd.io/namespace"}
	LabelTemplates map[string]string

	// Scoring weights for heuristic fallback (when perfect label match fails)
	NamePrefixWeight float64 // Weight for resource name prefix matching manager name
	NamespaceWeight  float64 // Weight for namespace match
	TemporalWeight   float64 // Weight for temporal proximity (resource created near manager reconcile)
	ReconcileWeight  float64 // Weight for reconcile event correlation (only used if CheckReconcileEvents=true)

	// Temporal proximity configuration
	TemporalWindowMs int64 // Max milliseconds between manager event and resource creation for correlation

	// Optional reconcile event checking
	CheckReconcileEvents bool // Whether to check for recent reconcile events in the manager
}

// ManagementScorer calculates confidence scores for MANAGES relationships between
// GitOps resources (HelmRelease, Kustomization, ArgoCD Application) and managed resources.
//
// Scoring algorithm:
// 1. Check for perfect label match (both name and namespace labels) → 1.0 confidence
// 2. Fall back to heuristic scoring based on:
//   - Name prefix match (e.g., "frontend" HelmRelease manages "frontend-deployment")
//   - Namespace match (resource in expected namespace)
//   - Temporal proximity (resource created shortly after manager reconcile)
//   - Reconcile events (optional, checks if manager recently reconciled)
type ManagementScorer struct {
	config ManagementScorerConfig
	lookup ResourceLookup
	logger func(format string, args ...interface{}) // Logger function for debug output
}

// NewManagementScorer creates a new ManagementScorer with the given configuration.
func NewManagementScorer(
	config ManagementScorerConfig,
	lookup ResourceLookup,
	logger func(format string, args ...interface{}),
) *ManagementScorer {
	return &ManagementScorer{
		config: config,
		lookup: lookup,
		logger: logger,
	}
}

// ScoreRelationship calculates the confidence score for a MANAGES relationship.
//
// Parameters:
//   - ctx: Context for cancellation
//   - managerEvent: The manager resource event (HelmRelease, Kustomization, Application)
//   - candidateUID: UID of the candidate managed resource
//   - managerName: Name of the manager resource
//   - managerNamespace: Namespace of the manager resource
//   - targetNamespace: Namespace where resources are deployed (may differ from managerNamespace)
//
// Returns:
//   - confidence: Score from 0.0 to 1.0 (1.0 = perfect match)
//   - evidence: List of evidence items explaining the score
func (s *ManagementScorer) ScoreRelationship(
	ctx context.Context,
	managerEvent models.Event,
	candidateUID string,
	managerName string,
	managerNamespace string,
	targetNamespace string,
) (float64, []graph.EvidenceItem) {
	evidence := []graph.EvidenceItem{}

	// Get candidate resource details
	candidate, err := s.lookup.FindResourceByUID(ctx, candidateUID)
	if err != nil {
		s.logger("Failed to lookup candidate resource: %v", err)
		return 0.0, evidence
	}
	if candidate == nil {
		s.logger("Candidate resource not found: %s", candidateUID)
		return 0.0, evidence
	}

	// STAGE 1: Check for perfect label match (100% confidence)
	if s.checkPerfectLabelMatch(candidate, managerName, managerNamespace, &evidence) {
		return 1.0, evidence
	}

	// STAGE 2: Fall back to heuristic scoring
	totalWeight := 0.0
	earnedWeight := 0.0

	// Evidence 1: Name prefix match
	totalWeight += s.config.NamePrefixWeight
	if s.checkNamePrefixMatch(candidate, managerName) {
		earnedWeight += s.config.NamePrefixWeight
		evidence = append(evidence, graph.EvidenceItem{
			Type:       graph.EvidenceTypeLabel,
			Value:      fmt.Sprintf("name prefix matches: %s", managerName),
			Weight:     s.config.NamePrefixWeight,
			Timestamp:  time.Now().UnixNano(),
			Key:        "name",
			MatchValue: managerName,
		})
	}

	// Evidence 2: Namespace match
	totalWeight += s.config.NamespaceWeight
	effectiveNamespace := targetNamespace
	if effectiveNamespace == "" {
		effectiveNamespace = managerNamespace
	}
	if candidate.Namespace == targetNamespace || (targetNamespace == "" && candidate.Namespace == managerNamespace) {
		earnedWeight += s.config.NamespaceWeight
		evidence = append(evidence, graph.EvidenceItem{
			Type:      graph.EvidenceTypeNamespace,
			Value:     effectiveNamespace,
			Weight:    s.config.NamespaceWeight,
			Timestamp: time.Now().UnixNano(),
			Namespace: effectiveNamespace,
		})
	}

	// Evidence 3: Temporal proximity
	totalWeight += s.config.TemporalWeight
	lagMs := (candidate.FirstSeen - managerEvent.Timestamp) / 1_000_000
	if lagMs >= 0 && lagMs <= s.config.TemporalWindowMs {
		// Scale confidence based on proximity (closer = higher confidence)
		proximityScore := 1.0 - (float64(lagMs) / float64(s.config.TemporalWindowMs))
		weight := s.config.TemporalWeight * proximityScore
		earnedWeight += weight

		evidence = append(evidence, graph.EvidenceItem{
			Type:      graph.EvidenceTypeTemporal,
			Value:     fmt.Sprintf("created %dms after reconcile", lagMs),
			Weight:    weight,
			Timestamp: time.Now().UnixNano(),
			LagMs:     lagMs,
			WindowMs:  s.config.TemporalWindowMs,
		})
	}

	// Evidence 4: Reconcile event correlation (optional)
	if s.config.CheckReconcileEvents {
		totalWeight += s.config.ReconcileWeight
		windowNs := int64(60 * time.Second.Nanoseconds())
		managerEvents, err := s.lookup.FindRecentEvents(ctx, managerEvent.Resource.UID, windowNs)
		if err == nil && len(managerEvents) > 0 {
			// Check for reconcile success
			for _, evt := range managerEvents {
				if evt.Status == "Ready" || evt.EventType == "UPDATE" {
					earnedWeight += s.config.ReconcileWeight
					evidence = append(evidence, graph.EvidenceItem{
						Type:      graph.EvidenceTypeReconcile,
						Value:     fmt.Sprintf("Manager reconciled at %d", evt.Timestamp),
						Weight:    s.config.ReconcileWeight,
						Timestamp: evt.Timestamp,
					})
					break
				}
			}
		}
	}

	// Calculate final confidence
	confidence := 0.0
	if totalWeight > 0 {
		confidence = earnedWeight / totalWeight
	}

	return confidence, evidence
}

// checkPerfectLabelMatch checks if the candidate resource has all required labels
// matching the manager name and namespace. Returns true and adds evidence if perfect match.
// Supports configurations with only a name label (e.g., ArgoCD) or both name and namespace labels (e.g., Flux).
func (s *ManagementScorer) checkPerfectLabelMatch(
	candidate *graph.ResourceIdentity,
	managerName string,
	managerNamespace string,
	evidence *[]graph.EvidenceItem,
) bool {
	if candidate.Labels == nil {
		return false
	}

	// Get label templates
	nameLabelKey, hasNameLabelConfig := s.config.LabelTemplates["name"]
	namespaceLabelKey, hasNamespaceLabelConfig := s.config.LabelTemplates["namespace"]

	if !hasNameLabelConfig {
		return false // At minimum, must have name label configured
	}

	// Check name label
	nameValue, hasNameLabel := candidate.Labels[nameLabelKey]
	if !hasNameLabel || nameValue != managerName {
		return false // Name label doesn't match
	}

	// If namespace label is configured, check it too
	if hasNamespaceLabelConfig {
		namespaceValue, hasNamespaceLabel := candidate.Labels[namespaceLabelKey]
		if !hasNamespaceLabel || namespaceValue != managerNamespace {
			return false // Namespace label doesn't match
		}

		// Both labels match
		*evidence = append(*evidence, graph.EvidenceItem{
			Type:       graph.EvidenceTypeLabel,
			Value:      fmt.Sprintf("Labels match: %s=%s, %s=%s", nameLabelKey, managerName, namespaceLabelKey, managerNamespace),
			Weight:     1.0,
			Timestamp:  time.Now().UnixNano(),
			Key:        nameLabelKey,
			MatchValue: managerName,
		})
		s.logger("Found perfect label match (name+namespace) - 100%% confidence: candidateUID=%s", candidate.UID)
		return true
	}

	// Only name label is configured and it matches
	*evidence = append(*evidence, graph.EvidenceItem{
		Type:       graph.EvidenceTypeLabel,
		Value:      fmt.Sprintf("Label matches: %s=%s", nameLabelKey, managerName),
		Weight:     1.0,
		Timestamp:  time.Now().UnixNano(),
		Key:        nameLabelKey,
		MatchValue: managerName,
	})
	s.logger("Found perfect label match (name only) - 100%% confidence: candidateUID=%s", candidate.UID)
	return true
}

// checkNamePrefixMatch checks if the candidate resource name starts with the manager name.
// This is a common pattern where managed resources are named like:
//
//	Manager: "frontend" → Managed: "frontend", "frontend-deployment", "frontend-svc"
func (s *ManagementScorer) checkNamePrefixMatch(
	candidate *graph.ResourceIdentity,
	managerName string,
) bool {
	candidateName := strings.ToLower(candidate.Name)
	managerNameLower := strings.ToLower(managerName)

	return strings.HasPrefix(candidateName, managerNameLower)
}
