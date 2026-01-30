package grafana

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockInvestigateToolGraphClient implements graph.Client for tool tests.
// Separate from service tests to allow independent mock behavior.
type mockInvestigateToolGraphClient struct {
	executeQueryFunc func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error)
	queries          []graph.GraphQuery
}

func newMockInvestigateToolGraphClient() *mockInvestigateToolGraphClient {
	return &mockInvestigateToolGraphClient{
		queries: make([]graph.GraphQuery, 0),
	}
}

func (m *mockInvestigateToolGraphClient) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	m.queries = append(m.queries, query)
	if m.executeQueryFunc != nil {
		return m.executeQueryFunc(ctx, query)
	}
	return &graph.QueryResult{}, nil
}

// Implement remaining graph.Client interface methods
func (m *mockInvestigateToolGraphClient) Connect(ctx context.Context) error { return nil }
func (m *mockInvestigateToolGraphClient) Close() error                      { return nil }
func (m *mockInvestigateToolGraphClient) Ping(ctx context.Context) error    { return nil }
func (m *mockInvestigateToolGraphClient) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}
func (m *mockInvestigateToolGraphClient) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}
func (m *mockInvestigateToolGraphClient) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}
func (m *mockInvestigateToolGraphClient) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}
func (m *mockInvestigateToolGraphClient) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}
func (m *mockInvestigateToolGraphClient) InitializeSchema(ctx context.Context) error { return nil }
func (m *mockInvestigateToolGraphClient) DeleteGraph(ctx context.Context) error      { return nil }
func (m *mockInvestigateToolGraphClient) CreateGraph(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockInvestigateToolGraphClient) DeleteGraphByName(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockInvestigateToolGraphClient) GraphExists(ctx context.Context, graphName string) (bool, error) {
	return false, nil
}

// mockToolQueryService implements QueryService for tool tests.
type mockToolQueryService struct {
	currentValueFunc    func(ctx context.Context, metricName, namespace, workload string) (float64, error)
	historicalValueFunc func(ctx context.Context, metricName, namespace, workload string, lookback time.Duration) (float64, error)
}

func (m *mockToolQueryService) FetchCurrentValue(ctx context.Context, metricName, namespace, workload string) (float64, error) {
	if m.currentValueFunc != nil {
		return m.currentValueFunc(ctx, metricName, namespace, workload)
	}
	return 0, errors.New("not implemented")
}

func (m *mockToolQueryService) FetchHistoricalValue(ctx context.Context, metricName, namespace, workload string, lookback time.Duration) (float64, error) {
	if m.historicalValueFunc != nil {
		return m.historicalValueFunc(ctx, metricName, namespace, workload, lookback)
	}
	return 0, errors.New("not implemented")
}

// =============================================================================
// ObservatorySignalDetailTool Tests
// =============================================================================

// TestObservatorySignalDetailTool_Execute_Success tests successful signal detail retrieval.
func TestObservatorySignalDetailTool_Execute_Success(t *testing.T) {
	logger := logging.GetLogger("test.signal_detail")

	mockGraph := newMockInvestigateToolGraphClient()
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

	mockQS := &mockToolQueryService{
		currentValueFunc: func(ctx context.Context, metricName, namespace, workload string) (float64, error) {
			return 300.0, nil // Slightly elevated value
		},
	}

	service := NewObservatoryInvestigateService(mockGraph, mockQS, "test-grafana", logger)
	tool := NewObservatorySignalDetailTool(service, logger)

	params := ObservatorySignalDetailParams{
		Namespace:  "default",
		Workload:   "nginx",
		MetricName: "http_request_duration_seconds",
	}
	argsJSON, _ := json.Marshal(params)

	ctx := context.Background()
	result, err := tool.Execute(ctx, argsJSON)

	require.NoError(t, err)
	require.NotNil(t, result)

	response, ok := result.(*ObservatorySignalDetailResponse)
	require.True(t, ok, "result should be ObservatorySignalDetailResponse")

	assert.Equal(t, "http_request_duration_seconds", response.MetricName)
	assert.Equal(t, "Latency", response.Role)
	assert.Equal(t, 300.0, response.CurrentValue)
	assert.Equal(t, 0.85, response.QualityScore)
	assert.Equal(t, "dashboard-abc123", response.SourceDashboard)

	// Verify baseline stats
	assert.Equal(t, 250.0, response.Baseline.Mean)
	assert.Equal(t, 50.0, response.Baseline.StdDev)
	assert.Equal(t, 240.0, response.Baseline.P50)
	assert.Equal(t, 350.0, response.Baseline.P90)
	assert.Equal(t, 450.0, response.Baseline.P99)
	assert.Equal(t, 150, response.Baseline.SampleCount)

	// Verify anomaly score and confidence are computed
	assert.GreaterOrEqual(t, response.AnomalyScore, 0.0)
	assert.LessOrEqual(t, response.AnomalyScore, 1.0)
	assert.Greater(t, response.Confidence, 0.0, "should have positive confidence with sufficient samples")

	// Verify timestamp is set
	assert.NotEmpty(t, response.Timestamp)
}

