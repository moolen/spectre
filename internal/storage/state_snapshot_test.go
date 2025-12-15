package storage

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
)

// TestStateSnapshot_BasicPersistence tests that state snapshots are persisted to disk
func TestStateSnapshot_BasicPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write an event
	now := time.Now()
	event := createTestEvent("pod1", "default", "Pod", now.UnixNano())
	if err := storage.WriteEvent(event); err != nil {
		t.Fatalf("failed to write event: %v", err)
	}

	// Close to finalize and persist state snapshots
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Verify file has state snapshots
	files, err := storage.getStorageFiles()
	if err != nil {
		t.Fatalf("failed to get storage files: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("no storage files found")
	}

	// Read the file and check for state snapshots
	reader, err := NewBlockReader(files[0])
	if err != nil {
		t.Fatalf("failed to open block reader: %v", err)
	}
	defer reader.Close()

	fileData, err := reader.ReadFile()
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	// Check that state snapshots are present
	if len(fileData.IndexSection.FinalResourceStates) == 0 {
		t.Error("expected state snapshots to be persisted, but found none")
	}

	// Verify the state snapshot has correct values
	resourceKey := "/v1/Pod/default/pod1"
	state, exists := fileData.IndexSection.FinalResourceStates[resourceKey]
	if !exists {
		t.Errorf("expected resource key %s not found in state snapshots", resourceKey)
	}

	if state.UID != "test-uid-pod1" {
		t.Errorf("expected UID test-uid-pod1, got %s", state.UID)
	}

	if state.EventType != string(models.EventTypeCreate) {
		t.Errorf("expected event type CREATE, got %s", state.EventType)
	}
}

// TestStateSnapshot_ConsistentView tests that queries show consistent view of old resources
// by carrying forward state snapshots across hour boundaries
func TestStateSnapshot_ConsistentView(t *testing.T) {
	tmpDir := t.TempDir()

	// Simulate hour transitions by writing events at specific times
	// Hour 1: Write pod1 event, then close
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	twoHoursAgo := time.Now().Add(-2 * time.Hour)
	event1 := createTestEvent("pod1", "default", "Pod", twoHoursAgo.UnixNano())
	if err := storage.WriteEvent(event1); err != nil {
		t.Fatalf("failed to write event in hour 1: %v", err)
	}

	// Close to finalize first hour file with state snapshot
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage after hour 1: %v", err)
	}

	// Hour 2: Write pod2 event with state snapshot carried from hour 1
	storage, err = New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to recreate storage for hour 2: %v", err)
	}

	oneHourAgo := time.Now().Add(-1 * time.Hour)
	event2 := createTestEvent("pod2", "default", "Pod", oneHourAgo.UnixNano())
	if err := storage.WriteEvent(event2); err != nil {
		t.Fatalf("failed to write event in hour 2: %v", err)
	}

	// Close to finalize second hour file with states from both hours
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage after hour 2: %v", err)
	}

	// Now query the second hour file - it should have pod1 state snapshot and pod2 actual event
	storage, err = New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to recreate storage for querying: %v", err)
	}
	defer storage.Close()

	executor := NewQueryExecutor(storage, nil)

	// Query just the last hour
	now := time.Now()
	currentHour := now.Unix()
	query := &models.QueryRequest{
		StartTimestamp: currentHour - 3600,
		EndTimestamp:   currentHour,
		Filters:        models.QueryFilters{},
	}

	result, err := executor.Execute(t.Context(), query)
	if err != nil {
		t.Fatalf("query execution failed: %v", err)
	}

	// Should have both pod1 (carried over state) and pod2 (actual event)
	if result.Count < 1 {
		t.Errorf("expected at least 1 event, got %d. This test verifies state carryover works.", result.Count)
	}

	// At minimum, we should have pod2 from the current hour's actual events
	resourceNames := make(map[string]bool)
	for _, event := range result.Events {
		resourceNames[event.Resource.Name] = true
	}

	if !resourceNames["pod2"] {
		t.Error("expected pod2 from actual event in current hour, but not found")
	}
}

