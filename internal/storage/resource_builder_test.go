package storage

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
)

// TestResourceBuilder_PreExistingFlag tests the PreExisting flag detection
func TestResourceBuilder_PreExistingFlag(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		events      []models.Event
		expected    bool
		description string
	}{
		{
			name: "PreExisting resource (state snapshot first)",
			events: []models.Event{
				{
					ID:        "state-example-key-" + string(rune(now.UnixNano())),
					Timestamp: now.Add(-2 * time.Hour).UnixNano(),
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						UID:  "uid-1",
						Kind: "Pod",
					},
					Data: json.RawMessage(`{"status":"Running"}`),
				},
				{
					ID:        "event-actual-1",
					Timestamp: now.Add(-1 * time.Hour).UnixNano(),
					Type:      models.EventTypeUpdate,
					Resource: models.ResourceMetadata{
						UID:  "uid-1",
						Kind: "Pod",
					},
					Data: json.RawMessage(`{"status":"Running"}`),
				},
			},
			expected:    true,
			description: "Resource with state snapshot as first event should be marked PreExisting",
		},
		{
			name: "Non-preexisting resource (actual event first)",
			events: []models.Event{
				{
					ID:        "event-actual-1",
					Timestamp: now.UnixNano(),
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						UID:  "uid-2",
						Kind: "Pod",
					},
					Data: json.RawMessage(`{"status":"Running"}`),
				},
				{
					ID:        "event-actual-2",
					Timestamp: now.Add(5 * time.Minute).UnixNano(),
					Type:      models.EventTypeUpdate,
					Resource: models.ResourceMetadata{
						UID:  "uid-2",
						Kind: "Pod",
					},
					Data: json.RawMessage(`{"status":"Running"}`),
				},
			},
			expected:    false,
			description: "Resource with actual CREATE event first should NOT be marked PreExisting",
		},
		{
			name: "Single state snapshot event",
			events: []models.Event{
				{
					ID:        "state-single-" + string(rune(now.UnixNano())),
					Timestamp: now.Add(-3 * time.Hour).UnixNano(),
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						UID:  "uid-3",
						Kind: "Pod",
					},
					Data: json.RawMessage(`{"status":"Running"}`),
				},
			},
			expected:    true,
			description: "Resource with only a state snapshot should be marked PreExisting",
		},
		{
			name: "Event with 'state-' in middle of ID",
			events: []models.Event{
				{
					ID:        "my-state-event-1",
					Timestamp: now.UnixNano(),
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						UID:  "uid-4",
						Kind: "Pod",
					},
					Data: json.RawMessage(`{"status":"Running"}`),
				},
			},
			expected:    false,
			description: "Event ID with 'state-' not at start should NOT be marked PreExisting",
		},
		{
			name:        "No events",
			events:      []models.Event{},
			expected:    false,
			description: "Resource with no events should NOT be marked PreExisting",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			builder := NewResourceBuilder()
			resources := builder.BuildResourcesFromEvents(test.events)

			// Verify PreExisting flag
			var actual bool
			if len(resources) > 0 {
				// Get the first (and typically only) resource
				for _, resource := range resources {
					actual = resource.PreExisting
					break
				}
			}

			if actual != test.expected {
				t.Errorf("%s: expected PreExisting=%v, got %v", test.description, test.expected, actual)
			}
		})
	}
}

