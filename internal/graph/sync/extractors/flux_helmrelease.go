package extractors

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
)

const (
	fluxAPIGroup    = "helm.toolkit.fluxcd.io"
	helmReleaseKind = "HelmRelease"

	// Flux standard labels
	fluxNameLabel      = "helm.toolkit.fluxcd.io/name"
	fluxNamespaceLabel = "helm.toolkit.fluxcd.io/namespace"

	// Confidence scoring weights
	labelMatchWeight        = 0.4
	namespaceMatchWeight    = 0.1
	temporalProximityWeight = 0.3
	reconcileEventWeight    = 0.2
)

// FluxHelmReleaseExtractor extracts relationships from Flux HelmRelease resources
type FluxHelmReleaseExtractor struct {
	*BaseExtractor
}

// NewFluxHelmReleaseExtractor creates a new Flux HelmRelease extractor
func NewFluxHelmReleaseExtractor() *FluxHelmReleaseExtractor {
	return &FluxHelmReleaseExtractor{
		BaseExtractor: NewBaseExtractor("flux-helmrelease", 10),
	}
}

func (e *FluxHelmReleaseExtractor) Name() string {
	return "flux-helmrelease"
}

func (e *FluxHelmReleaseExtractor) Priority() int {
	return 100 // Run after native K8s extractors (priority 0-99)
}

func (e *FluxHelmReleaseExtractor) Matches(event models.Event) bool {
	return event.Resource.Group == fluxAPIGroup &&
		event.Resource.Kind == helmReleaseKind
}

func (e *FluxHelmReleaseExtractor) ExtractRelationships(
	ctx context.Context,
	event models.Event,
	lookup ResourceLookup,
) ([]graph.Edge, error) {
	e.Logger().Debug("ExtractRelationships called (namespace=%s, name=%s)", event.Resource.Namespace, event.Resource.Name)

	edges := []graph.Edge{}

	// Parse HelmRelease spec
	var helmRelease map[string]interface{}
	if err := json.Unmarshal(event.Data, &helmRelease); err != nil {
		return nil, fmt.Errorf("failed to parse HelmRelease: %w", err)
	}

	spec, ok := helmRelease["spec"].(map[string]interface{})
	if !ok {
		e.Logger().Debug("HelmRelease has no spec (namespace=%s, name=%s)", event.Resource.Namespace, event.Resource.Name)
		return edges, nil // No spec, skip
	}

	// 1. Extract explicit spec references (REFERENCES_SPEC)
	refEdges := e.extractSpecReferences(ctx, event.Resource, spec, lookup)
	e.Logger().Debug("Extracted spec reference edges (count=%d)", len(refEdges))
	edges = append(edges, refEdges...)

	// 2. Extract managed resources (MANAGES) - only on CREATE/UPDATE
	if event.Type != models.EventTypeDelete {
		managedEdges, err := e.extractManagedResources(ctx, event, helmRelease, lookup)
		if err != nil {
			e.Logger().Warn("Failed to extract managed resources (error=%v)", err)
		} else {
			e.Logger().Debug("Extracted managed resource edges (count=%d)", len(managedEdges))
			edges = append(edges, managedEdges...)
		}
	}

	e.Logger().Debug("Returning total edges (count=%d)", len(edges))
	return edges, nil
}

// extractSpecReferences extracts REFERENCES_SPEC edges from spec fields
func (e *FluxHelmReleaseExtractor) extractSpecReferences(
	ctx context.Context,
	resource models.ResourceMetadata,
	spec map[string]interface{},
	lookup ResourceLookup,
) []graph.Edge {
	edges := []graph.Edge{}

	e.Logger().Debug("Extracting spec references (namespace=%s, name=%s)", resource.Namespace, resource.Name)

	// Extract valuesFrom references
	// spec.valuesFrom: [{ kind: Secret, name: my-values }]
	valuesFrom, ok := spec["valuesFrom"].([]interface{})
	if !ok {
		e.Logger().Debug("No valuesFrom found in spec")
	} else {
		e.Logger().Debug("Found valuesFrom entries (count=%d)", len(valuesFrom))
	}

	if ok {
		for i, vf := range valuesFrom {
			vfMap, ok := vf.(map[string]interface{})
			if !ok {
				e.Logger().Debug("valuesFrom entry is not a map (index=%d)", i)
				continue
			}

			kind, _ := vfMap["kind"].(string)
			name, _ := vfMap["name"].(string)

			e.Logger().Debug("Processing valuesFrom entry (index=%d, kind=%s, name=%s)", i, kind, name)

			if kind == "" || name == "" {
				e.Logger().Debug("Skipping valuesFrom entry - empty kind or name (index=%d)", i)
				continue
			}

			// Determine namespace (defaults to HelmRelease namespace)
			namespace := resource.Namespace
			if ns, ok := vfMap["targetNamespace"].(string); ok && ns != "" {
				namespace = ns
			}

			e.Logger().Debug("Looking up referenced resource (namespace=%s, kind=%s, name=%s)", namespace, kind, name)

			// Look up referenced resource
			targetResource, err := lookup.FindResourceByNamespace(ctx, namespace, kind, name)
			targetUID := ""
			if err != nil {
				e.Logger().Debug("Referenced resource not found (will create pending edge) (namespace=%s, kind=%s, name=%s, error=%v)", namespace, kind, name, err)
				// Create edge anyway - this handles race conditions where target isn't indexed yet
			} else {
				targetUID = targetResource.UID
				e.Logger().Debug("Found target resource (uid=%s)", targetUID)
			}

			fieldPath := fmt.Sprintf("spec.valuesFrom[%d]", i)

			edge := e.CreateReferencesSpecEdge(
				resource.UID,
				targetUID,
				fieldPath,
				kind,
				name,
				namespace,
			)

			if edge.ToUID != "" {
				e.Logger().Debug("Created REFERENCES_SPEC edge (from=%s, to=%s, target=%s)", resource.UID, targetUID, fmt.Sprintf("%s/%s/%s", namespace, kind, name))
				edges = append(edges, edge)
			}
		}
	}

	// Extract chart.spec.sourceRef (reference to HelmRepository or GitRepository)
	if chart, ok := spec["chart"].(map[string]interface{}); ok {
		if chartSpec, ok := chart["spec"].(map[string]interface{}); ok {
			if sourceRef, ok := chartSpec["sourceRef"].(map[string]interface{}); ok {
				kind, _ := sourceRef["kind"].(string)
				name, _ := sourceRef["name"].(string)
				namespace, _ := sourceRef["namespace"].(string)

				if namespace == "" {
					namespace = resource.Namespace
				}

				if kind != "" && name != "" {
					targetResource, _ := lookup.FindResourceByNamespace(ctx, namespace, kind, name)
					targetUID := ""
					if targetResource != nil {
						targetUID = targetResource.UID
					}

					edge := e.CreateReferencesSpecEdge(
						resource.UID,
						targetUID,
						"spec.chart.spec.sourceRef",
						kind,
						name,
						namespace,
					)

					if edge.ToUID != "" {
						edges = append(edges, edge)
					}
				}
			}
		}
	}

	// Extract kubeConfig.secretRef (reference to Secret)
	if kubeConfig, ok := spec["kubeConfig"].(map[string]interface{}); ok {
		if secretRef, ok := kubeConfig["secretRef"].(map[string]interface{}); ok {
			name, _ := secretRef["name"].(string)

			if name != "" {
				targetResource, _ := lookup.FindResourceByNamespace(ctx, resource.Namespace, "Secret", name)
				targetUID := ""
				if targetResource != nil {
					targetUID = targetResource.UID
				}

				edge := e.CreateReferencesSpecEdge(
					resource.UID,
					targetUID,
					"spec.kubeConfig.secretRef",
					"Secret",
					name,
					resource.Namespace,
				)

				if edge.ToUID != "" {
					edges = append(edges, edge)
				}
			}
		}
	}

	return edges
}

