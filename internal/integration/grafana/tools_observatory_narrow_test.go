package grafana

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockNarrowGraphClient implements graph.Client for narrow tools tests.
type mockNarrowGraphClient struct {
	executeQueryFunc func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error)
	queries          []graph.GraphQuery
}

func newMockNarrowGraphClient() *mockNarrowGraphClient {
	return &mockNarrowGraphClient{
		queries: make([]graph.GraphQuery, 0),
	}
}

func (m *mockNarrowGraphClient) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	m.queries = append(m.queries, query)
	if m.executeQueryFunc != nil {
		return m.executeQueryFunc(ctx, query)
	}
	return &graph.QueryResult{}, nil
}

// Implement remaining graph.Client interface methods
func (m *mockNarrowGraphClient) Connect(ctx context.Context) error { return nil }
func (m *mockNarrowGraphClient) Close() error                      { return nil }
func (m *mockNarrowGraphClient) Ping(ctx context.Context) error    { return nil }
func (m *mockNarrowGraphClient) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}
func (m *mockNarrowGraphClient) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}
func (m *mockNarrowGraphClient) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}
func (m *mockNarrowGraphClient) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}
func (m *mockNarrowGraphClient) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}
func (m *mockNarrowGraphClient) InitializeSchema(ctx context.Context) error { return nil }
func (m *mockNarrowGraphClient) DeleteGraph(ctx context.Context) error      { return nil }
func (m *mockNarrowGraphClient) CreateGraph(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockNarrowGraphClient) DeleteGraphByName(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockNarrowGraphClient) GraphExists(ctx context.Context, graphName string) (bool, error) {
	return false, nil
}

// --- ObservatoryScopeTool Tests ---

func TestObservatoryScopeTool_Execute_NamespaceOnly(t *testing.T) {
	logger := logging.GetLogger("test.narrow")
	ctx := context.Background()

	mockGraph := newMockNarrowGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Workloads in namespace query
		if strings.Contains(query.Query, "DISTINCT") && strings.Contains(query.Query, "workload_name") {
			return &graph.QueryResult{
				Columns: []string{"workload_name"},
				Rows: [][]interface{}{
					{"api-server"},
					{"frontend"},
				},
			}, nil
		}

		// Workload signals query - return anomalous signals (mean > P99)
		if strings.Contains(query.Query, "HAS_BASELINE") {
			workload := query.Parameters["workload_name"].(string)
			if workload == "api-server" {
				return &graph.QueryResult{
					Columns: []string{"metric_name", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
					Rows: [][]interface{}{
						{"cpu_usage", 0.9, 200.0, 10.0, 30.0, 70.0, 50.0, 65.0, 68.0, float64(100)},
						{"memory_usage", 0.8, 150.0, 5.0, 60.0, 80.0, 70.0, 75.0, 78.0, float64(100)},
					},
				}, nil
			}
			if workload == "frontend" {
				return &graph.QueryResult{
					Columns: []string{"metric_name", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
					Rows: [][]interface{}{
						{"http_requests", 0.85, 180.0, 15.0, 80.0, 100.0, 90.0, 95.0, 98.0, float64(100)},
					},
				}, nil
			}
		}

		return &graph.QueryResult{}, nil
	}

	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	service := NewObservatoryService(mockGraph, anomalyAgg, "test-grafana", logger)
	tool := NewObservatoryScopeTool(service, logger)

	args, _ := json.Marshal(ObservatoryScopeParams{Namespace: "prod"})
	result, err := tool.Execute(ctx, args)

	require.NoError(t, err)
	resp, ok := result.(*ObservatoryScopeResponse)
	require.True(t, ok)

	assert.Equal(t, "prod", resp.Scope)
	assert.NotEmpty(t, resp.Timestamp)

	// Check anomalies are returned with workload names
	assert.GreaterOrEqual(t, len(resp.Anomalies), 1)
	// Should have workload field populated at namespace level
	for _, a := range resp.Anomalies {
		assert.NotEmpty(t, a.Workload)
		assert.Greater(t, a.Score, 0.0)
		assert.GreaterOrEqual(t, a.Confidence, 0.0)
	}
}

func TestObservatoryScopeTool_Execute_WithWorkload(t *testing.T) {
	logger := logging.GetLogger("test.narrow")
	ctx := context.Background()

	mockGraph := newMockNarrowGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Workload signals query with baselines and roles
		if strings.Contains(query.Query, "HAS_BASELINE") {
			return &graph.QueryResult{
				Columns: []string{"metric_name", "role", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
				Rows: [][]interface{}{
					{"cpu_usage", "Saturation", 0.9, 200.0, 10.0, 30.0, 70.0, 50.0, 65.0, 68.0, float64(100)},
					{"request_latency", "Latency", 0.85, 150.0, 8.0, 40.0, 60.0, 45.0, 55.0, 58.0, float64(100)},
				},
			}, nil
		}

		return &graph.QueryResult{}, nil
	}

	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	service := NewObservatoryService(mockGraph, anomalyAgg, "test-grafana", logger)
	tool := NewObservatoryScopeTool(service, logger)

	args, _ := json.Marshal(ObservatoryScopeParams{
		Namespace: "prod",
		Workload:  "api-server",
	})
	result, err := tool.Execute(ctx, args)

	require.NoError(t, err)
	resp, ok := result.(*ObservatoryScopeResponse)
	require.True(t, ok)

	assert.Equal(t, "prod/api-server", resp.Scope)
	assert.NotEmpty(t, resp.Timestamp)

	// Check anomalies at workload level (signal-level, no Workload field)
	assert.GreaterOrEqual(t, len(resp.Anomalies), 1)
	for _, a := range resp.Anomalies {
		assert.Empty(t, a.Workload) // Workload omitted at signal level
		assert.NotEmpty(t, a.MetricName)
		assert.NotEmpty(t, a.Role)
		assert.Greater(t, a.Score, 0.0)
	}
}

func TestObservatoryScopeTool_Execute_Empty(t *testing.T) {
	logger := logging.GetLogger("test.narrow")
	ctx := context.Background()

	mockGraph := newMockNarrowGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// No workloads in namespace
		if strings.Contains(query.Query, "DISTINCT") && strings.Contains(query.Query, "workload_name") {
			return &graph.QueryResult{
				Columns: []string{"workload_name"},
				Rows:    [][]interface{}{},
			}, nil
		}
		return &graph.QueryResult{}, nil
	}

	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	service := NewObservatoryService(mockGraph, anomalyAgg, "test-grafana", logger)
	tool := NewObservatoryScopeTool(service, logger)

	args, _ := json.Marshal(ObservatoryScopeParams{Namespace: "empty-ns"})
	result, err := tool.Execute(ctx, args)

	require.NoError(t, err)
	resp, ok := result.(*ObservatoryScopeResponse)
	require.True(t, ok)

	// Empty anomalies array when nothing anomalous
	assert.Equal(t, "empty-ns", resp.Scope)
	assert.Empty(t, resp.Anomalies)
}

