package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/observatory"
)

// GrafanaObservatoryProvider implements observatory.Provider for Grafana integration.
// It adapts Grafana's graph-based signal storage to the Observatory interface.
type GrafanaObservatoryProvider struct {
	graphClient     graph.Client
	grafanaClient   *GrafanaClient
	integrationName string
	logger          *logging.Logger

	// Cache for current values (metric|ns|workload -> cachedValue)
	valueCache   sync.Map
	valueCacheTTL time.Duration
}

// cachedValue holds a cached current value with expiration.
type cachedValue struct {
	value     float64
	expiresAt time.Time
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
		valueCacheTTL:   2 * time.Minute, // Cache values for 2 minutes
	}
}

// SetGrafanaClient sets the Grafana HTTP client for executing metric queries.
// This enables GetCurrentValue to fetch live metric values from Grafana.
func (p *GrafanaObservatoryProvider) SetGrafanaClient(client *GrafanaClient) {
	p.grafanaClient = client
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
// Queries Grafana for the live metric value using the signal's associated dashboard/panel.
// Uses caching to avoid repeated queries for the same metric within the TTL period.
//
// Returns (value, found, error):
// - (value, true, nil) - Successfully fetched the current value
// - (0, false, nil) - Metric not found or no Grafana client configured (uses baseline mean fallback)
// - (0, false, error) - Query failed
func (p *GrafanaObservatoryProvider) GetCurrentValue(
	ctx context.Context,
	metricName, namespace, workload string,
) (float64, bool, error) {
	// Build cache key
	cacheKey := fmt.Sprintf("%s|%s|%s", metricName, namespace, workload)

	// Check cache first
	if cached, ok := p.valueCache.Load(cacheKey); ok {
		cv := cached.(*cachedValue)
		if time.Now().Before(cv.expiresAt) {
			return cv.value, true, nil
		}
		// Expired, delete from cache
		p.valueCache.Delete(cacheKey)
	}

	// If no Grafana client, fall back to baseline mean
	if p.grafanaClient == nil {
		return 0, false, nil
	}

	// Look up the SignalAnchor to get dashboard_uid and panel_id
	dashboardUID, panelID, datasourceUID, err := p.getSignalSource(ctx, metricName, namespace, workload)
	if err != nil {
		p.logger.Debug("Failed to get signal source for %s: %v", metricName, err)
		return 0, false, nil // Graceful degradation
	}
	if dashboardUID == "" {
		return 0, false, nil // No dashboard associated
	}

	// Get the PromQL query from the dashboard
	promQL, err := p.getPromQLFromDashboard(ctx, dashboardUID, panelID)
	if err != nil {
		p.logger.Debug("Failed to get PromQL for %s from dashboard %s panel %d: %v",
			metricName, dashboardUID, panelID, err)
		return 0, false, nil // Graceful degradation
	}
	if promQL == "" {
		return 0, false, nil // No query found
	}

	// Execute instant query via Grafana
	value, err := p.executeInstantQuery(ctx, datasourceUID, promQL)
	if err != nil {
		p.logger.Debug("Failed to execute instant query for %s: %v", metricName, err)
		return 0, false, nil // Graceful degradation
	}

	// Cache the result
	p.valueCache.Store(cacheKey, &cachedValue{
		value:     value,
		expiresAt: time.Now().Add(p.valueCacheTTL),
	})

	return value, true, nil
}

// getSignalSource retrieves the dashboard UID, panel ID, and datasource UID for a signal.
func (p *GrafanaObservatoryProvider) getSignalSource(
	ctx context.Context,
	metricName, namespace, workload string,
) (dashboardUID string, panelID int, datasourceUID string, err error) {
	query := `
		MATCH (s:SignalAnchor {
			metric_name: $metric_name,
			workload_namespace: $namespace,
			workload_name: $workload_name,
			integration: $integration
		})
		WHERE s.expires_at > $now
		OPTIONAL MATCH (s)-[:SOURCED_FROM]->(d:Dashboard)
		RETURN s.dashboard_uid AS dashboard_uid,
		       s.panel_id AS panel_id,
		       d.default_datasource_uid AS datasource_uid
	`

	result, err := p.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]any{
			"metric_name":   metricName,
			"namespace":     namespace,
			"workload_name": workload,
			"integration":   p.integrationName,
			"now":           time.Now().Unix(),
		},
	})
	if err != nil {
		return "", 0, "", err
	}

	if len(result.Rows) == 0 {
		return "", 0, "", nil
	}

	// Parse results
	colIdx := make(map[string]int)
	for i, col := range result.Columns {
		colIdx[col] = i
	}

	row := result.Rows[0]
	if idx, ok := colIdx["dashboard_uid"]; ok && idx < len(row) {
		if v, ok := row[idx].(string); ok {
			dashboardUID = v
		}
	}
	if idx, ok := colIdx["panel_id"]; ok && idx < len(row) {
		panelID = parseInt(row[idx])
	}
	if idx, ok := colIdx["datasource_uid"]; ok && idx < len(row) {
		if v, ok := row[idx].(string); ok {
			datasourceUID = v
		}
	}

	return dashboardUID, panelID, datasourceUID, nil
}

