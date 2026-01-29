package grafana

import (
	"context"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// BackfillService handles historical backfill of baseline statistics for signals.
// Implements opt-in catchup backfill (BASE-05) with rate limiting separate from forward collection.
//
// Backfill process:
// 1. Query graph for SignalAnchors without baselines (HAS_BASELINE relationship)
// 2. For each signal, fetch 7 days of historical data from Grafana
// 3. Compute rolling statistics and store as SignalBaseline
// 4. Create HAS_BASELINE relationship linking signal to baseline
//
// Rate limiting: 2 req/sec (slower than forward collection) to protect Grafana API.
type BackfillService struct {
	grafanaClient   *GrafanaClient
	queryService    *GrafanaQueryService
	graphClient     graph.Client
	integrationName string
	logger          *logging.Logger
	maxBackfillDays int
	rateLimiter     *time.Ticker
}

// NewBackfillService creates a new BackfillService instance.
//
// Parameters:
//   - grafanaClient: Grafana API client for dashboard fetching
//   - queryService: Query service for executing dashboard queries
//   - graphClient: Graph client for storing baselines
//   - integrationName: Grafana integration name
//   - logger: Logger for diagnostic output
func NewBackfillService(
	grafanaClient *GrafanaClient,
	queryService *GrafanaQueryService,
	graphClient graph.Client,
	integrationName string,
	logger *logging.Logger,
) *BackfillService {
	return &BackfillService{
		grafanaClient:   grafanaClient,
		queryService:    queryService,
		graphClient:     graphClient,
		integrationName: integrationName,
		logger:          logger,
		maxBackfillDays: 7, // Per CONTEXT.md: 7-day retention window
		rateLimiter:     time.NewTicker(500 * time.Millisecond), // 2 req/sec
	}
}

// BackfillSignal fetches historical data and computes baseline for a single signal.
//
// Process:
// 1. Calculate time range: now - 7 days to now
// 2. Execute dashboard query for the signal's panel
// 3. Extract values for the specific metric
// 4. If < 10 values: log debug, return nil (cold start, not error)
// 5. Compute rolling statistics via ComputeRollingStatistics
// 6. Check for associated alert thresholds (BASE-06)
// 7. Store baseline via UpsertSignalBaseline
//
// Returns nil error if insufficient data (< 10 samples) - this is expected during cold start.
func (s *BackfillService) BackfillSignal(ctx context.Context, signal SignalAnchor) error {
	// Rate limit before API call
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.rateLimiter.C:
		// Proceed with backfill
	}

	// Calculate time range: now - 7 days to now
	now := time.Now()
	from := now.Add(-time.Duration(s.maxBackfillDays) * 24 * time.Hour)

	timeRange := TimeRange{
		From: from.UTC().Format(time.RFC3339),
		To:   now.UTC().Format(time.RFC3339),
	}

	// Execute dashboard query for the signal's panel
	result, err := s.queryService.ExecuteDashboard(
		ctx,
		signal.DashboardUID,
		timeRange,
		nil, // No scoped vars for backfill
		0,   // All panels (we'll filter by metric)
	)
	if err != nil {
		return fmt.Errorf("execute dashboard query: %w", err)
	}

	// Extract values for the specific metric from all panels
	values := s.extractMetricValues(result, signal.MetricName)

	// Cold start check: if < 10 values, log and return nil (not an error)
	if len(values) < MinSamplesRequired {
		s.logger.Debug("Backfill for signal %s: insufficient data (%d samples, need %d)",
			signal.MetricName, len(values), MinSamplesRequired)
		return nil
	}

	// Compute rolling statistics
	stats := ComputeRollingStatistics(values)

	// Check for associated alert thresholds (BASE-06)
	hasAlert, alertThreshold := s.checkAlertThreshold(ctx, signal.MetricName)

	// Create SignalBaseline
	baseline := SignalBaseline{
		// Identity fields (composite key matching SignalAnchor)
		MetricName:        signal.MetricName,
		WorkloadNamespace: signal.WorkloadNamespace,
		WorkloadName:      signal.WorkloadName,
		Integration:       signal.SourceGrafana,

		// Rolling statistics
		Mean:        stats.Mean,
		StdDev:      stats.StdDev,
		Median:      stats.Median,
		P50:         stats.P50,
		P90:         stats.P90,
		P99:         stats.P99,
		Min:         stats.Min,
		Max:         stats.Max,
		SampleCount: stats.SampleCount,

		// Window metadata
		WindowStart: from.Unix(),
		WindowEnd:   now.Unix(),

		// TTL fields
		LastUpdated: now.Unix(),
		ExpiresAt:   now.Add(7 * 24 * time.Hour).Unix(), // 7-day TTL
	}

	// Store baseline via graph
	if err := s.upsertSignalBaseline(ctx, baseline, hasAlert, alertThreshold); err != nil {
		return fmt.Errorf("upsert baseline: %w", err)
	}

	s.logger.Debug("Backfilled baseline for signal %s: %d samples, mean=%.2f, stddev=%.2f",
		signal.MetricName, stats.SampleCount, stats.Mean, stats.StdDev)

	return nil
}

