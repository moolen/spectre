package grafana

import (
	"context"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/observatory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// observatoryMockGraphClient is a mock graph client for observatory provider tests
type observatoryMockGraphClient struct {
	queries []graph.GraphQuery
	result  *graph.QueryResult
}

func newObservatoryMockGraphClient() *observatoryMockGraphClient {
	return &observatoryMockGraphClient{
		queries: make([]graph.GraphQuery, 0),
	}
}

func (m *observatoryMockGraphClient) setResult(columns []string, rows [][]any) {
	m.result = &graph.QueryResult{
		Columns: columns,
		Rows:    rows,
	}
}

func (m *observatoryMockGraphClient) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	m.queries = append(m.queries, query)
	if m.result != nil {
		return m.result, nil
	}
	return &graph.QueryResult{}, nil
}

func (m *observatoryMockGraphClient) Connect(ctx context.Context) error  { return nil }
func (m *observatoryMockGraphClient) Close() error                       { return nil }
func (m *observatoryMockGraphClient) Ping(ctx context.Context) error     { return nil }
func (m *observatoryMockGraphClient) InitializeSchema(ctx context.Context) error { return nil }
func (m *observatoryMockGraphClient) DeleteGraph(ctx context.Context) error      { return nil }
func (m *observatoryMockGraphClient) CreateGraph(ctx context.Context, graphName string) error {
	return nil
}
func (m *observatoryMockGraphClient) DeleteGraphByName(ctx context.Context, graphName string) error {
	return nil
}
func (m *observatoryMockGraphClient) GraphExists(ctx context.Context, graphName string) (bool, error) {
	return false, nil
}
func (m *observatoryMockGraphClient) CreateNode(ctx context.Context, nodeType graph.NodeType, properties any) error {
	return nil
}
func (m *observatoryMockGraphClient) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties any) error {
	return nil
}
func (m *observatoryMockGraphClient) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}
func (m *observatoryMockGraphClient) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}
func (m *observatoryMockGraphClient) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}

func (m *observatoryMockGraphClient) lastQuery() *graph.GraphQuery {
	if len(m.queries) == 0 {
		return nil
	}
	return &m.queries[len(m.queries)-1]
}

func TestGrafanaObservatoryProvider_ImplementsInterface(t *testing.T) {
	// Verify at compile time that GrafanaObservatoryProvider implements observatory.Provider
	var _ observatory.Provider = (*GrafanaObservatoryProvider)(nil)
}

func TestGrafanaObservatoryProvider_Name(t *testing.T) {
	logger := logging.GetLogger("test.observatory.provider")
	mockGraph := newObservatoryMockGraphClient()

	provider := NewGrafanaObservatoryProvider(mockGraph, "grafana-prod", logger)

	assert.Equal(t, "grafana-prod", provider.Name())
}

func TestGrafanaObservatoryProvider_ListSignalAnchors(t *testing.T) {
	logger := logging.GetLogger("test.observatory.provider")
	ctx := context.Background()

	mockGraph := newObservatoryMockGraphClient()
	mockGraph.setResult(
		[]string{
			"metric_name", "workload_namespace", "workload_name",
			"role", "confidence", "quality_score",
			"dashboard_uid", "panel_id", "first_seen", "last_seen", "expires_at",
		},
		[][]any{
			{
				"http_requests_total", "prod", "api-server",
				"Traffic", 0.9, 0.85,
				"dashboard-123", 1, int64(1000), int64(2000), int64(time.Now().Unix() + 86400),
			},
			{
				"http_errors_total", "prod", "api-server",
				"Errors", 0.85, 0.8,
				"dashboard-123", 2, int64(1000), int64(2000), int64(time.Now().Unix() + 86400),
			},
		},
	)

	provider := NewGrafanaObservatoryProvider(mockGraph, "grafana-prod", logger)

	signals, err := provider.ListSignalAnchors(ctx, observatory.SignalListOptions{})
	require.NoError(t, err)
	require.Len(t, signals, 2)

	// Verify first signal
	assert.Equal(t, "http_requests_total", signals[0].MetricName)
	assert.Equal(t, "prod", signals[0].WorkloadNamespace)
	assert.Equal(t, "api-server", signals[0].WorkloadName)
	assert.Equal(t, observatory.SignalRole("Traffic"), signals[0].Role)
	assert.Equal(t, 0.9, signals[0].Confidence)
	assert.Equal(t, 0.85, signals[0].QualityScore)
	assert.Equal(t, "grafana-prod", signals[0].SourceProvider)
	assert.Equal(t, "dashboard-123", signals[0].SourceRef)

	// Verify second signal
	assert.Equal(t, "http_errors_total", signals[1].MetricName)
	assert.Equal(t, observatory.SignalRole("Errors"), signals[1].Role)
}