func TestObservatoryScopeTool_Execute_MissingNamespace(t *testing.T) {
	logger := logging.GetLogger("test.narrow")
	ctx := context.Background()

	mockGraph := newMockNarrowGraphClient()
	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	service := NewObservatoryService(mockGraph, anomalyAgg, "test-grafana", logger)
	tool := NewObservatoryScopeTool(service, logger)

	// Empty params
	args, _ := json.Marshal(ObservatoryScopeParams{})
	_, err := tool.Execute(ctx, args)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace is required")
}

// --- ObservatorySignalsTool Tests ---

func TestObservatorySignalsTool_Execute_Success(t *testing.T) {
	logger := logging.GetLogger("test.narrow")
	ctx := context.Background()

	mockGraph := newMockNarrowGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Signals query
		if strings.Contains(query.Query, "HAS_BASELINE") {
			return &graph.QueryResult{
				Columns: []string{"metric_name", "role", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
				Rows: [][]interface{}{
					{"cpu_usage", "Saturation", 0.9, 50.0, 10.0, 30.0, 70.0, 50.0, 65.0, 68.0, float64(100)},
					{"memory_usage", "Saturation", 0.85, 60.0, 5.0, 40.0, 80.0, 60.0, 75.0, 78.0, float64(100)},
					{"request_count", "Traffic", 0.8, 1000.0, 100.0, 800.0, 1200.0, 1000.0, 1150.0, 1180.0, float64(100)},
				},
			}, nil
		}
		return &graph.QueryResult{}, nil
	}

	investigateService := NewObservatoryInvestigateService(mockGraph, nil, "test-grafana", logger)
	tool := NewObservatorySignalsTool(investigateService, logger)

	args, _ := json.Marshal(ObservatorySignalsParams{
		Namespace: "prod",
		Workload:  "api-server",
	})
	result, err := tool.Execute(ctx, args)

	require.NoError(t, err)
	resp, ok := result.(*ObservatorySignalsResponse)
	require.True(t, ok)

	assert.Equal(t, "prod/api-server", resp.Scope)
	assert.NotEmpty(t, resp.Timestamp)
	assert.Len(t, resp.Signals, 3)

	// Verify signal fields
	for _, sig := range resp.Signals {
		assert.NotEmpty(t, sig.MetricName)
		assert.NotEmpty(t, sig.Role)
		assert.GreaterOrEqual(t, sig.Score, 0.0)
		assert.GreaterOrEqual(t, sig.Confidence, 0.0)
		assert.GreaterOrEqual(t, sig.QualityScore, 0.0)
	}
}

