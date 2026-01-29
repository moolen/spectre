package grafana

import (
	"context"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// mockGraphClientForCollector implements graph.Client for testing baseline collector.
type mockGraphClientForCollector struct {
	queries    []graph.GraphQuery
	signals    []SignalAnchor
	baselines  map[string]*SignalBaseline
}

func newMockGraphClientForCollector() *mockGraphClientForCollector {
	return &mockGraphClientForCollector{
		queries:   make([]graph.GraphQuery, 0),
		baselines: make(map[string]*SignalBaseline),
	}
}

func (m *mockGraphClientForCollector) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	m.queries = append(m.queries, query)

	// Check query type and return appropriate result
	queryStr := query.Query

	// Handle GetActiveSignalAnchors query
	if containsString(queryStr, "MATCH (s:SignalAnchor") {
		now := time.Now().Unix()
		expiresAt := now + (7 * 24 * 60 * 60)

		rows := make([][]interface{}, 0, len(m.signals))
		for _, sig := range m.signals {
			rows = append(rows, []interface{}{
				sig.MetricName, sig.WorkloadNamespace, sig.WorkloadName, sig.SourceGrafana,
				string(sig.Role), sig.Confidence, sig.QualityScore, sig.DashboardUID,
				int64(sig.PanelID), sig.QueryID, sig.FirstSeen, sig.LastSeen, expiresAt,
			})
		}

		return &graph.QueryResult{
			Columns: []string{
				"metric_name", "workload_namespace", "workload_name", "integration",
				"role", "confidence", "quality_score", "dashboard_uid", "panel_id",
				"query_id", "first_seen", "last_seen", "expires_at",
			},
			Rows: rows,
		}, nil
	}

	// Handle GetSignalBaseline query
	if containsString(queryStr, "MATCH (b:SignalBaseline") && !containsString(queryStr, "WHERE b.expires_at") {
		metricName, _ := query.Parameters["metric_name"].(string)
		namespace, _ := query.Parameters["workload_namespace"].(string)
		workload, _ := query.Parameters["workload_name"].(string)
		integration, _ := query.Parameters["integration"].(string)

		key := baselineKey(metricName, namespace, workload, integration)
		if baseline, ok := m.baselines[key]; ok {
			return &graph.QueryResult{
				Columns: []string{
					"metric_name", "workload_namespace", "workload_name", "integration",
					"mean", "stddev", "median", "p50", "p90", "p99", "min", "max",
					"sample_count", "window_start", "window_end", "last_updated", "expires_at",
				},
				Rows: [][]interface{}{
					{
						baseline.MetricName, baseline.WorkloadNamespace, baseline.WorkloadName, baseline.Integration,
						baseline.Mean, baseline.StdDev, baseline.Median, baseline.P50,
						baseline.P90, baseline.P99, baseline.Min, baseline.Max,
						int64(baseline.SampleCount), baseline.WindowStart, baseline.WindowEnd,
						baseline.LastUpdated, baseline.ExpiresAt,
					},
				},
			}, nil
		}

		// Not found
		return &graph.QueryResult{
			Columns: []string{
				"metric_name", "workload_namespace", "workload_name", "integration",
				"mean", "stddev", "median", "p50", "p90", "p99", "min", "max",
				"sample_count", "window_start", "window_end", "last_updated", "expires_at",
			},
			Rows: [][]interface{}{},
		}, nil
	}

	// Handle UpsertSignalBaseline query
	if containsString(queryStr, "MERGE (b:SignalBaseline") {
		// Store the baseline
		metricName, _ := query.Parameters["metric_name"].(string)
		namespace, _ := query.Parameters["workload_namespace"].(string)
		workload, _ := query.Parameters["workload_name"].(string)
		integration, _ := query.Parameters["integration"].(string)

		key := baselineKey(metricName, namespace, workload, integration)
		m.baselines[key] = &SignalBaseline{
			MetricName:        metricName,
			WorkloadNamespace: namespace,
			WorkloadName:      workload,
			Integration:       integration,
			Mean:              parseFloat64(query.Parameters["mean"]),
			StdDev:            parseFloat64(query.Parameters["stddev"]),
			Median:            parseFloat64(query.Parameters["median"]),
			P50:               parseFloat64(query.Parameters["p50"]),
			P90:               parseFloat64(query.Parameters["p90"]),
			P99:               parseFloat64(query.Parameters["p99"]),
			Min:               parseFloat64(query.Parameters["min"]),
			Max:               parseFloat64(query.Parameters["max"]),
			SampleCount:       parseInt(query.Parameters["sample_count"]),
			WindowStart:       parseInt64(query.Parameters["window_start"]),
			WindowEnd:         parseInt64(query.Parameters["window_end"]),
			LastUpdated:       parseInt64(query.Parameters["last_updated"]),
			ExpiresAt:         parseInt64(query.Parameters["expires_at"]),
		}

		return &graph.QueryResult{
			Stats: graph.QueryStats{NodesCreated: 1},
		}, nil
	}

	// Default result
	return &graph.QueryResult{}, nil
}

func baselineKey(metricName, namespace, workload, integration string) string {
	return metricName + "|" + namespace + "|" + workload + "|" + integration
}

