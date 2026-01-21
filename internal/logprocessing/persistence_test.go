package logprocessing

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewPersistenceManager(t *testing.T) {
	store := NewTemplateStore(DefaultDrainConfig())
	pm := NewPersistenceManager(store, "/tmp/test.json", 5*time.Minute)

	if pm == nil {
		t.Fatal("NewPersistenceManager returned nil")
	}

	if pm.store != store {
		t.Error("store reference not set correctly")
	}

	if pm.snapshotPath != "/tmp/test.json" {
		t.Errorf("snapshotPath = %s, want /tmp/test.json", pm.snapshotPath)
	}

	if pm.snapshotInterval != 5*time.Minute {
		t.Errorf("snapshotInterval = %v, want 5m", pm.snapshotInterval)
	}
}

func TestSnapshot_EmptyStore(t *testing.T) {
	store := NewTemplateStore(DefaultDrainConfig())
	tmpPath := filepath.Join(os.TempDir(), "test-empty-snapshot.json")
	defer os.Remove(tmpPath)

	pm := NewPersistenceManager(store, tmpPath, time.Minute)

	// Snapshot empty store
	if err := pm.Snapshot(); err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(tmpPath); err != nil {
		t.Fatalf("snapshot file not created: %v", err)
	}

	// Verify JSON is valid
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("failed to read snapshot: %v", err)
	}

	var snapshot SnapshotData
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("failed to unmarshal snapshot: %v", err)
	}

	if snapshot.Version != 1 {
		t.Errorf("snapshot version = %d, want 1", snapshot.Version)
	}

	if len(snapshot.Namespaces) != 0 {
		t.Errorf("empty store should have 0 namespaces, got %d", len(snapshot.Namespaces))
	}
}

func TestSnapshot_WithData(t *testing.T) {
	store := NewTemplateStore(DefaultDrainConfig())
	tmpPath := filepath.Join(os.TempDir(), "test-snapshot-with-data.json")
	defer os.Remove(tmpPath)

	// Add some templates
	store.Process("ns1", "connected to 10.0.0.1")
	store.Process("ns1", "connected to 10.0.0.2")
	store.Process("ns2", "error: connection timeout")

	pm := NewPersistenceManager(store, tmpPath, time.Minute)

	// Create snapshot
	if err := pm.Snapshot(); err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	// Read and verify snapshot
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("failed to read snapshot: %v", err)
	}

	var snapshot SnapshotData
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("failed to unmarshal snapshot: %v", err)
	}

	// Should have 2 namespaces
	if len(snapshot.Namespaces) != 2 {
		t.Errorf("expected 2 namespaces, got %d", len(snapshot.Namespaces))
	}

	// Verify ns1 has templates
	ns1 := snapshot.Namespaces["ns1"]
	if ns1 == nil {
		t.Fatal("ns1 not found in snapshot")
	}

	if len(ns1.Templates) == 0 {
		t.Error("ns1 should have templates")
	}

	if len(ns1.Counts) == 0 {
		t.Error("ns1 should have counts")
	}
}

func TestSnapshot_AtomicWrites(t *testing.T) {
	store := NewTemplateStore(DefaultDrainConfig())
	tmpPath := filepath.Join(os.TempDir(), "test-atomic-snapshot.json")
	tmpTempPath := tmpPath + ".tmp"
	defer os.Remove(tmpPath)
	defer os.Remove(tmpTempPath)

	// Add data
	store.Process("default", "test log message")

	pm := NewPersistenceManager(store, tmpPath, time.Minute)

	// Create snapshot
	if err := pm.Snapshot(); err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	// Main file should exist
	if _, err := os.Stat(tmpPath); err != nil {
		t.Errorf("main snapshot file not created: %v", err)
	}

	// Temp file should be removed (atomic rename)
	if _, err := os.Stat(tmpTempPath); !os.IsNotExist(err) {
		t.Error("temp file should be removed after rename")
	}
}

func TestLoad_FileNotExists(t *testing.T) {
	store := NewTemplateStore(DefaultDrainConfig())
	nonExistentPath := filepath.Join(os.TempDir(), "nonexistent-snapshot.json")

	pm := NewPersistenceManager(store, nonExistentPath, time.Minute)

	// Load should return os.IsNotExist error
	err := pm.Load()
	if !os.IsNotExist(err) {
		t.Errorf("expected IsNotExist error, got: %v", err)
	}
}

