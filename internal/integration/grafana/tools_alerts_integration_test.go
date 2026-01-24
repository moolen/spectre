package grafana

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAlertGraphClient implements graph.Client for alert tools testing
// Provides both Alert nodes and STATE_TRANSITION edges
type mockAlertGraphClient struct {
	alerts      map[string]mockAlertNode
	transitions map[string][]StateTransition
	queryCalls  int
}

type mockAlertNode struct {
	UID            string
	Name           string
	State          string
	StateTimestamp time.Time
	Labels         map[string]string
	Annotations    map[string]string
	Condition      string
	Integration    string
}

func newMockAlertGraphClient() *mockAlertGraphClient {
	return &mockAlertGraphClient{
		alerts:      make(map[string]mockAlertNode),
		transitions: make(map[string][]StateTransition),
	}
}

func (m *mockAlertGraphClient) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	m.queryCalls++

	// Detect query type by pattern matching
	if strings.Contains(query.Query, "STATE_TRANSITION") {
		// Return state transitions for specific alert
		uid, ok := query.Parameters["uid"].(string)
		if !ok {
			return &graph.QueryResult{
				Columns: []string{"from_state", "to_state", "timestamp"},
				Rows:    [][]interface{}{},
			}, nil
		}

		transitions, exists := m.transitions[uid]
		if !exists {
			return &graph.QueryResult{
				Columns: []string{"from_state", "to_state", "timestamp"},
				Rows:    [][]interface{}{},
			}, nil
		}

		// Build result rows
		rows := make([][]interface{}, 0)
		for _, t := range transitions {
			rows = append(rows, []interface{}{
				t.FromState,
				t.ToState,
				t.Timestamp.UTC().Format(time.RFC3339),
			})
		}

		return &graph.QueryResult{
			Columns: []string{"from_state", "to_state", "timestamp"},
			Rows:    rows,
		}, nil
	}

	// Detect Alert query for overview tool (uses labels as JSON string)
	if strings.Contains(query.Query, "a.labels") && strings.Contains(query.Query, "a.state") {
		return m.queryAlertsForOverview(query)
	}

	// Detect Alert query for aggregated/details tools (uses separate label columns)
	if strings.Contains(query.Query, "a.uid") {
		return m.queryAlertsForTools(query)
	}

	// Default empty result
	return &graph.QueryResult{
		Columns: []string{},
		Rows:    [][]interface{}{},
	}, nil
}

// queryAlertsForOverview handles overview tool queries (labels as JSON string)
func (m *mockAlertGraphClient) queryAlertsForOverview(query graph.GraphQuery) (*graph.QueryResult, error) {
	integration, _ := query.Parameters["integration"].(string)

	rows := make([][]interface{}, 0)
	for _, alert := range m.alerts {
		// Filter by integration
		if alert.Integration != integration {
			continue
		}

		// Filter by state (firing/pending)
		if !strings.Contains(query.Query, "IN ['firing', 'pending']") {
			continue
		}
		if alert.State != "firing" && alert.State != "pending" {
			continue
		}

		// Apply label filters if present
		if !m.matchesLabelFilters(alert, query.Query) {
			continue
		}

		// Serialize labels as JSON string
		labelsJSON, _ := json.Marshal(alert.Labels)

		rows = append(rows, []interface{}{
			alert.UID,
			alert.Name,
			alert.State,
			alert.StateTimestamp.Format(time.RFC3339),
			string(labelsJSON),
		})
	}

	return &graph.QueryResult{
		Columns: []string{"uid", "title", "state", "state_timestamp", "labels"},
		Rows:    rows,
	}, nil
}

