package native

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graph/sync/extractors"
	"github.com/moolen/spectre/internal/models"
)

// ServiceExtractor extracts Service→Pod SELECTS relationships
type ServiceExtractor struct {
	*extractors.BaseExtractor
}

// NewServiceExtractor creates a new Service extractor
func NewServiceExtractor() *ServiceExtractor {
	return &ServiceExtractor{
		BaseExtractor: extractors.NewBaseExtractor("service-selector", 50),
	}
}

// Matches checks if this extractor applies to Services
func (e *ServiceExtractor) Matches(event models.Event) bool {
	return event.Resource.Kind == "Service" &&
		event.Resource.Group == "" // Core API group
}

// ExtractRelationships extracts Service→Pod SELECTS edges
func (e *ServiceExtractor) ExtractRelationships(
	ctx context.Context,
	event models.Event,
	lookup extractors.ResourceLookup,
) ([]graph.Edge, error) {
	edges := []graph.Edge{}

	// Skip if service is being deleted
	if event.Type == models.EventTypeDelete {
		return edges, nil
	}

	var service map[string]interface{}
	if err := json.Unmarshal(event.Data, &service); err != nil {
		return nil, fmt.Errorf("failed to parse Service: %w", err)
	}

	// Get selector from spec
	selector, ok := extractors.GetNestedMap(service, "spec", "selector")
	if !ok || len(selector) == 0 {
		// No selector means the service doesn't select any pods
		return edges, nil
	}

	// Convert selector to map[string]string
	selectorLabels := extractors.ParseLabelsFromMap(selector)
	if len(selectorLabels) == 0 {
		return edges, nil
	}

	// Query for ALL Pods in the same namespace
	// We'll filter by label selector in-memory since Cypher JSON substring matching
	// is unreliable with special characters in label keys (e.g., "app.kubernetes.io/name")
	query := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity)
			WHERE r.kind = 'Pod'
			  AND r.namespace = $namespace
			RETURN r.uid, r.labels, r.deleted
			LIMIT 500
		`,
		Parameters: map[string]interface{}{
			"namespace": event.Resource.Namespace,
		},
	}

	// Debug logging
	e.Logger().Debug("Querying for Pods with selector=%v namespace=%s", selectorLabels, event.Resource.Namespace)
	e.Logger().Debug("Querying ALL Pods in namespace (including deleted), will filter by labels in-memory")

	result, err := lookup.QueryGraph(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query pods: %w", err)
	}

	e.Logger().Debug("Query returned Pods: %d", len(result.Rows))

	// Create SELECTS edges for each matching pod
	matchingCount := 0
	deletedCount := 0
	for _, row := range result.Rows {
		if len(row) < 3 {
			continue
		}

		podUID := extractors.ExtractUID(row)
		if podUID == "" {
			continue
		}

		// Check if Pod is deleted
		deleted := false
		if deletedVal, ok := row[2].(bool); ok {
			deleted = deletedVal
		}
		if deleted {
			deletedCount++
			e.Logger().Debug("Pod is deleted, skipping uid=%s", podUID)
			continue
		}

		// Parse labels from the result
		var podLabels map[string]string
		if labelsJSON, ok := row[1].(string); ok && labelsJSON != "" {
			if err := json.Unmarshal([]byte(labelsJSON), &podLabels); err != nil {
				e.Logger().Debug("Failed to parse labels for Pod uid=%s error=%v", podUID, err)
				continue
			}
		}

		e.Logger().Debug("Pod has labels uid=%s labels=%v", podUID, podLabels)

		// Check if Pod labels match selector
		if !extractors.LabelsMatchSelector(podLabels, selectorLabels) {
			e.Logger().Debug("Pod labels don't match selector, skipping uid=%s", podUID)
			continue
		}

		matchingCount++
		e.Logger().Debug("Pod matches selector! Creating SELECTS edge podUID=%s serviceUID=%s", podUID, event.Resource.UID)

		// Create SELECTS edge with selector information
		props := graph.SelectsEdge{
			SelectorLabels: selectorLabels,
		}
		edge := e.CreateObservedEdge(
			graph.EdgeTypeSelects,
			event.Resource.UID,
			podUID,
			props,
		)
		edges = append(edges, edge)
	}

	e.Logger().Debug("Created SELECTS edges: %d deletedPodsSkipped=%d matchingPods=%d", len(edges), deletedCount, matchingCount)

	return edges, nil
}

// IngressExtractor extracts Ingress→Service REFERENCES_SPEC relationships
type IngressExtractor struct {
	*extractors.BaseExtractor
}

// NewIngressExtractor creates a new Ingress extractor
func NewIngressExtractor() *IngressExtractor {
	return &IngressExtractor{
		BaseExtractor: extractors.NewBaseExtractor("ingress-backend", 50),
	}
}

// Matches checks if this extractor applies to Ingress resources
func (e *IngressExtractor) Matches(event models.Event) bool {
	return event.Resource.Kind == "Ingress" &&
		(event.Resource.Group == "networking.k8s.io" || event.Resource.Group == "extensions")
}

// ExtractRelationships extracts Ingress→Service edges
func (e *IngressExtractor) ExtractRelationships(
	ctx context.Context,
	event models.Event,
	lookup extractors.ResourceLookup,
) ([]graph.Edge, error) {
	edges := []graph.Edge{}

	var ingress map[string]interface{}
	if err := json.Unmarshal(event.Data, &ingress); err != nil {
		return nil, fmt.Errorf("failed to parse Ingress: %w", err)
	}

	spec, ok := extractors.GetNestedMap(ingress, "spec")
	if !ok {
		return edges, nil
	}

	// Extract default backend (if present)
	if defaultBackend, ok := extractors.GetNestedMap(spec, "defaultBackend"); ok {
		if edge := e.extractBackendEdge(ctx, event, defaultBackend, "spec.defaultBackend", lookup); edge != nil {
			edges = append(edges, *edge)
		}
	}

	// Extract rules
	rules, ok := extractors.GetNestedArray(spec, "rules")
	if !ok {
		return edges, nil
	}

	for ruleIdx, ruleInterface := range rules {
		rule, ok := ruleInterface.(map[string]interface{})
		if !ok {
			continue
		}

		// Get HTTP paths
		http, ok := extractors.GetNestedMap(rule, "http")
		if !ok {
			continue
		}

		paths, ok := extractors.GetNestedArray(http, "paths")
		if !ok {
			continue
		}

		for pathIdx, pathInterface := range paths {
			path, ok := pathInterface.(map[string]interface{})
			if !ok {
				continue
			}

			// Get backend
			backend, ok := extractors.GetNestedMap(path, "backend")
			if !ok {
				continue
			}

			fieldPath := fmt.Sprintf("spec.rules[%d].http.paths[%d].backend", ruleIdx, pathIdx)
			if edge := e.extractBackendEdge(ctx, event, backend, fieldPath, lookup); edge != nil {
				edges = append(edges, *edge)
			}
		}
	}

	return edges, nil
}

// extractBackendEdge extracts a single backend reference
func (e *IngressExtractor) extractBackendEdge(
	ctx context.Context,
	event models.Event,
	backend map[string]interface{},
	fieldPath string,
	lookup extractors.ResourceLookup,
) *graph.Edge {
	// Try new API (service.name)
	if service, ok := extractors.GetNestedMap(backend, "service"); ok {
		serviceName, ok := extractors.GetNestedString(service, "name")
		if !ok {
			return nil
		}

		// Look up the service
		targetResource, _ := lookup.FindResourceByNamespace(ctx, event.Resource.Namespace, "Service", serviceName)
		targetUID := ""
		if targetResource != nil {
			targetUID = targetResource.UID
		}

		edge := e.CreateReferencesSpecEdge(
			event.Resource.UID,
			targetUID,
			fieldPath+".service",
			"Service",
			serviceName,
			event.Resource.Namespace,
		)

		return extractors.ValidEdgeOrNil(edge)
	}

	// Try old API (serviceName)
	if serviceName, ok := extractors.GetNestedString(backend, "serviceName"); ok {
		// Look up the service
		targetResource, _ := lookup.FindResourceByNamespace(ctx, event.Resource.Namespace, "Service", serviceName)
		targetUID := ""
		if targetResource != nil {
			targetUID = targetResource.UID
		}

		edge := e.CreateReferencesSpecEdge(
			event.Resource.UID,
			targetUID,
			fieldPath+".serviceName",
			"Service",
			serviceName,
			event.Resource.Namespace,
		)

		return extractors.ValidEdgeOrNil(edge)
	}

	return nil
}
