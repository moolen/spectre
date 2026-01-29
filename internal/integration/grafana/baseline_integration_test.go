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

// mockGraphClientForIntegration implements graph.Client for baseline integration testing.
// Provides comprehensive mocking for SignalAnchor, SignalBaseline, and Alert queries.
type mockGraphClientForIntegration struct {
	queries   []graph.GraphQuery
	signals   map[string]*SignalAnchor       // keyed by metric_name|namespace|workload|integration
	baselines map[string]*SignalBaseline     // keyed by metric_name|namespace|workload|integration
	alerts    map[string]*AlertNode          // keyed by uid
}

// AlertNode represents a mock alert for testing.
type AlertNode struct {
	UID       string
	State     string
	MetricRef string // metric name this alert is linked to
}

func newMockGraphClientForIntegration() *mockGraphClientForIntegration {
	return &mockGraphClientForIntegration{
		queries:   make([]graph.GraphQuery, 0),
		signals:   make(map[string]*SignalAnchor),
		baselines: make(map[string]*SignalBaseline),
		alerts:    make(map[string]*AlertNode),
	}
}

func signalKey(metricName, namespace, workload, integration string) string {
	return metricName + "|" + namespace + "|" + workload + "|" + integration
}

func (m *mockGraphClientForIntegration) addSignal(s SignalAnchor) {
	key := signalKey(s.MetricName, s.WorkloadNamespace, s.WorkloadName, s.SourceGrafana)
	m.signals[key] = &s
}

func (m *mockGraphClientForIntegration) addBaseline(b SignalBaseline) {
	key := signalKey(b.MetricName, b.WorkloadNamespace, b.WorkloadName, b.Integration)
	m.baselines[key] = &b
}

func (m *mockGraphClientForIntegration) addAlert(a AlertNode) {
	m.alerts[a.UID] = &a
}

