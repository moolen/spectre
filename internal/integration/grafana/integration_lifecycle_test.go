package grafana

import (
	"context"
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
	syncer := NewDashboardSyncer(mockGrafana, mockGraph, nil, 100*time.Millisecond, logger)

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
