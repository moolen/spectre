package extractors

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock ResourceLookup for testing
type mockResourceLookup struct {
	resources map[string]*graph.ResourceIdentity
	events    map[string][]graph.ChangeEvent
}

func newMockResourceLookup() *mockResourceLookup {
	return &mockResourceLookup{
		resources: make(map[string]*graph.ResourceIdentity),
		events:    make(map[string][]graph.ChangeEvent),
	}
}

func (m *mockResourceLookup) AddResource(uid, kind, namespace, name string, firstSeen int64) {
	m.resources[uid] = &graph.ResourceIdentity{
		UID:       uid,
		Kind:      kind,
		Namespace: namespace,
		Name:      name,
		FirstSeen: firstSeen,
	}
}

func (m *mockResourceLookup) FindResourceByUID(ctx context.Context, uid string) (*graph.ResourceIdentity, error) {
	if res, ok := m.resources[uid]; ok {
		return res, nil
	}
	return nil, nil
}

func (m *mockResourceLookup) FindResourceByNamespace(ctx context.Context, namespace, kind, name string) (*graph.ResourceIdentity, error) {
	for _, res := range m.resources {
		if res.Namespace == namespace && res.Kind == kind && res.Name == name {
			return res, nil
		}
	}
	return nil, nil
}

func (m *mockResourceLookup) FindRecentEvents(ctx context.Context, uid string, windowNs int64) ([]graph.ChangeEvent, error) {
	return m.events[uid], nil
}

func (m *mockResourceLookup) QueryGraph(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	// For testing, return resources in the queried namespace
	namespace := query.Parameters["namespace"].(string)
	helmReleaseUID := query.Parameters["helmReleaseUID"].(string)

	var rows [][]interface{}
	for uid, res := range m.resources {
		if res.Namespace == namespace && uid != helmReleaseUID {
			rows = append(rows, []interface{}{
				map[string]interface{}{
					"uid":       res.UID,
					"kind":      res.Kind,
					"namespace": res.Namespace,
					"name":      res.Name,
					"firstSeen": res.FirstSeen,
				},
			})
		}
	}

	return &graph.QueryResult{Rows: rows}, nil
}

