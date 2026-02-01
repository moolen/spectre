package grafana

import (
	"context"
	"fmt"
	"regexp"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// AlertSignalMatch represents a matched pair of alert and signal anchor
type AlertSignalMatch struct {
	AlertUID       string
	AlertTitle     string
	SignalAnchorID string // composite key: metric_name/workload_namespace/workload_name
	MetricName     string
	WorkloadUID    string
	WorkloadName   string
	Namespace      string
	WorkloadKind   string
}

// AlertSignalMatcher finds SignalAnchors related to alerts by matching metric names
// extracted from alert PromQL expressions.
type AlertSignalMatcher struct {
	graphClient     graph.Client
	integrationName string
	metricNameRegex *regexp.Regexp
	logger          *logging.Logger
}

// NewAlertSignalMatcher creates a new AlertSignalMatcher.
func NewAlertSignalMatcher(graphClient graph.Client, integrationName string, logger *logging.Logger) *AlertSignalMatcher {
	// Regex to extract metric names from PromQL
	// Matches: metric_name{...}, metric_name[5m], metric_name alone
	// Does not match: function names (which are followed by '(')
	metricRegex := regexp.MustCompile(`\b([a-zA-Z_:][a-zA-Z0-9_:]*)\s*(?:\{|\[|$)`)

	return &AlertSignalMatcher{
		graphClient:     graphClient,
		integrationName: integrationName,
		metricNameRegex: metricRegex,
		logger:          logger,
	}
}

// ExtractMetricNames extracts metric names from a PromQL expression.
// Uses simple regex matching, not full PromQL parsing.
// Example: "rate(http_requests_total{code=~"5.."}[5m]) > 0.1"
//
//	-> ["http_requests_total"]
func (m *AlertSignalMatcher) ExtractMetricNames(promQL string) []string {
	if promQL == "" {
		return nil
	}

	// PromQL function names to exclude
	functions := map[string]bool{
		"abs": true, "absent": true, "absent_over_time": true, "avg": true,
		"bottomk": true, "ceil": true, "changes": true,
		"clamp": true, "clamp_max": true, "clamp_min": true, "count": true,
		"count_over_time": true, "count_values": true, "day_of_month": true,
		"day_of_week": true, "days_in_month": true, "delta": true, "deriv": true,
		"exp": true, "floor": true, "group": true, "histogram_quantile": true,
		"holt_winters": true, "hour": true, "idelta": true, "increase": true,
		"irate": true, "label_join": true, "label_replace": true, "last_over_time": true,
		"ln": true, "log10": true, "log2": true, "max": true, "max_over_time": true,
		"min": true, "min_over_time": true, "minute": true, "month": true,
		"predict_linear": true, "present_over_time": true, "quantile": true,
		"quantile_over_time": true, "rate": true, "resets": true, "round": true,
		"scalar": true, "sgn": true, "sort": true, "sort_desc": true, "sqrt": true,
		"stddev": true, "stddev_over_time": true, "stdvar": true, "stdvar_over_time": true,
		"sum": true, "sum_over_time": true, "time": true, "timestamp": true,
		"topk": true, "vector": true, "year": true, "avg_over_time": true,
		"by": true, "without": true, "on": true, "ignoring": true, "group_left": true,
		"group_right": true, "bool": true, "and": true, "or": true, "unless": true,
	}

	matches := m.metricNameRegex.FindAllStringSubmatch(promQL, -1)
	seen := make(map[string]bool)
	var metrics []string

	for _, match := range matches {
		if len(match) >= 2 {
			name := match[1]
			// Skip if it's a function name or already seen
			if !functions[name] && !seen[name] {
				seen[name] = true
				metrics = append(metrics, name)
			}
		}
	}

	return metrics
}

// FindMatchingSignals finds SignalAnchors that track metrics used by the given alert.
// Returns matches including workload context from MONITORS_WORKLOAD edges.
func (m *AlertSignalMatcher) FindMatchingSignals(ctx context.Context, alertUID string, alertPromQL string) ([]AlertSignalMatch, error) {
	metricNames := m.ExtractMetricNames(alertPromQL)
	if len(metricNames) == 0 {
		m.logger.Debug("No metric names found in alert %s PromQL", alertUID)
		return nil, nil
	}

	m.logger.Debug("Extracted %d metric names from alert %s: %v", len(metricNames), alertUID, metricNames)

	// Query for matching SignalAnchors with workload context
	// Note: FalkorDB quirk - use NOT r.deleted instead of r.deleted = false
	query := `
UNWIND $metricNames AS metricName
MATCH (s:SignalAnchor {metric_name: metricName})-[mw:MONITORS_WORKLOAD]->(r:ResourceIdentity)
WHERE NOT r.deleted
  AND NOT mw.stale
RETURN DISTINCT
    s.metric_name AS metricName,
    s.workload_namespace AS workloadNamespace,
    s.workload_name AS workloadName,
    r.uid AS workloadUID,
    r.name AS workloadResourceName,
    r.namespace AS namespace,
    r.kind AS kind
`

	result, err := m.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"metricNames": metricNames,
		},
		Timeout: 10000,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query matching signals: %w", err)
	}

	var matches []AlertSignalMatch
	for _, row := range result.Rows {
		if len(row) < 7 {
			continue
		}

		metricName, _ := row[0].(string)
		workloadNamespace, _ := row[1].(string)
		workloadName, _ := row[2].(string)
		workloadUID, _ := row[3].(string)
		_, _ = row[4].(string) // workloadResourceName - not needed
		namespace, _ := row[5].(string)
		kind, _ := row[6].(string)

		matches = append(matches, AlertSignalMatch{
			AlertUID:       alertUID,
			SignalAnchorID: fmt.Sprintf("%s/%s/%s", metricName, workloadNamespace, workloadName),
			MetricName:     metricName,
			WorkloadUID:    workloadUID,
			WorkloadName:   workloadName,
			Namespace:      namespace,
			WorkloadKind:   kind,
		})
	}

	m.logger.Debug("Found %d signal matches for alert %s", len(matches), alertUID)
	return matches, nil
}

// GetAlertPromQL retrieves the PromQL expression for an alert.
func (m *AlertSignalMatcher) GetAlertPromQL(ctx context.Context, alertUID string) (string, string, error) {
	query := `
MATCH (a:Alert {uid: $alertUID, integration: $integration})
RETURN a.condition AS promql, a.title AS title
LIMIT 1
`

	result, err := m.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"alertUID":    alertUID,
			"integration": m.integrationName,
		},
		Timeout: 5000,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to get alert PromQL: %w", err)
	}

	if len(result.Rows) == 0 || len(result.Rows[0]) < 2 {
		return "", "", fmt.Errorf("alert %s not found", alertUID)
	}

	promQL, _ := result.Rows[0][0].(string)
	title, _ := result.Rows[0][1].(string)
	return promQL, title, nil
}

// ListAlertsWithTransitions lists all alerts that have state transitions in the graph.
func (m *AlertSignalMatcher) ListAlertsWithTransitions(ctx context.Context) ([]string, error) {
	query := `
MATCH (a:Alert {integration: $integration})-[:STATE_TRANSITION]->(a)
RETURN DISTINCT a.uid AS uid
`

	result, err := m.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"integration": m.integrationName,
		},
		Timeout: 10000,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list alerts with transitions: %w", err)
	}

	var alertUIDs []string
	for _, row := range result.Rows {
		if len(row) > 0 {
			if uid, ok := row[0].(string); ok {
				alertUIDs = append(alertUIDs, uid)
			}
		}
	}

	return alertUIDs, nil
}
