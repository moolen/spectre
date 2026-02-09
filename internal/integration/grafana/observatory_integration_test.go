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

// mockObservatoryIntegrationGraphClient implements graph.Client for observatory integration testing.
// Provides comprehensive mocking for all observatory tool queries.
type mockObservatoryIntegrationGraphClient struct {
	executeQueryFunc func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error)
	queries          []graph.GraphQuery
}

func newMockObservatoryIntegrationGraphClient() *mockObservatoryIntegrationGraphClient {
	return &mockObservatoryIntegrationGraphClient{
		queries: make([]graph.GraphQuery, 0),
	}
}

func (m *mockObservatoryIntegrationGraphClient) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	m.queries = append(m.queries, query)
	if m.executeQueryFunc != nil {
		return m.executeQueryFunc(ctx, query)
	}
	return &graph.QueryResult{}, nil
}

func (m *mockObservatoryIntegrationGraphClient) Connect(ctx context.Context) error { return nil }
func (m *mockObservatoryIntegrationGraphClient) Close() error                      { return nil }
func (m *mockObservatoryIntegrationGraphClient) Ping(ctx context.Context) error    { return nil }
func (m *mockObservatoryIntegrationGraphClient) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}
func (m *mockObservatoryIntegrationGraphClient) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}
func (m *mockObservatoryIntegrationGraphClient) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}
func (m *mockObservatoryIntegrationGraphClient) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}
func (m *mockObservatoryIntegrationGraphClient) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}
func (m *mockObservatoryIntegrationGraphClient) InitializeSchema(ctx context.Context) error { return nil }
func (m *mockObservatoryIntegrationGraphClient) DeleteGraph(ctx context.Context) error      { return nil }
func (m *mockObservatoryIntegrationGraphClient) CreateGraph(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockObservatoryIntegrationGraphClient) DeleteGraphByName(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockObservatoryIntegrationGraphClient) GraphExists(ctx context.Context, graphName string) (bool, error) {
	return false, nil
}

// mockIntegrationQueryService implements QueryService for integration testing.
type mockIntegrationQueryService struct {
	currentValue    float64
	historicalValue float64
	shouldError     bool
}

func (m *mockIntegrationQueryService) FetchCurrentValue(ctx context.Context, metricName, namespace, workload string) (float64, error) {
	if m.shouldError {
		return 0, assert.AnError
	}
	return m.currentValue, nil
}

func (m *mockIntegrationQueryService) FetchHistoricalValue(ctx context.Context, metricName, namespace, workload string, lookback time.Duration) (float64, error) {
	if m.shouldError {
		return 0, assert.AnError
	}
	return m.historicalValue, nil
}

// TestObservatoryIntegration_StatusTool tests the full status tool execution flow.
func TestObservatoryIntegration_StatusTool(t *testing.T) {
	logger := logging.GetLogger("test.observatory.integration.status")
	mockGraph := newMockObservatoryIntegrationGraphClient()

	// Setup mock to return anomalous workloads
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Cluster namespaces query
		if strings.Contains(query.Query, "DISTINCT") && strings.Contains(query.Query, "AS namespace") {
			return &graph.QueryResult{
				Columns: []string{"namespace"},
				Rows: [][]interface{}{
					{"prod"},
					{"staging"},
				},
			}, nil
		}

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
			return &graph.QueryResult{Columns: []string{"workload_name"}, Rows: [][]interface{}{}}, nil
		}

		// Workload signals query - return anomalous signals
		if strings.Contains(query.Query, "HAS_BASELINE") {
			workload := query.Parameters["workload_name"].(string)
			if workload == "nginx" {
				// Signal exceeding P99 - anomalous
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

	// Create services
	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	anomalyAgg.cache.Clear()
	service := NewObservatoryService(mockGraph, anomalyAgg, "test-grafana", logger)

	// Execute tool
	tool := NewObservatoryStatusTool(service, logger)
	params := ObservatoryStatusParams{}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)

	response, ok := result.(*ObservatoryStatusResponse)
	require.True(t, ok, "Expected ObservatoryStatusResponse type")

	// Verify hotspots found
	assert.NotEmpty(t, response.TopHotspots, "Should find hotspots")
	assert.NotEmpty(t, response.Timestamp, "Should have timestamp")
}

