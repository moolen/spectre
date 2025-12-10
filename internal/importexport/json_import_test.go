package importexport

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/moolen/spectre/internal/storage"
)

func TestParseJSONEvents(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantCount   int
		wantErr     bool
		errContains string
	}{
		{
			name: "valid events array",
			input: `{
				"events": [
					{
						"id": "event1",
						"timestamp": 1234567890000000000,
						"type": "CREATE",
						"resource": {
							"group": "apps",
							"version": "v1",
							"kind": "Deployment",
							"namespace": "default",
							"name": "test-deployment",
							"uid": "test-uid"
						}
					}
				]
			}`,
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:        "empty file",
			input:       "",
			wantErr:     true,
			errContains: "empty file",
		},
		{
			name: "empty events array",
			input: `{
				"events": []
			}`,
			wantErr:     true,
			errContains: "events array is empty",
		},
		{
			name:        "invalid JSON",
			input:       `{"events": [invalid`,
			wantErr:     true,
			errContains: "failed to parse JSON",
		},
		{
			name: "multiple events",
			input: `{
				"events": [
					{
						"id": "event1",
						"timestamp": 1234567890000000000,
						"type": "CREATE",
						"resource": {
							"group": "apps",
							"version": "v1",
							"kind": "Deployment",
							"namespace": "default",
							"name": "test-deployment-1",
							"uid": "test-uid-1"
						}
					},
					{
						"id": "event2",
						"timestamp": 1234567891000000000,
						"type": "UPDATE",
						"resource": {
							"group": "apps",
							"version": "v1",
							"kind": "Deployment",
							"namespace": "default",
							"name": "test-deployment-2",
							"uid": "test-uid-2"
						}
					}
				]
			}`,
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "enriches Event resources with involvedObjectUID",
			input: `{
				"events": [
					{
						"id": "event1",
						"timestamp": 1234567890000000000,
						"type": "CREATE",
						"resource": {
							"group": "",
							"version": "v1",
							"kind": "Event",
							"namespace": "default",
							"name": "test-event",
							"uid": "event-uid-1"
						},
						"data": {
							"involvedObject": {
								"uid": "pod-uid-123",
								"kind": "Pod",
								"name": "test-pod"
							}
						}
					}
				]
			}`,
			wantCount: 1,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			events, err := ParseJSONEvents(reader)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseJSONEvents() expected error, got nil")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ParseJSONEvents() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseJSONEvents() unexpected error = %v", err)
				return
			}

			if len(events) != tt.wantCount {
				t.Errorf("ParseJSONEvents() got %d events, want %d", len(events), tt.wantCount)
			}

			// Verify enrichment for the enrichment test case
			if tt.name == "enriches Event resources with involvedObjectUID" {
				if len(events) > 0 && events[0].Resource.InvolvedObjectUID != "pod-uid-123" {
					t.Errorf("ParseJSONEvents() expected InvolvedObjectUID to be 'pod-uid-123', got %q", events[0].Resource.InvolvedObjectUID)
				}
			}
		})
	}
}