// queryAlertsForTools handles aggregated/details tool queries (separate label columns)
func (m *mockAlertGraphClient) queryAlertsForTools(query graph.GraphQuery) (*graph.QueryResult, error) {
	integration, _ := query.Parameters["integration"].(string)

	// Determine if this is a details query (has annotations/condition)
	isDetails := strings.Contains(query.Query, "a.annotations") || strings.Contains(query.Query, "a.condition")

	rows := make([][]interface{}, 0)
	for _, alert := range m.alerts {
		// Filter by integration
		if alert.Integration != integration {
			continue
		}

		// Apply parameter-based filters
		if uid, ok := query.Parameters["uid"].(string); ok {
			if alert.UID != uid {
				continue
			}
		}
		if severity, ok := query.Parameters["severity"].(string); ok {
			if alert.Labels["severity"] != severity {
				continue
			}
		}
		if cluster, ok := query.Parameters["cluster"].(string); ok {
			if alert.Labels["cluster"] != cluster {
				continue
			}
		}
		if service, ok := query.Parameters["service"].(string); ok {
			if alert.Labels["service"] != service {
				continue
			}
		}
		if namespace, ok := query.Parameters["namespace"].(string); ok {
			if alert.Labels["namespace"] != namespace {
				continue
			}
		}

		if isDetails {
			// Details query format
			labelsJSON, _ := json.Marshal(alert.Labels)
			annotationsJSON, _ := json.Marshal(alert.Annotations)

			rows = append(rows, []interface{}{
				alert.UID,
				alert.Name,
				string(labelsJSON),
				string(annotationsJSON),
				alert.Condition,
			})
		} else {
			// Aggregated query format
			rows = append(rows, []interface{}{
				alert.UID,
				alert.Name,
				alert.Labels["cluster"],
				alert.Labels["service"],
				alert.Labels["namespace"],
			})
		}
	}

	if isDetails {
		return &graph.QueryResult{
			Columns: []string{"uid", "name", "labels", "annotations", "condition"},
			Rows:    rows,
		}, nil
	}

	return &graph.QueryResult{
		Columns: []string{"uid", "name", "cluster", "service", "namespace"},
		Rows:    rows,
	}, nil
}

// matchesLabelFilters checks if alert matches label filters in query string
func (m *mockAlertGraphClient) matchesLabelFilters(alert mockAlertNode, query string) bool {
	// Check cluster filter
	if strings.Contains(query, "a.labels CONTAINS '\"cluster\":") {
		// Extract cluster value from filter (simplified)
		// In real query: a.labels CONTAINS '"cluster":"prod"'
		// We just check if alert has that label value
		if cluster := alert.Labels["cluster"]; cluster == "" {
			return false
		}
	}

	// Check severity filter (case-insensitive)
	if strings.Contains(query, "toLower(a.labels) CONTAINS '\"severity\":") {
		// Extract the severity value from the query
		// Pattern: toLower(a.labels) CONTAINS '"severity":"critical"'
		start := strings.Index(query, "toLower(a.labels) CONTAINS '\"severity\":\"")
		if start != -1 {
			start += len("toLower(a.labels) CONTAINS '\"severity\":\"")
			end := strings.Index(query[start:], "\"")
			if end != -1 {
				wantedSeverity := strings.ToLower(query[start : start+end])
				alertSeverity := strings.ToLower(alert.Labels["severity"])
				if alertSeverity != wantedSeverity {
					return false
				}
			}
		}
	}

	return true
}

func (m *mockAlertGraphClient) Connect(ctx context.Context) error              { return nil }
func (m *mockAlertGraphClient) Close() error                                   { return nil }
func (m *mockAlertGraphClient) Ping(ctx context.Context) error                 { return nil }
func (m *mockAlertGraphClient) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}
func (m *mockAlertGraphClient) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}
func (m *mockAlertGraphClient) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}
func (m *mockAlertGraphClient) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}
func (m *mockAlertGraphClient) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}
func (m *mockAlertGraphClient) InitializeSchema(ctx context.Context) error       { return nil }
func (m *mockAlertGraphClient) DeleteGraph(ctx context.Context) error            { return nil }
func (m *mockAlertGraphClient) CreateGraph(ctx context.Context, graphName string) error { return nil }
func (m *mockAlertGraphClient) DeleteGraphByName(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockAlertGraphClient) GraphExists(ctx context.Context, graphName string) (bool, error) {
	return true, nil
}

