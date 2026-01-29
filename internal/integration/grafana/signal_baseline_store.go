package grafana

import (
	"context"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/graph"
)

// UpsertSignalBaseline creates or updates a SignalBaseline node in FalkorDB.
// Uses MERGE with composite key: metric_name + workload_namespace + workload_name + integration.
//
// ON CREATE: Sets all fields including timestamps
// ON MATCH: Updates statistics fields, last_updated, expires_at (preserves first created timestamp)
//
// Also creates HAS_BASELINE relationship from SignalAnchor to SignalBaseline.
func UpsertSignalBaseline(ctx context.Context, graphClient graph.Client, baseline SignalBaseline) error {
	// MERGE SignalBaseline with composite key matching SignalAnchor
	// ON CREATE sets all fields
	// ON MATCH updates statistics but preserves identity
	query := `
		MERGE (b:SignalBaseline {
			metric_name: $metric_name,
			workload_namespace: $workload_namespace,
			workload_name: $workload_name,
			integration: $integration
		})
		ON CREATE SET
			b.mean = $mean,
			b.stddev = $stddev,
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
			b.expires_at = $expires_at
		ON MATCH SET
			b.mean = $mean,
			b.stddev = $stddev,
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
			b.expires_at = $expires_at
		WITH b
		MATCH (s:SignalAnchor {
			metric_name: $metric_name,
			workload_namespace: $workload_namespace,
			workload_name: $workload_name,
			integration: $integration
		})
		MERGE (s)-[:HAS_BASELINE]->(b)
	`

	_, err := graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"metric_name":         baseline.MetricName,
			"workload_namespace":  baseline.WorkloadNamespace,
			"workload_name":       baseline.WorkloadName,
			"integration":         baseline.Integration,
			"mean":                baseline.Mean,
			"stddev":              baseline.StdDev,
			"median":              baseline.Median,
			"p50":                 baseline.P50,
			"p90":                 baseline.P90,
			"p99":                 baseline.P99,
			"min":                 baseline.Min,
			"max":                 baseline.Max,
			"sample_count":        baseline.SampleCount,
			"window_start":        baseline.WindowStart,
			"window_end":          baseline.WindowEnd,
			"last_updated":        baseline.LastUpdated,
			"expires_at":          baseline.ExpiresAt,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to upsert signal baseline: %w", err)
	}

	return nil
}

// GetSignalBaseline retrieves a SignalBaseline by composite key.
// Returns nil, nil if not found (not an error).
func GetSignalBaseline(
	ctx context.Context,
	graphClient graph.Client,
	metricName, namespace, workloadName, integration string,
) (*SignalBaseline, error) {
	query := `
		MATCH (b:SignalBaseline {
			metric_name: $metric_name,
			workload_namespace: $workload_namespace,
			workload_name: $workload_name,
			integration: $integration
		})
		RETURN
			b.metric_name AS metric_name,
			b.workload_namespace AS workload_namespace,
			b.workload_name AS workload_name,
			b.integration AS integration,
			b.mean AS mean,
			b.stddev AS stddev,
			b.median AS median,
			b.p50 AS p50,
			b.p90 AS p90,
			b.p99 AS p99,
			b.min AS min,
			b.max AS max,
			b.sample_count AS sample_count,
			b.window_start AS window_start,
			b.window_end AS window_end,
			b.last_updated AS last_updated,
			b.expires_at AS expires_at
	`

	result, err := graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"metric_name":        metricName,
			"workload_namespace": namespace,
			"workload_name":      workloadName,
			"integration":        integration,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query signal baseline: %w", err)
	}

	// Not found - return nil, nil (not an error)
	if len(result.Rows) == 0 {
		return nil, nil
	}

	// Parse result row to SignalBaseline
	return parseSignalBaselineRow(result.Columns, result.Rows[0])
}

