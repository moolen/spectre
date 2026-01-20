package native

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graph/sync/extractors"
	"github.com/moolen/spectre/internal/models"
)

// NetworkPolicyExtractor extracts NetworkPolicy→Pod relationships
type NetworkPolicyExtractor struct {
	*extractors.BaseExtractor
}

// NewNetworkPolicyExtractor creates a new NetworkPolicy extractor
func NewNetworkPolicyExtractor() *NetworkPolicyExtractor {
	return &NetworkPolicyExtractor{
		BaseExtractor: extractors.NewBaseExtractor("networkpolicy-selector", 50),
	}
}

// Matches checks if this extractor applies to NetworkPolicy resources
func (e *NetworkPolicyExtractor) Matches(event models.Event) bool {
	return event.Resource.Kind == "NetworkPolicy" &&
		event.Resource.Group == "networking.k8s.io"
}

// ExtractRelationships extracts NetworkPolicy→Pod edges
func (e *NetworkPolicyExtractor) ExtractRelationships(
	ctx context.Context,
	event models.Event,
	lookup extractors.ResourceLookup,
) ([]graph.Edge, error) {
	e.Logger().Debug("Extracting edges namespace=%s name=%s", event.Resource.Namespace, event.Resource.Name)
	edges := []graph.Edge{}

	// Skip if being deleted
	if event.Type == models.EventTypeDelete {
		e.Logger().Debug("Skipping deleted NetworkPolicy")
		return edges, nil
	}

	var netpol map[string]interface{}
	if err := json.Unmarshal(event.Data, &netpol); err != nil {
		return nil, fmt.Errorf("failed to parse NetworkPolicy: %w", err)
	}

	spec, ok := extractors.GetNestedMap(netpol, "spec")
	if !ok {
		e.Logger().Debug("No spec found")
		return edges, nil
	}

	// Get podSelector
	podSelector, ok := extractors.GetNestedMap(spec, "podSelector")
	if !ok {
		e.Logger().Debug("No podSelector found")
		return edges, nil
	}

	// Get matchLabels from podSelector
	matchLabels, ok := extractors.GetNestedMap(podSelector, "matchLabels")
	if !ok {
		// Empty podSelector {} means select all pods in namespace
		e.Logger().Debug("Empty podSelector, selecting all pods")
		matchLabels = make(map[string]interface{})
	}

	selectorLabels := extractors.ParseLabelsFromMap(matchLabels)
	e.Logger().Debug("Selector labels: %v", selectorLabels)

	// Build query for matching pods
	var query graph.GraphQuery
	if len(selectorLabels) == 0 {
		// Empty selector - match all pods in namespace
		query = graph.GraphQuery{
			Query: `
				MATCH (p:ResourceIdentity)
				WHERE p.kind = 'Pod'
				  AND p.namespace = $namespace
				  AND NOT p.deleted
				RETURN p.uid
				LIMIT 500
			`,
			Parameters: map[string]interface{}{
				"namespace": event.Resource.Namespace,
			},
		}
	} else {
		// Selector with labels
		labelQuery := extractors.BuildLabelQuery(selectorLabels, "p")
		query = graph.GraphQuery{
			Query: `
				MATCH (p:ResourceIdentity)
				WHERE p.kind = 'Pod'
				  AND p.namespace = $namespace
				  AND NOT p.deleted
				  AND ` + labelQuery + `
				RETURN p.uid
				LIMIT 500
			`,
			Parameters: map[string]interface{}{
				"namespace": event.Resource.Namespace,
			},
		}
	}

	result, err := lookup.QueryGraph(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query pods: %w", err)
	}

	e.Logger().Debug("Query returned rows: %d", len(result.Rows))

	// Create edges for each matching pod
	// Using SELECTS edge type to indicate the policy applies to these pods
	for _, row := range result.Rows {
		podUID := extractors.ExtractUID(row)
		if podUID == "" {
			e.Logger().Debug("Empty UID in row")
			continue
		}

		e.Logger().Debug("Creating SELECTS edge to Pod uid=%s", podUID)

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

	e.Logger().Debug("Created edges: %d", len(edges))
	return edges, nil
}