// Test AlertsOverviewTool - Groups by severity
func TestAlertsOverviewTool_GroupsBySeverity(t *testing.T) {
	mockGraph := newMockAlertGraphClient()
	logger := logging.GetLogger("test")

	now := time.Now()

	// Create 5 alerts: 2 Critical, 2 Warning, 1 Info
	mockGraph.alerts["alert-1"] = mockAlertNode{
		UID:            "alert-1",
		Name:           "High CPU Usage",
		State:          "firing",
		StateTimestamp: now.Add(-30 * time.Minute),
		Labels: map[string]string{
			"severity": "critical",
			"cluster":  "prod",
		},
		Integration: "test-grafana",
	}
	mockGraph.alerts["alert-2"] = mockAlertNode{
		UID:            "alert-2",
		Name:           "Memory Exhaustion",
		State:          "firing",
		StateTimestamp: now.Add(-1 * time.Hour),
		Labels: map[string]string{
			"severity": "critical",
			"cluster":  "prod",
		},
		Integration: "test-grafana",
	}
	mockGraph.alerts["alert-3"] = mockAlertNode{
		UID:            "alert-3",
		Name:           "High Latency",
		State:          "firing",
		StateTimestamp: now.Add(-15 * time.Minute),
		Labels: map[string]string{
			"severity": "warning",
			"cluster":  "prod",
		},
		Integration: "test-grafana",
	}
	mockGraph.alerts["alert-4"] = mockAlertNode{
		UID:            "alert-4",
		Name:           "Disk Space Low",
		State:          "firing",
		StateTimestamp: now.Add(-2 * time.Hour),
		Labels: map[string]string{
			"severity": "warning",
			"cluster":  "prod",
		},
		Integration: "test-grafana",
	}
	mockGraph.alerts["alert-5"] = mockAlertNode{
		UID:            "alert-5",
		Name:           "Info Alert",
		State:          "firing",
		StateTimestamp: now.Add(-5 * time.Minute),
		Labels: map[string]string{
			"severity": "info",
			"cluster":  "prod",
		},
		Integration: "test-grafana",
	}

	// Create AlertsOverviewTool (without analysis service for this test)
	tool := NewAlertsOverviewTool(mockGraph, "test-grafana", nil, logger)

	// Execute tool
	params := AlertsOverviewParams{}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)
	require.NotNil(t, result)

	response := result.(*AlertsOverviewResponse)

	// Verify groups by severity
	assert.Len(t, response.AlertsBySeverity, 3)
	assert.Equal(t, 2, response.AlertsBySeverity["critical"].Count)
	assert.Equal(t, 2, response.AlertsBySeverity["warning"].Count)
	assert.Equal(t, 1, response.AlertsBySeverity["info"].Count)

	// Verify alert details in each bucket
	assert.Len(t, response.AlertsBySeverity["critical"].Alerts, 2)
	assert.Len(t, response.AlertsBySeverity["warning"].Alerts, 2)
	assert.Len(t, response.AlertsBySeverity["info"].Alerts, 1)
}

