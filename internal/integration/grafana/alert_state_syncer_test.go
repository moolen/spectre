package grafana

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// mockGrafanaClientForStates implements GrafanaClientInterface for testing state sync
type mockGrafanaClientForStates struct {
	getAlertStatesFunc func(ctx context.Context) ([]AlertState, error)
}

func (m *mockGrafanaClientForStates) ListDashboards(ctx context.Context) ([]DashboardMeta, error) {
	return nil, nil
}

func (m *mockGrafanaClientForStates) GetDashboard(ctx context.Context, uid string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockGrafanaClientForStates) ListAlertRules(ctx context.Context) ([]AlertRule, error) {
	return nil, nil
}

func (m *mockGrafanaClientForStates) GetAlertStates(ctx context.Context) ([]AlertState, error) {
	if m.getAlertStatesFunc != nil {
		return m.getAlertStatesFunc(ctx)
	}
	return nil, nil
}

// mockGraphClientForStates implements graph.Client for testing state sync
type mockGraphClientForStates struct {
	executeQueryFunc func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error)
	queryCalls       []string // Track query strings for verification
}

func (m *mockGraphClientForStates) ExecuteQuery(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
	// Track query calls
	m.queryCalls = append(m.queryCalls, query.Query)

	if m.executeQueryFunc != nil {
		return m.executeQueryFunc(ctx, query)
	}
	return &graph.QueryResult{Rows: [][]interface{}{}}, nil
}

func (m *mockGraphClientForStates) Close() error                                 { return nil }
func (m *mockGraphClientForStates) Connect(ctx context.Context) error            { return nil }
func (m *mockGraphClientForStates) Ping(ctx context.Context) error               { return nil }
func (m *mockGraphClientForStates) CreateNode(ctx context.Context, nodeType graph.NodeType, properties interface{}) error {
	return nil
}
func (m *mockGraphClientForStates) CreateEdge(ctx context.Context, edgeType graph.EdgeType, fromUID, toUID string, properties interface{}) error {
	return nil
}
func (m *mockGraphClientForStates) GetNode(ctx context.Context, nodeType graph.NodeType, uid string) (*graph.Node, error) {
	return nil, nil
}
func (m *mockGraphClientForStates) DeleteNodesByTimestamp(ctx context.Context, nodeType graph.NodeType, timestampField string, cutoffNs int64) (int, error) {
	return 0, nil
}
func (m *mockGraphClientForStates) GetGraphStats(ctx context.Context) (*graph.GraphStats, error) {
	return nil, nil
}
func (m *mockGraphClientForStates) InitializeSchema(ctx context.Context) error { return nil }
func (m *mockGraphClientForStates) DeleteGraph(ctx context.Context) error      { return nil }
func (m *mockGraphClientForStates) CreateGraph(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockGraphClientForStates) DeleteGraphByName(ctx context.Context, graphName string) error {
	return nil
}
func (m *mockGraphClientForStates) GraphExists(ctx context.Context, graphName string) (bool, error) {
	return true, nil
}

func TestAlertStateSyncer_SyncStates_Initial(t *testing.T) {
	// Test that new alerts (no previous state) create initial transitions

	logger := logging.GetLogger("test.alert_state_syncer")

	// Mock GetAlertStates to return 2 alerts in different states
	mockClient := &mockGrafanaClientForStates{
		getAlertStatesFunc: func(ctx context.Context) ([]AlertState, error) {
			return []AlertState{
				{
					UID:   "alert1",
					Title: "Test Alert 1",
					Instances: []AlertInstance{
						{State: "firing"},
					},
				},
				{
					UID:   "alert2",
					Title: "Test Alert 2",
					Instances: []AlertInstance{
						{State: "normal"},
					},
				},
			}, nil
		},
	}

	// Mock graph client - track queries by content
	transitionEdgeCount := 0
	lastSyncedAtCount := 0
	mockGraph := &mockGraphClientForStates{
		executeQueryFunc: func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
			queryStr := query.Query

			// CreateStateTransitionEdge: has from_state parameter
			if query.Parameters["from_state"] != nil {
				transitionEdgeCount++
				return &graph.QueryResult{}, nil
			}

			// getLastKnownState: contains "RETURN t.to_state"
			if strings.Contains(queryStr, "RETURN t.to_state") {
				return &graph.QueryResult{Rows: [][]interface{}{}}, nil // Empty = unknown
			}

			// updateLastSyncedAt: contains "SET a.last_synced_at"
			if strings.Contains(queryStr, "SET a.last_synced_at") {
				lastSyncedAtCount++
				return &graph.QueryResult{}, nil
			}

			return &graph.QueryResult{}, nil
		},
	}

	// Create syncer
	builder := NewGraphBuilder(mockGraph, nil, "test-integration", logger)
	syncer := NewAlertStateSyncer(mockClient, mockGraph, builder, "test-integration", logger)

	// Run sync
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	syncer.ctx = ctx
	err := syncer.syncStates()
	if err != nil {
		t.Fatalf("syncStates failed: %v", err)
	}

	// Verify CreateStateTransitionEdge called 2 times (both create initial transitions)
	if transitionEdgeCount != 2 {
		t.Errorf("Expected 2 state transitions, got %d", transitionEdgeCount)
	}

	// Verify last_synced_at updated for both alerts
	if lastSyncedAtCount != 2 {
		t.Errorf("Expected 2 last_synced_at updates, got %d", lastSyncedAtCount)
	}
}

