package grafana

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// AggregatedTool provides aggregated metrics for a specific service or namespace.
// Executes drill-down level dashboards with all panels.
type AggregatedTool struct {
	queryService *GrafanaQueryService
	graphClient  graph.Client
	logger       *logging.Logger
}

// NewAggregatedTool creates a new aggregated tool.
func NewAggregatedTool(qs *GrafanaQueryService, gc graph.Client, logger *logging.Logger) *AggregatedTool {
	return &AggregatedTool{
		queryService: qs,
		graphClient:  gc,
		logger:       logger,
	}
}

// AggregatedParams defines input parameters for aggregated tool.
type AggregatedParams struct {
	From      string `json:"from"`                // ISO8601: "2026-01-23T10:00:00Z"
	To        string `json:"to"`                  // ISO8601: "2026-01-23T11:00:00Z"
	Cluster   string `json:"cluster"`             // Required: cluster name for scoping
	Region    string `json:"region"`              // Required: region name for scoping
	Service   string `json:"service,omitempty"`   // Optional: service name (requires service OR namespace)
	Namespace string `json:"namespace,omitempty"` // Optional: namespace name (requires service OR namespace)
}

// AggregatedResponse contains the results from drill-down dashboards.
type AggregatedResponse struct {
	Dashboards []DashboardQueryResult `json:"dashboards"`
	Service    string                 `json:"service,omitempty"`
	Namespace  string                 `json:"namespace,omitempty"`
	TimeRange  string                 `json:"time_range"`
}

// Execute runs the aggregated tool.
func (t *AggregatedTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
	var params AggregatedParams
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

	// Require service OR namespace
	if params.Service == "" && params.Namespace == "" {
		return nil, fmt.Errorf("either service or namespace must be specified")
	}

	// Build scoping variables (include service/namespace)
	scopedVars := map[string]string{
		"cluster": params.Cluster,
		"region":  params.Region,
	}
	if params.Service != "" {
		scopedVars["service"] = params.Service
	}
	if params.Namespace != "" {
		scopedVars["namespace"] = params.Namespace
	}

	// Find drill-down level dashboards from graph
	dashboards, err := t.findDashboardsByHierarchy(ctx, "drilldown")
	if err != nil {
		return nil, fmt.Errorf("find drill-down dashboards: %w", err)
	}

	// Empty success when no dashboards match
	if len(dashboards) == 0 {
		return &AggregatedResponse{
			Dashboards: []DashboardQueryResult{},
			Service:    params.Service,
			Namespace:  params.Namespace,
			TimeRange:  timeRange.FormatDisplay(),
		}, nil
	}

	// Execute all panels in drill-down dashboards (maxPanels=0)
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

	return &AggregatedResponse{
		Dashboards: results,
		Service:    params.Service,
		Namespace:  params.Namespace,
		TimeRange:  timeRange.FormatDisplay(),
	}, nil
}

// findDashboardsByHierarchy finds dashboards by hierarchy level from the graph.
func (t *AggregatedTool) findDashboardsByHierarchy(ctx context.Context, level string) ([]dashboardInfo, error) {
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