// TestObservatorySignalDetailTool_Execute_NotFound tests error handling for missing signal.
func TestObservatorySignalDetailTool_Execute_NotFound(t *testing.T) {
	logger := logging.GetLogger("test.signal_detail")

	mockGraph := newMockInvestigateToolGraphClient()
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
	tool := NewObservatorySignalDetailTool(service, logger)

	params := ObservatorySignalDetailParams{
		Namespace:  "default",
		Workload:   "nginx",
		MetricName: "nonexistent_metric",
	}
	argsJSON, _ := json.Marshal(params)

	ctx := context.Background()
	result, err := tool.Execute(ctx, argsJSON)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "signal not found")
}

// TestObservatorySignalDetailTool_Execute_InsufficientBaseline tests partial data return for cold start.
func TestObservatorySignalDetailTool_Execute_InsufficientBaseline(t *testing.T) {
	logger := logging.GetLogger("test.signal_detail")

	mockGraph := newMockInvestigateToolGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Return signal with no baseline (nil values)
		return &graph.QueryResult{
			Columns: []string{
				"role", "quality_score", "dashboard_uid",
				"mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count",
			},
			Rows: [][]interface{}{
				// Signal exists but has no baseline
				{"Latency", 0.8, "dashboard-123", nil, nil, nil, nil, nil, nil, nil, nil},
			},
		}, nil
	}

	service := NewObservatoryInvestigateService(mockGraph, nil, "test-grafana", logger)
	tool := NewObservatorySignalDetailTool(service, logger)

	params := ObservatorySignalDetailParams{
		Namespace:  "default",
		Workload:   "nginx",
		MetricName: "new_metric",
	}
	argsJSON, _ := json.Marshal(params)

	ctx := context.Background()
	result, err := tool.Execute(ctx, argsJSON)

	// Should return partial data, not error
	require.NoError(t, err)
	require.NotNil(t, result)

	response, ok := result.(*ObservatorySignalDetailResponse)
	require.True(t, ok)

	// Confidence should be 0 to indicate insufficient data
	assert.Equal(t, 0.0, response.Confidence, "confidence should be 0 for insufficient baseline")
	assert.Equal(t, "new_metric", response.MetricName)
}

// TestObservatorySignalDetailTool_Execute_MissingParams tests parameter validation.
func TestObservatorySignalDetailTool_Execute_MissingParams(t *testing.T) {
	logger := logging.GetLogger("test.signal_detail")
	mockGraph := newMockInvestigateToolGraphClient()
	service := NewObservatoryInvestigateService(mockGraph, nil, "test-grafana", logger)
	tool := NewObservatorySignalDetailTool(service, logger)

	ctx := context.Background()

	testCases := []struct {
		name     string
		params   ObservatorySignalDetailParams
		expected string
	}{
		{
			name:     "missing namespace",
			params:   ObservatorySignalDetailParams{Workload: "nginx", MetricName: "cpu"},
			expected: "namespace is required",
		},
		{
			name:     "missing workload",
			params:   ObservatorySignalDetailParams{Namespace: "default", MetricName: "cpu"},
			expected: "workload is required",
		},
		{
			name:     "missing metric_name",
			params:   ObservatorySignalDetailParams{Namespace: "default", Workload: "nginx"},
			expected: "metric_name is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			argsJSON, _ := json.Marshal(tc.params)
			result, err := tool.Execute(ctx, argsJSON)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tc.expected)
		})
	}
}

// =============================================================================
// ObservatoryCompareTool Tests
// =============================================================================

