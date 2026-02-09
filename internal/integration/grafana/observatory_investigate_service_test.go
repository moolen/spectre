package grafana

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockInvestigateGraphClient implements graph.Client for investigate service tests.
type mockInvestigateGraphClient struct {
	executeQueryFunc func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error)
	queries          []graph.GraphQuery
}

func newMockInvestigateGraphClient() *mockInvestigateGraphClient {
	return &mockInvestigateGraphClient{
		queries: make([]graph.GraphQuery, 0),
	}
}

func (m *mockInvestigateGraphClient) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	m.queries = append(m.queries, query)
	if m.executeQueryFunc != nil {
		return m.executeQueryFunc(ctx, query)
	}
	return &graph.QueryResult{}, nil
}

// Implement remaining graph.Client interface methods
func (m *mockInvestigateGraphClient) Connect(ctx context.Context) error { return nil }
func (m *mockInvestigateGraphClient) Close() error                      { return nil }
func (m *mockInvestigateGraphClient) Ping(ctx context.Context) error    { return nil }
func (m *mockInvestigateGraphClient) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}
func (m *mockInvestigateGraphClient) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}
func (m *mockInvestigateGraphClient) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}
func (m *mockInvestigateGraphClient) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}
func (m *mockInvestigateGraphClient) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}
func (m *mockInvestigateGraphClient) InitializeSchema(ctx context.Context) error { return nil }
func (m *mockInvestigateGraphClient) DeleteGraph(ctx context.Context) error      { return nil }
func (m *mockInvestigateGraphClient) CreateGraph(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockInvestigateGraphClient) DeleteGraphByName(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockInvestigateGraphClient) GraphExists(ctx context.Context, graphName string) (bool, error) {
	return false, nil
}

// mockQueryService implements QueryService for testing.
type mockQueryService struct {
	currentValueFunc    func(ctx context.Context, metricName, namespace, workload string) (float64, error)
	historicalValueFunc func(ctx context.Context, metricName, namespace, workload string, lookback time.Duration) (float64, error)
}

func (m *mockQueryService) FetchCurrentValue(ctx context.Context, metricName, namespace, workload string) (float64, error) {
	if m.currentValueFunc != nil {
		return m.currentValueFunc(ctx, metricName, namespace, workload)
	}
	return 0, errors.New("not implemented")
}

func (m *mockQueryService) FetchHistoricalValue(ctx context.Context, metricName, namespace, workload string, lookback time.Duration) (float64, error) {
	if m.historicalValueFunc != nil {
		return m.historicalValueFunc(ctx, metricName, namespace, workload, lookback)
	}
	return 0, errors.New("not implemented")
}

// TestInvestigateService_GetWorkloadSignals_Success tests that GetWorkloadSignals returns signals sorted by score.
func TestInvestigateService_GetWorkloadSignals_Success(t *testing.T) {
	logger := logging.GetLogger("test.investigate")

	mockGraph := newMockInvestigateGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Return multiple signals with different anomaly characteristics
		// Signal with low stddev and value at mean will have low z-score
		// Signal with deviation from mean will have higher z-score
		return &graph.QueryResult{
			Columns: []string{
				"metric_name", "role", "quality_score",
				"mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count",
			},
			Rows: [][]interface{}{
				// Normal signal: value at mean -> low z-score -> low anomaly score
				{"cpu_normal", "Saturation", 0.8, 100.0, 10.0, 80.0, 120.0, 100.0, 115.0, 118.0, float64(100)},
				// High quality signal: also at mean
				{"cpu_high_quality", "Saturation", 0.9, 100.0, 10.0, 80.0, 120.0, 100.0, 115.0, 118.0, float64(100)},
				// Error rate signal: also at mean
				{"error_rate", "Errors", 0.7, 0.01, 0.005, 0.0, 0.05, 0.01, 0.03, 0.04, float64(100)},
			},
		}, nil
	}

	service := NewObservatoryInvestigateService(mockGraph, nil, "test-grafana", logger)

	ctx := context.Background()
	result, err := service.GetWorkloadSignals(ctx, "default", "nginx")

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "default/nginx", result.Scope)
	assert.Len(t, result.Signals, 3, "should return all 3 signals")

	// Verify signals are sorted by score descending (same score -> higher confidence wins)
	// All signals have value at mean, so scores should be similar
	// Tiebreaker is confidence which depends on quality score
	for i := 0; i < len(result.Signals)-1; i++ {
		if result.Signals[i].Score == result.Signals[i+1].Score {
			assert.GreaterOrEqual(t, result.Signals[i].Confidence, result.Signals[i+1].Confidence,
				"when scores equal, higher confidence should come first")
		} else {
			assert.Greater(t, result.Signals[i].Score, result.Signals[i+1].Score,
				"signals should be sorted by score descending")
		}
	}

	// Check roles were extracted correctly
	roles := make(map[string]string)
	for _, sig := range result.Signals {
		roles[sig.MetricName] = sig.Role
	}
	assert.Equal(t, "Saturation", roles["cpu_normal"])
	assert.Equal(t, "Errors", roles["error_rate"])
}

