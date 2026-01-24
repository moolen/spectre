package grafana

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// OverviewTool provides high-level metrics overview from overview-level dashboards.
// Executes only the first 5 panels per dashboard for a quick summary.
// Detects anomalies by comparing current metrics to 7-day baseline with severity ranking.
type OverviewTool struct {
	queryService   *GrafanaQueryService
	anomalyService *AnomalyService
	graphClient    graph.Client
	logger         *logging.Logger
}

// NewOverviewTool creates a new overview tool.
// anomalyService may be nil for backward compatibility (tool still works without anomaly detection).
func NewOverviewTool(qs *GrafanaQueryService, as *AnomalyService, gc graph.Client, logger *logging.Logger) *OverviewTool {
	return &OverviewTool{
		queryService:   qs,
		anomalyService: as,
		graphClient:    gc,
		logger:         logger,
	}
}

// OverviewParams defines input parameters for overview tool.
type OverviewParams struct {
	From    string `json:"from"`    // ISO8601: "2026-01-23T10:00:00Z"
	To      string `json:"to"`      // ISO8601: "2026-01-23T11:00:00Z"
	Cluster string `json:"cluster"` // Required: cluster name for scoping
	Region  string `json:"region"`  // Required: region name for scoping
}

// OverviewResponse contains the results from overview dashboards with optional anomaly detection.
type OverviewResponse struct {
	Dashboards []DashboardQueryResult `json:"dashboards,omitempty"`
	TimeRange  string                 `json:"time_range"`
	Anomalies  []MetricAnomaly        `json:"anomalies,omitempty"`
	Summary    *AnomalySummary        `json:"summary,omitempty"`
}

// AnomalySummary provides summary statistics for anomaly detection.
type AnomalySummary struct {
	MetricsChecked  int `json:"metrics_checked"`
	AnomaliesFound  int `json:"anomalies_found"`
	MetricsSkipped  int `json:"metrics_skipped"`
}

// Execute runs the overview tool.
func (t *OverviewTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
	var params OverviewParams
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Validate time range
	timeRange := TimeRange{From: params.From, To: params.To}
	if err := timeRange.Validate(); err != nil {
		return nil, fmt.Errorf("invalid time range: %w", err)
	}

	// Validate required scoping parameters
	if params.Cluster == "" {
		return nil, fmt.Errorf("cluster is required")
	}
	if params.Region == "" {
		return nil, fmt.Errorf("region is required")
	}

	// Build scoping variables
	scopedVars := map[string]string{
		"cluster": params.Cluster,
		"region":  params.Region,
	}

	// Find overview-level dashboards from graph
	dashboards, err := t.findDashboardsByHierarchy(ctx, "overview")
	if err != nil {
		return nil, fmt.Errorf("find overview dashboards: %w", err)
	}

	// Empty success when no dashboards match
	if len(dashboards) == 0 {
		return &OverviewResponse{
			Dashboards: []DashboardQueryResult{},
			TimeRange:  timeRange.FormatDisplay(),
		}, nil
	}

	// Execute dashboards with maxPanels=5 (overview limit)
	results := make([]DashboardQueryResult, 0)
	for _, dash := range dashboards {
		result, err := t.queryService.ExecuteDashboard(
			ctx, dash.UID, timeRange, scopedVars, 5,
		)
		if err != nil {
			t.logger.Warn("Dashboard %s query failed: %v", dash.UID, err)
			continue
		}
		results = append(results, *result)
	}

	// Initialize response with dashboard results
	response := &OverviewResponse{
		Dashboards: results,
		TimeRange:  timeRange.FormatDisplay(),
	}

	// Run anomaly detection if service is available
	if t.anomalyService != nil && len(dashboards) > 0 {
		// Run anomaly detection on first dashboard (typically the primary overview dashboard)
		anomalyResult, err := t.anomalyService.DetectAnomalies(
			ctx, dashboards[0].UID, timeRange, scopedVars,
		)
		if err != nil {
			// Graceful degradation - log warning but continue with non-anomaly response
			t.logger.Warn("Anomaly detection failed: %v", err)
		} else {
			// Format anomalies with minimal context
			response.Anomalies = formatAnomaliesMinimal(anomalyResult.Anomalies)
			response.Summary = &AnomalySummary{
				MetricsChecked: anomalyResult.MetricsChecked,
				AnomaliesFound: len(anomalyResult.Anomalies),
				MetricsSkipped: anomalyResult.SkipCount,
			}

			// When anomalies are detected, omit dashboard results for minimal context
			if len(response.Anomalies) > 0 {
				response.Dashboards = nil
			}
		}
	}

	return response, nil
}

// dashboardInfo holds minimal dashboard information.
type dashboardInfo struct {
	UID   string
	Title string
}

// findDashboardsByHierarchy finds dashboards by hierarchy level from the graph.
func (t *OverviewTool) findDashboardsByHierarchy(ctx context.Context, level string) ([]dashboardInfo, error) {
	query := `
		MATCH (d:Dashboard {hierarchy_level: $level})
		RETURN d.uid AS uid, d.title AS title
		ORDER BY d.title
	`

	result, err := t.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"level": level,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("graph query: %w", err)
	}

	// Find column indices
	uidIdx := -1
	titleIdx := -1
	for i, col := range result.Columns {
		if col == "uid" {
			uidIdx = i
		}
		if col == "title" {
			titleIdx = i
		}
	}

	dashboards := make([]dashboardInfo, 0)
	for _, row := range result.Rows {
		var uid, title string
		if uidIdx >= 0 && uidIdx < len(row) {
			uid, _ = row[uidIdx].(string)
		}
		if titleIdx >= 0 && titleIdx < len(row) {
			title, _ = row[titleIdx].(string)
		}
		if uid != "" {
			dashboards = append(dashboards, dashboardInfo{UID: uid, Title: title})
		}
	}

	return dashboards, nil
}

// formatAnomaliesMinimal formats anomalies with minimal context (no timestamp, no panel info)
// Returns only: metric name, current value, baseline, z-score, severity
func formatAnomaliesMinimal(anomalies []MetricAnomaly) []MetricAnomaly {
	// MetricAnomaly already has the minimal fields we need
	// Just strip the timestamp field by creating new slice
	minimal := make([]MetricAnomaly, len(anomalies))
	for i, a := range anomalies {
		minimal[i] = MetricAnomaly{
			MetricName: a.MetricName,
			Value:      a.Value,
			Baseline:   a.Baseline,
			ZScore:     a.ZScore,
			Severity:   a.Severity,
			// Timestamp intentionally omitted for minimal context
		}
	}
	return minimal
}