// Test AlertsOverviewTool - Filters by severity
func TestAlertsOverviewTool_FiltersBySeverity(t *testing.T) {
	mockGraph := newMockAlertGraphClient()
	logger := logging.GetLogger("test")

	now := time.Now()

	// Create multiple alerts with different severities
	mockGraph.alerts["alert-1"] = mockAlertNode{
		UID:            "alert-1",
		Name:           "Critical Alert",
		State:          "firing",
		StateTimestamp: now.Add(-30 * time.Minute),
		Labels: map[string]string{
			"severity": "critical",
		},
		Integration: "test-grafana",
	}
	mockGraph.alerts["alert-2"] = mockAlertNode{
		UID:            "alert-2",
		Name:           "Warning Alert",
		State:          "firing",
		StateTimestamp: now.Add(-1 * time.Hour),
		Labels: map[string]string{
			"severity": "warning",
		},
		Integration: "test-grafana",
	}

	// Create AlertsOverviewTool
	tool := NewAlertsOverviewTool(mockGraph, "test-grafana", nil, logger)

	// Execute tool with severity filter
	params := AlertsOverviewParams{
		Severity: "critical",
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)

	response := result.(*AlertsOverviewResponse)

	// Verify only critical alerts returned
	assert.Len(t, response.AlertsBySeverity, 1)
	assert.Equal(t, 1, response.AlertsBySeverity["critical"].Count)
	assert.NotContains(t, response.AlertsBySeverity, "warning")

	// Verify filters applied in response
	require.NotNil(t, response.FiltersApplied)
	assert.Equal(t, "critical", response.FiltersApplied.Severity)
}

// Test AlertsOverviewTool - Flappiness count
func TestAlertsOverviewTool_FlappinessCount(t *testing.T) {
	mockGraph := newMockAlertGraphClient()
	logger := logging.GetLogger("test")

	now := time.Now()

	// Create alert with high flappiness
	mockGraph.alerts["alert-flapping"] = mockAlertNode{
		UID:            "alert-flapping",
		Name:           "Flapping Alert",
		State:          "firing",
		StateTimestamp: now.Add(-1 * time.Hour),
		Labels: map[string]string{
			"severity": "critical",
		},
		Integration: "test-grafana",
	}

	// Create many transitions to trigger high flappiness (>0.7)
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-3 * 24 * time.Hour)},
	}
	// Add 10 state changes in last 6 hours
	for i := 0; i < 10; i++ {
		offset := time.Duration(i) * 30 * time.Minute
		if i%2 == 0 {
			transitions = append(transitions, StateTransition{
				FromState: "firing",
				ToState:   "normal",
				Timestamp: now.Add(-6*time.Hour + offset),
			})
		} else {
			transitions = append(transitions, StateTransition{
				FromState: "normal",
				ToState:   "firing",
				Timestamp: now.Add(-6*time.Hour + offset),
			})
		}
	}
	mockGraph.transitions["alert-flapping"] = transitions

	// Create analysis service with mock graph
	analysisService := NewAlertAnalysisService(mockGraph, "test-grafana", logger)

	// Create AlertsOverviewTool with analysis service
	tool := NewAlertsOverviewTool(mockGraph, "test-grafana", analysisService, logger)

	// Execute tool
	params := AlertsOverviewParams{}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)

	response := result.(*AlertsOverviewResponse)

	// Verify flapping_count is incremented
	assert.Equal(t, 1, response.AlertsBySeverity["critical"].FlappingCount)
}

// Test AlertsOverviewTool - Nil analysis service
func TestAlertsOverviewTool_NilAnalysisService(t *testing.T) {
	mockGraph := newMockAlertGraphClient()
	logger := logging.GetLogger("test")

	now := time.Now()

	// Create alert
	mockGraph.alerts["alert-1"] = mockAlertNode{
		UID:            "alert-1",
		Name:           "Test Alert",
		State:          "firing",
		StateTimestamp: now.Add(-30 * time.Minute),
		Labels: map[string]string{
			"severity": "critical",
		},
		Integration: "test-grafana",
	}

	// Create tool with nil analysis service (graph disabled scenario)
	tool := NewAlertsOverviewTool(mockGraph, "test-grafana", nil, logger)

	// Execute tool
	params := AlertsOverviewParams{}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)

	response := result.(*AlertsOverviewResponse)

	// Verify basic functionality works
	assert.Equal(t, 1, response.AlertsBySeverity["critical"].Count)
	// Flapping count should be 0 (no analysis service)
	assert.Equal(t, 0, response.AlertsBySeverity["critical"].FlappingCount)
}

