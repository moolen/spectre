package native

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockResourceLookup provides a mock implementation of ResourceLookup for testing
type MockResourceLookup struct {
	resources   map[string]*graph.ResourceIdentity
	queryResult *graph.QueryResult
	queryError  error
}

func (m *MockResourceLookup) FindResourceByUID(_ context.Context, uid string) (*graph.ResourceIdentity, error) {
	if res, ok := m.resources[uid]; ok {
		return res, nil
	}
	return nil, nil
}

func (m *MockResourceLookup) FindResourceByNamespace(_ context.Context, namespace, kind, name string) (*graph.ResourceIdentity, error) {
	key := namespace + "/" + kind + "/" + name
	if res, ok := m.resources[key]; ok {
		return res, nil
	}
	return nil, nil
}

func (m *MockResourceLookup) FindRecentEvents(_ context.Context, _ string, _ int64) ([]graph.ChangeEvent, error) {
	return nil, nil
}

func (m *MockResourceLookup) QueryGraph(_ context.Context, _ graph.GraphQuery) (*graph.QueryResult, error) {
	if m.queryError != nil {
		return nil, m.queryError
	}
	return m.queryResult, nil
}

func TestServiceExtractor_Matches(t *testing.T) {
	extractor := NewServiceExtractor()

	tests := []struct {
		name     string
		event    models.Event
		expected bool
	}{
		{
			name: "matches Service",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "Service",
					Group: "",
				},
			},
			expected: true,
		},
		{
			name: "does not match Deployment",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "Deployment",
					Group: "apps",
				},
			},
			expected: false,
		},
		{
			name: "does not match Service with group",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "Service",
					Group: "custom.io",
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

func TestServiceExtractor_ExtractRelationships(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		serviceData   map[string]interface{}
		mockQueryResult *graph.QueryResult
		expectedEdges int
		expectError   bool
	}{
		{
			name: "service with matching pods",
			serviceData: map[string]interface{}{
				"spec": map[string]interface{}{
					"selector": map[string]interface{}{
						"app": "frontend",
					},
				},
			},
			mockQueryResult: &graph.QueryResult{
				Rows: [][]interface{}{
					{"pod-1", `{"app":"frontend"}`, false},
					{"pod-2", `{"app":"frontend","other":"label"}`, false},
				},
			},
			expectedEdges: 2,
			expectError:   false,
		},
		{
			name: "service with no selector",
			serviceData: map[string]interface{}{
				"spec": map[string]interface{}{},
			},
			mockQueryResult: &graph.QueryResult{
				Rows: [][]interface{}{},
			},
			expectedEdges: 0,
			expectError:   false,
		},
		{
			name: "service with empty selector",
			serviceData: map[string]interface{}{
				"spec": map[string]interface{}{
					"selector": map[string]interface{}{},
				},
			},
			mockQueryResult: &graph.QueryResult{
				Rows: [][]interface{}{},
			},
			expectedEdges: 0,
			expectError:   false,
		},
		{
			name: "service with multiple selector labels",
			serviceData: map[string]interface{}{
				"spec": map[string]interface{}{
					"selector": map[string]interface{}{
						"app":  "frontend",
						"tier": "web",
					},
				},
			},
			mockQueryResult: &graph.QueryResult{
				Rows: [][]interface{}{
					{"pod-1", `{"app":"frontend","tier":"web"}`, false},
				},
			},
			expectedEdges: 1,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewServiceExtractor()

			serviceJSON, err := json.Marshal(tt.serviceData)
			require.NoError(t, err)

			event := models.Event{
				Resource: models.ResourceMetadata{
					UID:       "service-uid",
					Kind:      "Service",
					Namespace: "default",
					Name:      "test-service",
				},
				Data: serviceJSON,
				Type: models.EventTypeUpdate,
			}

			lookup := &MockResourceLookup{
				queryResult: tt.mockQueryResult,
			}

			edges, err := extractor.ExtractRelationships(ctx, event, lookup)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, edges, tt.expectedEdges)

				// Verify edge types and direction
				for _, edge := range edges {
					assert.Equal(t, graph.EdgeTypeSelects, edge.Type)
					assert.Equal(t, "service-uid", edge.FromUID)

					// Verify properties contain selector
					var props graph.SelectsEdge
					err := json.Unmarshal(edge.Properties, &props)
					assert.NoError(t, err)
					assert.NotNil(t, props.SelectorLabels)
				}
			}
		})
	}
}

