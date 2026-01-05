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

func TestGatewayExtractor_Matches(t *testing.T) {
	extractor := NewGatewayExtractor()

	tests := []struct {
		name     string
		event    models.Event
		expected bool
	}{
		{
			name: "matches Gateway",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "Gateway",
					Group: "gateway.networking.k8s.io",
				},
			},
			expected: true,
		},
		{
			name: "does not match HTTPRoute",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "HTTPRoute",
					Group: "gateway.networking.k8s.io",
				},
			},
			expected: false,
		},
		{
			name: "does not match Service",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "Service",
					Group: "",
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

func TestGatewayExtractor_ExtractRelationships(t *testing.T) {
	extractor := NewGatewayExtractor()

	gatewayData := map[string]interface{}{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "Gateway",
		"metadata": map[string]interface{}{
			"name":      "my-gateway",
			"namespace": "default",
			"uid":       "gateway-uid-123",
		},
		"spec": map[string]interface{}{
			"gatewayClassName": "istio",
		},
	}

	gatewayJSON, err := json.Marshal(gatewayData)
	require.NoError(t, err)

	event := models.Event{
		ID:   "event-1",
		Type: models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			UID:       "gateway-uid-123",
			Kind:      "Gateway",
			Group:     "gateway.networking.k8s.io",
			Version:   "v1",
			Namespace: "default",
			Name:      "my-gateway",
		},
		Data:      gatewayJSON,
		Timestamp: 1000000000,
	}

	t.Run("extracts gatewayClassName reference", func(t *testing.T) {
		lookup := extractors.NewMockResourceLookup()
		lookup.AddResource(&graph.ResourceIdentity{
			UID:       "gatewayclass-uid-456",
			Kind:      "GatewayClass",
			Namespace: "",
			Name:      "istio",
		})

		edges, err := extractor.ExtractRelationships(context.Background(), event, lookup)
		require.NoError(t, err)
		require.Len(t, edges, 1)

		edge := edges[0]
		assert.Equal(t, graph.EdgeTypeReferencesSpec, edge.Type)
		assert.Equal(t, "gateway-uid-123", edge.FromUID)
		assert.Equal(t, "gatewayclass-uid-456", edge.ToUID)

		var props graph.ReferencesSpecEdge
		err = json.Unmarshal(edge.Properties, &props)
		require.NoError(t, err)
		assert.Equal(t, "spec.gatewayClassName", props.FieldPath)
		assert.Equal(t, "GatewayClass", props.RefKind)
		assert.Equal(t, "istio", props.RefName)
		assert.Equal(t, "", props.RefNamespace) // Cluster-scoped
	})

	t.Run("handles missing gatewayClassName", func(t *testing.T) {
		gatewayDataNoClass := map[string]interface{}{
			"apiVersion": "gateway.networking.k8s.io/v1",
			"kind":       "Gateway",
			"metadata": map[string]interface{}{
				"name":      "my-gateway",
				"namespace": "default",
				"uid":       "gateway-uid-123",
			},
			"spec": map[string]interface{}{},
		}

		gatewayJSONNoClass, err := json.Marshal(gatewayDataNoClass)
		require.NoError(t, err)

		eventNoClass := event
		eventNoClass.Data = gatewayJSONNoClass

		lookup := extractors.NewMockResourceLookup()
		edges, err := extractor.ExtractRelationships(context.Background(), eventNoClass, lookup)
		require.NoError(t, err)
		assert.Empty(t, edges)
	})

	t.Run("skips edge creation when GatewayClass not found", func(t *testing.T) {
		lookup := extractors.NewMockResourceLookup()
		// Don't add the GatewayClass resource

		edges, err := extractor.ExtractRelationships(context.Background(), event, lookup)
		require.NoError(t, err)
		// Edge should be skipped when target resource (GatewayClass) is not found
		assert.Empty(t, edges)
	})
}
