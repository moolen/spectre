package commands

import "testing"

func TestResolveServerRuntimeMode(t *testing.T) {
	t.Run("embedded requires import path", func(t *testing.T) {
		_, err := resolveServerRuntimeMode(serverModeInput{
			Embedded:     true,
			ImportPath:   "",
			GraphEnabled: true,
			WatcherEnabled: true,
		})
		if err == nil {
			t.Fatalf("expected error when embedded is true and import path is empty")
		}
	})

	t.Run("embedded mode disables runtime components", func(t *testing.T) {
		mode, err := resolveServerRuntimeMode(serverModeInput{
			Embedded:     true,
			ImportPath:   "/tmp/import.jsonl",
			GraphEnabled: true,
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
}