func TestGrafanaObservatoryProvider_ListSignalAnchors_WithFilters(t *testing.T) {
	logger := logging.GetLogger("test.observatory.provider")
	ctx := context.Background()

	mockGraph := newObservatoryMockGraphClient()
	mockGraph.setResult(
		[]string{"metric_name", "workload_namespace", "workload_name", "role", "confidence", "quality_score", "dashboard_uid", "panel_id", "first_seen", "last_seen", "expires_at"},
		[][]any{},
	)

	provider := NewGrafanaObservatoryProvider(mockGraph, "grafana-prod", logger)

	// Call with namespace filter
	_, err := provider.ListSignalAnchors(ctx, observatory.SignalListOptions{
		Namespace:    "prod",
		WorkloadName: "api-server",
	})
	require.NoError(t, err)

	// Verify query parameters were passed
	lastQuery := mockGraph.lastQuery()
	require.NotNil(t, lastQuery)
	assert.Equal(t, "prod", lastQuery.Parameters["namespace"])
	assert.Equal(t, "api-server", lastQuery.Parameters["workload_name"])
}

func TestGrafanaObservatoryProvider_GetCurrentValue(t *testing.T) {
	logger := logging.GetLogger("test.observatory.provider")
	ctx := context.Background()

	mockGraph := newObservatoryMockGraphClient()
	provider := NewGrafanaObservatoryProvider(mockGraph, "grafana-prod", logger)

	// Currently returns not found (baseline mean fallback)
	value, found, err := provider.GetCurrentValue(ctx, "http_requests_total", "prod", "api-server")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Equal(t, 0.0, value)
}

func TestGrafanaObservatoryProvider_GetBaseline(t *testing.T) {
	logger := logging.GetLogger("test.observatory.provider")
	ctx := context.Background()

	mockGraph := newObservatoryMockGraphClient()
	mockGraph.setResult(
		[]string{
			"metric_name", "workload_namespace", "workload_name", "integration",
			"mean", "stddev", "median", "p50", "p90", "p99", "min", "max",
			"sample_count", "window_start", "window_end", "last_updated", "expires_at",
		},
		[][]any{
			{
				"http_requests_total", "prod", "api-server", "grafana-prod",
				100.0, 10.0, 98.0, 98.0, 115.0, 120.0, 80.0, 130.0,
				168, int64(1000), int64(2000), int64(3000), int64(time.Now().Unix() + 86400),
			},
		},
	)

	provider := NewGrafanaObservatoryProvider(mockGraph, "grafana-prod", logger)

	baseline, err := provider.GetBaseline(ctx, "http_requests_total", "prod", "api-server")
	require.NoError(t, err)
	require.NotNil(t, baseline)

	assert.Equal(t, "http_requests_total", baseline.MetricName)
	assert.Equal(t, "prod", baseline.WorkloadNamespace)
	assert.Equal(t, "api-server", baseline.WorkloadName)
	assert.Equal(t, "grafana-prod", baseline.SourceProvider)
	assert.Equal(t, 100.0, baseline.Mean)
	assert.Equal(t, 10.0, baseline.StdDev)
	assert.Equal(t, 168, baseline.SampleCount)
}

func TestGrafanaObservatoryProvider_GetBaseline_NotFound(t *testing.T) {
	logger := logging.GetLogger("test.observatory.provider")
	ctx := context.Background()

	mockGraph := newObservatoryMockGraphClient()
	mockGraph.setResult(
		[]string{
			"metric_name", "workload_namespace", "workload_name", "integration",
			"mean", "stddev", "median", "p50", "p90", "p99", "min", "max",
			"sample_count", "window_start", "window_end", "last_updated", "expires_at",
		},
		[][]any{},
	)

	provider := NewGrafanaObservatoryProvider(mockGraph, "grafana-prod", logger)

	baseline, err := provider.GetBaseline(ctx, "nonexistent_metric", "prod", "api-server")
	require.NoError(t, err)
	assert.Nil(t, baseline, "should return nil for non-existent baseline")
}

func TestGrafanaObservatoryProvider_GetAlertState(t *testing.T) {
	logger := logging.GetLogger("test.observatory.provider")
	ctx := context.Background()

	mockGraph := newObservatoryMockGraphClient()
	mockGraph.setResult(
		[]string{"state"},
		[][]any{
			{"firing"},
		},
	)

	provider := NewGrafanaObservatoryProvider(mockGraph, "grafana-prod", logger)

	state, err := provider.GetAlertState(ctx, "http_requests_total", "prod", "api-server")
	require.NoError(t, err)
	assert.Equal(t, "firing", state)
}