// GetBaselinesByWorkload retrieves all SignalBaselines for a workload.
// Filters by expires_at > now for TTL enforcement.
// Returns empty slice if none found.
func GetBaselinesByWorkload(
	ctx context.Context,
	graphClient graph.Client,
	namespace, workloadName, integration string,
) ([]SignalBaseline, error) {
	now := time.Now().Unix()

	query := `
		MATCH (b:SignalBaseline {
			workload_namespace: $workload_namespace,
			workload_name: $workload_name,
			integration: $integration
		})
		WHERE b.expires_at > $now
		RETURN
			b.metric_name AS metric_name,
			b.workload_namespace AS workload_namespace,
			b.workload_name AS workload_name,
			b.integration AS integration,
			b.mean AS mean,
			b.stddev AS stddev,
			b.median AS median,
			b.p50 AS p50,
			b.p90 AS p90,
			b.p99 AS p99,
			b.min AS min,
			b.max AS max,
			b.sample_count AS sample_count,
			b.window_start AS window_start,
			b.window_end AS window_end,
			b.last_updated AS last_updated,
			b.expires_at AS expires_at
	`

	result, err := graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"workload_namespace": namespace,
			"workload_name":      workloadName,
			"integration":        integration,
			"now":                now,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query baselines by workload: %w", err)
	}

	baselines := make([]SignalBaseline, 0, len(result.Rows))
	for _, row := range result.Rows {
		baseline, err := parseSignalBaselineRow(result.Columns, row)
		if err != nil {
			// Log error but continue with other rows
			continue
		}
		baselines = append(baselines, *baseline)
	}

	return baselines, nil
}

// GetActiveSignalAnchors retrieves all SignalAnchors that have not expired.
// Used by BaselineCollector to find signals needing baseline updates.
func GetActiveSignalAnchors(
	ctx context.Context,
	graphClient graph.Client,
	integration string,
) ([]SignalAnchor, error) {
	now := time.Now().Unix()

	query := `
		MATCH (s:SignalAnchor {integration: $integration})
		WHERE s.expires_at > $now
		RETURN
			s.metric_name AS metric_name,
			s.workload_namespace AS workload_namespace,
			s.workload_name AS workload_name,
			s.integration AS integration,
			s.role AS role,
			s.confidence AS confidence,
			s.quality_score AS quality_score,
			s.dashboard_uid AS dashboard_uid,
			s.panel_id AS panel_id,
			s.query_id AS query_id,
			s.first_seen AS first_seen,
			s.last_seen AS last_seen,
			s.expires_at AS expires_at
	`

	result, err := graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"integration": integration,
			"now":         now,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query active signal anchors: %w", err)
	}

	signals := make([]SignalAnchor, 0, len(result.Rows))
	for _, row := range result.Rows {
		signal, err := parseSignalAnchorRow(result.Columns, row)
		if err != nil {
			// Skip malformed rows
			continue
		}
		signals = append(signals, *signal)
	}

	return signals, nil
}

