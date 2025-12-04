package storage

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
)

// TestResourceBuilder_PreExistingWithStateSnapshot verifies PreExisting flag is set for state snapshots
func TestResourceBuilder_PreExistingWithStateSnapshot(t *testing.T) {
	now := time.Now()
	eightyMinutesAgo := now.Add(-80 * time.Minute)

	// Create a state snapshot event as the query executor would
	resourceKey := "/v1/Pod/external-secrets/external-secrets-cert-controller"
	stateSnapshotEvent := models.Event{
		ID:        fmt.Sprintf("state-%s-%d", resourceKey, eightyMinutesAgo.UnixNano()),
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

	t.Logf("State snapshot event ID: %s", stateSnapshotEvent.ID)
	t.Logf("ID starts with 'state-': %v", strings.HasPrefix(stateSnapshotEvent.ID, "state-"))

	// Test IsPreExisting method directly
	builder := NewResourceBuilder()
	isPreExisting := builder.IsPreExisting("uid-cert-controller", []models.Event{stateSnapshotEvent})
	t.Logf("IsPreExisting() returned: %v", isPreExisting)

	if !isPreExisting {
		t.Errorf("IsPreExisting() should return true for state snapshot, got false")
	}

	// Test BuildResourcesFromEvents
	resources := builder.BuildResourcesFromEvents([]models.Event{stateSnapshotEvent})

	t.Logf("Resources built: %d", len(resources))
	if len(resources) == 0 {
		t.Fatal("No resources were built from state snapshot event")
	}

	resource := resources["uid-cert-controller"]
	if resource == nil {
		t.Fatal("Resource with UID 'uid-cert-controller' not found")
	}

	t.Logf("Resource.PreExisting: %v", resource.PreExisting)
	t.Logf("Resource.Name: %s", resource.Name)
	t.Logf("Resource.Namespace: %s", resource.Namespace)

	if !resource.PreExisting {
		t.Errorf("Resource should have PreExisting=true for state snapshot, got false")
		t.Logf("Status segments: %d", len(resource.StatusSegments))
		if len(resource.StatusSegments) > 0 {
			t.Logf("First status segment timestamp: %d", resource.StatusSegments[0].StartTime)
		}
	}
}

// TestResourceBuilder_PreExistingEventIDFormat tests exact ID format matching
func TestResourceBuilder_PreExistingEventIDFormat(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		eventID       string
		shouldPreExist bool
	}{
		{
			name:          "Standard state snapshot ID from query executor",
			eventID:       fmt.Sprintf("state-/v1/Pod/default/my-pod-%d", now.UnixNano()),
			shouldPreExist: true,
		},
		{
			name:          "State snapshot with namespace in path",
			eventID:       fmt.Sprintf("state-/v1/Pod/external-secrets/cert-controller-%d", now.UnixNano()),
			shouldPreExist: true,
		},
		{
			name:          "Actual event (not state snapshot)",
			eventID:       "event-123",
			shouldPreExist: false,
		},
	}

	builder := NewResourceBuilder()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event := models.Event{
				ID:        test.eventID,
				Timestamp: now.UnixNano(),
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					UID: "uid-test",
				},
			}

			result := builder.IsPreExisting("uid-test", []models.Event{event})

			t.Logf("Event ID: %s", test.eventID)
			t.Logf("Expected PreExisting: %v", test.shouldPreExist)
			t.Logf("Got: %v", result)

			if result != test.shouldPreExist {
				t.Errorf("IsPreExisting() returned %v, expected %v", result, test.shouldPreExist)
			}
		})
	}
}

// TestResourceBuilder_StateSnapshotEventFlow simulates complete flow from query to resource
func TestResourceBuilder_StateSnapshotEventFlow(t *testing.T) {
	now := time.Now()
	eightyMinutesAgo := now.Add(-80 * time.Minute)

	// Simulate what query executor returns
	queryResult := &models.QueryResult{
		Events: []models.Event{
			{
				ID:        fmt.Sprintf("state-/v1/Pod/external-secrets/cert-controller-%d", eightyMinutesAgo.UnixNano()),
				Timestamp: eightyMinutesAgo.UnixNano(),
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					UID:       "uid-1",
					Group:     "",
					Version:   "v1",
					Kind:      "Pod",
					Namespace: "external-secrets",
					Name:      "external-secrets-cert-controller",
				},
				Data: json.RawMessage(`{"status":"Running"}`),
			},
		},
		Count: 1,
	}

	// Simulate what timeline handler does
	builder := NewResourceBuilder()
	resources := builder.BuildResourcesFromEvents(queryResult.Events)

	t.Logf("Query returned %d events", len(queryResult.Events))
	t.Logf("Builder created %d resources", len(resources))

	if len(resources) == 0 {
		t.Fatal("Expected resources to be created from query result")
	}

	for uid, res := range resources {
		t.Logf("\nResource UID: %s", uid)
		t.Logf("  Name: %s/%s", res.Namespace, res.Name)
		t.Logf("  PreExisting: %v", res.PreExisting)
		t.Logf("  Status Segments: %d", len(res.StatusSegments))

		if !res.PreExisting {
			t.Errorf("Resource should have PreExisting=true (created from state snapshot)")

			// Debug: check what the resource builder sees
			builder2 := NewResourceBuilder()
			result := builder2.IsPreExisting(uid, queryResult.Events)
			t.Logf("  DEBUG IsPreExisting returned: %v", result)

			// Check if the issue is in how resources are created
			t.Logf("  DEBUG First event ID: %s", queryResult.Events[0].ID)
			t.Logf("  DEBUG Event starts with 'state-': %v", strings.HasPrefix(queryResult.Events[0].ID, "state-"))
		}
	}
}
