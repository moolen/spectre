package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
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

	// Cached Prometheus datasource UID for fallback resolution
	promDatasourceMu  sync.Mutex
	promDatasourceUID string
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
	panels, err := s.extractPanels(ctx, dashboardJSON)
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

// fetchDashboardFromGraph retrieves dashboard JSON and title.
// First tries graph cache, then falls back to Grafana API.
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

	// Try to get JSON from graph first
	var dashboardJSON map[string]interface{}
	if jsonIdx >= 0 && jsonIdx < len(row) {
		if jsonStr, ok := row[jsonIdx].(string); ok && jsonStr != "" {
			if err := json.Unmarshal([]byte(jsonStr), &dashboardJSON); err == nil {
				return dashboardJSON, title, nil
			}
		}
	}

	// Fallback: fetch from Grafana API
	s.logger.Debug("Dashboard %s JSON not in graph, fetching from Grafana API", uid)
	dashboardData, err := s.grafanaClient.GetDashboard(ctx, uid)
	if err != nil {
		return nil, "", fmt.Errorf("fetch dashboard from Grafana: %w", err)
	}

	// Extract the dashboard object from the response (Grafana wraps it)
	if dashboard, ok := dashboardData["dashboard"].(map[string]interface{}); ok {
		dashboardJSON = dashboard
	} else {
		dashboardJSON = dashboardData
	}

	// Use title from API if not found in graph
	if title == "" {
		if apiTitle, ok := dashboardJSON["title"].(string); ok {
			title = apiTitle
		}
	}

	return dashboardJSON, title, nil
}

// extractPanels parses dashboard JSON and extracts panels with queries.
// Also resolves variable-based datasources to actual UIDs.
func (s *GrafanaQueryService) extractPanels(ctx context.Context, dashboardJSON map[string]interface{}) ([]dashboardPanel, error) {
	panels := make([]dashboardPanel, 0)

	// Extract default datasource UID from dashboard templating
	defaultDatasourceUID := s.extractDefaultDatasource(dashboardJSON)

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
			// Resolve variable-based datasource
			panel.DatasourceUID = s.resolveDatasourceUID(ctx, panel.DatasourceUID, defaultDatasourceUID)
			if panel.DatasourceUID != "" {
				panels = append(panels, *panel)
			}
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
					// Resolve variable-based datasource
					nestedPanel.DatasourceUID = s.resolveDatasourceUID(ctx, nestedPanel.DatasourceUID, defaultDatasourceUID)
					if nestedPanel.DatasourceUID != "" {
						panels = append(panels, *nestedPanel)
					}
				}
			}
		}
	}

	return panels, nil
}

// extractDefaultDatasource finds the default Prometheus datasource from dashboard templating.
// Looks for datasource variables and extracts the current/default value.
func (s *GrafanaQueryService) extractDefaultDatasource(dashboardJSON map[string]interface{}) string {
	templating, ok := dashboardJSON["templating"].(map[string]interface{})
	if !ok {
		return ""
	}

	list, ok := templating["list"].([]interface{})
	if !ok {
		return ""
	}

	for _, item := range list {
		variable, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		varType, _ := variable["type"].(string)
		if varType != "datasource" {
			continue
		}

		// Check if it's a Prometheus datasource variable
		query, _ := variable["query"].(string)
		if query != "prometheus" && query != "Prometheus" {
			// Also check regex field for "prometheus" type
			if queryMap, ok := variable["query"].(map[string]interface{}); ok {
				query, _ = queryMap["type"].(string)
			}
			if query != "prometheus" {
				continue
			}
		}

		// Try to get current value
		if current, ok := variable["current"].(map[string]interface{}); ok {
			// Try uid field first (Grafana 9+)
			if uid, ok := current["value"].(string); ok && uid != "" && !strings.HasPrefix(uid, "$") {
				return uid
			}
			// Try text as fallback
			if text, ok := current["text"].(string); ok && text != "" && !strings.HasPrefix(text, "$") {
				return text
			}
		}

		// Try options array for default
		if options, ok := variable["options"].([]interface{}); ok && len(options) > 0 {
			if opt, ok := options[0].(map[string]interface{}); ok {
				if uid, ok := opt["value"].(string); ok && uid != "" && !strings.HasPrefix(uid, "$") {
					return uid
				}
			}
		}
	}

	return ""
}

