package grafana

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// TestGrafanaIntegration_WithGraphClient tests the full lifecycle with graph client
func TestGrafanaIntegration_WithGraphClient(t *testing.T) {
	// Create integration
	config := map[string]interface{}{
		"url": "https://grafana.example.com",
	}

	integration, err := NewGrafanaIntegration("test-grafana", config)
	if err != nil {
		t.Fatalf("Failed to create integration: %v", err)
	}

	grafana := integration.(*GrafanaIntegration)

	// Set mock graph client
	mockGraph := newMockGraphClient()
	grafana.SetGraphClient(mockGraph)

	// Verify graph client was set
	if grafana.graphClient == nil {
		t.Error("Expected graph client to be set")
	}

	// Note: We don't actually start the integration in this test because it would
	// try to connect to Grafana and create a SecretWatcher. This test validates
	// that the graph client can be set and the integration structure is correct.
}

// TestGrafanaIntegration_WithoutGraphClient tests lifecycle without graph client
func TestGrafanaIntegration_WithoutGraphClient(t *testing.T) {
	// Create integration
	config := map[string]interface{}{
		"url": "https://grafana.example.com",
	}

	integration, err := NewGrafanaIntegration("test-grafana", config)
	if err != nil {
		t.Fatalf("Failed to create integration: %v", err)
	}

	grafana := integration.(*GrafanaIntegration)

	// Don't set graph client - verify it's nil
	if grafana.graphClient != nil {
		t.Error("Expected graph client to be nil initially")
	}

	// Integration should still be creatable without graph client
	// (dashboard sync will be disabled, but integration still works)
}

// TestDashboardSyncerLifecycle tests the syncer start/stop within integration context
func TestDashboardSyncerLifecycle(t *testing.T) {
	// This is more of a documentation test showing the expected usage pattern
	// In production, the integration manager would:
	// 1. Create the integration via factory
	// 2. Call SetGraphClient with the manager's graph client
	// 3. Call Start() which initializes the syncer

	mockGrafana := newMockGrafanaClient()
	mockGrafana.dashboards = []DashboardMeta{}
	mockGrafana.dashboardData = make(map[string]map[string]interface{})

	mockGraph := newMockGraphClient()
	mockGraph.results[""] = &graph.QueryResult{Rows: [][]interface{}{}}

	logger := logging.GetLogger("test")

	// Create syncer directly (bypass integration for this focused test)
	syncer := NewDashboardSyncer(mockGrafana, mockGraph, nil, "test-integration", 100*time.Millisecond, logger)

	ctx := context.Background()
	err := syncer.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start syncer: %v", err)
	}

	// Verify initial sync completed
	syncStatus := syncer.GetSyncStatus()
	if syncStatus.LastSyncTime == nil {
		t.Error("Expected lastSyncTime to be set")
	}
	if syncStatus.LastError != "" {
		t.Errorf("Expected no error, got: %v", syncStatus.LastError)
	}
	if syncStatus.DashboardCount != 0 {
		t.Errorf("Expected 0 dashboards, got %d", syncStatus.DashboardCount)
	}

	// Let syncer run for a bit
	time.Sleep(150 * time.Millisecond)

	// Stop syncer
	syncer.Stop()

	// Verify stopped
	select {
	case <-syncer.stopped:
		// Good - stopped channel closed
	case <-time.After(1 * time.Second):
		t.Error("Syncer did not stop within timeout")
	}
}

// mockGraphClientForAnalysis implements graph.Client for alert analysis testing
type mockGraphClientForAnalysis struct {
	transitions      []StateTransition
	queryCalls       int
	returnError      bool
	executeQueryFunc func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error)
}

func (m *mockGraphClientForAnalysis) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	m.queryCalls++

	if m.executeQueryFunc != nil {
		return m.executeQueryFunc(ctx, query)
	}

	if m.returnError {
		return nil, fmt.Errorf("mock error")
	}

	// Detect STATE_TRANSITION query by checking query content
	if containsStateTransition := query.Query != "" &&
		(query.Query[0] == '\n' || query.Query[0] == ' ' || query.Query[0] == 'M'); containsStateTransition {
		// Build result rows from mock transitions
		rows := make([][]interface{}, len(m.transitions))
		for i, t := range m.transitions {
			rows[i] = []interface{}{
				t.FromState,
				t.ToState,
				t.Timestamp.UTC().Format(time.RFC3339),
			}
		}
		return &graph.QueryResult{Rows: rows}, nil
	}

	return &graph.QueryResult{Rows: [][]interface{}{}}, nil
}