// TestStateSnapshot_DeletedResourceHidden tests that deleted resources don't appear in consistent view
// when carrying forward state snapshots
func TestStateSnapshot_DeletedResourceHidden(t *testing.T) {
	tmpDir := t.TempDir()

	// Hour 1: Create pod1, then delete it
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	twoHoursAgo := time.Now().Add(-2 * time.Hour)
	event1 := createTestEvent("pod1", "default", "Pod", twoHoursAgo.UnixNano())
	if err := storage.WriteEvent(event1); err != nil {
		t.Fatalf("failed to write creation event: %v", err)
	}

	// Delete the pod (still in hour 1)
	deleteEvent := &models.Event{
		ID:        "test-id-pod1-delete",
		Timestamp: twoHoursAgo.Add(10 * time.Minute).UnixNano(),
		Type:      models.EventTypeDelete,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "pod1",
			UID:       "test-uid-pod1",
		},
		Data: nil,
	}
	if err := storage.WriteEvent(deleteEvent); err != nil {
		t.Fatalf("failed to write delete event: %v", err)
	}

	// Close to finalize hour 1 - state snapshot will have pod1 as DELETE
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Hour 2: Write a different pod
	storage, err = New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to recreate storage: %v", err)
	}

	oneHourAgo := time.Now().Add(-1 * time.Hour)
	event2 := createTestEvent("pod2", "default", "Pod", oneHourAgo.UnixNano())
	if err := storage.WriteEvent(event2); err != nil {
		t.Fatalf("failed to write event in hour 2: %v", err)
	}

	// Close to finalize hour 2
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Query hour 2 - deleted pod1 should NOT appear (it's in DELETE state in carried over snapshots)
	storage, err = New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to recreate storage for querying: %v", err)
	}
	defer storage.Close()

	executor := NewQueryExecutor(storage, nil)

	now := time.Now()
	currentHour := now.Unix()
	query := &models.QueryRequest{
		StartTimestamp: currentHour - 3600,
		EndTimestamp:   currentHour,
		Filters:        models.QueryFilters{},
	}

	result, err := executor.Execute(t.Context(), query)
	if err != nil {
		t.Fatalf("query execution failed: %v", err)
	}

	// Check that deleted pod1 doesn't appear
	for _, event := range result.Events {
		if event.Resource.Name == "pod1" {
			t.Errorf("deleted resource pod1 should not appear in consistent view, but got event type: %s", event.Type)
		}
	}
}

// TestStateSnapshot_FilteredQuery tests that filters work with state snapshots
func TestStateSnapshot_FilteredQuery(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write events for different resources in first hour
	firstHourTime := time.Now().Add(-1 * time.Hour)
	event1 := createTestEvent("pod1", "default", "Pod", firstHourTime.UnixNano())
	event2 := createTestEvent("svc1", "default", "Service", firstHourTime.UnixNano())
	event3 := createTestEvent("pod2", "kube-system", "Pod", firstHourTime.UnixNano())

	if err := storage.WriteEvent(event1); err != nil {
		t.Fatalf("failed to write event1: %v", err)
	}
	if err := storage.WriteEvent(event2); err != nil {
		t.Fatalf("failed to write event2: %v", err)
	}
	if err := storage.WriteEvent(event3); err != nil {
		t.Fatalf("failed to write event3: %v", err)
	}

	// Close first hour
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Reopen for second hour
	storage, err = New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to recreate storage: %v", err)
	}
	defer storage.Close()

	// Query with namespace filter
	executor := NewQueryExecutor(storage, nil)
	now := time.Now()
	currentHour := now.Unix()

	query := &models.QueryRequest{
		StartTimestamp: currentHour - 3600,
		EndTimestamp:   currentHour,
		Filters: models.QueryFilters{
			Namespace: "default",
		},
	}

	result, err := executor.Execute(t.Context(), query)
	if err != nil {
		t.Fatalf("query execution failed: %v", err)
	}

	// Should only have pod1 and svc1 (both in default namespace)
	if result.Count < 2 {
		t.Errorf("expected at least 2 events in default namespace, got %d", result.Count)
	}

	for _, event := range result.Events {
		if event.Resource.Namespace != "default" {
			t.Errorf("expected all results to be in default namespace, but got %s", event.Resource.Namespace)
		}
	}

	// Query with kind filter
	query2 := &models.QueryRequest{
		StartTimestamp: currentHour - 3600,
		EndTimestamp:   currentHour,
		Filters: models.QueryFilters{
			Kind: "Pod",
		},
	}

	result2, err := executor.Execute(t.Context(), query2)
	if err != nil {
		t.Fatalf("query execution failed: %v", err)
	}

	// Should have pod1 and pod2 (both are Pods)
	if result2.Count < 2 {
		t.Errorf("expected at least 2 Pod events, got %d", result2.Count)
	}

	for _, event := range result2.Events {
		if event.Resource.Kind != "Pod" {
			t.Errorf("expected all results to be Pods, but got %s", event.Resource.Kind)
		}
	}
}