func TestAlertStateSyncer_SyncStates_Deduplication(t *testing.T) {
	// Test that unchanged state doesn't create transition edge

	logger := logging.GetLogger("test.alert_state_syncer")

	// Mock GetAlertStates to return alert still in "firing" state
	mockClient := &mockGrafanaClientForStates{
		getAlertStatesFunc: func(ctx context.Context) ([]AlertState, error) {
			return []AlertState{
				{
					UID:   "alert1",
					Title: "Test Alert 1",
					Instances: []AlertInstance{
						{State: "firing"},
					},
				},
			}, nil
		},
	}

	// Mock graph client
	transitionEdgeCount := 0
	lastSyncedAtCount := 0
	mockGraph := &mockGraphClientForStates{
		executeQueryFunc: func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
			queryStr := query.Query

			// CreateStateTransitionEdge: has from_state parameter
			if query.Parameters["from_state"] != nil {
				transitionEdgeCount++
				return &graph.QueryResult{}, nil
			}

			// getLastKnownState returns "firing" (unchanged)
			if strings.Contains(queryStr, "RETURN t.to_state") {
				return &graph.QueryResult{
					Rows: [][]interface{}{
						{"firing"}, // Previous state was also firing
					},
				}, nil
			}

			// updateLastSyncedAt: contains "SET a.last_synced_at"
			if strings.Contains(queryStr, "SET a.last_synced_at") {
				lastSyncedAtCount++
				return &graph.QueryResult{}, nil
			}

			return &graph.QueryResult{}, nil
		},
	}

	// Create syncer
	builder := NewGraphBuilder(mockGraph, nil, "test-integration", logger)
	syncer := NewAlertStateSyncer(mockClient, mockGraph, builder, "test-integration", logger)

	// Run sync
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	syncer.ctx = ctx
	err := syncer.syncStates()
	if err != nil {
		t.Fatalf("syncStates failed: %v", err)
	}

	// Verify CreateStateTransitionEdge NOT called (no state change)
	if transitionEdgeCount != 0 {
		t.Errorf("Expected 0 state transitions (deduplicated), got %d", transitionEdgeCount)
	}

	// Verify last_synced_at still updated (successful sync even if no change)
	if lastSyncedAtCount != 1 {
		t.Errorf("Expected 1 last_synced_at update, got %d", lastSyncedAtCount)
	}
}

func TestAlertStateSyncer_SyncStates_StateChange(t *testing.T) {
	// Test that state change creates transition edge

	logger := logging.GetLogger("test.alert_state_syncer")

	// Mock GetAlertStates to return alert in "firing" state
	mockClient := &mockGrafanaClientForStates{
		getAlertStatesFunc: func(ctx context.Context) ([]AlertState, error) {
			return []AlertState{
				{
					UID:   "alert1",
					Title: "Test Alert 1",
					Instances: []AlertInstance{
						{State: "firing"},
					},
				},
			}, nil
		},
	}

	// Mock graph client
	var capturedFromState, capturedToState string
	transitionEdgeCount := 0
	mockGraph := &mockGraphClientForStates{
		executeQueryFunc: func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
			queryStr := query.Query

			// Capture transition edge parameters
			if query.Parameters["from_state"] != nil {
				transitionEdgeCount++
				capturedFromState = query.Parameters["from_state"].(string)
				capturedToState = query.Parameters["to_state"].(string)
				return &graph.QueryResult{}, nil
			}

			// getLastKnownState returns "normal" (state changed)
			if strings.Contains(queryStr, "RETURN t.to_state") {
				return &graph.QueryResult{
					Rows: [][]interface{}{
						{"normal"}, // Previous state was normal
					},
				}, nil
			}

			return &graph.QueryResult{}, nil
		},
	}

	// Create syncer
	builder := NewGraphBuilder(mockGraph, nil, "test-integration", logger)
	syncer := NewAlertStateSyncer(mockClient, mockGraph, builder, "test-integration", logger)

	// Run sync
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	syncer.ctx = ctx
	err := syncer.syncStates()
	if err != nil {
		t.Fatalf("syncStates failed: %v", err)
	}

	// Verify CreateStateTransitionEdge called with from="normal", to="firing"
	if transitionEdgeCount != 1 {
		t.Errorf("Expected 1 state transition, got %d", transitionEdgeCount)
	}
	if capturedFromState != "normal" {
		t.Errorf("Expected from_state='normal', got '%s'", capturedFromState)
	}
	if capturedToState != "firing" {
		t.Errorf("Expected to_state='firing', got '%s'", capturedToState)
	}
}