// TestInvestigateService_GetWorkloadSignals_SkipsColdStart tests that signals without baseline are skipped.
func TestInvestigateService_GetWorkloadSignals_SkipsColdStart(t *testing.T) {
	logger := logging.GetLogger("test.investigate")

	mockGraph := newMockInvestigateGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Return one signal with baseline, one without (sample_count = nil), one with insufficient samples
		return &graph.QueryResult{
			Columns: []string{
				"metric_name", "role", "quality_score",
				"mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count",
			},
			Rows: [][]interface{}{
				// Signal with baseline
				{"with_baseline", "Availability", 0.8, 100.0, 10.0, 80.0, 120.0, 100.0, 115.0, 118.0, float64(100)},
				// Signal without baseline (nil values)
				{"without_baseline", "Latency", 0.9, nil, nil, nil, nil, nil, nil, nil, nil},
				// Signal with insufficient samples (< 10)
				{"insufficient_samples", "Errors", 0.7, 50.0, 5.0, 40.0, 60.0, 50.0, 55.0, 58.0, float64(5)},
			},
		}, nil
	}

	service := NewObservatoryInvestigateService(mockGraph, nil, "test-grafana", logger)

	ctx := context.Background()
	result, err := service.GetWorkloadSignals(ctx, "default", "nginx")

	require.NoError(t, err)
	require.NotNil(t, result)

	// Only the signal with valid baseline should be returned
	assert.Len(t, result.Signals, 1, "only signal with valid baseline should be counted")
	assert.Equal(t, "with_baseline", result.Signals[0].MetricName)
}

// TestInvestigateService_GetSignalDetail_Success tests that GetSignalDetail returns full detail with baseline.
func TestInvestigateService_GetSignalDetail_Success(t *testing.T) {
	logger := logging.GetLogger("test.investigate")

	mockGraph := newMockInvestigateGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		return &graph.QueryResult{
			Columns: []string{
				"role", "quality_score", "dashboard_uid",
				"mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count",
			},
			Rows: [][]interface{}{
				{"Latency", 0.85, "dashboard-abc123", 250.0, 50.0, 100.0, 500.0, 240.0, 350.0, 450.0, float64(150)},
			},
		}, nil
	}

	mockQS := &mockQueryService{
		currentValueFunc: func(ctx context.Context, metricName, namespace, workload string) (float64, error) {
			// Return a value that's above P99 to trigger anomaly
			return 600.0, nil
		},
	}

	service := NewObservatoryInvestigateService(mockGraph, mockQS, "test-grafana", logger)

	ctx := context.Background()
	result, err := service.GetSignalDetail(ctx, "default", "nginx", "http_request_duration_seconds")

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "http_request_duration_seconds", result.MetricName)
	assert.Equal(t, "Latency", result.Role)
	assert.Equal(t, 600.0, result.CurrentValue)
	assert.Equal(t, 0.85, result.QualityScore)
	assert.Equal(t, "dashboard-abc123", result.SourceDashboard)

	// Check baseline stats
	assert.Equal(t, 250.0, result.Baseline.Mean)
	assert.Equal(t, 50.0, result.Baseline.StdDev)
	assert.Equal(t, 240.0, result.Baseline.P50)
	assert.Equal(t, 350.0, result.Baseline.P90)
	assert.Equal(t, 450.0, result.Baseline.P99)
	assert.Equal(t, 150, result.Baseline.SampleCount)

	// Value of 600 is above P99 (450) and 7 stddevs from mean
	// Should have high anomaly score
	assert.Greater(t, result.AnomalyScore, 0.5, "value above P99 should have high anomaly score")
}

