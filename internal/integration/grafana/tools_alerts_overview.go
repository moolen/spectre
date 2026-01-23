package grafana

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// AlertsOverviewTool provides high-level overview of alerts with filtering and flappiness indicators.
// Groups alerts by severity and optionally filters by cluster, service, namespace, or severity level.
// Follows progressive disclosure pattern: overview -> list -> analyze.
type AlertsOverviewTool struct {
	graphClient     graph.Client
	integrationName string
	analysisService *AlertAnalysisService
	logger          *logging.Logger
}

// NewAlertsOverviewTool creates a new alerts overview tool.
// analysisService may be nil if graph disabled (tool still works, just no flappiness data).
func NewAlertsOverviewTool(gc graph.Client, integrationName string, as *AlertAnalysisService, logger *logging.Logger) *AlertsOverviewTool {
	return &AlertsOverviewTool{
		graphClient:     gc,
		integrationName: integrationName,
		analysisService: as,
		logger:          logger,
	}
}

// AlertsOverviewParams defines input parameters for alerts overview tool.
// All parameters are optional - no filters means "all alerts".
type AlertsOverviewParams struct {
	Severity  string `json:"severity"`  // Optional: "critical", "warning", "info" (case-insensitive)
	Cluster   string `json:"cluster"`   // Optional: filter by cluster label
	Service   string `json:"service"`   // Optional: filter by service label
	Namespace string `json:"namespace"` // Optional: filter by namespace label
}

// AlertsOverviewResponse contains aggregated alert counts grouped by severity.
type AlertsOverviewResponse struct {
	AlertsBySeverity map[string]SeverityBucket `json:"alerts_by_severity"`
	FiltersApplied   *AlertsOverviewParams     `json:"filters_applied,omitempty"`
	Timestamp        string                    `json:"timestamp"` // RFC3339
}

// SeverityBucket groups alerts within a severity level.
type SeverityBucket struct {
	Count         int            `json:"count"`
	FlappingCount int            `json:"flapping_count"` // Alerts with flappiness > 0.7
	Alerts        []AlertSummary `json:"alerts"`
}

