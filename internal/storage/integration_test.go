package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/moritz/rpk/internal/models"
)

// TestIntegration_WriteAndQuery tests the complete flow of writing events and querying them
func TestIntegration_WriteAndQuery(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write events with various attributes
	now := time.Now()
	baseTime := now.Unix()
	events := []*models.Event{
		createTestEvent("pod1", "default", "Pod", now.Add(-30*time.Minute).UnixNano()),
		createTestEvent("pod2", "default", "Pod", now.Add(-20*time.Minute).UnixNano()),
		createTestEvent("svc1", "default", "Service", now.Add(-10*time.Minute).UnixNano()),
		createTestEvent("pod3", "kube-system", "Pod", now.Add(-5*time.Minute).UnixNano()),
	}

	for _, event := range events {
		if err := storage.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	// Close to finalize segments
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Reopen storage
	storage, err = New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to recreate storage: %v", err)
	}
	defer storage.Close()

	// Create query executor
	executor := NewQueryExecutor(storage)

	// Query all events
	query := &models.QueryRequest{
		StartTimestamp: baseTime - 3600,
		EndTimestamp:   baseTime,
		Filters:        models.QueryFilters{},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Count < 4 {
		t.Errorf("expected at least 4 events, got %d", result.Count)
	}

	// Verify all events are present
	eventMap := make(map[string]bool)
	for _, event := range result.Events {
		eventMap[event.Resource.Name] = true
	}

	expectedNames := []string{"pod1", "pod2", "svc1", "pod3"}
	for _, name := range expectedNames {
		if !eventMap[name] {
			t.Errorf("expected event %s not found", name)
		}
	}
}

// TestIntegration_FilteredQuery tests querying with filters
func TestIntegration_FilteredQuery(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write events
	now := time.Now()
	baseTime := now.Unix()
	events := []*models.Event{
		createTestEvent("pod1", "default", "Pod", now.Add(-30*time.Minute).UnixNano()),
		createTestEvent("pod2", "default", "Pod", now.Add(-20*time.Minute).UnixNano()),
		createTestEvent("svc1", "default", "Service", now.Add(-10*time.Minute).UnixNano()),
	}

	for _, event := range events {
		if err := storage.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	// Close to finalize
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Reopen
	storage, err = New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to recreate storage: %v", err)
	}
	defer storage.Close()

	executor := NewQueryExecutor(storage)

	// Query for Pods only
	query := &models.QueryRequest{
		StartTimestamp: baseTime - 3600,
		EndTimestamp:   baseTime,
		Filters:        models.QueryFilters{Kind: "Pod"},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Should only get Pod events
	for _, event := range result.Events {
		if event.Resource.Kind != "Pod" {
			t.Errorf("expected Pod, got %s", event.Resource.Kind)
		}
	}

	if result.Count != int32(len(result.Events)) {
		t.Errorf("Count (%d) doesn't match events length (%d)", result.Count, len(result.Events))
	}
}

// TestIntegration_InMemoryAndFileEvents tests querying both in-memory and file-based events
func TestIntegration_InMemoryAndFileEvents(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	now := time.Now()
	baseTime := now.Unix()

	// Write events and close to create file
	for i := 0; i < 5; i++ {
		event := createTestEvent("file-pod", "default", "Pod", now.Add(-2*time.Hour).Add(time.Duration(i)*time.Minute).UnixNano())
		if err := storage.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	// Close to finalize file
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Reopen and write in-memory events
	storage, err = New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to recreate storage: %v", err)
	}
	defer storage.Close()

	// Write in-memory events
	for i := 0; i < 3; i++ {
		event := createTestEvent("memory-pod", "default", "Pod", now.Add(time.Duration(i)*time.Second).UnixNano())
		if err := storage.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	executor := NewQueryExecutor(storage)

	// Query should find both file and in-memory events
	query := &models.QueryRequest{
		StartTimestamp: baseTime - 7200,
		EndTimestamp:   baseTime + 100,
		Filters:        models.QueryFilters{},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Should find events from both sources
	if result.Count < 8 {
		t.Errorf("expected at least 8 events (5 file + 3 memory), got %d", result.Count)
	}
}

// TestIntegration_MultipleFiles tests querying across multiple hourly files
func TestIntegration_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create first file
	storage1, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	now := time.Now()
	baseTime := now.Unix()

	// Write events to first file
	for i := 0; i < 10; i++ {
		event := createTestEvent("file1-pod", "default", "Pod", now.Add(-2*time.Hour).Add(time.Duration(i)*time.Minute).UnixNano())
		if err := storage1.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	// Close first file
	if err := storage1.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Create second file
	storage2, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create second storage: %v", err)
	}

	// Write events to second file
	for i := 0; i < 10; i++ {
		event := createTestEvent("file2-pod", "default", "Pod", now.Add(-30*time.Minute).Add(time.Duration(i)*time.Minute).UnixNano())
		if err := storage2.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	// Close second file
	if err := storage2.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Reopen for querying
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to recreate storage: %v", err)
	}
	defer storage.Close()

	executor := NewQueryExecutor(storage)

	// Query across both files
	query := &models.QueryRequest{
		StartTimestamp: baseTime - 7200,
		EndTimestamp:   baseTime,
		Filters:        models.QueryFilters{},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Should find events from both files
	if result.Count < 20 {
		t.Errorf("expected at least 20 events from multiple files, got %d", result.Count)
	}
	if result.FilesSearched < 2 {
		t.Errorf("expected at least 2 files searched, got %d", result.FilesSearched)
	}
}

// TestIntegration_ConcurrentWrites tests concurrent event writes
func TestIntegration_ConcurrentWrites(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	now := time.Now()
	baseTime := now.Unix()

	// Write events concurrently
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			event := createTestEvent("concurrent-pod", "default", "Pod", now.Add(time.Duration(id)*time.Second).UnixNano())
			done <- storage.WriteEvent(event)
		}(i)
	}

	// Wait for all writes
	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Fatalf("concurrent write failed: %v", err)
		}
	}

	// Query events
	executor := NewQueryExecutor(storage)
	query := &models.QueryRequest{
		StartTimestamp: baseTime - 100,
		EndTimestamp:   baseTime + 100,
		Filters:        models.QueryFilters{},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Should find all events
	if result.Count < 10 {
		t.Errorf("expected at least 10 events, got %d", result.Count)
	}
}

// TestIntegration_QueryStatistics tests that query statistics are accurate
func TestIntegration_QueryStatistics(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write events
	now := time.Now()
	baseTime := now.Unix()
	for i := 0; i < 20; i++ {
		event := createTestEvent("stat-pod", "default", "Pod", now.Add(time.Duration(-i)*time.Minute).UnixNano())
		if err := storage.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	// Close to finalize
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Reopen
	storage, err = New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to recreate storage: %v", err)
	}
	defer storage.Close()

	executor := NewQueryExecutor(storage)
	query := &models.QueryRequest{
		StartTimestamp: baseTime - 3600,
		EndTimestamp:   baseTime,
		Filters:        models.QueryFilters{},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify statistics
	if result.Count != int32(len(result.Events)) {
		t.Errorf("Count (%d) doesn't match events length (%d)", result.Count, len(result.Events))
	}
	if result.ExecutionTimeMs < 0 {
		t.Error("execution time should be non-negative")
	}
	if result.SegmentsScanned < 0 {
		t.Error("segments scanned should be non-negative")
	}
	if result.FilesSearched < 1 {
		t.Error("should have searched at least 1 file")
	}
}

// TestIntegration_StorageLifecycle tests the complete storage lifecycle
func TestIntegration_StorageLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Start storage
	ctx := context.Background()
	if err := storage.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Write events
	now := time.Now()
	for i := 0; i < 5; i++ {
		event := createTestEvent("lifecycle-pod", "default", "Pod", now.Add(time.Duration(i)*time.Second).UnixNano())
		if err := storage.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	// Get stats
	stats, err := storage.GetStorageStats()
	if err != nil {
		t.Fatalf("GetStorageStats failed: %v", err)
	}
	if stats["fileCount"].(int) == 0 {
		t.Error("expected at least one file")
	}

	// Stop storage
	if err := storage.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

// TestIntegration_FileManagement tests file listing and deletion
func TestIntegration_FileManagement(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write events
	now := time.Now()
	for i := 0; i < 5; i++ {
		event := createTestEvent("file-mgmt-pod", "default", "Pod", now.Add(time.Duration(i)*time.Second).UnixNano())
		if err := storage.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	// Close to finalize
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Reopen
	storage, err = New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to recreate storage: %v", err)
	}
	defer storage.Close()

	// List files
	files, err := storage.ListFiles()
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}
	if len(files) == 0 {
		t.Error("expected at least one file")
	}

	// Verify files exist
	for _, file := range files {
		if _, err := os.Stat(file); err != nil {
			t.Errorf("file %s does not exist: %v", file, err)
		}
	}

	// Create an old file manually
	oldFile := filepath.Join(tmpDir, "2020-01-01-00.bin")
	file, err := os.Create(oldFile)
	if err != nil {
		t.Fatalf("failed to create old file: %v", err)
	}
	file.Close()

	// Set old modification time
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatalf("failed to set file time: %v", err)
	}

	// Delete old files
	if err := storage.DeleteOldFiles(24); err != nil {
		t.Fatalf("DeleteOldFiles failed: %v", err)
	}

	// Verify old file was deleted
	if _, err := os.Stat(oldFile); err == nil {
		t.Error("expected old file to be deleted")
	}

	// Verify recent files still exist
	files, err = storage.ListFiles()
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}
	if len(files) == 0 {
		t.Error("expected recent files to still exist")
	}
}