// TriggerBackfillForNewSignals finds all SignalAnchors without baselines and backfills them.
//
// Process:
// 1. Query graph for SignalAnchors without HAS_BASELINE relationship
// 2. For each signal: call BackfillSignal (rate-limited)
// 3. Log summary: backfilled N signals, M errors
//
// Returns error only if graph query fails, individual signal errors are logged but don't fail the batch.
func (s *BackfillService) TriggerBackfillForNewSignals(ctx context.Context) error {
	// Query graph for SignalAnchors without baselines
	signals, err := s.findSignalsWithoutBaselines(ctx)
	if err != nil {
		return fmt.Errorf("find signals without baselines: %w", err)
	}

	if len(signals) == 0 {
		s.logger.Debug("No signals without baselines found")
		return nil
	}

	s.logger.Info("Starting backfill for %d signals without baselines", len(signals))

	var successCount, errorCount int
	for _, signal := range signals {
		if err := s.BackfillSignal(ctx, signal); err != nil {
			s.logger.Warn("Backfill failed for signal %s: %v", signal.MetricName, err)
			errorCount++
			continue
		}
		successCount++
	}

	s.logger.Info("Backfill complete: %d succeeded, %d failed", successCount, errorCount)
	return nil
}

// extractMetricValues extracts float64 values for a specific metric from dashboard query result.
func (s *BackfillService) extractMetricValues(result *DashboardQueryResult, metricName string) []float64 {
	if result == nil {
		return nil
	}

	var values []float64

	for _, panel := range result.Panels {
		for _, metric := range panel.Metrics {
			// Check if this metric series matches the target metric
			// Metric name might be in labels or inferred from panel context
			if s.metricMatchesSignal(metric.Labels, metricName) {
				for _, dp := range metric.Values {
					values = append(values, dp.Value)
				}
			}
		}
	}

	return values
}

// metricMatchesSignal checks if a metric series matches the target signal metric.
// Uses __name__ label if present, otherwise matches any series from the target panel.
func (s *BackfillService) metricMatchesSignal(labels map[string]string, metricName string) bool {
	// Check __name__ label (standard Prometheus metric name label)
	if name, ok := labels["__name__"]; ok {
		return name == metricName
	}
	// If no __name__ label, accept all series (rely on panel filtering)
	return true
}

// checkAlertThreshold checks if a signal has an associated alert and returns its threshold.
// Implements BASE-06: Alert threshold bootstrapping.
//
// Returns:
//   - hasAlert: true if an alert monitors this metric
//   - threshold: P99 threshold from alert if available, 0 otherwise
func (s *BackfillService) checkAlertThreshold(ctx context.Context, metricName string) (bool, float64) {
	// Query for alerts that monitor this metric
	query := `
		MATCH (a:Alert {integration: $integration})-[:MONITORS]->(m:Metric {name: $metric_name})
		RETURN a.condition AS condition, a.uid AS uid
		LIMIT 1
	`

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"integration":  s.integrationName,
			"metric_name": metricName,
		},
	})
	if err != nil {
		s.logger.Debug("Failed to check alert threshold for %s: %v", metricName, err)
		return false, 0
	}

	if len(result.Rows) == 0 {
		return false, 0
	}

	// Alert exists - threshold extraction would require parsing the condition
	// For now, just flag that an alert exists (threshold parsing is complex)
	return true, 0
}

