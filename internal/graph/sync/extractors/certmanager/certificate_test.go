package certmanager

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

func TestCertificateExtractor_Matches(t *testing.T) {
	extractor := NewCertificateExtractor()

	tests := []struct {
		name     string
		event    models.Event
		expected bool
	}{
		{
			name: "matches Certificate",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "Certificate",
					Group: "cert-manager.io",
				},
			},
			expected: true,
		},
		{
			name: "does not match Issuer",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "Issuer",
					Group: "cert-manager.io",
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

func TestCertificateExtractor_ExtractRelationships(t *testing.T) {
	extractor := NewCertificateExtractor()

	t.Run("extracts issuerRef to Issuer", func(t *testing.T) {
		certData := map[string]interface{}{
			"apiVersion": "cert-manager.io/v1",
			"kind":       "Certificate",
			"metadata": map[string]interface{}{
				"name":      "example-cert",
				"namespace": "default",
				"uid":       "cert-uid-123",
			},
			"spec": map[string]interface{}{
				"secretName": "example-tls",
				"issuerRef": map[string]interface{}{
					"name": "letsencrypt-prod",
					"kind": "Issuer",
				},
				"dnsNames": []interface{}{
					"example.com",
				},
			},
		}

		certJSON, err := json.Marshal(certData)
		require.NoError(t, err)

		event := models.Event{
			ID:   "event-1",
			Type: models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				UID:       "cert-uid-123",
				Kind:      "Certificate",
				Group:     "cert-manager.io",
				Version:   "v1",
				Namespace: "default",
				Name:      "example-cert",
			},
			Data:      certJSON,
			Timestamp: 1000000000,
		}

		lookup := extractors.NewMockResourceLookup()
		lookup.AddResource(&graph.ResourceIdentity{
			UID:       "issuer-uid-456",
			Kind:      "Issuer",
			Namespace: "default",
			Name:      "letsencrypt-prod",
		})

		edges, err := extractor.ExtractRelationships(context.Background(), event, lookup)
		require.NoError(t, err)
		require.Len(t, edges, 1)

		edge := edges[0]
		assert.Equal(t, graph.EdgeTypeReferencesSpec, edge.Type)
		assert.Equal(t, "cert-uid-123", edge.FromUID)
		assert.Equal(t, "issuer-uid-456", edge.ToUID)

		var props graph.ReferencesSpecEdge
		err = json.Unmarshal(edge.Properties, &props)
		require.NoError(t, err)
		assert.Equal(t, "spec.issuerRef", props.FieldPath)
		assert.Equal(t, "Issuer", props.RefKind)
		assert.Equal(t, "letsencrypt-prod", props.RefName)
		assert.Equal(t, "default", props.RefNamespace)
	})

	t.Run("extracts issuerRef to ClusterIssuer", func(t *testing.T) {
		certData := map[string]interface{}{
			"apiVersion": "cert-manager.io/v1",
			"kind":       "Certificate",
			"metadata": map[string]interface{}{
				"name":      "example-cert",
				"namespace": "default",
				"uid":       "cert-uid-123",
			},
			"spec": map[string]interface{}{
				"secretName": "example-tls",
				"issuerRef": map[string]interface{}{
					"name": "letsencrypt-prod",
					"kind": "ClusterIssuer",
				},
			},
		}

		certJSON, err := json.Marshal(certData)
		require.NoError(t, err)

		event := models.Event{
			ID:   "event-1",
			Type: models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				UID:       "cert-uid-123",
				Kind:      "Certificate",
				Group:     "cert-manager.io",
				Version:   "v1",
				Namespace: "default",
				Name:      "example-cert",
			},
			Data:      certJSON,
			Timestamp: 1000000000,
		}

		lookup := extractors.NewMockResourceLookup()
		lookup.AddResource(&graph.ResourceIdentity{
			UID:       "clusterissuer-uid-789",
			Kind:      "ClusterIssuer",
			Namespace: "", // Cluster-scoped
			Name:      "letsencrypt-prod",
		})

		edges, err := extractor.ExtractRelationships(context.Background(), event, lookup)
		require.NoError(t, err)
		require.Len(t, edges, 1)

		edge := edges[0]
		var props graph.ReferencesSpecEdge
		err = json.Unmarshal(edge.Properties, &props)
		require.NoError(t, err)
		assert.Equal(t, "ClusterIssuer", props.RefKind)
		assert.Equal(t, "", props.RefNamespace) // Cluster-scoped
	})

	t.Run("extracts Secret relationship with high confidence", func(t *testing.T) {
		now := time.Now().UnixNano()
		certData := map[string]interface{}{
			"apiVersion": "cert-manager.io/v1",
			"kind":       "Certificate",
			"metadata": map[string]interface{}{
				"name":      "example-cert",
				"namespace": "default",
				"uid":       "cert-uid-123",
			},
			"spec": map[string]interface{}{
				"secretName": "example-tls",
				"issuerRef": map[string]interface{}{
					"name": "letsencrypt-prod",
					"kind": "Issuer",
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

		certJSON, err := json.Marshal(certData)
		require.NoError(t, err)

		event := models.Event{
			ID:   "event-1",
			Type: models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				UID:       "cert-uid-123",
				Kind:      "Certificate",
				Group:     "cert-manager.io",
				Version:   "v1",
				Namespace: "default",
				Name:      "example-cert",
			},
			Data:      certJSON,
			Timestamp: now,
		}

		lookup := extractors.NewMockResourceLookup()
		lookup.AddResource(&graph.ResourceIdentity{
			UID:       "issuer-uid-456",
			Kind:      "Issuer",
			Namespace: "default",
			Name:      "letsencrypt-prod",
		})
		lookup.AddResource(&graph.ResourceIdentity{
			UID:       "secret-uid-789",
			Kind:      "Secret",
			Namespace: "default",
			Name:      "example-tls",
			Labels: map[string]string{
				"cert-manager.io/certificate-name": "example-cert",
			},
			FirstSeen: now,
			LastSeen:  now + 1000000000, // 1 second later
		})

		edges, err := extractor.ExtractRelationships(context.Background(), event, lookup)
		require.NoError(t, err)
		require.Len(t, edges, 2) // issuerRef + Secret

		// Find the Secret edge
		var secretEdge *graph.Edge
		for _, e := range edges {
			if e.Type == graph.EdgeTypeCreatesObserved {
				secretEdge = &e
				break
			}
		}
		require.NotNil(t, secretEdge)

		assert.Equal(t, "cert-uid-123", secretEdge.FromUID)
		assert.Equal(t, "secret-uid-789", secretEdge.ToUID)

		var props graph.ManagesEdge
		err = json.Unmarshal(secretEdge.Properties, &props)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, props.Confidence, 0.7) // High confidence
		assert.NotEmpty(t, props.Evidence)
	})

	t.Run("handles missing spec", func(t *testing.T) {
		certData := map[string]interface{}{
			"apiVersion": "cert-manager.io/v1",
			"kind":       "Certificate",
			"metadata": map[string]interface{}{
				"name":      "example-cert",
				"namespace": "default",
				"uid":       "cert-uid-123",
			},
		}

		certJSON, err := json.Marshal(certData)
		require.NoError(t, err)

		event := models.Event{
			Resource: models.ResourceMetadata{
				UID:       "cert-uid-123",
				Kind:      "Certificate",
				Group:     "cert-manager.io",
				Namespace: "default",
				Name:      "example-cert",
			},
			Data: certJSON,
		}

		lookup := extractors.NewMockResourceLookup()
		edges, err := extractor.ExtractRelationships(context.Background(), event, lookup)
		require.NoError(t, err)
		assert.Empty(t, edges)
	})

	t.Run("does not create Secret edge for deleted Certificate", func(t *testing.T) {
		certData := map[string]interface{}{
			"apiVersion": "cert-manager.io/v1",
			"kind":       "Certificate",
			"metadata": map[string]interface{}{
				"name":      "example-cert",
				"namespace": "default",
				"uid":       "cert-uid-123",
			},
			"spec": map[string]interface{}{
				"secretName": "example-tls",
				"issuerRef": map[string]interface{}{
					"name": "letsencrypt-prod",
				},
			},
		}

		certJSON, err := json.Marshal(certData)
		require.NoError(t, err)

		event := models.Event{
			Type: models.EventTypeDelete,
			Resource: models.ResourceMetadata{
				UID:       "cert-uid-123",
				Kind:      "Certificate",
				Group:     "cert-manager.io",
				Namespace: "default",
				Name:      "example-cert",
			},
			Data: certJSON,
		}

		lookup := extractors.NewMockResourceLookup()
		lookup.AddResource(&graph.ResourceIdentity{
			UID:       "secret-uid-789",
			Kind:      "Secret",
			Namespace: "default",
			Name:      "example-tls",
		})

		edges, err := extractor.ExtractRelationships(context.Background(), event, lookup)
		require.NoError(t, err)

		// Should only have issuerRef edge, not Secret edge
		for _, edge := range edges {
			assert.NotEqual(t, graph.EdgeTypeCreatesObserved, edge.Type)
		}
	})
}

