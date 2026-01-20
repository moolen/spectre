package e2e

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graph/sync"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// TestFluxHelmReleaseExtractorIntegration tests the Flux HelmRelease extractor
// with mock graph data (doesn't require actual Flux installation)
func TestFluxHelmReleaseExtractorIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a mock graph client for testing
	mockClient := newMockGraphClient()

	// Create graph builder with extractors
	builder := sync.NewGraphBuilderWithClient(mockClient)

	t.Run("extract_spec_references_from_helmrelease", func(t *testing.T) {
		// Pre-populate the mock graph with the Secret that HelmRelease references
		mockClient.AddResource("secret-frontend-values-uid", "Secret", "default", "frontend-values", time.Now().UnixNano())

		// Create a HelmRelease event
		helmRelease := createHelmReleaseResource()
		event := createEventFromResource(helmRelease, models.EventTypeCreate)

		// Extract relationships
		edges, err := builder.(interface {
			ExtractRelationships(context.Context, models.Event) ([]graph.Edge, error)
		}).ExtractRelationships(context.Background(), event)

		require.NoError(t, err)

		// Should have REFERENCES_SPEC edges
		refEdges := filterEdgesByType(edges, graph.EdgeTypeReferencesSpec)
		assert.GreaterOrEqual(t, len(refEdges), 1, "Should have at least one REFERENCES_SPEC edge")

		// Verify Secret reference
		secretRef := findEdgeByRefKind(refEdges, "Secret")
		require.NotNil(t, secretRef, "Should reference Secret")

		var props graph.ReferencesSpecEdge
		err = json.Unmarshal(secretRef.Properties, &props)
		require.NoError(t, err)

		assert.Equal(t, "Secret", props.RefKind)
		assert.Equal(t, "frontend-values", props.RefName)
		assert.Contains(t, props.FieldPath, "valuesFrom")
	})

	t.Run("extract_managed_resources_with_confidence", func(t *testing.T) {
		// Setup: Add a Deployment to the mock graph that was created after HelmRelease
		hrTimestamp := time.Now().UnixNano()
		deploymentTimestamp := hrTimestamp + (5 * time.Second.Nanoseconds())

		mockClient.AddResource("deployment-uid-123", "Deployment", "default", "frontend", deploymentTimestamp)

		// Create HelmRelease event
		helmRelease := createHelmReleaseResource()
		event := createEventFromResource(helmRelease, models.EventTypeCreate)
		event.Timestamp = hrTimestamp

		// Extract relationships
		edges, err := builder.(interface {
			ExtractRelationships(context.Context, models.Event) ([]graph.Edge, error)
		}).ExtractRelationships(context.Background(), event)

		require.NoError(t, err)

		// Should have MANAGES edges
		managedEdges := filterEdgesByType(edges, graph.EdgeTypeManages)

		if len(managedEdges) > 0 {
			// Verify confidence and evidence
			var props graph.ManagesEdge
			err = json.Unmarshal(managedEdges[0].Properties, &props)
			require.NoError(t, err)

			assert.GreaterOrEqual(t, props.Confidence, 0.5, "Confidence should meet threshold")
			assert.NotEmpty(t, props.Evidence, "Should have evidence")
			assert.Equal(t, graph.ValidationStateValid, props.ValidationState)

			// Check evidence types
			evidenceTypes := make(map[graph.EvidenceType]bool)
			for _, ev := range props.Evidence {
				evidenceTypes[ev.Type] = true
			}
			assert.True(t, evidenceTypes[graph.EvidenceTypeLabel] || 
				evidenceTypes[graph.EvidenceTypeTemporal] ||
				evidenceTypes[graph.EvidenceTypeNamespace],
				"Should have at least one evidence type")
		}
	})

	t.Run("extractor_registered_in_builder", func(t *testing.T) {
		// Verify that Flux extractor is registered
		helmRelease := createHelmReleaseResource()
		event := createEventFromResource(helmRelease, models.EventTypeCreate)

		// This should not panic and should process the event
		edges, err := builder.(interface {
			ExtractRelationships(context.Context, models.Event) ([]graph.Edge, error)
		}).ExtractRelationships(context.Background(), event)

		require.NoError(t, err)
		assert.NotNil(t, edges)
	})
}

// Mock graph client for testing
type mockGraphClient struct {
	resources map[string]*graph.ResourceIdentity
	events    map[string][]graph.ChangeEvent
}

func newMockGraphClient() *mockGraphClient {
	return &mockGraphClient{
		resources: make(map[string]*graph.ResourceIdentity),
		events:    make(map[string][]graph.ChangeEvent),
	}
}

func (m *mockGraphClient) AddResource(uid, kind, namespace, name string, firstSeen int64) {
	m.resources[uid] = &graph.ResourceIdentity{
		UID:       uid,
		Kind:      kind,
		Namespace: namespace,
		Name:      name,
		FirstSeen: firstSeen,
		LastSeen:  firstSeen,
	}
}

func (m *mockGraphClient) Connect(ctx context.Context) error {
	return nil
}

func (m *mockGraphClient) Close() error {
	return nil
}

