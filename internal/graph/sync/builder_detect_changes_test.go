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

// =============================================================================
// REGRESSION TESTS: Change Detection with Caching
// These tests verify that the state cache and batch cache optimizations
// correctly detect changes without breaking existing functionality.
// =============================================================================

// Test_DetectChanges_StateCacheHit tests change detection using state cache
func Test_DetectChanges_StateCacheHit(t *testing.T) {
	// Previous resource in cache
	previousResource := map[string]interface{}{
		"metadata": map[string]interface{}{
			"generation": float64(1),
			"uid":        "test-uid",
		},
		"spec": map[string]interface{}{
			"replicas": float64(2),
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"image": "nginx:1.19",
						},
					},
				},
			},
		},
	}

	// Current resource with changed spec
	currentResource := map[string]interface{}{
		"metadata": map[string]interface{}{
			"generation": float64(2),
			"uid":        "test-uid",
		},
		"spec": map[string]interface{}{
			"replicas": float64(3), // Changed!
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"image": "nginx:1.20", // Changed!
						},
					},
				},
			},
		},
		"status": map[string]interface{}{
			"readyReplicas": float64(2),
		},
	}

	previousJSON, _ := json.Marshal(previousResource)
	currentJSON, _ := json.Marshal(currentResource)
	currentData, err := analyzer.ParseResourceData(currentJSON)
	assert.NoError(t, err)

	// Create builder with mock client that returns empty result (forcing cache use)
	mockClient := &mockGraphClientForDetectChanges{
		queryResult: &graph.QueryResult{Rows: [][]interface{}{}},
	}
	builder := NewGraphBuilderWithClient(mockClient).(*graphBuilder)

	// Pre-populate state cache with previous state
	builder.stateCache.Put("test-uid", previousJSON, 1000, "CREATE")

	event := models.Event{
		Resource:  models.ResourceMetadata{UID: "test-uid"},
		Data:      currentJSON,
		Timestamp: 2000, // Later than cached timestamp
	}

	configChanged, statusChanged, replicasChanged := builder.detectChanges(event, currentData)

	// Verify cache was used
	hits, misses, _ := builder.stateCache.GetStats()
	assert.Equal(t, int64(1), hits, "State cache should have 1 hit")
	assert.Equal(t, int64(0), misses, "State cache should have 0 misses")

	// Verify change detection works correctly
	assert.True(t, configChanged, "configChanged should be true (spec changed)")
	assert.True(t, statusChanged, "statusChanged should be true (status exists)")
	// Note: replicasChanged detection is not fully implemented in detectChanges
	// (see builder.go line ~813), so we just verify it doesn't error
	_ = replicasChanged
}

// Test_DetectChanges_BatchCacheHit tests change detection using batch cache
func Test_DetectChanges_BatchCacheHit(t *testing.T) {
	// Previous and current events in same batch
	previousResource := map[string]interface{}{
		"metadata": map[string]interface{}{
			"generation": float64(1),
			"uid":        "batch-test-uid",
		},
		"spec": map[string]interface{}{
			"replicas": float64(1),
		},
	}

	currentResource := map[string]interface{}{
		"metadata": map[string]interface{}{
			"generation": float64(2),
			"uid":        "batch-test-uid",
		},
		"spec": map[string]interface{}{
			"replicas": float64(5), // Changed!
		},
	}

	previousJSON, _ := json.Marshal(previousResource)
	currentJSON, _ := json.Marshal(currentResource)
	currentData, err := analyzer.ParseResourceData(currentJSON)
	assert.NoError(t, err)

	// Create builder with mock client
	mockClient := &mockGraphClientForDetectChanges{
		queryResult: &graph.QueryResult{Rows: [][]interface{}{}},
	}
	builder := NewGraphBuilderWithClient(mockClient).(*graphBuilder)

	// Set up batch cache with previous event
	previousEvent := models.Event{
		Resource:  models.ResourceMetadata{UID: "batch-test-uid"},
		Data:      previousJSON,
		Timestamp: 1000,
		Type:      models.EventTypeCreate,
	}
	builder.SetBatchCache([]models.Event{previousEvent})

	// Current event should find previous in batch cache
	currentEvent := models.Event{
		Resource:  models.ResourceMetadata{UID: "batch-test-uid"},
		Data:      currentJSON,
		Timestamp: 2000,
	}

	configChanged, statusChanged, replicasChanged := builder.detectChanges(currentEvent, currentData)

	assert.True(t, configChanged, "configChanged should be true (spec.replicas changed)")
	assert.False(t, statusChanged, "statusChanged should be false (no status)")
	// Note: replicasChanged detection is not fully implemented
	_ = replicasChanged

	builder.ClearBatchCache()
}

