package storage

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
)

// TestIntegration_StateSnapshotPreExistingFlag tests the complete flow:
// 1. Create resource, close file (state snapshot persisted)
// 2. Query across hour boundary
// 3. Verify state snapshot is included
// 4. Verify PreExisting=true in results
func TestIntegration_StateSnapshotPreExistingFlag(t *testing.T) {
	tmpDir := t.TempDir()
	now := time.Now()
	eightyMinutesAgo := now.Add(-80 * time.Minute)

	// === STEP 1: Create resource 80 minutes ago, close file to persist state snapshot ===
	storage1, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	podEvent := &models.Event{
		ID:        "event-pod-create",
		Timestamp: eightyMinutesAgo.UnixNano(),
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			UID:       "uid-cert-controller",
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "external-secrets",
			Name:      "external-secrets-cert-controller",
		},
		Data: json.RawMessage(`{"status":"Running"}`),
	}

	if err := storage1.WriteEvent(podEvent); err != nil {
		t.Fatalf("failed to write event: %v", err)
	}

	if err := storage1.Close(); err != nil {
		t.Fatalf("failed to close storage: %v", err)
	}

	// === STEP 2: Reopen storage and query with 30-minute range (should include state snapshot) ===
	storage2, err := New(tmpDir, 1024*1024)
	if err != nil {
		t.Fatalf("failed to reopen storage: %v", err)
	}
	defer storage2.Close()

	executor := NewQueryExecutor(storage2)

	// Query for last 30 minutes (but resource is 80 min old, so will come from state snapshot)
	queryStart := now.Add(-30 * time.Minute).Unix()
	queryEnd := now.Unix()

	query := &models.QueryRequest{
		StartTimestamp: queryStart,
		EndTimestamp:   queryEnd,
		Filters: models.QueryFilters{
			Namespace: "external-secrets",
			Kind:      "Pod",
		},
	}

	result, err := executor.Execute(query)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	t.Logf("Query returned %d events", result.Count)
	for _, event := range result.Events {
		t.Logf("  Event: %s (ID: %s)", event.Resource.Name, event.ID)
		if strings.HasPrefix(event.ID, "state-") {
			t.Logf("    ✓ State snapshot event detected")
		}
	}

	if result.Count == 0 {
		t.Logf("FAIL: Query returned 0 events. Expected state snapshot to be created.")
		t.FailNow()
	}

	// === STEP 3: Build resources from query result (as timeline handler does) ===
	builder := NewResourceBuilder()
	resources := builder.BuildResourcesFromEvents(result.Events)

	t.Logf("\nResources built: %d", len(resources))
	if len(resources) == 0 {
		t.Fatal("No resources built from query result")
	}

	// === STEP 4: Verify PreExisting=true ===
	for uid, res := range resources {
		t.Logf("\nResource: %s/%s", res.Namespace, res.Name)
		t.Logf("  UID: %s", uid)
		t.Logf("  PreExisting: %v", res.PreExisting)
		t.Logf("  Status Segments: %d", len(res.StatusSegments))

		// Check if first event was a state snapshot
		hasStateSnapshot := false
		for _, event := range result.Events {
			if event.Resource.UID == uid {
				t.Logf("  First matching event ID: %s", event.ID)
				if len(event.ID) > 6 && event.ID[:6] == "state-" {
					hasStateSnapshot = true
				}
				break
			}
		}

		if hasStateSnapshot && !res.PreExisting {
			t.Errorf("Resource with state snapshot should have PreExisting=true, got false")
		}

		if hasStateSnapshot && res.PreExisting {
			t.Logf("✓ PreExisting correctly set to true for state snapshot resource")
		}
	}
}
