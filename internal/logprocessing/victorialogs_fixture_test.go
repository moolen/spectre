package logprocessing

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VictoriaLogsEntry represents a log entry from VictoriaLogs
type VictoriaLogsEntry struct {
	Msg       string `json:"_msg"`
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Container string `json:"container"`
}

// TestVictoriaLogsFixture processes real logs from VictoriaLogs and verifies
// that the logprocessing pipeline produces expected template patterns.
//
// Note: Drain clustering creates templates incrementally. The first log of a pattern
// creates an initial template, and subsequent logs may match to a refined version.
// This is expected behavior - the test verifies that similar logs cluster together,
// not that they all end up in exactly one template.
func TestVictoriaLogsFixture(t *testing.T) {
	// Load fixture file
	file, err := os.Open("testdata/victorialogs_sample.jsonl")
	require.NoError(t, err, "failed to open fixture file")
	defer file.Close()

	// Parse logs
	var entries []VictoriaLogsEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry VictoriaLogsEntry
		err := json.Unmarshal(scanner.Bytes(), &entry)
		require.NoError(t, err, "failed to parse log entry: %s", scanner.Text())
		entries = append(entries, entry)
	}
	require.NoError(t, scanner.Err())
	require.NotEmpty(t, entries, "fixture file should contain log entries")

	t.Logf("Loaded %d log entries from fixture", len(entries))

	// Create template store with default config
	store := NewTemplateStore(DefaultDrainConfig())

	// Process all logs
	for _, entry := range entries {
		_, err := store.Process(entry.Namespace, entry.Msg)
		require.NoError(t, err, "failed to process log: %s", entry.Msg)
	}

	// Verify namespaces were created
	namespaces := store.GetNamespaces()
	t.Logf("Found %d namespaces: %v", len(namespaces), namespaces)
	assert.Contains(t, namespaces, "kube-system", "should have kube-system namespace")
	assert.Contains(t, namespaces, "immich", "should have immich namespace")
	assert.Contains(t, namespaces, "longhorn-system", "should have longhorn-system namespace")

	// Verify templates were created for each namespace
	t.Run("kube-system templates", func(t *testing.T) {
		templates, err := store.ListTemplates("kube-system")
		require.NoError(t, err)
		assert.NotEmpty(t, templates, "kube-system should have templates")
		t.Logf("kube-system has %d templates", len(templates))

		for _, tmpl := range templates {
			t.Logf("  [%d] %s", tmpl.Count, tmpl.Pattern)
		}

		// Should have templates for the cilium logs we provided
		// Total logs in kube-system: 4 (3 regenerating + 1 GC)
		totalCount := 0
		for _, tmpl := range templates {
			totalCount += tmpl.Count
		}
		assert.Equal(t, 4, totalCount, "should have processed all 4 kube-system logs")
	})

	t.Run("immich templates", func(t *testing.T) {
		templates, err := store.ListTemplates("immich")
		require.NoError(t, err)
		assert.NotEmpty(t, templates, "immich should have templates")
		t.Logf("immich has %d templates", len(templates))

		for _, tmpl := range templates {
			t.Logf("  [%d] %s", tmpl.Count, tmpl.Pattern)
		}

		// Redis AOF sync messages should cluster together
		// Even if split across 2 templates due to Drain learning, total should be 3
		totalCount := 0
		for _, tmpl := range templates {
			totalCount += tmpl.Count
		}
		assert.Equal(t, 3, totalCount, "should have processed all 3 immich logs")

		// All templates should contain the core Redis AOF message
		for _, tmpl := range templates {
			assert.Contains(t, tmpl.Pattern, "asynchronous aof fsync", "template should match Redis AOF pattern")
		}
	})

	t.Run("longhorn-system templates", func(t *testing.T) {
		templates, err := store.ListTemplates("longhorn-system")
		require.NoError(t, err)
		assert.NotEmpty(t, templates, "longhorn-system should have templates")
		t.Logf("longhorn-system has %d templates", len(templates))

		for _, tmpl := range templates {
			t.Logf("  [%d] %s", tmpl.Count, tmpl.Pattern)
		}

		// Total logs in longhorn-system: 8 (3 HTTP + 3 warnings + 2 admission)
		totalCount := 0
		for _, tmpl := range templates {
			totalCount += tmpl.Count
		}
		assert.Equal(t, 8, totalCount, "should have processed all 8 longhorn-system logs")
	})

	t.Run("template patterns contain masked UUIDs", func(t *testing.T) {
		// Check that UUID-like patterns are masked
		for _, ns := range namespaces {
			templates, err := store.ListTemplates(ns)
			require.NoError(t, err)

			for _, tmpl := range templates {
				// UUID in fixture: pvc-539ce20d-81ab-42c7-b8b0-e8b5d48a0841
				// Should be masked as <UUID> or similar
				assert.NotContains(t, tmpl.Pattern, "539ce20d", "pattern should mask UUIDs")
			}
		}
	})
}

// TestVictoriaLogsFixture_TemplateStability verifies that processing the same
// logs multiple times produces stable template IDs.
func TestVictoriaLogsFixture_TemplateStability(t *testing.T) {
	// Load fixture
	file, err := os.Open("testdata/victorialogs_sample.jsonl")
	require.NoError(t, err)
	defer file.Close()

	var entries []VictoriaLogsEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry VictoriaLogsEntry
		err := json.Unmarshal(scanner.Bytes(), &entry)
		require.NoError(t, err)
		entries = append(entries, entry)
	}

	// Process logs in two separate stores
	store1 := NewTemplateStore(DefaultDrainConfig())
	store2 := NewTemplateStore(DefaultDrainConfig())

	var templateIDs1, templateIDs2 []string

	for _, entry := range entries {
		id1, _ := store1.Process(entry.Namespace, entry.Msg)
		id2, _ := store2.Process(entry.Namespace, entry.Msg)
		templateIDs1 = append(templateIDs1, id1)
		templateIDs2 = append(templateIDs2, id2)
	}

	// Template IDs should be identical for the same logs
	assert.Equal(t, templateIDs1, templateIDs2, "template IDs should be stable across stores")
}
