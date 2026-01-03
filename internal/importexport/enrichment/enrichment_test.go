package enrichment

import (
	"encoding/json"
	"testing"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

func TestInvolvedObjectUIDEnricher(t *testing.T) {
	logger := logging.GetLogger("test")
	enricher := NewInvolvedObjectUIDEnricher()

	tests := []struct {
		name         string
		events       []models.Event
		expectedUIDs []string
		description  string
	}{
		{
			name: "populates InvolvedObjectUID for Event resources",
			events: []models.Event{
				{
					ID:        "event1",
					Timestamp: 1234567890000000000,
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Group:     "",
						Version:   "v1",
						Kind:      "Event",
						Namespace: "default",
						Name:      "test-event",
						UID:       "event-uid-1",
					},
					Data: json.RawMessage(`{
						"involvedObject": {
							"uid": "pod-uid-123"
						}
					}`),
				},
			},
			expectedUIDs: []string{"pod-uid-123"},
			description:  "Should extract involvedObject.uid from Event resource data",
		},
		{
			name: "does not affect non-Event resources",
			events: []models.Event{
				{
					ID:        "event1",
					Timestamp: 1234567890000000000,
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Group:     "apps",
						Version:   "v1",
						Kind:      "Deployment",
						Namespace: "default",
						Name:      "test-deployment",
						UID:       "deployment-uid-1",
					},
					Data: json.RawMessage(`{
						"involvedObject": {
							"uid": "should-not-be-used"
						}
					}`),
				},
			},
			expectedUIDs: []string{""},
			description:  "Should not populate InvolvedObjectUID for non-Event resources",
		},
		{
			name: "skips events with empty data",
			events: []models.Event{
				{
					ID:        "event1",
					Timestamp: 1234567890000000000,
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Group:     "",
						Version:   "v1",
						Kind:      "Event",
						Namespace: "default",
						Name:      "test-event",
						UID:       "event-uid-1",
					},
					Data: json.RawMessage(``),
				},
			},
			expectedUIDs: []string{""},
			description:  "Should skip events with empty data",
		},
		{
			name: "does not overwrite existing InvolvedObjectUID",
			events: []models.Event{
				{
					ID:        "event1",
					Timestamp: 1234567890000000000,
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Group:             "",
						Version:           "v1",
						Kind:              "Event",
						Namespace:         "default",
						Name:              "test-event",
						UID:               "event-uid-1",
						InvolvedObjectUID: "existing-uid-456",
					},
					Data: json.RawMessage(`{
						"involvedObject": {
							"uid": "new-uid-789"
						}
					}`),
				},
			},
			expectedUIDs: []string{"existing-uid-456"},
			description:  "Should not overwrite existing InvolvedObjectUID",
		},
		{
			name: "case-insensitive kind matching",
			events: []models.Event{
				{
					ID:        "event1",
					Timestamp: 1234567890000000000,
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Group:     "",
						Version:   "v1",
						Kind:      "event", // lowercase
						Namespace: "default",
						Name:      "test-event",
						UID:       "event-uid-1",
					},
					Data: json.RawMessage(`{
						"involvedObject": {
							"uid": "pod-uid-123"
						}
					}`),
				},
			},
			expectedUIDs: []string{"pod-uid-123"},
			description:  "Should match Event kind case-insensitively",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid modifying test data
			eventsCopy := make([]models.Event, len(tt.events))
			copy(eventsCopy, tt.events)

			// Apply enrichment
			enricher.Enrich(eventsCopy, logger)

			// Verify results
			for i, event := range eventsCopy {
				expectedUID := tt.expectedUIDs[i]
				if event.Resource.InvolvedObjectUID != expectedUID {
					t.Errorf("Event %d: Expected InvolvedObjectUID %q, got %q",
						i, expectedUID, event.Resource.InvolvedObjectUID)
				}
			}
		})
	}
}

func TestEnrichmentChain(t *testing.T) {
	logger := logging.GetLogger("test")

	// Create a chain with the default enricher
	chain := NewChain(NewInvolvedObjectUIDEnricher())

	events := []models.Event{
		{
			ID:        "event1",
			Timestamp: 1234567890000000000,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Group:     "",
				Version:   "v1",
				Kind:      "Event",
				Namespace: "default",
				Name:      "test-event",
				UID:       "event-uid-1",
			},
			Data: json.RawMessage(`{
				"involvedObject": {
					"uid": "pod-uid-123"
				}
			}`),
		},
	}

	chain.Enrich(events, logger)

	if events[0].Resource.InvolvedObjectUID != "pod-uid-123" {
		t.Errorf("Expected InvolvedObjectUID 'pod-uid-123', got %q",
			events[0].Resource.InvolvedObjectUID)
	}
}

func TestDefaultChain(t *testing.T) {
	logger := logging.GetLogger("test")

	// Use the default chain
	chain := Default()

	events := []models.Event{
		{
			ID:        "event1",
			Timestamp: 1234567890000000000,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Group:     "",
				Version:   "v1",
				Kind:      "Event",
				Namespace: "default",
				Name:      "test-event",
				UID:       "event-uid-1",
			},
			Data: json.RawMessage(`{
				"involvedObject": {
					"uid": "pod-uid-456"
				}
			}`),
		},
	}

	chain.Enrich(events, logger)

	if events[0].Resource.InvolvedObjectUID != "pod-uid-456" {
		t.Errorf("Expected InvolvedObjectUID 'pod-uid-456', got %q",
			events[0].Resource.InvolvedObjectUID)
	}
}