// TestObservatoryCompareTool_Execute_Success tests successful signal comparison.
func TestObservatoryCompareTool_Execute_Success(t *testing.T) {
	logger := logging.GetLogger("test.compare")

	mockGraph := newMockInvestigateToolGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
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

	mockQS := &mockToolQueryService{
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
	tool := NewObservatoryCompareTool(service, logger)

	params := ObservatoryCompareParams{
		Namespace:  "default",
		Workload:   "api",
		MetricName: "http_requests_errors_total",
		Lookback:   "12h",
	}
	argsJSON, _ := json.Marshal(params)

	ctx := context.Background()
	result, err := tool.Execute(ctx, argsJSON)

	require.NoError(t, err)
	require.NotNil(t, result)

	response, ok := result.(*ObservatoryCompareResponse)
	require.True(t, ok, "result should be ObservatoryCompareResponse")

	assert.Equal(t, "http_requests_errors_total", response.MetricName)
	assert.Equal(t, 0.08, response.CurrentValue)
	assert.Equal(t, 0.01, response.PastValue)
	assert.Equal(t, 12, response.LookbackHours)

	// Current value is anomalous, past is normal - score should increase
	assert.Greater(t, response.CurrentScore, response.PastScore, "current anomalous value should have higher score")
	assert.Greater(t, response.ScoreDelta, 0.0, "score delta should be positive (getting worse)")

	// Verify timestamp
	assert.NotEmpty(t, response.Timestamp)
}

// TestObservatoryCompareTool_Execute_DefaultLookback tests that 24h is used when not specified.
func TestObservatoryCompareTool_Execute_DefaultLookback(t *testing.T) {
	logger := logging.GetLogger("test.compare")

	mockGraph := newMockInvestigateToolGraphClient()
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
	mockQS := &mockToolQueryService{
		currentValueFunc: func(ctx context.Context, metricName, namespace, workload string) (float64, error) {
			return 99.9, nil
		},
		historicalValueFunc: func(ctx context.Context, metricName, namespace, workload string, lookback time.Duration) (float64, error) {
			capturedLookback = lookback
			return 99.9, nil
		},
	}

	service := NewObservatoryInvestigateService(mockGraph, mockQS, "test-grafana", logger)
	tool := NewObservatoryCompareTool(service, logger)

	// No lookback specified - should use default
	params := ObservatoryCompareParams{
		Namespace:  "default",
		Workload:   "nginx",
		MetricName: "uptime_percent",
	}
	argsJSON, _ := json.Marshal(params)

	ctx := context.Background()
	result, err := tool.Execute(ctx, argsJSON)

	require.NoError(t, err)
	require.NotNil(t, result)

	response, ok := result.(*ObservatoryCompareResponse)
	require.True(t, ok)

	assert.Equal(t, 24*time.Hour, capturedLookback, "should use 24h default lookback")
	assert.Equal(t, 24, response.LookbackHours)
}

// TestObservatoryCompareTool_Execute_ScoreDelta tests score delta calculation.
func TestObservatoryCompareTool_Execute_ScoreDelta(t *testing.T) {
	logger := logging.GetLogger("test.compare")

	mockGraph := newMockInvestigateToolGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		return &graph.QueryResult{
			Columns: []string{
				"role", "quality_score", "dashboard_uid",
				"mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count",
			},
			Rows: [][]interface{}{
				{"Latency", 0.8, "dashboard-123", 100.0, 20.0, 50.0, 200.0, 100.0, 150.0, 180.0, float64(100)},
			},
		}, nil
	}

	testCases := []struct {
		name               string
		currentValue       float64
		pastValue          float64
		expectPositiveDelta bool // positive means worsening
	}{
		{
			name:               "worsening - higher current score",
			currentValue:       500.0, // Far above mean -> high anomaly
			pastValue:          100.0, // At mean -> low anomaly
			expectPositiveDelta: true,
		},
		{
			name:               "improving - lower current score",
			currentValue:       100.0, // At mean -> low anomaly
			pastValue:          500.0, // Far above mean -> high anomaly
			expectPositiveDelta: false,
		},
		{
			name:               "stable - same values",
			currentValue:       100.0,
			pastValue:          100.0,
			expectPositiveDelta: false, // Score delta should be ~0
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockQS := &mockToolQueryService{
				currentValueFunc: func(ctx context.Context, metricName, namespace, workload string) (float64, error) {
					return tc.currentValue, nil
				},
				historicalValueFunc: func(ctx context.Context, metricName, namespace, workload string, lookback time.Duration) (float64, error) {
					return tc.pastValue, nil
				},
			}

			service := NewObservatoryInvestigateService(mockGraph, mockQS, "test-grafana", logger)
			tool := NewObservatoryCompareTool(service, logger)

			params := ObservatoryCompareParams{
				Namespace:  "default",
				Workload:   "nginx",
				MetricName: "http_latency",
				Lookback:   "24h",
			}
			argsJSON, _ := json.Marshal(params)

			ctx := context.Background()
			result, err := tool.Execute(ctx, argsJSON)

			require.NoError(t, err)
			require.NotNil(t, result)

			response, ok := result.(*ObservatoryCompareResponse)
			require.True(t, ok)

			if tc.expectPositiveDelta {
				assert.Greater(t, response.ScoreDelta, 0.0, "score delta should be positive (worsening)")
			} else if tc.name == "stable - same values" {
				// For stable case, delta should be approximately 0
				assert.InDelta(t, 0.0, response.ScoreDelta, 0.01, "score delta should be ~0 for stable")
			} else {
				assert.Less(t, response.ScoreDelta, 0.0, "score delta should be negative (improving)")
			}
		})
	}
}