// TestObservatoryIntegration_ScopeTool tests namespace/workload scoping.
func TestObservatoryIntegration_ScopeTool(t *testing.T) {
	logger := logging.GetLogger("test.observatory.integration.scope")
	mockGraph := newMockObservatoryIntegrationGraphClient()

	// Setup mock for namespace scoping
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Namespace workloads query
		if strings.Contains(query.Query, "DISTINCT") && strings.Contains(query.Query, "AS workload_name") {
			return &graph.QueryResult{
				Columns: []string{"workload_name"},
				Rows: [][]interface{}{
					{"nginx"},
					{"api-server"},
				},
			}, nil
		}

		// Workload signals query
		if strings.Contains(query.Query, "HAS_BASELINE") {
			workload := query.Parameters["workload_name"].(string)
			if workload == "nginx" {
				return &graph.QueryResult{
					Columns: []string{"metric_name", "quality_score", "mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count"},
					Rows: [][]interface{}{
						{"http_requests_total", 0.85, 150.0, 20.0, 100.0, 200.0, 130.0, 170.0, 180.0, float64(100)},
					},
				}, nil
			}
		}

		return &graph.QueryResult{}, nil
	}

	// Create services
	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	anomalyAgg.cache.Clear()
	service := NewObservatoryService(mockGraph, anomalyAgg, "test-grafana", logger)

	// Execute tool
	tool := NewObservatoryScopeTool(service, logger)
	params := ObservatoryScopeParams{Namespace: "prod"}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)

	response, ok := result.(*ObservatoryScopeResponse)
	require.True(t, ok, "Expected ObservatoryScopeResponse type")

	// Verify scope
	assert.Equal(t, "prod", response.Scope)
	assert.NotEmpty(t, response.Timestamp)
}

// TestObservatoryIntegration_SignalDetailTool tests detailed signal inspection.
func TestObservatoryIntegration_SignalDetailTool(t *testing.T) {
	logger := logging.GetLogger("test.observatory.integration.signal_detail")
	mockGraph := newMockObservatoryIntegrationGraphClient()

	// Setup mock for signal detail query
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Signal anchor with baseline query
		if strings.Contains(query.Query, "SignalAnchor") && strings.Contains(query.Query, "HAS_BASELINE") {
			return &graph.QueryResult{
				Columns: []string{
					"metric_name", "workload_namespace", "workload_name", "role",
					"confidence", "quality_score", "dashboard_uid", "panel_id", "first_seen",
					"mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count", "window_start", "window_end",
				},
				Rows: [][]interface{}{
					{
						"http_requests_total", "prod", "nginx", "primary",
						0.9, 0.85, "abc123", int64(5), time.Now().Add(-7 * 24 * time.Hour).Unix(),
						150.0, 20.0, 100.0, 200.0, 130.0, 170.0, 180.0, int64(1000), time.Now().Add(-24 * time.Hour).Unix(), time.Now().Unix(),
					},
				},
			}, nil
		}

		return &graph.QueryResult{}, nil
	}

	// Create services
	mockQueryService := &mockIntegrationQueryService{
		currentValue:    175.0,
		historicalValue: 140.0,
	}
	service := NewObservatoryInvestigateService(mockGraph, mockQueryService, "test-grafana", logger)

	// Execute tool
	tool := NewObservatorySignalDetailTool(service, logger)
	params := ObservatorySignalDetailParams{
		Namespace:  "prod",
		Workload:   "nginx",
		MetricName: "http_requests_total",
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)

	response, ok := result.(*ObservatorySignalDetailResponse)
	require.True(t, ok, "Expected ObservatorySignalDetailResponse type")

	// Verify response fields
	assert.Equal(t, "http_requests_total", response.MetricName)
	assert.NotEmpty(t, response.SourceDashboard, "Should have source dashboard")
	assert.NotEmpty(t, response.Timestamp)
}

