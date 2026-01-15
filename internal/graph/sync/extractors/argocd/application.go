package argocd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graph/sync/extractors"
	"github.com/moolen/spectre/internal/models"
)

const (
	argoCDApplicationExtractorName = "argocd-application"
	argoCDInstanceLabel            = "argocd.argoproj.io/instance"
	argoCDNamespaceAnnotation      = "argocd.argoproj.io/sync-options"
)

// ArgoCDApplicationExtractor extracts relationships for ArgoCD Application resources
type ArgoCDApplicationExtractor struct {
	*extractors.BaseExtractor
}

// NewArgoCDApplicationExtractor creates a new ArgoCD Application extractor
func NewArgoCDApplicationExtractor() *ArgoCDApplicationExtractor {
	return &ArgoCDApplicationExtractor{
		BaseExtractor: extractors.NewBaseExtractor("argocd-application", 20),
	}
}

// Name returns the extractor name
func (e *ArgoCDApplicationExtractor) Name() string {
	return argoCDApplicationExtractorName
}

// Priority returns execution priority (lower than Flux to run after)
func (e *ArgoCDApplicationExtractor) Priority() int {
	return 20 // Run after Flux extractors (10-15)
}

// Matches checks if this extractor should run for the event
func (e *ArgoCDApplicationExtractor) Matches(event models.Event) bool {
	return event.Resource.Kind == "Application" &&
		event.Resource.Group == "argoproj.io"
}

// ExtractRelationships extracts both REFERENCES_SPEC and MANAGES edges
func (e *ArgoCDApplicationExtractor) ExtractRelationships(
	ctx context.Context,
	event models.Event,
	lookup extractors.ResourceLookup,
) ([]graph.Edge, error) {
	edges := []graph.Edge{}

	// Parse Application spec
	var appData map[string]interface{}
	if err := json.Unmarshal(event.Data, &appData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal Application data: %w", err)
	}

	spec, ok := extractors.GetNestedMap(appData, "spec")
	if !ok {
		return edges, nil
	}

	// Extract REFERENCES_SPEC edges to Secrets
	secretEdges := e.extractSecretReferences(ctx, event.Resource, spec, lookup)
	edges = append(edges, secretEdges...)

	// Only extract MANAGES edges for UPDATE events (not CREATE or DELETE)
	// This ensures the Application has already reconciled
	if event.Type != models.EventTypeUpdate {
		return edges, nil
	}

	// Extract MANAGES edges to managed resources
	managedEdges, err := e.extractManagedResources(ctx, event, spec, lookup)
	if err != nil {
		e.Logger().Error("Failed to extract managed resources for Application %s/%s: %v",
			event.Resource.Namespace, event.Resource.Name, err)
		return edges, nil // Don't fail the entire extraction
	}
	edges = append(edges, managedEdges...)

	return edges, nil
}

// extractSecretReferences extracts REFERENCES_SPEC edges to Secret resources
func (e *ArgoCDApplicationExtractor) extractSecretReferences(
	ctx context.Context,
	appResource models.ResourceMetadata,
	spec map[string]interface{},
	lookup extractors.ResourceLookup,
) []graph.Edge {
	edges := []graph.Edge{}

	// Handle single source
	if source, ok := extractors.GetNestedMap(spec, "source"); ok {
		edges = append(edges, e.extractSecretRefsFromSource(ctx, appResource, source, "spec.source", lookup)...)
	}

	// Handle multiple sources (sources[])
	if sources, ok := extractors.GetNestedArray(spec, "sources"); ok {
		for i, sourceInterface := range sources {
			if source, ok := sourceInterface.(map[string]interface{}); ok {
				fieldPath := fmt.Sprintf("spec.sources[%d]", i)
				edges = append(edges, e.extractSecretRefsFromSource(ctx, appResource, source, fieldPath, lookup)...)
			}
		}
	}

	return edges
}

