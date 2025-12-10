package storage

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
)

const (
	namespaceDefault = "default"
)

func TestQueryExecutorExecute_EmptyStorage(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	executor := NewQueryExecutor(storage)
	now := time.Now().Unix()
	query := &models.QueryRequest{
		StartTimestamp: now - 3600,
		EndTimestamp:   now,
		Filters:        models.QueryFilters{},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Count != 0 {
		t.Errorf("expected 0 events, got %d", result.Count)
	}
	if len(result.Events) != 0 {
		t.Errorf("expected empty events slice, got %d", len(result.Events))
	}
	if result.FilesSearched != 0 {
		t.Errorf("expected 0 files searched, got %d", result.FilesSearched)
	}
}

func TestQueryExecutorExecute_WithEvents(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write some events
	now := time.Now()
	baseTime := now.Unix()
	events := []*models.Event{
		createTestEvent("pod1", "default", kindPod, now.Add(-30*time.Minute).UnixNano()),
		createTestEvent("pod2", "default", kindPod, now.Add(-20*time.Minute).UnixNano()),
		createTestEvent("svc1", "default", "Service", now.Add(-10*time.Minute).UnixNano()),
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

	// Reopen storage to read from files
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

	if result.Count < 3 {
		t.Errorf("expected at least 3 events, got %d", result.Count)
	}
	if result.ExecutionTimeMs < 0 {
		t.Error("execution time should be non-negative")
	}
}

func TestQueryExecutorExecute_WithFilters(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write events with different kinds
	now := time.Now()
	baseTime := now.Unix()
	events := []*models.Event{
		createTestEvent("pod1", "default", kindPod, now.Add(-30*time.Minute).UnixNano()),
		createTestEvent("pod2", "default", kindPod, now.Add(-20*time.Minute).UnixNano()),
		createTestEvent("svc1", "default", "Service", now.Add(-10*time.Minute).UnixNano()),
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

	executor := NewQueryExecutor(storage)

	// Query for Pods only
	query := &models.QueryRequest{
		StartTimestamp: baseTime - 3600,
		EndTimestamp:   baseTime,
		Filters:        models.QueryFilters{Kind: kindPod},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Should only get Pod events
	for _, event := range result.Events {
		if event.Resource.Kind != kindPod {
			t.Errorf("expected Pod, got %s", event.Resource.Kind)
		}
	}
}

func TestQueryExecutorExecute_InvalidQuery(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	executor := NewQueryExecutor(storage)

	// Invalid query: start > end
	query := &models.QueryRequest{
		StartTimestamp: 1000,
		EndTimestamp:   500,
		Filters:        models.QueryFilters{},
	}

	_, err = executor.Execute(query)
	if err == nil {
		t.Error("expected error for invalid query")
	}
}

func TestQueryExecutorExecute_WithInMemoryEvents(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write events (they'll be in memory until segment is finalized)
	now := time.Now()
	baseTime := now.Unix()
	event := createTestEvent("pod1", "default", kindPod, now.UnixNano())

	if err := storage.WriteEvent(event); err != nil {
		t.Fatalf("failed to write event: %v", err)
	}

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

	// Should find the in-memory event
	if result.Count == 0 {
		t.Error("expected to find in-memory event")
	}
}

func TestQueryExecutorQueryCount(t *testing.T) {
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
		createTestEvent("pod1", "default", kindPod, now.Add(-30*time.Minute).UnixNano()),
		createTestEvent("pod2", "default", kindPod, now.Add(-20*time.Minute).UnixNano()),
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
	query := &models.QueryRequest{
		StartTimestamp: baseTime - 3600,
		EndTimestamp:   baseTime,
		Filters:        models.QueryFilters{},
	}

	count, err := executor.QueryCount(query)
	if err != nil {
		t.Fatalf("QueryCount failed: %v", err)
	}

	if count < 2 {
		t.Errorf("expected at least 2 events, got %d", count)
	}
}

func TestQueryExecutorQueryIncompleteFile(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write an event but don't close (file will be incomplete)
	now := time.Now()
	baseTime := now.Unix()
	event := createTestEvent("pod1", "default", kindPod, now.UnixNano())

	if err := storage.WriteEvent(event); err != nil {
		t.Fatalf("failed to write event: %v", err)
	}

	// Don't close storage - file is incomplete

	executor := NewQueryExecutor(storage)
	query := &models.QueryRequest{
		StartTimestamp: baseTime - 100,
		EndTimestamp:   baseTime + 100,
		Filters:        models.QueryFilters{},
	}

	// Should handle incomplete file gracefully
	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("Execute should handle incomplete files: %v", err)
	}

	// Should still find in-memory events
	if result.Count == 0 {
		t.Error("expected to find in-memory events even with incomplete file")
	}
}

func TestQueryExecutorExecute_TimeRangeFiltering(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write events at different times
	now := time.Now()
	baseTime := now.Unix()
	events := []*models.Event{
		createTestEvent("pod1", "default", kindPod, now.Add(-2*time.Hour).UnixNano()),
		createTestEvent("pod2", "default", kindPod, now.Add(-30*time.Minute).UnixNano()),
		createTestEvent("pod3", "default", kindPod, now.Add(-10*time.Minute).UnixNano()),
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

	// Query for last hour only
	query := &models.QueryRequest{
		StartTimestamp: baseTime - 3600,
		EndTimestamp:   baseTime,
		Filters:        models.QueryFilters{},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Should only get events from last hour
	for _, event := range result.Events {
		eventTime := time.Unix(0, event.Timestamp).Unix()
		if eventTime < baseTime-3600 || eventTime > baseTime {
			t.Errorf("event outside time range: %d not in [%d, %d]", eventTime, baseTime-3600, baseTime)
		}
	}
}

func TestQueryExecutorExecute_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now()
	baseTime := now.Unix()

	// Create first file with hour timestamp 2 hours ago
	hour1 := now.Add(-2 * time.Hour)
	hour1Timestamp := time.Date(hour1.Year(), hour1.Month(), hour1.Day(), hour1.Hour(), 0, 0, 0, hour1.Location())
	file1Path := filepath.Join(tmpDir, fmt.Sprintf("%04d-%02d-%02d-%02d.bin",
		hour1.Year(), hour1.Month(), hour1.Day(), hour1.Hour()))

	file1, err := NewBlockStorageFile(file1Path, hour1Timestamp.Unix(), 1024*1024)
	if err != nil {
		t.Fatalf("failed to create first file: %v", err)
	}

	// Write events to first file
	for i := 0; i < 5; i++ {
		event := createTestEvent("pod1", "default", kindPod, hour1.Add(time.Duration(i)*time.Minute).UnixNano())
		if err := file1.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event to file1: %v", err)
		}
	}

	// Close first file
	if err := file1.Close(); err != nil {
		t.Fatalf("failed to close file1: %v", err)
	}

	// Create second file with hour timestamp 1 hour ago
	hour2 := now.Add(-1 * time.Hour)
	hour2Timestamp := time.Date(hour2.Year(), hour2.Month(), hour2.Day(), hour2.Hour(), 0, 0, 0, hour2.Location())
	file2Path := filepath.Join(tmpDir, fmt.Sprintf("%04d-%02d-%02d-%02d.bin",
		hour2.Year(), hour2.Month(), hour2.Day(), hour2.Hour()))

	file2, err := NewBlockStorageFile(file2Path, hour2Timestamp.Unix(), 1024*1024)
	if err != nil {
		t.Fatalf("failed to create second file: %v", err)
	}

	// Write events to second file
	for i := 0; i < 5; i++ {
		event := createTestEvent("pod2", "default", kindPod, hour2.Add(time.Duration(i)*time.Minute).UnixNano())
		if err := file2.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event to file2: %v", err)
		}
	}

	// Close second file
	if err := file2.Close(); err != nil {
		t.Fatalf("failed to close file2: %v", err)
	}

	// Reopen for querying
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to recreate storage: %v", err)
	}
	defer storage.Close()

	executor := NewQueryExecutor(storage)
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
	if result.Count < 10 {
		t.Errorf("expected at least 10 events from multiple files, got %d", result.Count)
	}
	if result.FilesSearched < 2 {
		t.Errorf("expected at least 2 files searched, got %d", result.FilesSearched)
	}
}

func TestQueryExecutorExecute_NamespaceFilter(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write events in different namespaces
	now := time.Now()
	baseTime := now.Unix()
	events := []*models.Event{
		createTestEvent("pod1", "default", kindPod, now.Add(-30*time.Minute).UnixNano()),
		createTestEvent("pod2", "kube-system", kindPod, now.Add(-20*time.Minute).UnixNano()),
		createTestEvent("pod3", "default", kindPod, now.Add(-10*time.Minute).UnixNano()),
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

	// Query for default namespace only
	query := &models.QueryRequest{
		StartTimestamp: baseTime - 3600,
		EndTimestamp:   baseTime,
		Filters:        models.QueryFilters{Namespace: "default"},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Should only get events from default namespace
	for _, event := range result.Events {
		if event.Resource.Namespace != namespaceDefault {
			t.Errorf("expected default namespace, got %s", event.Resource.Namespace)
		}
	}
}

func TestQueryExecutorExecute_CombinedFilters(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write events with different combinations
	now := time.Now()
	baseTime := now.Unix()
	events := []*models.Event{
		createTestEvent("pod1", "default", kindPod, now.Add(-30*time.Minute).UnixNano()),
		createTestEvent("svc1", "default", "Service", now.Add(-20*time.Minute).UnixNano()),
		createTestEvent("pod2", "kube-system", kindPod, now.Add(-10*time.Minute).UnixNano()),
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

	// Query for Pods in default namespace
	query := &models.QueryRequest{
		StartTimestamp: baseTime - 3600,
		EndTimestamp:   baseTime,
		Filters:        models.QueryFilters{Kind: kindPod, Namespace: "default"},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Should only get Pods from default namespace
	for _, event := range result.Events {
		if event.Resource.Kind != kindPod {
			t.Errorf("expected Pod, got %s", event.Resource.Kind)
		}
		if event.Resource.Namespace != namespaceDefault {
			t.Errorf("expected default namespace, got %s", event.Resource.Namespace)
		}
	}
}

func TestQueryExecutorExecute_ResultStatistics(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write events
	now := time.Now()
	baseTime := now.Unix()
	for i := 0; i < 10; i++ {
		event := createTestEvent("pod1", "default", kindPod, now.Add(time.Duration(-i)*time.Minute).UnixNano())
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
	if result.SegmentsSkipped < 0 {
		t.Error("segments skipped should be non-negative")
	}
	if result.FilesSearched < 0 {
		t.Error("files searched should be non-negative")
	}
}

func TestQueryExecutorExecute_EmptyResult(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	executor := NewQueryExecutor(storage)
	now := time.Now().Unix()

	// Query for future time range (no events)
	query := &models.QueryRequest{
		StartTimestamp: now + 3600,
		EndTimestamp:   now + 7200,
		Filters:        models.QueryFilters{},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result.Count != 0 {
		t.Errorf("expected 0 events, got %d", result.Count)
	}
	if len(result.Events) != 0 {
		t.Errorf("expected empty events slice, got %d", len(result.Events))
	}
}

func TestQueryExecutorExecute_NonExistentDirectory(t *testing.T) {
	// Use a non-existent directory path
	nonExistentPath := "/tmp/non-existent-storage-test-" + time.Now().Format("20060102150405")
	storage, err := New(nonExistentPath, 1024*1024)
	if err != nil {
		t.Fatalf("New should create directory: %v", err)
	}
	defer storage.Close()

	executor := NewQueryExecutor(storage)
	now := time.Now().Unix()
	query := &models.QueryRequest{
		StartTimestamp: now - 3600,
		EndTimestamp:   now,
		Filters:        models.QueryFilters{},
	}

	// Should handle empty directory gracefully
	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("Execute should handle empty directory: %v", err)
	}

	if result.Count != 0 {
		t.Errorf("expected 0 events in empty directory, got %d", result.Count)
	}
}

func TestQueryExecutorExecute_OpenFileWithoutIndexAndClosedFile(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now()
	baseTime := now.Unix()

	// === PART 1: Create and close a file with some events (has index/footer) ===
	hour1 := now.Add(-2 * time.Hour)
	hour1Timestamp := time.Date(hour1.Year(), hour1.Month(), hour1.Day(), hour1.Hour(), 0, 0, 0, hour1.Location())
	closedFilePath := filepath.Join(tmpDir, fmt.Sprintf("%04d-%02d-%02d-%02d.bin",
		hour1.Year(), hour1.Month(), hour1.Day(), hour1.Hour()))

	closedFile, err := NewBlockStorageFile(closedFilePath, hour1Timestamp.Unix(), 1024*1024)
	if err != nil {
		t.Fatalf("failed to create closed file: %v", err)
	}

	// Write events to closed file
	closedFileEvents := []*models.Event{
		createTestEvent("closed-pod1", "default", kindPod, hour1.Add(0*time.Minute).UnixNano()),
		createTestEvent("closed-pod2", "default", kindPod, hour1.Add(1*time.Minute).UnixNano()),
		createTestEvent("closed-svc1", "default", "Service", hour1.Add(2*time.Minute).UnixNano()),
	}
	for _, event := range closedFileEvents {
		if err := closedFile.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event to closed file: %v", err)
		}
	}

	// Close the file to finalize with footer and index
	if err := closedFile.Close(); err != nil {
		t.Fatalf("failed to close file: %v", err)
	}

	// === PART 2: Create a storage instance and write events to an open file (no index/footer) ===
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Write events that stay in the open file's buffer (not closed = no footer/index)
	openFileEvents := []*models.Event{
		createTestEvent("open-pod1", "default", kindPod, now.Add(0*time.Minute).UnixNano()),
		createTestEvent("open-pod2", "default", kindPod, now.Add(1*time.Minute).UnixNano()),
		createTestEvent("open-svc1", "kube-system", "Service", now.Add(2*time.Minute).UnixNano()),
	}
	for _, event := range openFileEvents {
		if err := storage.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event to open file: %v", err)
		}
	}

	// DON'T close storage - so the current file remains open with no footer/index

	// === PART 3: Query for all events (both closed file and open file) ===
	executor := NewQueryExecutor(storage)
	query := &models.QueryRequest{
		StartTimestamp: baseTime - 10800, // 3 hours ago
		EndTimestamp:   baseTime + 3600,  // 1 hour in future
		Filters:        models.QueryFilters{},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// === PART 4: Verify that we got events from BOTH the closed file AND the open file ===
	expectedEventCount := len(closedFileEvents) + len(openFileEvents) // 3 + 3 = 6
	if result.Count < int32(expectedEventCount) {
		t.Errorf("expected at least %d events (from closed and open files), got %d",
			expectedEventCount, result.Count)
	}

	// Verify we got events from the open file
	openFileEventNames := map[string]bool{
		"open-pod1": false,
		"open-pod2": false,
		"open-svc1": false,
	}
	closedFileEventNames := map[string]bool{
		"closed-pod1": false,
		"closed-pod2": false,
		"closed-svc1": false,
	}

	for _, event := range result.Events {
		if _, exists := openFileEventNames[event.Resource.Name]; exists {
			openFileEventNames[event.Resource.Name] = true
		}
		if _, exists := closedFileEventNames[event.Resource.Name]; exists {
			closedFileEventNames[event.Resource.Name] = true
		}
	}

	// Check that we got events from the open file
	for eventName, found := range openFileEventNames {
		if !found {
			t.Errorf("event from OPEN file '%s' was not found in query results - BUG CONFIRMED: open files without index are not being queried", eventName)
		}
	}

	// Check that we got events from the closed file too
	for eventName, found := range closedFileEventNames {
		if !found {
			t.Errorf("event from CLOSED file '%s' was not found in query results", eventName)
		}
	}

	storage.Close()
}

func TestQueryExecutorExecute_OpenFileRestoredBlocksAndNewEvents(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now()
	baseTime := now.Unix()

	// === PART 1: Create initial storage and write some events, then close ===
	storage1, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create initial storage: %v", err)
	}

	initialEvents := []*models.Event{
		createTestEvent("initial-pod1", "default", kindPod, now.Add(-30*time.Minute).UnixNano()),
		createTestEvent("initial-pod2", "default", kindPod, now.Add(-25*time.Minute).UnixNano()),
	}
	for _, event := range initialEvents {
		if err := storage1.WriteEvent(event); err != nil {
			t.Fatalf("failed to write initial event: %v", err)
		}
	}

	// Close to finalize the file with index/footer
	if err := storage1.Close(); err != nil {
		t.Fatalf("failed to close initial storage: %v", err)
	}

	// === PART 2: Reopen storage (file now has restored blocks + index) and add new events ===
	storage2, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to reopen storage: %v", err)
	}

	// Add new events to the reopened file (has restored blocks but new events in buffer)
	newEvents := []*models.Event{
		createTestEvent("new-pod1", "default", kindPod, now.Add(-10*time.Minute).UnixNano()),
		createTestEvent("new-svc1", "kube-system", "Service", now.Add(-5*time.Minute).UnixNano()),
	}
	for _, event := range newEvents {
		if err := storage2.WriteEvent(event); err != nil {
			t.Fatalf("failed to write new event: %v", err)
		}
	}

	// Don't close storage2 - file has restored blocks + new unbuffered events

	// === PART 3: Query for all events ===
	executor := NewQueryExecutor(storage2)
	query := &models.QueryRequest{
		StartTimestamp: baseTime - 7200, // 2 hours ago
		EndTimestamp:   baseTime + 3600, // 1 hour in future
		Filters:        models.QueryFilters{},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// === PART 4: Verify we get BOTH initial events (restored blocks) and new events (buffer) ===
	expectedEventCount := len(initialEvents) + len(newEvents) // 2 + 2 = 4
	if result.Count < int32(expectedEventCount) {
		t.Errorf("expected at least %d events (initial + new), got %d",
			expectedEventCount, result.Count)
	}

	// Verify all events are present
	allEventNames := map[string]bool{
		"initial-pod1": false,
		"initial-pod2": false,
		"new-pod1":     false,
		"new-svc1":     false,
	}

	for _, event := range result.Events {
		if _, exists := allEventNames[event.Resource.Name]; exists {
			allEventNames[event.Resource.Name] = true
		}
	}

	for eventName, found := range allEventNames {
		if !found {
			t.Errorf("event '%s' was not found in query results", eventName)
		}
	}

	storage2.Close()
}

func TestQueryExecutorExecute_OnlyRestoredBlocksNoNewEvents(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now()
	baseTime := now.Unix()

	// === PART 1: Create initial storage and write some events, then close ===
	storage1, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create initial storage: %v", err)
	}

	initialEvents := []*models.Event{
		createTestEvent("initial-pod1", "default", kindPod, now.Add(-30*time.Minute).UnixNano()),
		createTestEvent("initial-pod2", "default", kindPod, now.Add(-25*time.Minute).UnixNano()),
		createTestEvent("initial-svc1", "default", "Service", now.Add(-20*time.Minute).UnixNano()),
	}
	for _, event := range initialEvents {
		if err := storage1.WriteEvent(event); err != nil {
			t.Fatalf("failed to write initial event: %v", err)
		}
	}

	// Close to finalize the file with index/footer
	if err := storage1.Close(); err != nil {
		t.Fatalf("failed to close initial storage: %v", err)
	}

	// === PART 2: Reopen storage (file now has restored blocks) ===
	storage2, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to reopen storage: %v", err)
	}

	// DON'T add any new events - just restored blocks, no new buffer events
	// Don't close storage2 - file has restored blocks but no new events

	// === PART 3: Query for restored events only ===
	executor := NewQueryExecutor(storage2)
	query := &models.QueryRequest{
		StartTimestamp: baseTime - 7200, // 2 hours ago
		EndTimestamp:   baseTime + 3600, // 1 hour in future
		Filters:        models.QueryFilters{},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// === PART 4: Verify we get ONLY the restored events (not new events since there are none) ===
	expectedEventCount := len(initialEvents) // 3
	if result.Count < int32(expectedEventCount) {
		t.Errorf("expected at least %d restored events (no new events were added), got %d",
			expectedEventCount, result.Count)
	}

	// Verify all initial events are present
	allEventNames := map[string]bool{
		"initial-pod1": false,
		"initial-pod2": false,
		"initial-svc1": false,
	}

	for _, event := range result.Events {
		if _, exists := allEventNames[event.Resource.Name]; exists {
			allEventNames[event.Resource.Name] = true
		}
	}

	for eventName, found := range allEventNames {
		if !found {
			t.Errorf("restored event '%s' was not found in query results - BUG: restored blocks from open file are not accessible", eventName)
		}
	}

	storage2.Close()
}

func TestQueryExecutorExecute_WithNamespaceFilterOnOpenFile(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now()
	baseTime := now.Unix()

	// === PART 1: Create and close a file with events in different namespaces ===
	storage1, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	closedFileEvents := []*models.Event{
		createTestEvent("pod1", "default", kindPod, now.Add(-30*time.Minute).UnixNano()),
		createTestEvent("pod2", "kube-system", kindPod, now.Add(-25*time.Minute).UnixNano()),
	}
	for _, event := range closedFileEvents {
		if err := storage1.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}
	if err := storage1.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// === PART 2: Reopen and add events to open file with different namespaces ===
	storage2, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to reopen storage: %v", err)
	}

	openFileEvents := []*models.Event{
		createTestEvent("pod3", "default", kindPod, now.Add(-10*time.Minute).UnixNano()),
		createTestEvent("pod4", "kube-system", kindPod, now.Add(-5*time.Minute).UnixNano()),
	}
	for _, event := range openFileEvents {
		if err := storage2.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	// === PART 3: Query with namespace filter ===
	executor := NewQueryExecutor(storage2)
	query := &models.QueryRequest{
		StartTimestamp: baseTime - 7200,
		EndTimestamp:   baseTime + 3600,
		Filters:        models.QueryFilters{Namespace: "default"},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// === PART 4: Verify we get filtered events from BOTH restored blocks and open buffer ===
	// Expected: pod1 (closed, default) + pod3 (open, default) = 2
	expectedEventCount := 2
	if result.Count < int32(expectedEventCount) {
		t.Errorf("expected at least %d events from 'default' namespace, got %d",
			expectedEventCount, result.Count)
	}

	// Verify all events are from the correct namespace
	for _, event := range result.Events {
		if event.Resource.Namespace != namespaceDefault {
			t.Errorf("got event from '%s' namespace, expected 'default'", event.Resource.Namespace)
		}
	}

	// Verify we got the specific events
	expectedNames := map[string]bool{
		"pod1": false,
		"pod3": false,
	}

	for _, event := range result.Events {
		if _, exists := expectedNames[event.Resource.Name]; exists {
			expectedNames[event.Resource.Name] = true
		}
	}

	for eventName, found := range expectedNames {
		if !found {
			t.Errorf("expected event '%s' not found in query results", eventName)
		}
	}

	storage2.Close()
}

func TestQueryExecutorExecute_CurrentFileStillBeingWritten(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now()
	baseTime := now.Unix()

	// === PART 1: Create storage and write events WITHOUT closing ===
	// This simulates a fresh start where the current file is still being written
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Write multiple events that will be flushed to disk in blocks
	// but the file will NOT have a footer (because storage is still open)
	events := []*models.Event{
		createTestEvent("pod1", "default", kindPod, now.Add(-5*time.Minute).UnixNano()),
		createTestEvent("pod2", "default", kindPod, now.Add(-4*time.Minute).UnixNano()),
		createTestEvent("pod3", "default", kindPod, now.Add(-3*time.Minute).UnixNano()),
		createTestEvent("pod4", "default", kindPod, now.Add(-2*time.Minute).UnixNano()),
		createTestEvent("pod5", "default", kindPod, now.Add(-1*time.Minute).UnixNano()),
	}
	for _, event := range events {
		if err := storage.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	// === PART 2: Query WITHOUT closing storage ===
	// The file is still open, has no footer, but has events in memory + on disk
	executor := NewQueryExecutor(storage)
	query := &models.QueryRequest{
		StartTimestamp: baseTime - 600, // 10 minutes ago
		EndTimestamp:   baseTime,       // now
		Filters:        models.QueryFilters{},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// === PART 3: Verify we can query the current file while it's still being written ===
	// Expected: All 5 events should be retrievable
	expectedEventCount := len(events)
	if result.Count < int32(expectedEventCount) {
		t.Errorf("BUG CONFIRMED: expected %d events from current file while it's being written, got %d. "+
			"This is the actual bug - the current file should be queryable even without a footer!",
			expectedEventCount, result.Count)
	}

	// Verify all events are present
	foundEvents := make(map[string]bool)
	for _, event := range result.Events {
		foundEvents[event.Resource.Name] = true
	}

	for i := 1; i <= 5; i++ {
		podName := fmt.Sprintf("pod%d", i)
		if !foundEvents[podName] {
			t.Errorf("event '%s' not found in results", podName)
		}
	}

	storage.Close()
}

func TestQueryExecutorExecute_CurrentFileFinalizedBlocksNotInMemory(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now()
	baseTime := now.Unix()

	// === PART 1: Write events with a smaller block size to force finalization ===
	// Using a very small block size (1KB) to force events into multiple blocks
	smallBlockSize := int64(1024)
	storage, err := New(tmpDir, smallBlockSize)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Create large events that will force block finalization
	// Each event is ~150 bytes, so 1KB blocks will have ~6-7 events before finalizing
	largeEvents := make([]*models.Event, 20)
	for i := 0; i < 20; i++ {
		largeEvents[i] = createTestEvent(
			fmt.Sprintf("pod%d-with-long-name-to-make-event-bigger", i),
			"default",
			kindPod,
			now.Add(time.Duration(-20+i)*time.Minute).UnixNano(),
		)
	}

	// Write all events (some will be finalized into blocks, some in buffer)
	for _, event := range largeEvents {
		if err := storage.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	// At this point:
	// - Some events are in finalized blocks (written to disk, in s.currentFile.blocks)
	// - Some events are still in the currentBuffer
	// - No footer has been written (file is still open)

	// === PART 2: Query the current file while it's still being written ===
	executor := NewQueryExecutor(storage)
	query := &models.QueryRequest{
		StartTimestamp: baseTime - 2400, // 40 minutes ago
		EndTimestamp:   baseTime + 3600, // 1 hour in future
		Filters:        models.QueryFilters{},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// === PART 3: Verify we get ALL events (from both finalized blocks and buffer) ===
	// Expected: All 20 events should be retrievable
	expectedEventCount := len(largeEvents)
	if result.Count < int32(expectedEventCount) {
		t.Errorf("BUG: expected %d events (some from finalized blocks, some from buffer), got %d. "+
			"The bug is that finalized blocks are not being queried when file has no footer",
			expectedEventCount, result.Count)
	}

	// Verify all events are present
	foundEvents := make(map[string]bool)
	for _, event := range result.Events {
		foundEvents[event.Resource.Name] = true
	}

	for i := 0; i < 20; i++ {
		podName := fmt.Sprintf("pod%d-with-long-name-to-make-event-bigger", i)
		if !foundEvents[podName] {
			t.Errorf("event '%s' not found in results (likely in a finalized block that wasn't queried)", podName)
		}
	}

	storage.Close()
}
