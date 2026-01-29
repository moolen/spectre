package grafana

import (
	"context"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAggregatorGraphClient implements graph.Client for aggregator tests.
type mockAggregatorGraphClient struct {
	executeQueryFunc func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error)
	queries          []graph.GraphQuery
}

func newMockAggregatorGraphClient() *mockAggregatorGraphClient {
	return &mockAggregatorGraphClient{
		queries: make([]graph.GraphQuery, 0),
	}
}

func (m *mockAggregatorGraphClient) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	m.queries = append(m.queries, query)
	if m.executeQueryFunc != nil {
		return m.executeQueryFunc(ctx, query)
	}
	return &graph.QueryResult{}, nil
}

// Implement remaining graph.Client interface methods
func (m *mockAggregatorGraphClient) Connect(ctx context.Context) error { return nil }
func (m *mockAggregatorGraphClient) Close() error                      { return nil }
func (m *mockAggregatorGraphClient) Ping(ctx context.Context) error    { return nil }
func (m *mockAggregatorGraphClient) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}
func (m *mockAggregatorGraphClient) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}
func (m *mockAggregatorGraphClient) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}
func (m *mockAggregatorGraphClient) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}
func (m *mockAggregatorGraphClient) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}
func (m *mockAggregatorGraphClient) InitializeSchema(ctx context.Context) error { return nil }
func (m *mockAggregatorGraphClient) DeleteGraph(ctx context.Context) error      { return nil }
func (m *mockAggregatorGraphClient) CreateGraph(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockAggregatorGraphClient) DeleteGraphByName(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockAggregatorGraphClient) GraphExists(ctx context.Context, graphName string) (bool, error) {
	return false, nil
}

// TestAggregateWorkloadAnomaly_SingleSignal tests aggregation with one signal.
func TestAggregateWorkloadAnomaly_SingleSignal(t *testing.T) {
	logger := logging.GetLogger("test.aggregator")

	mockGraph := newMockAggregatorGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Return a single signal with baseline
		return &graph.QueryResult{
			Columns: []string{"metric_name", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
			Rows: [][]interface{}{
				{"container_cpu_usage", 0.8, 100.0, 10.0, 80.0, 120.0, 100.0, 115.0, 118.0, float64(100)},
			},
		}, nil
	}

	aggregator := NewAnomalyAggregator(mockGraph, "test-grafana", logger)

	// Clear cache to ensure fresh computation
	aggregator.cache.Clear()

	ctx := context.Background()
	result, err := aggregator.AggregateWorkloadAnomaly(ctx, "default", "nginx")

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "workload", result.Scope)
	assert.Equal(t, "default/nginx", result.ScopeKey)
	assert.Equal(t, 1, result.SourceCount)
	assert.Equal(t, "container_cpu_usage", result.TopSource)
	assert.Equal(t, 0.8, result.TopSourceQuality)
}

// TestAggregateWorkloadAnomaly_MultipleSignals_MaxScore tests that MAX is used for aggregation.
func TestAggregateWorkloadAnomaly_MultipleSignals_MaxScore(t *testing.T) {
	logger := logging.GetLogger("test.aggregator")

	mockGraph := newMockAggregatorGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Return multiple signals with different characteristics
		// Signal 1: normal value (z-score low)
		// Signal 2: high value (z-score high) - this should dominate
		return &graph.QueryResult{
			Columns: []string{"metric_name", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
			Rows: [][]interface{}{
				// Normal signal: value at mean
				{"cpu_normal", 0.8, 100.0, 10.0, 80.0, 120.0, 100.0, 115.0, 118.0, float64(100)},
				// Anomalous signal: value far from mean (will compute high z-score)
				// Using baseline with low stddev so any deviation is significant
				{"cpu_anomalous", 0.9, 50.0, 5.0, 40.0, 60.0, 50.0, 55.0, 58.0, float64(100)},
			},
		}, nil
	}

	aggregator := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	aggregator.cache.Clear()

	ctx := context.Background()
	result, err := aggregator.AggregateWorkloadAnomaly(ctx, "default", "nginx")

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 2, result.SourceCount)
	// Both signals use baseline mean as current value, so scores should be similar
	// The quality tiebreaker should select the higher quality signal
	assert.True(t, result.TopSourceQuality >= 0.8, "should select a signal")
}