// AlertSummary provides minimal alert context for triage.
type AlertSummary struct {
	Name          string `json:"name"`
	FiringDuration string `json:"firing_duration"` // Human-readable like "2h" or "45m"
	Cluster       string `json:"cluster,omitempty"`
	Service       string `json:"service,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
}

// Execute runs the alerts overview tool.
func (t *AlertsOverviewTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
	var params AlertsOverviewParams
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Normalize severity filter for case-insensitive matching
	if params.Severity != "" {
		params.Severity = strings.ToLower(params.Severity)
	}

	// Query graph for firing/pending alerts matching filters
	alerts, err := t.queryAlerts(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("query alerts: %w", err)
	}

	// Group by severity and enrich with flappiness
	alertsBySeverity := t.groupBySeverity(ctx, alerts)

	// Build response
	response := &AlertsOverviewResponse{
		AlertsBySeverity: alertsBySeverity,
		Timestamp:        time.Now().UTC().Format(time.RFC3339),
	}

	// Include filters in response if any were applied
	if params.Severity != "" || params.Cluster != "" || params.Service != "" || params.Namespace != "" {
		response.FiltersApplied = &params
	}

	return response, nil
}

// alertData holds alert information from graph query
type alertData struct {
	UID            string
	Title          string
	State          string
	StateTimestamp time.Time
	Labels         string // JSON string
}

// queryAlerts fetches alerts from graph matching filters.
// Returns alerts in firing or pending state.
func (t *AlertsOverviewTool) queryAlerts(ctx context.Context, params AlertsOverviewParams) ([]alertData, error) {
	// Build base query for firing/pending alerts
	query := `
		MATCH (a:Alert {integration: $integration})
		WHERE a.state IN ['firing', 'pending']
	`

	queryParams := map[string]interface{}{
		"integration": t.integrationName,
	}

	// Add label-based filters if specified
	// Labels are stored as JSON string, so we use string matching
	labelFilters := []string{}

	if params.Cluster != "" {
		labelFilters = append(labelFilters, fmt.Sprintf("a.labels CONTAINS '\"cluster\":\"%s\"'", params.Cluster))
	}
	if params.Service != "" {
		labelFilters = append(labelFilters, fmt.Sprintf("a.labels CONTAINS '\"service\":\"%s\"'", params.Service))
	}
	if params.Namespace != "" {
		labelFilters = append(labelFilters, fmt.Sprintf("a.labels CONTAINS '\"namespace\":\"%s\"'", params.Namespace))
	}
	if params.Severity != "" {
		// Severity normalization: match case-insensitively
		labelFilters = append(labelFilters, fmt.Sprintf("toLower(a.labels) CONTAINS '\"severity\":\"%s\"'", params.Severity))
	}

	// Append label filters to query
	for _, filter := range labelFilters {
		query += fmt.Sprintf(" AND %s", filter)
	}

	// Return alert data with state timestamp
	query += `
		RETURN a.uid AS uid,
		       a.title AS title,
		       a.state AS state,
		       a.state_timestamp AS state_timestamp,
		       a.labels AS labels
		ORDER BY a.title
	`

	result, err := t.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query:      query,
		Parameters: queryParams,
	})
	if err != nil {
		return nil, fmt.Errorf("graph query: %w", err)
	}

	// Parse results
	alerts := make([]alertData, 0)
	for _, row := range result.Rows {
		alert := alertData{}

		// Extract columns safely
		if len(row) >= 5 {
			alert.UID, _ = row[0].(string)
			alert.Title, _ = row[1].(string)
			alert.State, _ = row[2].(string)

			// Parse state timestamp
			if timestampStr, ok := row[3].(string); ok {
				if ts, err := time.Parse(time.RFC3339, timestampStr); err == nil {
					alert.StateTimestamp = ts
				}
			}

			alert.Labels, _ = row[4].(string)
		}

		if alert.UID != "" {
			alerts = append(alerts, alert)
		}
	}

	return alerts, nil
}

// groupBySeverity groups alerts by severity and enriches with flappiness data.
func (t *AlertsOverviewTool) groupBySeverity(ctx context.Context, alerts []alertData) map[string]SeverityBucket {
	buckets := make(map[string]SeverityBucket)

	for _, alert := range alerts {
		// Extract severity from labels (default to "unknown" if missing)
		severity := extractSeverity(alert.Labels)

		// Get or create bucket
		bucket, exists := buckets[severity]
		if !exists {
			bucket = SeverityBucket{
				Count:         0,
				FlappingCount: 0,
				Alerts:        []AlertSummary{},
			}
		}

		// Compute firing duration
		firingDuration := computeFiringDuration(alert.StateTimestamp)

		// Extract labels for summary
		cluster := extractLabel(alert.Labels, "cluster")
		service := extractLabel(alert.Labels, "service")
		namespace := extractLabel(alert.Labels, "namespace")

		// Create alert summary
		summary := AlertSummary{
			Name:           alert.Title,
			FiringDuration: firingDuration,
			Cluster:        cluster,
			Service:        service,
			Namespace:      namespace,
		}

		// Check flappiness if analysis service available
		isFlapping := false
		if t.analysisService != nil {
			analysis, err := t.analysisService.AnalyzeAlert(ctx, alert.UID)
			if err == nil {
				// Flapping threshold: 0.7 (from Phase 22-02)
				if analysis.FlappinessScore > 0.7 {
					isFlapping = true
					bucket.FlappingCount++
				}
			} else {
				// Handle ErrInsufficientData gracefully - not an error, just new alert
				var insufficientErr ErrInsufficientData
				if !errors.As(err, &insufficientErr) {
					// Log unexpected errors but continue
					t.logger.Warn("Failed to analyze alert %s: %v", alert.UID, err)
				}
			}
		}

		// Update bucket
		bucket.Count++
		bucket.Alerts = append(bucket.Alerts, summary)
		buckets[severity] = bucket

		t.logger.Debug("Alert %s: severity=%s, flapping=%v, duration=%s",
			alert.Title, severity, isFlapping, firingDuration)
	}

	return buckets
}

// extractSeverity extracts severity label from JSON labels string.
// Returns "unknown" if severity label not found.
func extractSeverity(labelsJSON string) string {
	severity := extractLabel(labelsJSON, "severity")
	if severity == "" {
		return "unknown"
	}
	// Normalize to lowercase for consistent bucketing
	return strings.ToLower(severity)
}

// extractLabel extracts a label value from JSON labels string.
// Returns empty string if label not found.
func extractLabel(labelsJSON, key string) string {
	// Parse JSON labels
	var labels map[string]string
	if err := json.Unmarshal([]byte(labelsJSON), &labels); err != nil {
		return ""
	}
	return labels[key]
}

// computeFiringDuration computes human-readable duration since alert started firing.
// Returns strings like "2h", "45m", "3d"
func computeFiringDuration(stateTimestamp time.Time) string {
	if stateTimestamp.IsZero() {
		return "unknown"
	}

	duration := time.Since(stateTimestamp)

	// Format duration in human-readable form
	if duration < time.Minute {
		return "< 1m"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		return fmt.Sprintf("%dm", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%dh", hours)
	} else {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%dd", days)
	}
}
