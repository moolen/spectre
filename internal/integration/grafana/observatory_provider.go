package grafana

import (
	"context"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/observatory"
)

// GrafanaObservatoryProvider implements observatory.Provider for Grafana integration.
// It adapts Grafana's graph-based signal storage to the Observatory interface.
type GrafanaObservatoryProvider struct {
	graphClient     graph.Client
	integrationName string
	logger          *logging.Logger
}

// NewGrafanaObservatoryProvider creates a new Grafana provider for Observatory.
func NewGrafanaObservatoryProvider(
	graphClient graph.Client,
	integrationName string,
	logger *logging.Logger,
) *GrafanaObservatoryProvider {
	return &GrafanaObservatoryProvider{
		graphClient:     graphClient,
		integrationName: integrationName,
		logger:          logger,
	}
}

// Name returns the unique identifier for this provider.
func (p *GrafanaObservatoryProvider) Name() string {
	return p.integrationName
}

// ListSignalAnchors returns all active SignalAnchors from this provider.
// Queries the graph for non-expired SignalAnchor nodes and converts them
// to observatory.SignalAnchor format.
func (p *GrafanaObservatoryProvider) ListSignalAnchors(
	ctx context.Context,
	opts observatory.SignalListOptions,
) ([]observatory.SignalAnchor, error) {
	// Build query with optional filters
	query, params := p.buildSignalListQuery(opts)

	result, err := p.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query:      query,
		Parameters: params,
	})
	if err != nil {
		return nil, err
	}

	// Map column names to indices
	colIdx := make(map[string]int)
	for i, col := range result.Columns {
		colIdx[col] = i
	}

	signals := make([]observatory.SignalAnchor, 0, len(result.Rows))
	for _, row := range result.Rows {
		signal := p.parseSignalAnchorRow(colIdx, row)
		if signal != nil {
			signals = append(signals, *signal)
		}
	}

	return signals, nil
}

// buildSignalListQuery constructs the Cypher query for listing signals.
func (p *GrafanaObservatoryProvider) buildSignalListQuery(
	opts observatory.SignalListOptions,
) (string, map[string]any) {
	now := time.Now().Unix()

	params := map[string]any{
		"integration": p.integrationName,
		"now":         now,
	}

	// Build WHERE clauses
	whereClause := "WHERE s.expires_at > $now"

	if opts.Namespace != "" {
		whereClause += " AND s.workload_namespace = $namespace"
		params["namespace"] = opts.Namespace
	}

	if opts.WorkloadName != "" {
		whereClause += " AND s.workload_name = $workload_name"
		params["workload_name"] = opts.WorkloadName
	}

	if opts.Role != "" {
		whereClause += " AND s.role = $role"
		params["role"] = string(opts.Role)
	}

	query := `
		MATCH (s:SignalAnchor {integration: $integration})
		` + whereClause + `
		RETURN
			s.metric_name AS metric_name,
			s.workload_namespace AS workload_namespace,
			s.workload_name AS workload_name,
			s.role AS role,
			s.confidence AS confidence,
			s.quality_score AS quality_score,
			s.dashboard_uid AS dashboard_uid,
			s.panel_id AS panel_id,
			s.first_seen AS first_seen,
			s.last_seen AS last_seen,
			s.expires_at AS expires_at
	`

	return query, params
}