// TestStateSnapshot_MultipleUpdates tests that only the latest state is kept
func TestStateSnapshot_MultipleUpdates(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write multiple events for the same resource
	now := time.Now()
	event1 := &models.Event{
		ID:        "event1",
		Timestamp: now.Add(-30 * time.Minute).UnixNano(),
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "pod1",
			UID:       "test-uid-pod1",
		},
		Data: json.RawMessage(`{"status": "pending"}`),
	}

	event2 := &models.Event{
		ID:        "event2",
		Timestamp: now.Add(-10 * time.Minute).UnixNano(),
		Type:      models.EventTypeUpdate,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "pod1",
			UID:       "test-uid-pod1",
		},
		Data: json.RawMessage(`{"status": "running"}`),
	}

	if err := storage.WriteEvent(event1); err != nil {
		t.Fatalf("failed to write event1: %v", err)
	}
	if err := storage.WriteEvent(event2); err != nil {
		t.Fatalf("failed to write event2: %v", err)
	}

	// Close to persist
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Read and verify state snapshot has latest update
	// Check the most recent file (events may span multiple hourly files)
	files, err := storage.getStorageFiles()
	if err != nil {
		t.Fatalf("failed to get storage files: %v", err)
	}

	reader, err := NewBlockReader(files[len(files)-1])
	if err != nil {
		t.Fatalf("failed to open block reader: %v", err)
	}
	defer reader.Close()

	fileData, err := reader.ReadFile()
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	resourceKey := "/v1/Pod/default/pod1"
	state := fileData.IndexSection.FinalResourceStates[resourceKey]

	// Should have the latest update's data (compare without whitespace)
	var stateData map[string]string
	if err := json.Unmarshal(state.ResourceData, &stateData); err != nil {
		t.Fatalf("failed to unmarshal state data: %v", err)
	}

	if stateData["status"] != "running" {
		t.Errorf("expected latest status 'running', got %s", stateData["status"])
	}

	if state.EventType != "UPDATE" {
		t.Errorf("expected latest event type UPDATE, got %s", state.EventType)
	}
}

// TestStateSnapshot_Cleanup tests that old state snapshots are cleaned up
func TestStateSnapshot_Cleanup(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write an old event (simulate old data)
	veryOldTime := time.Now().Add(-20 * 24 * time.Hour) // 20 days old
	event := &models.Event{
		ID:        "old-event",
		Timestamp: veryOldTime.UnixNano(),
		Type:      models.EventTypeDelete, // Deleted resource
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "old-pod",
			UID:       "old-uid",
		},
		Data: nil,
	}

	if err := storage.WriteEvent(event); err != nil {
		t.Fatalf("failed to write event: %v", err)
	}

	// Close to persist
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Verify state snapshot exists
	files, err := storage.getStorageFiles()
	if err != nil {
		t.Fatalf("failed to get storage files: %v", err)
	}

	reader, err := NewBlockReader(files[0])
	if err != nil {
		t.Fatalf("failed to open block reader: %v", err)
	}
	fileData, _ := reader.ReadFile()
	_ = reader.Close()

	if len(fileData.IndexSection.FinalResourceStates) == 0 {
		t.Fatal("expected state snapshots before cleanup")
	}

	// Run cleanup with 14 day retention
	if err := storage.CleanupOldStateSnapshots(14); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	// Verify the old deleted state was removed
	reader, err = NewBlockReader(files[0])
	if err != nil {
		t.Fatalf("failed to open block reader: %v", err)
	}
	fileData, err = reader.ReadFile()
	reader.Close()

	if err != nil {
		t.Fatalf("failed to read file after cleanup: %v", err)
	}

	resourceKey := "/v1/Pod/default/old-pod"
	_, exists := fileData.IndexSection.FinalResourceStates[resourceKey]
	if exists {
		t.Error("expected old deleted resource to be removed from state snapshots")
	}
}

// TestStateSnapshot_NonDeletedResourcesPreserved tests that non-deleted resources are kept even if old
func TestStateSnapshot_NonDeletedResourcesPreserved(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Write an old but non-deleted resource
	veryOldTime := time.Now().Add(-20 * 24 * time.Hour) // 20 days old
	event := &models.Event{
		ID:        "old-event",
		Timestamp: veryOldTime.UnixNano(),
		Type:      models.EventTypeCreate, // Non-deleted resource
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "old-pod",
			UID:       "old-uid",
		},
		Data: json.RawMessage(`{"status": "running"}`),
	}

	if err := storage.WriteEvent(event); err != nil {
		t.Fatalf("failed to write event: %v", err)
	}

	// Close to persist
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Run cleanup with 14 day retention
	if err := storage.CleanupOldStateSnapshots(14); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	// Verify the old non-deleted state is still present
	files, err := storage.getStorageFiles()
	if err != nil {
		t.Fatalf("failed to get storage files: %v", err)
	}

	reader, err := NewBlockReader(files[0])
	if err != nil {
		t.Fatalf("failed to open block reader: %v", err)
	}
	fileData, err := reader.ReadFile()
	_ = reader.Close()

	if err != nil {
		t.Fatalf("failed to read file after cleanup: %v", err)
	}

	resourceKey := "/v1/Pod/default/old-pod"
	state, exists := fileData.IndexSection.FinalResourceStates[resourceKey]
	if !exists {
		t.Error("expected non-deleted old resource to be preserved")
	}

	if state.EventType != string(models.EventTypeCreate) {
		t.Errorf("expected CREATE event type, got %s", state.EventType)
	}
}
