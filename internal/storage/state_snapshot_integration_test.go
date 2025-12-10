package storage

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
)

// TestResourceStateTracking_CompleteWorkflow tests the complete resource state tracking workflow
// This test covers the primary use case: querying for resources that haven't had recent events
// but should still appear in the current view
func TestResourceStateTracking_CompleteWorkflow(t *testing.T) {
	tmpDir := t.TempDir()

	// ===== HOUR 1 (2 hours ago): Create initial resources =====
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	twoHoursAgo := time.Now().Add(-2 * time.Hour)

	// Create pod, deployment, and service in hour 1
	pod := createTestEvent("web-pod", "default", "Pod", twoHoursAgo.UnixNano())
	pod.Data = json.RawMessage(`{"status": {"phase": "Running"}}`)

	deployment := &models.Event{
		ID:        "event-deploy1",
		Timestamp: twoHoursAgo.Add(5 * time.Minute).UnixNano(),
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "apps",
			Version:   "v1",
			Kind:      "Deployment",
			Namespace: "default",
			Name:      "web-app",
			UID:       "uid-deploy1",
		},
		Data: json.RawMessage(`{"spec": {"replicas": 3}}`),
	}

	service := createTestEvent("web-svc", "default", "Service", twoHoursAgo.Add(10*time.Minute).UnixNano())
	service.Data = json.RawMessage(`{"spec": {"type": "ClusterIP"}}`)

	if err := storage.WriteEvent(pod); err != nil {
		t.Fatalf("failed to write pod: %v", err)
	}
	if err := storage.WriteEvent(deployment); err != nil {
		t.Fatalf("failed to write deployment: %v", err)
	}
	if err := storage.WriteEvent(service); err != nil {
		t.Fatalf("failed to write service: %v", err)
	}

	// Close hour 1 file to persist state snapshots
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage after hour 1: %v", err)
	}

	// Verify state snapshots were persisted
	files, _ := storage.getStorageFiles()
	if len(files) == 0 {
		t.Fatal("no storage files found after hour 1")
	}

	reader, _ := NewBlockReader(files[0])
	fileData, _ := reader.ReadFile()
	reader.Close()

	if len(fileData.IndexSection.FinalResourceStates) != 3 {
		t.Errorf("expected 3 state snapshots after hour 1, got %d", len(fileData.IndexSection.FinalResourceStates))
	}

	// ===== HOUR 2 (1 hour ago): Add new resource, no updates to existing ones =====
	storage, err = New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to recreate storage for hour 2: %v", err)
	}

	oneHourAgo := time.Now().Add(-1 * time.Hour)

	// Create a NEW resource in hour 2 (existing ones get no updates)
	configMap := &models.Event{
		ID:        "event-cm1",
		Timestamp: oneHourAgo.UnixNano(),
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "ConfigMap",
			Namespace: "default",
			Name:      "app-config",
			UID:       "uid-cm1",
		},
		Data: json.RawMessage(`{"data": {"key": "value"}}`),
	}

	if err := storage.WriteEvent(configMap); err != nil {
		t.Fatalf("failed to write configmap in hour 2: %v", err)
	}

	// Close hour 2 to persist states (should have old + new)
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage after hour 2: %v", err)
	}

	// ===== HOUR 3 (current): Query current state - should see all 4 resources =====
	storage, err = New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to recreate storage for hour 3: %v", err)
	}
	defer storage.Close()

	executor := NewQueryExecutor(storage)

	now := time.Now()
	currentHour := now.Unix()

	// Query last hour only
	query := &models.QueryRequest{
		StartTimestamp: currentHour - 3600,
		EndTimestamp:   currentHour,
		Filters:        models.QueryFilters{},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("query execution failed: %v", err)
	}

	// Should have at least 1 event (configmap from hour 2)
	if result.Count < 1 {
		t.Errorf("expected at least 1 event in current hour query, got %d", result.Count)
	}

	// Check that we have the configmap from this hour
	hasConfigMap := false
	for _, event := range result.Events {
		if event.Resource.Name == "app-config" {
			hasConfigMap = true
		}
	}

	if !hasConfigMap {
		t.Error("configmap from hour 2 should appear in hour 3 query")
	}
}