// TestResourceBuilder_MultipleResources tests PreExisting flag with multiple resources
func TestResourceBuilder_MultipleResources(t *testing.T) {
	now := time.Now()

	// Two resources: one pre-existing, one not
	events := []models.Event{
		// Resource 1: Pre-existing (state snapshot first)
		{
			ID:        "state-res1-key",
			Timestamp: now.Add(-2 * time.Hour).UnixNano(),
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				UID:  "uid-res1",
				Kind: "Pod",
				Name: "pre-existing-pod",
			},
			Data: json.RawMessage(`{"status":"Running"}`),
		},
		// Resource 2: Not pre-existing (actual event first)
		{
			ID:        "event-res2-1",
			Timestamp: now.UnixNano(),
			Type:      models.EventTypeCreate,
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

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify resource 1 (pre-existing)
	res1 := resources["uid-res1"]
	if res1 == nil {
		t.Fatal("resource 1 not found")
	}
	if !res1.PreExisting {
		t.Errorf("resource 1 (pre-existing pod) should have PreExisting=true, got false")
	}

	// Verify resource 2 (not pre-existing)
	res2 := resources["uid-res2"]
	if res2 == nil {
		t.Fatal("resource 2 not found")
	}
	if res2.PreExisting {
		t.Errorf("resource 2 (new pod) should have PreExisting=false, got true")
	}
}

// TestResourceBuilder_StateSnapshotIdPrefix tests that state snapshot detection uses correct prefix
func TestResourceBuilder_StateSnapshotIdPrefix(t *testing.T) {
	now := time.Now()

	tests := []struct {
		eventID   string
		isPreEx   bool
		reason    string
	}{
		{"state-something", true, "Standard state- prefix"},
		{"state-", true, "Just state- prefix"},
		{"state-with-multiple-state-markers", true, "Multiple state mentions"},
		{"State-uppercase", false, "Uppercase State- should not match"},
		{"notstate-prefix", false, "state not at start"},
		{"my-state-thing", false, "state- in middle"},
		{"eventstate-1", false, "event prefix before state"},
		{"normal-event", false, "Normal event"},
	}

	for _, test := range tests {
		t.Run(test.eventID, func(t *testing.T) {
			event := models.Event{
				ID:        test.eventID,
				Timestamp: now.UnixNano(),
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					UID:  "test-uid",
					Kind: "Pod",
				},
				Data: json.RawMessage(`{}`),
			}

			builder := NewResourceBuilder()
			resources := builder.BuildResourcesFromEvents([]models.Event{event})

			var actual bool
			if len(resources) > 0 {
				for _, r := range resources {
					actual = r.PreExisting
					break
				}
			}

			if actual != test.isPreEx {
				t.Errorf("%s: expected PreExisting=%v for ID %q, got %v. Reason: %s",
					test.eventID, test.isPreEx, test.eventID, actual, test.reason)
			}
		})
	}
}

// TestResourceBuilder_IsPreExisting tests the IsPreExisting method directly
func TestResourceBuilder_IsPreExisting(t *testing.T) {
	now := time.Now()
	builder := NewResourceBuilder()

	t.Run("first event is state snapshot", func(t *testing.T) {
		events := []models.Event{
			{
				ID:        "state-key-123",
				Timestamp: now.Add(-1 * time.Hour).UnixNano(),
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					UID: "uid-1",
				},
			},
			{
				ID:        "event-actual",
				Timestamp: now.UnixNano(),
				Type:      models.EventTypeUpdate,
				Resource: models.ResourceMetadata{
					UID: "uid-1",
				},
			},
		}

		result := builder.IsPreExisting("uid-1", events)
		if !result {
			t.Errorf("expected true when first event is state snapshot, got false")
		}
	})

	t.Run("first event is actual event", func(t *testing.T) {
		events := []models.Event{
			{
				ID:        "event-actual",
				Timestamp: now.UnixNano(),
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					UID: "uid-2",
				},
			},
			{
				ID:        "state-key-123",
				Timestamp: now.Add(1 * time.Hour).UnixNano(),
				Type:      models.EventTypeUpdate,
				Resource: models.ResourceMetadata{
					UID: "uid-2",
				},
			},
		}

		result := builder.IsPreExisting("uid-2", events)
		if result {
			t.Errorf("expected false when first event is actual event, got true")
		}
	})

	t.Run("resource has no events", func(t *testing.T) {
		events := []models.Event{
			{
				ID:        "event-other",
				Timestamp: now.UnixNano(),
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					UID: "uid-other",
				},
			},
		}

		result := builder.IsPreExisting("uid-nonexistent", events)
		if result {
			t.Errorf("expected false for nonexistent resource, got true")
		}
	})
}

// TestResourceBuilder_PreExistingJsonMarshalling tests that PreExisting serializes correctly to JSON
func TestResourceBuilder_PreExistingJsonMarshalling(t *testing.T) {
	now := time.Now()

	event := models.Event{
		ID:        "state-test-key",
		Timestamp: now.UnixNano(),
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			UID:       "uid-test",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "test-pod",
			Group:     "",
			Version:   "v1",
		},
		Data: json.RawMessage(`{"status":"Running"}`),
	}

	builder := NewResourceBuilder()
	resources := builder.BuildResourcesFromEvents([]models.Event{event})

	if len(resources) == 0 {
		t.Fatal("no resources created")
	}

	var resource *models.Resource
	for _, r := range resources {
		resource = r
		break
	}

	// Marshal to JSON
	data, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("failed to marshal resource to JSON: %v", err)
	}

	// Verify PreExisting field is in JSON
	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"preExisting":true`) {
		t.Errorf("JSON should contain 'preExisting':true, got: %s", jsonStr)
	}

	// Unmarshal and verify
	var unmarshaled models.Resource
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if !unmarshaled.PreExisting {
		t.Errorf("unmarshaled PreExisting should be true, got false")
	}
}