func TestLoad_CorruptedJSON(t *testing.T) {
	store := NewTemplateStore(DefaultDrainConfig())
	tmpPath := filepath.Join(os.TempDir(), "test-corrupted-snapshot.json")
	defer os.Remove(tmpPath)

	// Write invalid JSON
	if err := os.WriteFile(tmpPath, []byte("not valid json {"), 0644); err != nil {
		t.Fatalf("failed to write corrupted file: %v", err)
	}

	pm := NewPersistenceManager(store, tmpPath, time.Minute)

	// Load should return error
	if err := pm.Load(); err == nil {
		t.Error("Load should fail on corrupted JSON")
	}
}

func TestLoad_UnsupportedVersion(t *testing.T) {
	store := NewTemplateStore(DefaultDrainConfig())
	tmpPath := filepath.Join(os.TempDir(), "test-version-snapshot.json")
	defer os.Remove(tmpPath)

	// Create snapshot with unsupported version
	snapshot := SnapshotData{
		Version:    999,
		Timestamp:  time.Now(),
		Namespaces: make(map[string]*NamespaceSnapshot),
	}

	data, _ := json.Marshal(snapshot)
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		t.Fatalf("failed to write snapshot: %v", err)
	}

	pm := NewPersistenceManager(store, tmpPath, time.Minute)

	// Load should fail with version error
	err := pm.Load()
	if err == nil {
		t.Error("Load should fail on unsupported version")
	}

	if err != nil && err.Error() != "unsupported snapshot version: 999" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoad_RestoresTemplates(t *testing.T) {
	// Create store and add templates
	store1 := NewTemplateStore(DefaultDrainConfig())
	id1, _ := store1.Process("default", "connected to 10.0.0.1")
	id2, _ := store1.Process("default", "connected to 10.0.0.2")
	store1.Process("ns2", "error: connection failed")

	tmpPath := filepath.Join(os.TempDir(), "test-restore-snapshot.json")
	defer os.Remove(tmpPath)

	// Snapshot store1
	pm1 := NewPersistenceManager(store1, tmpPath, time.Minute)
	if err := pm1.Snapshot(); err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	// Create new store and restore
	store2 := NewTemplateStore(DefaultDrainConfig())
	pm2 := NewPersistenceManager(store2, tmpPath, time.Minute)
	if err := pm2.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify templates restored
	template, err := store2.GetTemplate("default", id1)
	if err != nil {
		t.Fatalf("failed to get restored template: %v", err)
	}

	if template.ID != id1 {
		t.Errorf("template ID mismatch: got %s, want %s", template.ID, id1)
	}

	// Should have both templates in default namespace with count=2
	// (they map to same template due to IP masking)
	if template.Count != 2 {
		t.Errorf("template count = %d, want 2", template.Count)
	}

	// Verify namespaces
	namespaces := store2.GetNamespaces()
	if len(namespaces) != 2 {
		t.Errorf("expected 2 namespaces, got %d", len(namespaces))
	}

	// Verify second template exists
	_, err = store2.GetTemplate("default", id2)
	if err != nil {
		t.Error("second template should be restored")
	}
}