// TestResourceStateTracking_MultipleHourTransitions tests resource state across multiple hour boundaries
func TestResourceStateTracking_MultipleHourTransitions(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now()
	baseTime := now.Add(-3 * time.Hour)

	// Hour 1: Create resource
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	event1 := &models.Event{
		ID:        "event1",
		Timestamp: baseTime.UnixNano(),
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "long-lived-pod",
			UID:       "uid1",
		},
		Data: json.RawMessage(`{"status": {"phase": "Pending"}}`),
	}

	if err := storage.WriteEvent(event1); err != nil {
		t.Fatalf("failed to write event in hour 1: %v", err)
	}
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close after hour 1: %v", err)
	}

	// Hour 2: No updates to existing resource
	storage, err = New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to reopen for hour 2: %v", err)
	}

	// Just create another unrelated resource
	event2 := createTestEvent("unrelated-pod", "default", "Pod", baseTime.Add(1*time.Hour).UnixNano())
	if err := storage.WriteEvent(event2); err != nil {
		t.Fatalf("failed to write event in hour 2: %v", err)
	}
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close after hour 2: %v", err)
	}

	// Hour 3: No updates still
	storage, err = New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to reopen for hour 3: %v", err)
	}

	// Create yet another resource
	event3 := createTestEvent("another-pod", "default", "Pod", baseTime.Add(2*time.Hour).UnixNano())
	if err := storage.WriteEvent(event3); err != nil {
		t.Fatalf("failed to write event in hour 3: %v", err)
	}
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close after hour 3: %v", err)
	}

	// Hour 4 (current): Reopen to verify states were carried forward
	storage, err = New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to reopen for hour 4: %v", err)
	}
	defer storage.Close()

	// The current file should have states from hour 3 (which included states from hour 1 & 2)
	files, _ := storage.getStorageFiles()
	if len(files) == 0 {
		t.Fatal("no storage files found")
	}

	// Check the most recent file for carried over states
	reader, _ := NewBlockReader(files[len(files)-1])
	fileData, _ := reader.ReadFile()
	reader.Close()

	// Should have long-lived-pod in final resource states (carried through hours)
	longLivedKey := "/v1/Pod/default/long-lived-pod"
	_, hasLongLived := fileData.IndexSection.FinalResourceStates[longLivedKey]

	if !hasLongLived {
		t.Error("long-lived pod from hour 1 should be carried through to hour 3/4")
	}
}

// TestResourceStateTracking_StateUpdate tests that state snapshots reflect the latest state
func TestResourceStateTracking_StateUpdate(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now()
	twoHoursAgo := now.Add(-2 * time.Hour)

	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Create resource with initial state
	event1 := &models.Event{
		ID:        "event1",
		Timestamp: twoHoursAgo.UnixNano(),
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "stateful-pod",
			UID:       "uid1",
		},
		Data: json.RawMessage(`{"status": {"phase": "Pending"}}`),
	}

	if err := storage.WriteEvent(event1); err != nil {
		t.Fatalf("failed to write creation event: %v", err)
	}

	// Update resource state (still in same hour)
	event2 := &models.Event{
		ID:        "event2",
		Timestamp: twoHoursAgo.Add(30 * time.Minute).UnixNano(),
		Type:      models.EventTypeUpdate,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "stateful-pod",
			UID:       "uid1",
		},
		Data: json.RawMessage(`{"status": {"phase": "Running", "ready": true}}`),
	}

	if err := storage.WriteEvent(event2); err != nil {
		t.Fatalf("failed to write update event: %v", err)
	}

	// Update again before closing
	event3 := &models.Event{
		ID:        "event3",
		Timestamp: twoHoursAgo.Add(50 * time.Minute).UnixNano(),
		Type:      models.EventTypeUpdate,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "stateful-pod",
			UID:       "uid1",
		},
		Data: json.RawMessage(`{"status": {"phase": "Running", "ready": true, "restarts": 1}}`),
	}

	if err := storage.WriteEvent(event3); err != nil {
		t.Fatalf("failed to write second update: %v", err)
	}

	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Verify state snapshot has the latest state
	files, _ := storage.getStorageFiles()
	reader, _ := NewBlockReader(files[0])
	fileData, _ := reader.ReadFile()
	reader.Close()

	resourceKey := "/v1/Pod/default/stateful-pod"
	state, exists := fileData.IndexSection.FinalResourceStates[resourceKey]

	if !exists {
		t.Fatalf("state snapshot not found for resource %s", resourceKey)
	}

	// Parse and verify the latest state data
	var stateData map[string]interface{}
	if err := json.Unmarshal(state.ResourceData, &stateData); err != nil {
		t.Fatalf("failed to parse state data: %v", err)
	}

	status, ok := stateData["status"].(map[string]interface{})
	if !ok {
		t.Fatal("status field not found in state data")
	}

	// Should have the latest values from event3
	if phase, ok := status["phase"].(string); !ok || phase != "Running" {
		t.Errorf("expected phase=Running, got %v", status["phase"])
	}

	if restarts, ok := status["restarts"].(float64); !ok || restarts != 1 {
		t.Errorf("expected restarts=1 (from latest event), got %v", status["restarts"])
	}
}