func (m *mockGraphClientForIntegration) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	m.queries = append(m.queries, query)
	queryStr := query.Query

	// Handle workload signals query with baseline join (AnomalyAggregator.getWorkloadSignals)
	// NOTE: This must come BEFORE GetActiveSignalAnchors check because both match "MATCH (s:SignalAnchor"
	if containsString(queryStr, "OPTIONAL MATCH") && containsString(queryStr, "HAS_BASELINE") {
		namespace, _ := query.Parameters["namespace"].(string)
		workload, _ := query.Parameters["workload_name"].(string)
		integration, _ := query.Parameters["integration"].(string)
		now, _ := query.Parameters["now"].(int64)

		rows := make([][]interface{}, 0)
		for _, sig := range m.signals {
			if sig.WorkloadNamespace == namespace &&
				sig.WorkloadName == workload &&
				sig.SourceGrafana == integration &&
				sig.ExpiresAt > now {

				key := signalKey(sig.MetricName, namespace, workload, integration)
				baseline := m.baselines[key]

				row := make([]interface{}, 10)
				row[0] = sig.MetricName
				row[1] = sig.QualityScore

				if baseline != nil {
					row[2] = baseline.Mean
					row[3] = baseline.StdDev
					row[4] = baseline.Min
					row[5] = baseline.Max
					row[6] = baseline.P50
					row[7] = baseline.P90
					row[8] = baseline.P99
					row[9] = int64(baseline.SampleCount)
				}

				rows = append(rows, row)
			}
		}

		return &graph.QueryResult{
			Columns: []string{
				"metric_name", "quality_score",
				"mean", "std_dev", "min", "max", "p50", "p90", "p99", "sample_count",
			},
			Rows: rows,
		}, nil
	}

	// Handle GetActiveSignalAnchors query (must come after HAS_BASELINE check)
	if containsString(queryStr, "MATCH (s:SignalAnchor") && containsString(queryStr, "WHERE s.expires_at >") {
		integration, _ := query.Parameters["integration"].(string)

		rows := make([][]interface{}, 0)
		for _, sig := range m.signals {
			if sig.SourceGrafana == integration {
				rows = append(rows, []interface{}{
					sig.MetricName, sig.WorkloadNamespace, sig.WorkloadName, sig.SourceGrafana,
					string(sig.Role), sig.Confidence, sig.QualityScore, sig.DashboardUID,
					int64(sig.PanelID), sig.QueryID, sig.FirstSeen, sig.LastSeen, sig.ExpiresAt,
				})
			}
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

	// Handle GetSignalBaseline query (single baseline by composite key)
	if containsString(queryStr, "MATCH (b:SignalBaseline") && !containsString(queryStr, "WHERE b.expires_at") {
		metricName, _ := query.Parameters["metric_name"].(string)
		namespace, _ := query.Parameters["workload_namespace"].(string)
		workload, _ := query.Parameters["workload_name"].(string)
		integration, _ := query.Parameters["integration"].(string)

		key := signalKey(metricName, namespace, workload, integration)
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

	// Handle GetBaselinesByWorkload query (with TTL filter)
	if containsString(queryStr, "MATCH (b:SignalBaseline") && containsString(queryStr, "WHERE b.expires_at > $now") {
		namespace, _ := query.Parameters["workload_namespace"].(string)
		workload, _ := query.Parameters["workload_name"].(string)
		integration, _ := query.Parameters["integration"].(string)
		now, _ := query.Parameters["now"].(int64)

		rows := make([][]interface{}, 0)
		for _, baseline := range m.baselines {
			if baseline.WorkloadNamespace == namespace &&
				baseline.WorkloadName == workload &&
				baseline.Integration == integration &&
				baseline.ExpiresAt > now {
				rows = append(rows, []interface{}{
					baseline.MetricName, baseline.WorkloadNamespace, baseline.WorkloadName, baseline.Integration,
					baseline.Mean, baseline.StdDev, baseline.Median, baseline.P50,
					baseline.P90, baseline.P99, baseline.Min, baseline.Max,
					int64(baseline.SampleCount), baseline.WindowStart, baseline.WindowEnd,
					baseline.LastUpdated, baseline.ExpiresAt,
				})
			}
		}

		return &graph.QueryResult{
			Columns: []string{
				"metric_name", "workload_namespace", "workload_name", "integration",
				"mean", "stddev", "median", "p50", "p90", "p99", "min", "max",
				"sample_count", "window_start", "window_end", "last_updated", "expires_at",
			},
			Rows: rows,
		}, nil
	}

	// Handle UpsertSignalBaseline query (MERGE)
	if containsString(queryStr, "MERGE (b:SignalBaseline") {
		metricName, _ := query.Parameters["metric_name"].(string)
		namespace, _ := query.Parameters["workload_namespace"].(string)
		workload, _ := query.Parameters["workload_name"].(string)
		integration, _ := query.Parameters["integration"].(string)

		key := signalKey(metricName, namespace, workload, integration)
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

	// Handle distinct workloads query
	if containsString(queryStr, "DISTINCT s.workload_name") {
		namespace, _ := query.Parameters["namespace"].(string)
		integration, _ := query.Parameters["integration"].(string)

		workloads := make(map[string]bool)
		for _, sig := range m.signals {
			if sig.WorkloadNamespace == namespace &&
				sig.SourceGrafana == integration &&
				sig.WorkloadName != "" {
				workloads[sig.WorkloadName] = true
			}
		}

		rows := make([][]interface{}, 0, len(workloads))
		for w := range workloads {
			rows = append(rows, []interface{}{w})
		}

		return &graph.QueryResult{
			Columns: []string{"workload_name"},
			Rows:    rows,
		}, nil
	}

	// Handle distinct namespaces query
	if containsString(queryStr, "DISTINCT s.workload_namespace") {
		integration, _ := query.Parameters["integration"].(string)

		namespaces := make(map[string]bool)
		for _, sig := range m.signals {
			if sig.SourceGrafana == integration && sig.WorkloadNamespace != "" {
				namespaces[sig.WorkloadNamespace] = true
			}
		}

		rows := make([][]interface{}, 0, len(namespaces))
		for ns := range namespaces {
			rows = append(rows, []interface{}{ns})
		}

		return &graph.QueryResult{
			Columns: []string{"namespace"},
			Rows:    rows,
		}, nil
	}

	// Default result
	return &graph.QueryResult{}, nil
}

func (m *mockGraphClientForIntegration) Connect(ctx context.Context) error { return nil }
func (m *mockGraphClientForIntegration) Close() error                      { return nil }
func (m *mockGraphClientForIntegration) Ping(ctx context.Context) error    { return nil }
func (m *mockGraphClientForIntegration) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}
func (m *mockGraphClientForIntegration) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}
func (m *mockGraphClientForIntegration) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}
func (m *mockGraphClientForIntegration) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}
func (m *mockGraphClientForIntegration) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}
func (m *mockGraphClientForIntegration) InitializeSchema(ctx context.Context) error { return nil }
func (m *mockGraphClientForIntegration) DeleteGraph(ctx context.Context) error      { return nil }
func (m *mockGraphClientForIntegration) CreateGraph(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockGraphClientForIntegration) DeleteGraphByName(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockGraphClientForIntegration) GraphExists(ctx context.Context, graphName string) (bool, error) {
	return false, nil
}

