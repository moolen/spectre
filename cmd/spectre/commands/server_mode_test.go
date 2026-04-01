package commands

import "testing"

func TestResolveServerRuntimeMode(t *testing.T) {
	t.Run("embedded requires import path", func(t *testing.T) {
		_, err := resolveServerRuntimeMode(serverModeInput{
			Embedded:       true,
			ImportPath:     "",
			GraphEnabled:   true,
			WatcherEnabled: true,
		})
		if err == nil {
			t.Fatalf("expected error when embedded is true and import path is empty")
		}
	})

	t.Run("embedded mode disables runtime components", func(t *testing.T) {
		mode, err := resolveServerRuntimeMode(serverModeInput{
			Embedded:       true,
			ImportPath:     "/tmp/import.jsonl",
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
		if mode.StartGraph || mode.StartWatcher || mode.StartMCP {
			t.Fatalf("expected embedded mode to disable graph/watcher/mcp start flags")
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