func TestImportJSONFile(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create a valid JSON file
	validFile := filepath.Join(tmpDir, "valid.json")
	validData := `{
		"events": [
			{
				"id": "event1",
				"timestamp": 1234567890000000000,
				"type": "CREATE",
				"resource": {
					"group": "apps",
					"version": "v1",
					"kind": "Deployment",
					"namespace": "default",
					"name": "test-deployment",
					"uid": "test-uid"
				}
			}
		]
	}`
	if err := os.WriteFile(validFile, []byte(validData), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create an invalid JSON file
	invalidFile := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(invalidFile, []byte(`{invalid}`), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name      string
		filePath  string
		wantCount int
		wantErr   bool
	}{
		{
			name:      "valid file",
			filePath:  validFile,
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:     "invalid JSON",
			filePath: invalidFile,
			wantErr:  true,
		},
		{
			name:     "non-existent file",
			filePath: filepath.Join(tmpDir, "nonexistent.json"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := ImportJSONFile(tt.filePath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ImportJSONFile() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ImportJSONFile() unexpected error = %v", err)
				return
			}

			if len(events) != tt.wantCount {
				t.Errorf("ImportJSONFile() got %d events, want %d", len(events), tt.wantCount)
			}
		})
	}
}

func TestWalkAndImportJSON(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create subdirectories
	subDir1 := filepath.Join(tmpDir, "dir1")
	subDir2 := filepath.Join(tmpDir, "dir2")
	if err := os.MkdirAll(subDir1, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	if err := os.MkdirAll(subDir2, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Helper to create an event
	createEvent := func(id string, timestamp int64) *models.Event {
		return &models.Event{
			ID:        id,
			Timestamp: timestamp,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Group:     "apps",
				Version:   "v1",
				Kind:      "Deployment",
				Namespace: "default",
				Name:      id,
				UID:       id + "-uid",
			},
		}
	}

	// Create JSON files with events
	writeEventsFile := func(path string, events []*models.Event) error {
		data := BatchEventImportRequest{Events: events}
		jsonData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(path, jsonData, 0644)
	}

	// Create test files
	if err := writeEventsFile(filepath.Join(tmpDir, "file1.json"), []*models.Event{
		createEvent("event1", 1234567890000000000),
	}); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if err := writeEventsFile(filepath.Join(subDir1, "file2.json"), []*models.Event{
		createEvent("event2", 1234567891000000000),
		createEvent("event3", 1234567892000000000),
	}); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	if err := writeEventsFile(filepath.Join(subDir2, "file3.json"), []*models.Event{
		createEvent("event4", 1234567893000000000),
	}); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a non-JSON file that should be ignored
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("ignore me"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create temporary storage
	storageDir := t.TempDir()
	st, err := storage.New(storageDir, 10*1024*1024)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer st.Close()

	// Track progress
	var progressCalls int
	progressCallback := func(filename string, eventCount int) {
		progressCalls++
	}

	// Test import
	opts := storage.ImportOptions{
		ValidateFiles:     true,
		OverwriteExisting: true,
	}

	report, err := WalkAndImportJSON(tmpDir, st, opts, progressCallback)
	if err != nil {
		t.Fatalf("WalkAndImportJSON() error = %v", err)
	}

	// Verify results
	if report.TotalEvents != 4 {
		t.Errorf("Expected 4 total events, got %d", report.TotalEvents)
	}

	if progressCalls != 3 {
		t.Errorf("Expected 3 progress callbacks (one per JSON file), got %d", progressCalls)
	}

	if report.TotalFiles != 3 {
		t.Errorf("Expected 3 files processed, got %d", report.TotalFiles)
	}
}

func TestWalkAndImportJSON_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	storageDir := t.TempDir()
	st, err := storage.New(storageDir, 10*1024*1024)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer st.Close()

	opts := storage.ImportOptions{
		ValidateFiles:     true,
		OverwriteExisting: true,
	}

	report, err := WalkAndImportJSON(tmpDir, st, opts, nil)
	if err != nil {
		t.Fatalf("WalkAndImportJSON() error = %v", err)
	}

	if report.TotalEvents != 0 {
		t.Errorf("Expected 0 total events, got %d", report.TotalEvents)
	}
}

func TestWalkAndImportJSON_InvalidDirectory(t *testing.T) {
	storageDir := t.TempDir()
	st, err := storage.New(storageDir, 10*1024*1024)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer st.Close()

	opts := storage.ImportOptions{
		ValidateFiles:     true,
		OverwriteExisting: true,
	}

	_, err = WalkAndImportJSON("/nonexistent/path", st, opts, nil)
	if err == nil {
		t.Error("Expected error for non-existent directory, got nil")
	}
}

func TestFormatImportReport(t *testing.T) {
	report := &ImportReport{
		TotalFiles:    5,
		ImportedFiles: 4,
		MergedHours:   2,
		SkippedFiles:  1,
		FailedFiles:   0,
		TotalEvents:   100,
		Errors:        []string{},
	}

	output := FormatImportReport(report)

	if !strings.Contains(output, "Import Summary") {
		t.Error("Expected output to contain 'Import Summary'")
	}

	if !strings.Contains(output, "100") {
		t.Error("Expected output to contain total events count")
	}

	if !strings.Contains(output, "Merged Hours:   2") {
		t.Error("Expected output to contain merged hours")
	}
}

func TestEnrichEventsWithInvolvedObjectUID(t *testing.T) {
	tests := []struct {
		name         string
		events       []*models.Event
		expectedUIDs []string // Expected InvolvedObjectUID values in order
		description  string
	}{
		{
			name: "populates InvolvedObjectUID for Event resources",
			events: []*models.Event{
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
			events: []*models.Event{
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
			events: []*models.Event{
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
			events: []*models.Event{
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
			name: "handles missing involvedObject field",
			events: []*models.Event{
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
						"someOtherField": "value"
					}`),
				},
			},
			expectedUIDs: []string{""},
			description:  "Should gracefully handle missing involvedObject field",
		},
		{
			name: "handles invalid involvedObject structure",
			events: []*models.Event{
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
						"involvedObject": "not-a-map"
					}`),
				},
			},
			expectedUIDs: []string{""},
			description:  "Should gracefully handle invalid involvedObject structure",
		},
		{
			name: "handles missing uid in involvedObject",
			events: []*models.Event{
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
							"name": "test-pod",
							"kind": "Pod"
						}
					}`),
				},
			},
			expectedUIDs: []string{""},
			description:  "Should gracefully handle missing uid field",
		},
		{
			name: "handles empty uid string",
			events: []*models.Event{
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
							"uid": ""
						}
					}`),
				},
			},
			expectedUIDs: []string{""},
			description:  "Should not populate empty uid string",
		},
		{
			name: "handles invalid JSON gracefully",
			events: []*models.Event{
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
					Data: json.RawMessage(`{invalid json`),
				},
			},
			expectedUIDs: []string{""},
			description:  "Should gracefully handle invalid JSON data",
		},
		{
			name: "processes multiple events correctly",
			events: []*models.Event{
				{
					ID:        "event1",
					Timestamp: 1234567890000000000,
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Group:     "",
						Version:   "v1",
						Kind:      "Event",
						Namespace: "default",
						Name:      "test-event-1",
						UID:       "event-uid-1",
					},
					Data: json.RawMessage(`{
						"involvedObject": {
							"uid": "pod-uid-1"
						}
					}`),
				},
				{
					ID:        "event2",
					Timestamp: 1234567891000000000,
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
				{
					ID:        "event3",
					Timestamp: 1234567892000000000,
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Group:     "",
						Version:   "v1",
						Kind:      "Event",
						Namespace: "default",
						Name:      "test-event-2",
						UID:       "event-uid-2",
					},
					Data: json.RawMessage(`{
						"involvedObject": {
							"uid": "pod-uid-2"
						}
					}`),
				},
			},
			expectedUIDs: []string{"pod-uid-1", "", "pod-uid-2"},
			description:  "Should process multiple events correctly, only enriching Event resources",
		},
		{
			name: "case-insensitive kind matching",
			events: []*models.Event{
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
				{
					ID:        "event2",
					Timestamp: 1234567891000000000,
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Group:     "",
						Version:   "v1",
						Kind:      "EVENT", // uppercase
						Namespace: "default",
						Name:      "test-event-2",
						UID:       "event-uid-2",
					},
					Data: json.RawMessage(`{
						"involvedObject": {
							"uid": "pod-uid-456"
						}
					}`),
				},
			},
			expectedUIDs: []string{"pod-uid-123", "pod-uid-456"},
			description:  "Should match Event kind case-insensitively",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy of events to avoid modifying the original test data
			eventsCopy := make([]*models.Event, len(tt.events))
			for i, event := range tt.events {
				eventCopy := *event
				resourceCopy := event.Resource
				eventCopy.Resource = resourceCopy
				eventsCopy[i] = &eventCopy
			}

			// Call the function
			enrichEventsWithInvolvedObjectUID(eventsCopy)

			// Verify results
			if len(eventsCopy) != len(tt.expectedUIDs) {
				t.Fatalf("Expected %d events, got %d", len(tt.expectedUIDs), len(eventsCopy))
			}

			for i, event := range eventsCopy {
				expectedUID := tt.expectedUIDs[i]
				if event.Resource.InvolvedObjectUID != expectedUID {
					t.Errorf("Event %d: Expected InvolvedObjectUID %q, got %q", i, expectedUID, event.Resource.InvolvedObjectUID)
				}
			}
		})
	}
}
