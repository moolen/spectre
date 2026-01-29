package grafana

import (
	"context"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
)

// mockGraphClientForBaseline implements graph.Client for testing baseline storage.
type mockGraphClientForBaseline struct {
	queries  []graph.GraphQuery
	results  map[string]*graph.QueryResult
	baselines map[string]*SignalBaseline // In-memory storage for testing
}

func newMockGraphClientForBaseline() *mockGraphClientForBaseline {
	return &mockGraphClientForBaseline{
		queries:   make([]graph.GraphQuery, 0),
		results:   make(map[string]*graph.QueryResult),
		baselines: make(map[string]*SignalBaseline),
	}
}

func (m *mockGraphClientForBaseline) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	m.queries = append(m.queries, query)

	// Check if we have a specific result for this query
	for key, result := range m.results {
		if key != "" && containsSubstring(query.Query, key) {
			return result, nil
		}
	}

	// Default result
	return &graph.QueryResult{
		Stats: graph.QueryStats{
			NodesCreated:         1,
			RelationshipsCreated: 1,
		},
	}, nil
}

func containsSubstring(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || containsSubstring(s[1:], substr)))
}

func (m *mockGraphClientForBaseline) Connect(ctx context.Context) error                { return nil }
func (m *mockGraphClientForBaseline) Close() error                                     { return nil }
func (m *mockGraphClientForBaseline) Ping(ctx context.Context) error                   { return nil }
func (m *mockGraphClientForBaseline) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}
func (m *mockGraphClientForBaseline) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}
func (m *mockGraphClientForBaseline) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}
func (m *mockGraphClientForBaseline) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}
func (m *mockGraphClientForBaseline) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}
func (m *mockGraphClientForBaseline) InitializeSchema(ctx context.Context) error { return nil }
func (m *mockGraphClientForBaseline) DeleteGraph(ctx context.Context) error      { return nil }
func (m *mockGraphClientForBaseline) CreateGraph(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockGraphClientForBaseline) DeleteGraphByName(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockGraphClientForBaseline) GraphExists(ctx context.Context, graphName string) (bool, error) {
	return false, nil
}

func TestUpsertSignalBaseline_Create(t *testing.T) {
	mockClient := newMockGraphClientForBaseline()
	ctx := context.Background()

	now := time.Now().Unix()
	expiresAt := now + (7 * 24 * 60 * 60) // 7 days

	baseline := SignalBaseline{
		MetricName:        "container_cpu_usage_seconds_total",
		WorkloadNamespace: "production",
		WorkloadName:      "frontend",
		Integration:       "test-grafana",
		Mean:              0.45,
		StdDev:            0.12,
		Median:            0.42,
		P50:               0.42,
		P90:               0.65,
		P99:               0.85,
		Min:               0.10,
		Max:               0.95,
		SampleCount:       100,
		WindowStart:       now - (7 * 24 * 60 * 60),
		WindowEnd:         now,
		LastUpdated:       now,
		ExpiresAt:         expiresAt,
	}

	err := UpsertSignalBaseline(ctx, mockClient, baseline)
	if err != nil {
		t.Fatalf("UpsertSignalBaseline failed: %v", err)
	}

	// Verify query was executed
	if len(mockClient.queries) == 0 {
		t.Fatal("Expected query to be executed")
	}

	// Verify MERGE query was used
	lastQuery := mockClient.queries[len(mockClient.queries)-1]
	if lastQuery.Parameters["metric_name"] != "container_cpu_usage_seconds_total" {
		t.Errorf("Expected metric_name parameter, got %v", lastQuery.Parameters["metric_name"])
	}
	if lastQuery.Parameters["workload_namespace"] != "production" {
		t.Errorf("Expected workload_namespace parameter, got %v", lastQuery.Parameters["workload_namespace"])
	}
	if lastQuery.Parameters["workload_name"] != "frontend" {
		t.Errorf("Expected workload_name parameter, got %v", lastQuery.Parameters["workload_name"])
	}
	if lastQuery.Parameters["integration"] != "test-grafana" {
		t.Errorf("Expected integration parameter, got %v", lastQuery.Parameters["integration"])
	}
	if lastQuery.Parameters["mean"] != 0.45 {
		t.Errorf("Expected mean parameter 0.45, got %v", lastQuery.Parameters["mean"])
	}
	if lastQuery.Parameters["sample_count"] != 100 {
		t.Errorf("Expected sample_count parameter 100, got %v", lastQuery.Parameters["sample_count"])
	}
}

