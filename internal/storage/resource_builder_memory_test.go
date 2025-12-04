package storage

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
)

// TestResourceBuilder_PreExistingWithMixedSources tests PreExisting flag with events from both disk and memory
// This simulates the real query flow where events come from both queryFile() and GetInMemoryEvents()
func TestResourceBuilder_PreExistingWithMixedSources(t *testing.T) {
	now := time.Now()

	t.Run("resource with disk state snapshot and memory update", func(t *testing.T) {
		// Scenario: Resource existed 80 min ago (state snapshot in disk)
		// and was just updated 5 min ago (update event in memory buffer)
		events := []models.Event{
			// From disk (state snapshot - older)
			{
				ID:        "state-res1-80min-ago",
				Timestamp: now.Add(-80 * time.Minute).UnixNano(),
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					UID:  "uid-res1",
					Kind: "Pod",
					Name: "long-lived-pod",
				},
				Data: json.RawMessage(`{"status":"Running"}`),
			},
			// From memory buffer (actual update - newer)
			{
				ID:        "event-update-5min-ago",
				Timestamp: now.Add(-5 * time.Minute).UnixNano(),
				Type:      models.EventTypeUpdate,
				Resource: models.ResourceMetadata{
					UID:  "uid-res1",
					Kind: "Pod",
					Name: "long-lived-pod",
				},
				Data: json.RawMessage(`{"status":"Running","ready":true}`),
			},
		}

		builder := NewResourceBuilder()
		resources := builder.BuildResourcesFromEvents(events)

		if len(resources) != 1 {
			t.Fatalf("expected 1 resource, got %d", len(resources))
		}

		resource := resources["uid-res1"]
		if !resource.PreExisting {
			t.Errorf("expected PreExisting=true for resource with state snapshot as first event, got false")
		}
	})

	t.Run("resource created recently, events only in memory", func(t *testing.T) {
		// Scenario: Resource created 30 min ago, all events still in memory buffer
		events := []models.Event{
			// From memory buffer (actual CREATE - first event)
			{
				ID:        "event-create-30min-ago",
				Timestamp: now.Add(-30 * time.Minute).UnixNano(),
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					UID:  "uid-res2",
					Kind: "Pod",
					Name: "new-pod",
				},
				Data: json.RawMessage(`{"status":"Pending"}`),
			},
			// From memory buffer (actual UPDATE - newer)
			{
				ID:        "event-update-5min-ago",
				Timestamp: now.Add(-5 * time.Minute).UnixNano(),
				Type:      models.EventTypeUpdate,
				Resource: models.ResourceMetadata{
					UID:  "uid-res2",
					Kind: "Pod",
					Name: "new-pod",
				},
				Data: json.RawMessage(`{"status":"Running"}`),
			},
		}

		builder := NewResourceBuilder()
		resources := builder.BuildResourcesFromEvents(events)

		if len(resources) != 1 {
			t.Fatalf("expected 1 resource, got %d", len(resources))
		}

		resource := resources["uid-res2"]
		if resource.PreExisting {
			t.Errorf("expected PreExisting=false for resource with actual CREATE as first event, got true")
		}
	})

	t.Run("multiple resources with mixed disk and memory events", func(t *testing.T) {
		// Scenario: Two resources, one pre-existing with memory updates, one recently created
		events := []models.Event{
			// Resource 1: Pre-existing
			{
				ID:        "state-res1-key",
				Timestamp: now.Add(-2 * time.Hour).UnixNano(),
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					UID:       "uid-res1",
					Kind:      "Pod",
					Namespace: "production",
					Name:      "prod-app",
				},
				Data: json.RawMessage(`{}`),
			},
			{
				ID:        "event-update-res1",
				Timestamp: now.Add(-10 * time.Minute).UnixNano(),
				Type:      models.EventTypeUpdate,
				Resource: models.ResourceMetadata{
					UID:       "uid-res1",
					Kind:      "Pod",
					Namespace: "production",
					Name:      "prod-app",
				},
				Data: json.RawMessage(`{}`),
			},

			// Resource 2: Recently created
			{
				ID:        "event-create-res2",
				Timestamp: now.Add(-20 * time.Minute).UnixNano(),
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					UID:       "uid-res2",
					Kind:      "Pod",
					Namespace: "staging",
					Name:      "staging-app",
				},
				Data: json.RawMessage(`{}`),
			},
		}

		builder := NewResourceBuilder()
		resources := builder.BuildResourcesFromEvents(events)

		if len(resources) != 2 {
			t.Fatalf("expected 2 resources, got %d", len(resources))
		}

		// Resource 1 should be pre-existing
		res1 := resources["uid-res1"]
		if !res1.PreExisting {
			t.Errorf("res1: expected PreExisting=true, got false")
		}

		// Resource 2 should NOT be pre-existing
		res2 := resources["uid-res2"]
		if res2.PreExisting {
			t.Errorf("res2: expected PreExisting=false, got true")
		}
	})

	t.Run("only state snapshot, no memory events", func(t *testing.T) {
		// Scenario: Old resource with state snapshot, no recent updates in memory
		events := []models.Event{
			{
				ID:        "state-old-pod-80min",
				Timestamp: now.Add(-80 * time.Minute).UnixNano(),
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					UID:  "uid-old",
					Kind: "Pod",
					Name: "old-pod",
				},
				Data: json.RawMessage(`{}`),
			},
		}

		builder := NewResourceBuilder()
		resources := builder.BuildResourcesFromEvents(events)

		if len(resources) != 1 {
			t.Fatalf("expected 1 resource, got %d", len(resources))
		}

		resource := resources["uid-old"]
		if !resource.PreExisting {
			t.Errorf("expected PreExisting=true for state-snapshot-only resource, got false")
		}
	})

	t.Run("event ID with state- in middle, from memory", func(t *testing.T) {
		// Edge case: Event ID happens to contain "state-" but not at the start
		events := []models.Event{
			{
				ID:        "deployment-state-123",
				Timestamp: now.Add(-30 * time.Minute).UnixNano(),
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					UID:  "uid-edge",
					Kind: "Pod",
					Name: "edge-case-pod",
				},
				Data: json.RawMessage(`{}`),
			},
		}

		builder := NewResourceBuilder()
		resources := builder.BuildResourcesFromEvents(events)

		resource := resources["uid-edge"]
		if resource.PreExisting {
			t.Errorf("expected PreExisting=false for event with 'state-' in middle of ID, got true")
		}
	})
}