// Test AlertsAggregatedTool - State timeline bucketization
func TestAlertsAggregatedTool_StateTimeline(t *testing.T) {
	mockGraph := newMockAlertGraphClient()
	logger := logging.GetLogger("test")

	now := time.Now()

	// Create alert
	mockGraph.alerts["alert-1"] = mockAlertNode{
		UID:   "alert-1",
		Name:  "Test Alert",
		State: "firing",
		Labels: map[string]string{
			"severity": "critical",
			"cluster":  "prod",
		},
		Integration: "test-grafana",
	}

	// Create transitions: N→F (10:00), F→N (10:30), N→F (10:40)
	// Simulating transitions within 1 hour window
	mockGraph.transitions["alert-1"] = []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-60 * time.Minute)}, // Bucket 0
		{FromState: "firing", ToState: "normal", Timestamp: now.Add(-30 * time.Minute)}, // Bucket 3
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-20 * time.Minute)}, // Bucket 4
	}

	// Create tool (no analysis service needed for timeline test)
	tool := NewAlertsAggregatedTool(mockGraph, "test-grafana", nil, logger)

	// Execute tool with 1h lookback
	params := AlertsAggregatedParams{
		Lookback: "1h",
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)

	response := result.(*AlertsAggregatedResponse)

	// Verify timeline is present and formatted correctly
	require.Len(t, response.Alerts, 1)
	alert := response.Alerts[0]

	// Timeline should be in format "[F F F N N F]" or similar
	assert.Contains(t, alert.Timeline, "[")
	assert.Contains(t, alert.Timeline, "]")
	assert.Contains(t, alert.Timeline, "F") // Should have firing states
	assert.Contains(t, alert.Timeline, "N") // Should have normal states

	// Verify timeline has 6 buckets (1h / 10min = 6)
	buckets := strings.Split(strings.Trim(alert.Timeline, "[]"), " ")
	assert.Len(t, buckets, 6)
}

// Test AlertsAggregatedTool - Category enrichment
func TestAlertsAggregatedTool_CategoryEnrichment(t *testing.T) {
	mockGraph := newMockAlertGraphClient()
	logger := logging.GetLogger("test")

	now := time.Now()

	// Create alert
	mockGraph.alerts["alert-chronic"] = mockAlertNode{
		UID:   "alert-chronic",
		Name:  "Chronic Alert",
		State: "firing",
		Labels: map[string]string{
			"severity": "critical",
			"cluster":  "prod",
		},
		Integration: "test-grafana",
	}

	// Create chronic pattern (firing for >7 days, >80% time)
	mockGraph.transitions["alert-chronic"] = []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-8 * 24 * time.Hour)},
		// Brief normal period
		{FromState: "firing", ToState: "normal", Timestamp: now.Add(-7*24*time.Hour - 1*time.Hour)},
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-7 * 24 * time.Hour)},
		// Firing for rest of 7 days
	}

	// Create analysis service
	analysisService := NewAlertAnalysisService(mockGraph, "test-grafana", logger)

	// Create tool with analysis service
	tool := NewAlertsAggregatedTool(mockGraph, "test-grafana", analysisService, logger)

	// Execute tool
	params := AlertsAggregatedParams{
		Lookback: "1h",
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)

	response := result.(*AlertsAggregatedResponse)

	// Verify category enrichment
	require.Len(t, response.Alerts, 1)
	alert := response.Alerts[0]

	// Should have category format: "CHRONIC + stable-firing" or similar
	assert.Contains(t, strings.ToLower(alert.Category), "chronic")
	assert.NotEmpty(t, alert.Category)
}

