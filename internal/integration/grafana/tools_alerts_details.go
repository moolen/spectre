package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// AlertsDetailsTool provides deep debugging with full state history
// Returns complete 7-day state timeline with timestamps, rule definitions, and metadata
type AlertsDetailsTool struct {
	graphClient     graph.Client
	integrationName string
	analysisService *AlertAnalysisService
	logger          *logging.Logger
}

// NewAlertsDetailsTool creates a new details alerts tool
func NewAlertsDetailsTool(
	graphClient graph.Client,
	integrationName string,
	analysisService *AlertAnalysisService,
	logger *logging.Logger,
) *AlertsDetailsTool {
	return &AlertsDetailsTool{
		graphClient:     graphClient,
		integrationName: integrationName,
		analysisService: analysisService,
		logger:          logger,
	}
}

// AlertsDetailsParams defines input parameters for details alerts tool
type AlertsDetailsParams struct {
	AlertUID  string `json:"alert_uid,omitempty"`  // Optional: specific alert UID
	Severity  string `json:"severity,omitempty"`   // Optional: "critical", "warning", "info"
	Cluster   string `json:"cluster,omitempty"`    // Optional: cluster name
	Service   string `json:"service,omitempty"`    // Optional: service name
	Namespace string `json:"namespace,omitempty"`  // Optional: namespace name
}

// AlertsDetailsResponse contains detailed alert information
type AlertsDetailsResponse struct {
	Alerts    []DetailAlert `json:"alerts"`
	Timestamp string        `json:"timestamp"` // ISO8601
}

// DetailAlert represents complete alert details for deep debugging
type DetailAlert struct {
	Name           string                 `json:"name"`
	State          string                 `json:"state"`           // Current state
	UID            string                 `json:"uid"`             // Unique identifier
	Labels         map[string]string      `json:"labels"`          // All alert labels
	Annotations    map[string]string      `json:"annotations"`     // All annotations
	RuleDefinition string                 `json:"rule_definition"` // Alert rule condition
	StateTimeline  []StatePoint           `json:"state_timeline"`  // Full 7-day history
	Analysis       *AnalysisDetail        `json:"analysis,omitempty"` // Optional analysis
}

// StatePoint represents a single state transition with duration
type StatePoint struct {
	Timestamp      string `json:"timestamp"`        // ISO8601
	FromState      string `json:"from_state"`       // Previous state
	ToState        string `json:"to_state"`         // New state
	DurationInState string `json:"duration_in_state"` // Time spent in from_state before transition
}

// AnalysisDetail contains full analysis metrics
type AnalysisDetail struct {
	FlappinessScore float64           `json:"flappiness_score"`
	Category        string            `json:"category"`
	DeviationScore  float64           `json:"deviation_score"`
	Baseline        StateDistribution `json:"baseline"`
}

// Execute runs the details alerts tool
func (t *AlertsDetailsTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
	var params AlertsDetailsParams
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Validate: require either alert_uid OR at least one filter
	if params.AlertUID == "" && params.Severity == "" && params.Cluster == "" &&
		params.Service == "" && params.Namespace == "" {
		return nil, fmt.Errorf("must provide alert_uid or at least one filter (severity, cluster, service, namespace)")
	}

	// Query graph for Alert nodes
	alerts, err := t.fetchDetailAlerts(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("fetch alerts: %w", err)
	}

	// Warn if multiple alerts without alert_uid (can produce large responses)
	if params.AlertUID == "" && len(alerts) > 5 {
		t.logger.Warn("Fetching details for %d alerts - response may be large", len(alerts))
	}

	// Process each alert: fetch full state history and analysis
	currentTime := time.Now()
	sevenDaysAgo := currentTime.Add(-7 * 24 * time.Hour)
	detailAlerts := make([]DetailAlert, 0, len(alerts))

	for _, alertInfo := range alerts {
		// Fetch full 7-day state transition history
		transitions, err := FetchStateTransitions(
			ctx,
			t.graphClient,
			alertInfo.UID,
			t.integrationName,
			sevenDaysAgo,
			currentTime,
		)
		if err != nil {
			t.logger.Warn("Failed to fetch transitions for alert %s: %v", alertInfo.Name, err)
			continue
		}

		// Build full state timeline with durations
		stateTimeline := buildDetailStateTimeline(transitions, sevenDaysAgo)

		// Determine current state
		currentState := determineCurrentState(transitions, currentTime)

		// Get full analysis if service available
		var analysisDetail *AnalysisDetail
		if t.analysisService != nil {
			analysis, err := t.analysisService.AnalyzeAlert(ctx, alertInfo.UID)
			if err == nil {
				analysisDetail = &AnalysisDetail{
					FlappinessScore: analysis.FlappinessScore,
					Category:        formatCategory(analysis.Categories, analysis.FlappinessScore),
					DeviationScore:  analysis.DeviationScore,
					Baseline:        analysis.Baseline,
				}
			} else {
				// Don't fail on analysis error, just skip enrichment
				t.logger.Debug("Failed to analyze alert %s: %v", alertInfo.Name, err)
			}
		}

		detailAlerts = append(detailAlerts, DetailAlert{
			Name:           alertInfo.Name,
			State:          currentState,
			UID:            alertInfo.UID,
			Labels:         alertInfo.Labels,
			Annotations:    alertInfo.Annotations,
			RuleDefinition: alertInfo.RuleDefinition,
			StateTimeline:  stateTimeline,
			Analysis:       analysisDetail,
		})
	}

	return &AlertsDetailsResponse{
		Alerts:    detailAlerts,
		Timestamp: currentTime.Format(time.RFC3339),
	}, nil
}