// TestInvestigateService_GetSignalDetail_NotFound tests that error is returned for missing signal.
func TestInvestigateService_GetSignalDetail_NotFound(t *testing.T) {
	logger := logging.GetLogger("test.investigate")

	mockGraph := newMockInvestigateGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Return empty result
		return &graph.QueryResult{
			Columns: []string{
				"role", "quality_score", "dashboard_uid",
				"mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count",
			},
			Rows: [][]interface{}{},
		}, nil
	}

	service := NewObservatoryInvestigateService(mockGraph, nil, "test-grafana", logger)

	ctx := context.Background()
	result, err := service.GetSignalDetail(ctx, "default", "nginx", "nonexistent_metric")

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "signal not found")
}

// TestInvestigateService_CompareSignal_Success tests time comparison with score delta.
func TestInvestigateService_CompareSignal_Success(t *testing.T) {
	logger := logging.GetLogger("test.investigate")

	mockGraph := newMockInvestigateGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Return signal with baseline
		return &graph.QueryResult{
			Columns: []string{
				"role", "quality_score", "dashboard_uid",
				"mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count",
			},
			Rows: [][]interface{}{
				{"Errors", 0.8, "dashboard-xyz", 0.01, 0.005, 0.0, 0.05, 0.01, 0.03, 0.04, float64(100)},
			},
		}, nil
	}

	mockQS := &mockQueryService{
		currentValueFunc: func(ctx context.Context, metricName, namespace, workload string) (float64, error) {
			// Current value is anomalous (high error rate)
			return 0.08, nil
		},
		historicalValueFunc: func(ctx context.Context, metricName, namespace, workload string, lookback time.Duration) (float64, error) {
			// Historical value was normal (at mean)
			return 0.01, nil
		},
	}

	service := NewObservatoryInvestigateService(mockGraph, mockQS, "test-grafana", logger)

	ctx := context.Background()
	result, err := service.CompareSignal(ctx, "default", "api", "http_requests_errors_total", 12*time.Hour)

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "http_requests_errors_total", result.MetricName)
	assert.Equal(t, 0.08, result.CurrentValue)
	assert.Equal(t, 0.01, result.PastValue)
	assert.Equal(t, 12, result.LookbackHours)

	// Current value is anomalous (far from mean), past value is at mean
	assert.Greater(t, result.CurrentScore, result.PastScore, "current anomalous value should have higher score than past normal value")
	assert.Greater(t, result.ScoreDelta, 0.0, "score delta should be positive (getting worse)")
}

