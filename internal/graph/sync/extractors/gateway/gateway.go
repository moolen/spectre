package gateway

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graph/sync/extractors"
	"github.com/moolen/spectre/internal/models"
)

const (
	gatewayAPIGroup = "gateway.networking.k8s.io"
	gatewayKind     = "Gateway"
)

// GatewayExtractor extracts Gateway→GatewayClass REFERENCES_SPEC relationships
type GatewayExtractor struct {
	*extractors.BaseExtractor
}

// NewGatewayExtractor creates a new Gateway extractor
func NewGatewayExtractor() *GatewayExtractor {
	return &GatewayExtractor{
		BaseExtractor: extractors.NewBaseExtractor("gateway", 100),
	}
}

// Matches checks if this extractor applies to Gateway resources
func (e *GatewayExtractor) Matches(event models.Event) bool {
	return event.Resource.Kind == gatewayKind &&
		event.Resource.Group == gatewayAPIGroup
}

// ExtractRelationships extracts Gateway→GatewayClass edges
func (e *GatewayExtractor) ExtractRelationships(
	ctx context.Context,
	event models.Event,
	lookup extractors.ResourceLookup,
) ([]graph.Edge, error) {
	edges := []graph.Edge{}

	var gateway map[string]interface{}
	if err := json.Unmarshal(event.Data, &gateway); err != nil {
		return nil, fmt.Errorf("failed to parse Gateway: %w", err)
	}

	spec, ok := extractors.GetNestedMap(gateway, "spec")
	if !ok {
		return edges, nil
	}

	// Extract gatewayClassName
	gatewayClassName, ok := extractors.GetNestedString(spec, "gatewayClassName")
	if !ok || gatewayClassName == "" {
		return edges, nil
	}

	// GatewayClass is cluster-scoped, so namespace is empty
	targetResource, _ := lookup.FindResourceByNamespace(ctx, "", "GatewayClass", gatewayClassName)
	targetUID := ""
	if targetResource != nil {
		targetUID = targetResource.UID
	}

	edge := e.CreateReferencesSpecEdge(
		event.Resource.UID,
		targetUID,
		"spec.gatewayClassName",
		"GatewayClass",
		gatewayClassName,
		"", // GatewayClass is cluster-scoped
	)
	edges = append(edges, edge)

	return edges, nil
}