// extractManagedResources infers MANAGES edges by finding resources with Flux labels
func (e *FluxHelmReleaseExtractor) extractManagedResources(
	ctx context.Context,
	event models.Event,
	helmRelease map[string]interface{},
	lookup ResourceLookup,
) ([]graph.Edge, error) {
	edges := []graph.Edge{}

	releaseName := event.Resource.Name
	releaseNamespace := event.Resource.Namespace

	// Determine target namespace (where resources are deployed)
	targetNamespace := releaseNamespace
	if spec, ok := helmRelease["spec"].(map[string]interface{}); ok {
		if ns, ok := spec["targetNamespace"].(string); ok && ns != "" {
			targetNamespace = ns
		}
	}

	// Find top-level resources in target namespace or cluster-scoped (excluding the HelmRelease itself)
	// Cluster-scoped resources (ClusterRoles, ClusterRoleBindings, CRDs, etc.) have empty namespace
	// IMPORTANT: Exclude resources that have owners (via OWNS edges) to avoid creating
	// transitive MANAGES edges. For example, we want:
	//   HelmRelease --[MANAGES]--> Deployment --[OWNS]--> ReplicaSet --[OWNS]--> Pod
	// NOT:
	//   HelmRelease --[MANAGES]--> Pod (bypassing the ownership chain)
	query := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity)
			WHERE (r.namespace = $namespace OR r.namespace = "")
			  AND NOT r.deleted
			  AND r.uid <> $helmReleaseUID
			OPTIONAL MATCH (owner:ResourceIdentity)-[:OWNS]->(r)
			WITH r, owner
			WHERE owner IS NULL
			RETURN r
			LIMIT 500
		`,
		Parameters: map[string]interface{}{
			"namespace":      targetNamespace,
			"helmReleaseUID": event.Resource.UID,
		},
	}

	result, err := lookup.QueryGraph(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query potential managed resources: %w", err)
	}

	e.Logger().Debug("Found potential managed resources (count=%d, namespace=%s)", len(result.Rows), targetNamespace)

	// Create scorer for FluxHelmRelease management relationships
	scorer := NewManagementScorer(
		ManagementScorerConfig{
			LabelTemplates: map[string]string{
				"name":      fluxNameLabel,
				"namespace": fluxNamespaceLabel,
			},
			NamePrefixWeight:     labelMatchWeight,
			NamespaceWeight:      namespaceMatchWeight,
			TemporalWeight:       temporalProximityWeight,
			ReconcileWeight:      reconcileEventWeight,
			TemporalWindowMs:     30000, // 30 seconds
			CheckReconcileEvents: true,
		},
		lookup,
		func(format string, args ...interface{}) {
			e.Logger().Debug(format, args...)
		},
	)

	// For each candidate resource, check if it's managed by this HelmRelease
	for _, row := range result.Rows {
		candidateUID := ExtractUID(row)
		if candidateUID == "" {
			continue
		}

		// Score this relationship using shared scorer
		confidence, evidence := scorer.ScoreRelationship(
			ctx,
			event,
			candidateUID,
			releaseName,
			releaseNamespace,
			targetNamespace,
		)

		// Only create edge if confidence meets threshold
		if confidence >= 0.5 {
			edge := e.CreateInferredEdge(
				graph.EdgeTypeManages,
				event.Resource.UID,
				candidateUID,
				confidence,
				evidence,
			)

			edges = append(edges, edge)
			e.Logger().Debug("Created MANAGES edge (toUID=%s, confidence=%f)", candidateUID, confidence)
		}
	}

	return edges, nil
}