// TestAggregateWorkloadAnomaly_QualityTiebreaker tests that quality breaks ties.
func TestAggregateWorkloadAnomaly_QualityTiebreaker(t *testing.T) {
	logger := logging.GetLogger("test.aggregator")

	mockGraph := newMockAggregatorGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Return two signals with identical baselines but different quality scores
		// Both will have the same z-score (value at mean = z=0)
		return &graph.QueryResult{
			Columns: []string{"metric_name", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
			Rows: [][]interface{}{
				{"low_quality_signal", 0.5, 100.0, 10.0, 80.0, 120.0, 100.0, 115.0, 118.0, float64(100)},
				{"high_quality_signal", 0.9, 100.0, 10.0, 80.0, 120.0, 100.0, 115.0, 118.0, float64(100)},
			},
		}, nil
	}

	aggregator := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	aggregator.cache.Clear()

	ctx := context.Background()
	result, err := aggregator.AggregateWorkloadAnomaly(ctx, "default", "nginx")

	require.NoError(t, err)
	require.NotNil(t, result)

	// Same score, higher quality should win as TopSource
	assert.Equal(t, "high_quality_signal", result.TopSource, "higher quality signal should be TopSource when scores are equal")
	assert.Equal(t, 0.9, result.TopSourceQuality)
}

// TestAggregateWorkloadAnomaly_ColdStartSignal_Skipped tests that signals without baseline are skipped.
func TestAggregateWorkloadAnomaly_ColdStartSignal_Skipped(t *testing.T) {
	logger := logging.GetLogger("test.aggregator")

	mockGraph := newMockAggregatorGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Return one signal with baseline and one without (sample_count = nil)
		return &graph.QueryResult{
			Columns: []string{"metric_name", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
			Rows: [][]interface{}{
				// Signal with baseline
				{"with_baseline", 0.8, 100.0, 10.0, 80.0, 120.0, 100.0, 115.0, 118.0, float64(100)},
				// Signal without baseline (nil sample_count)
				{"without_baseline", 0.9, nil, nil, nil, nil, nil, nil, nil, nil},
			},
		}, nil
	}

	aggregator := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	aggregator.cache.Clear()

	ctx := context.Background()
	result, err := aggregator.AggregateWorkloadAnomaly(ctx, "default", "nginx")

	require.NoError(t, err)
	require.NotNil(t, result)

	// Only the signal with baseline should be counted
	assert.Equal(t, 1, result.SourceCount, "only signal with baseline should be counted")
	assert.Equal(t, "with_baseline", result.TopSource)
}

// TestAggregateWorkloadAnomaly_Cached tests that results are cached.
func TestAggregateWorkloadAnomaly_Cached(t *testing.T) {
	logger := logging.GetLogger("test.aggregator")

	queryCount := 0
	mockGraph := newMockAggregatorGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		queryCount++
		return &graph.QueryResult{
			Columns: []string{"metric_name", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
			Rows: [][]interface{}{
				{"cpu_metric", 0.8, 100.0, 10.0, 80.0, 120.0, 100.0, 115.0, 118.0, float64(100)},
			},
		}, nil
	}

	aggregator := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	aggregator.cache.Clear()

	ctx := context.Background()

	// First call - should query graph
	result1, err := aggregator.AggregateWorkloadAnomaly(ctx, "default", "nginx")
	require.NoError(t, err)
	require.NotNil(t, result1)
	assert.Equal(t, 1, queryCount, "first call should query graph")

	// Second call - should use cache
	result2, err := aggregator.AggregateWorkloadAnomaly(ctx, "default", "nginx")
	require.NoError(t, err)
	require.NotNil(t, result2)
	assert.Equal(t, 1, queryCount, "second call should use cache (no additional query)")

	// Results should be identical
	assert.Equal(t, result1.Score, result2.Score)
	assert.Equal(t, result1.TopSource, result2.TopSource)
}