func (m *mockGraphClientForAnalysis) Close() error                                 { return nil }
func (m *mockGraphClientForAnalysis) Connect(ctx context.Context) error            { return nil }
func (m *mockGraphClientForAnalysis) Ping(ctx context.Context) error               { return nil }
func (m *mockGraphClientForAnalysis) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}
func (m *mockGraphClientForAnalysis) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}
func (m *mockGraphClientForAnalysis) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}
func (m *mockGraphClientForAnalysis) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}
func (m *mockGraphClientForAnalysis) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}
func (m *mockGraphClientForAnalysis) InitializeSchema(ctx context.Context) error { return nil }
func (m *mockGraphClientForAnalysis) DeleteGraph(ctx context.Context) error      { return nil }
func (m *mockGraphClientForAnalysis) CreateGraph(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockGraphClientForAnalysis) DeleteGraphByName(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockGraphClientForAnalysis) GraphExists(ctx context.Context, graphName string) (bool, error) {
	return true, nil
}

// TestGrafanaIntegration_AlertAnalysis_FullHistory tests analysis with 7 days of stable firing
func TestGrafanaIntegration_AlertAnalysis_FullHistory(t *testing.T) {
	logger := logging.GetLogger("test.alert_analysis")

	// Create mock transitions for 7 days of stable firing
	now := time.Now()
	transitions := []StateTransition{
		{FromState: "unknown", ToState: "firing", Timestamp: now.Add(-7 * 24 * time.Hour)},
		// No other transitions - stable firing for 7 days
	}

	mockGraph := &mockGraphClientForAnalysis{
		transitions: transitions,
	}

	// Create alert analysis service
	service := NewAlertAnalysisService(mockGraph, "test-integration", logger)

	// Analyze alert
	ctx := context.Background()
	result, err := service.AnalyzeAlert(ctx, "test-alert-stable")
	if err != nil {
		t.Fatalf("AnalyzeAlert failed: %v", err)
	}

	// Verify flappiness score is low (stable alert)
	if result.FlappinessScore > 0.3 {
		t.Errorf("Expected low flappiness score for stable alert, got %.2f", result.FlappinessScore)
	}

	// Verify categories include chronic (>7d firing)
	hasChronicOnset := false
	for _, cat := range result.Categories.Onset {
		if cat == "chronic" {
			hasChronicOnset = true
			break
		}
	}
	if !hasChronicOnset {
		t.Errorf("Expected 'chronic' onset category, got %v", result.Categories.Onset)
	}

	// Verify categories include stable-firing pattern
	hasStableFiring := false
	for _, cat := range result.Categories.Pattern {
		if cat == "stable-firing" {
			hasStableFiring = true
			break
		}
	}
	if !hasStableFiring {
		t.Errorf("Expected 'stable-firing' pattern category, got %v", result.Categories.Pattern)
	}

	// Verify baseline is present
	if result.Baseline.PercentFiring == 0 {
		t.Error("Expected non-zero firing percentage in baseline")
	}
}

// TestGrafanaIntegration_AlertAnalysis_Flapping tests analysis with flapping pattern
func TestGrafanaIntegration_AlertAnalysis_Flapping(t *testing.T) {
	logger := logging.GetLogger("test.alert_analysis")

	// Create mock transitions with 10+ state changes in 6h window
	now := time.Now()
	transitions := []StateTransition{
		{FromState: "unknown", ToState: "normal", Timestamp: now.Add(-7 * 24 * time.Hour)},
	}

	// Add 12 state changes in last 6 hours (flapping pattern)
	for i := 0; i < 12; i++ {
		offset := time.Duration(i) * 30 * time.Minute
		if i%2 == 0 {
			transitions = append(transitions, StateTransition{
				FromState: "normal",
				ToState:   "firing",
				Timestamp: now.Add(-6*time.Hour + offset),
			})
		} else {
			transitions = append(transitions, StateTransition{
				FromState: "firing",
				ToState:   "normal",
				Timestamp: now.Add(-6*time.Hour + offset),
			})
		}
	}

	mockGraph := &mockGraphClientForAnalysis{
		transitions: transitions,
	}

	// Create alert analysis service
	service := NewAlertAnalysisService(mockGraph, "test-integration", logger)

	// Analyze alert
	ctx := context.Background()
	result, err := service.AnalyzeAlert(ctx, "test-alert-flapping")
	if err != nil {
		t.Fatalf("AnalyzeAlert failed: %v", err)
	}

	// Verify flappiness score is high (>0.7)
	if result.FlappinessScore <= 0.7 {
		t.Errorf("Expected high flappiness score (>0.7), got %.2f", result.FlappinessScore)
	}

	// Verify categories include "flapping" pattern
	hasFlapping := false
	for _, cat := range result.Categories.Pattern {
		if cat == "flapping" {
			hasFlapping = true
			break
		}
	}
	if !hasFlapping {
		t.Errorf("Expected 'flapping' pattern category, got %v", result.Categories.Pattern)
	}
}

