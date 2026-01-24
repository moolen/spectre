package grafana

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// mockGrafanaClientForAlerts implements GrafanaClientInterface for testing
type mockGrafanaClientForAlerts struct {
	listAlertRulesFunc func(ctx context.Context) ([]AlertRule, error)
}

func (m *mockGrafanaClientForAlerts) ListDashboards(ctx context.Context) ([]DashboardMeta, error) {
	return nil, nil
}

func (m *mockGrafanaClientForAlerts) GetDashboard(ctx context.Context, uid string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockGrafanaClientForAlerts) ListAlertRules(ctx context.Context) ([]AlertRule, error) {
	if m.listAlertRulesFunc != nil {
		return m.listAlertRulesFunc(ctx)
	}
	return nil, nil
}

func (m *mockGrafanaClientForAlerts) GetAlertStates(ctx context.Context) ([]AlertState, error) {
	return nil, nil
}

// mockGraphClientForAlerts implements graph.Client for testing
type mockGraphClientForAlerts struct {
	executeQueryFunc func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error)
}

func (m *mockGraphClientForAlerts) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	if m.executeQueryFunc != nil {
		return m.executeQueryFunc(ctx, query)
	}
	return &graph.QueryResult{Rows: [][]interface{}{}}, nil
}

func (m *mockGraphClientForAlerts) Close() error {
	return nil
}

func (m *mockGraphClientForAlerts) Connect(ctx context.Context) error {
	return nil
}

func (m *mockGraphClientForAlerts) Ping(ctx context.Context) error {
	return nil
}

func (m *mockGraphClientForAlerts) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}

func (m *mockGraphClientForAlerts) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}

func (m *mockGraphClientForAlerts) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}

func (m *mockGraphClientForAlerts) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}

func (m *mockGraphClientForAlerts) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}

func (m *mockGraphClientForAlerts) InitializeSchema(ctx context.Context) error {
	return nil
}

func (m *mockGraphClientForAlerts) DeleteGraph(ctx context.Context) error {
	return nil
}

func (m *mockGraphClientForAlerts) CreateGraph(ctx context.Context, graphName string) error {
	return nil
}

func (m *mockGraphClientForAlerts) DeleteGraphByName(ctx context.Context, graphName string) error {
	return nil
}

func (m *mockGraphClientForAlerts) GraphExists(ctx context.Context, graphName string) (bool, error) {
	return true, nil
}

func TestAlertSyncer_NewAlertRule(t *testing.T) {
	// Test that new alert rules (not in graph) are synced without errors

	// Create mock alert rule with PromQL query
	alertRule := AlertRule{
		UID:       "test-alert-1",
		Title:     "Test Alert",
		Updated:   time.Now(),
		FolderUID: "folder-1",
		RuleGroup: "group-1",
		Data: []AlertQuery{
			{
				RefID:     "A",
				QueryType: "prometheus",
				Model:     []byte(`{"expr": "rate(http_requests_total[5m])"}`),
			},
		},
	}

	// Mock client returns one alert rule
	mockClient := &mockGrafanaClientForAlerts{
		listAlertRulesFunc: func(ctx context.Context) ([]AlertRule, error) {
			return []AlertRule{alertRule}, nil
		},
	}

	// Mock graph client returns empty result (alert not found), then accepts creates
	mockGraph := &mockGraphClientForAlerts{
		executeQueryFunc: func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
			// Return empty for MATCH queries (alert not found)
			return &graph.QueryResult{Rows: [][]interface{}{}}, nil
		},
	}

	// Create builder
	mockBuilder := NewGraphBuilder(mockGraph, nil, "test-integration", logging.GetLogger("test.graphbuilder"))

	// Create syncer
	logger := logging.GetLogger("test.alertsyncer")
	syncer := NewAlertSyncer(mockClient, mockGraph, mockBuilder, "test-integration", logger)

	// Run sync - should complete without errors
	if err := syncer.syncAlerts(); err != nil {
		t.Fatalf("syncAlerts failed: %v", err)
	}
}