func TestAlertStateSyncer_SyncStates_APIError(t *testing.T) {
	// Test that API error doesn't panic and sets lastError

	logger := logging.GetLogger("test.alert_state_syncer")

	// Mock GetAlertStates to return error
	mockClient := &mockGrafanaClientForStates{
		getAlertStatesFunc: func(ctx context.Context) ([]AlertState, error) {
			return nil, fmt.Errorf("API unavailable")
		},
	}

	mockGraph := &mockGraphClientForStates{}

	// Create syncer
	builder := NewGraphBuilder(mockGraph, nil, "test-integration", logger)
	syncer := NewAlertStateSyncer(mockClient, mockGraph, builder, "test-integration", logger)

	// Record initial lastSyncTime (should not be updated on error)
	initialSyncTime := time.Now().Add(-1 * time.Hour)
	syncer.lastSyncTime = initialSyncTime

	// Run sync
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	syncer.ctx = ctx
	err := syncer.syncStates()

	// Verify error returned
	if err == nil {
		t.Fatal("Expected error from syncStates, got nil")
	}

	// Note: lastError is NOT automatically set in syncStates on return
	// It's set by the caller (syncLoop) via setLastError
	// The test directly calls syncStates, so we just verify error is returned

	// Verify lastSyncTime NOT updated (staleness detection)
	syncer.mu.RLock()
	lastSyncTime := syncer.lastSyncTime
	syncer.mu.RUnlock()

	if lastSyncTime != initialSyncTime {
		t.Errorf("Expected lastSyncTime to remain unchanged on error, but it was updated")
	}
}

func TestAlertStateSyncer_AggregateInstanceStates(t *testing.T) {
	// Test state aggregation logic

	logger := logging.GetLogger("test.alert_state_syncer")
	syncer := NewAlertStateSyncer(nil, nil, nil, "test", logger)

	tests := []struct {
		name      string
		instances []AlertInstance
		expected  string
	}{
		{
			name: "firing has highest priority",
			instances: []AlertInstance{
				{State: "firing"},
				{State: "normal"},
				{State: "normal"},
			},
			expected: "firing",
		},
		{
			name: "pending has medium priority",
			instances: []AlertInstance{
				{State: "pending"},
				{State: "normal"},
				{State: "normal"},
			},
			expected: "pending",
		},
		{
			name: "all normal",
			instances: []AlertInstance{
				{State: "normal"},
				{State: "normal"},
				{State: "normal"},
			},
			expected: "normal",
		},
		{
			name:      "empty instances defaults to normal",
			instances: []AlertInstance{},
			expected:  "normal",
		},
		{
			name: "alerting state treated as firing",
			instances: []AlertInstance{
				{State: "alerting"},
				{State: "normal"},
			},
			expected: "firing",
		},
		{
			name: "firing overrides pending",
			instances: []AlertInstance{
				{State: "pending"},
				{State: "firing"},
				{State: "normal"},
			},
			expected: "firing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := syncer.aggregateInstanceStates(tt.instances)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestAlertStateSyncer_StartStop(t *testing.T) {
	// Test lifecycle: Start and Stop work correctly

	logger := logging.GetLogger("test.alert_state_syncer")

	// Mock client with no errors
	mockClient := &mockGrafanaClientForStates{
		getAlertStatesFunc: func(ctx context.Context) ([]AlertState, error) {
			return []AlertState{}, nil
		},
	}

	mockGraph := &mockGraphClientForStates{
		executeQueryFunc: func(ctx context.Context, query graph.GraphQuery) (*graph.QueryResult, error) {
			return &graph.QueryResult{Rows: [][]interface{}{}}, nil
		},
	}

	builder := NewGraphBuilder(mockGraph, nil, "test-integration", logger)
	syncer := NewAlertStateSyncer(mockClient, mockGraph, builder, "test-integration", logger)

	// Start syncer
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := syncer.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify syncer is running (check sync loop started)
	time.Sleep(100 * time.Millisecond)

	// Stop syncer
	syncer.Stop()

	// Verify stopped channel closed
	select {
	case <-syncer.stopped:
		// Success - channel closed
	case <-time.After(6 * time.Second):
		t.Fatal("Stop did not complete within timeout")
	}
}