// TestGrafanaIntegration_AlertAnalysis_InsufficientData tests handling of insufficient data
func TestGrafanaIntegration_AlertAnalysis_InsufficientData(t *testing.T) {
	logger := logging.GetLogger("test.alert_analysis")

	// Create mock transitions spanning only 12h (< 24h minimum)
	now := time.Now()
	transitions := []StateTransition{
		{FromState: "unknown", ToState: "firing", Timestamp: now.Add(-12 * time.Hour)},
	}

	mockGraph := &mockGraphClientForAnalysis{
		transitions: transitions,
	}

	// Create alert analysis service
	service := NewAlertAnalysisService(mockGraph, "test-integration", logger)

	// Analyze alert
	ctx := context.Background()
	result, err := service.AnalyzeAlert(ctx, "test-alert-insufficient")

	// Verify returns ErrInsufficientData
	if err == nil {
		t.Fatal("Expected ErrInsufficientData, got nil")
	}

	insufficientErr, ok := err.(ErrInsufficientData)
	if !ok {
		t.Fatalf("Expected ErrInsufficientData, got %T: %v", err, err)
	}

	// Verify error contains duration info
	if insufficientErr.Available >= 24*time.Hour {
		t.Errorf("Expected available < 24h, got %v", insufficientErr.Available)
	}
	if insufficientErr.Required != 24*time.Hour {
		t.Errorf("Expected required = 24h, got %v", insufficientErr.Required)
	}

	// Verify result is nil
	if result != nil {
		t.Error("Expected nil result for insufficient data")
	}
}

// TestGrafanaIntegration_AlertAnalysis_Cache tests cache behavior
func TestGrafanaIntegration_AlertAnalysis_Cache(t *testing.T) {
	logger := logging.GetLogger("test.alert_analysis")

	// Create mock transitions for 7 days of stable firing
	now := time.Now()
	transitions := []StateTransition{
		{FromState: "unknown", ToState: "firing", Timestamp: now.Add(-7 * 24 * time.Hour)},
	}

	mockGraph := &mockGraphClientForAnalysis{
		transitions: transitions,
	}

	// Create alert analysis service
	service := NewAlertAnalysisService(mockGraph, "test-integration", logger)

	// First call - should query graph
	ctx := context.Background()
	result1, err := service.AnalyzeAlert(ctx, "test-alert-cache")
	if err != nil {
		t.Fatalf("First AnalyzeAlert failed: %v", err)
	}

	initialQueryCount := mockGraph.queryCalls

	// Second call - should use cache (within 5 minutes)
	result2, err := service.AnalyzeAlert(ctx, "test-alert-cache")
	if err != nil {
		t.Fatalf("Second AnalyzeAlert failed: %v", err)
	}

	// Verify query count didn't increase (cache hit)
	if mockGraph.queryCalls != initialQueryCount {
		t.Errorf("Expected cache hit (no new queries), but query count increased from %d to %d",
			initialQueryCount, mockGraph.queryCalls)
	}

	// Verify both results have same ComputedAt timestamp
	if !result1.ComputedAt.Equal(result2.ComputedAt) {
		t.Errorf("Expected same ComputedAt for cached result, got %v and %v",
			result1.ComputedAt, result2.ComputedAt)
	}
}

// TestGrafanaIntegration_Lifecycle_AnalysisService tests service lifecycle integration
func TestGrafanaIntegration_Lifecycle_AnalysisService(t *testing.T) {
	// Create integration
	config := map[string]interface{}{
		"url": "https://grafana.example.com",
	}

	integration, err := NewGrafanaIntegration("test-grafana", config)
	if err != nil {
		t.Fatalf("Failed to create integration: %v", err)
	}

	grafana := integration.(*GrafanaIntegration)

	// Set mock graph client
	mockGraph := &mockGraphClientForAnalysis{
		transitions: []StateTransition{},
	}
	grafana.SetGraphClient(mockGraph)

	// Before Start, analysis service should be nil
	if grafana.GetAnalysisService() != nil {
		t.Error("Expected analysis service to be nil before Start")
	}

	// Note: We can't actually call Start() in this test because it would try to
	// connect to Grafana and create a SecretWatcher. Instead, we test the service
	// creation directly.

	logger := logging.GetLogger("test")
	grafana.analysisService = NewAlertAnalysisService(mockGraph, "test-grafana", logger)

	// After manual initialization, service should be non-nil
	service := grafana.GetAnalysisService()
	if service == nil {
		t.Fatal("Expected analysis service to be non-nil after initialization")
	}

	// Verify service has correct integration name
	if service.integrationName != "test-grafana" {
		t.Errorf("Expected integrationName 'test-grafana', got %s", service.integrationName)
	}

	// Simulate Stop - clear service
	grafana.analysisService = nil

	// After Stop, service should be nil
	if grafana.GetAnalysisService() != nil {
		t.Error("Expected analysis service to be nil after Stop")
	}
}