func TestFluxHelmReleaseExtractor_Matches(t *testing.T) {
	extractor := NewFluxHelmReleaseExtractor()

	tests := []struct {
		name     string
		event    models.Event
		expected bool
	}{
		{
			name: "Flux HelmRelease",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Group: "helm.toolkit.fluxcd.io",
					Kind:  "HelmRelease",
				},
			},
			expected: true,
		},
		{
			name: "Regular Deployment",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Group: "apps",
					Kind:  "Deployment",
				},
			},
			expected: false,
		},
		{
			name: "Wrong API group",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Group: "source.toolkit.fluxcd.io",
					Kind:  "HelmRelease",
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

func TestFluxHelmReleaseExtractor_ExtractSpecReferences(t *testing.T) {
	tests := []struct {
		name          string
		helmRelease   map[string]interface{}
		expectedEdges int
		expectedKinds []string
		setupLookup   func(*mockResourceLookup)
	}{
		{
			name: "valuesFrom references Secret",
			helmRelease: map[string]interface{}{
				"spec": map[string]interface{}{
					"valuesFrom": []interface{}{
						map[string]interface{}{
							"kind": "Secret",
							"name": "my-values",
						},
					},
				},
			},
			expectedEdges: 1,
			expectedKinds: []string{"Secret"},
			setupLookup: func(m *mockResourceLookup) {
				m.AddResource("secret-uid-123", "Secret", "default", "my-values", time.Now().UnixNano())
			},
		},
		{
			name: "multiple references",
			helmRelease: map[string]interface{}{
				"spec": map[string]interface{}{
					"valuesFrom": []interface{}{
						map[string]interface{}{
							"kind": "Secret",
							"name": "values-1",
						},
						map[string]interface{}{
							"kind": "ConfigMap",
							"name": "values-2",
						},
					},
					"chart": map[string]interface{}{
						"spec": map[string]interface{}{
							"sourceRef": map[string]interface{}{
								"kind": "HelmRepository",
								"name": "bitnami",
							},
						},
					},
				},
			},
			expectedEdges: 3,
			expectedKinds: []string{"Secret", "ConfigMap", "HelmRepository"},
			setupLookup: func(m *mockResourceLookup) {
				m.AddResource("secret-uid", "Secret", "default", "values-1", time.Now().UnixNano())
				m.AddResource("cm-uid", "ConfigMap", "default", "values-2", time.Now().UnixNano())
				m.AddResource("repo-uid", "HelmRepository", "default", "bitnami", time.Now().UnixNano())
			},
		},
		{
			name: "kubeConfig secretRef",
			helmRelease: map[string]interface{}{
				"spec": map[string]interface{}{
					"kubeConfig": map[string]interface{}{
						"secretRef": map[string]interface{}{
							"name": "remote-kubeconfig",
						},
					},
				},
			},
			expectedEdges: 1,
			expectedKinds: []string{"Secret"},
			setupLookup: func(m *mockResourceLookup) {
				m.AddResource("kubeconfig-secret", "Secret", "default", "remote-kubeconfig", time.Now().UnixNano())
			},
		},
		{
			name: "no references",
			helmRelease: map[string]interface{}{
				"spec": map[string]interface{}{
					"chart": map[string]interface{}{
						"spec": map[string]interface{}{
							"chart": "nginx",
						},
					},
				},
			},
			expectedEdges: 0,
			expectedKinds: []string{},
			setupLookup:   func(m *mockResourceLookup) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewFluxHelmReleaseExtractor()

			lookup := newMockResourceLookup()
			tt.setupLookup(lookup)

			event := models.Event{
				Type: models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					Group:     "helm.toolkit.fluxcd.io",
					Version:   "v2beta1",
					Kind:      "HelmRelease",
					Namespace: "default",
					Name:      "test-release",
					UID:       "release-uid-123",
				},
				Data: marshalJSON(tt.helmRelease),
			}

			edges, err := extractor.ExtractRelationships(context.Background(), event, lookup)

			require.NoError(t, err)
			assert.Len(t, edges, tt.expectedEdges)

			// Verify edge kinds
			if tt.expectedEdges > 0 {
				kinds := extractRefKinds(edges)
				assert.ElementsMatch(t, tt.expectedKinds, kinds)
			}
		})
	}
}

