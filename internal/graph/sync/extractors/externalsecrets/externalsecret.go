package externalsecrets

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graph/sync/extractors"
	"github.com/moolen/spectre/internal/models"
)

const (
	externalSecretsGroup = "external-secrets.io"
	externalSecretKind   = "ExternalSecret"
	secretStoreKind      = "SecretStore"
	clusterSecretStoreKind = "ClusterSecretStore"
)

// ExternalSecretExtractor extracts ExternalSecret relationships:
// - ExternalSecret → SecretStore/ClusterSecretStore (REFERENCES_SPEC)
// - ExternalSecret → Secret (CREATES_OBSERVED with evidence)
type ExternalSecretExtractor struct {
	*extractors.BaseExtractor
}

// NewExternalSecretExtractor creates a new ExternalSecret extractor
func NewExternalSecretExtractor() *ExternalSecretExtractor {
	return &ExternalSecretExtractor{
		BaseExtractor: extractors.NewBaseExtractor("external-secret", 200),
	}
}

// Matches checks if this extractor applies to ExternalSecret resources
func (e *ExternalSecretExtractor) Matches(event models.Event) bool {
	return event.Resource.Kind == externalSecretKind &&
		event.Resource.Group == externalSecretsGroup
}

// ExtractRelationships extracts ExternalSecret relationships
func (e *ExternalSecretExtractor) ExtractRelationships(
	ctx context.Context,
	event models.Event,
	lookup extractors.ResourceLookup,
) ([]graph.Edge, error) {
	edges := []graph.Edge{}

	var externalSecret map[string]interface{}
	if err := json.Unmarshal(event.Data, &externalSecret); err != nil {
		return nil, fmt.Errorf("failed to parse ExternalSecret: %w", err)
	}

	spec, ok := extractors.GetNestedMap(externalSecret, "spec")
	if !ok {
		return edges, nil
	}

	// Extract secretStoreRef (REFERENCES_SPEC)
	if secretStoreRef, ok := extractors.GetNestedMap(spec, "secretStoreRef"); ok {
		if edge := e.extractSecretStoreRefEdge(ctx, event, secretStoreRef, lookup); edge != nil {
			edges = append(edges, *edge)
		}
	}

	// Extract Secret relationship (CREATES_OBSERVED)
	// Only if ExternalSecret is not being deleted
	if event.Type != models.EventTypeDelete {
		if secretEdge := e.extractSecretEdge(ctx, event, externalSecret, spec, lookup); secretEdge != nil {
			edges = append(edges, *secretEdge)
		}
	}

	return edges, nil
}

// extractSecretStoreRefEdge extracts ExternalSecret → SecretStore/ClusterSecretStore relationship
func (e *ExternalSecretExtractor) extractSecretStoreRefEdge(
	ctx context.Context,
	event models.Event,
	secretStoreRef map[string]interface{},
	lookup extractors.ResourceLookup,
) *graph.Edge {
	// Get store kind (defaults to SecretStore)
	kind := secretStoreKind
	if k, ok := extractors.GetNestedString(secretStoreRef, "kind"); ok && k != "" {
		kind = k
	}

	// Get store name
	name, ok := extractors.GetNestedString(secretStoreRef, "name")
	if !ok || name == "" {
		return nil
	}

	// Determine namespace
	// ClusterSecretStore is cluster-scoped, SecretStore is namespaced
	namespace := ""
	if kind == secretStoreKind {
		namespace = event.Resource.Namespace
	}

	// Look up the secret store
	targetResource, _ := lookup.FindResourceByNamespace(ctx, namespace, kind, name)
	targetUID := ""
	if targetResource != nil {
		targetUID = targetResource.UID
	}

	edge := e.CreateReferencesSpecEdge(
		event.Resource.UID,
		targetUID,
		"spec.secretStoreRef",
		kind,
		name,
		namespace,
	)
	return &edge
}

// extractSecretEdge extracts ExternalSecret → Secret relationship with evidence
func (e *ExternalSecretExtractor) extractSecretEdge(
	ctx context.Context,
	event models.Event,
	externalSecret map[string]interface{},
	spec map[string]interface{},
	lookup extractors.ResourceLookup,
) *graph.Edge {
	// Get target secret name from spec.target.name
	// If not specified, defaults to ExternalSecret name
	secretName := event.Resource.Name
	
	if target, ok := extractors.GetNestedMap(spec, "target"); ok {
		if name, ok := extractors.GetNestedString(target, "name"); ok && name != "" {
			secretName = name
		}
	}

	// Look up the secret
	secret, err := lookup.FindResourceByNamespace(ctx, event.Resource.Namespace, "Secret", secretName)
	if err != nil || secret == nil {
		// Secret doesn't exist yet
		return nil
	}

	// Score the relationship based on evidence
	confidence, evidence := e.scoreSecretRelationship(
		ctx,
		event,
		externalSecret,
		secret,
		secretName,
	)

	// Only create edge if confidence meets threshold
	if confidence < 0.5 {
		return nil
	}

	// Create CREATES_OBSERVED edge
	now := time.Now().UnixNano()
	props := graph.ManagesEdge{
		Confidence:      confidence,
		Evidence:        evidence,
		FirstObserved:   now,
		LastValidated:   now,
		ValidationState: graph.ValidationStateValid,
	}
	propsJSON, _ := json.Marshal(props)

	edge := graph.Edge{
		Type:       graph.EdgeTypeCreatesObserved,
		FromUID:    event.Resource.UID,
		ToUID:      secret.UID,
		Properties: propsJSON,
	}
	return &edge
}

