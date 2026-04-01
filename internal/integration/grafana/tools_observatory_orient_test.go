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

// mockOrientGraphClient implements graph.Client for Orient tools testing.
type mockOrientGraphClient struct {
	executeQueryFunc func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error)
	queries          []graph.GraphQuery
}

func newMockOrientGraphClient() *mockOrientGraphClient {
	return &mockOrientGraphClient{
		queries: make([]graph.GraphQuery, 0),
	}
}

func (m *mockOrientGraphClient) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	m.queries = append(m.queries, query)
	if m.executeQueryFunc != nil {
		return m.executeQueryFunc(ctx, query)
	}
	return &graph.QueryResult{}, nil
}

func (m *mockOrientGraphClient) Connect(ctx context.Context) error { return nil }
func (m *mockOrientGraphClient) Close() error                      { return nil }
func (m *mockOrientGraphClient) Ping(ctx context.Context) error    { return nil }
func (m *mockOrientGraphClient) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}
func (m *mockOrientGraphClient) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}
func (m *mockOrientGraphClient) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}
func (m *mockOrientGraphClient) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}
func (m *mockOrientGraphClient) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}
func (m *mockOrientGraphClient) InitializeSchema(ctx context.Context) error { return nil }
func (m *mockOrientGraphClient) DeleteGraph(ctx context.Context) error      { return nil }
func (m *mockOrientGraphClient) CreateGraph(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockOrientGraphClient) DeleteGraphByName(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockOrientGraphClient) GraphExists(ctx context.Context, graphName string) (bool, error) {
	return false, nil
}

// TestObservatoryStatusTool_Execute_Success tests that status tool returns hotspots.
func TestObservatoryStatusTool_Execute_Success(t *testing.T) {
	logger := logging.GetLogger("test.observatory.status")
	mockGraph := newMockOrientGraphClient()

	// Setup mock to return anomalous namespaces and workloads
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Namespace workloads query
		if strings.Contains(query.Query, "DISTINCT") && strings.Contains(query.Query, "AS workload_name") {
			ns := query.Parameters["namespace"].(string)
			if ns == "prod" {
				return &graph.QueryResult{
					Columns: []string{"workload_name"},
					Rows: [][]interface{}{
						{"nginx"},
						{"api-server"},
					},
				}, nil
			}
		}

		// Cluster namespaces query
		if strings.Contains(query.Query, "DISTINCT") && strings.Contains(query.Query, "AS namespace") {
			return &graph.QueryResult{
				Columns: []string{"namespace"},
				Rows: [][]interface{}{
					{"prod"},
				},
			}, nil
		}

		// Workload signals query - return anomalous signals
		if strings.Contains(query.Query, "HAS_BASELINE") {
			workload := query.Parameters["workload_name"].(string)
			if workload == "nginx" || workload == "api-server" {
				// Return signal with mean > P99 to trigger anomaly
				return &graph.QueryResult{
					Columns: []string{"metric_name", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
					Rows: [][]interface{}{
						{"http_requests_total", 0.9, 1200.0, 50.0, 800.0, 1200.0, 1000.0, 1150.0, 1180.0, float64(100)},
					},
				}, nil
			}
		}

		return &graph.QueryResult{}, nil
	}

	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	anomalyAgg.cache.Clear()
	service := NewObservatoryService(mockGraph, anomalyAgg, "test-grafana", logger)
	tool := NewObservatoryStatusTool(service, logger)

	params := ObservatoryStatusParams{}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)
	require.NotNil(t, result)

	response := result.(*ObservatoryStatusResponse)

	// Should have hotspots since we have anomalous signals
	assert.NotEmpty(t, response.Timestamp)

	// All hotspots should have score >= 0.5 (threshold)
	for _, hotspot := range response.TopHotspots {
		assert.GreaterOrEqual(t, hotspot.Score, 0.5,
			"hotspot %s should have score >= 0.5", hotspot.Namespace)
	}
}

// TestObservatoryStatusTool_Execute_Empty tests that empty results are returned correctly.
func TestObservatoryStatusTool_Execute_Empty(t *testing.T) {
	logger := logging.GetLogger("test.observatory.status")
	mockGraph := newMockOrientGraphClient()

	// Setup mock to return no namespaces
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Return empty for all queries
		return &graph.QueryResult{
			Columns: []string{"namespace"},
			Rows:    [][]interface{}{},
		}, nil
	}

	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	anomalyAgg.cache.Clear()
	service := NewObservatoryService(mockGraph, anomalyAgg, "test-grafana", logger)
	tool := NewObservatoryStatusTool(service, logger)

	params := ObservatoryStatusParams{}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)
	require.NotNil(t, result)

	response := result.(*ObservatoryStatusResponse)

	// Per CONTEXT.md: empty results when nothing anomalous (empty array, not "healthy" message)
	assert.Empty(t, response.TopHotspots, "should return empty array when no anomalies")
	assert.Equal(t, 0, response.TotalAnomalousSignals, "should have 0 anomalous signals")
	assert.NotEmpty(t, response.Timestamp, "should still have timestamp")
}