func TestIngressExtractor_Matches(t *testing.T) {
	extractor := NewIngressExtractor()

	tests := []struct {
		name     string
		event    models.Event
		expected bool
	}{
		{
			name: "matches Ingress networking.k8s.io",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "Ingress",
					Group: "networking.k8s.io",
				},
			},
			expected: true,
		},
		{
			name: "matches Ingress extensions",
			event: models.Event{
				Resource: models.ResourceMetadata{
					Kind:  "Ingress",
					Group: "extensions",
				},
			},
			expected: true,
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

func TestIngressExtractor_ExtractRelationships(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		ingressData   map[string]interface{}
		mockServices  map[string]*graph.ResourceIdentity // services to add to mock lookup
		expectedEdges int
	}{
		{
			name: "ingress with new API backend",
			ingressData: map[string]interface{}{
				"spec": map[string]interface{}{
					"rules": []interface{}{
						map[string]interface{}{
							"http": map[string]interface{}{
								"paths": []interface{}{
									map[string]interface{}{
										"backend": map[string]interface{}{
											"service": map[string]interface{}{
												"name": "frontend",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			mockServices: map[string]*graph.ResourceIdentity{
				"default/Service/frontend": {
					UID:       "frontend-service-uid",
					Kind:      "Service",
					Namespace: "default",
					Name:      "frontend",
				},
			},
			expectedEdges: 1,
		},
		{
			name: "ingress with old API backend",
			ingressData: map[string]interface{}{
				"spec": map[string]interface{}{
					"rules": []interface{}{
						map[string]interface{}{
							"http": map[string]interface{}{
								"paths": []interface{}{
									map[string]interface{}{
										"backend": map[string]interface{}{
											"serviceName": "frontend",
										},
									},
								},
							},
						},
					},
				},
			},
			mockServices: map[string]*graph.ResourceIdentity{
				"default/Service/frontend": {
					UID:       "frontend-service-uid",
					Kind:      "Service",
					Namespace: "default",
					Name:      "frontend",
				},
			},
			expectedEdges: 1,
		},
		{
			name: "ingress with default backend",
			ingressData: map[string]interface{}{
				"spec": map[string]interface{}{
					"defaultBackend": map[string]interface{}{
						"service": map[string]interface{}{
							"name": "default-backend",
						},
					},
				},
			},
			mockServices: map[string]*graph.ResourceIdentity{
				"default/Service/default-backend": {
					UID:       "default-backend-uid",
					Kind:      "Service",
					Namespace: "default",
					Name:      "default-backend",
				},
			},
			expectedEdges: 1,
		},
		{
			name: "ingress with multiple paths",
			ingressData: map[string]interface{}{
				"spec": map[string]interface{}{
					"rules": []interface{}{
						map[string]interface{}{
							"http": map[string]interface{}{
								"paths": []interface{}{
									map[string]interface{}{
										"backend": map[string]interface{}{
											"service": map[string]interface{}{
												"name": "frontend",
											},
										},
									},
									map[string]interface{}{
										"backend": map[string]interface{}{
											"service": map[string]interface{}{
												"name": "backend",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			mockServices: map[string]*graph.ResourceIdentity{
				"default/Service/frontend": {
					UID:       "frontend-service-uid",
					Kind:      "Service",
					Namespace: "default",
					Name:      "frontend",
				},
				"default/Service/backend": {
					UID:       "backend-service-uid",
					Kind:      "Service",
					Namespace: "default",
					Name:      "backend",
				},
			},
			expectedEdges: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extractor := NewIngressExtractor()

			ingressJSON, err := json.Marshal(tt.ingressData)
			require.NoError(t, err)

			event := models.Event{
				Resource: models.ResourceMetadata{
					UID:       "ingress-uid",
					Kind:      "Ingress",
					Namespace: "default",
					Name:      "test-ingress",
				},
				Data: ingressJSON,
				Type: models.EventTypeUpdate,
			}

			// Create lookup with mock services
			lookup := &MockResourceLookup{
				resources: make(map[string]*graph.ResourceIdentity),
			}
			// Add mock services to the lookup
			for key, service := range tt.mockServices {
				lookup.resources[key] = service
			}

			edges, err := extractor.ExtractRelationships(ctx, event, lookup)

			assert.NoError(t, err)
			assert.Len(t, edges, tt.expectedEdges)

			// Verify edge types
			for _, edge := range edges {
				assert.Equal(t, graph.EdgeTypeReferencesSpec, edge.Type)
				assert.Equal(t, "ingress-uid", edge.FromUID)
				assert.NotEmpty(t, edge.ToUID, "Edge ToUID should not be empty")
			}
		})
	}
}
