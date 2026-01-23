package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// TimeRange represents an absolute time range for queries.
type TimeRange struct {
	From string `json:"from"` // ISO8601: "2026-01-23T10:00:00Z"
	To   string `json:"to"`   // ISO8601: "2026-01-23T11:00:00Z"
}

// Validate checks that the time range is valid.
// Returns an error if timestamps are malformed or if to <= from.
func (tr TimeRange) Validate() error {
	fromTime, err := time.Parse(time.RFC3339, tr.From)
	if err != nil {
		return fmt.Errorf("invalid from timestamp (expected ISO8601): %w", err)
	}
	toTime, err := time.Parse(time.RFC3339, tr.To)
	if err != nil {
		return fmt.Errorf("invalid to timestamp (expected ISO8601): %w", err)
	}
	if !toTime.After(fromTime) {
		return fmt.Errorf("to must be after from (got from=%s, to=%s)", tr.From, tr.To)
	}
	duration := toTime.Sub(fromTime)
	if duration > 7*24*time.Hour {
		return fmt.Errorf("time range too large (max 7 days, got %s)", duration)
	}
	return nil
}

// ToGrafanaRequest converts the time range to Grafana API format (epoch milliseconds as strings).
func (tr TimeRange) ToGrafanaRequest() (string, string) {
	fromTime, _ := time.Parse(time.RFC3339, tr.From)
	toTime, _ := time.Parse(time.RFC3339, tr.To)
	return fmt.Sprintf("%d", fromTime.UnixMilli()), fmt.Sprintf("%d", toTime.UnixMilli())
}

// FormatDisplay returns a human-readable time range string.
func (tr TimeRange) FormatDisplay() string {
	return fmt.Sprintf("%s to %s", tr.From, tr.To)
}

// GrafanaQueryService executes Grafana dashboard queries.
// It fetches dashboard structure from the graph and executes PromQL queries via Grafana API.
type GrafanaQueryService struct {
	grafanaClient *GrafanaClient
	graphClient   graph.Client
	logger        *logging.Logger
}

// NewGrafanaQueryService creates a new query service.
func NewGrafanaQueryService(client *GrafanaClient, graphClient graph.Client, logger *logging.Logger) *GrafanaQueryService {
	return &GrafanaQueryService{
		grafanaClient: client,
		graphClient:   graphClient,
		logger:        logger,
	}
}

// dashboardPanel represents a panel extracted from dashboard JSON.
type dashboardPanel struct {
	ID            int
	Title         string
	Type          string
	DatasourceUID string
	Targets       []panelTarget
}

// panelTarget represents a query target within a panel.
type panelTarget struct {
	RefID string
	Expr  string
}

// ExecuteDashboard executes queries for a dashboard and returns formatted results.
// dashboardUID: the dashboard's UID
// timeRange: the time range for queries
// scopedVars: variables for server-side substitution (cluster, region, etc.)
// maxPanels: limit number of panels (0 = all panels)
// Returns partial results when some panels fail.
func (s *GrafanaQueryService) ExecuteDashboard(
	ctx context.Context,
	dashboardUID string,
	timeRange TimeRange,
	scopedVars map[string]string,
	maxPanels int,
) (*DashboardQueryResult, error) {
	// Fetch dashboard from graph
	dashboardJSON, title, err := s.fetchDashboardFromGraph(ctx, dashboardUID)
	if err != nil {
		return nil, fmt.Errorf("fetch dashboard %s: %w", dashboardUID, err)
	}

	// Parse panels from dashboard JSON
	panels, err := s.extractPanels(dashboardJSON)
	if err != nil {
		return nil, fmt.Errorf("extract panels from dashboard %s: %w", dashboardUID, err)
	}

	// Filter panels if maxPanels > 0
	if maxPanels > 0 && len(panels) > maxPanels {
		panels = panels[:maxPanels]
	}

	// Initialize result
	result := &DashboardQueryResult{
		DashboardUID:   dashboardUID,
		DashboardTitle: title,
		Panels:         make([]PanelResult, 0),
		Errors:         make([]PanelError, 0),
		TimeRange:      timeRange.FormatDisplay(),
	}

	// Convert scopedVars to Grafana format
	grafanaScopedVars := make(map[string]ScopedVar)
	for k, v := range scopedVars {
		grafanaScopedVars[k] = ScopedVar{Text: v, Value: v}
	}

	// Convert time range to Grafana format
	from, to := timeRange.ToGrafanaRequest()

	// Execute queries for each panel
	for _, panel := range panels {
		panelResult, err := s.executePanel(ctx, panel, from, to, grafanaScopedVars)
		if err != nil {
			// Partial results pattern - collect errors, don't fail entire request
			for _, target := range panel.Targets {
				result.Errors = append(result.Errors, PanelError{
					PanelID:    panel.ID,
					PanelTitle: panel.Title,
					Query:      target.Expr,
					Error:      err.Error(),
				})
			}
			s.logger.Debug("Panel %d (%s) query failed: %v", panel.ID, panel.Title, err)
			continue
		}

		// Omit panels with no data
		if len(panelResult.Metrics) == 0 {
			continue
		}

		result.Panels = append(result.Panels, *panelResult)
	}

	return result, nil
}