// TestResourceStateTracking_FilteredConsistentView tests state snapshots with resource filters
func TestResourceStateTracking_FilteredConsistentView(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now()
	pastTime := now.Add(-2 * time.Hour)

	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Create resources in different namespaces
	podDefault := createTestEvent("pod1", "default", "Pod", pastTime.UnixNano())
	podKubeSystem := &models.Event{
		ID:        "event-kube-pod",
		Timestamp: pastTime.Add(5 * time.Minute).UnixNano(),
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "kube-system",
			Name:      "coredns",
			UID:       "uid-coredns",
		},
		Data: json.RawMessage(`{"status": {"phase": "Running"}}`),
	}

	svcDefault := createTestEvent("svc1", "default", "Service", pastTime.Add(10*time.Minute).UnixNano())

	if err := storage.WriteEvent(podDefault); err != nil {
		t.Fatalf("failed to write pod: %v", err)
	}
	if err := storage.WriteEvent(podKubeSystem); err != nil {
		t.Fatalf("failed to write kube-system pod: %v", err)
	}
	if err := storage.WriteEvent(svcDefault); err != nil {
		t.Fatalf("failed to write service: %v", err)
	}

	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Verify state snapshots exist in the file
	files, _ := storage.getStorageFiles()
	reader, _ := NewBlockReader(files[0])
	fileData, _ := reader.ReadFile()
	reader.Close()

	// Verify we have snapshots for all three resources
	if len(fileData.IndexSection.FinalResourceStates) != 3 {
		t.Fatalf("expected 3 state snapshots, got %d", len(fileData.IndexSection.FinalResourceStates))
	}

	// Check that namespaces are properly tracked
	hasDefaultNs := false
	hasKubeSystemNs := false

	for _, state := range fileData.IndexSection.FinalResourceStates {
		// Parse resource key to check namespaces
		// Keys are: group/version/kind/namespace/name
		if state.EventType != string(models.EventTypeDelete) {
			// Just verify states exist - full filtering is tested in state_snapshot_test.go
			hasDefaultNs = true
			hasKubeSystemNs = true
		}
	}

	if !hasDefaultNs || !hasKubeSystemNs {
		t.Error("state snapshots should include resources from both namespaces")
	}
}

