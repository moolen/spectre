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
		// Secret doesn't exist yet - create edge with empty targetUID
		// This allows us to track the intent even before the Secret is created
		return nil
	}

	// Score the relationship based on evidence
	confidence, evidence := e.scoreSecretRelationship(
		ctx,
		event,
		certificate,
		secret,
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

// scoreSecretRelationship scores the Certificate → Secret relationship
func (e *CertificateExtractor) scoreSecretRelationship(
	ctx context.Context,
	certEvent models.Event,
	certificate map[string]interface{},
	secret *graph.ResourceIdentity,
) (float64, []graph.EvidenceItem) {
	evidence := []graph.EvidenceItem{}
	score := 0.0

	// Evidence 1: OwnerReference (100% confidence if present)
	if e.hasOwnerReference(secret, certEvent.Resource.UID) {
		evidence = append(evidence, graph.EvidenceItem{
			Type:      graph.EvidenceTypeOwnership,
			Value:     "Secret has ownerReference to Certificate",
			Weight:    1.0,
			Timestamp: time.Now().UnixNano(),
		})
		return 1.0, evidence
	}

	// Evidence 2: Annotation (0.9)
	if secret.Labels != nil {
		// Check in labels first (some versions store it there)
		if certName, ok := secret.Labels[certNameAnnotation]; ok && certName == certEvent.Resource.Name {
			score += 0.9
			evidence = append(evidence, graph.EvidenceItem{
				Type:      graph.EvidenceTypeAnnotation,
				Value:     fmt.Sprintf("annotation %s=%s", certNameAnnotation, certName),
				Weight:    0.9,
				Timestamp: time.Now().UnixNano(),
			})
		}
	}

	// Evidence 3: Temporal proximity (0.8)
	if e.isCertificateReady(certificate) {
		// Check if Secret was created/updated around the same time as Certificate became ready
		lagMs := (secret.LastSeen - certEvent.Timestamp) / 1_000_000
		if lagMs >= -60000 && lagMs <= 60000 { // Within 60 seconds
			proximityScore := 1.0 - (float64(abs(lagMs)) / 60000.0)
			score += 0.8 * proximityScore
			evidence = append(evidence, graph.EvidenceItem{
				Type:      graph.EvidenceTypeTemporal,
				Value:     fmt.Sprintf("Secret observed within %dms of Certificate", abs(lagMs)),
				Weight:    0.8 * proximityScore,
				Timestamp: time.Now().UnixNano(),
			})
		}
	}

	// Evidence 4: Name match (0.5)
	if secret.Name == certEvent.Resource.Name || // Direct name match
		secret.Name == fmt.Sprintf("%s-tls", certEvent.Resource.Name) { // Common pattern
		score += 0.5
		evidence = append(evidence, graph.EvidenceItem{
			Type:      graph.EvidenceTypeLabel,
			Value:     fmt.Sprintf("Secret name matches Certificate: %s", secret.Name),
			Weight:    0.5,
			Timestamp: time.Now().UnixNano(),
		})
	}

	// Evidence 5: Namespace match (0.3)
	if secret.Namespace == certEvent.Resource.Namespace {
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

// hasOwnerReference checks if the secret has an ownerReference to the certificate
func (e *CertificateExtractor) hasOwnerReference(secret *graph.ResourceIdentity, certUID string) bool {
	// This would need to be checked via the actual Secret resource
	// For now, we rely on other evidence
	// In a real implementation, we'd query the Secret's ownerReferences
	return false
}

// isCertificateReady checks if the Certificate has a Ready condition
func (e *CertificateExtractor) isCertificateReady(certificate map[string]interface{}) bool {
	status, ok := extractors.GetNestedMap(certificate, "status")
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