func TestObservatorySignalsTool_Execute_SortedByScore(t *testing.T) {
	logger := logging.GetLogger("test.narrow")
	ctx := context.Background()

	mockGraph := newMockNarrowGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		if strings.Contains(query.Query, "HAS_BASELINE") {
			// Return signals with varying anomaly levels (mean vs P99 determines score)
			return &graph.QueryResult{
				Columns: []string{"metric_name", "role", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
				Rows: [][]interface{}{
					// Normal (mean within baseline)
					{"metric_a", "Latency", 0.8, 50.0, 10.0, 30.0, 70.0, 50.0, 65.0, 68.0, float64(100)},
					// Highly anomalous (mean much > P99)
					{"metric_b", "Errors", 0.9, 500.0, 10.0, 30.0, 70.0, 50.0, 65.0, 68.0, float64(100)},
					// Moderately anomalous (mean somewhat > P99)
					{"metric_c", "Traffic", 0.85, 100.0, 10.0, 30.0, 70.0, 50.0, 65.0, 68.0, float64(100)},
				},
			}, nil
		}
		return &graph.QueryResult{}, nil
	}

	investigateService := NewObservatoryInvestigateService(mockGraph, nil, "test-grafana", logger)
	tool := NewObservatorySignalsTool(investigateService, logger)

	args, _ := json.Marshal(ObservatorySignalsParams{
		Namespace: "prod",
		Workload:  "api-server",
	})
	result, err := tool.Execute(ctx, args)

	require.NoError(t, err)
	resp, ok := result.(*ObservatorySignalsResponse)
	require.True(t, ok)

	// Verify signals are sorted by score descending
	require.Len(t, resp.Signals, 3)
	for i := 1; i < len(resp.Signals); i++ {
		assert.GreaterOrEqual(t, resp.Signals[i-1].Score, resp.Signals[i].Score,
			"Signals should be sorted by score descending")
	}

	// metric_b should be first (highest anomaly)
	assert.Equal(t, "metric_b", resp.Signals[0].MetricName)
}

func TestObservatorySignalsTool_Execute_Empty(t *testing.T) {
	logger := logging.GetLogger("test.narrow")
	ctx := context.Background()

	mockGraph := newMockNarrowGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// No signals for workload
		if strings.Contains(query.Query, "HAS_BASELINE") {
			return &graph.QueryResult{
				Columns: []string{"metric_name", "role", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
				Rows:    [][]interface{}{},
			}, nil
		}
		return &graph.QueryResult{}, nil
	}

	investigateService := NewObservatoryInvestigateService(mockGraph, nil, "test-grafana", logger)
	tool := NewObservatorySignalsTool(investigateService, logger)

	args, _ := json.Marshal(ObservatorySignalsParams{
		Namespace: "prod",
		Workload:  "empty-workload",
	})
	result, err := tool.Execute(ctx, args)

	require.NoError(t, err)
	resp, ok := result.(*ObservatorySignalsResponse)
	require.True(t, ok)

	// Empty signals array when no signals
	assert.Equal(t, "prod/empty-workload", resp.Scope)
	assert.Empty(t, resp.Signals)
}

func TestObservatorySignalsTool_Execute_MissingParams(t *testing.T) {
	logger := logging.GetLogger("test.narrow")
	ctx := context.Background()

	mockGraph := newMockNarrowGraphClient()
	investigateService := NewObservatoryInvestigateService(mockGraph, nil, "test-grafana", logger)
	tool := NewObservatorySignalsTool(investigateService, logger)

	testCases := []struct {
		name   string
		params ObservatorySignalsParams
		errMsg string
	}{
		{
			name:   "missing namespace",
			params: ObservatorySignalsParams{Workload: "api-server"},
			errMsg: "namespace is required",
		},
		{
			name:   "missing workload",
			params: ObservatorySignalsParams{Namespace: "prod"},
			errMsg: "workload is required",
		},
		{
			name:   "both missing",
			params: ObservatorySignalsParams{},
			errMsg: "namespace is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args, _ := json.Marshal(tc.params)
			_, err := tool.Execute(ctx, args)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}
}

// --- Helper tests ---

func TestObservatoryScopeTool_Timestamp_RFC3339(t *testing.T) {
	logger := logging.GetLogger("test.narrow")
	ctx := context.Background()

	mockGraph := newMockNarrowGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		return &graph.QueryResult{
			Columns: []string{"workload_name"},
			Rows:    [][]interface{}{},
		}, nil
	}

	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	service := NewObservatoryService(mockGraph, anomalyAgg, "test-grafana", logger)
	tool := NewObservatoryScopeTool(service, logger)

	args, _ := json.Marshal(ObservatoryScopeParams{Namespace: "test"})
	result, err := tool.Execute(ctx, args)

	require.NoError(t, err)
	resp, ok := result.(*ObservatoryScopeResponse)
	require.True(t, ok)

	// Verify timestamp is valid RFC3339
	_, err = time.Parse(time.RFC3339, resp.Timestamp)
	assert.NoError(t, err, "Timestamp should be valid RFC3339")
}