// Test AlertsAggregatedTool - Insufficient data handling
func TestAlertsAggregatedTool_InsufficientData(t *testing.T) {
	mockGraph := newMockAlertGraphClient()
	logger := logging.GetLogger("test")

	now := time.Now()

	// Create new alert with no history
	mockGraph.alerts["alert-new"] = mockAlertNode{
		UID:   "alert-new",
		Name:  "New Alert",
		State: "firing",
		Labels: map[string]string{
			"severity": "critical",
			"cluster":  "prod",
		},
		Integration: "test-grafana",
	}

	// Only 12h of history (< 24h minimum)
	mockGraph.transitions["alert-new"] = []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-12 * time.Hour)},
	}

	// Create analysis service
	analysisService := NewAlertAnalysisService(mockGraph, "test-grafana", logger)

	// Create tool with analysis service
	tool := NewAlertsAggregatedTool(mockGraph, "test-grafana", analysisService, logger)

	// Execute tool
	params := AlertsAggregatedParams{
		Lookback: "1h",
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)

	response := result.(*AlertsAggregatedResponse)

	// Verify category shows "new (insufficient history)"
	require.Len(t, response.Alerts, 1)
	alert := response.Alerts[0]

	assert.Equal(t, "new (insufficient history)", alert.Category)
	assert.Equal(t, 0.0, alert.FlappinessScore)
}

// Test AlertsDetailsTool - Full history returned
func TestAlertsDetailsTool_FullHistory(t *testing.T) {
	mockGraph := newMockAlertGraphClient()
	logger := logging.GetLogger("test")

	now := time.Now()

	// Create alert with full metadata
	mockGraph.alerts["alert-1"] = mockAlertNode{
		UID:   "alert-1",
		Name:  "Test Alert",
		State: "firing",
		Labels: map[string]string{
			"severity": "critical",
			"cluster":  "prod",
			"service":  "api",
		},
		Annotations: map[string]string{
			"summary":     "High CPU usage",
			"description": "CPU usage above 80%",
		},
		Condition:   "avg(cpu_usage) > 80",
		Integration: "test-grafana",
	}

	// Create 7-day state history
	mockGraph.transitions["alert-1"] = []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-7 * 24 * time.Hour)},
		{FromState: "firing", ToState: "normal", Timestamp: now.Add(-6 * 24 * time.Hour)},
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-5 * 24 * time.Hour)},
		{FromState: "firing", ToState: "normal", Timestamp: now.Add(-4 * 24 * time.Hour)},
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-3 * 24 * time.Hour)},
	}

	// Create tool
	tool := NewAlertsDetailsTool(mockGraph, "test-grafana", nil, logger)

	// Execute tool with alert_uid
	params := AlertsDetailsParams{
		AlertUID: "alert-1",
	}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)
	require.NoError(t, err)

	response := result.(*AlertsDetailsResponse)

	// Verify full details returned
	require.Len(t, response.Alerts, 1)
	alert := response.Alerts[0]

	assert.Equal(t, "Test Alert", alert.Name)
	assert.Equal(t, "alert-1", alert.UID)
	assert.Equal(t, "critical", alert.Labels["severity"])
	assert.Equal(t, "High CPU usage", alert.Annotations["summary"])
	assert.Equal(t, "avg(cpu_usage) > 80", alert.RuleDefinition)

	// Verify state timeline
	assert.Len(t, alert.StateTimeline, 5) // 5 transitions
	for _, sp := range alert.StateTimeline {
		assert.NotEmpty(t, sp.Timestamp)
		assert.NotEmpty(t, sp.FromState)
		assert.NotEmpty(t, sp.ToState)
		assert.NotEmpty(t, sp.DurationInState)
	}
}

// Test AlertsDetailsTool - Requires filter or UID
func TestAlertsDetailsTool_RequiresFilterOrUID(t *testing.T) {
	mockGraph := newMockAlertGraphClient()
	logger := logging.GetLogger("test")

	// Create tool
	tool := NewAlertsDetailsTool(mockGraph, "test-grafana", nil, logger)

	// Execute tool without any parameters
	params := AlertsDetailsParams{}
	paramsJSON, _ := json.Marshal(params)

	result, err := tool.Execute(context.Background(), paramsJSON)

	// Should return error
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "must provide alert_uid or at least one filter")
}