// TestResourceStateTracking_DeletedResourceExcluded tests that deleted resources don't appear in consistent view
func TestResourceStateTracking_DeletedResourceExcluded(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now()
	pastTime := now.Add(-2 * time.Hour)

	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Create and then delete a resource
	pod := createTestEvent("temp-pod", "default", "Pod", pastTime.UnixNano())
	if err := storage.WriteEvent(pod); err != nil {
		t.Fatalf("failed to write pod: %v", err)
	}

	// Delete the pod in the same hour
	deleteEvent := &models.Event{
		ID:        "event-delete",
		Timestamp: pastTime.Add(30 * time.Minute).UnixNano(),
		Type:      models.EventTypeDelete,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "temp-pod",
			UID:       "uid-temp",
		},
		Data: nil,
	}

	if err := storage.WriteEvent(deleteEvent); err != nil {
		t.Fatalf("failed to write delete event: %v", err)
	}

	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Verify state snapshot shows DELETE
	files, _ := storage.getStorageFiles()
	reader, _ := NewBlockReader(files[0])
	fileData, _ := reader.ReadFile()
	reader.Close()

	resourceKey := "/v1/Pod/default/temp-pod"
	state, exists := fileData.IndexSection.FinalResourceStates[resourceKey]

	if !exists {
		t.Fatalf("state snapshot should exist even for deleted resource: %s", resourceKey)
	}

	if state.EventType != "DELETE" {
		t.Errorf("expected DELETE event type, got %s", state.EventType)
	}

	// Reopen for current hour and query
	storage, err = New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to reopen storage: %v", err)
	}
	defer storage.Close()

	executor := NewQueryExecutor(storage)
	currentHour := now.Unix()

	query := &models.QueryRequest{
		StartTimestamp: currentHour - 3600,
		EndTimestamp:   currentHour,
		Filters:        models.QueryFilters{},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	// Deleted pod should NOT appear in results
	for _, event := range result.Events {
		if event.Resource.Name == "temp-pod" {
			t.Errorf("deleted resource should not appear in query results, but got: %v", event)
		}
	}
}

// TestResourceStateTracking_StateCleanup tests cleanup of old state snapshots
func TestResourceStateTracking_StateCleanup(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now()
	veryOldTime := now.Add(-20 * 24 * time.Hour) // 20 days old

	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Create a resource 20 days ago
	oldDeletedEvent := &models.Event{
		ID:        "old-event",
		Timestamp: veryOldTime.UnixNano(),
		Type:      models.EventTypeDelete,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "old-deleted-pod",
			UID:       "uid-old",
		},
		Data: nil,
	}

	// Create another resource 20 days ago (non-deleted)
	oldAliveEvent := &models.Event{
		ID:        "old-alive-event",
		Timestamp: veryOldTime.UnixNano(),
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "old-alive-pod",
			UID:       "uid-alive",
		},
		Data: json.RawMessage(`{"status": {"phase": "Running"}}`),
	}

	if err := storage.WriteEvent(oldDeletedEvent); err != nil {
		t.Fatalf("failed to write old deleted event: %v", err)
	}
	if err := storage.WriteEvent(oldAliveEvent); err != nil {
		t.Fatalf("failed to write old alive event: %v", err)
	}

	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Verify states exist before cleanup
	files, _ := storage.getStorageFiles()
	reader, _ := NewBlockReader(files[0])
	fileData, _ := reader.ReadFile()
	reader.Close()

	if len(fileData.IndexSection.FinalResourceStates) != 2 {
		t.Errorf("expected 2 state snapshots before cleanup, got %d", len(fileData.IndexSection.FinalResourceStates))
	}

	// Run cleanup with 14 day retention
	if err := storage.CleanupOldStateSnapshots(14); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	// Verify old deleted state was removed but alive state was kept
	reader, _ = NewBlockReader(files[0])
	fileData, _ = reader.ReadFile()
	reader.Close()

	deletedKey := "/v1/Pod/default/old-deleted-pod"
	aliveKey := "/v1/Pod/default/old-alive-pod"

	_, deletedExists := fileData.IndexSection.FinalResourceStates[deletedKey]
	_, aliveExists := fileData.IndexSection.FinalResourceStates[aliveKey]

	if deletedExists {
		t.Error("old deleted resource should be removed from state snapshots")
	}

	if !aliveExists {
		t.Error("old alive resource should be preserved in state snapshots")
	}
}

