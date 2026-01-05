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

	// Return nil if edge is invalid (toUID is empty)
	if edge.ToUID == "" {
		return nil
	}
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

	// Score the relationship using SecretRelationshipScorer
	scorer := extractors.NewSecretRelationshipScorer(
		extractors.CreateExternalSecretScorerConfig(),
		lookup,
		func(format string, args ...interface{}) {
			e.Logger().Debug(format, args...)
		},
	)

	confidence, evidence := scorer.ScoreRelationship(
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