// TestResourceBuilder_PreExistingEventOrdering tests that PreExisting is based on chronological ordering
// not the order events arrive in the slice
func TestResourceBuilder_PreExistingEventOrdering(t *testing.T) {
	now := time.Now()

	t.Run("events provided in reverse chronological order", func(t *testing.T) {
		// Events are provided newest first, but IsPreExisting should sort by timestamp
		events := []models.Event{
			// Provided first (newest)
			{
				ID:        "event-update-5min",
				Timestamp: now.Add(-5 * time.Minute).UnixNano(),
				Type:      models.EventTypeUpdate,
				Resource: models.ResourceMetadata{
					UID: "uid-test",
				},
				Data: json.RawMessage(`{}`),
			},
			// Provided second (oldest - state snapshot)
			{
				ID:        "state-80min-key",
				Timestamp: now.Add(-80 * time.Minute).UnixNano(),
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					UID: "uid-test",
				},
				Data: json.RawMessage(`{}`),
			},
		}

		builder := NewResourceBuilder()
		resources := builder.BuildResourcesFromEvents(events)

		resource := resources["uid-test"]
		if !resource.PreExisting {
			t.Errorf("expected PreExisting=true even with reverse chronological order, got false")
		}
	})

	t.Run("events provided in mixed order", func(t *testing.T) {
		// Random order in the input slice
		events := []models.Event{
			{
				ID:        "event-update-10min",
				Timestamp: now.Add(-10 * time.Minute).UnixNano(),
				Type:      models.EventTypeUpdate,
				Resource: models.ResourceMetadata{
					UID: "uid-mixed",
				},
				Data: json.RawMessage(`{}`),
			},
			{
				ID:        "state-90min-key",
				Timestamp: now.Add(-90 * time.Minute).UnixNano(),
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					UID: "uid-mixed",
				},
				Data: json.RawMessage(`{}`),
			},
			{
				ID:        "event-update-5min",
				Timestamp: now.Add(-5 * time.Minute).UnixNano(),
				Type:      models.EventTypeUpdate,
				Resource: models.ResourceMetadata{
					UID: "uid-mixed",
				},
				Data: json.RawMessage(`{}`),
			},
		}

		builder := NewResourceBuilder()
		resources := builder.BuildResourcesFromEvents(events)

		resource := resources["uid-mixed"]
		if !resource.PreExisting {
			t.Errorf("expected PreExisting=true for mixed order events with state snapshot oldest, got false")
		}
	})
}
