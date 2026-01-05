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
	httpRouteKind = "HTTPRoute"
)

// HTTPRouteExtractor extracts HTTPRoute relationships:
// - HTTPRoute→Gateway (spec.parentRefs)
// - HTTPRoute→Service (spec.rules[].backendRefs)
type HTTPRouteExtractor struct {
	*extractors.BaseExtractor
}

// NewHTTPRouteExtractor creates a new HTTPRoute extractor
func NewHTTPRouteExtractor() *HTTPRouteExtractor {
	return &HTTPRouteExtractor{
		BaseExtractor: extractors.NewBaseExtractor("httproute", 100),
	}
}

// Matches checks if this extractor applies to HTTPRoute resources
func (e *HTTPRouteExtractor) Matches(event models.Event) bool {
	return event.Resource.Kind == httpRouteKind &&
		event.Resource.Group == gatewayAPIGroup
}

// ExtractRelationships extracts HTTPRoute→Gateway and HTTPRoute→Service edges
func (e *HTTPRouteExtractor) ExtractRelationships(
	ctx context.Context,
	event models.Event,
	lookup extractors.ResourceLookup,
) ([]graph.Edge, error) {
	edges := []graph.Edge{}

	var httpRoute map[string]interface{}
	if err := json.Unmarshal(event.Data, &httpRoute); err != nil {
		return nil, fmt.Errorf("failed to parse HTTPRoute: %w", err)
	}

	spec, ok := extractors.GetNestedMap(httpRoute, "spec")
	if !ok {
		return edges, nil
	}

	// Extract parentRefs (HTTPRoute→Gateway)
	parentRefs, ok := extractors.GetNestedArray(spec, "parentRefs")
	if ok {
		for idx, parentRefInterface := range parentRefs {
			parentRef, ok := parentRefInterface.(map[string]interface{})
			if !ok {
				continue
			}

			if edge := e.extractParentRefEdge(ctx, event, parentRef, idx, lookup); edge != nil {
				edges = append(edges, *edge)
			}
		}
	}

	// Extract backendRefs (HTTPRoute→Service)
	rules, ok := extractors.GetNestedArray(spec, "rules")
	if ok {
		for ruleIdx, ruleInterface := range rules {
			rule, ok := ruleInterface.(map[string]interface{})
			if !ok {
				continue
			}

			backendRefs, ok := extractors.GetNestedArray(rule, "backendRefs")
			if !ok {
				continue
			}

			for backendIdx, backendRefInterface := range backendRefs {
				backendRef, ok := backendRefInterface.(map[string]interface{})
				if !ok {
					continue
				}

				if edge := e.extractBackendRefEdge(ctx, event, backendRef, ruleIdx, backendIdx, lookup); edge != nil {
					edges = append(edges, *edge)
				}
			}
		}
	}

	return edges, nil
}

// extractParentRefEdge extracts a single parentRef (HTTPRoute→Gateway)
func (e *HTTPRouteExtractor) extractParentRefEdge(
	ctx context.Context,
	event models.Event,
	parentRef map[string]interface{},
	idx int,
	lookup extractors.ResourceLookup,
) *graph.Edge {
	// Get kind (defaults to Gateway)
	kind := "Gateway"
	if k, ok := extractors.GetNestedString(parentRef, "kind"); ok {
		kind = k
	}

	// Get name
	name, ok := extractors.GetNestedString(parentRef, "name")
	if !ok || name == "" {
		return nil
	}

	// Get namespace (defaults to HTTPRoute's namespace)
	namespace := event.Resource.Namespace
	if ns, ok := extractors.GetNestedString(parentRef, "namespace"); ok {
		namespace = ns
	}

	// Look up the parent resource
	targetResource, _ := lookup.FindResourceByNamespace(ctx, namespace, kind, name)
	targetUID := ""
	if targetResource != nil {
		targetUID = targetResource.UID
	}

	fieldPath := fmt.Sprintf("spec.parentRefs[%d]", idx)
	edge := e.CreateReferencesSpecEdge(
		event.Resource.UID,
		targetUID,
		fieldPath,
		kind,
		name,
		namespace,
	)

	return extractors.ValidEdgeOrNil(edge)
}

// extractBackendRefEdge extracts a single backendRef (HTTPRoute→Service)
func (e *HTTPRouteExtractor) extractBackendRefEdge(
	ctx context.Context,
	event models.Event,
	backendRef map[string]interface{},
	ruleIdx, backendIdx int,
	lookup extractors.ResourceLookup,
) *graph.Edge {
	// Get kind (defaults to Service)
	kind := "Service"
	if k, ok := extractors.GetNestedString(backendRef, "kind"); ok {
		kind = k
	}

	// Get name
	name, ok := extractors.GetNestedString(backendRef, "name")
	if !ok || name == "" {
		return nil
	}

	// Get namespace (defaults to HTTPRoute's namespace)
	namespace := event.Resource.Namespace
	if ns, ok := extractors.GetNestedString(backendRef, "namespace"); ok {
		namespace = ns
	}

	// Look up the backend resource
	targetResource, _ := lookup.FindResourceByNamespace(ctx, namespace, kind, name)
	targetUID := ""
	if targetResource != nil {
		targetUID = targetResource.UID
	}

	fieldPath := fmt.Sprintf("spec.rules[%d].backendRefs[%d]", ruleIdx, backendIdx)
	edge := e.CreateReferencesSpecEdge(
		event.Resource.UID,
		targetUID,
		fieldPath,
		kind,
		name,
		namespace,
	)

	return extractors.ValidEdgeOrNil(edge)
}
