package grafana

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockVerifyGraphClient implements graph.Client for verify stage tool tests.
type mockVerifyGraphClient struct {
	executeQueryFunc func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error)
	queries          []graph.GraphQuery
}

func newMockVerifyGraphClient() *mockVerifyGraphClient {
	return &mockVerifyGraphClient{
		queries: make([]graph.GraphQuery, 0),
	}
}

func (m *mockVerifyGraphClient) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	m.queries = append(m.queries, query)
	if m.executeQueryFunc != nil {
		return m.executeQueryFunc(ctx, query)
	}
	return &graph.QueryResult{}, nil
}

// Implement remaining graph.Client interface methods
func (m *mockVerifyGraphClient) Connect(ctx context.Context) error { return nil }
func (m *mockVerifyGraphClient) Close() error                      { return nil }
func (m *mockVerifyGraphClient) Ping(ctx context.Context) error    { return nil }
func (m *mockVerifyGraphClient) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}
func (m *mockVerifyGraphClient) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}
func (m *mockVerifyGraphClient) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}
func (m *mockVerifyGraphClient) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}
func (m *mockVerifyGraphClient) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}
func (m *mockVerifyGraphClient) InitializeSchema(ctx context.Context) error { return nil }
func (m *mockVerifyGraphClient) DeleteGraph(ctx context.Context) error      { return nil }
func (m *mockVerifyGraphClient) CreateGraph(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockVerifyGraphClient) DeleteGraphByName(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockVerifyGraphClient) GraphExists(ctx context.Context, graphName string) (bool, error) {
	return false, nil
}

// =============================================================================
// ObservatoryExplainTool Tests
// =============================================================================

// TestObservatoryExplainTool_Execute_Success tests returning upstream deps and recent changes.
func TestObservatoryExplainTool_Execute_Success(t *testing.T) {
	logger := logging.GetLogger("test.explain")
	mockGraph := newMockVerifyGraphClient()

	// Mock returns upstream dependencies and recent changes
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		if query.Parameters["workload"] != nil {
			// Upstream dependencies query
			return &graph.QueryResult{
				Columns: []string{"hops1", "hops2"},
				Rows: [][]interface{}{
					{
						[]interface{}{
							map[string]interface{}{
								"kind":      "Service",
								"namespace": "production",
								"name":      "api-service",
								"hops":      int64(1),
							},
						},
						[]interface{}{
							map[string]interface{}{
								"kind":      "Ingress",
								"namespace": "production",
								"name":      "api-ingress",
								"hops":      int64(2),
							},
						},
					},
				},
			}, nil
		}
		// Recent changes query
		return &graph.QueryResult{
			Columns: []string{"kind", "namespace", "name", "reason", "timestamp"},
			Rows: [][]interface{}{
				{"Deployment", "production", "api-server", "DeploymentUpdated", "2026-01-30T00:10:00Z"},
			},
		}, nil
	}

	evidenceService := NewObservatoryEvidenceService(mockGraph, nil, "test-grafana", logger)
	tool := NewObservatoryExplainTool(evidenceService, logger)

	params := ObservatoryExplainParams{
		Namespace:  "production",
		Workload:   "api-server",
		MetricName: "container_cpu_usage",
	}
	argsJSON, _ := json.Marshal(params)

	ctx := context.Background()
	result, err := tool.Execute(ctx, argsJSON)

	require.NoError(t, err)
	require.NotNil(t, result)

	resp, ok := result.(*ObservatoryExplainResponse)
	require.True(t, ok)

	// Verify upstream dependencies
	assert.Len(t, resp.UpstreamDeps, 2)

	// Check 1-hop dependency
	found1Hop := false
	for _, dep := range resp.UpstreamDeps {
		if dep.HopsAway == 1 {
			assert.Equal(t, "Service", dep.Kind)
			assert.Equal(t, "api-service", dep.Name)
			found1Hop = true
		}
	}
	assert.True(t, found1Hop, "should have 1-hop dependency")

	// Check 2-hop dependency
	found2Hop := false
	for _, dep := range resp.UpstreamDeps {
		if dep.HopsAway == 2 {
			assert.Equal(t, "Ingress", dep.Kind)
			assert.Equal(t, "api-ingress", dep.Name)
			found2Hop = true
		}
	}
	assert.True(t, found2Hop, "should have 2-hop dependency")

	// Verify recent changes
	assert.Len(t, resp.RecentChanges, 1)
	assert.Equal(t, "Deployment", resp.RecentChanges[0].Kind)
	assert.Equal(t, "api-server", resp.RecentChanges[0].Name)

	// Timestamp should be set
	assert.NotEmpty(t, resp.Timestamp)
}