func TestSnapshotRoundtrip(t *testing.T) {
	// Create store with various templates
	store1 := NewTemplateStore(DefaultDrainConfig())

	logs := []struct {
		namespace string
		message   string
	}{
		{"default", "user login successful"},
		{"default", "user logout successful"},
		{"api", "POST /api/users returned 200"},
		{"api", "GET /api/health returned 200"},
		{"db", "connected to 10.0.0.1:5432"},
		{"db", "connected to 10.0.0.2:5432"},
	}

	for _, log := range logs {
		store1.Process(log.namespace, log.message)
	}

	tmpPath := filepath.Join(os.TempDir(), "test-roundtrip-snapshot.json")
	defer os.Remove(tmpPath)

	// Snapshot
	pm1 := NewPersistenceManager(store1, tmpPath, time.Minute)
	if err := pm1.Snapshot(); err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	// Load into new store
	store2 := NewTemplateStore(DefaultDrainConfig())
	pm2 := NewPersistenceManager(store2, tmpPath, time.Minute)
	if err := pm2.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Compare namespace counts
	ns1 := store1.GetNamespaces()
	ns2 := store2.GetNamespaces()
	if len(ns1) != len(ns2) {
		t.Errorf("namespace count mismatch: %d vs %d", len(ns1), len(ns2))
	}

	// Compare template counts per namespace
	for _, ns := range ns1 {
		templates1, _ := store1.ListTemplates(ns)
		templates2, _ := store2.ListTemplates(ns)

		if len(templates1) != len(templates2) {
			t.Errorf("namespace %s: template count mismatch: %d vs %d",
				ns, len(templates1), len(templates2))
		}

		// Build map of templates by ID for comparison (order-independent)
		templateMap1 := make(map[string]Template)
		for _, tmpl := range templates1 {
			templateMap1[tmpl.ID] = tmpl
		}

		templateMap2 := make(map[string]Template)
		for _, tmpl := range templates2 {
			templateMap2[tmpl.ID] = tmpl
		}

		// Verify each template from store1 exists in store2
		for id, t1 := range templateMap1 {
			t2, exists := templateMap2[id]
			if !exists {
				t.Errorf("template %s from store1 not found in store2", id)
				continue
			}

			if t1.Pattern != t2.Pattern {
				t.Errorf("pattern mismatch for %s: %s vs %s", id, t1.Pattern, t2.Pattern)
			}

			if t1.Count != t2.Count {
				t.Errorf("count mismatch for %s: %d vs %d", id, t1.Count, t2.Count)
			}
		}
	}
}

func TestStart_PeriodicSnapshots(t *testing.T) {
	store := NewTemplateStore(DefaultDrainConfig())
	tmpPath := filepath.Join(os.TempDir(), "test-periodic-snapshot.json")
	defer os.Remove(tmpPath)

	// Use short interval for testing
	pm := NewPersistenceManager(store, tmpPath, 100*time.Millisecond)

	// Start persistence manager with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()

	// Add data before starting
	store.Process("default", "test message")

	// Start manager (blocks until context timeout)
	err := pm.Start(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got: %v", err)
	}

	// Should have created snapshot file
	if _, err := os.Stat(tmpPath); err != nil {
		t.Errorf("snapshot file not created: %v", err)
	}

	// Verify snapshot contains data
	data, _ := os.ReadFile(tmpPath)
	var snapshot SnapshotData
	json.Unmarshal(data, &snapshot)

	if len(snapshot.Namespaces) == 0 {
		t.Error("snapshot should contain namespaces")
	}
}

func TestStart_LoadsExistingSnapshot(t *testing.T) {
	// Create store and snapshot
	store1 := NewTemplateStore(DefaultDrainConfig())
	store1.Process("default", "initial message")

	tmpPath := filepath.Join(os.TempDir(), "test-load-on-start.json")
	defer os.Remove(tmpPath)

	pm1 := NewPersistenceManager(store1, tmpPath, time.Minute)
	pm1.Snapshot()

	// Create new store and start manager
	store2 := NewTemplateStore(DefaultDrainConfig())
	pm2 := NewPersistenceManager(store2, tmpPath, time.Hour) // long interval

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	pm2.Start(ctx)

	// Store2 should have loaded the snapshot
	templates, err := store2.ListTemplates("default")
	if err != nil {
		t.Fatalf("failed to list templates: %v", err)
	}

	if len(templates) == 0 {
		t.Error("templates should be loaded from snapshot")
	}
}

func TestStop(t *testing.T) {
	store := NewTemplateStore(DefaultDrainConfig())
	tmpPath := filepath.Join(os.TempDir(), "test-stop-snapshot.json")
	defer os.Remove(tmpPath)

	pm := NewPersistenceManager(store, tmpPath, time.Hour) // long interval

	// Start manager in goroutine
	ctx := context.Background()
	done := make(chan error)
	go func() {
		done <- pm.Start(ctx)
	}()

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	// Add data
	store.Process("default", "test before stop")

	// Stop manager
	pm.Stop()

	// Wait for Start() to return
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Start returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Error("Start() did not return after Stop()")
	}

	// Verify final snapshot was created
	if _, err := os.Stat(tmpPath); err != nil {
		t.Error("final snapshot not created")
	}
}