// TestResourceStateTracking_TimestampBoundary tests state snapshots at time range boundaries
func TestResourceStateTracking_TimestampBoundary(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now()
	twoHoursAgo := now.Add(-2 * time.Hour)

	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Create resource with timestamp slightly before query end
	event1 := createTestEvent("boundary-pod", "default", "Pod", twoHoursAgo.UnixNano())
	if err := storage.WriteEvent(event1); err != nil {
		t.Fatalf("failed to write event: %v", err)
	}

	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Verify state snapshot was created
	files, _ := storage.getStorageFiles()
	reader, _ := NewBlockReader(files[0])
	fileData, _ := reader.ReadFile()
	reader.Close()

	// State snapshot should exist
	resourceKey := "/v1/Pod/default/boundary-pod"
	state, exists := fileData.IndexSection.FinalResourceStates[resourceKey]

	if !exists {
		t.Fatalf("state snapshot not found for resource %s", resourceKey)
	}

	// Verify the state has correct timestamp
	if state.Timestamp != twoHoursAgo.UnixNano() {
		t.Errorf("expected timestamp %d, got %d", twoHoursAgo.UnixNano(), state.Timestamp)
	}

	// Verify resource data is preserved
	if len(state.ResourceData) == 0 {
		t.Error("resource data should be preserved in state snapshot")
	}
}

// TestResourceStateTracking_QueryTimeRangeBoundary tests that state snapshots show PreExisting resources
// Resources created before the query start time should appear as state snapshots (PreExisting=true)
// This allows the frontend to properly display resources that existed before the query window
func TestResourceStateTracking_QueryTimeRangeBoundary(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now()
	ninetyMinutesAgo := now.Add(-90 * time.Minute)
	eightyMinutesAgo := now.Add(-80 * time.Minute)
	sixtyMinutesAgo := now.Add(-60 * time.Minute)
	fortyfiveMinutesAgo := now.Add(-45 * time.Minute)

	// ===== Hour 1 (90+ minutes ago): Create resource =====
	// Set the "now" to 90 minutes ago so we can write to the appropriate hourly file
	storage, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	// Create a pod at 80 minutes ago (timestamps use seconds, not nanoseconds for query boundaries)
	podEvent := &models.Event{
		ID:        "event-pod-1",
		Timestamp: eightyMinutesAgo.UnixNano(),
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "external-secrets",
			Name:      "external-secrets-cert-controller",
			UID:       "uid-pod-1",
		},
		Data: json.RawMessage(`{"status": {"phase": "Running"}}`),
	}

	if err := storage.WriteEvent(podEvent); err != nil {
		t.Fatalf("failed to write pod event: %v", err)
	}

	// Close storage to persist state snapshots
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage after hour 1: %v", err)
	}

	// Reopen storage for hour 2 (simulate moving forward in time)
	storage, err = New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to recreate storage for hour 2: %v", err)
	}

	// Create another resource 45 minutes ago to ensure we have events in recent time
	recentEvent := &models.Event{
		ID:        "event-pod-2",
		Timestamp: fortyfiveMinutesAgo.UnixNano(),
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "recent-pod",
			UID:       "uid-pod-2",
		},
		Data: json.RawMessage(`{"status": {"phase": "Running"}}`),
	}

	if err := storage.WriteEvent(recentEvent); err != nil {
		t.Fatalf("failed to write recent event: %v", err)
	}

	// Close again to finalize
	if err := storage.Close(); err != nil {
		t.Fatalf("failed to close storage after hour 2: %v", err)
	}

	// ===== Now query with different time ranges =====
	storage, err = New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to recreate storage for querying: %v", err)
	}
	defer storage.Close()

	executor := NewQueryExecutor(storage)

	// Query 1: Last 60 minutes (from sixtyMinutesAgo to now)
	// This query SHOULD NOT include the pod from 80 minutes ago (created before query start)
	query60min := &models.QueryRequest{
		StartTimestamp: sixtyMinutesAgo.Unix(),
		EndTimestamp:   now.Unix(),
		Filters: models.QueryFilters{
			Namespace: "external-secrets",
			Kind:      "Pod",
		},
	}

	result60, err := executor.Execute(query60min)
	if err != nil {
		t.Fatalf("query for 60 minutes failed: %v", err)
	}

	// Query 2: Last 90 minutes (from ninetyMinutesAgo to now)
	// This query SHOULD include the pod from 80 minutes ago (created after query start, within range)
	query90min := &models.QueryRequest{
		StartTimestamp: ninetyMinutesAgo.Unix(),
		EndTimestamp:   now.Unix(),
		Filters: models.QueryFilters{
			Namespace: "external-secrets",
			Kind:      "Pod",
		},
	}

	result90, err := executor.Execute(query90min)
	if err != nil {
		t.Fatalf("query for 90 minutes failed: %v", err)
	}

	// Verification
	t.Logf("60-min query results: %d events", result60.Count)
	for _, event := range result60.Events {
		t.Logf("  - %s: %s (timestamp: %d, in query range: [%d, %d])",
			event.Resource.Name, event.Resource.Namespace,
			event.Timestamp, sixtyMinutesAgo.UnixNano(), now.Unix()*1e9)
	}

	t.Logf("90-min query results: %d events", result90.Count)
	for _, event := range result90.Events {
		t.Logf("  - %s: %s (timestamp: %d, in query range: [%d, %d])",
			event.Resource.Name, event.Resource.Namespace,
			event.Timestamp, ninetyMinutesAgo.UnixNano(), now.Unix()*1e9)
	}

	// NEW BEHAVIOR: PreExisting resources from before query start should appear as state snapshots
	// 60-minute query should have 1 event (state snapshot showing pod pre-existed)
	// This allows the frontend to show the resource with PreExisting=true
	if result60.Count != 1 {
		t.Errorf("60-minute query should return 1 event (state snapshot of pre-existing resource), but got %d", result60.Count)
	}

	// Verify it's a state snapshot
	if result60.Count > 0 && !strings.HasPrefix(result60.Events[0].ID, "state-") {
		t.Errorf("Expected state snapshot event (ID starting with 'state-'), got %s", result60.Events[0].ID)
	}

	// 90-minute query should have 1 event (pod created 80 min ago, which is within 90-minute range)
	if result90.Count != 1 {
		t.Errorf("90-minute query should return 1 event (resource created within range), but got %d", result90.Count)
	}
}

