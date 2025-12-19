package extractors

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
)

const (
	kustomizeAPIGroup  = "kustomize.toolkit.fluxcd.io"
	kustomizationKind  = "Kustomization"
	kustomizeNameLabel = "kustomize.toolkit.fluxcd.io/name"
	kustomizeNsLabel   = "kustomize.toolkit.fluxcd.io/namespace"
)

// FluxKustomizationExtractor extracts relationships from Flux Kustomization resources
type FluxKustomizationExtractor struct {
	*BaseExtractor
}

// NewFluxKustomizationExtractor creates a new Flux Kustomization extractor
func NewFluxKustomizationExtractor() *FluxKustomizationExtractor {
	return &FluxKustomizationExtractor{
		BaseExtractor: NewBaseExtractor("flux-kustomization", 100),
	}
}

// Matches checks if this extractor applies to Flux Kustomization resources
func (e *FluxKustomizationExtractor) Matches(event models.Event) bool {
	return event.Resource.Group == kustomizeAPIGroup &&
		event.Resource.Kind == kustomizationKind
}

// ExtractRelationships extracts Kustomization relationships
func (e *FluxKustomizationExtractor) ExtractRelationships(
	ctx context.Context,
	event models.Event,
	lookup ResourceLookup,
) ([]graph.Edge, error) {
	edges := []graph.Edge{}

	var kustomization map[string]interface{}
	if err := json.Unmarshal(event.Data, &kustomization); err != nil {
		return nil, fmt.Errorf("failed to parse Kustomization: %w", err)
	}

	spec, ok := GetNestedMap(kustomization, "spec")
	if !ok {
		return edges, nil
	}

	// 1. Extract sourceRef (REFERENCES_SPEC)
	if sourceRef, ok := GetNestedMap(spec, "sourceRef"); ok {
		kind, _ := GetNestedString(sourceRef, "kind")
		name, _ := GetNestedString(sourceRef, "name")
		namespace, _ := GetNestedString(sourceRef, "namespace")

		if namespace == "" {
			namespace = event.Resource.Namespace
		}

		if kind != "" && name != "" {
			targetResource, _ := lookup.FindResourceByNamespace(ctx, namespace, kind, name)
			targetUID := ""
			if targetResource != nil {
				targetUID = targetResource.UID
			}

			edge := e.CreateReferencesSpecEdge(
				event.Resource.UID,
				targetUID,
				"spec.sourceRef",
				kind,
				name,
				namespace,
			)
			edges = append(edges, edge)
		}
	}

	// 2. Extract decryption secretRef (REFERENCES_SPEC)
	if decryption, ok := GetNestedMap(spec, "decryption"); ok {
		if secretRef, ok := GetNestedMap(decryption, "secretRef"); ok {
			if name, ok := GetNestedString(secretRef, "name"); ok {
				targetResource, _ := lookup.FindResourceByNamespace(ctx, event.Resource.Namespace, "Secret", name)
				targetUID := ""
				if targetResource != nil {
					targetUID = targetResource.UID
				}

				edge := e.CreateReferencesSpecEdge(
					event.Resource.UID,
					targetUID,
					"spec.decryption.secretRef",
					"Secret",
					name,
					event.Resource.Namespace,
				)
				edges = append(edges, edge)
			}
		}
	}

	// 3. Extract managed resources (MANAGES) - only for non-delete events
	if event.Type != models.EventTypeDelete {
		managedEdges, err := e.extractManagedResources(ctx, event, spec, lookup)
		if err != nil {
			e.logger.Warn("Failed to extract managed resources: %v", err)
		} else {
			edges = append(edges, managedEdges...)
		}
	}

	return edges, nil
}