// fetchDetailAlerts queries the graph for Alert nodes with full metadata
func (t *AlertsDetailsTool) fetchDetailAlerts(ctx context.Context, params AlertsDetailsParams) ([]detailAlertInfo, error) {
	// Build WHERE clause dynamically based on filters
	whereClauses := []string{"a.integration = $integration"}
	parameters := map[string]interface{}{
		"integration": t.integrationName,
	}

	if params.AlertUID != "" {
		whereClauses = append(whereClauses, "a.uid = $uid")
		parameters["uid"] = params.AlertUID
	}
	if params.Severity != "" {
		whereClauses = append(whereClauses, "a.severity = $severity")
		parameters["severity"] = params.Severity
	}
	if params.Cluster != "" {
		whereClauses = append(whereClauses, "a.cluster = $cluster")
		parameters["cluster"] = params.Cluster
	}
	if params.Service != "" {
		whereClauses = append(whereClauses, "a.service = $service")
		parameters["service"] = params.Service
	}
	if params.Namespace != "" {
		whereClauses = append(whereClauses, "a.namespace = $namespace")
		parameters["namespace"] = params.Namespace
	}

	whereClause := strings.Join(whereClauses, " AND ")

	query := fmt.Sprintf(`
MATCH (a:Alert)
WHERE %s
RETURN a.uid AS uid,
       a.name AS name,
       a.labels AS labels,
       a.annotations AS annotations,
       a.condition AS condition
ORDER BY a.name
`, whereClause)

	result, err := t.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query:      query,
		Parameters: parameters,
		Timeout:    5000, // 5 seconds
	})
	if err != nil {
		return nil, fmt.Errorf("graph query failed: %w", err)
	}

	// Parse results
	alerts := make([]detailAlertInfo, 0)
	for _, row := range result.Rows {
		if len(row) < 5 {
			continue
		}

		uid, _ := row[0].(string)
		name, _ := row[1].(string)

		// Parse labels (stored as JSON string in graph)
		labels := make(map[string]string)
		if labelsRaw, ok := row[2].(string); ok && labelsRaw != "" {
			_ = json.Unmarshal([]byte(labelsRaw), &labels)
		}

		// Parse annotations (stored as JSON string in graph)
		annotations := make(map[string]string)
		if annotationsRaw, ok := row[3].(string); ok && annotationsRaw != "" {
			_ = json.Unmarshal([]byte(annotationsRaw), &annotations)
		}

		condition, _ := row[4].(string)

		if uid != "" && name != "" {
			alerts = append(alerts, detailAlertInfo{
				UID:            uid,
				Name:           name,
				Labels:         labels,
				Annotations:    annotations,
				RuleDefinition: condition,
			})
		}
	}

	return alerts, nil
}

// buildDetailStateTimeline creates full state timeline with explicit timestamps and durations
func buildDetailStateTimeline(transitions []StateTransition, windowStart time.Time) []StatePoint {
	if len(transitions) == 0 {
		return []StatePoint{}
	}

	statePoints := make([]StatePoint, 0, len(transitions))

	// Track previous timestamp for duration calculation
	var prevTimestamp time.Time
	if len(transitions) > 0 {
		// Use windowStart or first transition time
		if transitions[0].Timestamp.After(windowStart) {
			prevTimestamp = windowStart
		} else {
			prevTimestamp = transitions[0].Timestamp
		}
	}

	for i, t := range transitions {
		// Calculate duration in from_state (time since last transition)
		var durationInState time.Duration
		if i == 0 {
			// First transition: duration from window start to this transition
			if t.Timestamp.After(windowStart) {
				durationInState = t.Timestamp.Sub(windowStart)
			} else {
				durationInState = 0
			}
		} else {
			durationInState = t.Timestamp.Sub(prevTimestamp)
		}

		statePoints = append(statePoints, StatePoint{
			Timestamp:       t.Timestamp.Format(time.RFC3339),
			FromState:       t.FromState,
			ToState:         t.ToState,
			DurationInState: formatDuration(durationInState),
		})

		prevTimestamp = t.Timestamp
	}

	return statePoints
}

// detailAlertInfo holds complete alert information from graph query
type detailAlertInfo struct {
	UID            string
	Name           string
	Labels         map[string]string
	Annotations    map[string]string
	RuleDefinition string
}