// TestObservatoryExplainTool_Execute_NoUpstream tests returning empty upstream_deps array.
func TestObservatoryExplainTool_Execute_NoUpstream(t *testing.T) {
	logger := logging.GetLogger("test.explain")
	mockGraph := newMockVerifyGraphClient()

	// Mock returns empty upstream but has recent changes
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		if query.Parameters["workload"] != nil {
			// Upstream dependencies query - empty
			return &graph.QueryResult{
				Columns: []string{"hops1", "hops2"},
				Rows: [][]interface{}{
					{[]interface{}{}, []interface{}{}},
				},
			}, nil
		}
		// Recent changes query
		return &graph.QueryResult{
			Columns: []string{"kind", "namespace", "name", "reason", "timestamp"},
			Rows: [][]interface{}{
				{"ConfigMap", "production", "app-config", "ConfigChanged", "2026-01-30T00:05:00Z"},
			},
		}, nil
	}

	evidenceService := NewObservatoryEvidenceService(mockGraph, nil, "test-grafana", logger)
	tool := NewObservatoryExplainTool(evidenceService, logger)

	params := ObservatoryExplainParams{
		Namespace:  "production",
		Workload:   "standalone-app",
		MetricName: "container_memory_usage",
	}
	argsJSON, _ := json.Marshal(params)

	ctx := context.Background()
	result, err := tool.Execute(ctx, argsJSON)

	require.NoError(t, err)
	require.NotNil(t, result)

	resp, ok := result.(*ObservatoryExplainResponse)
	require.True(t, ok)

	// Should have empty upstream deps (not nil)
	assert.Empty(t, resp.UpstreamDeps)

	// Should still have recent changes
	assert.Len(t, resp.RecentChanges, 1)
}

// TestObservatoryExplainTool_Execute_NoChanges tests returning empty recent_changes array.
func TestObservatoryExplainTool_Execute_NoChanges(t *testing.T) {
	logger := logging.GetLogger("test.explain")
	mockGraph := newMockVerifyGraphClient()

	// Mock returns upstream deps but no recent changes
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		if query.Parameters["workload"] != nil {
			// Upstream dependencies query
			return &graph.QueryResult{
				Columns: []string{"hops1", "hops2"},
				Rows: [][]interface{}{
					{
						[]interface{}{
							map[string]interface{}{
								"kind":      "Service",
								"namespace": "production",
								"name":      "db-service",
								"hops":      int64(1),
							},
						},
						[]interface{}{},
					},
				},
			}, nil
		}
		// Recent changes query - empty
		return &graph.QueryResult{
			Columns: []string{"kind", "namespace", "name", "reason", "timestamp"},
			Rows:    [][]interface{}{},
		}, nil
	}

	evidenceService := NewObservatoryEvidenceService(mockGraph, nil, "test-grafana", logger)
	tool := NewObservatoryExplainTool(evidenceService, logger)

	params := ObservatoryExplainParams{
		Namespace:  "production",
		Workload:   "stable-app",
		MetricName: "request_latency",
	}
	argsJSON, _ := json.Marshal(params)

	ctx := context.Background()
	result, err := tool.Execute(ctx, argsJSON)

	require.NoError(t, err)
	require.NotNil(t, result)

	resp, ok := result.(*ObservatoryExplainResponse)
	require.True(t, ok)

	// Should have upstream deps
	assert.Len(t, resp.UpstreamDeps, 1)
	assert.Equal(t, "Service", resp.UpstreamDeps[0].Kind)

	// Should have empty recent changes (not nil)
	assert.Empty(t, resp.RecentChanges)
}

