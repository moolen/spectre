package extractors

import (
	"context"
	"encoding/json"
	"fmt"

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
			if edge.ToUID != "" {
				edges = append(edges, edge)
			}
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
				if edge.ToUID != "" {
					edges = append(edges, edge)
				}
			}
		}
	}

	// 3. Extract managed resources (MANAGES) - only for non-delete events
	if event.Type != models.EventTypeDelete {
		managedEdges, err := e.extractManagedResources(ctx, event, spec, lookup)
		if err != nil {
			e.Logger().Warn("Failed to extract managed resources: %v", err)
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

	// Query for top-level resources with Kustomize labels
	// IMPORTANT: Exclude resources that have owners (via OWNS edges) to avoid creating
	// transitive MANAGES edges. For example, we want:
	//   Kustomization --[MANAGES]--> Deployment --[OWNS]--> ReplicaSet --[OWNS]--> Pod
	// NOT:
	//   Kustomization --[MANAGES]--> Pod (bypassing the ownership chain)
	query := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity)
			WHERE (r.namespace = $namespace OR r.namespace = "")
			  AND NOT r.deleted
			  AND r.uid <> $kustomizationUID
			OPTIONAL MATCH (owner:ResourceIdentity)-[:OWNS]->(r)
			WITH r, owner
			WHERE owner IS NULL
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

	// Create scorer for FluxKustomization management relationships
	scorer := NewManagementScorer(
		ManagementScorerConfig{
			LabelTemplates: map[string]string{
				"name":      kustomizeNameLabel,
				"namespace": kustomizeNsLabel,
			},
			NamePrefixWeight:     0.4,
			NamespaceWeight:      0.3,
			TemporalWeight:       0.3,
			TemporalWindowMs:     30000, // 30 seconds
			CheckReconcileEvents: false,
		},
		lookup,
		func(format string, args ...interface{}) {
			e.Logger().Debug(format, args...)
		},
	)

	for _, row := range result.Rows {
		candidateUID := ExtractUID(row)
		if candidateUID == "" {
			continue
		}

		confidence, evidence := scorer.ScoreRelationship(
			ctx,
			event,
			candidateUID,
			kustomizationName,
			kustomizationNamespace,
			targetNamespace,
		)

		if confidence >= 0.5 {
			edge := e.CreateInferredEdge(
				graph.EdgeTypeManages,
				event.Resource.UID,
				candidateUID,
				confidence,
				evidence,
			)
			edges = append(edges, edge)
		}
	}

	return edges, nil
}