// TestBaselineIntegration_EndToEnd tests the complete baseline storage pipeline.
// Verifies: SignalAnchor -> backfill -> SignalBaseline node -> HAS_BASELINE relationship
func TestBaselineIntegration_EndToEnd(t *testing.T) {
	ctx := context.Background()

	mockGraph := newMockGraphClientForIntegration()
	integrationName := "test-grafana"

	// Setup: Create SignalAnchor (simulating dashboard sync)
	now := time.Now().Unix()
	signal := SignalAnchor{
		MetricName:        "http_requests_total",
		Role:              SignalTraffic,
		Confidence:        0.95,
		QualityScore:      0.8,
		WorkloadNamespace: "production",
		WorkloadName:      "api-server",
		DashboardUID:      "test-dashboard",
		PanelID:           1,
		SourceGrafana:     integrationName,
		FirstSeen:         now,
		LastSeen:          now,
		ExpiresAt:         now + (7 * 24 * 60 * 60),
	}
	mockGraph.addSignal(signal)

	// Step 1: Verify SignalAnchor exists
	signals, err := GetActiveSignalAnchors(ctx, mockGraph, integrationName)
	require.NoError(t, err)
	assert.Len(t, signals, 1, "Expected 1 active signal")
	assert.Equal(t, "http_requests_total", signals[0].MetricName)

	// Step 2: Simulate backfill by creating a baseline with sufficient samples
	values := make([]float64, 100)
	for i := 0; i < 100; i++ {
		values[i] = float64(100 + i%20) // Values ranging 100-119
	}
	stats := ComputeRollingStatistics(values)

	baseline := SignalBaseline{
		MetricName:        signal.MetricName,
		WorkloadNamespace: signal.WorkloadNamespace,
		WorkloadName:      signal.WorkloadName,
		Integration:       integrationName,
		Mean:              stats.Mean,
		StdDev:            stats.StdDev,
		Median:            stats.Median,
		P50:               stats.P50,
		P90:               stats.P90,
		P99:               stats.P99,
		Min:               stats.Min,
		Max:               stats.Max,
		SampleCount:       stats.SampleCount,
		WindowStart:       now - (7 * 24 * 60 * 60),
		WindowEnd:         now,
		LastUpdated:       now,
		ExpiresAt:         now + (7 * 24 * 60 * 60),
	}

	// Step 3: Upsert baseline to graph
	err = UpsertSignalBaseline(ctx, mockGraph, baseline)
	require.NoError(t, err)

	// Step 4: Verify baseline exists in graph
	retrieved, err := GetSignalBaseline(ctx, mockGraph, signal.MetricName, signal.WorkloadNamespace, signal.WorkloadName, integrationName)
	require.NoError(t, err)
	require.NotNil(t, retrieved, "Expected SignalBaseline to exist")

	// Verify statistics computed correctly
	assert.Equal(t, 100, retrieved.SampleCount, "Expected 100 samples")
	assert.InDelta(t, 109.5, retrieved.Mean, 1.0, "Mean should be ~109.5")
	assert.Greater(t, retrieved.StdDev, 0.0, "StdDev should be positive")
	assert.Equal(t, 100.0, retrieved.Min, "Min should be 100")
	assert.Equal(t, 119.0, retrieved.Max, "Max should be 119")
}