// TestObservatoryExplainTool_Execute_MissingParams tests error on missing required parameters.
func TestObservatoryExplainTool_Execute_MissingParams(t *testing.T) {
	logger := logging.GetLogger("test.explain")
	mockGraph := newMockVerifyGraphClient()
	evidenceService := NewObservatoryEvidenceService(mockGraph, nil, "test-grafana", logger)
	tool := NewObservatoryExplainTool(evidenceService, logger)

	ctx := context.Background()

	// Test missing namespace
	params := ObservatoryExplainParams{
		Workload:   "api-server",
		MetricName: "cpu_usage",
	}
	argsJSON, _ := json.Marshal(params)
	_, err := tool.Execute(ctx, argsJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "namespace is required")

	// Test missing workload
	params = ObservatoryExplainParams{
		Namespace:  "production",
		MetricName: "cpu_usage",
	}
	argsJSON, _ = json.Marshal(params)
	_, err = tool.Execute(ctx, argsJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workload is required")

	// Test missing metric_name
	params = ObservatoryExplainParams{
		Namespace: "production",
		Workload:  "api-server",
	}
	argsJSON, _ = json.Marshal(params)
	_, err = tool.Execute(ctx, argsJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "metric_name is required")
}

// =============================================================================
// ObservatoryEvidenceTool Tests
// =============================================================================

// TestObservatoryEvidenceTool_Execute_Success tests returning metric values and alert states.
func TestObservatoryEvidenceTool_Execute_Success(t *testing.T) {
	logger := logging.GetLogger("test.evidence")
	mockGraph := newMockVerifyGraphClient()

	// Mock returns metric values and alert states
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		if query.Parameters["metric_name"] != nil {
			// Metric values query (SignalBaseline)
			return &graph.QueryResult{
				Columns: []string{"mean", "std_dev", "min", "max", "p50", "p90", "p99", "window_start", "window_end"},
				Rows: [][]interface{}{
					{85.5, 8.0, 70.0, 100.0, 85.0, 95.0, 98.0, int64(1706572800), int64(1706659200)},
				},
			}, nil
		}
		if query.Parameters["start"] != nil && query.Parameters["end"] != nil {
			// Alert states query
			return &graph.QueryResult{
				Columns: []string{"title", "state", "since"},
				Rows: [][]interface{}{
					{"High CPU Alert", "firing", "2026-01-30T00:15:00Z"},
					{"Memory Warning", "pending", "2026-01-30T00:18:00Z"},
				},
			}, nil
		}
		if query.Parameters["since"] != nil {
			// Log excerpts query - return empty
			return &graph.QueryResult{
				Columns: []string{"timestamp", "level", "message", "source"},
				Rows:    [][]interface{}{},
			}, nil
		}
		return &graph.QueryResult{}, nil
	}

	evidenceService := NewObservatoryEvidenceService(mockGraph, nil, "test-grafana", logger)
	tool := NewObservatoryEvidenceTool(evidenceService, logger)

	params := ObservatoryEvidenceParams{
		Namespace:  "production",
		Workload:   "api-server",
		MetricName: "container_cpu_usage",
	}
	argsJSON, _ := json.Marshal(params)

	ctx := context.Background()
	result, err := tool.Execute(ctx, argsJSON)

	require.NoError(t, err)
	require.NotNil(t, result)

	resp, ok := result.(*ObservatoryEvidenceResponse)
	require.True(t, ok)

	// Verify metric values
	assert.Len(t, resp.MetricValues, 1)
	assert.Equal(t, 85.5, resp.MetricValues[0].Value)

	// Verify alert states
	assert.Len(t, resp.AlertStates, 2)

	// Check firing alert
	foundFiring := false
	for _, alert := range resp.AlertStates {
		if alert.State == "firing" {
			assert.Equal(t, "High CPU Alert", alert.AlertName)
			foundFiring = true
		}
	}
	assert.True(t, foundFiring, "should have firing alert")

	// Default lookback should be used
	assert.Equal(t, "1h", resp.Lookback)

	// Timestamp should be set
	assert.NotEmpty(t, resp.Timestamp)
}