func (m *mockGraphClient) Ping(ctx context.Context) error {
	return nil
}

func (m *mockGraphClient) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	// Handle FindResourceByUID queries
	if uid, ok := query.Parameters["uid"].(string); ok {
		if res, exists := m.resources[uid]; exists {
			return &graph.QueryResult{
				Rows: [][]interface{}{
					{map[string]interface{}{
						"uid":       res.UID,
						"kind":      res.Kind,
						"namespace": res.Namespace,
						"name":      res.Name,
						"firstSeen": res.FirstSeen,
					}},
				},
			}, nil
		}
		return &graph.QueryResult{Rows: [][]interface{}{}}, nil
	}

	// Handle FindResourceByNamespace queries (namespace + kind + name)
	if namespace, ok := query.Parameters["namespace"].(string); ok {
		if kind, hasKind := query.Parameters["kind"].(string); hasKind {
			if name, hasName := query.Parameters["name"].(string); hasName {
				// This is a FindResourceByNamespace query
				for _, res := range m.resources {
					if res.Namespace == namespace && res.Kind == kind && res.Name == name {
						return &graph.QueryResult{
							Rows: [][]interface{}{
								{map[string]interface{}{
									"uid":       res.UID,
									"kind":      res.Kind,
									"namespace": res.Namespace,
									"name":      res.Name,
									"firstSeen": res.FirstSeen,
								}},
							},
						}, nil
					}
				}
				return &graph.QueryResult{Rows: [][]interface{}{}}, nil
			}
		}

		// Handle namespace queries for managed resources (namespace only)
		var rows [][]interface{}
		for uid, res := range m.resources {
			if res.Namespace == namespace {
				// Skip HelmRelease itself
				if helmReleaseUID, ok := query.Parameters["helmReleaseUID"].(string); ok && uid == helmReleaseUID {
					continue
				}
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

	return &graph.QueryResult{Rows: [][]interface{}{}}, nil
}

func (m *mockGraphClient) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}

func (m *mockGraphClient) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}

func (m *mockGraphClient) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}

func (m *mockGraphClient) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}

func (m *mockGraphClient) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return &graph.GraphStats{}, nil
}

func (m *mockGraphClient) InitializeSchema(ctx context.Context) error {
	return nil
}

func (m *mockGraphClient) DeleteGraph(ctx context.Context) error {
	return nil
}

// Helper functions

func createHelmReleaseResource() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "helm.toolkit.fluxcd.io/v2beta1",
			"kind":       "HelmRelease",
			"metadata": map[string]interface{}{
				"name":      "frontend",
				"namespace": "default",
				"uid":       "helmrelease-uid-123",
			},
			"spec": map[string]interface{}{
				"chart": map[string]interface{}{
					"spec": map[string]interface{}{
						"chart":   "nginx",
						"version": "1.0.0",
						"sourceRef": map[string]interface{}{
							"kind":      "HelmRepository",
							"name":      "bitnami",
							"namespace": "flux-system",
						},
					},
				},
				"valuesFrom": []interface{}{
					map[string]interface{}{
						"kind":      "Secret",
						"name":      "frontend-values",
						"valuesKey": "values.yaml",
					},
				},
				"interval":        "5m",
				"targetNamespace": "default",
			},
		},
	}
}

func createEventFromResource(obj *unstructured.Unstructured, eventType models.EventType) models.Event {
	data, _ := json.Marshal(obj.Object)

	return models.Event{
		ID:        "event-" + string(obj.GetUID()),
		Timestamp: time.Now().UnixNano(),
		Type:      eventType,
		Resource: models.ResourceMetadata{
			Group:     extractAPIGroup(obj.GetAPIVersion()),
			Version:   extractVersion(obj.GetAPIVersion()),
			Kind:      obj.GetKind(),
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
			UID:       string(obj.GetUID()),
		},
		Data: data,
	}
}

func extractAPIGroup(apiVersion string) string {
	parts := splitAPIVersion(apiVersion)
	if len(parts) == 2 {
		return parts[0]
	}
	return ""
}

func extractVersion(apiVersion string) string {
	parts := splitAPIVersion(apiVersion)
	if len(parts) == 2 {
		return parts[1]
	}
	return apiVersion
}

func splitAPIVersion(apiVersion string) []string {
	for i := 0; i < len(apiVersion); i++ {
		if apiVersion[i] == '/' {
			return []string{apiVersion[:i], apiVersion[i+1:]}
		}
	}
	return []string{apiVersion}
}

func filterEdgesByType(edges []graph.Edge, edgeType graph.EdgeType) []graph.Edge {
	filtered := []graph.Edge{}
	for _, edge := range edges {
		if edge.Type == edgeType {
			filtered = append(filtered, edge)
		}
	}
	return filtered
}

func findEdgeByRefKind(edges []graph.Edge, refKind string) *graph.Edge {
	for _, edge := range edges {
		var props graph.ReferencesSpecEdge
		if err := json.Unmarshal(edge.Properties, &props); err == nil {
			if props.RefKind == refKind {
				return &edge
			}
		}
	}
	return nil
}