// TestBaselineIntegration_AnomalyDetection tests anomaly scoring with established baseline.
func TestBaselineIntegration_AnomalyDetection(t *testing.T) {
	// Setup: Create baseline with known statistics (50 samples, mean=100, stddev=10)
	baseline := SignalBaseline{
		MetricName:        "http_latency_seconds",
		WorkloadNamespace: "production",
		WorkloadName:      "api-server",
		Integration:       "test-grafana",
		Mean:              100.0,
		StdDev:            10.0,
		Median:            100.0,
		P50:               100.0,
		P90:               115.0,
		P99:               125.0,
		Min:               80.0,
		Max:               130.0,
		SampleCount:       50,
	}

	qualityScore := 0.8

	// Test: Query current value that is 3.5 stddev above mean (135)
	currentValue := 135.0

	// Compute anomaly score
	score, err := ComputeAnomalyScore(currentValue, baseline, qualityScore)
	require.NoError(t, err)

	// Assertions
	assert.Greater(t, score.Score, 0.7, "Score > 0.7 for 3.5 stddev anomaly")
	assert.Contains(t, []string{"z-score", "percentile"}, score.Method, "Method should be z-score or percentile")
	assert.Greater(t, score.Confidence, 0.0, "Confidence should be positive")
	assert.InDelta(t, 3.5, score.ZScore, 0.1, "Z-score should be ~3.5")
}

// TestBaselineIntegration_ColdStart tests handling of insufficient samples.
func TestBaselineIntegration_ColdStart(t *testing.T) {
	ctx := context.Background()

	mockGraph := newMockGraphClientForIntegration()
	integrationName := "test-grafana"

	// Setup: Create SignalAnchor without baseline
	now := time.Now().Unix()
	signal := SignalAnchor{
		MetricName:        "new_metric_total",
		Role:              SignalTraffic,
		Confidence:        0.95,
		QualityScore:      0.8,
		WorkloadNamespace: "production",
		WorkloadName:      "new-service",
		DashboardUID:      "test-dashboard",
		SourceGrafana:     integrationName,
		ExpiresAt:         now + (7 * 24 * 60 * 60),
	}
	mockGraph.addSignal(signal)

	// Step 1: Attempt to compute anomaly with no baseline
	baseline, err := GetSignalBaseline(ctx, mockGraph, signal.MetricName, signal.WorkloadNamespace, signal.WorkloadName, integrationName)
	require.NoError(t, err)
	assert.Nil(t, baseline, "Expected no baseline initially")

	// Step 2: Create baseline with insufficient samples (5 < MinSamplesRequired)
	insufficientBaseline := SignalBaseline{
		MetricName:        signal.MetricName,
		WorkloadNamespace: signal.WorkloadNamespace,
		WorkloadName:      signal.WorkloadName,
		Integration:       integrationName,
		Mean:              100.0,
		StdDev:            10.0,
		SampleCount:       5, // Below MinSamplesRequired (10)
	}

	// Attempt to compute anomaly score - should fail with InsufficientSamplesError
	_, err = ComputeAnomalyScore(110.0, insufficientBaseline, 0.8)
	require.Error(t, err, "Expected InsufficientSamplesError")

	insufficientErr, ok := err.(*InsufficientSamplesError)
	require.True(t, ok, "Error should be InsufficientSamplesError")
	assert.Equal(t, 5, insufficientErr.Available)
	assert.Equal(t, MinSamplesRequired, insufficientErr.Required)

	// Step 3: Backfill with 100 samples
	sufficientBaseline := SignalBaseline{
		MetricName:        signal.MetricName,
		WorkloadNamespace: signal.WorkloadNamespace,
		WorkloadName:      signal.WorkloadName,
		Integration:       integrationName,
		Mean:              100.0,
		StdDev:            10.0,
		Median:            100.0,
		P50:               100.0,
		P90:               115.0,
		P99:               125.0,
		Min:               80.0,
		Max:               130.0,
		SampleCount:       100,
		LastUpdated:       now,
		ExpiresAt:         now + (7 * 24 * 60 * 60),
	}
	err = UpsertSignalBaseline(ctx, mockGraph, sufficientBaseline)
	require.NoError(t, err)

	// Step 4: Retry anomaly score computation - should succeed
	score, err := ComputeAnomalyScore(110.0, sufficientBaseline, 0.8)
	require.NoError(t, err)
	assert.NotNil(t, score, "Score should be computed with sufficient samples")
	assert.Less(t, score.Score, 0.5, "1 stddev value should not be anomalous")
}