func containsString(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func (m *mockGraphClientForCollector) Connect(ctx context.Context) error                { return nil }
func (m *mockGraphClientForCollector) Close() error                                     { return nil }
func (m *mockGraphClientForCollector) Ping(ctx context.Context) error                   { return nil }
func (m *mockGraphClientForCollector) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}
func (m *mockGraphClientForCollector) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}
func (m *mockGraphClientForCollector) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}
func (m *mockGraphClientForCollector) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}
func (m *mockGraphClientForCollector) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}
func (m *mockGraphClientForCollector) InitializeSchema(ctx context.Context) error { return nil }
func (m *mockGraphClientForCollector) DeleteGraph(ctx context.Context) error      { return nil }
func (m *mockGraphClientForCollector) CreateGraph(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockGraphClientForCollector) DeleteGraphByName(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockGraphClientForCollector) GraphExists(ctx context.Context, graphName string) (bool, error) {
	return false, nil
}

func TestBaselineCollector_StartStop(t *testing.T) {
	mockClient := newMockGraphClientForCollector()
	logger := logging.GetLogger("test")

	// Use very short intervals for testing
	config := BaselineCollectorConfig{
		SyncInterval:      50 * time.Millisecond,
		RateLimitInterval: 1 * time.Millisecond,
	}

	collector := NewBaselineCollectorWithConfig(
		nil, // grafanaClient not used in this test
		nil, // queryService not used in this test
		mockClient,
		"test-grafana",
		logger,
		config,
	)

	ctx := context.Background()

	// Start collector
	err := collector.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Stop collector
	collector.Stop()

	// Verify stopped
	select {
	case <-collector.stopped:
		// Good - collector stopped
	case <-time.After(2 * time.Second):
		t.Fatal("Collector did not stop within timeout")
	}
}

func TestBaselineCollector_Status(t *testing.T) {
	mockClient := newMockGraphClientForCollector()
	logger := logging.GetLogger("test")

	config := BaselineCollectorConfig{
		SyncInterval:      1 * time.Hour, // Long interval to prevent auto-sync
		RateLimitInterval: 1 * time.Millisecond,
	}

	collector := NewBaselineCollectorWithConfig(
		nil,
		nil,
		mockClient,
		"test-grafana",
		logger,
		config,
	)

	// Initial status
	status := collector.Status()
	if status.InProgress {
		t.Error("Expected InProgress to be false initially")
	}
	if status.BaselineCount != 0 {
		t.Errorf("Expected BaselineCount 0, got %d", status.BaselineCount)
	}
}

func TestBaselineCollector_RateLimiting(t *testing.T) {
	logger := logging.GetLogger("test")

	// Test rate limiter ticker behavior directly
	// Rate limit interval: 50ms between calls
	config := BaselineCollectorConfig{
		SyncInterval:      1 * time.Hour,
		RateLimitInterval: 50 * time.Millisecond,
	}

	// Verify config is set correctly
	if config.RateLimitInterval != 50*time.Millisecond {
		t.Errorf("Expected RateLimitInterval 50ms, got %v", config.RateLimitInterval)
	}

	// Create collector and verify rateLimiter is created
	mockClient := newMockGraphClientForCollector()
	collector := NewBaselineCollectorWithConfig(
		nil,
		nil,
		mockClient,
		"test-grafana",
		logger,
		config,
	)

	if collector.rateLimiter == nil {
		t.Fatal("Expected rateLimiter to be non-nil")
	}

	// Test that rate limiter ticks at the expected interval
	startTime := time.Now()

	// Wait for 3 ticks
	<-collector.rateLimiter.C
	<-collector.rateLimiter.C
	<-collector.rateLimiter.C

	duration := time.Since(startTime)

	// With 3 ticks and 50ms rate limit, we expect at least 100ms
	// (first tick after 50ms, second after 100ms, third after 150ms)
	minimumExpected := 100 * time.Millisecond

	if duration < minimumExpected {
		t.Errorf("Expected rate limiter to take at least %v for 3 ticks, took %v",
			minimumExpected, duration)
	}

	// Clean up
	collector.rateLimiter.Stop()
}

func TestUpdateBaselineWithSample_FirstSample(t *testing.T) {
	mockClient := newMockGraphClientForCollector()
	logger := logging.GetLogger("test")

	collector := NewBaselineCollector(nil, nil, mockClient, "test", logger)

	baseline := &SignalBaseline{
		MetricName:  "test_metric",
		SampleCount: 0,
	}

	now := time.Now().Unix()
	result := collector.updateBaselineWithSample(baseline, 100.0, now)

	if result.SampleCount != 1 {
		t.Errorf("Expected SampleCount 1, got %d", result.SampleCount)
	}
	if result.Mean != 100.0 {
		t.Errorf("Expected Mean 100.0, got %v", result.Mean)
	}
	if result.Min != 100.0 {
		t.Errorf("Expected Min 100.0, got %v", result.Min)
	}
	if result.Max != 100.0 {
		t.Errorf("Expected Max 100.0, got %v", result.Max)
	}
	if result.StdDev != 0 {
		t.Errorf("Expected StdDev 0 for single sample, got %v", result.StdDev)
	}
}