// parseSignalAnchorRow converts a graph result row to observatory.SignalAnchor.
func (p *GrafanaObservatoryProvider) parseSignalAnchorRow(
	colIdx map[string]int,
	row []any,
) *observatory.SignalAnchor {
	if len(row) == 0 {
		return nil
	}

	signal := &observatory.SignalAnchor{
		SourceProvider: p.integrationName,
	}

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

	// Parse classification fields
	if idx, ok := colIdx["role"]; ok && idx < len(row) {
		if v, ok := row[idx].(string); ok {
			signal.Role = observatory.SignalRole(v)
		}
	}
	if idx, ok := colIdx["confidence"]; ok && idx < len(row) {
		signal.Confidence = parseFloat64(row[idx])
	}
	if idx, ok := colIdx["quality_score"]; ok && idx < len(row) {
		signal.QualityScore = parseFloat64(row[idx])
	}

	// Parse source reference (dashboard UID)
	if idx, ok := colIdx["dashboard_uid"]; ok && idx < len(row) {
		if v, ok := row[idx].(string); ok {
			signal.SourceRef = v
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

	return signal
}

// GetCurrentValue fetches the current value of a metric for anomaly scoring.
// Currently returns not found (uses baseline mean as fallback).
// Future: Query Prometheus/Grafana for live values.
func (p *GrafanaObservatoryProvider) GetCurrentValue(
	ctx context.Context,
	metricName, namespace, workload string,
) (float64, bool, error) {
	// For now, return not found to use baseline mean fallback.
	// Full implementation would query Prometheus via GrafanaQueryService.
	return 0, false, nil
}

// GetBaseline retrieves the baseline statistics for a signal.
// Returns nil if no baseline exists (cold start condition).
func (p *GrafanaObservatoryProvider) GetBaseline(
	ctx context.Context,
	metricName, namespace, workload string,
) (*observatory.SignalBaseline, error) {
	// Use existing graph query function
	grafanaBaseline, err := GetSignalBaseline(
		ctx,
		p.graphClient,
		metricName,
		namespace,
		workload,
		p.integrationName,
	)
	if err != nil {
		return nil, err
	}

	if grafanaBaseline == nil {
		return nil, nil // Cold start
	}

	// Convert to observatory.SignalBaseline
	return &observatory.SignalBaseline{
		MetricName:        grafanaBaseline.MetricName,
		WorkloadNamespace: grafanaBaseline.WorkloadNamespace,
		WorkloadName:      grafanaBaseline.WorkloadName,
		SourceProvider:    p.integrationName,
		Mean:              grafanaBaseline.Mean,
		StdDev:            grafanaBaseline.StdDev,
		Median:            grafanaBaseline.Median,
		P50:               grafanaBaseline.P50,
		P90:               grafanaBaseline.P90,
		P99:               grafanaBaseline.P99,
		Min:               grafanaBaseline.Min,
		Max:               grafanaBaseline.Max,
		SampleCount:       grafanaBaseline.SampleCount,
		WindowStart:       grafanaBaseline.WindowStart,
		WindowEnd:         grafanaBaseline.WindowEnd,
		LastUpdated:       grafanaBaseline.LastUpdated,
		ExpiresAt:         grafanaBaseline.ExpiresAt,
	}, nil
}

// GetAlertState returns the current alert state for a signal.
// Queries the graph for alerts monitoring this metric in this workload.
func (p *GrafanaObservatoryProvider) GetAlertState(
	ctx context.Context,
	metricName, namespace, workload string,
) (string, error) {
	// Query for alert state via graph relationships
	// Alert -> MONITORS -> Metric
	// SignalAnchor has the metric name and workload info
	// We need to find if any alert is linked to this metric and is firing
	query := `
		MATCH (a:Alert {integration: $integration})-[:MONITORS]->(m:Metric {name: $metric_name})
		WHERE EXISTS {
			MATCH (s:SignalAnchor {
				metric_name: $metric_name,
				workload_namespace: $namespace,
				workload_name: $workload_name,
				integration: $integration
			})
		}
		OPTIONAL MATCH (a)-[t:STATE_TRANSITION]->(a)
		WITH a, t
		ORDER BY t.timestamp DESC
		LIMIT 1
		RETURN COALESCE(t.to_state, 'normal') AS state
	`

	result, err := p.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]any{
			"integration":   p.integrationName,
			"metric_name":   metricName,
			"namespace":     namespace,
			"workload_name": workload,
		},
	})
	if err != nil {
		// Log error but return empty state (graceful degradation)
		p.logger.Debug("Failed to query alert state for %s: %v", metricName, err)
		return "", nil
	}

	if len(result.Rows) == 0 || len(result.Rows[0]) == 0 {
		return "", nil // No alert associated
	}

	state, ok := result.Rows[0][0].(string)
	if !ok {
		return "", nil
	}

	return state, nil
}

// Ensure GrafanaObservatoryProvider implements observatory.Provider
var _ observatory.Provider = (*GrafanaObservatoryProvider)(nil)