// TestObservatoryCompareTool_Execute_MaxLookback tests that lookback is capped at 7 days.
func TestObservatoryCompareTool_Execute_MaxLookback(t *testing.T) {
	logger := logging.GetLogger("test.compare")

	mockGraph := newMockInvestigateToolGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		return &graph.QueryResult{
			Columns: []string{
				"role", "quality_score", "dashboard_uid",
				"mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count",
			},
			Rows: [][]interface{}{
				{"Traffic", 0.8, "dashboard-123", 1000.0, 100.0, 500.0, 1500.0, 1000.0, 1200.0, 1400.0, float64(100)},
			},
		}, nil
	}

	var capturedLookback time.Duration
	mockQS := &mockToolQueryService{
		currentValueFunc: func(ctx context.Context, metricName, namespace, workload string) (float64, error) {
			return 1000.0, nil
		},
		historicalValueFunc: func(ctx context.Context, metricName, namespace, workload string, lookback time.Duration) (float64, error) {
			capturedLookback = lookback
			return 1000.0, nil
		},
	}

	service := NewObservatoryInvestigateService(mockGraph, mockQS, "test-grafana", logger)
	tool := NewObservatoryCompareTool(service, logger)

	// Request 30 days (720h) - should be capped to 168h (7 days)
	params := ObservatoryCompareParams{
		Namespace:  "default",
		Workload:   "nginx",
		MetricName: "requests_total",
		Lookback:   "720h", // 30 days
	}
	argsJSON, _ := json.Marshal(params)

	ctx := context.Background()
	result, err := tool.Execute(ctx, argsJSON)

	require.NoError(t, err)
	require.NotNil(t, result)

	response, ok := result.(*ObservatoryCompareResponse)
	require.True(t, ok)

	assert.Equal(t, 168*time.Hour, capturedLookback, "lookback should be capped at 168h (7 days)")
	assert.Equal(t, 168, response.LookbackHours)
}

// TestObservatoryCompareTool_Execute_MissingParams tests parameter validation.
func TestObservatoryCompareTool_Execute_MissingParams(t *testing.T) {
	logger := logging.GetLogger("test.compare")
	mockGraph := newMockInvestigateToolGraphClient()
	service := NewObservatoryInvestigateService(mockGraph, nil, "test-grafana", logger)
	tool := NewObservatoryCompareTool(service, logger)

	ctx := context.Background()

	testCases := []struct {
		name     string
		params   ObservatoryCompareParams
		expected string
	}{
		{
			name:     "missing namespace",
			params:   ObservatoryCompareParams{Workload: "nginx", MetricName: "cpu"},
			expected: "namespace is required",
		},
		{
			name:     "missing workload",
			params:   ObservatoryCompareParams{Namespace: "default", MetricName: "cpu"},
			expected: "workload is required",
		},
		{
			name:     "missing metric_name",
			params:   ObservatoryCompareParams{Namespace: "default", Workload: "nginx"},
			expected: "metric_name is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			argsJSON, _ := json.Marshal(tc.params)
			result, err := tool.Execute(ctx, argsJSON)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tc.expected)
		})
	}
}

// TestObservatoryCompareTool_Execute_InvalidLookback tests invalid lookback handling.
func TestObservatoryCompareTool_Execute_InvalidLookback(t *testing.T) {
	logger := logging.GetLogger("test.compare")
	mockGraph := newMockInvestigateToolGraphClient()
	service := NewObservatoryInvestigateService(mockGraph, nil, "test-grafana", logger)
	tool := NewObservatoryCompareTool(service, logger)

	ctx := context.Background()

	testCases := []struct {
		name     string
		lookback string
		expected string
	}{
		{
			name:     "invalid format",
			lookback: "invalid",
			expected: "invalid lookback duration",
		},
		{
			name:     "negative duration",
			lookback: "-24h",
			expected: "lookback must be positive",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params := ObservatoryCompareParams{
				Namespace:  "default",
				Workload:   "nginx",
				MetricName: "cpu",
				Lookback:   tc.lookback,
			}
			argsJSON, _ := json.Marshal(params)

			result, err := tool.Execute(ctx, argsJSON)

			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), tc.expected)
		})
	}
}