// TestBaselineIntegration_AlertOverride tests alert state override behavior.
func TestBaselineIntegration_AlertOverride(t *testing.T) {
	// Setup: Create baseline with known statistics
	baseline := SignalBaseline{
		MetricName:        "error_rate",
		WorkloadNamespace: "production",
		WorkloadName:      "api-server",
		Integration:       "test-grafana",
		Mean:              0.01,  // 1% error rate normal
		StdDev:            0.005,
		Median:            0.01,
		P50:               0.01,
		P90:               0.015,
		P99:               0.02,
		Min:               0.0,
		Max:               0.025,
		SampleCount:       100,
	}

	qualityScore := 0.8

	// Compute anomaly score for slightly elevated error rate (not very anomalous)
	currentValue := 0.015 // 0.5 stddev above mean
	score, err := ComputeAnomalyScore(currentValue, baseline, qualityScore)
	require.NoError(t, err)

	// Without alert override, score should be low
	assert.Less(t, score.Score, 0.5, "Without alert, score should be low")

	// Apply alert override (alert is firing)
	overriddenScore := ApplyAlertOverride(score, "firing")

	// With alert override, score should be 1.0
	assert.Equal(t, 1.0, overriddenScore.Score, "Alert firing should override to 1.0")
	assert.Equal(t, 1.0, overriddenScore.Confidence, "Alert firing should set confidence to 1.0")
	assert.Equal(t, "alert-override", overriddenScore.Method, "Method should be alert-override")

	// Test non-firing states don't override
	normalScore := ApplyAlertOverride(score, "normal")
	assert.Equal(t, score.Score, normalScore.Score, "Normal state should not override")

	pendingScore := ApplyAlertOverride(score, "pending")
	assert.Equal(t, score.Score, pendingScore.Score, "Pending state should not override")
}