// findSignalsWithoutBaselines queries the graph for SignalAnchors that don't have baselines.
func (s *BackfillService) findSignalsWithoutBaselines(ctx context.Context) ([]SignalAnchor, error) {
	// Query for signals without HAS_BASELINE relationship
	query := `
		MATCH (s:SignalAnchor {integration: $integration})
		WHERE NOT EXISTS {
			MATCH (s)-[:HAS_BASELINE]->(:SignalBaseline)
		}
		AND s.expires_at > $now
		RETURN s.metric_name AS metric_name,
		       s.workload_namespace AS workload_namespace,
		       s.workload_name AS workload_name,
		       s.dashboard_uid AS dashboard_uid,
		       s.panel_id AS panel_id,
		       s.role AS role,
		       s.confidence AS confidence,
		       s.quality_score AS quality_score
	`

	now := time.Now().Unix()
	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"integration": s.integrationName,
			"now":         now,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("query signals without baselines: %w", err)
	}

	// Map column names to indices
	colIdx := make(map[string]int)
	for i, col := range result.Columns {
		colIdx[col] = i
	}

	signals := make([]SignalAnchor, 0, len(result.Rows))
	for _, row := range result.Rows {
		signal := SignalAnchor{
			SourceGrafana: s.integrationName,
		}

		// Extract fields from row
		if idx, ok := colIdx["metric_name"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				signal.MetricName = v
			}
		}
		if idx, ok := colIdx["workload_namespace"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				signal.WorkloadNamespace = v
			}
		}
		if idx, ok := colIdx["workload_name"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				signal.WorkloadName = v
			}
		}
		if idx, ok := colIdx["dashboard_uid"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				signal.DashboardUID = v
			}
		}
		if idx, ok := colIdx["panel_id"]; ok && idx < len(row) {
			if v, ok := row[idx].(float64); ok {
				signal.PanelID = int(v)
			} else if v, ok := row[idx].(int64); ok {
				signal.PanelID = int(v)
			}
		}
		if idx, ok := colIdx["role"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				signal.Role = SignalRole(v)
			}
		}
		if idx, ok := colIdx["confidence"]; ok && idx < len(row) {
			if v, ok := row[idx].(float64); ok {
				signal.Confidence = v
			}
		}
		if idx, ok := colIdx["quality_score"]; ok && idx < len(row) {
			if v, ok := row[idx].(float64); ok {
				signal.QualityScore = v
			}
		}

		signals = append(signals, signal)
	}

	return signals, nil
}

// upsertSignalBaseline stores or updates a SignalBaseline in the graph.
// Creates the baseline node and HAS_BASELINE relationship from SignalAnchor.
func (s *BackfillService) upsertSignalBaseline(ctx context.Context, baseline SignalBaseline, hasAlert bool, alertThreshold float64) error {
	// Use MERGE for idempotent upsert
	query := `
		MATCH (sig:SignalAnchor {
			metric_name: $metric_name,
			workload_namespace: $workload_namespace,
			workload_name: $workload_name,
			integration: $integration
		})
		MERGE (b:SignalBaseline {
			metric_name: $metric_name,
			workload_namespace: $workload_namespace,
			workload_name: $workload_name,
			integration: $integration
		})
		ON CREATE SET
			b.mean = $mean,
			b.std_dev = $std_dev,
			b.median = $median,
			b.p50 = $p50,
			b.p90 = $p90,
			b.p99 = $p99,
			b.min = $min,
			b.max = $max,
			b.sample_count = $sample_count,
			b.window_start = $window_start,
			b.window_end = $window_end,
			b.last_updated = $last_updated,
			b.expires_at = $expires_at,
			b.has_alert = $has_alert,
			b.alert_threshold = $alert_threshold
		ON MATCH SET
			b.mean = $mean,
			b.std_dev = $std_dev,
			b.median = $median,
			b.p50 = $p50,
			b.p90 = $p90,
			b.p99 = $p99,
			b.min = $min,
			b.max = $max,
			b.sample_count = $sample_count,
			b.window_start = $window_start,
			b.window_end = $window_end,
			b.last_updated = $last_updated,
			b.expires_at = $expires_at,
			b.has_alert = $has_alert,
			b.alert_threshold = $alert_threshold
		MERGE (sig)-[:HAS_BASELINE]->(b)
	`

	_, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"metric_name":        baseline.MetricName,
			"workload_namespace": baseline.WorkloadNamespace,
			"workload_name":      baseline.WorkloadName,
			"integration":        baseline.Integration,
			"mean":               baseline.Mean,
			"std_dev":            baseline.StdDev,
			"median":             baseline.Median,
			"p50":                baseline.P50,
			"p90":                baseline.P90,
			"p99":                baseline.P99,
			"min":                baseline.Min,
			"max":                baseline.Max,
			"sample_count":       baseline.SampleCount,
			"window_start":       baseline.WindowStart,
			"window_end":         baseline.WindowEnd,
			"last_updated":       baseline.LastUpdated,
			"expires_at":         baseline.ExpiresAt,
			"has_alert":          hasAlert,
			"alert_threshold":    alertThreshold,
		},
	})
	if err != nil {
		return fmt.Errorf("upsert baseline to graph: %w", err)
	}

	return nil
}

// Stop releases resources held by the BackfillService.
// Should be called when the service is no longer needed.
func (s *BackfillService) Stop() {
	if s.rateLimiter != nil {
		s.rateLimiter.Stop()
	}
}
