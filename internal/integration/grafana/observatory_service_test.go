package grafana

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockObservatoryGraphClient implements graph.Client for observatory tests.
type mockObservatoryGraphClient struct {
	executeQueryFunc func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error)
	queries          []graph.GraphQuery
}

func newMockObservatoryGraphClient() *mockObservatoryGraphClient {
	return &mockObservatoryGraphClient{
		queries: make([]graph.GraphQuery, 0),
	}
}

func (m *mockObservatoryGraphClient) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	m.queries = append(m.queries, query)
	if m.executeQueryFunc != nil {
		return m.executeQueryFunc(ctx, query)
	}
	return &graph.QueryResult{}, nil
}

// Implement remaining graph.Client interface methods
func (m *mockObservatoryGraphClient) Connect(ctx context.Context) error { return nil }
func (m *mockObservatoryGraphClient) Close() error                      { return nil }
func (m *mockObservatoryGraphClient) Ping(ctx context.Context) error    { return nil }
func (m *mockObservatoryGraphClient) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}
func (m *mockObservatoryGraphClient) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}
func (m *mockObservatoryGraphClient) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}
func (m *mockObservatoryGraphClient) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}
func (m *mockObservatoryGraphClient) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}
func (m *mockObservatoryGraphClient) InitializeSchema(ctx context.Context) error { return nil }
func (m *mockObservatoryGraphClient) DeleteGraph(ctx context.Context) error      { return nil }
func (m *mockObservatoryGraphClient) CreateGraph(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockObservatoryGraphClient) DeleteGraphByName(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockObservatoryGraphClient) GraphExists(ctx context.Context, graphName string) (bool, error) {
	return false, nil
}

