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

// AlertsAggregatedTool provides focused alert investigation with compact state timelines
// Shows specific alerts with 1h state progression in bucket notation [F F N N]
type AlertsAggregatedTool struct {
	graphClient     graph.Client
	integrationName string
	analysisService *AlertAnalysisService
	logger          *logging.Logger
}

// NewAlertsAggregatedTool creates a new aggregated alerts tool
func NewAlertsAggregatedTool(
	graphClient graph.Client,
	integrationName string,
	analysisService *AlertAnalysisService,
	logger *logging.Logger,
) *AlertsAggregatedTool {
	return &AlertsAggregatedTool{
		graphClient:     graphClient,
		integrationName: integrationName,
		analysisService: analysisService,
		logger:          logger,
	}
}

// AlertsAggregatedParams defines input parameters for aggregated alerts tool
type AlertsAggregatedParams struct {
	Lookback  string `json:"lookback,omitempty"`  // Duration string (default "1h")
	Severity  string `json:"severity,omitempty"`  // Optional: "critical", "warning", "info"
	Cluster   string `json:"cluster,omitempty"`   // Optional: cluster name
	Service   string `json:"service,omitempty"`   // Optional: service name
	Namespace string `json:"namespace,omitempty"` // Optional: namespace name
}

// AlertsAggregatedResponse contains aggregated alert results with compact timelines
type AlertsAggregatedResponse struct {
	Alerts         []AggregatedAlert `json:"alerts"`
	Lookback       string            `json:"lookback"`
	FiltersApplied map[string]string `json:"filters_applied,omitempty"`
	Timestamp      string            `json:"timestamp"` // ISO8601
}

// AggregatedAlert represents a single alert with compact state timeline
type AggregatedAlert struct {
	Name             string  `json:"name"`
	State            string  `json:"state"`             // Current state: "firing", "normal", "pending"
	FiringDuration   string  `json:"firing_duration"`   // Human readable duration if firing
	Timeline         string  `json:"timeline"`          // Compact: "[F F N N F F]"
	Category         string  `json:"category"`          // "CHRONIC + flapping", "RECENT + trending-worse"
	FlappinessScore  float64 `json:"flappiness_score"`  // 0.0-1.0
	TransitionCount  int     `json:"transition_count"`  // Number of state changes in lookback
	Cluster          string  `json:"cluster"`
	Service          string  `json:"service,omitempty"`
	Namespace        string  `json:"namespace,omitempty"`
}

// Execute runs the aggregated alerts tool
func (t *AlertsAggregatedTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
	var params AlertsAggregatedParams
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Default lookback to 1h if not specified
	if params.Lookback == "" {
		params.Lookback = "1h"
	}

	// Parse lookback duration
	lookbackDuration, err := time.ParseDuration(params.Lookback)
	if err != nil {
		return nil, fmt.Errorf("invalid lookback duration %q: %w", params.Lookback, err)
	}

	// Build filter map for tracking
	filtersApplied := make(map[string]string)
	if params.Severity != "" {
		filtersApplied["severity"] = params.Severity
	}
	if params.Cluster != "" {
		filtersApplied["cluster"] = params.Cluster
	}
	if params.Service != "" {
		filtersApplied["service"] = params.Service
	}
	if params.Namespace != "" {
		filtersApplied["namespace"] = params.Namespace
	}

	// Query graph for Alert nodes matching filters
	alerts, err := t.fetchAlerts(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("fetch alerts: %w", err)
	}

	// Process each alert: fetch state timeline and enrich with analysis
	currentTime := time.Now()
	startTime := currentTime.Add(-lookbackDuration)
	aggregatedAlerts := make([]AggregatedAlert, 0, len(alerts))

	for _, alertInfo := range alerts {
		// Fetch state transitions for lookback window
		transitions, err := FetchStateTransitions(
			ctx,
			t.graphClient,
			alertInfo.UID,
			t.integrationName,
			startTime,
			currentTime,
		)
		if err != nil {
			t.logger.Warn("Failed to fetch transitions for alert %s: %v", alertInfo.Name, err)
			continue
		}

		// Build compact state timeline (10-minute buckets)
		timeline := buildStateTimeline(transitions, lookbackDuration, startTime, currentTime)

		// Determine current state
		currentState := determineCurrentState(transitions, currentTime)

		// Calculate firing duration if currently firing
		firingDuration := ""
		if currentState == "firing" {
			firingDuration = calculateFiringDuration(transitions, currentTime)
		}

		// Get analysis enrichment (flappiness and categories)
		var flappinessScore float64
		var category string
		var transitionCount int

		if t.analysisService != nil {
			analysis, err := t.analysisService.AnalyzeAlert(ctx, alertInfo.UID)
			if err != nil {
				// Handle insufficient data error gracefully
				var insufficientErr ErrInsufficientData
				if errors.As(err, &insufficientErr) {
					category = "new (insufficient history)"
					flappinessScore = 0.0
				} else {
					t.logger.Warn("Failed to analyze alert %s: %v", alertInfo.Name, err)
					category = "unknown"
					flappinessScore = 0.0
				}
			} else {
				flappinessScore = analysis.FlappinessScore
				category = formatCategory(analysis.Categories, flappinessScore)
			}
		}

		// Count transitions in lookback window
		transitionCount = len(transitions)

		aggregatedAlerts = append(aggregatedAlerts, AggregatedAlert{
			Name:             alertInfo.Name,
			State:            currentState,
			FiringDuration:   firingDuration,
			Timeline:         timeline,
			Category:         category,
			FlappinessScore:  flappinessScore,
			TransitionCount:  transitionCount,
			Cluster:          alertInfo.Cluster,
			Service:          alertInfo.Service,
			Namespace:        alertInfo.Namespace,
		})
	}

	return &AlertsAggregatedResponse{
		Alerts:         aggregatedAlerts,
		Lookback:       params.Lookback,
		FiltersApplied: filtersApplied,
		Timestamp:      currentTime.Format(time.RFC3339),
	}, nil
}