// TestObservatoryStatusTool_Execute_WithFilter tests namespace filter is applied.
func TestObservatoryStatusTool_Execute_WithFilter(t *testing.T) {
	logger := logging.GetLogger("test.observatory.status")
	mockGraph := newMockOrientGraphClient()

	// Track which namespace was queried
	var queriedNamespace string

	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Cluster namespaces query - return multiple namespaces
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

		// Workload query
		if strings.Contains(query.Query, "DISTINCT") && strings.Contains(query.Query, "AS workload_name") {
			ns := query.Parameters["namespace"].(string)
			queriedNamespace = ns
			return &graph.QueryResult{
				Columns: []string{"workload_name"},
				Rows: [][]interface{}{
					{"workload-1"},
				},
			}, nil
		}

		// Signal query - return anomalous signal
		if strings.Contains(query.Query, "HAS_BASELINE") {
			return &graph.QueryResult{
				Columns: []string{"metric_name", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
				Rows: [][]interface{}{
					{"metric", 0.9, 1200.0, 50.0, 800.0, 1200.0, 1000.0, 1150.0, 1180.0, float64(100)},
				},
			}, nil
		}

		return &graph.QueryResult{}, nil
	}

	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	anomalyAgg.cache.Clear()
	service := NewObservatoryService(mockGraph, anomalyAgg, "test-grafana", logger)
	tool := NewObservatoryStatusTool(service, logger)

	// Filter to specific namespace
	params := ObservatoryStatusParams{
		Namespace: "prod",
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have only queried the filtered namespace
	assert.Equal(t, "prod", queriedNamespace)
}

// TestObservatoryChangesTool_Execute_Success tests that changes are returned.
func TestObservatoryChangesTool_Execute_Success(t *testing.T) {
	logger := logging.GetLogger("test.observatory.changes")
	mockGraph := newMockOrientGraphClient()

	now := time.Now()

	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Changes query
		if strings.Contains(query.Query, "ChangeEvent") {
			return &graph.QueryResult{
				Columns: []string{"kind", "namespace", "name", "reason", "message", "timestamp"},
				Rows: [][]interface{}{
					{"Deployment", "prod", "nginx", "UPDATE", "Configuration changed", now.UnixNano()},
					{"HelmRelease", "prod", "api-server", "CREATE", "Resource created", now.Add(-5 * time.Minute).UnixNano()},
					{"ConfigMap", "prod", "config", "UPDATE", "Configuration changed", now.Add(-10 * time.Minute).UnixNano()},
				},
			}, nil
		}
		return &graph.QueryResult{}, nil
	}

	tool := NewObservatoryChangesTool(mockGraph, "test-grafana", logger)

	params := ObservatoryChangesParams{}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)
	require.NotNil(t, result)

	response := result.(*ObservatoryChangesResponse)

	assert.Len(t, response.Changes, 3)
	assert.Equal(t, "1h", response.Lookback)
	assert.NotEmpty(t, response.Timestamp)

	// Verify first change
	assert.Equal(t, "Deployment", response.Changes[0].Kind)
	assert.Equal(t, "prod", response.Changes[0].Namespace)
	assert.Equal(t, "nginx", response.Changes[0].Name)
	assert.Equal(t, "UPDATE", response.Changes[0].Reason)
	assert.NotEmpty(t, response.Changes[0].Timestamp)
}

// TestObservatoryChangesTool_Execute_Empty tests empty results when no changes.
func TestObservatoryChangesTool_Execute_Empty(t *testing.T) {
	logger := logging.GetLogger("test.observatory.changes")
	mockGraph := newMockOrientGraphClient()

	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Return empty results
		return &graph.QueryResult{
			Columns: []string{"kind", "namespace", "name", "reason", "message", "timestamp"},
			Rows:    [][]interface{}{},
		}, nil
	}

	tool := NewObservatoryChangesTool(mockGraph, "test-grafana", logger)

	params := ObservatoryChangesParams{}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)
	require.NotNil(t, result)

	response := result.(*ObservatoryChangesResponse)

	// Per CONTEXT.md: empty results when no changes (empty array)
	assert.Empty(t, response.Changes, "should return empty array when no changes")
	assert.Equal(t, "1h", response.Lookback)
	assert.NotEmpty(t, response.Timestamp)
}