// scoreSecretRelationship scores the ExternalSecret → Secret relationship
func (e *ExternalSecretExtractor) scoreSecretRelationship(
	ctx context.Context,
	esEvent models.Event,
	externalSecret map[string]interface{},
	secret *graph.ResourceIdentity,
	targetSecretName string,
) (float64, []graph.EvidenceItem) {
	evidence := []graph.EvidenceItem{}
	score := 0.0

	// Evidence 1: OwnerReference (100% confidence if present)
	if e.hasOwnerReference(secret, esEvent.Resource.UID) {
		evidence = append(evidence, graph.EvidenceItem{
			Type:      graph.EvidenceTypeOwnership,
			Value:     "Secret has ownerReference to ExternalSecret",
			Weight:    1.0,
			Timestamp: time.Now().UnixNano(),
		})
		return 1.0, evidence
	}

	// Evidence 2: Name match (0.9)
	if secret.Name == targetSecretName {
		score += 0.9
		evidence = append(evidence, graph.EvidenceItem{
			Type:      graph.EvidenceTypeLabel,
			Value:     fmt.Sprintf("Secret name matches spec.target.name: %s", targetSecretName),
			Weight:    0.9,
			Timestamp: time.Now().UnixNano(),
		})
	}

	// Evidence 3: Temporal proximity (0.7)
	if e.isExternalSecretReady(externalSecret) {
		// Check if Secret was created/updated recently
		lagMs := (secret.LastSeen - esEvent.Timestamp) / 1_000_000
		if lagMs >= -120000 && lagMs <= 120000 { // Within 2 minutes
			proximityScore := 1.0 - (float64(abs(lagMs)) / 120000.0)
			score += 0.7 * proximityScore
			evidence = append(evidence, graph.EvidenceItem{
				Type:      graph.EvidenceTypeTemporal,
				Value:     fmt.Sprintf("Secret observed within %dms of ExternalSecret sync", abs(lagMs)),
				Weight:    0.7 * proximityScore,
				Timestamp: time.Now().UnixNano(),
			})
		}
	}

	// Evidence 4: Label match (0.5)
	if secret.Labels != nil {
		// Check for external-secrets.io labels
		if _, ok := secret.Labels["external-secrets.io/name"]; ok {
			score += 0.5
			evidence = append(evidence, graph.EvidenceItem{
				Type:      graph.EvidenceTypeLabel,
				Value:     "Secret has external-secrets.io/name label",
				Weight:    0.5,
				Timestamp: time.Now().UnixNano(),
			})
		}
	}

	// Evidence 5: Namespace match (0.3)
	if secret.Namespace == esEvent.Resource.Namespace {
		score += 0.3
		evidence = append(evidence, graph.EvidenceItem{
			Type:      graph.EvidenceTypeNamespace,
			Value:     secret.Namespace,
			Weight:    0.3,
			Timestamp: time.Now().UnixNano(),
		})
	}

	// Cap at 1.0
	if score > 1.0 {
		score = 1.0
	}

	return score, evidence
}

// hasOwnerReference checks if the secret has an ownerReference to the ExternalSecret
func (e *ExternalSecretExtractor) hasOwnerReference(secret *graph.ResourceIdentity, esUID string) bool {
	// This would need to be checked via the actual Secret resource
	// For now, we rely on other evidence
	return false
}

// isExternalSecretReady checks if the ExternalSecret has synced successfully
func (e *ExternalSecretExtractor) isExternalSecretReady(externalSecret map[string]interface{}) bool {
	status, ok := extractors.GetNestedMap(externalSecret, "status")
	if !ok {
		return false
	}

	conditions, ok := extractors.GetNestedArray(status, "conditions")
	if !ok {
		return false
	}

	for _, condInterface := range conditions {
		cond, ok := condInterface.(map[string]interface{})
		if !ok {
			continue
		}

		condType, _ := extractors.GetNestedString(cond, "type")
		condStatus, _ := extractors.GetNestedString(cond, "status")

		if condType == "Ready" && condStatus == "True" {
			return true
		}
	}

	return false
}

// abs returns the absolute value of an int64
func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