// fetchAlerts queries the graph for Alert nodes matching the provided filters
func (t *AlertsAggregatedTool) fetchAlerts(ctx context.Context, params AlertsAggregatedParams) ([]alertInfo, error) {
	// Build WHERE clause dynamically based on filters
	whereClauses := []string{"a.integration = $integration"}
	parameters := map[string]interface{}{
		"integration": t.integrationName,
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
       a.cluster AS cluster,
       a.service AS service,
       a.namespace AS namespace
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
	alerts := make([]alertInfo, 0)
	for _, row := range result.Rows {
		if len(row) < 5 {
			continue
		}

		uid, _ := row[0].(string)
		name, _ := row[1].(string)
		cluster, _ := row[2].(string)
		service, _ := row[3].(string)
		namespace, _ := row[4].(string)

		if uid != "" && name != "" {
			alerts = append(alerts, alertInfo{
				UID:       uid,
				Name:      name,
				Cluster:   cluster,
				Service:   service,
				Namespace: namespace,
			})
		}
	}

	return alerts, nil
}

// buildStateTimeline creates compact state timeline in bucket notation
// Uses 10-minute buckets with LOCF interpolation
// Format: "[F F F N N N]" (left-to-right = oldest→newest)
func buildStateTimeline(transitions []StateTransition, lookback time.Duration, startTime, endTime time.Time) string {
	// 10-minute buckets
	bucketSize := 10 * time.Minute
	numBuckets := int(lookback / bucketSize)
	if numBuckets == 0 {
		numBuckets = 1
	}

	// Initialize buckets with 'N' (normal)
	buckets := make([]string, numBuckets)
	for i := range buckets {
		buckets[i] = "N"
	}

	// Handle empty transitions (all normal)
	if len(transitions) == 0 {
		return fmt.Sprintf("[%s]", strings.Join(buckets, " "))
	}

	// Determine initial state using LOCF from before window
	currentState := "normal" // Default if no prior history
	for _, t := range transitions {
		if t.Timestamp.Before(startTime) {
			currentState = t.ToState
		} else {
			break
		}
	}

	// Fill buckets using LOCF
	for i := 0; i < numBuckets; i++ {
		bucketStart := startTime.Add(time.Duration(i) * bucketSize)
		bucketEnd := bucketStart.Add(bucketSize)

		// Check if any transitions occur in this bucket
		for _, t := range transitions {
			if !t.Timestamp.Before(bucketStart) && t.Timestamp.Before(bucketEnd) {
				currentState = t.ToState
			}
		}

		// Set bucket symbol based on current state
		buckets[i] = stateToSymbol(currentState)
	}

	return fmt.Sprintf("[%s]", strings.Join(buckets, " "))
}

// stateToSymbol converts state string to compact symbol
func stateToSymbol(state string) string {
	switch state {
	case "firing":
		return "F"
	case "pending":
		return "P"
	case "normal":
		return "N"
	default:
		return "?"
	}
}

// determineCurrentState finds the current alert state from transitions
func determineCurrentState(transitions []StateTransition, currentTime time.Time) string {
	if len(transitions) == 0 {
		return "normal"
	}

	// Find most recent transition at or before currentTime
	currentState := "normal"
	for _, t := range transitions {
		if !t.Timestamp.After(currentTime) {
			currentState = t.ToState
		} else {
			break
		}
	}

	return currentState
}

// calculateFiringDuration calculates how long alert has been firing continuously
func calculateFiringDuration(transitions []StateTransition, currentTime time.Time) string {
	if len(transitions) == 0 {
		return "unknown"
	}

	// Find the most recent transition to "firing"
	var firingStartTime *time.Time
	for i := len(transitions) - 1; i >= 0; i-- {
		t := transitions[i]
		if t.ToState == "firing" {
			firingStartTime = &t.Timestamp
			break
		}
		// If we hit a non-firing state, stop looking
		if t.ToState != "firing" {
			break
		}
	}

	if firingStartTime == nil {
		return "unknown"
	}

	duration := currentTime.Sub(*firingStartTime)
	return formatDuration(duration)
}

// formatDuration formats duration in human-readable format
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd%dh", days, hours)
}

// formatCategory formats alert categories for display
// Combines onset and pattern categories into readable string
func formatCategory(categories AlertCategories, flappinessScore float64) string {
	// Special case: stable-normal onset means never fired
	if len(categories.Onset) == 1 && categories.Onset[0] == "stable-normal" {
		return "stable-normal"
	}

	// Start with onset category (time-based)
	var parts []string
	if len(categories.Onset) > 0 {
		onset := strings.ToUpper(categories.Onset[0])
		parts = append(parts, onset)
	}

	// Add pattern category (behavior-based)
	if len(categories.Pattern) > 0 {
		pattern := categories.Pattern[0]
		// Skip redundant "stable-normal" pattern
		if pattern != "stable-normal" {
			parts = append(parts, pattern)
		}
	}

	if len(parts) == 0 {
		return "unknown"
	}

	return strings.Join(parts, " + ")
}

// alertInfo holds basic alert information from graph query
type alertInfo struct {
	UID       string
	Name      string
	Cluster   string
	Service   string
	Namespace string
}
