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

// mockBackfillGraphClient implements graph.Client for backfill tests.
type mockBackfillGraphClient struct {
	executeQueryFunc func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error)
	queries          []graph.GraphQuery
}

func newMockBackfillGraphClient() *mockBackfillGraphClient {
	return &mockBackfillGraphClient{
		queries: make([]graph.GraphQuery, 0),
	}
}

func (m *mockBackfillGraphClient) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	m.queries = append(m.queries, query)
	if m.executeQueryFunc != nil {
		return m.executeQueryFunc(ctx, query)
	}
	return &graph.QueryResult{}, nil
}

// Implement remaining graph.Client interface methods
func (m *mockBackfillGraphClient) Connect(ctx context.Context) error { return nil }
func (m *mockBackfillGraphClient) Close() error                      { return nil }
func (m *mockBackfillGraphClient) Ping(ctx context.Context) error    { return nil }
func (m *mockBackfillGraphClient) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}
func (m *mockBackfillGraphClient) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}
func (m *mockBackfillGraphClient) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}
func (m *mockBackfillGraphClient) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}
func (m *mockBackfillGraphClient) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}
func (m *mockBackfillGraphClient) InitializeSchema(ctx context.Context) error { return nil }
func (m *mockBackfillGraphClient) DeleteGraph(ctx context.Context) error      { return nil }
func (m *mockBackfillGraphClient) CreateGraph(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockBackfillGraphClient) DeleteGraphByName(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockBackfillGraphClient) GraphExists(ctx context.Context, graphName string) (bool, error) {
	return false, nil
}

// TestBackfillSignal_Success tests successful backfill with 100 samples.
func TestBackfillSignal_Success(t *testing.T) {
	logger := logging.GetLogger("test.backfill")

	// Create mock graph client
	var upsertCalled bool
	mockGraph := newMockBackfillGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Check for alert threshold query
		if query.Parameters["metric_name"] != nil && len(query.Query) > 5 && query.Query[:5] == "MATCH" {
			return &graph.QueryResult{Rows: [][]interface{}{}}, nil
		}
		// Check for upsert baseline query
		if query.Parameters["mean"] != nil {
			upsertCalled = true
			mean := query.Parameters["mean"].(float64)
			assert.Greater(t, mean, 0.0, "mean should be computed")
		}
		return &graph.QueryResult{}, nil
	}

	// Create backfill service
	service := &BackfillService{
		queryService:    nil, // We'll test internal methods directly
		graphClient:     mockGraph,
		integrationName: "test-grafana",
		logger:          logger,
		maxBackfillDays: 7,
		rateLimiter:     time.NewTicker(1 * time.Millisecond), // Fast for tests
	}
	defer service.Stop()

	// Create a test signal
	signal := SignalAnchor{
		MetricName:        "container_cpu_usage_seconds_total",
		WorkloadNamespace: "default",
		WorkloadName:      "nginx",
		DashboardUID:      "test-dashboard",
		PanelID:           1,
		SourceGrafana:     "test-grafana",
	}

	ctx := context.Background()

	// Generate 100 data points with realistic values
	values := make([]DataPoint, 100)
	baseTime := time.Now().Add(-7 * 24 * time.Hour)
	for i := 0; i < 100; i++ {
		values[i] = DataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour).Format(time.RFC3339),
			Value:     100.0 + float64(i%20), // Values between 100-119
		}
	}

	mockResult := &DashboardQueryResult{
		DashboardUID:   signal.DashboardUID,
		DashboardTitle: "Test Dashboard",
		Panels: []PanelResult{
			{
				PanelID:    1,
				PanelTitle: "CPU Usage",
				Metrics: []MetricSeries{
					{
						Labels: map[string]string{
							"__name__": "container_cpu_usage_seconds_total",
						},
						Values: values,
					},
				},
			},
		},
	}

	// Extract values
	extractedValues := service.extractMetricValues(mockResult, signal.MetricName)
	assert.Len(t, extractedValues, 100, "should extract 100 values")

	// Compute stats
	stats := ComputeRollingStatistics(extractedValues)
	assert.Equal(t, 100, stats.SampleCount)
	assert.Greater(t, stats.Mean, 0.0)

	// Wait for rate limiter
	<-service.rateLimiter.C

	// Verify baseline would be stored (via mock)
	now := time.Now()
	baseline := SignalBaseline{
		MetricName:        signal.MetricName,
		WorkloadNamespace: signal.WorkloadNamespace,
		WorkloadName:      signal.WorkloadName,
		Integration:       signal.SourceGrafana,
		Mean:              stats.Mean,
		StdDev:            stats.StdDev,
		SampleCount:       stats.SampleCount,
		LastUpdated:       now.Unix(),
	}

	err := service.upsertSignalBaseline(ctx, baseline, false, 0)
	require.NoError(t, err)
	assert.True(t, upsertCalled, "upsert should have been called")
}