func TestFluxHelmReleaseExtractor_ConfidenceScoring(t *testing.T) {
	tests := []struct {
		name               string
		setupLookup        func(*mockResourceLookup, int64)
		expectedConfidence struct{ min, max float64 }
	}{
		{
			name: "all evidence present",
			setupLookup: func(m *mockResourceLookup, hrTimestamp int64) {
				// Resource created 5 seconds after HelmRelease
				deploymentTime := hrTimestamp + (5 * int64(time.Second.Nanoseconds()))
				m.AddResource("deploy-uid", "Deployment", "default", "frontend", deploymentTime)

				// Add reconcile event
				m.events["helm-release-uid"] = []graph.ChangeEvent{
					{
						ID:        "event-1",
						Timestamp: hrTimestamp,
						EventType: "UPDATE",
						Status:    "Ready",
					},
				}
			},
			expectedConfidence: struct{ min, max float64 }{min: 0.85, max: 1.0},
		},
		{
			name: "label match only",
			setupLookup: func(m *mockResourceLookup, hrTimestamp int64) {
				// Resource created long ago (no temporal evidence)
				oldTime := hrTimestamp - (60 * int64(time.Second.Nanoseconds()))
				m.AddResource("deploy-uid", "Deployment", "default", "frontend", oldTime)
			},
			expectedConfidence: struct{ min, max float64 }{min: 0.4, max: 0.6},
		},
		{
			name: "no evidence",
			setupLookup: func(m *mockResourceLookup, hrTimestamp int64) {
				// Resource with different name, created long ago
				oldTime := hrTimestamp - (60 * int64(time.Second.Nanoseconds()))
				m.AddResource("deploy-uid", "Deployment", "default", "backend", oldTime)
			},
			expectedConfidence: struct{ min, max float64 }{min: 0.0, max: 0.2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewFluxHelmReleaseExtractor()

			hrTimestamp := time.Now().UnixNano()
			lookup := newMockResourceLookup()
			tt.setupLookup(lookup, hrTimestamp)

			helmRelease := map[string]interface{}{
				"spec": map[string]interface{}{
					"chart": map[string]interface{}{
						"spec": map[string]interface{}{
							"chart": "frontend",
						},
					},
				},
			}

			event := models.Event{
				Type:      models.EventTypeCreate,
				Timestamp: hrTimestamp,
				Resource: models.ResourceMetadata{
					Group:     "helm.toolkit.fluxcd.io",
					Version:   "v2beta1",
					Kind:      "HelmRelease",
					Namespace: "default",
					Name:      "frontend",
					UID:       "helm-release-uid",
				},
				Data: marshalJSON(helmRelease),
			}

			edges, err := extractor.ExtractRelationships(context.Background(), event, lookup)

			require.NoError(t, err)

			if tt.expectedConfidence.max > 0 {
				// Should have at least one MANAGES edge
				managedEdges := filterManagesEdges(edges)
				if len(managedEdges) > 0 {
					edge := managedEdges[0]
					var props graph.ManagesEdge
					err := json.Unmarshal(edge.Properties, &props)
					require.NoError(t, err)

					assert.GreaterOrEqual(t, props.Confidence, tt.expectedConfidence.min)
					assert.LessOrEqual(t, props.Confidence, tt.expectedConfidence.max)
					assert.NotEmpty(t, props.Evidence, "Should provide evidence items")
				}
			}
		})
	}
}

func TestFluxHelmReleaseExtractor_TargetNamespace(t *testing.T) {
	extractor := NewFluxHelmReleaseExtractor()

	lookup := newMockResourceLookup()
	hrTimestamp := time.Now().UnixNano()
	deployTime := hrTimestamp + (3 * int64(time.Second.Nanoseconds()))

	// Add resource in production namespace
	lookup.AddResource("deploy-uid", "Deployment", "production", "frontend", deployTime)

	helmRelease := map[string]interface{}{
		"spec": map[string]interface{}{
			"targetNamespace": "production", // Deploy to different namespace
			"chart": map[string]interface{}{
				"spec": map[string]interface{}{
					"chart": "frontend",
				},
			},
		},
	}

	event := models.Event{
		Type:      models.EventTypeCreate,
		Timestamp: hrTimestamp,
		Resource: models.ResourceMetadata{
			Group:     "helm.toolkit.fluxcd.io",
			Version:   "v2beta1",
			Kind:      "HelmRelease",
			Namespace: "flux-system", // HelmRelease in different namespace
			Name:      "frontend",
			UID:       "helm-release-uid",
		},
		Data: marshalJSON(helmRelease),
	}

	edges, err := extractor.ExtractRelationships(context.Background(), event, lookup)

	require.NoError(t, err)

	// Should find managed resource in production namespace
	managedEdges := filterManagesEdges(edges)
	assert.Greater(t, len(managedEdges), 0, "Should find managed resources in target namespace")
}

// Helper functions

func marshalJSON(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func extractRefKinds(edges []graph.Edge) []string {
	kinds := []string{}
	for _, edge := range edges {
		if edge.Type == graph.EdgeTypeReferencesSpec {
			var props graph.ReferencesSpecEdge
			if err := json.Unmarshal(edge.Properties, &props); err == nil {
				kinds = append(kinds, props.RefKind)
			}
		}
	}
	return kinds
}

func filterManagesEdges(edges []graph.Edge) []graph.Edge {
	managed := []graph.Edge{}
	for _, edge := range edges {
		if edge.Type == graph.EdgeTypeManages {
			managed = append(managed, edge)
		}
	}
	return managed
}