func TestGrafanaObservatoryProvider_GetAlertState_NoAlert(t *testing.T) {
	logger := logging.GetLogger("test.observatory.provider")
	ctx := context.Background()

	mockGraph := newObservatoryMockGraphClient()
	mockGraph.setResult(
		[]string{"state"},
		[][]any{},
	)

	provider := NewGrafanaObservatoryProvider(mockGraph, "grafana-prod", logger)

	state, err := provider.GetAlertState(ctx, "http_requests_total", "prod", "api-server")
	require.NoError(t, err)
	assert.Empty(t, state, "should return empty for no alert")
}

func TestGrafanaObservatoryProvider_CanRegisterWithRegistry(t *testing.T) {
	logger := logging.GetLogger("test.observatory.provider")

	mockGraph := newObservatoryMockGraphClient()
	mockGraph.setResult(
		[]string{
			"metric_name", "workload_namespace", "workload_name",
			"role", "confidence", "quality_score",
			"dashboard_uid", "panel_id", "first_seen", "last_seen", "expires_at",
		},
		[][]any{
			{
				"http_requests_total", "prod", "api-server",
				"Traffic", 0.9, 0.85,
				"dashboard-123", 1, int64(1000), int64(2000), int64(time.Now().Unix() + 86400),
			},
		},
	)

	provider := NewGrafanaObservatoryProvider(mockGraph, "grafana-prod", logger)

	// Register with observatory.Registry
	registry := observatory.NewRegistry()
	err := registry.Register(provider)
	require.NoError(t, err)

	// List signals via registry
	ctx := context.Background()
	signals, err := registry.ListAllSignalAnchors(ctx, observatory.SignalListOptions{})
	require.NoError(t, err)
	require.Len(t, signals, 1)
	assert.Equal(t, "http_requests_total", signals[0].MetricName)
	assert.Equal(t, "grafana-prod", signals[0].SourceProvider)
}

func TestGrafanaIntegration_ObservatoryRegistryMethods(t *testing.T) {
	// Create a minimal GrafanaIntegration to test registry methods
	logger := logging.GetLogger("test.observatory.provider")

	mockGraph := newObservatoryMockGraphClient()
	mockGraph.setResult(
		[]string{
			"metric_name", "workload_namespace", "workload_name",
			"role", "confidence", "quality_score",
			"dashboard_uid", "panel_id", "first_seen", "last_seen", "expires_at",
		},
		[][]any{
			{
				"test_metric", "default", "nginx",
				"Traffic", 0.85, 0.8,
				"dash-1", 1, int64(1000), int64(2000), int64(time.Now().Unix() + 86400),
			},
		},
	)

	// Create the provider and registry manually (simulating what Start() does)
	provider := NewGrafanaObservatoryProvider(mockGraph, "test-integration", logger)
	registry := observatory.NewRegistry()
	err := registry.Register(provider)
	require.NoError(t, err)

	// Create a minimal integration struct with the registry wired
	integration := &GrafanaIntegration{
		name:                "test-integration",
		logger:              logger,
		observatoryRegistry: registry,
		observatoryProvider: provider,
	}

	// Test GetObservatoryRegistry
	gotRegistry := integration.GetObservatoryRegistry()
	assert.NotNil(t, gotRegistry)
	assert.Equal(t, registry, gotRegistry)

	// Test GetObservatoryProvider
	gotProvider := integration.GetObservatoryProvider()
	assert.NotNil(t, gotProvider)
	assert.Equal(t, "test-integration", gotProvider.Name())

	// Test NewObservatoryServiceFromRegistry
	svc := integration.NewObservatoryServiceFromRegistry()
	assert.NotNil(t, svc, "should create observatory.Service from registry")

	// Test NewObservatoryInvestigateServiceFromRegistry
	invSvc := integration.NewObservatoryInvestigateServiceFromRegistry()
	assert.NotNil(t, invSvc, "should create observatory.InvestigateService from registry")

	// Verify the services work with the registry
	ctx := context.Background()
	result, err := svc.GetClusterAnomalies(ctx, nil)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGrafanaIntegration_ObservatoryRegistryMethods_NilRegistry(t *testing.T) {
	// Test that methods handle nil registry gracefully
	integration := &GrafanaIntegration{
		name:                "test-integration",
		observatoryRegistry: nil,
		observatoryProvider: nil,
	}

	// Test GetObservatoryRegistry returns nil
	assert.Nil(t, integration.GetObservatoryRegistry())

	// Test GetObservatoryProvider returns nil
	assert.Nil(t, integration.GetObservatoryProvider())

	// Test NewObservatoryServiceFromRegistry returns nil
	assert.Nil(t, integration.NewObservatoryServiceFromRegistry())

	// Test NewObservatoryInvestigateServiceFromRegistry returns nil
	assert.Nil(t, integration.NewObservatoryInvestigateServiceFromRegistry())
}
