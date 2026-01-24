package sync

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moolen/spectre/internal/analyzer"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/assert"
)

// mockGraphClientForDetectChanges implements graph.Client for detectChanges tests
type mockGraphClientForDetectChanges struct {
	queryResult *graph.QueryResult
}

func (m *mockGraphClientForDetectChanges) Connect(ctx context.Context) error { return nil }
func (m *mockGraphClientForDetectChanges) Close() error                      { return nil }
func (m *mockGraphClientForDetectChanges) Ping(ctx context.Context) error    { return nil }
func (m *mockGraphClientForDetectChanges) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}
func (m *mockGraphClientForDetectChanges) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}
func (m *mockGraphClientForDetectChanges) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}
func (m *mockGraphClientForDetectChanges) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}
func (m *mockGraphClientForDetectChanges) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}
func (m *mockGraphClientForDetectChanges) InitializeSchema(ctx context.Context) error { return nil }
func (m *mockGraphClientForDetectChanges) DeleteGraph(ctx context.Context) error      { return nil }
func (m *mockGraphClientForDetectChanges) CreateGraph(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockGraphClientForDetectChanges) DeleteGraphByName(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockGraphClientForDetectChanges) GraphExists(ctx context.Context, graphName string) (bool, error) {
	return true, nil
}
func (m *mockGraphClientForDetectChanges) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	return m.queryResult, nil
}

func createQueryResultFromResource(resource map[string]interface{}) *graph.QueryResult {
	resourceJSON, _ := json.Marshal(resource)
	return &graph.QueryResult{
		Columns: []string{"ce.data"},
		Rows: [][]interface{}{
			{string(resourceJSON)},
		},
	}
}

func Test_DetectChanges_GenerationIncreasesWithSpecChange(t *testing.T) {
	// Previous resource with generation 1
	previousResource := map[string]interface{}{
		"metadata": map[string]interface{}{
			"generation": float64(1),
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"image": "nginx:1.19",
				},
			},
		},
	}

	// Current resource with generation 2 and different spec
	currentResource := map[string]interface{}{
		"metadata": map[string]interface{}{
			"generation": float64(2),
		},
		"spec": map[string]interface{}{
			"containers": []interface{}{
				map[string]interface{}{
					"image": "nginx:1.20", // Changed!
				},
			},
		},
		"status": map[string]interface{}{
			"phase": "Running",
		},
	}

	// Create ResourceData from current resource
	currentJSON, _ := json.Marshal(currentResource)
	currentData, err := analyzer.ParseResourceData(currentJSON)
	assert.NoError(t, err)

	// Mock client returns previous resource
	mockClient := &mockGraphClientForDetectChanges{
		queryResult: createQueryResultFromResource(previousResource),
	}
	builder := NewGraphBuilderWithClient(mockClient).(*graphBuilder)

	event := models.Event{
		Resource: models.ResourceMetadata{UID: "test-uid"},
		Data:     currentJSON,
	}

	configChanged, statusChanged, replicasChanged := builder.detectChanges(event, currentData)

	assert.True(t, configChanged, "configChanged should be true when generation increases and spec changes")
	assert.True(t, statusChanged, "statusChanged should be true when status exists")
	assert.False(t, replicasChanged)
}

func Test_DetectChanges_GenerationIncreasesButSpecUnchanged(t *testing.T) {
	// Previous resource with generation 835
	previousResource := map[string]interface{}{
		"metadata": map[string]interface{}{
			"generation": float64(835),
			"annotations": map[string]interface{}{
				"deployment.kubernetes.io/revision": "838",
			},
		},
		"spec": map[string]interface{}{
			"replicas": float64(0),
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"app": "external-secrets",
					},
				},
			},
		},
	}

	// Current resource with generation 836 but SAME spec (only metadata changed)
	currentResource := map[string]interface{}{
		"metadata": map[string]interface{}{
			"generation": float64(836),
			"annotations": map[string]interface{}{
				"deployment.kubernetes.io/revision": "839", // Different annotation
			},
		},
		"spec": map[string]interface{}{
			"replicas": float64(0),
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"app": "external-secrets",
					},
				},
			},
		},
		"status": map[string]interface{}{
			"observedGeneration": float64(836),
		},
	}

	currentJSON, _ := json.Marshal(currentResource)
	currentData, err := analyzer.ParseResourceData(currentJSON)
	assert.NoError(t, err)

	mockClient := &mockGraphClientForDetectChanges{
		queryResult: createQueryResultFromResource(previousResource),
	}
	builder := NewGraphBuilderWithClient(mockClient).(*graphBuilder)

	event := models.Event{
		Resource: models.ResourceMetadata{UID: "replicaset-uid"},
		Data:     currentJSON,
	}

	configChanged, statusChanged, replicasChanged := builder.detectChanges(event, currentData)

	assert.False(t, configChanged, "configChanged should be false when only metadata changed (spec identical)")
	assert.True(t, statusChanged)
	assert.False(t, replicasChanged)
}