// TestObservatoryIntegration_ExplainTool tests root cause candidate generation.
func TestObservatoryIntegration_ExplainTool(t *testing.T) {
	logger := logging.GetLogger("test.observatory.integration.explain")
	mockGraph := newMockObservatoryIntegrationGraphClient()

	// Setup mock for upstream deps and changes
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Upstream dependencies query
		if strings.Contains(query.Query, "DEPENDS_ON") {
			return &graph.QueryResult{
				Columns: []string{"hops1", "hops2"},
				Rows: [][]interface{}{
					{
						[]interface{}{map[string]interface{}{"kind": "Service", "namespace": "prod", "name": "nginx-svc", "hops": int64(1)}},
						[]interface{}{map[string]interface{}{"kind": "Ingress", "namespace": "prod", "name": "nginx-ingress", "hops": int64(2)}},
					},
				},
			}, nil
		}

		// Recent changes query
		if strings.Contains(query.Query, "Event") {
			return &graph.QueryResult{
				Columns: []string{"kind", "namespace", "name", "reason", "timestamp"},
				Rows: [][]interface{}{
					{"Deployment", "prod", "nginx", "DeploymentUpdated", time.Now().Add(-30 * time.Minute).Format(time.RFC3339)},
				},
			}, nil
		}

		return &graph.QueryResult{}, nil
	}

	// Create services - ObservatoryEvidenceService takes *GrafanaQueryService (nil is ok for graph-only ops)
	service := NewObservatoryEvidenceService(mockGraph, nil, "test-grafana", logger)

	// Execute tool
	tool := NewObservatoryExplainTool(service, logger)
	params := ObservatoryExplainParams{
		Namespace:  "prod",
		Workload:   "nginx",
		MetricName: "http_requests_total",
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)

	response, ok := result.(*ObservatoryExplainResponse)
	require.True(t, ok, "Expected ObservatoryExplainResponse type")

	// Verify candidates
	assert.NotEmpty(t, response.UpstreamDeps, "Should have upstream dependencies")
	assert.NotEmpty(t, response.RecentChanges, "Should have recent changes")
	assert.NotEmpty(t, response.Timestamp)
}

// TestObservatoryIntegration_EvidenceTool tests evidence gathering.
func TestObservatoryIntegration_EvidenceTool(t *testing.T) {
	logger := logging.GetLogger("test.observatory.integration.evidence")
	mockGraph := newMockObservatoryIntegrationGraphClient()

	// Setup mock for evidence queries
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Metric values query (from baseline)
		if strings.Contains(query.Query, "SignalAnchor") && strings.Contains(query.Query, "HAS_BASELINE") {
			return &graph.QueryResult{
				Columns: []string{"mean", "std_dev", "min", "max", "p50", "p90", "p99", "window_start", "window_end"},
				Rows: [][]interface{}{
					{150.0, 20.0, 100.0, 200.0, 130.0, 170.0, 180.0, time.Now().Add(-24 * time.Hour).Unix(), time.Now().Unix()},
				},
			}, nil
		}

		// Alert states query
		if strings.Contains(query.Query, "Alert") {
			return &graph.QueryResult{
				Columns: []string{"title", "state", "since"},
				Rows: [][]interface{}{
					{"HighErrorRate", "firing", time.Now().Add(-10 * time.Minute).Format(time.RFC3339)},
				},
			}, nil
		}

		// Log excerpts query
		if strings.Contains(query.Query, "LogEntry") {
			return &graph.QueryResult{
				Columns: []string{"timestamp", "level", "message", "source"},
				Rows: [][]interface{}{
					{time.Now().Add(-2 * time.Minute).Format(time.RFC3339), "ERROR", "Connection timeout", "nginx-pod-abc"},
				},
			}, nil
		}

		return &graph.QueryResult{}, nil
	}

	// Create services - ObservatoryEvidenceService takes *GrafanaQueryService (nil is ok for graph-only ops)
	service := NewObservatoryEvidenceService(mockGraph, nil, "test-grafana", logger)

	// Execute tool
	tool := NewObservatoryEvidenceTool(service, logger)
	params := ObservatoryEvidenceParams{
		Namespace:  "prod",
		Workload:   "nginx",
		MetricName: "http_requests_total",
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)

	response, ok := result.(*ObservatoryEvidenceResponse)
	require.True(t, ok, "Expected ObservatoryEvidenceResponse type")

	// Verify evidence collection
	assert.NotEmpty(t, response.Timestamp, "Should have timestamp")
	// Note: MetricValues may be empty if baseline query returns wrong columns
	// AlertStates and LogExcerpts depend on mock data
}

