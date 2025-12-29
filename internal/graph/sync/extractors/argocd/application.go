package argocd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graph/sync/extractors"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

const (
	argoCDApplicationExtractorName = "argocd-application"
	argoCDInstanceLabel            = "argocd.argoproj.io/instance"
	argoCDNamespaceAnnotation      = "argocd.argoproj.io/sync-options"
)

// ArgoCDApplicationExtractor extracts relationships for ArgoCD Application resources
type ArgoCDApplicationExtractor struct {
	extractors.BaseExtractor
	logger *logging.Logger
}

// NewArgoCDApplicationExtractor creates a new ArgoCD Application extractor
func NewArgoCDApplicationExtractor() *ArgoCDApplicationExtractor {
	return &ArgoCDApplicationExtractor{
		BaseExtractor: extractors.BaseExtractor{},
		logger:        logging.GetLogger("extractors.argocd-application"),
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
	secretEdges := e.extractSecretReferences(event.Resource, spec)
	edges = append(edges, secretEdges...)

	// Only extract MANAGES edges for UPDATE events (not CREATE or DELETE)
	// This ensures the Application has already reconciled
	if event.Type != models.EventTypeUpdate {
		return edges, nil
	}

	// Extract MANAGES edges to managed resources
	managedEdges, err := e.extractManagedResources(ctx, event, spec, lookup)
	if err != nil {
		e.logger.Error("Failed to extract managed resources for Application %s/%s: %v",
			event.Resource.Namespace, event.Resource.Name, err)
		return edges, nil // Don't fail the entire extraction
	}
	edges = append(edges, managedEdges...)

	return edges, nil
}

// extractSecretReferences extracts REFERENCES_SPEC edges to Secret resources
func (e *ArgoCDApplicationExtractor) extractSecretReferences(
	appResource models.ResourceMetadata,
	spec map[string]interface{},
) []graph.Edge {
	edges := []graph.Edge{}

	// Handle single source
	if source, ok := extractors.GetNestedMap(spec, "source"); ok {
		edges = append(edges, e.extractSecretRefsFromSource(appResource, source, "spec.source")...)
	}

	// Handle multiple sources (sources[])
	if sources, ok := extractors.GetNestedArray(spec, "sources"); ok {
		for i, sourceInterface := range sources {
			if source, ok := sourceInterface.(map[string]interface{}); ok {
				fieldPath := fmt.Sprintf("spec.sources[%d]", i)
				edges = append(edges, e.extractSecretRefsFromSource(appResource, source, fieldPath)...)
			}
		}
	}

	return edges
}

// extractSecretRefsFromSource extracts Secret references from a single source
func (e *ArgoCDApplicationExtractor) extractSecretRefsFromSource(
	appResource models.ResourceMetadata,
	source map[string]interface{},
	fieldPathPrefix string,
) []graph.Edge {
	edges := []graph.Edge{}

	// Git repository secrets
	if username, ok := extractors.GetNestedMap(source, "usernameSecret"); ok {
		if name, ok := username["name"].(string); ok {
			edges = append(edges, e.createSecretRefEdge(
				appResource, name, appResource.Namespace,
				fieldPathPrefix+".usernameSecret.name"))
		}
	}

	if password, ok := extractors.GetNestedMap(source, "passwordSecret"); ok {
		if name, ok := password["name"].(string); ok {
			edges = append(edges, e.createSecretRefEdge(
				appResource, name, appResource.Namespace,
				fieldPathPrefix+".passwordSecret.name"))
		}
	}

	if token, ok := extractors.GetNestedMap(source, "tokenSecret"); ok {
		if name, ok := token["name"].(string); ok {
			edges = append(edges, e.createSecretRefEdge(
				appResource, name, appResource.Namespace,
				fieldPathPrefix+".tokenSecret.name"))
		}
	}

	if tlsCert, ok := extractors.GetNestedMap(source, "tlsClientCertSecret"); ok {
		if name, ok := tlsCert["name"].(string); ok {
			edges = append(edges, e.createSecretRefEdge(
				appResource, name, appResource.Namespace,
				fieldPathPrefix+".tlsClientCertSecret.name"))
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
							edges = append(edges, e.createSecretRefEdge(
								appResource, name, appResource.Namespace,
								fmt.Sprintf("%s.helm.valuesFrom[%d].secretKeyRef.name", fieldPathPrefix, i)))
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
	appResource models.ResourceMetadata,
	secretName, secretNamespace, fieldPath string,
) graph.Edge {
	return e.BaseExtractor.CreateReferencesSpecEdge(
		appResource.UID,
		"", // ToUID will be resolved by builder
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

	// Query for resources with ArgoCD instance label
	// Note: We query across the target namespace, not the Application's namespace
	query := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity)
			WHERE r.deleted = false
			  AND r.labels CONTAINS $labelQuery
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

	// Score each candidate resource
	for _, row := range result.Rows {
		candidateUID := extractors.ExtractUID(row)
		if candidateUID == "" {
			continue
		}

		confidence, evidence := e.scoreManagementRelationship(
			ctx,
			event,
			candidateUID,
			appName,
			targetNamespace,
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
func (e *ArgoCDApplicationExtractor) scoreManagementRelationship(
	ctx context.Context,
	appEvent models.Event,
	candidateUID string,
	appName string,
	targetNamespace string,
	lookup extractors.ResourceLookup,
) (float64, []graph.EvidenceItem) {
	evidence := []graph.EvidenceItem{}

	candidate, err := lookup.FindResourceByUID(ctx, candidateUID)
	if err != nil {
		return 0.0, evidence
	}

	// Check for ArgoCD instance label (perfect confidence if matches)
	if candidate.Labels != nil {
		if instanceLabel, ok := candidate.Labels[argoCDInstanceLabel]; ok && instanceLabel == appName {
			evidence = append(evidence, graph.EvidenceItem{
				Type:      graph.EvidenceTypeLabel,
				Value:     fmt.Sprintf("ArgoCD instance label matches: %s=%s", argoCDInstanceLabel, appName),
				Weight:    1.0,
				Timestamp: time.Now().UnixNano(),
			})
			return 1.0, evidence
		}
	}

	// Fallback to heuristic scoring if label doesn't match perfectly
	// This handles cases where labels might be customized or missing
	totalWeight := 0.0
	earnedWeight := 0.0

	// Evidence 1: Name prefix match (0.4 weight)
	// ArgoCD often names resources with Application name as prefix
	totalWeight += 0.4
	if strings.HasPrefix(strings.ToLower(candidate.Name), strings.ToLower(appName)) {
		earnedWeight += 0.4
		evidence = append(evidence, graph.EvidenceItem{
			Type:      graph.EvidenceTypeLabel,
			Value:     fmt.Sprintf("name prefix matches: %s", appName),
			Weight:    0.4,
			Timestamp: time.Now().UnixNano(),
		})
	}

	// Evidence 2: Namespace match (0.3 weight)
	totalWeight += 0.3
	if candidate.Namespace == targetNamespace {
		earnedWeight += 0.3
		evidence = append(evidence, graph.EvidenceItem{
			Type:      graph.EvidenceTypeNamespace,
			Value:     fmt.Sprintf("deployed to target namespace: %s", targetNamespace),
			Weight:    0.3,
			Timestamp: time.Now().UnixNano(),
		})
	}

	// Evidence 3: Temporal proximity (0.3 weight)
	// Resources created within 2 minutes of Application sync
	totalWeight += 0.3
	lagMs := (candidate.FirstSeen - appEvent.Timestamp) / 1_000_000
	maxLagMs := int64(120000) // 2 minutes
	if lagMs >= 0 && lagMs <= maxLagMs {
		proximityScore := 1.0 - (float64(lagMs) / float64(maxLagMs))
		weight := 0.3 * proximityScore
		earnedWeight += weight
		evidence = append(evidence, graph.EvidenceItem{
			Type:      graph.EvidenceTypeTemporal,
			Value:     fmt.Sprintf("created %dms after Application sync", lagMs),
			Weight:    weight,
			Timestamp: time.Now().UnixNano(),
		})
	}

	if totalWeight > 0 {
		return earnedWeight / totalWeight, evidence
	}
	return 0.0, evidence
}

// createManagesEdge creates a MANAGES edge with evidence
func (e *ArgoCDApplicationExtractor) createManagesEdge(
	fromUID, toUID string,
	confidence float64,
	evidence []graph.EvidenceItem,
	timestamp int64,
) graph.Edge {
	return e.BaseExtractor.CreateInferredEdge(
		graph.EdgeTypeManages,
		fromUID,
		toUID,
		confidence,
		evidence,
	)
}