func TestCertificateExtractor_ScoreSecretRelationship(t *testing.T) {
	extractor := NewCertificateExtractor()
	now := time.Now().UnixNano()

	tests := []struct {
		name           string
		certData       map[string]interface{}
		secret         *graph.ResourceIdentity
		certTimestamp  int64
		expectedMinConf float64
		expectedMaxConf float64
	}{
		{
			name: "high confidence with annotation and temporal match",
			certData: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type":   "Ready",
							"status": "True",
						},
					},
				},
			},
			secret: &graph.ResourceIdentity{
				UID:       "secret-uid",
				Name:      "example-tls",
				Namespace: "default",
				Labels: map[string]string{
					"cert-manager.io/certificate-name": "example-cert",
				},
				FirstSeen: now,
				LastSeen:  now + 1000000000, // 1 second later
			},
			certTimestamp:  now,
			expectedMinConf: 0.8,
			expectedMaxConf: 1.0,
		},
		{
			name: "medium confidence with name match only",
			certData: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type":   "Ready",
							"status": "True",
						},
					},
				},
			},
			secret: &graph.ResourceIdentity{
				UID:       "secret-uid",
				Name:      "example-cert-tls",
				Namespace: "default",
				LastSeen:  now + 120000000000, // 2 minutes later (no temporal score)
			},
			certTimestamp:  now,
			expectedMinConf: 0.7, // name match (0.5) + namespace (0.3) = 0.8
			expectedMaxConf: 0.9,
		},
		{
			name: "low confidence without matches",
			certData: map[string]interface{}{},
			secret: &graph.ResourceIdentity{
				UID:       "secret-uid",
				Name:      "other-secret",
				Namespace: "different-namespace",
				LastSeen:  now + 3600000000000, // 1 hour later
			},
			certTimestamp:  now,
			expectedMinConf: 0.0,
			expectedMaxConf: 0.3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := models.Event{
				Resource: models.ResourceMetadata{
					UID:       "cert-uid",
					Name:      "example-cert",
					Namespace: "default",
				},
				Timestamp: tt.certTimestamp,
			}

			confidence, evidence := extractor.scoreSecretRelationship(
				context.Background(),
				event,
				tt.certData,
				tt.secret,
			)

			assert.GreaterOrEqual(t, confidence, tt.expectedMinConf)
			assert.LessOrEqual(t, confidence, tt.expectedMaxConf)
			assert.NotNil(t, evidence)
		})
	}
}