// extractManagedResources finds resources managed by this Kustomization
func (e *FluxKustomizationExtractor) extractManagedResources(
	ctx context.Context,
	event models.Event,
	spec map[string]interface{},
	lookup ResourceLookup,
) ([]graph.Edge, error) {
	edges := []graph.Edge{}

	kustomizationName := event.Resource.Name
	kustomizationNamespace := event.Resource.Namespace

	// Determine target namespace
	targetNamespace := kustomizationNamespace
	if ns, ok := GetNestedString(spec, "targetNamespace"); ok && ns != "" {
		targetNamespace = ns
	}

	// Query for resources with Kustomize labels
	query := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity)
			WHERE (r.namespace = $namespace OR r.namespace = "")
			  AND r.deleted = false
			  AND r.uid <> $kustomizationUID
			RETURN r
			LIMIT 500
		`,
		Parameters: map[string]interface{}{
			"namespace":         targetNamespace,
			"kustomizationUID": event.Resource.UID,
		},
	}

	result, err := lookup.QueryGraph(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query potential managed resources: %w", err)
	}

	for _, row := range result.Rows {
		candidateUID := ExtractUID(row)
		if candidateUID == "" {
			continue
		}

		confidence, evidence := e.scoreManagementRelationship(
			ctx,
			event,
			candidateUID,
			kustomizationName,
			kustomizationNamespace,
			lookup,
		)

		if confidence >= 0.5 {
			edge := e.createManagesEdge(
				event.Resource.UID,
				candidateUID,
				confidence,
				evidence,
				event.Timestamp,
			)
			edges = append(edges, edge)
		}
	}

	return edges, nil
}

// scoreManagementRelationship calculates confidence score for MANAGES relationship
func (e *FluxKustomizationExtractor) scoreManagementRelationship(
	ctx context.Context,
	kustomizationEvent models.Event,
	candidateUID string,
	kustomizationName string,
	kustomizationNamespace string,
	lookup ResourceLookup,
) (float64, []graph.EvidenceItem) {
	evidence := []graph.EvidenceItem{}

	candidate, err := lookup.FindResourceByUID(ctx, candidateUID)
	if err != nil {
		return 0.0, evidence
	}

	// Check for Kustomize labels (100% confidence if both present)
	if candidate.Labels != nil {
		if nameLabel, ok := candidate.Labels[kustomizeNameLabel]; ok && nameLabel == kustomizationName {
			if nsLabel, ok := candidate.Labels[kustomizeNsLabel]; ok && nsLabel == kustomizationNamespace {
				evidence = append(evidence, graph.EvidenceItem{
					Type:      graph.EvidenceTypeLabel,
					Value:     fmt.Sprintf("Kustomize labels match: %s=%s, %s=%s", kustomizeNameLabel, kustomizationName, kustomizeNsLabel, kustomizationNamespace),
					Weight:    1.0,
					Timestamp: time.Now().UnixNano(),
				})
				return 1.0, evidence
			}
		}
	}

	// Fallback to heuristic scoring
	totalWeight := 0.0
	earnedWeight := 0.0

	// Evidence 1: Name prefix match (0.4)
	totalWeight += 0.4
	if strings.HasPrefix(strings.ToLower(candidate.Name), strings.ToLower(kustomizationName)) {
		earnedWeight += 0.4
		evidence = append(evidence, graph.EvidenceItem{
			Type:      graph.EvidenceTypeLabel,
			Value:     fmt.Sprintf("name prefix matches: %s", kustomizationName),
			Weight:    0.4,
			Timestamp: time.Now().UnixNano(),
		})
	}

	// Evidence 2: Temporal proximity (0.3)
	totalWeight += 0.3
	lagMs := (candidate.FirstSeen - kustomizationEvent.Timestamp) / 1_000_000
	if lagMs >= 0 && lagMs <= 30000 {
		proximityScore := 1.0 - (float64(lagMs) / 30000.0)
		earnedWeight += 0.3 * proximityScore
		evidence = append(evidence, graph.EvidenceItem{
			Type:      graph.EvidenceTypeTemporal,
			Value:     fmt.Sprintf("created %dms after reconcile", lagMs),
			Weight:    0.3 * proximityScore,
			Timestamp: time.Now().UnixNano(),
		})
	}

	// Evidence 3: Namespace match (0.3)
	totalWeight += 0.3
	if candidate.Namespace == kustomizationEvent.Resource.Namespace {
		earnedWeight += 0.3
		evidence = append(evidence, graph.EvidenceItem{
			Type:      graph.EvidenceTypeNamespace,
			Value:     kustomizationEvent.Resource.Namespace,
			Weight:    0.3,
			Timestamp: time.Now().UnixNano(),
		})
	}

	confidence := 0.0
	if totalWeight > 0 {
		confidence = earnedWeight / totalWeight
	}

	return confidence, evidence
}

// createManagesEdge creates a MANAGES edge with evidence
func (e *FluxKustomizationExtractor) createManagesEdge(
	managerUID, managedUID string,
	confidence float64,
	evidence []graph.EvidenceItem,
	timestamp int64,
) graph.Edge {
	props := graph.ManagesEdge{
		Confidence:      confidence,
		Evidence:        evidence,
		FirstObserved:   timestamp,
		LastValidated:   timestamp,
		ValidationState: graph.ValidationStateValid,
	}
	propsJSON, _ := json.Marshal(props)

	return graph.Edge{
		Type:       graph.EdgeTypeManages,
		FromUID:    managerUID,
		ToUID:      managedUID,
		Properties: propsJSON,
	}
}