// parseSignalBaselineRow parses a graph result row into a SignalBaseline.
func parseSignalBaselineRow(columns []string, row []interface{}) (*SignalBaseline, error) {
	if len(row) == 0 {
		return nil, fmt.Errorf("empty row")
	}

	// Build column index map
	colIdx := make(map[string]int)
	for i, col := range columns {
		colIdx[col] = i
	}

	baseline := &SignalBaseline{}

	// Parse identity fields
	if idx, ok := colIdx["metric_name"]; ok && idx < len(row) {
		if v, ok := row[idx].(string); ok {
			baseline.MetricName = v
		}
	}
	if idx, ok := colIdx["workload_namespace"]; ok && idx < len(row) {
		if v, ok := row[idx].(string); ok {
			baseline.WorkloadNamespace = v
		}
	}
	if idx, ok := colIdx["workload_name"]; ok && idx < len(row) {
		if v, ok := row[idx].(string); ok {
			baseline.WorkloadName = v
		}
	}
	if idx, ok := colIdx["integration"]; ok && idx < len(row) {
		if v, ok := row[idx].(string); ok {
			baseline.Integration = v
		}
	}

	// Parse statistics fields
	if idx, ok := colIdx["mean"]; ok && idx < len(row) {
		baseline.Mean = parseFloat64(row[idx])
	}
	if idx, ok := colIdx["stddev"]; ok && idx < len(row) {
		baseline.StdDev = parseFloat64(row[idx])
	}
	if idx, ok := colIdx["median"]; ok && idx < len(row) {
		baseline.Median = parseFloat64(row[idx])
	}
	if idx, ok := colIdx["p50"]; ok && idx < len(row) {
		baseline.P50 = parseFloat64(row[idx])
	}
	if idx, ok := colIdx["p90"]; ok && idx < len(row) {
		baseline.P90 = parseFloat64(row[idx])
	}
	if idx, ok := colIdx["p99"]; ok && idx < len(row) {
		baseline.P99 = parseFloat64(row[idx])
	}
	if idx, ok := colIdx["min"]; ok && idx < len(row) {
		baseline.Min = parseFloat64(row[idx])
	}
	if idx, ok := colIdx["max"]; ok && idx < len(row) {
		baseline.Max = parseFloat64(row[idx])
	}
	if idx, ok := colIdx["sample_count"]; ok && idx < len(row) {
		baseline.SampleCount = parseInt(row[idx])
	}

	// Parse window metadata
	if idx, ok := colIdx["window_start"]; ok && idx < len(row) {
		baseline.WindowStart = parseInt64(row[idx])
	}
	if idx, ok := colIdx["window_end"]; ok && idx < len(row) {
		baseline.WindowEnd = parseInt64(row[idx])
	}

	// Parse TTL fields
	if idx, ok := colIdx["last_updated"]; ok && idx < len(row) {
		baseline.LastUpdated = parseInt64(row[idx])
	}
	if idx, ok := colIdx["expires_at"]; ok && idx < len(row) {
		baseline.ExpiresAt = parseInt64(row[idx])
	}

	return baseline, nil
}

// parseSignalAnchorRow parses a graph result row into a SignalAnchor.
func parseSignalAnchorRow(columns []string, row []interface{}) (*SignalAnchor, error) {
	if len(row) == 0 {
		return nil, fmt.Errorf("empty row")
	}

	// Build column index map
	colIdx := make(map[string]int)
	for i, col := range columns {
		colIdx[col] = i
	}

	signal := &SignalAnchor{}

	// Parse identity fields
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
	if idx, ok := colIdx["integration"]; ok && idx < len(row) {
		if v, ok := row[idx].(string); ok {
			signal.SourceGrafana = v
		}
	}

	// Parse classification fields
	if idx, ok := colIdx["role"]; ok && idx < len(row) {
		if v, ok := row[idx].(string); ok {
			signal.Role = SignalRole(v)
		}
	}
	if idx, ok := colIdx["confidence"]; ok && idx < len(row) {
		signal.Confidence = parseFloat64(row[idx])
	}
	if idx, ok := colIdx["quality_score"]; ok && idx < len(row) {
		signal.QualityScore = parseFloat64(row[idx])
	}

	// Parse source fields
	if idx, ok := colIdx["dashboard_uid"]; ok && idx < len(row) {
		if v, ok := row[idx].(string); ok {
			signal.DashboardUID = v
		}
	}
	if idx, ok := colIdx["panel_id"]; ok && idx < len(row) {
		signal.PanelID = parseInt(row[idx])
	}
	if idx, ok := colIdx["query_id"]; ok && idx < len(row) {
		if v, ok := row[idx].(string); ok {
			signal.QueryID = v
		}
	}

	// Parse timestamp fields
	if idx, ok := colIdx["first_seen"]; ok && idx < len(row) {
		signal.FirstSeen = parseInt64(row[idx])
	}
	if idx, ok := colIdx["last_seen"]; ok && idx < len(row) {
		signal.LastSeen = parseInt64(row[idx])
	}
	if idx, ok := colIdx["expires_at"]; ok && idx < len(row) {
		signal.ExpiresAt = parseInt64(row[idx])
	}

	return signal, nil
}

// parseFloat64 safely extracts a float64 from an interface value.
func parseFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	case int:
		return float64(val)
	default:
		return 0
	}
}

// parseInt safely extracts an int from an interface value.
func parseInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	default:
		return 0
	}
}

// parseInt64 safely extracts an int64 from an interface value.
func parseInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case float64:
		return int64(val)
	default:
		return 0
	}
}