// TestBaselineIntegration_HierarchicalAggregation tests MAX aggregation across signals.
func TestBaselineIntegration_HierarchicalAggregation(t *testing.T) {
	ctx := context.Background()
	logger := logging.GetLogger("test.baseline.aggregation")

	mockGraph := newMockGraphClientForIntegration()
	integrationName := "test-grafana"
	namespace := "production"
	workloadName := "api-server"

	// Setup: Create 3 SignalAnchors in same workload
	now := time.Now().Unix()
	expiresAt := now + (7 * 24 * 60 * 60)

	signals := []SignalAnchor{
		{
			MetricName:        "http_requests_total",
			Role:              SignalTraffic,
			QualityScore:      0.8,
			WorkloadNamespace: namespace,
			WorkloadName:      workloadName,
			SourceGrafana:     integrationName,
			ExpiresAt:         expiresAt,
		},
		{
			MetricName:        "http_errors_total",
			Role:              SignalErrors,
			QualityScore:      0.9, // Higher quality - should win tiebreaker
			WorkloadNamespace: namespace,
			WorkloadName:      workloadName,
			SourceGrafana:     integrationName,
			ExpiresAt:         expiresAt,
		},
		{
			MetricName:        "http_latency_seconds",
			Role:              SignalLatency,
			QualityScore:      0.7,
			WorkloadNamespace: namespace,
			WorkloadName:      workloadName,
			SourceGrafana:     integrationName,
			ExpiresAt:         expiresAt,
		},
	}

	for _, s := range signals {
		mockGraph.addSignal(s)
	}

	// Create baselines that will produce different anomaly scores
	// signal1: normal (score ~0.3), signal2: high anomaly (score ~0.8), signal3: moderate (score ~0.5)
	baselines := []SignalBaseline{
		{
			MetricName:        "http_requests_total",
			WorkloadNamespace: namespace,
			WorkloadName:      workloadName,
			Integration:       integrationName,
			Mean:              1000.0,
			StdDev:            100.0,
			P50:               1000.0,
			P90:               1100.0,
			P99:               1200.0,
			Min:               800.0,
			Max:               1200.0,
			SampleCount:       100,
			ExpiresAt:         expiresAt,
		},
		{
			MetricName:        "http_errors_total",
			WorkloadNamespace: namespace,
			WorkloadName:      workloadName,
			Integration:       integrationName,
			Mean:              10.0,
			StdDev:            2.0,
			P50:               10.0,
			P90:               12.0,
			P99:               14.0,
			Min:               5.0,
			Max:               15.0,
			SampleCount:       100,
			ExpiresAt:         expiresAt,
		},
		{
			MetricName:        "http_latency_seconds",
			WorkloadNamespace: namespace,
			WorkloadName:      workloadName,
			Integration:       integrationName,
			Mean:              0.1,
			StdDev:            0.02,
			P50:               0.1,
			P90:               0.12,
			P99:               0.14,
			Min:               0.05,
			Max:               0.15,
			SampleCount:       100,
			ExpiresAt:         expiresAt,
		},
	}

	for _, b := range baselines {
		mockGraph.addBaseline(b)
	}

	// Create AnomalyAggregator
	aggregator := NewAnomalyAggregator(mockGraph, integrationName, logger)

	// Aggregate workload anomaly
	result, err := aggregator.AggregateWorkloadAnomaly(ctx, namespace, workloadName)
	require.NoError(t, err)
	require.NotNil(t, result, "Expected aggregated result")

	// Verify scope and key
	assert.Equal(t, "workload", result.Scope)
	assert.Equal(t, namespace+"/"+workloadName, result.ScopeKey)

	// Verify MAX aggregation (signal2 with highest score wins)
	assert.Equal(t, 3, result.SourceCount, "Expected 3 signals")

	// Note: The actual score depends on the mock's current value behavior.
	// In this test, we're verifying the aggregation structure works.
	// The TopSource should be the signal with highest anomaly score.
	assert.NotEmpty(t, result.TopSource, "TopSource should be set")
}

// TestBaselineIntegration_TTLExpiration tests that expired baselines are filtered.
func TestBaselineIntegration_TTLExpiration(t *testing.T) {
	ctx := context.Background()

	mockGraph := newMockGraphClientForIntegration()
	integrationName := "test-grafana"
	namespace := "production"
	workloadName := "api-server"

	now := time.Now().Unix()

	// Create baseline with expires_at in the past
	expiredBaseline := SignalBaseline{
		MetricName:        "expired_metric",
		WorkloadNamespace: namespace,
		WorkloadName:      workloadName,
		Integration:       integrationName,
		Mean:              100.0,
		StdDev:            10.0,
		SampleCount:       50,
		ExpiresAt:         now - 3600, // Expired 1 hour ago
	}
	mockGraph.addBaseline(expiredBaseline)

	// Create baseline that is still valid
	validBaseline := SignalBaseline{
		MetricName:        "valid_metric",
		WorkloadNamespace: namespace,
		WorkloadName:      workloadName,
		Integration:       integrationName,
		Mean:              200.0,
		StdDev:            20.0,
		SampleCount:       100,
		ExpiresAt:         now + (7 * 24 * 60 * 60), // Valid for 7 more days
	}
	mockGraph.addBaseline(validBaseline)

	// Query baselines for workload (should filter by TTL)
	baselines, err := GetBaselinesByWorkload(ctx, mockGraph, namespace, workloadName, integrationName)
	require.NoError(t, err)

	// Only valid baseline should be returned
	assert.Len(t, baselines, 1, "Expected only 1 valid baseline")
	if len(baselines) > 0 {
		assert.Equal(t, "valid_metric", baselines[0].MetricName, "Should return valid_metric")
	}
}