// resolveDatasourceUID resolves variable-based datasources to actual UIDs.
// Returns the original UID if not a variable, or the default if it is.
// Falls back to querying Grafana API for a Prometheus datasource if needed.
func (s *GrafanaQueryService) resolveDatasourceUID(ctx context.Context, uid string, defaultUID string) string {
	// Check if a UID needs resolution (is not a real datasource UID)
	needsResolution := func(u string) bool {
		return u == "" ||
			u == "default" ||
			strings.HasPrefix(u, "$") ||
			strings.HasPrefix(u, "${") ||
			strings.HasPrefix(u, "-- ")
	}

	if needsResolution(uid) {
		// Check if defaultUID is a valid (non-special) UID
		if defaultUID != "" && !needsResolution(defaultUID) {
			return defaultUID
		}
		// Try to get a Prometheus datasource from Grafana API
		if promUID := s.getPrometheusDatasourceUID(ctx); promUID != "" {
			return promUID
		}
		// Log that we couldn't resolve the datasource
		s.logger.Debug("Could not resolve datasource %q, no default or fallback available", uid)
		return ""
	}
	return uid
}

// getPrometheusDatasourceUID fetches and caches a Prometheus datasource UID from Grafana.
// It first looks for the default Prometheus datasource, then falls back to any Prometheus datasource.
// Uses a mutex to allow retries if the initial lookup fails.
func (s *GrafanaQueryService) getPrometheusDatasourceUID(ctx context.Context) string {
	s.promDatasourceMu.Lock()
	defer s.promDatasourceMu.Unlock()

	// Return cached value if available
	if s.promDatasourceUID != "" {
		return s.promDatasourceUID
	}

	datasources, err := s.grafanaClient.ListDatasources(ctx)
	if err != nil {
		s.logger.Debug("Failed to list datasources for fallback resolution: %v", err)
		return ""
	}

	// First pass: look for default Prometheus datasource
	for _, ds := range datasources {
		dsType, _ := ds["type"].(string)
		isDefault, _ := ds["isDefault"].(bool)
		if dsType == "prometheus" && isDefault {
			if uid, ok := ds["uid"].(string); ok {
				s.promDatasourceUID = uid
				s.logger.Debug("Using default Prometheus datasource for variable resolution: %s", uid)
				return uid
			}
		}
	}

	// Second pass: find any Prometheus datasource
	for _, ds := range datasources {
		dsType, _ := ds["type"].(string)
		if dsType == "prometheus" {
			if uid, ok := ds["uid"].(string); ok {
				s.promDatasourceUID = uid
				s.logger.Debug("Using Prometheus datasource for variable resolution: %s", uid)
				return uid
			}
		}
	}

	s.logger.Warn("No Prometheus datasource found in Grafana for variable resolution")
	return ""
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

// FetchCurrentValue fetches the current value of a metric for a workload.
// This method implements the QueryService interface for ObservatoryInvestigateService.
//
// Note: In production, this would query Grafana for the actual current value.
// For now, it returns an error indicating the method is not fully implemented.
// The ObservatoryInvestigateService will fall back to using the baseline mean.
func (s *GrafanaQueryService) FetchCurrentValue(ctx context.Context, metricName, namespace, workload string) (float64, error) {
	// TODO: Implement actual Grafana query for current metric value
	// This would require:
	// 1. Finding the dashboard/panel that sources this metric
	// 2. Executing a point-in-time query via Grafana API
	// 3. Extracting the current value from the response
	//
	// For now, return an error to trigger the baseline fallback
	return 0, fmt.Errorf("FetchCurrentValue not implemented: %s/%s/%s", namespace, workload, metricName)
}

// FetchHistoricalValue fetches a metric value from lookback duration ago.
// This method implements the QueryService interface for ObservatoryInvestigateService.
//
// Note: In production, this would query Grafana for the historical value.
// For now, it returns an error indicating the method is not fully implemented.
// The ObservatoryInvestigateService will fall back to using the baseline mean.
func (s *GrafanaQueryService) FetchHistoricalValue(ctx context.Context, metricName, namespace, workload string, lookback time.Duration) (float64, error) {
	// TODO: Implement actual Grafana query for historical metric value
	// This would require:
	// 1. Finding the dashboard/panel that sources this metric
	// 2. Executing a point-in-time query at (now - lookback) via Grafana API
	// 3. Extracting the historical value from the response
	//
	// For now, return an error to trigger the baseline fallback
	return 0, fmt.Errorf("FetchHistoricalValue not implemented: %s/%s/%s at -%s", namespace, workload, metricName, lookback)
}