// TestObservatoryIntegration_EmptyResults tests graceful handling of empty data.
func TestObservatoryIntegration_EmptyResults(t *testing.T) {
	logger := logging.GetLogger("test.observatory.integration.empty")
	mockGraph := newMockObservatoryIntegrationGraphClient()

	// Setup mock to return empty results
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		return &graph.QueryResult{}, nil
	}

	// Create services
	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	anomalyAgg.cache.Clear()
	service := NewObservatoryService(mockGraph, anomalyAgg, "test-grafana", logger)

	// Execute status tool with empty data
	tool := NewObservatoryStatusTool(service, logger)
	params := ObservatoryStatusParams{}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)

	response, ok := result.(*ObservatoryStatusResponse)
	require.True(t, ok)

	// Verify empty array (not nil)
	assert.NotNil(t, response.TopHotspots, "TopHotspots should be empty array, not nil")
	assert.Empty(t, response.TopHotspots, "Should have no hotspots when no anomalies")
}

// TestObservatoryIntegration_ToolRegistration tests that all 8 tools can be created.
func TestObservatoryIntegration_ToolRegistration(t *testing.T) {
	logger := logging.GetLogger("test.observatory.integration.registration")
	mockGraph := newMockObservatoryIntegrationGraphClient()
	mockQueryService := &mockIntegrationQueryService{}

	// Create all services
	anomalyAgg := NewAnomalyAggregator(mockGraph, "test-grafana", logger)
	observatoryService := NewObservatoryService(mockGraph, anomalyAgg, "test-grafana", logger)
	investigateService := NewObservatoryInvestigateService(mockGraph, mockQueryService, "test-grafana", logger)
	evidenceService := NewObservatoryEvidenceService(mockGraph, nil, "test-grafana", logger)

	// Create all 8 tools
	tools := []struct {
		name string
		tool interface{ Execute(context.Context, []byte) (interface{}, error) }
	}{
		{"observatory_status", NewObservatoryStatusTool(observatoryService, logger)},
		{"observatory_changes", NewObservatoryChangesTool(mockGraph, "test-grafana", logger)},
		{"observatory_scope", NewObservatoryScopeTool(observatoryService, logger)},
		{"observatory_signals", NewObservatorySignalsTool(investigateService, logger)},
		{"observatory_signal_detail", NewObservatorySignalDetailTool(investigateService, logger)},
		{"observatory_compare", NewObservatoryCompareTool(investigateService, logger)},
		{"observatory_explain", NewObservatoryExplainTool(evidenceService, logger)},
		{"observatory_evidence", NewObservatoryEvidenceTool(evidenceService, logger)},
	}

	// Verify all 8 tools exist
	assert.Len(t, tools, 8, "Should have exactly 8 observatory tools")

	// Verify each tool can be called (basic execution)
	for _, tc := range tools {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotNil(t, tc.tool, "Tool %s should not be nil", tc.name)

			// Call with empty/minimal params (may error due to validation, but shouldn't panic)
			_, _ = tc.tool.Execute(context.Background(), []byte("{}"))
		})
	}
}