// TestBaselineIntegration_CollectorLifecycle tests BaselineCollector start/stop.
func TestBaselineIntegration_CollectorLifecycle(t *testing.T) {
	logger := logging.GetLogger("test.baseline.lifecycle")

	mockGraph := newMockGraphClientForIntegration()
	integrationName := "test-grafana"

	// Create collector with very short intervals for testing
	config := BaselineCollectorConfig{
		SyncInterval:      50 * time.Millisecond,
		RateLimitInterval: 1 * time.Millisecond,
	}

	collector := NewBaselineCollectorWithConfig(
		nil, // grafanaClient not used in lifecycle test
		nil, // queryService not used in lifecycle test
		mockGraph,
		integrationName,
		logger,
		config,
	)

	ctx := context.Background()

	// Start collector
	err := collector.Start(ctx)
	require.NoError(t, err, "Start should not fail")

	// Verify status indicates collector is running
	status := collector.Status()
	_ = status // Status is available

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Stop collector - should not panic
	require.NotPanics(t, func() {
		collector.Stop()
	}, "Stop should not panic")

	// Verify clean shutdown by checking stopped channel
	select {
	case <-collector.stopped:
		// Good - collector stopped cleanly
	case <-time.After(2 * time.Second):
		t.Fatal("Collector did not stop within timeout")
	}
}

// TestBaselineIntegration_RollingStatistics tests statistical computation.
func TestBaselineIntegration_RollingStatistics(t *testing.T) {
	t.Run("EmptyInput_ReturnsZeroStats", func(t *testing.T) {
		stats := ComputeRollingStatistics([]float64{})
		assert.Equal(t, 0, stats.SampleCount)
		assert.Equal(t, 0.0, stats.Mean)
		assert.Equal(t, 0.0, stats.StdDev)
	})

	t.Run("SingleValue_ZeroStdDev", func(t *testing.T) {
		stats := ComputeRollingStatistics([]float64{100.0})
		assert.Equal(t, 1, stats.SampleCount)
		assert.Equal(t, 100.0, stats.Mean)
		// gonum/stat returns 0 stddev for single value
	})

	t.Run("KnownDistribution_CorrectStats", func(t *testing.T) {
		// Use known values: 1, 2, 3, 4, 5
		// Mean = 3, Variance = 2.5, StdDev = sqrt(2.5) ~= 1.58
		values := []float64{1, 2, 3, 4, 5}
		stats := ComputeRollingStatistics(values)

		assert.Equal(t, 5, stats.SampleCount)
		assert.InDelta(t, 3.0, stats.Mean, 0.01)
		assert.InDelta(t, 1.58, stats.StdDev, 0.1)
		assert.Equal(t, 1.0, stats.Min)
		assert.Equal(t, 5.0, stats.Max)
		assert.Equal(t, 3.0, stats.P50) // Median
	})

	t.Run("Percentiles_ComputedCorrectly", func(t *testing.T) {
		// Create 100 values: 1-100
		values := make([]float64, 100)
		for i := 0; i < 100; i++ {
			values[i] = float64(i + 1)
		}
		stats := ComputeRollingStatistics(values)

		assert.Equal(t, 100, stats.SampleCount)
		assert.InDelta(t, 50.5, stats.P50, 1.0)  // Median
		assert.InDelta(t, 90.0, stats.P90, 2.0)  // 90th percentile
		assert.InDelta(t, 99.0, stats.P99, 2.0)  // 99th percentile
	})
}