// TestObservatoryEvidenceTool_Execute_WithLogs tests returning log excerpts when available.
func TestObservatoryEvidenceTool_Execute_WithLogs(t *testing.T) {
	logger := logging.GetLogger("test.evidence")
	mockGraph := newMockVerifyGraphClient()

	// Mock returns metric, alerts, and log excerpts
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		if query.Parameters["metric_name"] != nil {
			// Metric values query
			return &graph.QueryResult{
				Columns: []string{"mean", "std_dev", "min", "max", "p50", "p90", "p99", "window_start", "window_end"},
				Rows: [][]interface{}{
					{50.0, 5.0, 40.0, 60.0, 50.0, 55.0, 58.0, int64(1706572800), int64(1706659200)},
				},
			}, nil
		}
		if query.Parameters["start"] != nil && query.Parameters["end"] != nil {
			// Alert states query - empty
			return &graph.QueryResult{
				Columns: []string{"title", "state", "since"},
				Rows:    [][]interface{}{},
			}, nil
		}
		if query.Parameters["since"] != nil {
			// Log excerpts query - return logs
			return &graph.QueryResult{
				Columns: []string{"timestamp", "level", "message", "source"},
				Rows: [][]interface{}{
					{"2026-01-30T00:20:00Z", "ERROR", "Connection timeout to database", "api-server-pod-1"},
					{"2026-01-30T00:20:05Z", "ERROR", "Retry failed after 3 attempts", "api-server-pod-1"},
				},
			}, nil
		}
		return &graph.QueryResult{}, nil
	}

	evidenceService := NewObservatoryEvidenceService(mockGraph, nil, "test-grafana", logger)
	tool := NewObservatoryEvidenceTool(evidenceService, logger)

	params := ObservatoryEvidenceParams{
		Namespace:  "production",
		Workload:   "api-server",
		MetricName: "error_rate",
		Lookback:   "30m",
	}
	argsJSON, _ := json.Marshal(params)

	ctx := context.Background()
	result, err := tool.Execute(ctx, argsJSON)

	require.NoError(t, err)
	require.NotNil(t, result)

	resp, ok := result.(*ObservatoryEvidenceResponse)
	require.True(t, ok)

	// Verify log excerpts are present
	assert.Len(t, resp.LogExcerpts, 2)
	assert.Equal(t, "ERROR", resp.LogExcerpts[0].Level)
	assert.Contains(t, resp.LogExcerpts[0].Message, "Connection timeout")
	assert.Equal(t, "api-server-pod-1", resp.LogExcerpts[0].Source)

	// Custom lookback should be used
	assert.Equal(t, "30m", resp.Lookback)
}

// TestObservatoryEvidenceTool_Execute_NoLogs tests graceful handling when logs unavailable.
func TestObservatoryEvidenceTool_Execute_NoLogs(t *testing.T) {
	logger := logging.GetLogger("test.evidence")
	mockGraph := newMockVerifyGraphClient()

	// Mock returns metric and alerts but no logs
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		if query.Parameters["metric_name"] != nil {
			// Metric values query
			return &graph.QueryResult{
				Columns: []string{"mean", "std_dev", "min", "max", "p50", "p90", "p99", "window_start", "window_end"},
				Rows: [][]interface{}{
					{75.0, 7.5, 60.0, 90.0, 75.0, 85.0, 88.0, int64(1706572800), int64(1706659200)},
				},
			}, nil
		}
		if query.Parameters["start"] != nil && query.Parameters["end"] != nil {
			// Alert states query
			return &graph.QueryResult{
				Columns: []string{"title", "state", "since"},
				Rows: [][]interface{}{
					{"Latency Alert", "normal", "2026-01-29T23:00:00Z"},
				},
			}, nil
		}
		if query.Parameters["since"] != nil {
			// Log excerpts query - return empty (log integration not configured)
			return &graph.QueryResult{
				Columns: []string{"timestamp", "level", "message", "source"},
				Rows:    [][]interface{}{},
			}, nil
		}
		return &graph.QueryResult{}, nil
	}

	evidenceService := NewObservatoryEvidenceService(mockGraph, nil, "test-grafana", logger)
	tool := NewObservatoryEvidenceTool(evidenceService, logger)

	params := ObservatoryEvidenceParams{
		Namespace:  "production",
		Workload:   "api-server",
		MetricName: "request_latency",
	}
	argsJSON, _ := json.Marshal(params)

	ctx := context.Background()
	result, err := tool.Execute(ctx, argsJSON)

	// Should succeed despite no logs
	require.NoError(t, err)
	require.NotNil(t, result)

	resp, ok := result.(*ObservatoryEvidenceResponse)
	require.True(t, ok)

	// Metric values should work
	assert.Len(t, resp.MetricValues, 1)

	// Alert states should work
	assert.Len(t, resp.AlertStates, 1)

	// Log excerpts should be empty (graceful degradation)
	assert.Empty(t, resp.LogExcerpts)
}