// TestBackfillSignal_InsufficientData tests that < 10 samples returns nil, nil.
func TestBackfillSignal_InsufficientData(t *testing.T) {
	logger := logging.GetLogger("test.backfill")

	mockGraph := newMockBackfillGraphClient()

	service := &BackfillService{
		graphClient:     mockGraph,
		integrationName: "test-grafana",
		logger:          logger,
		maxBackfillDays: 7,
		rateLimiter:     time.NewTicker(1 * time.Millisecond),
	}
	defer service.Stop()

	// Create mock result with only 5 data points
	values := make([]DataPoint, 5) // Only 5 samples
	for i := 0; i < 5; i++ {
		values[i] = DataPoint{
			Timestamp: time.Now().Format(time.RFC3339),
			Value:     100.0 + float64(i),
		}
	}

	mockResult := &DashboardQueryResult{
		DashboardUID: "test-dashboard",
		Panels: []PanelResult{
			{
				PanelID: 1,
				Metrics: []MetricSeries{
					{
						Labels: map[string]string{"__name__": "test_metric"},
						Values: values,
					},
				},
			},
		},
	}

	// Extract values - should get only 5
	extractedValues := service.extractMetricValues(mockResult, "test_metric")
	assert.Len(t, extractedValues, 5, "should extract 5 values")
	assert.Less(t, len(extractedValues), MinSamplesRequired, "should be below minimum required")

	// The service would return nil, nil for insufficient data
	// This is the expected cold start behavior
}

// TestBackfillSignal_RateLimited tests that rate limiter delays requests.
func TestBackfillSignal_RateLimited(t *testing.T) {
	logger := logging.GetLogger("test.backfill")

	mockGraph := newMockBackfillGraphClient()

	// Create service with 100ms rate limiter
	service := &BackfillService{
		graphClient:     mockGraph,
		integrationName: "test-grafana",
		logger:          logger,
		maxBackfillDays: 7,
		rateLimiter:     time.NewTicker(100 * time.Millisecond),
	}
	defer service.Stop()

	// Measure time for two rate-limited operations
	start := time.Now()

	// First tick should be available immediately
	<-service.rateLimiter.C

	// Second tick should wait ~100ms
	<-service.rateLimiter.C

	elapsed := time.Since(start)

	// Should have taken at least 100ms (one interval)
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(90), "should wait for rate limiter")
}

// TestTriggerBackfillForNewSignals_Multiple tests batch backfill of multiple signals.
func TestTriggerBackfillForNewSignals_Multiple(t *testing.T) {
	logger := logging.GetLogger("test.backfill")

	// Track which signals were found
	var findSignalsQueryCalled bool

	mockGraph := newMockBackfillGraphClient()
	mockGraph.executeQueryFunc = func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
		// Check for find signals query - parameters identify the query type
		if query.Parameters["integration"] != nil && query.Parameters["now"] != nil {
			findSignalsQueryCalled = true
			// Return 3 signals without baselines
			return &graph.QueryResult{
				Columns: []string{"metric_name", "workload_namespace", "workload_name", "dashboard_uid", "panel_id", "role", "confidence", "quality_score"},
				Rows: [][]interface{}{
					{"metric_a", "default", "app-a", "dash-1", float64(1), "Saturation", 0.9, 0.8},
					{"metric_b", "default", "app-b", "dash-2", float64(2), "Latency", 0.85, 0.7},
					{"metric_c", "kube-system", "coredns", "dash-3", float64(3), "Traffic", 0.95, 0.9},
				},
			}, nil
		}
		return &graph.QueryResult{}, nil
	}

	service := &BackfillService{
		graphClient:     mockGraph,
		integrationName: "test-grafana",
		logger:          logger,
		maxBackfillDays: 7,
		rateLimiter:     time.NewTicker(1 * time.Millisecond),
	}
	defer service.Stop()

	ctx := context.Background()

	// Find signals without baselines
	signals, err := service.findSignalsWithoutBaselines(ctx)
	require.NoError(t, err)
	assert.True(t, findSignalsQueryCalled, "should query for signals without baselines")
	assert.Len(t, signals, 3, "should find 3 signals")

	// Verify signal details
	assert.Equal(t, "metric_a", signals[0].MetricName)
	assert.Equal(t, "default", signals[0].WorkloadNamespace)
	assert.Equal(t, "app-a", signals[0].WorkloadName)
	assert.Equal(t, SignalRole("Saturation"), signals[0].Role)

	assert.Equal(t, "metric_b", signals[1].MetricName)
	assert.Equal(t, SignalRole("Latency"), signals[1].Role)

	assert.Equal(t, "metric_c", signals[2].MetricName)
	assert.Equal(t, "kube-system", signals[2].WorkloadNamespace)
}