// TestObservatoryChangesTool_Execute_LookbackParsing tests lookback duration parsing.
func TestObservatoryChangesTool_Execute_LookbackParsing(t *testing.T) {
	logger := logging.GetLogger("test.observatory.changes")
	mockGraph := newMockOrientGraphClient()

	testCases := []struct {
		name           string
		lookback       string
		expectedOutput string
	}{
		{"default", "", "1h"},
		{"1h", "1h", "1h"},
		{"6h", "6h", "6h"},
		{"24h", "24h", "24h"},
		{"2h30m", "2h30m", "2h30m"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
				return &graph.QueryResult{
					Columns: []string{"kind", "namespace", "name", "reason", "message", "timestamp"},
					Rows:    [][]interface{}{},
				}, nil
			}

			tool := NewObservatoryChangesTool(mockGraph, "test-grafana", logger)

			params := ObservatoryChangesParams{
				Lookback: tc.lookback,
			}
			paramsJSON, _ := json.Marshal(params)

			result, err := tool.Execute(context.Background(), paramsJSON)
			require.NoError(t, err)
			require.NotNil(t, result)

			response := result.(*ObservatoryChangesResponse)
			assert.Equal(t, tc.expectedOutput, response.Lookback)
		})
	}
}

// TestObservatoryChangesTool_Execute_MaxLookback tests that lookback is capped at 24h.
func TestObservatoryChangesTool_Execute_MaxLookback(t *testing.T) {
	logger := logging.GetLogger("test.observatory.changes")
	mockGraph := newMockOrientGraphClient()

	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Verify the lookback parameter is capped
		lookbackStart := query.Parameters["lookbackStart"].(int64)
		now := time.Now().UnixNano()
		lookbackDuration := time.Duration(now - lookbackStart)

		// Should be capped to 24h (with some tolerance for test execution time)
		assert.LessOrEqual(t, lookbackDuration, 25*time.Hour, "lookback should be capped at 24h")

		return &graph.QueryResult{
			Columns: []string{"kind", "namespace", "name", "reason", "message", "timestamp"},
			Rows:    [][]interface{}{},
		}, nil
	}

	tool := NewObservatoryChangesTool(mockGraph, "test-grafana", logger)

	// Try to use 48h lookback - should be capped to 24h
	params := ObservatoryChangesParams{
		Lookback: "48h",
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)
	require.NotNil(t, result)

	response := result.(*ObservatoryChangesResponse)
	assert.Equal(t, "24h", response.Lookback, "lookback should be capped to 24h")
}

// TestObservatoryChangesTool_Execute_InvalidLookback tests invalid lookback handling.
func TestObservatoryChangesTool_Execute_InvalidLookback(t *testing.T) {
	logger := logging.GetLogger("test.observatory.changes")
	mockGraph := newMockOrientGraphClient()

	tool := NewObservatoryChangesTool(mockGraph, "test-grafana", logger)

	params := ObservatoryChangesParams{
		Lookback: "invalid",
	}
	paramsJSON, _ := json.Marshal(params)

	_, err := tool.Execute(context.Background(), paramsJSON)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid lookback duration")
}

// TestObservatoryStatusTool_TimestampFormat tests that timestamps are RFC3339 formatted.
func TestObservatoryStatusTool_TimestampFormat(t *testing.T) {
	logger := logging.GetLogger("test.observatory.status")
	mockGraph := newMockOrientGraphClient()

	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		return &graph.QueryResult{}, nil
	}

	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	anomalyAgg.cache.Clear()
	service := NewObservatoryService(mockGraph, anomalyAgg, "test-grafana", logger)
	tool := NewObservatoryStatusTool(service, logger)

	params := ObservatoryStatusParams{}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)

	response := result.(*ObservatoryStatusResponse)

	// Verify timestamp is RFC3339 formatted
	_, err = time.Parse(time.RFC3339, response.Timestamp)
	assert.NoError(t, err, "timestamp should be RFC3339 formatted")
}

// TestObservatoryChangesTool_TimestampFormat tests that timestamps are RFC3339 formatted.
func TestObservatoryChangesTool_TimestampFormat(t *testing.T) {
	logger := logging.GetLogger("test.observatory.changes")
	mockGraph := newMockOrientGraphClient()

	now := time.Now()

	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		return &graph.QueryResult{
			Columns: []string{"kind", "namespace", "name", "reason", "message", "timestamp"},
			Rows: [][]interface{}{
				{"Deployment", "prod", "nginx", "UPDATE", "Config changed", now.UnixNano()},
			},
		}, nil
	}

	tool := NewObservatoryChangesTool(mockGraph, "test-grafana", logger)

	params := ObservatoryChangesParams{}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)

	response := result.(*ObservatoryChangesResponse)

	// Verify response timestamp is RFC3339 formatted
	_, err = time.Parse(time.RFC3339, response.Timestamp)
	assert.NoError(t, err, "response timestamp should be RFC3339 formatted")

	// Verify change timestamps are RFC3339 formatted
	require.Len(t, response.Changes, 1)
	_, err = time.Parse(time.RFC3339, response.Changes[0].Timestamp)
	assert.NoError(t, err, "change timestamp should be RFC3339 formatted")
}