func Test_DetectChanges_GenerationUnchanged(t *testing.T) {
	// Both have same generation and spec
	resource := map[string]interface{}{
		"metadata": map[string]interface{}{
			"generation": float64(5),
		},
		"spec": map[string]interface{}{
			"replicas": float64(3),
		},
	}

	currentJSON, _ := json.Marshal(resource)
	currentData, err := analyzer.ParseResourceData(currentJSON)
	assert.NoError(t, err)

	mockClient := &mockGraphClientForDetectChanges{
		queryResult: createQueryResultFromResource(resource),
	}
	builder := NewGraphBuilderWithClient(mockClient).(*graphBuilder)

	event := models.Event{
		Resource: models.ResourceMetadata{UID: "test-uid"},
		Data:     currentJSON,
	}

	configChanged, statusChanged, replicasChanged := builder.detectChanges(event, currentData)

	assert.False(t, configChanged, "configChanged should be false when generation unchanged")
	assert.False(t, statusChanged, "statusChanged should be false when no status field")
	assert.False(t, replicasChanged)
}

func Test_DetectChanges_SpecAdded(t *testing.T) {
	// Previous had no spec
	previousResource := map[string]interface{}{
		"metadata": map[string]interface{}{
			"generation": float64(1),
		},
	}

	// Current has spec
	currentResource := map[string]interface{}{
		"metadata": map[string]interface{}{
			"generation": float64(2),
		},
		"spec": map[string]interface{}{
			"replicas": float64(3),
		},
	}

	currentJSON, _ := json.Marshal(currentResource)
	currentData, err := analyzer.ParseResourceData(currentJSON)
	assert.NoError(t, err)

	mockClient := &mockGraphClientForDetectChanges{
		queryResult: createQueryResultFromResource(previousResource),
	}
	builder := NewGraphBuilderWithClient(mockClient).(*graphBuilder)

	event := models.Event{
		Resource: models.ResourceMetadata{UID: "test-uid"},
		Data:     currentJSON,
	}

	configChanged, statusChanged, replicasChanged := builder.detectChanges(event, currentData)

	assert.True(t, configChanged, "configChanged should be true when spec is added")
	assert.False(t, statusChanged)
	assert.False(t, replicasChanged)
}

func Test_DetectChanges_NoPreviousEvent(t *testing.T) {
	currentResource := map[string]interface{}{
		"metadata": map[string]interface{}{
			"generation": float64(1),
		},
		"spec": map[string]interface{}{
			"replicas": float64(3),
		},
	}

	currentJSON, _ := json.Marshal(currentResource)
	currentData, err := analyzer.ParseResourceData(currentJSON)
	assert.NoError(t, err)

	// Empty query result (no previous event)
	mockClient := &mockGraphClientForDetectChanges{
		queryResult: &graph.QueryResult{Rows: [][]interface{}{}},
	}
	builder := NewGraphBuilderWithClient(mockClient).(*graphBuilder)

	event := models.Event{
		Resource: models.ResourceMetadata{UID: "test-uid"},
		Data:     currentJSON,
	}

	configChanged, statusChanged, replicasChanged := builder.detectChanges(event, currentData)

	assert.False(t, configChanged, "configChanged should be false when no previous event exists")
	assert.True(t, statusChanged, "statusChanged should be true (conservative for first event)")
	assert.False(t, replicasChanged)
}

func Test_DeepEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        interface{}
		b        interface{}
		expected bool
	}{
		{
			name:     "nil values",
			a:        nil,
			b:        nil,
			expected: true,
		},
		{
			name:     "one nil value",
			a:        nil,
			b:        "test",
			expected: false,
		},
		{
			name:     "equal strings",
			a:        "hello",
			b:        "hello",
			expected: true,
		},
		{
			name:     "different strings",
			a:        "hello",
			b:        "world",
			expected: false,
		},
		{
			name:     "equal numbers",
			a:        float64(42),
			b:        float64(42),
			expected: true,
		},
		{
			name:     "different numbers",
			a:        float64(42),
			b:        float64(43),
			expected: false,
		},
		{
			name:     "equal booleans",
			a:        true,
			b:        true,
			expected: true,
		},
		{
			name:     "different booleans",
			a:        true,
			b:        false,
			expected: false,
		},
		{
			name: "equal maps",
			a: map[string]interface{}{
				"key1": "value1",
				"key2": float64(42),
			},
			b: map[string]interface{}{
				"key1": "value1",
				"key2": float64(42),
			},
			expected: true,
		},
		{
			name: "different maps - different values",
			a: map[string]interface{}{
				"key1": "value1",
			},
			b: map[string]interface{}{
				"key1": "value2",
			},
			expected: false,
		},
		{
			name: "equal slices",
			a: []interface{}{
				"item1",
				float64(42),
			},
			b: []interface{}{
				"item1",
				float64(42),
			},
			expected: true,
		},
		{
			name: "different slices - different lengths",
			a: []interface{}{
				"item1",
			},
			b: []interface{}{
				"item1",
				"item2",
			},
			expected: false,
		},
		{
			name: "complex nested structure - equal",
			a: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": float64(3),
					"template": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "nginx",
								"image": "nginx:1.19",
							},
						},
					},
				},
			},
			b: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": float64(3),
					"template": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "nginx",
								"image": "nginx:1.19",
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "complex nested structure - different",
			a: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": float64(3),
					"template": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "nginx",
								"image": "nginx:1.19",
							},
						},
					},
				},
			},
			b: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": float64(3),
					"template": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "nginx",
								"image": "nginx:1.20", // Different!
							},
						},
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deepEqual(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}
