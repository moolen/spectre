package commands

import (
	"reflect"
	"testing"

	"github.com/moolen/spectre/internal/embedded"
	"github.com/moolen/spectre/internal/embeddedstore"
	"github.com/moolen/spectre/internal/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestResolveServerRuntimeMode(t *testing.T) {
	t.Run("embedded live mode enables watcher and mcp", func(t *testing.T) {
		mode, err := resolveServerRuntimeMode(serverModeInput{
			Embedded:       true,
			ImportPath:     "",
			GraphEnabled:   true,
			WatcherEnabled: true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mode.Name != string(runtimeNameEmbedded) {
			t.Fatalf("expected mode name %s, got %q", runtimeNameEmbedded, mode.Name)
		}
		if !mode.Embedded {
			t.Fatalf("expected embedded to be true")
		}
		if mode.Backend != string(backendEmbedded) {
			t.Fatalf("expected backend %s, got %q", backendEmbedded, mode.Backend)
		}
		if mode.IngestionMode != string(ingestionModeLive) {
			t.Fatalf("expected ingestion mode %s, got %q", ingestionModeLive, mode.IngestionMode)
		}
		if mode.ImportOnly {
			t.Fatalf("expected import-only flag to be false")
		}
		if mode.StartGraph {
			t.Fatalf("expected StartGraph to be false in embedded live mode")
		}
		if !mode.StartWatcher {
			t.Fatalf("expected StartWatcher to be true in embedded live mode")
		}
		if !mode.StartMCP {
			t.Fatalf("expected StartMCP to be true in embedded live mode")
		}
	})

	t.Run("embedded import-only mode disables watcher and enables mcp", func(t *testing.T) {
		mode, err := resolveServerRuntimeMode(serverModeInput{
			Embedded:       true,
			ImportPath:     "/tmp/import.jsonl",
			GraphEnabled:   false,
			WatcherEnabled: false,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mode.Name != string(runtimeNameEmbedded) {
			t.Fatalf("expected mode name %s, got %q", runtimeNameEmbedded, mode.Name)
		}
		if !mode.Embedded {
			t.Fatalf("expected embedded to be true")
		}
		if mode.AuditOnly {
			t.Fatalf("expected audit-only to be false")
		}
		if mode.Backend != string(backendEmbedded) {
			t.Fatalf("expected backend %s, got %q", backendEmbedded, mode.Backend)
		}
		if mode.IngestionMode != string(ingestionModeImportOnly) {
			t.Fatalf("expected ingestion mode %s, got %q", ingestionModeImportOnly, mode.IngestionMode)
		}
		if !mode.ImportOnly {
			t.Fatalf("expected import-only flag to be true")
		}
		if mode.StartGraph {
			t.Fatalf("expected StartGraph to be false in embedded import-only mode")
		}
		if mode.StartWatcher {
			t.Fatalf("expected StartWatcher to be false in embedded import-only mode")
		}
		if !mode.StartMCP {
			t.Fatalf("expected StartMCP to be true in embedded import-only mode")
		}
	})

	t.Run("embedded import-only mode without import path still serves persisted data", func(t *testing.T) {
		mode, err := resolveServerRuntimeMode(serverModeInput{
			Embedded:       true,
			ImportPath:     "",
			GraphEnabled:   true,
			WatcherEnabled: false,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mode.IngestionMode != string(ingestionModeImportOnly) {
			t.Fatalf("expected ingestion mode %s, got %q", ingestionModeImportOnly, mode.IngestionMode)
		}
		if mode.StartWatcher {
			t.Fatalf("expected StartWatcher to be false when watcher is disabled")
		}
	})

	t.Run("graph disabled without audit-only returns error", func(t *testing.T) {
		_, err := resolveServerRuntimeMode(serverModeInput{
			GraphEnabled:   false,
			WatcherEnabled: true,
			AuditLogPath:   "",
		})
		if err == nil {
			t.Fatalf("expected error when graph is disabled without audit-only mode")
		}
	})

	t.Run("audit-only mode uses watcher without graph", func(t *testing.T) {
		mode, err := resolveServerRuntimeMode(serverModeInput{
			GraphEnabled:   false,
			WatcherEnabled: true,
			AuditLogPath:   "/tmp/audit.jsonl",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !mode.AuditOnly {
			t.Fatalf("expected audit-only mode")
		}
		if mode.StartGraph {
			t.Fatalf("expected StartGraph to be false in audit-only mode")
		}
		if !mode.StartWatcher {
			t.Fatalf("expected StartWatcher to be true in audit-only mode")
		}
		if mode.StartMCP {
			t.Fatalf("expected StartMCP to be false in audit-only mode")
		}
	})

	t.Run("graph enabled with watcher disabled disables watcher", func(t *testing.T) {
		mode, err := resolveServerRuntimeMode(serverModeInput{
			GraphEnabled:   true,
			WatcherEnabled: false,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mode.Name != string(runtimeNameGraph) {
			t.Fatalf("expected mode name %s, got %q", runtimeNameGraph, mode.Name)
		}
		if mode.Backend != string(backendFalkor) {
			t.Fatalf("expected backend %s, got %q", backendFalkor, mode.Backend)
		}
		if mode.IngestionMode != string(ingestionModeLive) {
			t.Fatalf("expected ingestion mode %s, got %q", ingestionModeLive, mode.IngestionMode)
		}
		if mode.StartWatcher {
			t.Fatalf("expected StartWatcher to be false when watcher is disabled")
		}
	})
}

func TestEmbeddedExecutorHasUsableEvents(t *testing.T) {
	t.Run("returns false when executor has no usable resources", func(t *testing.T) {
		executor, err := embedded.NewQueryExecutor([]models.Event{
			{
				ID:        "missing-uid",
				Timestamp: 1,
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					Kind:    "Pod",
					Version: "v1",
					Name:    "pod-1",
					UID:     "",
				},
			},
			{
				ID:        "event-missing-involved",
				Timestamp: 2,
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					Kind:    "Event",
					Version: "v1",
					Name:    "event-1",
					UID:     "event-uid",
				},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error building executor: %v", err)
		}

		usable, err := hasUsableEmbeddedEvents(executor)
		if err != nil {
			t.Fatalf("unexpected error checking executor: %v", err)
		}
		if usable {
			t.Fatalf("expected unusable executor to report no usable events")
		}
	})

	t.Run("returns true when executor has usable resources", func(t *testing.T) {
		executor, err := embedded.NewQueryExecutor([]models.Event{
			{
				ID:        "pod-create",
				Timestamp: 10,
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					Kind:      "Pod",
					Version:   "v1",
					Name:      "pod-1",
					Namespace: "default",
					UID:       "pod-uid",
				},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error building executor: %v", err)
		}

		usable, err := hasUsableEmbeddedEvents(executor)
		if err != nil {
			t.Fatalf("unexpected error checking executor: %v", err)
		}
		if !usable {
			t.Fatalf("expected executor to report usable events")
		}
	})
}

func TestServer_EmbeddedProjectionHistoryFallbackFlag(t *testing.T) {
	flag := serverCmd.Flags().Lookup("embedded-projection-history-fallback")
	require.NotNil(t, flag)

	originalDataDir := dataDir
	originalFallback := embeddedProjectionHistoryFallback
	t.Cleanup(func() {
		dataDir = originalDataDir
		embeddedProjectionHistoryFallback = originalFallback
	})

	dataDir = t.TempDir()
	require.NoError(t, serverCmd.Flags().Set("embedded-projection-history-fallback", "true"))
	require.True(t, embeddedProjectionHistoryFallback)

	embeddedCfg := embeddedStoreConfig()
	embeddedCfg.MetricsRegisterer = prometheus.NewRegistry()
	backend, err := embeddedstore.Open(embeddedCfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, backend.Close())
	})

	require.False(t, queryExecutorPlannerInstalled(t, backend.QueryExecutor()))

	require.NoError(t, serverCmd.Flags().Set("embedded-projection-history-fallback", "false"))
	dataDir = t.TempDir()

	embeddedCfgWithFallbackDisabled := embeddedStoreConfig()
	embeddedCfgWithFallbackDisabled.MetricsRegisterer = prometheus.NewRegistry()
	backendWithFallbackDisabled, err := embeddedstore.Open(embeddedCfgWithFallbackDisabled)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, backendWithFallbackDisabled.Close())
	})

	require.True(t, queryExecutorPlannerInstalled(t, backendWithFallbackDisabled.QueryExecutor()))
}

func queryExecutorPlannerInstalled(t *testing.T, executor interface{}) bool {
	t.Helper()
	require.NotNil(t, executor)

	value := reflect.ValueOf(executor)
	require.Equal(t, reflect.Pointer, value.Kind())

	field := value.Elem().FieldByName("planner")
	require.True(t, field.IsValid(), "expected planner field on query executor")
	require.Equal(t, reflect.Pointer, field.Kind())

	return !field.IsNil()
}