func TestUpdateBaselineWithSample_MultipleSamples(t *testing.T) {
	mockClient := newMockGraphClientForCollector()
	logger := logging.GetLogger("test")

	collector := NewBaselineCollector(nil, nil, mockClient, "test", logger)

	baseline := &SignalBaseline{
		MetricName:  "test_metric",
		SampleCount: 0,
	}

	now := time.Now().Unix()

	// Add samples: 10, 20, 30
	baseline = collector.updateBaselineWithSample(baseline, 10.0, now)
	baseline = collector.updateBaselineWithSample(baseline, 20.0, now+1)
	baseline = collector.updateBaselineWithSample(baseline, 30.0, now+2)

	if baseline.SampleCount != 3 {
		t.Errorf("Expected SampleCount 3, got %d", baseline.SampleCount)
	}

	// Mean of 10, 20, 30 = 20
	if baseline.Mean != 20.0 {
		t.Errorf("Expected Mean 20.0, got %v", baseline.Mean)
	}

	if baseline.Min != 10.0 {
		t.Errorf("Expected Min 10.0, got %v", baseline.Min)
	}
	if baseline.Max != 30.0 {
		t.Errorf("Expected Max 30.0, got %v", baseline.Max)
	}

	// StdDev should be positive for samples with variance
	if baseline.StdDev <= 0 {
		t.Errorf("Expected positive StdDev, got %v", baseline.StdDev)
	}
}

func TestUpdateBaselineWithSample_UpdatesMinMax(t *testing.T) {
	mockClient := newMockGraphClientForCollector()
	logger := logging.GetLogger("test")

	collector := NewBaselineCollector(nil, nil, mockClient, "test", logger)

	baseline := &SignalBaseline{
		MetricName:  "test_metric",
		Mean:        50.0,
		Min:         40.0,
		Max:         60.0,
		SampleCount: 10,
	}

	now := time.Now().Unix()

	// Add new minimum
	baseline = collector.updateBaselineWithSample(baseline, 20.0, now)
	if baseline.Min != 20.0 {
		t.Errorf("Expected Min 20.0 after lower value, got %v", baseline.Min)
	}

	// Add new maximum
	baseline = collector.updateBaselineWithSample(baseline, 100.0, now+1)
	if baseline.Max != 100.0 {
		t.Errorf("Expected Max 100.0 after higher value, got %v", baseline.Max)
	}
}

func TestUpdatePercentile(t *testing.T) {
	// Test that percentiles move in the right direction
	current := 50.0
	n := 100

	// Value above current - should increase for high percentiles
	result := updatePercentile(current, 100.0, 0.99, n)
	if result <= current {
		t.Errorf("P99 should increase when new value is above current: current=%v, result=%v", current, result)
	}

	// Value below current - should decrease for low percentiles
	result = updatePercentile(current, 10.0, 0.50, n)
	if result >= current {
		t.Errorf("P50 should decrease when new value is below current: current=%v, result=%v", current, result)
	}
}

func TestNewBaselineCollector(t *testing.T) {
	mockClient := newMockGraphClientForCollector()
	logger := logging.GetLogger("test")

	collector := NewBaselineCollector(nil, nil, mockClient, "test-integration", logger)

	if collector == nil {
		t.Fatal("Expected non-nil collector")
	}

	// Verify default config
	if collector.syncInterval != 5*time.Minute {
		t.Errorf("Expected default syncInterval 5m, got %v", collector.syncInterval)
	}

	if collector.integrationName != "test-integration" {
		t.Errorf("Expected integrationName 'test-integration', got %q", collector.integrationName)
	}
}

func TestDefaultBaselineCollectorConfig(t *testing.T) {
	config := DefaultBaselineCollectorConfig()

	if config.SyncInterval != 5*time.Minute {
		t.Errorf("Expected SyncInterval 5m, got %v", config.SyncInterval)
	}

	if config.RateLimitInterval != 100*time.Millisecond {
		t.Errorf("Expected RateLimitInterval 100ms, got %v", config.RateLimitInterval)
	}
}

func TestBaselineCollector_CollectAndUpdate_NoSignals(t *testing.T) {
	mockClient := newMockGraphClientForCollector()
	logger := logging.GetLogger("test")

	// No signals configured
	mockClient.signals = []SignalAnchor{}

	config := BaselineCollectorConfig{
		SyncInterval:      1 * time.Hour,
		RateLimitInterval: 1 * time.Millisecond,
	}

	collector := NewBaselineCollectorWithConfig(nil, nil, mockClient, "test-grafana", logger, config)
	collector.ctx = context.Background()

	err := collector.collectAndUpdate()
	if err != nil {
		t.Errorf("Expected no error with no signals, got: %v", err)
	}

	status := collector.Status()
	if status.BaselineCount != 0 {
		t.Errorf("Expected BaselineCount 0 with no signals, got %d", status.BaselineCount)
	}
	if status.ErrorCount != 0 {
		t.Errorf("Expected ErrorCount 0 with no signals, got %d", status.ErrorCount)
	}
}
