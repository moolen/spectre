package gateway

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graph/sync/extractors"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPRouteExtractor_Matches(t *testing.T) {
	extractor := NewHTTPRouteExtractor()

	tests := []struct {
		name     string
		event    models.Event
		expected bool
	}{
		{
			name: "matches HTTPRoute",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "HTTPRoute",
					Group: "gateway.networking.k8s.io",
				},
			},
			expected: true,
		},
		{
			name: "does not match Gateway",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "Gateway",
					Group: "gateway.networking.k8s.io",
				},
			},
			expected: false,
		},
		{
			name: "does not match Ingress",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "Ingress",
					Group: "networking.k8s.io",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractor.Matches(tt.event)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHTTPRouteExtractor_ExtractRelationships(t *testing.T) {
	extractor := NewHTTPRouteExtractor()

	httpRouteData := map[string]interface{}{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "HTTPRoute",
		"metadata": map[string]interface{}{
			"name":      "frontend-route",
			"namespace": "default",
			"uid":       "httproute-uid-123",
		},
		"spec": map[string]interface{}{
			"parentRefs": []interface{}{
				map[string]interface{}{
					"name": "my-gateway",
					"kind": "Gateway",
				},
			},
			"rules": []interface{}{
				map[string]interface{}{
					"backendRefs": []interface{}{
						map[string]interface{}{
							"name": "frontend-service",
							"kind": "Service",
							"port": 80,
						},
					},
				},
			},
		},
	}

	httpRouteJSON, err := json.Marshal(httpRouteData)
	require.NoError(t, err)

	event := models.Event{
		ID:   "event-1",
		Type: models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			UID:       "httproute-uid-123",
			Kind:      "HTTPRoute",
			Group:     "gateway.networking.k8s.io",
			Version:   "v1",
			Namespace: "default",
			Name:      "frontend-route",
		},
		Data:      httpRouteJSON,
		Timestamp: 1000000000,
	}

	t.Run("extracts parentRefs and backendRefs", func(t *testing.T) {
		lookup := extractors.NewMockResourceLookup()
		lookup.AddResource(&graph.ResourceIdentity{
			UID:       "gateway-uid-456",
			Kind:      "Gateway",
			Namespace: "default",
			Name:      "my-gateway",
		})
		lookup.AddResource(&graph.ResourceIdentity{
			UID:       "service-uid-789",
			Kind:      "Service",
			Namespace: "default",
			Name:      "frontend-service",
		})

		edges, err := extractor.ExtractRelationships(context.Background(), event, lookup)
		require.NoError(t, err)
		require.Len(t, edges, 2)

		// Check parentRef edge
		parentRefEdge := edges[0]
		assert.Equal(t, graph.EdgeTypeReferencesSpec, parentRefEdge.Type)
		assert.Equal(t, "httproute-uid-123", parentRefEdge.FromUID)
		assert.Equal(t, "gateway-uid-456", parentRefEdge.ToUID)

		var parentRefProps graph.ReferencesSpecEdge
		err = json.Unmarshal(parentRefEdge.Properties, &parentRefProps)
		require.NoError(t, err)
		assert.Equal(t, "spec.parentRefs[0]", parentRefProps.FieldPath)
		assert.Equal(t, "Gateway", parentRefProps.RefKind)
		assert.Equal(t, "my-gateway", parentRefProps.RefName)

		// Check backendRef edge
		backendRefEdge := edges[1]
		assert.Equal(t, graph.EdgeTypeReferencesSpec, backendRefEdge.Type)
		assert.Equal(t, "httproute-uid-123", backendRefEdge.FromUID)
		assert.Equal(t, "service-uid-789", backendRefEdge.ToUID)

		var backendRefProps graph.ReferencesSpecEdge
		err = json.Unmarshal(backendRefEdge.Properties, &backendRefProps)
		require.NoError(t, err)
		assert.Equal(t, "spec.rules[0].backendRefs[0]", backendRefProps.FieldPath)
		assert.Equal(t, "Service", backendRefProps.RefKind)
		assert.Equal(t, "frontend-service", backendRefProps.RefName)
	})

	t.Run("handles multiple parentRefs and backendRefs", func(t *testing.T) {
		multipleRefsData := map[string]interface{}{
			"apiVersion": "gateway.networking.k8s.io/v1",
			"kind":       "HTTPRoute",
			"metadata": map[string]interface{}{
				"name":      "frontend-route",
				"namespace": "default",
				"uid":       "httproute-uid-123",
			},
			"spec": map[string]interface{}{
				"parentRefs": []interface{}{
					map[string]interface{}{
						"name": "gateway-1",
					},
					map[string]interface{}{
						"name": "gateway-2",
					},
				},
				"rules": []interface{}{
					map[string]interface{}{
						"backendRefs": []interface{}{
							map[string]interface{}{
								"name": "service-1",
							},
							map[string]interface{}{
								"name": "service-2",
							},
						},
					},
				},
			},
		}

		multipleRefsJSON, err := json.Marshal(multipleRefsData)
		require.NoError(t, err)

		multipleRefsEvent := event
		multipleRefsEvent.Data = multipleRefsJSON

		lookup := extractors.NewMockResourceLookup()
		lookup.AddResource(&graph.ResourceIdentity{
			UID:       "gateway-1-uid",
			Kind:      "Gateway",
			Namespace: "default",
			Name:      "gateway-1",
		})
		lookup.AddResource(&graph.ResourceIdentity{
			UID:       "gateway-2-uid",
			Kind:      "Gateway",
			Namespace: "default",
			Name:      "gateway-2",
		})
		lookup.AddResource(&graph.ResourceIdentity{
			UID:       "service-1-uid",
			Kind:      "Service",
			Namespace: "default",
			Name:      "service-1",
		})
		lookup.AddResource(&graph.ResourceIdentity{
			UID:       "service-2-uid",
			Kind:      "Service",
			Namespace: "default",
			Name:      "service-2",
		})

		edges, err := extractor.ExtractRelationships(context.Background(), multipleRefsEvent, lookup)
		require.NoError(t, err)
		assert.Len(t, edges, 4) // 2 parentRefs + 2 backendRefs
	})

	t.Run("handles cross-namespace references", func(t *testing.T) {
		crossNsData := map[string]interface{}{
			"apiVersion": "gateway.networking.k8s.io/v1",
			"kind":       "HTTPRoute",
			"metadata": map[string]interface{}{
				"name":      "frontend-route",
				"namespace": "default",
				"uid":       "httproute-uid-123",
			},
			"spec": map[string]interface{}{
				"parentRefs": []interface{}{
					map[string]interface{}{
						"name":      "shared-gateway",
						"namespace": "gateway-system",
					},
				},
				"rules": []interface{}{
					map[string]interface{}{
						"backendRefs": []interface{}{
							map[string]interface{}{
								"name":      "backend-service",
								"namespace": "backend",
							},
						},
					},
				},
			},
		}

		crossNsJSON, err := json.Marshal(crossNsData)
		require.NoError(t, err)

		crossNsEvent := event
		crossNsEvent.Data = crossNsJSON

		lookup := extractors.NewMockResourceLookup()
		lookup.AddResource(&graph.ResourceIdentity{
			UID:       "gateway-uid-cross",
			Kind:      "Gateway",
			Namespace: "gateway-system",
			Name:      "shared-gateway",
		})
		lookup.AddResource(&graph.ResourceIdentity{
			UID:       "service-uid-cross",
			Kind:      "Service",
			Namespace: "backend",
			Name:      "backend-service",
		})

		edges, err := extractor.ExtractRelationships(context.Background(), crossNsEvent, lookup)
		require.NoError(t, err)
		require.Len(t, edges, 2)

		// Check parentRef has correct namespace
		var parentRefProps graph.ReferencesSpecEdge
		err = json.Unmarshal(edges[0].Properties, &parentRefProps)
		require.NoError(t, err)
		assert.Equal(t, "gateway-system", parentRefProps.RefNamespace)

		// Check backendRef has correct namespace
		var backendRefProps graph.ReferencesSpecEdge
		err = json.Unmarshal(edges[1].Properties, &backendRefProps)
		require.NoError(t, err)
		assert.Equal(t, "backend", backendRefProps.RefNamespace)
	})

	t.Run("handles missing spec", func(t *testing.T) {
		noSpecData := map[string]interface{}{
			"apiVersion": "gateway.networking.k8s.io/v1",
			"kind":       "HTTPRoute",
			"metadata": map[string]interface{}{
				"name":      "frontend-route",
				"namespace": "default",
				"uid":       "httproute-uid-123",
			},
		}

		noSpecJSON, err := json.Marshal(noSpecData)
		require.NoError(t, err)

		noSpecEvent := event
		noSpecEvent.Data = noSpecJSON

		lookup := extractors.NewMockResourceLookup()
		edges, err := extractor.ExtractRelationships(context.Background(), noSpecEvent, lookup)
		require.NoError(t, err)
		assert.Empty(t, edges)
	})
}