// TestInvestigateService_CompareSignal_DefaultLookback tests that 24h is used when not specified.
func TestInvestigateService_CompareSignal_DefaultLookback(t *testing.T) {
	logger := logging.GetLogger("test.investigate")

	mockGraph := newMockInvestigateGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		return &graph.QueryResult{
			Columns: []string{
				"role", "quality_score", "dashboard_uid",
				"mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count",
			},
			Rows: [][]interface{}{
				{"Availability", 0.8, "dashboard-123", 99.9, 0.1, 99.5, 100.0, 99.9, 99.95, 99.99, float64(100)},
			},
		}, nil
	}

	var capturedLookback time.Duration
	mockQS := &mockQueryService{
		currentValueFunc: func(ctx context.Context, metricName, namespace, workload string) (float64, error) {
			return 99.9, nil
		},
		historicalValueFunc: func(ctx context.Context, metricName, namespace, workload string, lookback time.Duration) (float64, error) {
			capturedLookback = lookback
			return 99.9, nil
		},
	}

	service := NewObservatoryInvestigateService(mockGraph, mockQS, "test-grafana", logger)

	ctx := context.Background()
	// Pass 0 duration to test default
	result, err := service.CompareSignal(ctx, "default", "nginx", "uptime_percent", 0)

	require.NoError(t, err)
	require.NotNil(t, result)

	// Should use default 24h lookback
	assert.Equal(t, 24*time.Hour, capturedLookback, "should use 24h default lookback")
	assert.Equal(t, 24, result.LookbackHours)
}

// TestInvestigateService_EmptyParams tests validation of required parameters.
func TestInvestigateService_EmptyParams(t *testing.T) {
	logger := logging.GetLogger("test.investigate")
	mockGraph := newMockInvestigateGraphClient()
	service := NewObservatoryInvestigateService(mockGraph, nil, "test-grafana", logger)
	ctx := context.Background()

	// Test GetWorkloadSignals
	_, err := service.GetWorkloadSignals(ctx, "", "nginx")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace and workload are required")

	_, err = service.GetWorkloadSignals(ctx, "default", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace and workload are required")

	// Test GetSignalDetail
	_, err = service.GetSignalDetail(ctx, "", "nginx", "cpu")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace, workload, and metric_name are required")

	_, err = service.GetSignalDetail(ctx, "default", "", "cpu")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace, workload, and metric_name are required")

	_, err = service.GetSignalDetail(ctx, "default", "nginx", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace, workload, and metric_name are required")

	// Test CompareSignal
	_, err = service.CompareSignal(ctx, "", "nginx", "cpu", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace, workload, and metric_name are required")
}

// TestInvestigateService_GetSignalDetail_FallbackToBaseline tests fallback when query service fails.
func TestInvestigateService_GetSignalDetail_FallbackToBaseline(t *testing.T) {
	logger := logging.GetLogger("test.investigate")

	mockGraph := newMockInvestigateGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		return &graph.QueryResult{
			Columns: []string{
				"role", "quality_score", "dashboard_uid",
				"mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count",
			},
			Rows: [][]interface{}{
				{"Latency", 0.8, "dashboard-123", 100.0, 10.0, 80.0, 120.0, 100.0, 115.0, 118.0, float64(100)},
			},
		}, nil
	}

	mockQS := &mockQueryService{
		currentValueFunc: func(ctx context.Context, metricName, namespace, workload string) (float64, error) {
			// Simulate Grafana query failure
			return 0, errors.New("grafana unavailable")
		},
	}

	service := NewObservatoryInvestigateService(mockGraph, mockQS, "test-grafana", logger)

	ctx := context.Background()
	result, err := service.GetSignalDetail(ctx, "default", "nginx", "http_latency")

	// Should succeed despite query service failure
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should fall back to baseline mean as current value
	assert.Equal(t, 100.0, result.CurrentValue, "should use baseline mean as fallback")
}

// TestInvestigateService_GetWorkloadSignals_EmptyResult tests empty result handling.
func TestInvestigateService_GetWorkloadSignals_EmptyResult(t *testing.T) {
	logger := logging.GetLogger("test.investigate")

	mockGraph := newMockInvestigateGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Return empty result (no signals for workload)
		return &graph.QueryResult{
			Columns: []string{
				"metric_name", "role", "quality_score",
				"mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count",
			},
			Rows: [][]interface{}{},
		}, nil
	}

	service := NewObservatoryInvestigateService(mockGraph, nil, "test-grafana", logger)

	ctx := context.Background()
	result, err := service.GetWorkloadSignals(ctx, "default", "nonexistent")

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Empty(t, result.Signals, "should return empty signals list")
	assert.Equal(t, "default/nonexistent", result.Scope)
}