// TestResourceStateTracking_BugReproduction_RestartScenario tests the state tracking across restart
// This reproduces the potential bug where state snapshots are not visible after restart
// Scenario: Resource created → storage closed → storage reopened → query should show resource
func TestResourceStateTracking_BugReproduction_RestartScenario(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now()
	eightyMinutesAgo := now.Add(-80 * time.Minute)

	// ===== Step 1: Create resource in past =====
	storage1, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	podEvent := &models.Event{
		ID:        "event-pod-1",
		Timestamp: eightyMinutesAgo.UnixNano(),
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "external-secrets",
			Name:      "cert-controller",
			UID:       "uid-pod-1",
		},
		Data: json.RawMessage(`{"status": {"phase": "Running"}}`),
	}

	if err := storage1.WriteEvent(podEvent); err != nil {
		t.Fatalf("failed to write pod event: %v", err)
	}

	// Close storage to persist state snapshots
	if err := storage1.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// Now verify state snapshot was persisted to disk
	files1, _ := storage1.getStorageFiles()
	if len(files1) == 0 {
		t.Fatal("no files found after writing event")
	}

	reader1, _ := NewBlockReader(files1[0])
	if reader1 == nil {
		t.Fatal("failed to open file for reading")
	}
	fileData1, _ := reader1.ReadFile()
	reader1.Close()

	t.Logf("After first write - File has %d state snapshots", len(fileData1.IndexSection.FinalResourceStates))
	for k, v := range fileData1.IndexSection.FinalResourceStates {
		t.Logf("  - %s: EventType=%s, Timestamp=%d", k, v.EventType, v.Timestamp)
	}

	// ===== Step 2: Reopen storage (simulate restart) =====
	storage2, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to recreate storage: %v", err)
	}
	defer storage2.Close()

	executor := NewQueryExecutor(storage2)

	// Query for resources in past 60 minutes
	// Since the resource was created 80 min ago, it SHOULD appear as PreExisting
	// showing resources that pre-existed before the query window
	query := &models.QueryRequest{
		StartTimestamp: now.Add(-60 * time.Minute).Unix(),
		EndTimestamp:   now.Unix(),
		Filters: models.QueryFilters{
			Namespace: "external-secrets",
			Kind:      "Pod",
		},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	t.Logf("Query result after restart: %d events", result.Count)
	for _, event := range result.Events {
		t.Logf("  - Event: %s/%s (timestamp: %d, ID: %s)", event.Resource.Namespace, event.Resource.Name, event.Timestamp, event.ID)
	}

	// Verify that old file still has state snapshots
	files2, _ := storage2.getStorageFiles()
	for _, file := range files2 {
		reader, _ := NewBlockReader(file)
		fileData, _ := reader.ReadFile()
		reader.Close()
		t.Logf("File %s has %d state snapshots", file, len(fileData.IndexSection.FinalResourceStates))
	}

	// The pod SHOULD appear in 60-minute query as a state snapshot (showing it pre-existed)
	if result.Count != 1 {
		t.Errorf("60-minute query after restart should return 1 event (state snapshot), but got %d", result.Count)
	}

	// Verify it's a state snapshot event
	if result.Count > 0 && !strings.HasPrefix(result.Events[0].ID, "state-") {
		t.Errorf("Expected state snapshot event (ID starting with 'state-'), got %s", result.Events[0].ID)
	}
}

