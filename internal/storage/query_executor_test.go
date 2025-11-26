package storage

import (
	"testing"
	"time"

	"github.com/moritz/rpk/internal/models"
)

func TestNewQueryExecutor(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	executor := NewQueryExecutor(storage)
	if executor == nil {
		t.Fatal("expected non-nil QueryExecutor")
	}
	if executor.storage != storage {
		t.Error("storage not set correctly")
	}
	if executor.filterEngine == nil {
		t.Error("filterEngine not initialized")
	}
	if executor.metadataIndex == nil {
		t.Error("metadataIndex not initialized")
	}
	if executor.indexManager == nil {
		t.Error("indexManager not initialized")
	}
}

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
		createTestEvent("pod1", "default", "Pod", now.Add(-30*time.Minute).UnixNano()),
		createTestEvent("pod2", "default", "Pod", now.Add(-20*time.Minute).UnixNano()),
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
		createTestEvent("pod1", "default", "Pod", now.Add(-30*time.Minute).UnixNano()),
		createTestEvent("pod2", "default", "Pod", now.Add(-20*time.Minute).UnixNano()),
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
	event := createTestEvent("pod1", "default", "Pod", now.UnixNano())

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
		createTestEvent("pod1", "default", "Pod", now.Add(-30*time.Minute).UnixNano()),
		createTestEvent("pod2", "default", "Pod", now.Add(-20*time.Minute).UnixNano()),
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
	event := createTestEvent("pod1", "default", "Pod", now.UnixNano())

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
		createTestEvent("pod1", "default", "Pod", now.Add(-2*time.Hour).UnixNano()),
		createTestEvent("pod2", "default", "Pod", now.Add(-30*time.Minute).UnixNano()),
		createTestEvent("pod3", "default", "Pod", now.Add(-10*time.Minute).UnixNano()),
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
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write events and close to create multiple files
	now := time.Now()
	baseTime := now.Unix()

	// Create first file
	for i := 0; i < 5; i++ {
		event := createTestEvent("pod1", "default", "Pod", now.Add(-2*time.Hour).Add(time.Duration(i)*time.Minute).UnixNano())
		if err := storage.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	// Close to finalize first file
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Reopen to create new file
	storage, err = New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to recreate storage: %v", err)
	}
	defer storage.Close()

	// Create second file
	for i := 0; i < 5; i++ {
		event := createTestEvent("pod2", "default", "Pod", now.Add(-30*time.Minute).Add(time.Duration(i)*time.Minute).UnixNano())
		if err := storage.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event: %v", err)
		}
	}

	// Close to finalize
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Reopen for querying
	storage, err = New(tmpDir, 1024*1024)
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
		createTestEvent("pod1", "default", "Pod", now.Add(-30*time.Minute).UnixNano()),
		createTestEvent("pod2", "kube-system", "Pod", now.Add(-20*time.Minute).UnixNano()),
		createTestEvent("pod3", "default", "Pod", now.Add(-10*time.Minute).UnixNano()),
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
		if event.Resource.Namespace != "default" {
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
		createTestEvent("pod1", "default", "Pod", now.Add(-30*time.Minute).UnixNano()),
		createTestEvent("svc1", "default", "Service", now.Add(-20*time.Minute).UnixNano()),
		createTestEvent("pod2", "kube-system", "Pod", now.Add(-10*time.Minute).UnixNano()),
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
		Filters:        models.QueryFilters{Kind: "Pod", Namespace: "default"},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Should only get Pods from default namespace
	for _, event := range result.Events {
		if event.Resource.Kind != "Pod" {
			t.Errorf("expected Pod, got %s", event.Resource.Kind)
		}
		if event.Resource.Namespace != "default" {
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
		event := createTestEvent("pod1", "default", "Pod", now.Add(time.Duration(-i)*time.Minute).UnixNano())
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