// Test Progressive Disclosure Workflow (end-to-end)
func TestAlertsProgressiveDisclosure(t *testing.T) {
	mockGraph := newMockAlertGraphClient()
	logger := logging.GetLogger("test")

	now := time.Now()

	// Setup: 5 alerts (2 Critical/1 flapping, 2 Warning, 1 Info)
	mockGraph.alerts["alert-critical-1"] = mockAlertNode{
		UID:            "alert-critical-1",
		Name:           "Critical Alert 1",
		State:          "firing",
		StateTimestamp: now.Add(-1 * time.Hour),
		Labels: map[string]string{
			"severity":  "critical",
			"cluster":   "prod",
			"service":   "api",
			"namespace": "default",
		},
		Annotations: map[string]string{
			"summary": "High CPU",
		},
		Condition:   "cpu > 90",
		Integration: "test-grafana",
	}

	mockGraph.alerts["alert-critical-flapping"] = mockAlertNode{
		UID:            "alert-critical-flapping",
		Name:           "Critical Flapping Alert",
		State:          "firing",
		StateTimestamp: now.Add(-2 * time.Hour),
		Labels: map[string]string{
			"severity":  "critical",
			"cluster":   "prod",
			"service":   "web",
			"namespace": "default",
		},
		Integration: "test-grafana",
	}

	mockGraph.alerts["alert-warning-1"] = mockAlertNode{
		UID:            "alert-warning-1",
		Name:           "Warning Alert 1",
		State:          "firing",
		StateTimestamp: now.Add(-30 * time.Minute),
		Labels: map[string]string{
			"severity": "warning",
			"cluster":  "prod",
		},
		Integration: "test-grafana",
	}

	mockGraph.alerts["alert-warning-2"] = mockAlertNode{
		UID:            "alert-warning-2",
		Name:           "Warning Alert 2",
		State:          "firing",
		StateTimestamp: now.Add(-15 * time.Minute),
		Labels: map[string]string{
			"severity": "warning",
			"cluster":  "prod",
		},
		Integration: "test-grafana",
	}

	mockGraph.alerts["alert-info"] = mockAlertNode{
		UID:            "alert-info",
		Name:           "Info Alert",
		State:          "firing",
		StateTimestamp: now.Add(-5 * time.Minute),
		Labels: map[string]string{
			"severity": "info",
			"cluster":  "prod",
		},
		Integration: "test-grafana",
	}

	// Setup transitions for flapping alert
	transitions := []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-3 * 24 * time.Hour)},
	}
	// Add 12 state changes in last 6 hours (flapping pattern)
	for i := 0; i < 12; i++ {
		offset := time.Duration(i) * 30 * time.Minute
		if i%2 == 0 {
			transitions = append(transitions, StateTransition{
				FromState: "firing",
				ToState:   "normal",
				Timestamp: now.Add(-6*time.Hour + offset),
			})
		} else {
			transitions = append(transitions, StateTransition{
				FromState: "normal",
				ToState:   "firing",
				Timestamp: now.Add(-6*time.Hour + offset),
			})
		}
	}
	mockGraph.transitions["alert-critical-flapping"] = transitions

	// Setup stable transitions for other critical alert
	mockGraph.transitions["alert-critical-1"] = []StateTransition{
		{FromState: "normal", ToState: "firing", Timestamp: now.Add(-2 * 24 * time.Hour)},
	}

	// Create analysis service
	analysisService := NewAlertAnalysisService(mockGraph, "test-grafana", logger)

	// Step 1: Call OverviewTool with no filters
	overviewTool := NewAlertsOverviewTool(mockGraph, "test-grafana", analysisService, logger)
	overviewParams := AlertsOverviewParams{}
	overviewParamsJSON, _ := json.Marshal(overviewParams)

	overviewResult, err := overviewTool.Execute(context.Background(), overviewParamsJSON)
	require.NoError(t, err)

	overviewResponse := overviewResult.(*AlertsOverviewResponse)

	// Verify counts by severity
	assert.Equal(t, 2, overviewResponse.AlertsBySeverity["critical"].Count)
	assert.Equal(t, 2, overviewResponse.AlertsBySeverity["warning"].Count)
	assert.Equal(t, 1, overviewResponse.AlertsBySeverity["info"].Count)

	// Verify flapping count shows 1 for Critical
	assert.Equal(t, 1, overviewResponse.AlertsBySeverity["critical"].FlappingCount)

	// Step 2: Call AggregatedTool with severity="critical"
	aggregatedTool := NewAlertsAggregatedTool(mockGraph, "test-grafana", analysisService, logger)
	aggregatedParams := AlertsAggregatedParams{
		Lookback: "1h",
		Severity: "critical",
	}
	aggregatedParamsJSON, _ := json.Marshal(aggregatedParams)

	aggregatedResult, err := aggregatedTool.Execute(context.Background(), aggregatedParamsJSON)
	require.NoError(t, err)

	aggregatedResponse := aggregatedResult.(*AlertsAggregatedResponse)

	// Verify returns 2 Critical alerts with timelines
	assert.Len(t, aggregatedResponse.Alerts, 2)

	// Find the flapping alert
	var flappingAlert *AggregatedAlert
	for i := range aggregatedResponse.Alerts {
		if aggregatedResponse.Alerts[i].Name == "Critical Flapping Alert" {
			flappingAlert = &aggregatedResponse.Alerts[i]
			break
		}
	}
	require.NotNil(t, flappingAlert)

	// Verify timeline present
	assert.Contains(t, flappingAlert.Timeline, "[")
	assert.Contains(t, flappingAlert.Timeline, "]")

	// Verify category enrichment
	assert.NotEmpty(t, flappingAlert.Category)

	// Verify flappiness score
	assert.Greater(t, flappingAlert.FlappinessScore, 0.7)

	// Step 3: Call DetailsTool with alert_uid of the flapping alert
	detailsTool := NewAlertsDetailsTool(mockGraph, "test-grafana", analysisService, logger)
	detailsParams := AlertsDetailsParams{
		AlertUID: "alert-critical-flapping",
	}
	detailsParamsJSON, _ := json.Marshal(detailsParams)

	detailsResult, err := detailsTool.Execute(context.Background(), detailsParamsJSON)
	require.NoError(t, err)

	detailsResponse := detailsResult.(*AlertsDetailsResponse)

	// Verify full 7-day history returned
	require.Len(t, detailsResponse.Alerts, 1)
	detailAlert := detailsResponse.Alerts[0]

	assert.Equal(t, "Critical Flapping Alert", detailAlert.Name)
	assert.Len(t, detailAlert.StateTimeline, len(transitions))

	// Verify analysis section populated
	require.NotNil(t, detailAlert.Analysis)
	assert.Greater(t, detailAlert.Analysis.FlappinessScore, 0.7)
	assert.NotEmpty(t, detailAlert.Analysis.Category)

	// Verify progressive disclosure: response sizes increase
	// Overview: minimal (just counts)
	// Aggregated: compact timelines
	// Details: full history and metadata

	t.Logf("Progressive disclosure verified:")
	t.Logf("  Step 1 (Overview): %d severity buckets", len(overviewResponse.AlertsBySeverity))
	t.Logf("  Step 2 (Aggregated): %d alerts with timelines", len(aggregatedResponse.Alerts))
	t.Logf("  Step 3 (Details): %d alerts with full history", len(detailsResponse.Alerts))
}