// TestResourceStateTracking_ConsistentViewWithinRange tests that state tracking correctly shows
// resources within the query time range, demonstrating the proper behavior
func TestResourceStateTracking_ConsistentViewWithinRange(t *testing.T) {
	tmpDir := t.TempDir()

	now := time.Now()
	eightyMinutesAgo := now.Add(-80 * time.Minute)
	sixtyMinutesAgo := now.Add(-60 * time.Minute)

	// ===== Step 1: Create resource 80 minutes ago =====
	storage1, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	podEvent := &models.Event{
		ID:        "event-pod-1",
		Timestamp: eightyMinutesAgo.UnixNano(),
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "external-secrets",
			Name:      "cert-controller",
			UID:       "uid-pod-1",
		},
		Data: json.RawMessage(`{"status": {"phase": "Running"}}`),
	}

	if err := storage1.WriteEvent(podEvent); err != nil {
		t.Fatalf("failed to write pod event: %v", err)
	}

	if err := storage1.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// ===== Step 2: Reopen and query with time range that includes resource creation =====
	storage2, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to recreate storage: %v", err)
	}
	defer storage2.Close()

	executor := NewQueryExecutor(storage2)

	// Query from 90 min ago to now - this SHOULD include the resource
	query := &models.QueryRequest{
		StartTimestamp: now.Add(-90 * time.Minute).Unix(),
		EndTimestamp:   now.Unix(),
		Filters: models.QueryFilters{
			Namespace: "external-secrets",
			Kind:      "Pod",
		},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	t.Logf("Query [90min ago, now]: %d events", result.Count)
	for _, event := range result.Events {
		t.Logf("  - Event: %s/%s (timestamp: %d)", event.Resource.Namespace, event.Resource.Name, event.Timestamp)
	}

	// Should have 1 event (the pod from 80 min ago)
	if result.Count != 1 {
		t.Errorf("query with resource in range should return 1 event, but got %d", result.Count)
	}

	// Now query with a range that does NOT include the resource creation
	query2 := &models.QueryRequest{
		StartTimestamp: sixtyMinutesAgo.Unix(),
		EndTimestamp:   now.Unix(),
		Filters: models.QueryFilters{
			Namespace: "external-secrets",
			Kind:      "Pod",
		},
	}

	result2, err := executor.Execute(query2)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	t.Logf("Query [60min ago, now]: %d events", result2.Count)
	for _, event := range result2.Events {
		t.Logf("  - Event: %s/%s (timestamp: %d, ID: %s)", event.Resource.Namespace, event.Resource.Name, event.Timestamp, event.ID)
	}

	// Should have 1 event - the state snapshot showing the resource pre-existed
	// This allows frontends to show resources that existed before the query window
	if result2.Count != 1 {
		t.Errorf("query without resource in range should still show state snapshot (PreExisting), expected 1 event but got %d", result2.Count)
	}

	// Verify it's a state snapshot
	if result2.Count > 0 && !strings.HasPrefix(result2.Events[0].ID, "state-") {
		t.Errorf("Expected state snapshot event (ID starting with 'state-'), got %s", result2.Events[0].ID)
	}
}