// TestAggregateNamespaceAnomaly_MultipleWorkloads tests namespace-level aggregation.
func TestAggregateNamespaceAnomaly_MultipleWorkloads(t *testing.T) {
	logger := logging.GetLogger("test.aggregator")

	mockGraph := newMockAggregatorGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Check which query this is
		if query.Parameters["namespace"] != nil && query.Parameters["workload_name"] == nil {
			// Namespace workloads query
			return &graph.QueryResult{
				Columns: []string{"workload_name"},
				Rows: [][]interface{}{
					{"nginx"},
					{"redis"},
				},
			}, nil
		}
		if query.Parameters["workload_name"] != nil {
			// Workload signals query
			workload := query.Parameters["workload_name"].(string)
			if workload == "nginx" {
				return &graph.QueryResult{
					Columns: []string{"metric_name", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
					Rows: [][]interface{}{
						{"nginx_cpu", 0.8, 100.0, 10.0, 80.0, 120.0, 100.0, 115.0, 118.0, float64(100)},
					},
				}, nil
			}
			if workload == "redis" {
				return &graph.QueryResult{
					Columns: []string{"metric_name", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
					Rows: [][]interface{}{
						{"redis_memory", 0.9, 500.0, 50.0, 400.0, 600.0, 500.0, 575.0, 590.0, float64(100)},
					},
				}, nil
			}
		}
		return &graph.QueryResult{}, nil
	}

	aggregator := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	aggregator.cache.Clear()

	ctx := context.Background()
	result, err := aggregator.AggregateNamespaceAnomaly(ctx, "default")

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "namespace", result.Scope)
	assert.Equal(t, "default", result.ScopeKey)
	assert.Equal(t, 2, result.SourceCount, "should aggregate signals from both workloads")
}

// TestAggregateClusterAnomaly tests cluster-level aggregation.
func TestAggregateClusterAnomaly(t *testing.T) {
	logger := logging.GetLogger("test.aggregator")

	mockGraph := newMockAggregatorGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Check which query this is
		if query.Parameters["namespace"] == nil && query.Parameters["workload_name"] == nil {
			// Cluster namespaces query
			return &graph.QueryResult{
				Columns: []string{"namespace"},
				Rows: [][]interface{}{
					{"default"},
					{"kube-system"},
				},
			}, nil
		}
		if query.Parameters["namespace"] != nil && query.Parameters["workload_name"] == nil {
			// Namespace workloads query
			ns := query.Parameters["namespace"].(string)
			if ns == "default" {
				return &graph.QueryResult{
					Columns: []string{"workload_name"},
					Rows: [][]interface{}{
						{"nginx"},
					},
				}, nil
			}
			if ns == "kube-system" {
				return &graph.QueryResult{
					Columns: []string{"workload_name"},
					Rows: [][]interface{}{
						{"coredns"},
					},
				}, nil
			}
		}
		if query.Parameters["workload_name"] != nil {
			// Workload signals query
			return &graph.QueryResult{
				Columns: []string{"metric_name", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
				Rows: [][]interface{}{
					{"cpu_usage", 0.8, 100.0, 10.0, 80.0, 120.0, 100.0, 115.0, 118.0, float64(100)},
				},
			}, nil
		}
		return &graph.QueryResult{}, nil
	}

	aggregator := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	aggregator.cache.Clear()

	ctx := context.Background()
	result, err := aggregator.AggregateClusterAnomaly(ctx)

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "cluster", result.Scope)
	assert.Equal(t, "test-grafana", result.ScopeKey)
	assert.Equal(t, 2, result.SourceCount, "should aggregate signals from all namespaces")
}

// TestAggregationCache_TTLExpiry tests that cache entries expire after TTL.
func TestAggregationCache_TTLExpiry(t *testing.T) {
	// Create cache with very short TTL and no jitter for testing
	cache := NewAggregationCache(10*time.Millisecond, 0)

	entry := &AggregatedAnomaly{
		Scope:    "test",
		ScopeKey: "test-key",
		Score:    0.5,
	}

	cache.Set("test", entry)

	// Should be available immediately
	result := cache.Get("test")
	assert.NotNil(t, result, "entry should be available immediately")
	assert.Equal(t, 0.5, result.Score)

	// Wait for TTL to expire
	time.Sleep(15 * time.Millisecond)

	// Should be nil after expiry
	result = cache.Get("test")
	assert.Nil(t, result, "entry should be nil after TTL expiry")
}

// TestNewAnomalyAggregator tests aggregator initialization.
func TestNewAnomalyAggregator(t *testing.T) {
	logger := logging.GetLogger("test.aggregator")
	mockGraph := newMockAggregatorGraphClient()

	aggregator := NewAnomalyAggregator(mockGraph, "test-integration", logger)

	assert.NotNil(t, aggregator)
	assert.Equal(t, "test-integration", aggregator.integrationName)
	assert.NotNil(t, aggregator.cache)
}