// fetchDashboardFromGraph retrieves dashboard JSON and title from the graph.
func (s *GrafanaQueryService) fetchDashboardFromGraph(ctx context.Context, uid string) (map[string]interface{}, string, error) {
	query := `MATCH (d:Dashboard {uid: $uid}) RETURN d.json AS json, d.title AS title`

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"uid": uid,
		},
	})
	if err != nil {
		return nil, "", fmt.Errorf("graph query: %w", err)
	}

	if len(result.Rows) == 0 {
		return nil, "", fmt.Errorf("dashboard %s not found in graph", uid)
	}

	row := result.Rows[0]

	// Find column indices
	jsonIdx := -1
	titleIdx := -1
	for i, col := range result.Columns {
		if col == "json" {
			jsonIdx = i
		}
		if col == "title" {
			titleIdx = i
		}
	}

	// Extract title
	var title string
	if titleIdx >= 0 && titleIdx < len(row) {
		title, _ = row[titleIdx].(string)
	}

	// Parse JSON
	if jsonIdx < 0 || jsonIdx >= len(row) {
		return nil, "", fmt.Errorf("dashboard JSON not found")
	}
	jsonStr, ok := row[jsonIdx].(string)
	if !ok {
		return nil, "", fmt.Errorf("dashboard JSON not found")
	}

	var dashboardJSON map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &dashboardJSON); err != nil {
		return nil, "", fmt.Errorf("parse dashboard JSON: %w", err)
	}

	return dashboardJSON, title, nil
}

// extractPanels parses dashboard JSON and extracts panels with queries.
func (s *GrafanaQueryService) extractPanels(dashboardJSON map[string]interface{}) ([]dashboardPanel, error) {
	panels := make([]dashboardPanel, 0)

	// Get panels array from dashboard
	panelsRaw, ok := dashboardJSON["panels"].([]interface{})
	if !ok {
		return panels, nil // No panels
	}

	for _, p := range panelsRaw {
		panelMap, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		panel := s.extractPanelInfo(panelMap)
		if panel != nil && len(panel.Targets) > 0 {
			panels = append(panels, *panel)
		}

		// Handle nested panels (rows with collapsed panels)
		if nestedPanels, ok := panelMap["panels"].([]interface{}); ok {
			for _, np := range nestedPanels {
				nestedMap, ok := np.(map[string]interface{})
				if !ok {
					continue
				}
				nestedPanel := s.extractPanelInfo(nestedMap)
				if nestedPanel != nil && len(nestedPanel.Targets) > 0 {
					panels = append(panels, *nestedPanel)
				}
			}
		}
	}

	return panels, nil
}

// extractPanelInfo extracts panel information from a panel map.
func (s *GrafanaQueryService) extractPanelInfo(panelMap map[string]interface{}) *dashboardPanel {
	// Skip non-graph/stat panels (text, row, etc.)
	panelType, _ := panelMap["type"].(string)
	if panelType == "text" || panelType == "row" {
		return nil
	}

	panel := &dashboardPanel{
		Type:    panelType,
		Targets: make([]panelTarget, 0),
	}

	// Extract ID
	if id, ok := panelMap["id"].(float64); ok {
		panel.ID = int(id)
	}

	// Extract title
	if title, ok := panelMap["title"].(string); ok {
		panel.Title = title
	}

	// Extract datasource UID
	if ds, ok := panelMap["datasource"].(map[string]interface{}); ok {
		if uid, ok := ds["uid"].(string); ok {
			panel.DatasourceUID = uid
		}
	}

	// Extract targets (queries)
	if targets, ok := panelMap["targets"].([]interface{}); ok {
		for _, t := range targets {
			targetMap, ok := t.(map[string]interface{})
			if !ok {
				continue
			}

			target := panelTarget{}

			// Extract refId
			if refID, ok := targetMap["refId"].(string); ok {
				target.RefID = refID
			}

			// Extract expr (PromQL)
			if expr, ok := targetMap["expr"].(string); ok && expr != "" {
				target.Expr = expr
				panel.Targets = append(panel.Targets, target)
			}
		}
	}

	if len(panel.Targets) == 0 {
		return nil
	}

	return panel
}

// executePanel executes queries for a single panel.
func (s *GrafanaQueryService) executePanel(
	ctx context.Context,
	panel dashboardPanel,
	from, to string,
	scopedVars map[string]ScopedVar,
) (*PanelResult, error) {
	if len(panel.Targets) == 0 {
		return nil, fmt.Errorf("panel has no targets")
	}

	if panel.DatasourceUID == "" {
		return nil, fmt.Errorf("panel has no datasource UID")
	}

	// Execute the first target (most panels have single target)
	// TODO: Support multiple targets per panel if needed
	target := panel.Targets[0]

	response, err := s.grafanaClient.QueryDataSource(
		ctx,
		panel.DatasourceUID,
		target.Expr,
		from,
		to,
		scopedVars,
	)
	if err != nil {
		return nil, err
	}

	// Check for query-level errors in response
	for _, result := range response.Results {
		if result.Error != "" {
			return nil, fmt.Errorf("query error: %s", result.Error)
		}
	}

	// Format response
	return formatTimeSeriesResponse(panel.ID, panel.Title, target.Expr, response), nil
}