func TestUpsertSignalBaseline_Update(t *testing.T) {
	mockClient := newMockGraphClientForBaseline()
	ctx := context.Background()

	now := time.Now().Unix()
	expiresAt := now + (7 * 24 * 60 * 60)

	// First insert
	baseline1 := SignalBaseline{
		MetricName:        "http_requests_total",
		WorkloadNamespace: "default",
		WorkloadName:      "api",
		Integration:       "test-grafana",
		Mean:              100.0,
		StdDev:            20.0,
		SampleCount:       50,
		LastUpdated:       now,
		ExpiresAt:         expiresAt,
	}

	err := UpsertSignalBaseline(ctx, mockClient, baseline1)
	if err != nil {
		t.Fatalf("First UpsertSignalBaseline failed: %v", err)
	}

	firstQueryCount := len(mockClient.queries)

	// Second insert - same composite key, updated statistics
	baseline2 := SignalBaseline{
		MetricName:        "http_requests_total",
		WorkloadNamespace: "default",
		WorkloadName:      "api",
		Integration:       "test-grafana",
		Mean:              150.0,  // Updated mean
		StdDev:            25.0,   // Updated stddev
		SampleCount:       100,    // More samples
		LastUpdated:       now + 300,
		ExpiresAt:         expiresAt + 300,
	}

	err = UpsertSignalBaseline(ctx, mockClient, baseline2)
	if err != nil {
		t.Fatalf("Second UpsertSignalBaseline failed: %v", err)
	}

	// Verify second query was executed
	if len(mockClient.queries) <= firstQueryCount {
		t.Error("Expected second query to be executed")
	}

	// Verify updated fields
	lastQuery := mockClient.queries[len(mockClient.queries)-1]
	if lastQuery.Parameters["mean"] != 150.0 {
		t.Errorf("Expected updated mean 150.0, got %v", lastQuery.Parameters["mean"])
	}
	if lastQuery.Parameters["sample_count"] != 100 {
		t.Errorf("Expected updated sample_count 100, got %v", lastQuery.Parameters["sample_count"])
	}
}

func TestGetSignalBaseline_Found(t *testing.T) {
	mockClient := newMockGraphClientForBaseline()
	ctx := context.Background()

	now := time.Now().Unix()

	// Set up mock result for the query
	mockClient.results["MATCH (b:SignalBaseline"] = &graph.QueryResult{
		Columns: []string{
			"metric_name", "workload_namespace", "workload_name", "integration",
			"mean", "stddev", "median", "p50", "p90", "p99", "min", "max",
			"sample_count", "window_start", "window_end", "last_updated", "expires_at",
		},
		Rows: [][]interface{}{
			{
				"container_memory_usage_bytes", "production", "backend", "test-grafana",
				float64(1024000000), float64(102400000), float64(1000000000), float64(1000000000),
				float64(1200000000), float64(1400000000), float64(500000000), float64(1500000000),
				int64(75), now - (5 * 24 * 60 * 60), now, now, now + (7 * 24 * 60 * 60),
			},
		},
	}

	baseline, err := GetSignalBaseline(ctx, mockClient, "container_memory_usage_bytes", "production", "backend", "test-grafana")
	if err != nil {
		t.Fatalf("GetSignalBaseline failed: %v", err)
	}

	if baseline == nil {
		t.Fatal("Expected baseline to be returned, got nil")
	}

	// Verify parsed fields
	if baseline.MetricName != "container_memory_usage_bytes" {
		t.Errorf("Expected metric_name 'container_memory_usage_bytes', got %q", baseline.MetricName)
	}
	if baseline.WorkloadNamespace != "production" {
		t.Errorf("Expected workload_namespace 'production', got %q", baseline.WorkloadNamespace)
	}
	if baseline.WorkloadName != "backend" {
		t.Errorf("Expected workload_name 'backend', got %q", baseline.WorkloadName)
	}
	if baseline.Integration != "test-grafana" {
		t.Errorf("Expected integration 'test-grafana', got %q", baseline.Integration)
	}
	if baseline.Mean != 1024000000 {
		t.Errorf("Expected mean 1024000000, got %v", baseline.Mean)
	}
	if baseline.SampleCount != 75 {
		t.Errorf("Expected sample_count 75, got %v", baseline.SampleCount)
	}
}

