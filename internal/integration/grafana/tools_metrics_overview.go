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
type OverviewTool struct {
	queryService *GrafanaQueryService
	graphClient  graph.Client
	logger       *logging.Logger
}

// NewOverviewTool creates a new overview tool.
func NewOverviewTool(qs *GrafanaQueryService, gc graph.Client, logger *logging.Logger) *OverviewTool {
	return &OverviewTool{
		queryService: qs,
		graphClient:  gc,
		logger:       logger,
	}
}

// OverviewParams defines input parameters for overview tool.
type OverviewParams struct {
	From    string `json:"from"`    // ISO8601: "2026-01-23T10:00:00Z"
	To      string `json:"to"`      // ISO8601: "2026-01-23T11:00:00Z"
	Cluster string `json:"cluster"` // Required: cluster name for scoping
	Region  string `json:"region"`  // Required: region name for scoping
}

// OverviewResponse contains the results from overview dashboards.
type OverviewResponse struct {
	Dashboards []DashboardQueryResult `json:"dashboards"`
	TimeRange  string                 `json:"time_range"`
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

	return &OverviewResponse{
		Dashboards: results,
		TimeRange:  timeRange.FormatDisplay(),
	}, nil
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