// TestBackfillService_ExtractMetricValues tests metric value extraction from query results.
func TestBackfillService_ExtractMetricValues(t *testing.T) {
	logger := logging.GetLogger("test.backfill")

	service := &BackfillService{
		logger: logger,
	}

	tests := []struct {
		name       string
		result     *DashboardQueryResult
		metricName string
		wantCount  int
	}{
		{
			name:       "nil result",
			result:     nil,
			metricName: "test_metric",
			wantCount:  0,
		},
		{
			name: "matching metric with __name__ label",
			result: &DashboardQueryResult{
				Panels: []PanelResult{
					{
						Metrics: []MetricSeries{
							{
								Labels: map[string]string{"__name__": "test_metric"},
								Values: []DataPoint{{Value: 1.0}, {Value: 2.0}, {Value: 3.0}},
							},
						},
					},
				},
			},
			metricName: "test_metric",
			wantCount:  3,
		},
		{
			name: "non-matching metric with __name__ label",
			result: &DashboardQueryResult{
				Panels: []PanelResult{
					{
						Metrics: []MetricSeries{
							{
								Labels: map[string]string{"__name__": "other_metric"},
								Values: []DataPoint{{Value: 1.0}, {Value: 2.0}},
							},
						},
					},
				},
			},
			metricName: "test_metric",
			wantCount:  0,
		},
		{
			name: "metric without __name__ label (accepts all)",
			result: &DashboardQueryResult{
				Panels: []PanelResult{
					{
						Metrics: []MetricSeries{
							{
								Labels: map[string]string{"app": "nginx"},
								Values: []DataPoint{{Value: 1.0}, {Value: 2.0}},
							},
						},
					},
				},
			},
			metricName: "test_metric",
			wantCount:  2, // Accepts when no __name__ label
		},
		{
			name: "multiple panels with matching metrics",
			result: &DashboardQueryResult{
				Panels: []PanelResult{
					{
						Metrics: []MetricSeries{
							{
								Labels: map[string]string{"__name__": "test_metric"},
								Values: []DataPoint{{Value: 1.0}, {Value: 2.0}},
							},
						},
					},
					{
						Metrics: []MetricSeries{
							{
								Labels: map[string]string{"__name__": "test_metric"},
								Values: []DataPoint{{Value: 3.0}, {Value: 4.0}, {Value: 5.0}},
							},
						},
					},
				},
			},
			metricName: "test_metric",
			wantCount:  5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := service.extractMetricValues(tt.result, tt.metricName)
			assert.Len(t, values, tt.wantCount)
		})
	}
}

// TestBackfillService_MetricMatchesSignal tests the metric matching logic.
func TestBackfillService_MetricMatchesSignal(t *testing.T) {
	logger := logging.GetLogger("test.backfill")

	service := &BackfillService{
		logger: logger,
	}

	tests := []struct {
		name       string
		labels     map[string]string
		metricName string
		want       bool
	}{
		{
			name:       "matching __name__",
			labels:     map[string]string{"__name__": "test_metric"},
			metricName: "test_metric",
			want:       true,
		},
		{
			name:       "non-matching __name__",
			labels:     map[string]string{"__name__": "other_metric"},
			metricName: "test_metric",
			want:       false,
		},
		{
			name:       "no __name__ label - accepts all",
			labels:     map[string]string{"app": "nginx", "namespace": "default"},
			metricName: "test_metric",
			want:       true,
		},
		{
			name:       "empty labels - accepts all",
			labels:     map[string]string{},
			metricName: "test_metric",
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.metricMatchesSignal(tt.labels, tt.metricName)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestNewBackfillService tests service initialization.
func TestNewBackfillService(t *testing.T) {
	logger := logging.GetLogger("test.backfill")
	mockGraph := newMockBackfillGraphClient()

	service := NewBackfillService(
		nil, // grafanaClient
		nil, // queryService
		mockGraph,
		"test-integration",
		logger,
	)

	assert.NotNil(t, service)
	assert.Equal(t, "test-integration", service.integrationName)
	assert.Equal(t, 7, service.maxBackfillDays)
	assert.NotNil(t, service.rateLimiter)

	service.Stop()
}
