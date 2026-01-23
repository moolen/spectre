package grafana

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// DetailsTool provides detailed metrics from detail-level dashboards.
// Executes all panels in detail dashboards.
type DetailsTool struct {
	queryService *GrafanaQueryService
	graphClient  graph.Client
	logger       *logging.Logger
}

// NewDetailsTool creates a new details tool.
func NewDetailsTool(qs *GrafanaQueryService, gc graph.Client, logger *logging.Logger) *DetailsTool {
	return &DetailsTool{
		queryService: qs,
		graphClient:  gc,
		logger:       logger,
	}
}

// DetailsParams defines input parameters for details tool.
type DetailsParams struct {
	From    string `json:"from"`    // ISO8601: "2026-01-23T10:00:00Z"
	To      string `json:"to"`      // ISO8601: "2026-01-23T11:00:00Z"
	Cluster string `json:"cluster"` // Required: cluster name for scoping
	Region  string `json:"region"`  // Required: region name for scoping
}

// DetailsResponse contains the results from detail dashboards.
type DetailsResponse struct {
	Dashboards []DashboardQueryResult `json:"dashboards"`
	TimeRange  string                 `json:"time_range"`
}

// Execute runs the details tool.
func (t *DetailsTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
	var params DetailsParams
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

	// Find detail-level dashboards from graph
	dashboards, err := t.findDashboardsByHierarchy(ctx, "detail")
	if err != nil {
		return nil, fmt.Errorf("find detail dashboards: %w", err)
	}

	// Empty success when no dashboards match
	if len(dashboards) == 0 {
		return &DetailsResponse{
			Dashboards: []DashboardQueryResult{},
			TimeRange:  timeRange.FormatDisplay(),
		}, nil
	}

	// Execute all panels in detail dashboards (maxPanels=0)
	results := make([]DashboardQueryResult, 0)
	for _, dash := range dashboards {
		result, err := t.queryService.ExecuteDashboard(
			ctx, dash.UID, timeRange, scopedVars, 0,
		)
		if err != nil {
			t.logger.Warn("Dashboard %s query failed: %v", dash.UID, err)
			continue
		}
		results = append(results, *result)
	}

	return &DetailsResponse{
		Dashboards: results,
		TimeRange:  timeRange.FormatDisplay(),
	}, nil
}

// findDashboardsByHierarchy finds dashboards by hierarchy level from the graph.
func (t *DetailsTool) findDashboardsByHierarchy(ctx context.Context, level string) ([]dashboardInfo, error) {
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