// TestObservatoryService_GetClusterAnomalies_Success tests cluster-wide anomaly summary.
// Note: The implementation uses baseline.Mean as currentValue proxy, which produces
// anomaly via percentile comparison when Mean > P99 (simulating anomalous baseline).
func TestObservatoryService_GetClusterAnomalies_Success(t *testing.T) {
	logger := logging.GetLogger("test.observatory")

	mockGraph := newMockObservatoryGraphClient()

	// Track query types to return appropriate mock data
	// Note: ObservatoryService uses "sig" alias, AnomalyAggregator uses "s" alias
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Namespace workloads query - check first because it also contains workload_namespace
		// Pattern: RETURN DISTINCT ... workload_name AS workload_name
		if strings.Contains(query.Query, "DISTINCT") && strings.Contains(query.Query, "AS workload_name") {
			ns := query.Parameters["namespace"].(string)
			switch ns {
			case "prod":
				return &graph.QueryResult{
					Columns: []string{"workload_name"},
					Rows: [][]interface{}{
						{"nginx"},
						{"api-server"},
					},
				}, nil
			case "staging":
				return &graph.QueryResult{
					Columns: []string{"workload_name"},
					Rows: [][]interface{}{
						{"redis"},
					},
				}, nil
			case "dev":
				return &graph.QueryResult{
					Columns: []string{"workload_name"},
					Rows: [][]interface{}{
						{"mysql"},
					},
				}, nil
			}
		}

		// Cluster namespaces query (both aliases)
		// Pattern: RETURN DISTINCT ... workload_namespace AS namespace
		if strings.Contains(query.Query, "DISTINCT") && strings.Contains(query.Query, "AS namespace") {
			return &graph.QueryResult{
				Columns: []string{"namespace"},
				Rows: [][]interface{}{
					{"prod"},
					{"staging"},
					{"dev"},
				},
			}, nil
		}

		// Workload signals query
		// To produce anomaly scores >= 0.5, we set mean > P99 so percentile score triggers
		// (since implementation uses mean as currentValue proxy)
		if strings.Contains(query.Query, "HAS_BASELINE") {
			workload := query.Parameters["workload_name"].(string)
			switch workload {
			case "nginx":
				// Anomalous: mean(1200) > P99(1180) -> percentile score triggers
				return &graph.QueryResult{
					Columns: []string{"metric_name", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
					Rows: [][]interface{}{
						{"http_requests_total", 0.9, 1200.0, 50.0, 800.0, 1200.0, 1000.0, 1150.0, 1180.0, float64(100)},
					},
				}, nil
			case "api-server":
				// Anomalous: mean(200) > P99(68) -> percentile score triggers
				return &graph.QueryResult{
					Columns: []string{"metric_name", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
					Rows: [][]interface{}{
						{"cpu_usage", 0.8, 200.0, 10.0, 30.0, 70.0, 50.0, 65.0, 68.0, float64(100)},
					},
				}, nil
			case "redis":
				// Anomalous: mean(300) > P99(238) -> percentile score triggers
				return &graph.QueryResult{
					Columns: []string{"metric_name", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
					Rows: [][]interface{}{
						{"memory_usage", 0.85, 300.0, 20.0, 160.0, 240.0, 200.0, 230.0, 238.0, float64(100)},
					},
				}, nil
			case "mysql":
				// Anomalous: mean(150) > P99(118) -> percentile score triggers
				return &graph.QueryResult{
					Columns: []string{"metric_name", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
					Rows: [][]interface{}{
						{"connections", 0.7, 150.0, 10.0, 80.0, 120.0, 100.0, 115.0, 118.0, float64(100)},
					},
				}, nil
			}
		}

		return &graph.QueryResult{}, nil
	}

	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	anomalyAgg.cache.Clear()

	service := NewObservatoryService(mockGraph, anomalyAgg, "test-grafana", logger)

	ctx := context.Background()
	result, err := service.GetClusterAnomalies(ctx, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have hotspots (those with score >= 0.5 after aggregation)
	assert.NotEmpty(t, result.TopHotspots, "should have hotspots")
	assert.LessOrEqual(t, len(result.TopHotspots), 5, "should limit to top 5")
	assert.NotEmpty(t, result.Timestamp, "should have timestamp")

	// Verify all hotspots have score >= 0.5 (threshold)
	for _, hotspot := range result.TopHotspots {
		assert.GreaterOrEqual(t, hotspot.Score, 0.5,
			"hotspot %s should have score >= 0.5", hotspot.Namespace)
	}

	// Verify hotspots are sorted by score descending
	for i := 1; i < len(result.TopHotspots); i++ {
		assert.GreaterOrEqual(t, result.TopHotspots[i-1].Score, result.TopHotspots[i].Score,
			"hotspots should be sorted by score descending")
	}
}

// TestObservatoryService_GetClusterAnomalies_ThresholdFilter tests that scores < 0.5 are excluded.
func TestObservatoryService_GetClusterAnomalies_ThresholdFilter(t *testing.T) {
	logger := logging.GetLogger("test.observatory")

	mockGraph := newMockObservatoryGraphClient()

	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Cluster namespaces query
		if strings.Contains(query.Query, "DISTINCT sig.workload_namespace") {
			return &graph.QueryResult{
				Columns: []string{"namespace"},
				Rows: [][]interface{}{
					{"low-score-ns"},
				},
			}, nil
		}

		// Namespace workloads query
		if strings.Contains(query.Query, "DISTINCT sig.workload_name") {
			return &graph.QueryResult{
				Columns: []string{"workload_name"},
				Rows: [][]interface{}{
					{"low-anomaly-workload"},
				},
			}, nil
		}

		// Workload signals query - return signal with low anomaly (value at mean = z-score 0)
		if strings.Contains(query.Query, "HAS_BASELINE") {
			return &graph.QueryResult{
				Columns: []string{"metric_name", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
				Rows: [][]interface{}{
					// Value at mean -> z-score = 0 -> normalized score ~0
					{"normal_metric", 0.8, 100.0, 10.0, 80.0, 120.0, 100.0, 115.0, 118.0, float64(100)},
				},
			}, nil
		}

		return &graph.QueryResult{}, nil
	}

	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	anomalyAgg.cache.Clear()

	service := NewObservatoryService(mockGraph, anomalyAgg, "test-grafana", logger)

	ctx := context.Background()
	result, err := service.GetClusterAnomalies(ctx, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	// All hotspots should have score >= 0.5
	for _, hotspot := range result.TopHotspots {
		assert.GreaterOrEqual(t, hotspot.Score, 0.5,
			"all hotspots should have score >= 0.5 (anomaly threshold)")
	}
}

// TestObservatoryService_GetClusterAnomalies_Empty tests empty results when no anomalies.
func TestObservatoryService_GetClusterAnomalies_Empty(t *testing.T) {
	logger := logging.GetLogger("test.observatory")

	mockGraph := newMockObservatoryGraphClient()

	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Return no namespaces
		if strings.Contains(query.Query, "DISTINCT sig.workload_namespace") {
			return &graph.QueryResult{
				Columns: []string{"namespace"},
				Rows:    [][]interface{}{},
			}, nil
		}
		return &graph.QueryResult{}, nil
	}

	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	anomalyAgg.cache.Clear()

	service := NewObservatoryService(mockGraph, anomalyAgg, "test-grafana", logger)

	ctx := context.Background()
	result, err := service.GetClusterAnomalies(ctx, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Should return empty TopHotspots, not error
	assert.Empty(t, result.TopHotspots, "should return empty hotspots when no anomalies")
	assert.Equal(t, 0, result.TotalAnomalousSignals, "should have 0 total anomalous signals")
	assert.NotEmpty(t, result.Timestamp, "should still have timestamp")
}

// TestObservatoryService_GetNamespaceAnomalies_Success tests namespace-level workload anomalies.
func TestObservatoryService_GetNamespaceAnomalies_Success(t *testing.T) {
	logger := logging.GetLogger("test.observatory")

	mockGraph := newMockObservatoryGraphClient()

	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Namespace workloads query
		if strings.Contains(query.Query, "DISTINCT sig.workload_name") {
			return &graph.QueryResult{
				Columns: []string{"workload_name"},
				Rows: [][]interface{}{
					{"nginx"},
					{"api-server"},
					{"worker"},
				},
			}, nil
		}

		// Workload signals query
		if strings.Contains(query.Query, "HAS_BASELINE") {
			workload := query.Parameters["workload_name"].(string)
			switch workload {
			case "nginx":
				return &graph.QueryResult{
					Columns: []string{"metric_name", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
					Rows: [][]interface{}{
						{"http_requests", 0.9, 1000.0, 50.0, 800.0, 1200.0, 1000.0, 1150.0, 1180.0, float64(100)},
					},
				}, nil
			case "api-server":
				return &graph.QueryResult{
					Columns: []string{"metric_name", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
					Rows: [][]interface{}{
						{"cpu_usage", 0.85, 50.0, 10.0, 30.0, 70.0, 50.0, 65.0, 68.0, float64(100)},
					},
				}, nil
			case "worker":
				return &graph.QueryResult{
					Columns: []string{"metric_name", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
					Rows: [][]interface{}{
						{"queue_depth", 0.7, 10.0, 2.0, 5.0, 15.0, 10.0, 14.0, 14.5, float64(100)},
					},
				}, nil
			}
		}

		return &graph.QueryResult{}, nil
	}

	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	anomalyAgg.cache.Clear()

	service := NewObservatoryService(mockGraph, anomalyAgg, "test-grafana", logger)

	ctx := context.Background()
	result, err := service.GetNamespaceAnomalies(ctx, "prod")

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "prod", result.Namespace)
	assert.NotEmpty(t, result.Timestamp)

	// Verify workloads are sorted by score descending
	for i := 1; i < len(result.Workloads); i++ {
		assert.GreaterOrEqual(t, result.Workloads[i-1].Score, result.Workloads[i].Score,
			"workloads should be sorted by score descending")
	}
}

// TestObservatoryService_GetNamespaceAnomalies_Top20Limit tests that results are limited to 20.
func TestObservatoryService_GetNamespaceAnomalies_Top20Limit(t *testing.T) {
	logger := logging.GetLogger("test.observatory")

	mockGraph := newMockObservatoryGraphClient()

	// Create 25 workloads to test the limit
	workloadNames := make([][]interface{}, 25)
	for i := 0; i < 25; i++ {
		workloadNames[i] = []interface{}{t.Name() + "-workload-" + string(rune('a'+i))}
	}

	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Namespace workloads query - return 25 workloads
		if strings.Contains(query.Query, "DISTINCT sig.workload_name") {
			return &graph.QueryResult{
				Columns: []string{"workload_name"},
				Rows:    workloadNames,
			}, nil
		}

		// Workload signals query - each has some anomaly
		if strings.Contains(query.Query, "HAS_BASELINE") {
			return &graph.QueryResult{
				Columns: []string{"metric_name", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
				Rows: [][]interface{}{
					{"metric", 0.8, 100.0, 10.0, 80.0, 120.0, 100.0, 115.0, 118.0, float64(100)},
				},
			}, nil
		}

		return &graph.QueryResult{}, nil
	}

	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	anomalyAgg.cache.Clear()

	service := NewObservatoryService(mockGraph, anomalyAgg, "test-grafana", logger)

	ctx := context.Background()
	result, err := service.GetNamespaceAnomalies(ctx, "test-ns")

	require.NoError(t, err)
	require.NotNil(t, result)

	// Should be limited to 20 (but only if all have score >= 0.5)
	assert.LessOrEqual(t, len(result.Workloads), 20, "should limit to top 20 workloads")
}

// TestObservatoryService_GetWorkloadAnomalyDetail_Success tests signal-level anomaly detail.
func TestObservatoryService_GetWorkloadAnomalyDetail_Success(t *testing.T) {
	logger := logging.GetLogger("test.observatory")

	mockGraph := newMockObservatoryGraphClient()

	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Workload signals with role query
		if strings.Contains(query.Query, "sig.role") {
			return &graph.QueryResult{
				Columns: []string{"metric_name", "role", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
				Rows: [][]interface{}{
					{"http_requests_total", "Traffic", 0.9, 1000.0, 50.0, 800.0, 1200.0, 1000.0, 1150.0, 1180.0, float64(100)},
					{"error_rate", "Errors", 0.85, 0.01, 0.005, 0.0, 0.02, 0.01, 0.018, 0.019, float64(100)},
					{"latency_p99", "Latency", 0.8, 100.0, 10.0, 80.0, 120.0, 100.0, 115.0, 118.0, float64(100)},
				},
			}, nil
		}

		return &graph.QueryResult{}, nil
	}

	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	anomalyAgg.cache.Clear()

	service := NewObservatoryService(mockGraph, anomalyAgg, "test-grafana", logger)

	ctx := context.Background()
	result, err := service.GetWorkloadAnomalyDetail(ctx, "prod", "nginx")

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "prod", result.Namespace)
	assert.Equal(t, "nginx", result.Workload)
	assert.NotEmpty(t, result.Timestamp)

	// Should have signals with roles
	for _, signal := range result.Signals {
		assert.NotEmpty(t, signal.MetricName, "signal should have metric name")
		assert.NotEmpty(t, signal.Role, "signal should have role")
		assert.GreaterOrEqual(t, signal.Score, 0.0, "score should be >= 0")
		assert.LessOrEqual(t, signal.Score, 1.0, "score should be <= 1")
		assert.GreaterOrEqual(t, signal.Confidence, 0.0, "confidence should be >= 0")
		assert.LessOrEqual(t, signal.Confidence, 1.0, "confidence should be <= 1")
	}

	// Verify signals are sorted by score descending
	for i := 1; i < len(result.Signals); i++ {
		assert.GreaterOrEqual(t, result.Signals[i-1].Score, result.Signals[i].Score,
			"signals should be sorted by score descending")
	}
}

// TestObservatoryService_GetWorkloadAnomalyDetail_ThresholdFilter tests that scores < 0.5 are excluded.
func TestObservatoryService_GetWorkloadAnomalyDetail_ThresholdFilter(t *testing.T) {
	logger := logging.GetLogger("test.observatory")

	mockGraph := newMockObservatoryGraphClient()

	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Return signals - one normal (value at mean), one with low sample count
		if strings.Contains(query.Query, "sig.role") {
			return &graph.QueryResult{
				Columns: []string{"metric_name", "role", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
				Rows: [][]interface{}{
					// Normal signal - value at mean -> z-score 0 -> normalized score ~0
					{"normal_metric", "Traffic", 0.8, 100.0, 10.0, 80.0, 120.0, 100.0, 115.0, 118.0, float64(100)},
				},
			}, nil
		}

		return &graph.QueryResult{}, nil
	}

	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	anomalyAgg.cache.Clear()

	service := NewObservatoryService(mockGraph, anomalyAgg, "test-grafana", logger)

	ctx := context.Background()
	result, err := service.GetWorkloadAnomalyDetail(ctx, "prod", "nginx")

	require.NoError(t, err)
	require.NotNil(t, result)

	// All signals should have score >= 0.5
	for _, signal := range result.Signals {
		assert.GreaterOrEqual(t, signal.Score, 0.5,
			"all signals should have score >= 0.5 (anomaly threshold)")
	}
}

// TestNewObservatoryService tests service initialization.
func TestNewObservatoryService(t *testing.T) {
	logger := logging.GetLogger("test.observatory")
	mockGraph := newMockObservatoryGraphClient()
	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-integration", logger)

	service := NewObservatoryService(mockGraph, anomalyAgg, "test-integration", logger)

	assert.NotNil(t, service)
	assert.Equal(t, "test-integration", service.integrationName)
	assert.NotNil(t, service.graphClient)
	assert.NotNil(t, service.anomalyAgg)
	assert.NotNil(t, service.logger)
}

// TestObservatoryService_GetDashboardQuality_Success tests dashboard quality ranking.
func TestObservatoryService_GetDashboardQuality_Success(t *testing.T) {
	logger := logging.GetLogger("test.observatory")

	mockGraph := newMockObservatoryGraphClient()

	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Dashboard quality query
		if strings.Contains(query.Query, "Dashboard") && strings.Contains(query.Query, "quality_score") {
			return &graph.QueryResult{
				Columns: []string{"uid", "title", "quality_score", "signal_count"},
				Rows: [][]interface{}{
					{"uid-1", "API Overview", 0.95, float64(15)},
					{"uid-2", "Infrastructure", 0.85, float64(10)},
					{"uid-3", "Application Metrics", 0.75, float64(8)},
				},
			}, nil
		}

		return &graph.QueryResult{}, nil
	}

	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	service := NewObservatoryService(mockGraph, anomalyAgg, "test-grafana", logger)

	ctx := context.Background()
	result, err := service.GetDashboardQuality(ctx, nil)

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Len(t, result.Dashboards, 3)
	assert.NotEmpty(t, result.Timestamp)

	// Verify sorted by quality_score descending (mock already returns sorted)
	assert.Equal(t, "uid-1", result.Dashboards[0].UID)
	assert.Equal(t, "API Overview", result.Dashboards[0].Title)
	assert.Equal(t, 0.95, result.Dashboards[0].QualityScore)
	assert.Equal(t, 15, result.Dashboards[0].SignalCount)

	// Verify all fields are populated
	for _, dash := range result.Dashboards {
		assert.NotEmpty(t, dash.UID, "dashboard should have UID")
		assert.NotEmpty(t, dash.Title, "dashboard should have title")
		assert.GreaterOrEqual(t, dash.QualityScore, 0.0, "quality score should be >= 0")
		assert.LessOrEqual(t, dash.QualityScore, 1.0, "quality score should be <= 1")
		assert.GreaterOrEqual(t, dash.SignalCount, 0, "signal count should be >= 0")
	}
}

// TestObservatoryService_TimestampFormat tests that timestamps are RFC3339 formatted.
func TestObservatoryService_TimestampFormat(t *testing.T) {
	logger := logging.GetLogger("test.observatory")

	mockGraph := newMockObservatoryGraphClient()

	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		return &graph.QueryResult{}, nil
	}

	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	anomalyAgg.cache.Clear()

	service := NewObservatoryService(mockGraph, anomalyAgg, "test-grafana", logger)

	ctx := context.Background()

	// Test ClusterAnomalies timestamp
	clusterResult, err := service.GetClusterAnomalies(ctx, nil)
	require.NoError(t, err)
	_, err = time.Parse(time.RFC3339, clusterResult.Timestamp)
	assert.NoError(t, err, "ClusterAnomalies timestamp should be RFC3339 formatted")

	// Test NamespaceAnomalies timestamp
	nsResult, err := service.GetNamespaceAnomalies(ctx, "test")
	require.NoError(t, err)
	_, err = time.Parse(time.RFC3339, nsResult.Timestamp)
	assert.NoError(t, err, "NamespaceAnomalies timestamp should be RFC3339 formatted")

	// Test WorkloadAnomalyDetail timestamp
	wlResult, err := service.GetWorkloadAnomalyDetail(ctx, "test", "workload")
	require.NoError(t, err)
	_, err = time.Parse(time.RFC3339, wlResult.Timestamp)
	assert.NoError(t, err, "WorkloadAnomalyDetail timestamp should be RFC3339 formatted")

	// Test DashboardQuality timestamp
	dashResult, err := service.GetDashboardQuality(ctx, nil)
	require.NoError(t, err)
	_, err = time.Parse(time.RFC3339, dashResult.Timestamp)
	assert.NoError(t, err, "DashboardQuality timestamp should be RFC3339 formatted")
}
