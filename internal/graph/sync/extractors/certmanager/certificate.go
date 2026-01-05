package certmanager

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
	certManagerGroup     = "cert-manager.io"
	certificateKind      = "Certificate"
	issuerKind           = "Issuer"
	clusterIssuerKind    = "ClusterIssuer"
	certNameAnnotation   = "cert-manager.io/certificate-name"
)

// CertificateExtractor extracts Certificate relationships:
// - Certificate → Issuer/ClusterIssuer (REFERENCES_SPEC)
// - Certificate → Secret (CREATES_OBSERVED with evidence)
type CertificateExtractor struct {
	*extractors.BaseExtractor
}

// NewCertificateExtractor creates a new Certificate extractor
func NewCertificateExtractor() *CertificateExtractor {
	return &CertificateExtractor{
		BaseExtractor: extractors.NewBaseExtractor("cert-manager-certificate", 200),
	}
}

// Matches checks if this extractor applies to Certificate resources
func (e *CertificateExtractor) Matches(event models.Event) bool {
	return event.Resource.Kind == certificateKind &&
		event.Resource.Group == certManagerGroup
}

// ExtractRelationships extracts Certificate relationships
func (e *CertificateExtractor) ExtractRelationships(
	ctx context.Context,
	event models.Event,
	lookup extractors.ResourceLookup,
) ([]graph.Edge, error) {
	edges := []graph.Edge{}

	var certificate map[string]interface{}
	if err := json.Unmarshal(event.Data, &certificate); err != nil {
		return nil, fmt.Errorf("failed to parse Certificate: %w", err)
	}

	spec, ok := extractors.GetNestedMap(certificate, "spec")
	if !ok {
		return edges, nil
	}

	// Extract issuerRef (REFERENCES_SPEC)
	if issuerRef, ok := extractors.GetNestedMap(spec, "issuerRef"); ok {
		if edge := e.extractIssuerRefEdge(ctx, event, issuerRef, lookup); edge != nil {
			edges = append(edges, *edge)
		}
	}

	// Extract Secret relationship (CREATES_OBSERVED)
	// Only if Certificate is not being deleted
	if event.Type != models.EventTypeDelete {
		if secretEdge := e.extractSecretEdge(ctx, event, certificate, spec, lookup); secretEdge != nil {
			edges = append(edges, *secretEdge)
		}
	}

	return edges, nil
}

// extractIssuerRefEdge extracts Certificate → Issuer/ClusterIssuer relationship
func (e *CertificateExtractor) extractIssuerRefEdge(
	ctx context.Context,
	event models.Event,
	issuerRef map[string]interface{},
	lookup extractors.ResourceLookup,
) *graph.Edge {
	// Get issuer kind (defaults to Issuer)
	kind := issuerKind
	if k, ok := extractors.GetNestedString(issuerRef, "kind"); ok && k != "" {
		kind = k
	}

	// Get issuer name
	name, ok := extractors.GetNestedString(issuerRef, "name")
	if !ok || name == "" {
		return nil
	}

	// Determine namespace
	// ClusterIssuer is cluster-scoped, Issuer is namespaced
	namespace := ""
	if kind == issuerKind {
		namespace = event.Resource.Namespace
	}

	// Look up the issuer
	targetResource, _ := lookup.FindResourceByNamespace(ctx, namespace, kind, name)
	targetUID := ""
	if targetResource != nil {
		targetUID = targetResource.UID
	}

	edge := e.CreateReferencesSpecEdge(
		event.Resource.UID,
		targetUID,
		"spec.issuerRef",
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

// extractSecretEdge extracts Certificate → Secret relationship with evidence
func (e *CertificateExtractor) extractSecretEdge(
	ctx context.Context,
	event models.Event,
	certificate map[string]interface{},
	spec map[string]interface{},
	lookup extractors.ResourceLookup,
) *graph.Edge {
	// Get secret name from spec
	secretName, ok := extractors.GetNestedString(spec, "secretName")
	if !ok || secretName == "" {
		return nil
	}

	// Look up the secret
	secret, err := lookup.FindResourceByNamespace(ctx, event.Resource.Namespace, "Secret", secretName)
	if err != nil || secret == nil {
		// Secret doesn't exist yet
		return nil
	}

	// Score the relationship using SecretRelationshipScorer
	scorer := extractors.NewSecretRelationshipScorer(
		extractors.CreateCertificateSecretScorerConfig(),
		lookup,
		func(format string, args ...interface{}) {
			e.Logger().Debug(format, args...)
		},
	)

	confidence, evidence := scorer.ScoreRelationship(
		ctx,
		event,
		certificate,
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
