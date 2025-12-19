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
	fmt.Printf("[DEBUG] NetworkPolicyExtractor: extracting edges for %s/%s\n", event.Resource.Namespace, event.Resource.Name)
	edges := []graph.Edge{}

	// Skip if being deleted
	if event.Type == models.EventTypeDelete {
		fmt.Printf("[DEBUG] NetworkPolicyExtractor: skipping deleted NetworkPolicy\n")
		return edges, nil
	}

	var netpol map[string]interface{}
	if err := json.Unmarshal(event.Data, &netpol); err != nil {
		return nil, fmt.Errorf("failed to parse NetworkPolicy: %w", err)
	}

	spec, ok := extractors.GetNestedMap(netpol, "spec")
	if !ok {
		fmt.Printf("[DEBUG] NetworkPolicyExtractor: no spec found\n")
		return edges, nil
	}

	// Get podSelector
	podSelector, ok := extractors.GetNestedMap(spec, "podSelector")
	if !ok {
		fmt.Printf("[DEBUG] NetworkPolicyExtractor: no podSelector found\n")
		return edges, nil
	}

	// Get matchLabels from podSelector
	matchLabels, ok := extractors.GetNestedMap(podSelector, "matchLabels")
	if !ok {
		// Empty podSelector {} means select all pods in namespace
		fmt.Printf("[DEBUG] NetworkPolicyExtractor: empty podSelector, selecting all pods\n")
		matchLabels = make(map[string]interface{})
	}

	selectorLabels := extractors.ParseLabelsFromMap(matchLabels)
	fmt.Printf("[DEBUG] NetworkPolicyExtractor: selector labels: %v\n", selectorLabels)

	// Build query for matching pods
	var query graph.GraphQuery
	if len(selectorLabels) == 0 {
		// Empty selector - match all pods in namespace
		query = graph.GraphQuery{
			Query: `
				MATCH (p:ResourceIdentity)
				WHERE p.kind = 'Pod'
				  AND p.namespace = $namespace
				  AND p.deleted = false
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
				  AND p.deleted = false
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

	fmt.Printf("[DEBUG] NetworkPolicyExtractor: query returned %d rows\n", len(result.Rows))

	// Create edges for each matching pod
	// Using SELECTS edge type to indicate the policy applies to these pods
	for _, row := range result.Rows {
		podUID := extractors.ExtractUID(row)
		if podUID == "" {
			fmt.Printf("[DEBUG] NetworkPolicyExtractor: empty UID in row\n")
			continue
		}

		fmt.Printf("[DEBUG] NetworkPolicyExtractor: creating SELECTS edge to Pod %s\n", podUID)

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

	fmt.Printf("[DEBUG] NetworkPolicyExtractor: created %d edges\n", len(edges))
	return edges, nil
}