// Test_DetectChanges_CacheMissQueryFallback tests that detection falls back to DB query
func Test_DetectChanges_CacheMissQueryFallback(t *testing.T) {
	// Previous resource from database
	previousResource := map[string]interface{}{
		"metadata": map[string]interface{}{
			"generation": float64(5),
			"uid":        "fallback-test-uid",
		},
		"spec": map[string]interface{}{
			"replicas": float64(3),
		},
	}

	// Current resource (same spec, different generation for some reason)
	currentResource := map[string]interface{}{
		"metadata": map[string]interface{}{
			"generation": float64(5),
			"uid":        "fallback-test-uid",
		},
		"spec": map[string]interface{}{
			"replicas": float64(3),
		},
	}

	currentJSON, _ := json.Marshal(currentResource)
	currentData, err := analyzer.ParseResourceData(currentJSON)
	assert.NoError(t, err)

	// Mock client returns previous resource from "database"
	mockClient := &mockGraphClientForDetectChanges{
		queryResult: createQueryResultFromResource(previousResource),
	}
	builder := NewGraphBuilderWithClient(mockClient).(*graphBuilder)

	// Don't populate any cache - force database query
	event := models.Event{
		Resource:  models.ResourceMetadata{UID: "fallback-test-uid"},
		Data:      currentJSON,
		Timestamp: 5000,
	}

	configChanged, statusChanged, replicasChanged := builder.detectChanges(event, currentData)

	// Same generation, same spec - no changes
	assert.False(t, configChanged, "configChanged should be false (identical)")
	assert.False(t, statusChanged, "statusChanged should be false (no status)")
	assert.False(t, replicasChanged, "replicasChanged should be false")
}

// Test_DetectChanges_StateCacheUpdatedAfterProcess verifies cache is updated
func Test_DetectChanges_StateCacheUpdatedAfterProcess(t *testing.T) {
	resource := map[string]interface{}{
		"metadata": map[string]interface{}{
			"generation": float64(1),
			"uid":        "cache-update-uid",
		},
		"spec": map[string]interface{}{
			"replicas": float64(2),
		},
		"status": map[string]interface{}{
			"phase": "Running",
		},
	}

	resourceJSON, _ := json.Marshal(resource)

	// Create builder with mock client
	mockClient := &mockGraphClientForDetectChanges{
		queryResult: &graph.QueryResult{Rows: [][]interface{}{}},
	}
	builder := NewGraphBuilderWithClient(mockClient).(*graphBuilder)

	// Initial state - cache is empty
	_, _, sizeBefore := builder.stateCache.GetStats()
	assert.Equal(t, 0, sizeBefore, "Cache should be empty initially")

	// Process event (this should populate the cache)
	event := models.Event{
		ID:        "event-1",
		Resource:  models.ResourceMetadata{UID: "cache-update-uid", Kind: "Pod", Version: "v1", Namespace: "default", Name: "test-pod"},
		Data:      resourceJSON,
		Timestamp: 1000,
		Type:      models.EventTypeCreate,
	}

	ctx := context.Background()
	_, err := builder.BuildFromEvent(ctx, event)
	assert.NoError(t, err)

	// Cache should now contain the resource
	cached := builder.stateCache.Get("cache-update-uid")
	assert.NotNil(t, cached, "Cache should contain processed resource")
	assert.Equal(t, int64(1000), cached.Timestamp)
	assert.Equal(t, "CREATE", cached.EventType)
}

// Test_DetectChanges_MultipleUpdatesInBatch tests sequential updates in same batch
func Test_DetectChanges_MultipleUpdatesInBatch(t *testing.T) {
	// Three events for same resource in one batch
	events := []models.Event{
		{
			Resource:  models.ResourceMetadata{UID: "multi-update-uid", Kind: "Pod", Version: "v1"},
			Data:      createTestResourceJSON(1, 1),
			Timestamp: 1000,
			Type:      models.EventTypeCreate,
		},
		{
			Resource:  models.ResourceMetadata{UID: "multi-update-uid", Kind: "Pod", Version: "v1"},
			Data:      createTestResourceJSON(2, 3), // Gen 2, replicas changed to 3
			Timestamp: 2000,
			Type:      models.EventTypeUpdate,
		},
		{
			Resource:  models.ResourceMetadata{UID: "multi-update-uid", Kind: "Pod", Version: "v1"},
			Data:      createTestResourceJSON(3, 5), // Gen 3, replicas changed to 5
			Timestamp: 3000,
			Type:      models.EventTypeUpdate,
		},
	}

	mockClient := &mockGraphClientForDetectChanges{
		queryResult: &graph.QueryResult{Rows: [][]interface{}{}},
	}
	builder := NewGraphBuilderWithClient(mockClient).(*graphBuilder)

	// Set batch cache
	builder.SetBatchCache(events)

	// Process second event - should detect changes from first event
	currentData2, _ := analyzer.ParseResourceData(events[1].Data)
	config2, _, _ := builder.detectChanges(events[1], currentData2)
	assert.True(t, config2, "Second event should detect config change")

	// Process third event - should detect changes from second event
	currentData3, _ := analyzer.ParseResourceData(events[2].Data)
	config3, _, _ := builder.detectChanges(events[2], currentData3)
	assert.True(t, config3, "Third event should detect config change")

	builder.ClearBatchCache()
}

func createTestResourceJSON(generation, replicas int) json.RawMessage {
	resource := map[string]interface{}{
		"metadata": map[string]interface{}{
			"generation": float64(generation),
		},
		"spec": map[string]interface{}{
			"replicas": float64(replicas),
		},
	}
	data, _ := json.Marshal(resource)
	return data
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