func TestGetSignalBaseline_NotFound(t *testing.T) {
	mockClient := newMockGraphClientForBaseline()
	ctx := context.Background()

	// Set up empty result for the query
	mockClient.results["MATCH (b:SignalBaseline"] = &graph.QueryResult{
		Columns: []string{
			"metric_name", "workload_namespace", "workload_name", "integration",
			"mean", "stddev", "median", "p50", "p90", "p99", "min", "max",
			"sample_count", "window_start", "window_end", "last_updated", "expires_at",
		},
		Rows: [][]interface{}{}, // Empty - no results
	}

	baseline, err := GetSignalBaseline(ctx, mockClient, "nonexistent_metric", "default", "app", "test-grafana")

	// Should NOT return error for not found
	if err != nil {
		t.Fatalf("GetSignalBaseline should not return error for not found, got: %v", err)
	}

	// Should return nil for not found
	if baseline != nil {
		t.Errorf("Expected nil baseline for not found, got %+v", baseline)
	}
}

func TestGetBaselinesByWorkload_Multiple(t *testing.T) {
	mockClient := newMockGraphClientForBaseline()
	ctx := context.Background()

	now := time.Now().Unix()
	expiresAt := now + (7 * 24 * 60 * 60)

	// Set up mock result with multiple baselines
	mockClient.results["MATCH (b:SignalBaseline"] = &graph.QueryResult{
		Columns: []string{
			"metric_name", "workload_namespace", "workload_name", "integration",
			"mean", "stddev", "median", "p50", "p90", "p99", "min", "max",
			"sample_count", "window_start", "window_end", "last_updated", "expires_at",
		},
		Rows: [][]interface{}{
			{
				"container_cpu_usage_seconds_total", "production", "frontend", "test-grafana",
				float64(0.45), float64(0.12), float64(0.42), float64(0.42),
				float64(0.65), float64(0.85), float64(0.10), float64(0.95),
				int64(100), now - (7 * 24 * 60 * 60), now, now, expiresAt,
			},
			{
				"container_memory_usage_bytes", "production", "frontend", "test-grafana",
				float64(512000000), float64(50000000), float64(500000000), float64(500000000),
				float64(600000000), float64(700000000), float64(400000000), float64(800000000),
				int64(100), now - (7 * 24 * 60 * 60), now, now, expiresAt,
			},
			{
				"http_requests_total", "production", "frontend", "test-grafana",
				float64(1000), float64(200), float64(950), float64(950),
				float64(1300), float64(1500), float64(500), float64(2000),
				int64(100), now - (7 * 24 * 60 * 60), now, now, expiresAt,
			},
		},
	}

	baselines, err := GetBaselinesByWorkload(ctx, mockClient, "production", "frontend", "test-grafana")
	if err != nil {
		t.Fatalf("GetBaselinesByWorkload failed: %v", err)
	}

	if len(baselines) != 3 {
		t.Fatalf("Expected 3 baselines, got %d", len(baselines))
	}

	// Verify each baseline has correct workload info
	for _, baseline := range baselines {
		if baseline.WorkloadNamespace != "production" {
			t.Errorf("Expected workload_namespace 'production', got %q", baseline.WorkloadNamespace)
		}
		if baseline.WorkloadName != "frontend" {
			t.Errorf("Expected workload_name 'frontend', got %q", baseline.WorkloadName)
		}
		if baseline.Integration != "test-grafana" {
			t.Errorf("Expected integration 'test-grafana', got %q", baseline.Integration)
		}
	}

	// Verify distinct metrics
	metrics := make(map[string]bool)
	for _, baseline := range baselines {
		metrics[baseline.MetricName] = true
	}

	expectedMetrics := []string{
		"container_cpu_usage_seconds_total",
		"container_memory_usage_bytes",
		"http_requests_total",
	}

	for _, expected := range expectedMetrics {
		if !metrics[expected] {
			t.Errorf("Expected metric %q not found in baselines", expected)
		}
	}
}

func TestGetBaselinesByWorkload_EmptyResult(t *testing.T) {
	mockClient := newMockGraphClientForBaseline()
	ctx := context.Background()

	// Set up empty result
	mockClient.results["MATCH (b:SignalBaseline"] = &graph.QueryResult{
		Columns: []string{
			"metric_name", "workload_namespace", "workload_name", "integration",
			"mean", "stddev", "median", "p50", "p90", "p99", "min", "max",
			"sample_count", "window_start", "window_end", "last_updated", "expires_at",
		},
		Rows: [][]interface{}{},
	}

	baselines, err := GetBaselinesByWorkload(ctx, mockClient, "default", "nonexistent", "test-grafana")
	if err != nil {
		t.Fatalf("GetBaselinesByWorkload failed: %v", err)
	}

	if len(baselines) != 0 {
		t.Errorf("Expected empty slice, got %d baselines", len(baselines))
	}
}

