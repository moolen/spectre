package importexport

import (
	"os"
	"strings"
	"testing"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

func TestValidateEvents(t *testing.T) {
	logger := logging.GetLogger("test")

	tests := []struct {
		name              string
		events            []models.Event
		expectedValid     int
		expectedInvalid   int
		description       string
	}{
		{
			name: "all valid events",
			events: []models.Event{
				{
					ID:        "event1",
					Timestamp: 1234567890000000000,
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Kind: "Pod",
						Name: "test-pod",
					},
				},
				{
					ID:        "event2",
					Timestamp: 1234567891000000000,
					Type:      models.EventTypeUpdate,
					Resource: models.ResourceMetadata{
						Kind: "Deployment",
						Name: "test-deployment",
					},
				},
			},
			expectedValid:   2,
			expectedInvalid: 0,
			description:     "All events should pass validation",
		},
		{
			name: "event with empty ID",
			events: []models.Event{
				{
					ID:        "",
					Timestamp: 1234567890000000000,
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Kind: "Pod",
						Name: "test-pod",
					},
				},
			},
			expectedValid:   0,
			expectedInvalid: 1,
			description:     "Event with empty ID should be invalid",
		},
		{
			name: "event with zero timestamp",
			events: []models.Event{
				{
					ID:        "event1",
					Timestamp: 0,
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Kind: "Pod",
						Name: "test-pod",
					},
				},
			},
			expectedValid:   0,
			expectedInvalid: 1,
			description:     "Event with zero timestamp should be invalid",
		},
		{
			name: "event with negative timestamp",
			events: []models.Event{
				{
					ID:        "event1",
					Timestamp: -1,
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Kind: "Pod",
						Name: "test-pod",
					},
				},
			},
			expectedValid:   0,
			expectedInvalid: 1,
			description:     "Event with negative timestamp should be invalid",
		},
		{
			name: "event with empty type",
			events: []models.Event{
				{
					ID:        "event1",
					Timestamp: 1234567890000000000,
					Type:      "",
					Resource: models.ResourceMetadata{
						Kind: "Pod",
						Name: "test-pod",
					},
				},
			},
			expectedValid:   0,
			expectedInvalid: 1,
			description:     "Event with empty type should be invalid",
		},
		{
			name: "event with empty resource kind",
			events: []models.Event{
				{
					ID:        "event1",
					Timestamp: 1234567890000000000,
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Kind: "",
						Name: "test-pod",
					},
				},
			},
			expectedValid:   0,
			expectedInvalid: 1,
			description:     "Event with empty resource kind should be invalid",
		},
		{
			name: "event with empty resource name",
			events: []models.Event{
				{
					ID:        "event1",
					Timestamp: 1234567890000000000,
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Kind: "Pod",
						Name: "",
					},
				},
			},
			expectedValid:   0,
			expectedInvalid: 1,
			description:     "Event with empty resource name should be invalid",
		},
		{
			name: "mixed valid and invalid events",
			events: []models.Event{
				{
					ID:        "event1",
					Timestamp: 1234567890000000000,
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Kind: "Pod",
						Name: "valid-pod",
					},
				},
				{
					ID:        "",
					Timestamp: 1234567891000000000,
					Type:      models.EventTypeUpdate,
					Resource: models.ResourceMetadata{
						Kind: "Deployment",
						Name: "invalid-deployment",
					},
				},
				{
					ID:        "event3",
					Timestamp: 1234567892000000000,
					Type:      models.EventTypeDelete,
					Resource: models.ResourceMetadata{
						Kind: "Service",
						Name: "valid-service",
					},
				},
				{
					ID:        "event4",
					Timestamp: 0,
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Kind: "ConfigMap",
						Name: "invalid-configmap",
					},
				},
			},
			expectedValid:   2,
			expectedInvalid: 2,
			description:     "Should filter out invalid events and keep valid ones",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validEvents, invalidCount := validateEvents(tt.events, logger)

			if len(validEvents) != tt.expectedValid {
				t.Errorf("Expected %d valid events, got %d", tt.expectedValid, len(validEvents))
			}

			if invalidCount != tt.expectedInvalid {
				t.Errorf("Expected %d invalid events, got %d", tt.expectedInvalid, invalidCount)
			}
		})
	}
}

func TestImportWithValidation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a JSON file with mixed valid and invalid events
	testFile := tmpDir + "/test.json"
	testData := `{
		"events": [
			{
				"id": "valid-event",
				"timestamp": 1234567890000000000,
				"type": "CREATE",
				"resource": {
					"kind": "Pod",
					"name": "test-pod"
				}
			},
			{
				"id": "",
				"timestamp": 1234567891000000000,
				"type": "UPDATE",
				"resource": {
					"kind": "Deployment",
					"name": "test-deployment"
				}
			},
			{
				"id": "another-valid-event",
				"timestamp": 1234567892000000000,
				"type": "DELETE",
				"resource": {
					"kind": "Service",
					"name": "test-service"
				}
			}
		]
	}`

	if err := writeTestFile(testFile, testData); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	events, err := Import(FromFile(testFile))
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Should only import the 2 valid events (invalid one with empty ID should be filtered)
	if len(events) != 2 {
		t.Errorf("Expected 2 valid events, got %d", len(events))
	}

	// Verify the valid events are present
	if events[0].ID != "valid-event" {
		t.Errorf("Expected first event ID 'valid-event', got %q", events[0].ID)
	}
	if events[1].ID != "another-valid-event" {
		t.Errorf("Expected second event ID 'another-valid-event', got %q", events[1].ID)
	}
}

func TestImportAllInvalidEvents(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a JSON file with only invalid events
	testFile := tmpDir + "/test.json"
	testData := `{
		"events": [
			{
				"id": "",
				"timestamp": 1234567890000000000,
				"type": "CREATE",
				"resource": {
					"kind": "Pod",
					"name": "test-pod"
				}
			},
			{
				"id": "event2",
				"timestamp": 0,
				"type": "UPDATE",
				"resource": {
					"kind": "Deployment",
					"name": "test-deployment"
				}
			}
		]
	}`

	if err := writeTestFile(testFile, testData); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err := Import(FromFile(testFile))
	if err == nil {
		t.Error("Expected error when all events are invalid")
	}

	if !strings.Contains(err.Error(), "all") && !strings.Contains(err.Error(), "failed validation") {
		t.Errorf("Expected validation error, got: %v", err)
	}
}

func writeTestFile(path, content string) error {
	// This is a helper that would normally be in a test utils package
	// but we'll inline it here for simplicity
	return os.WriteFile(path, []byte(content), 0644)
}