func TestAlertSyncer_UpdatedAlertRule(t *testing.T) {
	// Test that updated alert rules (newer timestamp) trigger sync

	oldTime := time.Date(2026, 1, 20, 10, 0, 0, 0, time.UTC)
	newTime := time.Date(2026, 1, 23, 10, 0, 0, 0, time.UTC)

	// Create mock alert rule with new timestamp
	alertRule := AlertRule{
		UID:       "test-alert-2",
		Title:     "Test Alert",
		Updated:   newTime,
		FolderUID: "folder-1",
		RuleGroup: "group-1",
		Data: []AlertQuery{
			{
				RefID:     "A",
				QueryType: "prometheus",
				Model:     []byte(`{"expr": "up"}`),
			},
		},
	}

	// Mock client returns one alert rule
	mockClient := &mockGrafanaClientForAlerts{
		listAlertRulesFunc: func(ctx context.Context) ([]AlertRule, error) {
			return []AlertRule{alertRule}, nil
		},
	}

	// Mock graph client returns old timestamp
	mockGraph := &mockGraphClientForAlerts{
		executeQueryFunc: func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
			// Return old timestamp for needsSync check
			return &graph.QueryResult{
				Rows: [][]interface{}{
					{oldTime.Format(time.RFC3339)},
				},
			}, nil
		},
	}

	// Create builder
	mockBuilder := NewGraphBuilder(mockGraph, nil, "test-integration", logging.GetLogger("test.graphbuilder"))

	// Create syncer
	logger := logging.GetLogger("test.alertsyncer")
	syncer := NewAlertSyncer(mockClient, mockGraph, mockBuilder, "test-integration", logger)

	// Run sync - should complete without errors
	if err := syncer.syncAlerts(); err != nil {
		t.Fatalf("syncAlerts failed: %v", err)
	}
}

func TestAlertSyncer_UnchangedAlertRule(t *testing.T) {
	// Test that unchanged alert rules (same timestamp) are skipped

	sameTime := time.Date(2026, 1, 23, 10, 0, 0, 0, time.UTC)

	// Create mock alert rule
	alertRule := AlertRule{
		UID:       "test-alert-3",
		Title:     "Test Alert",
		Updated:   sameTime,
		FolderUID: "folder-1",
		RuleGroup: "group-1",
	}

	// Mock client returns one alert rule
	mockClient := &mockGrafanaClientForAlerts{
		listAlertRulesFunc: func(ctx context.Context) ([]AlertRule, error) {
			return []AlertRule{alertRule}, nil
		},
	}

	// Mock graph client returns same timestamp
	mockGraph := &mockGraphClientForAlerts{
		executeQueryFunc: func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
			// Return same timestamp for needsSync check
			return &graph.QueryResult{
				Rows: [][]interface{}{
					{sameTime.Format(time.RFC3339)},
				},
			}, nil
		},
	}

	// Create builder
	mockBuilder := NewGraphBuilder(mockGraph, nil, "test-integration", logging.GetLogger("test.graphbuilder"))

	// Create syncer
	logger := logging.GetLogger("test.alertsyncer")
	syncer := NewAlertSyncer(mockClient, mockGraph, mockBuilder, "test-integration", logger)

	// Run sync - should complete without errors (alert skipped)
	if err := syncer.syncAlerts(); err != nil {
		t.Fatalf("syncAlerts failed: %v", err)
	}
}

func TestAlertSyncer_APIError(t *testing.T) {
	// Test that API errors are propagated and sync stops

	// Mock client returns error
	mockClient := &mockGrafanaClientForAlerts{
		listAlertRulesFunc: func(ctx context.Context) ([]AlertRule, error) {
			return nil, fmt.Errorf("API connection failed")
		},
	}

	// Mock graph client
	mockGraph := &mockGraphClientForAlerts{}

	// Create builder
	mockBuilder := NewGraphBuilder(mockGraph, nil, "test-integration", logging.GetLogger("test.graphbuilder"))

	// Create syncer
	logger := logging.GetLogger("test.alertsyncer")
	syncer := NewAlertSyncer(mockClient, mockGraph, mockBuilder, "test-integration", logger)

	// Run sync - should return error
	err := syncer.syncAlerts()
	if err == nil {
		t.Error("syncAlerts should return error when API call fails")
	}

	// Verify error message contains expected text
	if err != nil && err.Error() != "failed to list alert rules: API connection failed" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestAlertSyncer_Lifecycle(t *testing.T) {
	// Test that Start/Stop lifecycle works correctly
	ctx := context.Background()

	// Mock client returns empty list
	mockClient := &mockGrafanaClientForAlerts{
		listAlertRulesFunc: func(ctx context.Context) ([]AlertRule, error) {
			return []AlertRule{}, nil
		},
	}

	// Mock graph client
	mockGraph := &mockGraphClientForAlerts{
		executeQueryFunc: func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
			return &graph.QueryResult{Rows: [][]interface{}{}}, nil
		},
	}

	// Create builder
	mockBuilder := NewGraphBuilder(mockGraph, nil, "test-integration", logging.GetLogger("test.graphbuilder"))

	// Create syncer
	logger := logging.GetLogger("test.alertsyncer")
	syncer := NewAlertSyncer(mockClient, mockGraph, mockBuilder, "test-integration", logger)

	// Start syncer
	if err := syncer.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify context is set
	if syncer.ctx == nil {
		t.Error("Context should be set after Start")
	}

	// Stop syncer
	syncer.Stop()

	// Verify stopped channel is closed (with timeout)
	select {
	case <-syncer.stopped:
		// Success - channel closed
	case <-time.After(6 * time.Second):
		t.Error("Stopped channel was not closed after Stop")
	}
}
