package commands

import (
	"context"
	"reflect"
	"testing"

	"github.com/moolen/spectre/internal/embedded"
	"github.com/moolen/spectre/internal/embeddedstore"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestResolveServerRuntimeMode(t *testing.T) {
	t.Run("live mode enables watcher and mcp", func(t *testing.T) {
		mode, err := resolveServerRuntimeMode(serverModeInput{
			WatcherEnabled: true,
			ImportPath:     "",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mode.Name != string(runtimeNameEmbedded) {
			t.Fatalf("expected mode name %s, got %q", runtimeNameEmbedded, mode.Name)
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
		if !mode.StartWatcher {
			t.Fatalf("expected StartWatcher to be true in embedded live mode")
		}
		if !mode.StartMCP {
			t.Fatalf("expected StartMCP to be true in embedded live mode")
		}
	})

	t.Run("import-only mode disables watcher and keeps mcp enabled", func(t *testing.T) {
		mode, err := resolveServerRuntimeMode(serverModeInput{
			WatcherEnabled: false,
			ImportPath:     "/tmp/import.jsonl",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if mode.Name != string(runtimeNameEmbedded) {
			t.Fatalf("expected mode name %s, got %q", runtimeNameEmbedded, mode.Name)
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
		if mode.StartWatcher {
			t.Fatalf("expected StartWatcher to be false in embedded import-only mode")
		}
		if !mode.StartMCP {
			t.Fatalf("expected StartMCP to be true in embedded import-only mode")
		}
	})

	t.Run("watcher-disabled mode without import path still serves persisted data", func(t *testing.T) {
		mode, err := resolveServerRuntimeMode(serverModeInput{
			WatcherEnabled: false,
			ImportPath:     "",
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

}

func TestServer_RemovedGraphAndIntegrationFlags(t *testing.T) {
	t.Helper()

	removedFlags := []string{
		"embedded",
		"graph-enabled",
		"graph-host",
		"graph-port",
		"graph-name",
		"graph-retention-hours",
		"integrations-config",
		"min-integration-version",
	}

	for _, name := range removedFlags {
		require.Nilf(t, serverCmd.Flags().Lookup(name), "flag %q should not be registered", name)
	}
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

func TestShouldSkipEmbeddedInitialListReplay(t *testing.T) {
	backend, err := embeddedstore.Open(embeddedstore.Config{
		DataDir:           t.TempDir(),
		MetricsRegisterer: prometheus.NewRegistry(),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, backend.Close())
	})

	logger := logging.GetLogger("server.test")
	require.False(t, shouldSkipEmbeddedInitialListReplay(backend, logger))

	require.NoError(t, backend.ProcessEvent(context.Background(), models.Event{
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
	}))

	require.True(t, shouldSkipEmbeddedInitialListReplay(backend, logger))
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
