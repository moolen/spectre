package externalsecrets

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graph/sync/extractors"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExternalSecretExtractor_Matches(t *testing.T) {
	extractor := NewExternalSecretExtractor()

	tests := []struct {
		name     string
		event    models.Event
		expected bool
	}{
		{
			name: "matches ExternalSecret",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "ExternalSecret",
					Group: "external-secrets.io",
				},
			},
			expected: true,
		},
		{
			name: "does not match SecretStore",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "SecretStore",
					Group: "external-secrets.io",
				},
			},
			expected: false,
		},
		{
			name: "does not match Secret",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "Secret",
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

func TestExternalSecretExtractor_ExtractRelationships(t *testing.T) {
	extractor := NewExternalSecretExtractor()

	t.Run("extracts secretStoreRef to SecretStore", func(t *testing.T) {
		esData := map[string]interface{}{
			"apiVersion": "external-secrets.io/v1beta1",
			"kind":       "ExternalSecret",
			"metadata": map[string]interface{}{
				"name":      "example-es",
				"namespace": "default",
				"uid":       "es-uid-123",
			},
			"spec": map[string]interface{}{
				"secretStoreRef": map[string]interface{}{
					"name": "vault-backend",
					"kind": "SecretStore",
				},
				"target": map[string]interface{}{
					"name": "my-secret",
				},
				"data": []interface{}{
					map[string]interface{}{
						"secretKey": "password",
						"remoteRef": map[string]interface{}{
							"key": "secret/data/myapp",
						},
					},
				},
			},
		}

		esJSON, err := json.Marshal(esData)
		require.NoError(t, err)

		event := models.Event{
			ID:   "event-1",
			Type: models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				UID:       "es-uid-123",
				Kind:      "ExternalSecret",
				Group:     "external-secrets.io",
				Version:   "v1beta1",
				Namespace: "default",
				Name:      "example-es",
			},
			Data:      esJSON,
			Timestamp: 1000000000,
		}

		lookup := extractors.NewMockResourceLookup()
		lookup.AddResource(&graph.ResourceIdentity{
			UID:       "store-uid-456",
			Kind:      "SecretStore",
			Namespace: "default",
			Name:      "vault-backend",
		})

		edges, err := extractor.ExtractRelationships(context.Background(), event, lookup)
		require.NoError(t, err)
		require.Len(t, edges, 1)

		edge := edges[0]
		assert.Equal(t, graph.EdgeTypeReferencesSpec, edge.Type)
		assert.Equal(t, "es-uid-123", edge.FromUID)
		assert.Equal(t, "store-uid-456", edge.ToUID)

		var props graph.ReferencesSpecEdge
		err = json.Unmarshal(edge.Properties, &props)
		require.NoError(t, err)
		assert.Equal(t, "spec.secretStoreRef", props.FieldPath)
		assert.Equal(t, "SecretStore", props.RefKind)
		assert.Equal(t, "vault-backend", props.RefName)
		assert.Equal(t, "default", props.RefNamespace)
	})

	t.Run("extracts secretStoreRef to ClusterSecretStore", func(t *testing.T) {
		esData := map[string]interface{}{
			"apiVersion": "external-secrets.io/v1beta1",
			"kind":       "ExternalSecret",
			"metadata": map[string]interface{}{
				"name":      "example-es",
				"namespace": "default",
				"uid":       "es-uid-123",
			},
			"spec": map[string]interface{}{
				"secretStoreRef": map[string]interface{}{
					"name": "global-vault",
					"kind": "ClusterSecretStore",
				},
				"target": map[string]interface{}{
					"name": "my-secret",
				},
			},
		}

		esJSON, err := json.Marshal(esData)
		require.NoError(t, err)

		event := models.Event{
			Resource: models.ResourceMetadata{
				UID:       "es-uid-123",
				Kind:      "ExternalSecret",
				Group:     "external-secrets.io",
				Namespace: "default",
				Name:      "example-es",
			},
			Data: esJSON,
		}

		lookup := extractors.NewMockResourceLookup()
		lookup.AddResource(&graph.ResourceIdentity{
			UID:       "clusterstore-uid-789",
			Kind:      "ClusterSecretStore",
			Namespace: "", // Cluster-scoped
			Name:      "global-vault",
		})

		edges, err := extractor.ExtractRelationships(context.Background(), event, lookup)
		require.NoError(t, err)
		require.Len(t, edges, 1)

		edge := edges[0]
		var props graph.ReferencesSpecEdge
		err = json.Unmarshal(edge.Properties, &props)
		require.NoError(t, err)
		assert.Equal(t, "ClusterSecretStore", props.RefKind)
		assert.Equal(t, "", props.RefNamespace) // Cluster-scoped
	})

	t.Run("extracts Secret relationship with high confidence", func(t *testing.T) {
		now := time.Now().UnixNano()
		esData := map[string]interface{}{
			"apiVersion": "external-secrets.io/v1beta1",
			"kind":       "ExternalSecret",
			"metadata": map[string]interface{}{
				"name":      "example-es",
				"namespace": "default",
				"uid":       "es-uid-123",
			},
			"spec": map[string]interface{}{
				"secretStoreRef": map[string]interface{}{
					"name": "vault-backend",
				},
				"target": map[string]interface{}{
					"name": "my-secret",
				},
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
					},
				},
			},
		}

		esJSON, err := json.Marshal(esData)
		require.NoError(t, err)

		event := models.Event{
			ID:   "event-1",
			Type: models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				UID:       "es-uid-123",
				Kind:      "ExternalSecret",
				Group:     "external-secrets.io",
				Version:   "v1beta1",
				Namespace: "default",
				Name:      "example-es",
			},
			Data:      esJSON,
			Timestamp: now,
		}

		lookup := extractors.NewMockResourceLookup()
		lookup.AddResource(&graph.ResourceIdentity{
			UID:       "store-uid-456",
			Kind:      "SecretStore",
			Namespace: "default",
			Name:      "vault-backend",
		})
		lookup.AddResource(&graph.ResourceIdentity{
			UID:       "secret-uid-789",
			Kind:      "Secret",
			Namespace: "default",
			Name:      "my-secret",
			Labels: map[string]string{
				"external-secrets.io/name": "example-es",
			},
			FirstSeen: now,
			LastSeen:  now + 1000000000, // 1 second later
		})

		edges, err := extractor.ExtractRelationships(context.Background(), event, lookup)
		require.NoError(t, err)
		require.Len(t, edges, 2) // secretStoreRef + Secret

		// Find the Secret edge
		var secretEdge *graph.Edge
		for _, e := range edges {
			if e.Type == graph.EdgeTypeCreatesObserved {
				secretEdge = &e
				break
			}
		}
		require.NotNil(t, secretEdge)

		assert.Equal(t, "es-uid-123", secretEdge.FromUID)
		assert.Equal(t, "secret-uid-789", secretEdge.ToUID)

		var props graph.ManagesEdge
		err = json.Unmarshal(secretEdge.Properties, &props)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, props.Confidence, 0.7) // High confidence
		assert.NotEmpty(t, props.Evidence)
	})

	t.Run("defaults target name to ExternalSecret name", func(t *testing.T) {
		now := time.Now().UnixNano()
		esData := map[string]interface{}{
			"apiVersion": "external-secrets.io/v1beta1",
			"kind":       "ExternalSecret",
			"metadata": map[string]interface{}{
				"name":      "example-es",
				"namespace": "default",
				"uid":       "es-uid-123",
			},
			"spec": map[string]interface{}{
				"secretStoreRef": map[string]interface{}{
					"name": "vault-backend",
				},
				// No target.name specified
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
					},
				},
			},
		}

		esJSON, err := json.Marshal(esData)
		require.NoError(t, err)

		event := models.Event{
			Type: models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				UID:       "es-uid-123",
				Kind:      "ExternalSecret",
				Group:     "external-secrets.io",
				Namespace: "default",
				Name:      "example-es",
			},
			Data:      esJSON,
			Timestamp: now,
		}

		lookup := extractors.NewMockResourceLookup()
		lookup.AddResource(&graph.ResourceIdentity{
			UID:       "store-uid-456",
			Kind:      "SecretStore",
			Namespace: "default",
			Name:      "vault-backend",
		})
		lookup.AddResource(&graph.ResourceIdentity{
			UID:       "secret-uid-789",
			Kind:      "Secret",
			Namespace: "default",
			Name:      "example-es", // Same as ExternalSecret name
			LastSeen:  now + 1000000000,
		})

		edges, err := extractor.ExtractRelationships(context.Background(), event, lookup)
		require.NoError(t, err)
		
		t.Logf("Got %d edges", len(edges))
		for i, e := range edges {
			t.Logf("Edge %d: %s %s -> %s", i, e.Type, e.FromUID, e.ToUID)
		}

		// Should have Secret edge
		var hasSecretEdge bool
		for _, e := range edges {
			if e.Type == graph.EdgeTypeCreatesObserved {
				hasSecretEdge = true
				assert.Equal(t, "secret-uid-789", e.ToUID)
			}
		}
		assert.True(t, hasSecretEdge)
	})

	t.Run("does not create Secret edge for deleted ExternalSecret", func(t *testing.T) {
		esData := map[string]interface{}{
			"apiVersion": "external-secrets.io/v1beta1",
			"kind":       "ExternalSecret",
			"metadata": map[string]interface{}{
				"name":      "example-es",
				"namespace": "default",
				"uid":       "es-uid-123",
			},
			"spec": map[string]interface{}{
				"secretStoreRef": map[string]interface{}{
					"name": "vault-backend",
				},
				"target": map[string]interface{}{
					"name": "my-secret",
				},
			},
		}

		esJSON, err := json.Marshal(esData)
		require.NoError(t, err)

		event := models.Event{
			Type: models.EventTypeDelete,
			Resource: models.ResourceMetadata{
				UID:       "es-uid-123",
				Kind:      "ExternalSecret",
				Group:     "external-secrets.io",
				Namespace: "default",
				Name:      "example-es",
			},
			Data: esJSON,
		}

		lookup := extractors.NewMockResourceLookup()

		edges, err := extractor.ExtractRelationships(context.Background(), event, lookup)
		require.NoError(t, err)

		// Should only have secretStoreRef edge, not Secret edge
		for _, edge := range edges {
			assert.NotEqual(t, graph.EdgeTypeCreatesObserved, edge.Type)
		}
	})
}