// getPromQLFromDashboard retrieves the PromQL query from a dashboard's panel.
func (p *GrafanaObservatoryProvider) getPromQLFromDashboard(
	ctx context.Context,
	dashboardUID string,
	panelID int,
) (string, error) {
	// Query the dashboard JSON from graph
	query := `
		MATCH (d:Dashboard {uid: $uid})
		RETURN d.json AS json
	`

	result, err := p.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]any{
			"uid": dashboardUID,
		},
	})
	if err != nil {
		return "", err
	}

	if len(result.Rows) == 0 || len(result.Rows[0]) == 0 {
		return "", nil
	}

	jsonStr, ok := result.Rows[0][0].(string)
	if !ok || jsonStr == "" {
		return "", nil
	}

	// Parse dashboard JSON
	var dashboardJSON map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &dashboardJSON); err != nil {
		return "", err
	}

	// Find the panel with matching ID
	panels, ok := dashboardJSON["panels"].([]any)
	if !ok {
		return "", nil
	}

	for _, p := range panels {
		panel, ok := p.(map[string]any)
		if !ok {
			continue
		}

		// Check panel ID
		id, _ := panel["id"].(float64)
		if int(id) != panelID {
			// Check nested panels (rows)
			if nestedPanels, ok := panel["panels"].([]any); ok {
				for _, np := range nestedPanels {
					nestedPanel, ok := np.(map[string]any)
					if !ok {
						continue
					}
					nestedID, _ := nestedPanel["id"].(float64)
					if int(nestedID) == panelID {
						return extractPromQLFromPanel(nestedPanel), nil
					}
				}
			}
			continue
		}

		return extractPromQLFromPanel(panel), nil
	}

	return "", nil
}

// extractPromQLFromPanel extracts the PromQL expression from a panel's targets.
func extractPromQLFromPanel(panel map[string]any) string {
	targets, ok := panel["targets"].([]any)
	if !ok || len(targets) == 0 {
		return ""
	}

	// Get the first target's expression
	target, ok := targets[0].(map[string]any)
	if !ok {
		return ""
	}

	expr, _ := target["expr"].(string)
	return expr
}

// executeInstantQuery executes a PromQL instant query via Grafana.
func (p *GrafanaObservatoryProvider) executeInstantQuery(
	ctx context.Context,
	datasourceUID, promQL string,
) (float64, error) {
	if datasourceUID == "" {
		// Try to get default datasource
		datasources, err := p.grafanaClient.ListDatasources(ctx)
		if err != nil {
			return 0, err
		}
		for _, ds := range datasources {
			dsType, _ := ds["type"].(string)
			isDefault, _ := ds["isDefault"].(bool)
			if dsType == "prometheus" && isDefault {
				datasourceUID, _ = ds["uid"].(string)
				break
			}
		}
		if datasourceUID == "" {
			// Find any prometheus datasource
			for _, ds := range datasources {
				dsType, _ := ds["type"].(string)
				if dsType == "prometheus" {
					datasourceUID, _ = ds["uid"].(string)
					break
				}
			}
		}
	}

	if datasourceUID == "" {
		return 0, fmt.Errorf("no Prometheus datasource found")
	}

	// Execute instant query (now to now)
	now := time.Now()
	from := fmt.Sprintf("%d", now.Add(-1*time.Minute).UnixMilli())
	to := fmt.Sprintf("%d", now.UnixMilli())

	response, err := p.grafanaClient.QueryDataSource(ctx, datasourceUID, promQL, from, to, nil)
	if err != nil {
		return 0, err
	}

	// Extract the most recent value from the response
	for _, queryResult := range response.Results {
		if queryResult.Error != "" {
			continue
		}
		for _, frame := range queryResult.Frames {
			if len(frame.Data.Values) >= 2 && len(frame.Data.Values[1]) > 0 {
				// Values[0] = timestamps, Values[1] = values
				values := frame.Data.Values[1]
				// Get the last (most recent) value
				if len(values) > 0 {
					if v, ok := values[len(values)-1].(float64); ok {
						return v, nil
					}
				}
			}
		}
	}

	return 0, fmt.Errorf("no data returned from query")
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