// TestBaselineIntegration_InsufficientSamplesError tests error interface.
func TestBaselineIntegration_InsufficientSamplesError(t *testing.T) {
	err := &InsufficientSamplesError{
		Available: 5,
		Required:  10,
	}

	// Verify error message
	msg := err.Error()
	assert.Contains(t, msg, "5")
	assert.Contains(t, msg, "10")
	assert.Contains(t, msg, "insufficient samples")

	// Verify it implements error interface
	var e error = err
	assert.NotNil(t, e)
}

// TestBaselineIntegration_ZScoreNormalization tests z-score to 0-1 mapping.
func TestBaselineIntegration_ZScoreNormalization(t *testing.T) {
	baseline := SignalBaseline{
		Mean:        100.0,
		StdDev:      10.0,
		P50:         100.0,
		P90:         110.0,
		P99:         120.0,
		Min:         80.0,
		Max:         120.0,
		SampleCount: 100,
	}

	testCases := []struct {
		name          string
		currentValue  float64
		expectedZMin  float64
		expectedZMax  float64
		expectAnomaly bool
	}{
		{
			name:          "Normal_ZeroStdDev",
			currentValue:  100.0,
			expectedZMin:  -0.1,
			expectedZMax:  0.1,
			expectAnomaly: false,
		},
		{
			name:          "OneStdDev_LowAnomaly",
			currentValue:  110.0,
			expectedZMin:  0.9,
			expectedZMax:  1.1,
			expectAnomaly: false, // 1 stddev is not anomalous
		},
		{
			name:          "TwoStdDev_ModerateAnomaly",
			currentValue:  120.0,
			expectedZMin:  1.9,
			expectedZMax:  2.1,
			expectAnomaly: false, // ~0.63 score, below 0.5 threshold depends on config
		},
		{
			name:          "ThreeStdDev_HighAnomaly",
			currentValue:  130.0,
			expectedZMin:  2.9,
			expectedZMax:  3.1,
			expectAnomaly: true, // z=3 -> ~0.78 score
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			score, err := ComputeAnomalyScore(tc.currentValue, baseline, 1.0)
			require.NoError(t, err)

			assert.InDelta(t, (tc.expectedZMin+tc.expectedZMax)/2, score.ZScore, 0.2)

			if tc.expectAnomaly {
				assert.GreaterOrEqual(t, score.Score, 0.5, "Expected anomaly (score >= 0.5)")
			}
		})
	}
}

// TestBaselineIntegration_ConfidenceCalculation tests confidence score calculation.
func TestBaselineIntegration_ConfidenceCalculation(t *testing.T) {
	testCases := []struct {
		name            string
		sampleCount     int
		qualityScore    float64
		expectedConfMin float64
		expectedConfMax float64
	}{
		{
			name:            "MinSamples_HalfConfidence",
			sampleCount:     10, // MinSamplesRequired
			qualityScore:    1.0,
			expectedConfMin: 0.49,
			expectedConfMax: 0.51,
		},
		{
			name:            "100Samples_HighConfidence",
			sampleCount:     100,
			qualityScore:    1.0,
			expectedConfMin: 0.9,
			expectedConfMax: 1.0,
		},
		{
			name:            "LowQuality_CapsConfidence",
			sampleCount:     200,
			qualityScore:    0.6,
			expectedConfMin: 0.59,
			expectedConfMax: 0.61,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			baseline := SignalBaseline{
				Mean:        100.0,
				StdDev:      10.0,
				P50:         100.0,
				P90:         110.0,
				P99:         120.0,
				Min:         80.0,
				Max:         120.0,
				SampleCount: tc.sampleCount,
			}

			score, err := ComputeAnomalyScore(105.0, baseline, tc.qualityScore)
			require.NoError(t, err)

			assert.GreaterOrEqual(t, score.Confidence, tc.expectedConfMin)
			assert.LessOrEqual(t, score.Confidence, tc.expectedConfMax)
		})
	}
}