// TestObservatoryEvidenceTool_Execute_DefaultLookback tests using 1h when not specified.
func TestObservatoryEvidenceTool_Execute_DefaultLookback(t *testing.T) {
	logger := logging.GetLogger("test.evidence")
	mockGraph := newMockVerifyGraphClient()

	// Mock returns basic data
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		if query.Parameters["metric_name"] != nil {
			return &graph.QueryResult{
				Columns: []string{"mean", "std_dev", "min", "max", "p50", "p90", "p99", "window_start", "window_end"},
				Rows: [][]interface{}{
					{100.0, 10.0, 80.0, 120.0, 100.0, 115.0, 118.0, int64(1706572800), int64(1706659200)},
				},
			}, nil
		}
		if query.Parameters["start"] != nil && query.Parameters["end"] != nil {
			return &graph.QueryResult{
				Columns: []string{"title", "state", "since"},
				Rows:    [][]interface{}{},
			}, nil
		}
		if query.Parameters["since"] != nil {
			return &graph.QueryResult{
				Columns: []string{"timestamp", "level", "message", "source"},
				Rows:    [][]interface{}{},
			}, nil
		}
		return &graph.QueryResult{}, nil
	}

	evidenceService := NewObservatoryEvidenceService(mockGraph, nil, "test-grafana", logger)
	tool := NewObservatoryEvidenceTool(evidenceService, logger)

	// No lookback specified
	params := ObservatoryEvidenceParams{
		Namespace:  "production",
		Workload:   "api-server",
		MetricName: "container_cpu_usage",
	}
	argsJSON, _ := json.Marshal(params)

	ctx := context.Background()
	result, err := tool.Execute(ctx, argsJSON)

	require.NoError(t, err)
	require.NotNil(t, result)

	resp, ok := result.(*ObservatoryEvidenceResponse)
	require.True(t, ok)

	// Should use default 1h lookback
	assert.Equal(t, "1h", resp.Lookback)
}

// TestObservatoryEvidenceTool_Execute_MissingParams tests error on missing required parameters.
func TestObservatoryEvidenceTool_Execute_MissingParams(t *testing.T) {
	logger := logging.GetLogger("test.evidence")
	mockGraph := newMockVerifyGraphClient()
	evidenceService := NewObservatoryEvidenceService(mockGraph, nil, "test-grafana", logger)
	tool := NewObservatoryEvidenceTool(evidenceService, logger)

	ctx := context.Background()

	// Test missing namespace
	params := ObservatoryEvidenceParams{
		Workload:   "api-server",
		MetricName: "cpu_usage",
	}
	argsJSON, _ := json.Marshal(params)
	_, err := tool.Execute(ctx, argsJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "namespace is required")

	// Test missing workload
	params = ObservatoryEvidenceParams{
		Namespace:  "production",
		MetricName: "cpu_usage",
	}
	argsJSON, _ = json.Marshal(params)
	_, err = tool.Execute(ctx, argsJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "workload is required")

	// Test missing metric_name
	params = ObservatoryEvidenceParams{
		Namespace: "production",
		Workload:  "api-server",
	}
	argsJSON, _ = json.Marshal(params)
	_, err = tool.Execute(ctx, argsJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "metric_name is required")

	// Test invalid lookback format
	params = ObservatoryEvidenceParams{
		Namespace:  "production",
		Workload:   "api-server",
		MetricName: "cpu_usage",
		Lookback:   "invalid",
	}
	argsJSON, _ = json.Marshal(params)
	_, err = tool.Execute(ctx, argsJSON)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid lookback format")
}
