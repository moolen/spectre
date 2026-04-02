package commands

import (
	"testing"

	"github.com/moolen/spectre/internal/embedded"
	"github.com/moolen/spectre/internal/models"
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
		if mode.Name != "embedded" {
			t.Fatalf("expected mode name embedded, got %q", mode.Name)
		}
		if !mode.Embedded {
			t.Fatalf("expected embedded to be true")
		}
		if mode.Backend != "embedded" {
			t.Fatalf("expected backend embedded, got %q", mode.Backend)
		}
		if mode.IngestionMode != "live" {
			t.Fatalf("expected ingestion mode live, got %q", mode.IngestionMode)
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
		if mode.Backend != "embedded" {
			t.Fatalf("expected backend embedded, got %q", mode.Backend)
		}
		if mode.IngestionMode != "import-only" {
			t.Fatalf("expected ingestion mode import-only, got %q", mode.IngestionMode)
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

	t.Run("embedded mode with watcher disabled requires import path", func(t *testing.T) {
		_, err := resolveServerRuntimeMode(serverModeInput{
			Embedded:       true,
			ImportPath:     "",
			GraphEnabled:   true,
			WatcherEnabled: false,
		})
		if err == nil {
			t.Fatalf("expected error when embedded has watcher disabled without import path")
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
		if mode.Name != "graph" {
			t.Fatalf("expected mode name graph, got %q", mode.Name)
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