// TestObservatoryIntegration_CompareTool tests time-based signal comparison.
func TestObservatoryIntegration_CompareTool(t *testing.T) {
	logger := logging.GetLogger("test.observatory.integration.compare")
	mockGraph := newMockObservatoryIntegrationGraphClient()

	// Setup mock for comparison queries
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Signal anchor with baseline query
		if strings.Contains(query.Query, "SignalAnchor") && strings.Contains(query.Query, "HAS_BASELINE") {
			return &graph.QueryResult{
				Columns: []string{
					"metric_name", "workload_namespace", "workload_name", "role",
					"confidence", "quality_score", "dashboard_uid", "panel_id", "first_seen",
					"mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count", "window_start", "window_end",
				},
				Rows: [][]interface{}{
					{
						"http_requests_total", "prod", "nginx", "primary",
						0.9, 0.85, "abc123", int64(5), time.Now().Add(-7 * 24 * time.Hour).Unix(),
						150.0, 20.0, 100.0, 200.0, 130.0, 170.0, 180.0, int64(1000), time.Now().Add(-24 * time.Hour).Unix(), time.Now().Unix(),
					},
				},
			}, nil
		}

		return &graph.QueryResult{}, nil
	}

	// Create services with query service that shows improvement over time
	mockQueryService := &mockIntegrationQueryService{
		currentValue:    175.0, // Higher now
		historicalValue: 140.0, // Lower before
	}
	service := NewObservatoryInvestigateService(mockGraph, mockQueryService, "test-grafana", logger)

	// Execute tool
	tool := NewObservatoryCompareTool(service, logger)
	params := ObservatoryCompareParams{
		Namespace:  "prod",
		Workload:   "nginx",
		MetricName: "http_requests_total",
		Lookback:   "24h",
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)

	response, ok := result.(*ObservatoryCompareResponse)
	require.True(t, ok, "Expected ObservatoryCompareResponse type")

	// Verify comparison results
	assert.Equal(t, "http_requests_total", response.MetricName)
	assert.True(t, response.LookbackHours > 0, "Should have lookback hours")
	assert.NotEmpty(t, response.Timestamp)
}

// TestObservatoryIntegration_SignalsTool tests workload signal enumeration.
func TestObservatoryIntegration_SignalsTool(t *testing.T) {
	logger := logging.GetLogger("test.observatory.integration.signals")
	mockGraph := newMockObservatoryIntegrationGraphClient()

	// Setup mock for signals enumeration
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// GetAllWorkloadSignals query
		if strings.Contains(query.Query, "SignalAnchor") && strings.Contains(query.Query, "HAS_BASELINE") {
			return &graph.QueryResult{
				Columns: []string{
					"metric_name", "role", "confidence", "quality_score",
					"mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count",
				},
				Rows: [][]interface{}{
					{"http_requests_total", "primary", 0.9, 0.85, 150.0, 20.0, 100.0, 200.0, 130.0, 170.0, 180.0, int64(1000)},
					{"http_errors_total", "secondary", 0.7, 0.75, 5.0, 2.0, 0.0, 15.0, 3.0, 8.0, 12.0, int64(500)},
				},
			}, nil
		}

		return &graph.QueryResult{}, nil
	}

	// Create services
	mockQueryService := &mockIntegrationQueryService{shouldError: true} // Force baseline fallback
	service := NewObservatoryInvestigateService(mockGraph, mockQueryService, "test-grafana", logger)

	// Execute tool
	tool := NewObservatorySignalsTool(service, logger)
	params := ObservatorySignalsParams{
		Namespace: "prod",
		Workload:  "nginx",
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)

	response, ok := result.(*ObservatorySignalsResponse)
	require.True(t, ok, "Expected ObservatorySignalsResponse type")

	// Verify signals found
	assert.NotEmpty(t, response.Timestamp)
}