func TestParseFloat64(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected float64
	}{
		{"float64", float64(1.5), 1.5},
		{"int64", int64(100), 100.0},
		{"int", int(50), 50.0},
		{"string", "invalid", 0.0},
		{"nil", nil, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseFloat64(tt.input)
			if result != tt.expected {
				t.Errorf("parseFloat64(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int
	}{
		{"int", int(100), 100},
		{"int64", int64(200), 200},
		{"float64", float64(50.5), 50},
		{"string", "invalid", 0},
		{"nil", nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseInt(tt.input)
			if result != tt.expected {
				t.Errorf("parseInt(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseInt64(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int64
	}{
		{"int64", int64(1000000), 1000000},
		{"int", int(500), 500},
		{"float64", float64(750.5), 750},
		{"string", "invalid", 0},
		{"nil", nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseInt64(tt.input)
			if result != tt.expected {
				t.Errorf("parseInt64(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestUpsertSignalBaseline_HAS_BASELINE_Relationship(t *testing.T) {
	mockClient := newMockGraphClientForBaseline()
	ctx := context.Background()

	now := time.Now().Unix()
	baseline := SignalBaseline{
		MetricName:        "test_metric",
		WorkloadNamespace: "default",
		WorkloadName:      "app",
		Integration:       "test-grafana",
		Mean:              1.0,
		SampleCount:       10,
		LastUpdated:       now,
		ExpiresAt:         now + (7 * 24 * 60 * 60),
	}

	err := UpsertSignalBaseline(ctx, mockClient, baseline)
	if err != nil {
		t.Fatalf("UpsertSignalBaseline failed: %v", err)
	}

	// Verify query contains HAS_BASELINE relationship
	lastQuery := mockClient.queries[len(mockClient.queries)-1]

	queryContainsHasBaseline := false
	if len(lastQuery.Query) > 0 {
		// Check for HAS_BASELINE in the query string
		for i := 0; i < len(lastQuery.Query)-12; i++ {
			if lastQuery.Query[i:i+12] == "HAS_BASELINE" {
				queryContainsHasBaseline = true
				break
			}
		}
	}

	if !queryContainsHasBaseline {
		t.Error("Expected query to contain HAS_BASELINE relationship")
	}
}

func TestGetActiveSignalAnchors(t *testing.T) {
	mockClient := newMockGraphClientForBaseline()
	ctx := context.Background()

	now := time.Now().Unix()
	expiresAt := now + (7 * 24 * 60 * 60)

	// Set up mock result with active signal anchors
	mockClient.results["MATCH (s:SignalAnchor"] = &graph.QueryResult{
		Columns: []string{
			"metric_name", "workload_namespace", "workload_name", "integration",
			"role", "confidence", "quality_score", "dashboard_uid", "panel_id",
			"query_id", "first_seen", "last_seen", "expires_at",
		},
		Rows: [][]interface{}{
			{
				"container_cpu_usage_seconds_total", "production", "frontend", "test-grafana",
				"Saturation", float64(0.95), float64(0.8), "dash-1", int64(1),
				"dash-1-1-A", now - 1000, now, expiresAt,
			},
			{
				"http_requests_total", "production", "api", "test-grafana",
				"Traffic", float64(0.85), float64(0.75), "dash-2", int64(2),
				"dash-2-2-A", now - 2000, now, expiresAt,
			},
		},
	}

	signals, err := GetActiveSignalAnchors(ctx, mockClient, "test-grafana")
	if err != nil {
		t.Fatalf("GetActiveSignalAnchors failed: %v", err)
	}

	if len(signals) != 2 {
		t.Fatalf("Expected 2 signals, got %d", len(signals))
	}

	// Verify first signal
	if signals[0].MetricName != "container_cpu_usage_seconds_total" {
		t.Errorf("Expected metric_name 'container_cpu_usage_seconds_total', got %q", signals[0].MetricName)
	}
	if signals[0].Role != SignalSaturation {
		t.Errorf("Expected role 'Saturation', got %q", signals[0].Role)
	}

	// Verify second signal
	if signals[1].MetricName != "http_requests_total" {
		t.Errorf("Expected metric_name 'http_requests_total', got %q", signals[1].MetricName)
	}
	if signals[1].Role != SignalTraffic {
		t.Errorf("Expected role 'Traffic', got %q", signals[1].Role)
	}
}