// extractSecretRefsFromSource extracts Secret references from a single source
func (e *ArgoCDApplicationExtractor) extractSecretRefsFromSource(
	ctx context.Context,
	appResource models.ResourceMetadata,
	source map[string]interface{},
	fieldPathPrefix string,
	lookup extractors.ResourceLookup,
) []graph.Edge {
	edges := []graph.Edge{}

	// Git repository secrets
	if username, ok := extractors.GetNestedMap(source, "usernameSecret"); ok {
		if name, ok := username["name"].(string); ok {
			if edge := e.createSecretRefEdge(ctx, appResource, name, appResource.Namespace, fieldPathPrefix+".usernameSecret.name", lookup); edge.ToUID != "" {
				edges = append(edges, edge)
			}
		}
	}

	if password, ok := extractors.GetNestedMap(source, "passwordSecret"); ok {
		if name, ok := password["name"].(string); ok {
			if edge := e.createSecretRefEdge(ctx, appResource, name, appResource.Namespace, fieldPathPrefix+".passwordSecret.name", lookup); edge.ToUID != "" {
				edges = append(edges, edge)
			}
		}
	}

	if token, ok := extractors.GetNestedMap(source, "tokenSecret"); ok {
		if name, ok := token["name"].(string); ok {
			if edge := e.createSecretRefEdge(ctx, appResource, name, appResource.Namespace, fieldPathPrefix+".tokenSecret.name", lookup); edge.ToUID != "" {
				edges = append(edges, edge)
			}
		}
	}

	if tlsCert, ok := extractors.GetNestedMap(source, "tlsClientCertSecret"); ok {
		if name, ok := tlsCert["name"].(string); ok {
			if edge := e.createSecretRefEdge(ctx, appResource, name, appResource.Namespace, fieldPathPrefix+".tlsClientCertSecret.name", lookup); edge.ToUID != "" {
				edges = append(edges, edge)
			}
		}
	}

	// Helm repository secrets
	if helm, ok := extractors.GetNestedMap(source, "helm"); ok {
		// Helm values from secrets
		if valuesFrom, ok := extractors.GetNestedArray(helm, "valuesFrom"); ok {
			for i, valFromInterface := range valuesFrom {
				if valFrom, ok := valFromInterface.(map[string]interface{}); ok {
					if secretKeyRef, ok := extractors.GetNestedMap(valFrom, "secretKeyRef"); ok {
						if name, ok := secretKeyRef["name"].(string); ok {
							fieldPath := fmt.Sprintf("%s.helm.valuesFrom[%d].secretKeyRef.name", fieldPathPrefix, i)
							if edge := e.createSecretRefEdge(ctx, appResource, name, appResource.Namespace, fieldPath, lookup); edge.ToUID != "" {
								edges = append(edges, edge)
							}
						}
					}
				}
			}
		}
	}

	return edges
}

// createSecretRefEdge creates a REFERENCES_SPEC edge to a Secret
func (e *ArgoCDApplicationExtractor) createSecretRefEdge(
	ctx context.Context,
	appResource models.ResourceMetadata,
	secretName, secretNamespace, fieldPath string,
	lookup extractors.ResourceLookup,
) graph.Edge {
	// Look up the Secret resource
	targetResource, _ := lookup.FindResourceByNamespace(ctx, secretNamespace, "Secret", secretName)
	targetUID := ""
	if targetResource != nil {
		targetUID = targetResource.UID
	}

	return e.BaseExtractor.CreateReferencesSpecEdge(
		appResource.UID,
		targetUID,
		fieldPath,
		"Secret",
		secretName,
		secretNamespace,
	)
}

// extractManagedResources extracts MANAGES edges to resources managed by this Application
func (e *ArgoCDApplicationExtractor) extractManagedResources(
	ctx context.Context,
	event models.Event,
	spec map[string]interface{},
	lookup extractors.ResourceLookup,
) ([]graph.Edge, error) {
	edges := []graph.Edge{}

	appName := event.Resource.Name

	// Get destination namespace (where resources are deployed)
	// This may be different from the Application's namespace
	destination, ok := extractors.GetNestedMap(spec, "destination")
	if !ok {
		return edges, nil
	}

	targetNamespace, _ := destination["namespace"].(string)
	// If no namespace specified, it could be cluster-scoped or same as Application
	if targetNamespace == "" {
		targetNamespace = event.Resource.Namespace
	}

	// Query for top-level resources with ArgoCD instance label
	// Note: We query across the target namespace, not the Application's namespace
	// IMPORTANT: Exclude resources that have owners (via OWNS edges) to avoid creating
	// transitive MANAGES edges. For example, we want:
	//   Application --[MANAGES]--> Deployment --[OWNS]--> ReplicaSet --[OWNS]--> Pod
	// NOT:
	//   Application --[MANAGES]--> Pod (bypassing the ownership chain)
	query := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity)
			WHERE NOT r.deleted
			  AND r.labels CONTAINS $labelQuery
			  AND NOT EXISTS { MATCH (:ResourceIdentity)-[:OWNS]->(r) }
			RETURN r.uid
		`,
		Parameters: map[string]interface{}{
			"labelQuery": fmt.Sprintf(`"%s":"%s"`, argoCDInstanceLabel, appName),
		},
	}

	result, err := lookup.QueryGraph(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query managed resources: %w", err)
	}

	// Create scorer for ArgoCD Application management relationships
	scorer := extractors.NewManagementScorer(
		extractors.ManagementScorerConfig{
			LabelTemplates: map[string]string{
				"name": argoCDInstanceLabel,
			},
			NamePrefixWeight:     0.4,
			NamespaceWeight:      0.3,
			TemporalWeight:       0.3,
			TemporalWindowMs:     120000, // 2 minutes (ArgoCD syncs can take longer)
			CheckReconcileEvents: false,
		},
		lookup,
		func(format string, args ...interface{}) {
			e.Logger().Debug(format, args...)
		},
	)

	// Score each candidate resource
	for _, row := range result.Rows {
		candidateUID := extractors.ExtractUID(row)
		if candidateUID == "" {
			continue
		}

		confidence, evidence := scorer.ScoreRelationship(
			ctx,
			event,
			candidateUID,
			appName,
			event.Resource.Namespace,
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

