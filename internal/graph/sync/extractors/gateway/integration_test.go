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

// TestGatewayAPIIntegration tests the complete Gateway API flow
func TestGatewayAPIIntegration(t *testing.T) {
	// Setup mock lookup
	lookup := extractors.NewMockResourceLookup()

	// Add a GatewayClass
	lookup.AddResource(&graph.ResourceIdentity{
		UID:       "gatewayclass-uid-1",
		Kind:      "GatewayClass",
		Namespace: "",
		Name:      "istio",
	})

	// Add a Gateway
	lookup.AddResource(&graph.ResourceIdentity{
		UID:       "gateway-uid-1",
		Kind:      "Gateway",
		Namespace: "default",
		Name:      "my-gateway",
	})

	// Add Services
	lookup.AddResource(&graph.ResourceIdentity{
		UID:       "service-uid-1",
		Kind:      "Service",
		Namespace: "default",
		Name:      "frontend-service",
	})
	lookup.AddResource(&graph.ResourceIdentity{
		UID:       "service-uid-2",
		Kind:      "Service",
		Namespace: "default",
		Name:      "backend-service",
	})

	// Test 1: Gateway extraction
	t.Run("Gateway extracts GatewayClass reference", func(t *testing.T) {
		gatewayData := map[string]interface{}{
			"apiVersion": "gateway.networking.k8s.io/v1",
			"kind":       "Gateway",
			"metadata": map[string]interface{}{
				"name":      "my-gateway",
				"namespace": "default",
				"uid":       "gateway-uid-1",
			},
			"spec": map[string]interface{}{
				"gatewayClassName": "istio",
				"listeners": []interface{}{
					map[string]interface{}{
						"name":     "http",
						"protocol": "HTTP",
						"port":     80,
					},
				},
			},
		}

		gatewayJSON, err := json.Marshal(gatewayData)
		require.NoError(t, err)

		event := models.Event{
			ID:   "event-gateway-1",
			Type: models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				UID:       "gateway-uid-1",
				Kind:      "Gateway",
				Group:     "gateway.networking.k8s.io",
				Version:   "v1",
				Namespace: "default",
				Name:      "my-gateway",
			},
			Data:      gatewayJSON,
			Timestamp: 1000000000,
		}

		extractor := NewGatewayExtractor()
		edges, err := extractor.ExtractRelationships(context.Background(), event, lookup)
		require.NoError(t, err)
		require.Len(t, edges, 1)

		// Verify the edge
		edge := edges[0]
		assert.Equal(t, graph.EdgeTypeReferencesSpec, edge.Type)
		assert.Equal(t, "gateway-uid-1", edge.FromUID)
		assert.Equal(t, "gatewayclass-uid-1", edge.ToUID)
	})

	// Test 2: HTTPRoute extraction
	t.Run("HTTPRoute extracts Gateway and Service references", func(t *testing.T) {
		httpRouteData := map[string]interface{}{
			"apiVersion": "gateway.networking.k8s.io/v1",
			"kind":       "HTTPRoute",
			"metadata": map[string]interface{}{
				"name":      "frontend-route",
				"namespace": "default",
				"uid":       "httproute-uid-1",
			},
			"spec": map[string]interface{}{
				"parentRefs": []interface{}{
					map[string]interface{}{
						"name": "my-gateway",
					},
				},
				"rules": []interface{}{
					map[string]interface{}{
						"matches": []interface{}{
							map[string]interface{}{
								"path": map[string]interface{}{
									"type":  "PathPrefix",
									"value": "/api",
								},
							},
						},
						"backendRefs": []interface{}{
							map[string]interface{}{
								"name": "frontend-service",
								"port": 8080,
							},
						},
					},
					map[string]interface{}{
						"matches": []interface{}{
							map[string]interface{}{
								"path": map[string]interface{}{
									"type":  "PathPrefix",
									"value": "/admin",
								},
							},
						},
						"backendRefs": []interface{}{
							map[string]interface{}{
								"name": "backend-service",
								"port": 9090,
							},
						},
					},
				},
			},
		}

		httpRouteJSON, err := json.Marshal(httpRouteData)
		require.NoError(t, err)

		event := models.Event{
			ID:   "event-httproute-1",
			Type: models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				UID:       "httproute-uid-1",
				Kind:      "HTTPRoute",
				Group:     "gateway.networking.k8s.io",
				Version:   "v1",
				Namespace: "default",
				Name:      "frontend-route",
			},
			Data:      httpRouteJSON,
			Timestamp: 1000000000,
		}

		extractor := NewHTTPRouteExtractor()
		edges, err := extractor.ExtractRelationships(context.Background(), event, lookup)
		require.NoError(t, err)
		require.Len(t, edges, 3) // 1 parentRef + 2 backendRefs

		// Verify parentRef edge
		parentRefEdge := edges[0]
		assert.Equal(t, graph.EdgeTypeReferencesSpec, parentRefEdge.Type)
		assert.Equal(t, "httproute-uid-1", parentRefEdge.FromUID)
		assert.Equal(t, "gateway-uid-1", parentRefEdge.ToUID)

		var parentRefProps graph.ReferencesSpecEdge
		err = json.Unmarshal(parentRefEdge.Properties, &parentRefProps)
		require.NoError(t, err)
		assert.Equal(t, "spec.parentRefs[0]", parentRefProps.FieldPath)
		assert.Equal(t, "Gateway", parentRefProps.RefKind)

		// Verify first backendRef edge
		backendRef1Edge := edges[1]
		assert.Equal(t, graph.EdgeTypeReferencesSpec, backendRef1Edge.Type)
		assert.Equal(t, "httproute-uid-1", backendRef1Edge.FromUID)
		assert.Equal(t, "service-uid-1", backendRef1Edge.ToUID)

		var backendRef1Props graph.ReferencesSpecEdge
		err = json.Unmarshal(backendRef1Edge.Properties, &backendRef1Props)
		require.NoError(t, err)
		assert.Equal(t, "spec.rules[0].backendRefs[0]", backendRef1Props.FieldPath)
		assert.Equal(t, "Service", backendRef1Props.RefKind)
		assert.Equal(t, "frontend-service", backendRef1Props.RefName)

		// Verify second backendRef edge
		backendRef2Edge := edges[2]
		assert.Equal(t, graph.EdgeTypeReferencesSpec, backendRef2Edge.Type)
		assert.Equal(t, "httproute-uid-1", backendRef2Edge.FromUID)
		assert.Equal(t, "service-uid-2", backendRef2Edge.ToUID)

		var backendRef2Props graph.ReferencesSpecEdge
		err = json.Unmarshal(backendRef2Edge.Properties, &backendRef2Props)
		require.NoError(t, err)
		assert.Equal(t, "spec.rules[1].backendRefs[0]", backendRef2Props.FieldPath)
		assert.Equal(t, "Service", backendRef2Props.RefKind)
		assert.Equal(t, "backend-service", backendRef2Props.RefName)
	})

	// Test 3: Complete flow Gateway → HTTPRoute → Service
	t.Run("Complete Gateway API relationship chain", func(t *testing.T) {
		// This test demonstrates the complete relationship chain:
		// GatewayClass ← Gateway ← HTTPRoute → Service

		// Extract Gateway edges
		gatewayExtractor := NewGatewayExtractor()
		gatewayData := map[string]interface{}{
			"apiVersion": "gateway.networking.k8s.io/v1",
			"kind":       "Gateway",
			"metadata": map[string]interface{}{
				"name":      "my-gateway",
				"namespace": "default",
				"uid":       "gateway-uid-1",
			},
			"spec": map[string]interface{}{
				"gatewayClassName": "istio",
			},
		}
		gatewayJSON, _ := json.Marshal(gatewayData)
		gatewayEvent := models.Event{
			Resource: models.ResourceMetadata{
				UID:       "gateway-uid-1",
				Kind:      "Gateway",
				Group:     "gateway.networking.k8s.io",
				Namespace: "default",
				Name:      "my-gateway",
			},
			Data: gatewayJSON,
		}
		gatewayEdges, err := gatewayExtractor.ExtractRelationships(context.Background(), gatewayEvent, lookup)
		require.NoError(t, err)

		// Extract HTTPRoute edges
		httpRouteExtractor := NewHTTPRouteExtractor()
		httpRouteData := map[string]interface{}{
			"apiVersion": "gateway.networking.k8s.io/v1",
			"kind":       "HTTPRoute",
			"metadata": map[string]interface{}{
				"name":      "frontend-route",
				"namespace": "default",
				"uid":       "httproute-uid-1",
			},
			"spec": map[string]interface{}{
				"parentRefs": []interface{}{
					map[string]interface{}{
						"name": "my-gateway",
					},
				},
				"rules": []interface{}{
					map[string]interface{}{
						"backendRefs": []interface{}{
							map[string]interface{}{
								"name": "frontend-service",
							},
						},
					},
				},
			},
		}
		httpRouteJSON, _ := json.Marshal(httpRouteData)
		httpRouteEvent := models.Event{
			Resource: models.ResourceMetadata{
				UID:       "httproute-uid-1",
				Kind:      "HTTPRoute",
				Group:     "gateway.networking.k8s.io",
				Namespace: "default",
				Name:      "frontend-route",
			},
			Data: httpRouteJSON,
		}
		httpRouteEdges, err := httpRouteExtractor.ExtractRelationships(context.Background(), httpRouteEvent, lookup)
		require.NoError(t, err)

		// Verify the relationship chain
		assert.Len(t, gatewayEdges, 1)   // Gateway → GatewayClass
		assert.Len(t, httpRouteEdges, 2) // HTTPRoute → Gateway + HTTPRoute → Service

		// Verify the chain: GatewayClass ← Gateway ← HTTPRoute → Service
		assert.Equal(t, "gateway-uid-1", gatewayEdges[0].FromUID)
		assert.Equal(t, "gatewayclass-uid-1", gatewayEdges[0].ToUID)

		assert.Equal(t, "httproute-uid-1", httpRouteEdges[0].FromUID)
		assert.Equal(t, "gateway-uid-1", httpRouteEdges[0].ToUID)

		assert.Equal(t, "httproute-uid-1", httpRouteEdges[1].FromUID)
		assert.Equal(t, "service-uid-1", httpRouteEdges[1].ToUID)
	})
}
