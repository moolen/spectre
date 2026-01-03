package importexport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
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
			logger := logging.GetLogger("test")
			events, err := Import(FromReader(reader), WithLogger(logger))

			if tt.wantErr {
				if err == nil {
					t.Errorf("Import() expected error, got nil")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Import() error = %v, want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("Import() unexpected error = %v", err)
				return
			}

			if len(events) != tt.wantCount {
				t.Errorf("Import() got %d events, want %d", len(events), tt.wantCount)
			}

			// Verify enrichment for the enrichment test case
			if tt.name == "enriches Event resources with involvedObjectUID" {
				if len(events) > 0 && events[0].Resource.InvolvedObjectUID != "pod-uid-123" {
					t.Errorf("Import() expected InvolvedObjectUID to be 'pod-uid-123', got %q", events[0].Resource.InvolvedObjectUID)
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
			logger := logging.GetLogger("test")
			events, err := Import(FromFile(tt.filePath), WithLogger(logger))

			if tt.wantErr {
				if err == nil {
					t.Errorf("Import() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Import() unexpected error = %v", err)
				return
			}

			if len(events) != tt.wantCount {
				t.Errorf("Import() got %d events, want %d", len(events), tt.wantCount)
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
		// Convert pointers to values for BatchEventImportRequest
		eventValues := make([]models.Event, len(events))
		for i, event := range events {
			eventValues[i] = *event
		}
		data := BatchEventImportRequest{Events: eventValues}
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

	// Storage-based import removed - test skipped
	// TODO: Reimplement with graph-based import
}

func TestWalkAndImportJSON_EmptyDirectory(t *testing.T) {
	t.Skip("Skipping storage-based import test - storage package removed, graph-based import needs to be implemented")
}

func TestWalkAndImportJSON_InvalidDirectory(t *testing.T) {
	t.Skip("Skipping storage-based import test - storage package removed, graph-based import needs to be implemented")
	// Storage-based import removed - test skipped
	// TODO: Reimplement with graph-based import
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

// TestEnrichEventsWithInvolvedObjectUID is deprecated and moved to the enrichment package
// This test is kept for backward compatibility but now tests the integration through
// the Import API which uses the enrichment package internally.
func TestEnrichEventsWithInvolvedObjectUID(t *testing.T) {
	t.Skip("This test has been migrated to internal/importexport/enrichment package. Use enrichment.TestInvolvedObjectUIDEnricher instead.")
}

// New API Tests

func TestImportFromFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test file
	validFile := filepath.Join(tmpDir, "test.json")
	testData := `{
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
	}`
	if err := os.WriteFile(validFile, []byte(testData), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test successful import
	events, err := Import(FromFile(validFile))
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}

	// Verify events are values, not pointers
	if events[0].ID != "event1" {
		t.Errorf("Expected event ID 'event1', got %q", events[0].ID)
	}

	// Test with custom logger
	logger := logging.GetLogger("test")
	events, err = Import(FromFile(validFile), WithLogger(logger))
	if err != nil {
		t.Fatalf("Import with logger failed: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}

	// Test non-existent file
	_, err = Import(FromFile(filepath.Join(tmpDir, "nonexistent.json")))
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestImportFromReader(t *testing.T) {
	testData := `{
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

	reader := strings.NewReader(testData)
	events, err := Import(FromReader(reader))
	if err != nil {
		t.Fatalf("Import from reader failed: %v", err)
	}

	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}

	if events[0].ID != "event1" {
		t.Errorf("Expected event ID 'event1', got %q", events[0].ID)
	}
}

func TestImportFromDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create subdirectories
	subDir1 := filepath.Join(tmpDir, "dir1")
	subDir2 := filepath.Join(tmpDir, "dir2")
	if err := os.MkdirAll(subDir1, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}
	if err := os.MkdirAll(subDir2, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Create test files
	files := map[string]int{
		filepath.Join(tmpDir, "file1.json"):       1,
		filepath.Join(subDir1, "file2.json"):      2,
		filepath.Join(subDir2, "file3.json"):      1,
		filepath.Join(tmpDir, "readme.txt"):       0, // Should be ignored
		filepath.Join(subDir1, "another.txt"):     0, // Should be ignored
	}

	createEventFile := func(path string, eventCount int) error {
		if !strings.HasSuffix(path, ".json") {
			// Create non-JSON file
			return os.WriteFile(path, []byte("ignored"), 0644)
		}

		events := make([]models.Event, eventCount)
		for i := 0; i < eventCount; i++ {
			events[i] = models.Event{
				ID:        fmt.Sprintf("event-%d", i+1),
				Timestamp: 1234567890000000000 + int64(i),
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					Group:     "apps",
					Version:   "v1",
					Kind:      "Deployment",
					Namespace: "default",
					Name:      fmt.Sprintf("deploy-%d", i+1),
					UID:       fmt.Sprintf("uid-%d", i+1),
				},
			}
		}

		data := BatchEventImportRequest{Events: events}
		jsonData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(path, jsonData, 0644)
	}

	for path, count := range files {
		if err := createEventFile(path, count); err != nil {
			t.Fatalf("Failed to create file %s: %v", path, err)
		}
	}

	// Test directory import
	events, err := Import(FromDirectory(tmpDir))
	if err != nil {
		t.Fatalf("Import from directory failed: %v", err)
	}

	// Should import 4 events total (1 + 2 + 1, ignoring non-JSON files)
	if len(events) != 4 {
		t.Errorf("Expected 4 events, got %d", len(events))
	}

	// Test empty directory
	emptyDir := filepath.Join(tmpDir, "empty")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatalf("Failed to create empty directory: %v", err)
	}

	_, err = Import(FromDirectory(emptyDir))
	if err == nil {
		t.Error("Expected error for empty directory")
	}
	if !strings.Contains(err.Error(), "no JSON files found") {
		t.Errorf("Expected 'no JSON files found' error, got: %v", err)
	}
}

func TestImportFromPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file
	testFile := filepath.Join(tmpDir, "test.json")
	testData := `{
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
	if err := os.WriteFile(testFile, []byte(testData), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test file path
	events, err := Import(FromPath(testFile))
	if err != nil {
		t.Fatalf("Import from file path failed: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}

	// Create a directory with file
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}
	subFile := filepath.Join(subDir, "sub.json")
	if err := os.WriteFile(subFile, []byte(testData), 0644); err != nil {
		t.Fatalf("Failed to create sub file: %v", err)
	}

	// Test directory path
	events, err = Import(FromPath(subDir))
	if err != nil {
		t.Fatalf("Import from directory path failed: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(events))
	}

	// Test non-existent path
	_, err = Import(FromPath(filepath.Join(tmpDir, "nonexistent")))
	if err == nil {
		t.Error("Expected error for non-existent path")
	}
}

func TestImportEventEnrichment(t *testing.T) {
	tmpDir := t.TempDir()

	// Create file with Kubernetes Event that needs enrichment
	testFile := filepath.Join(tmpDir, "events.json")
	testData := `{
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
	}`
	if err := os.WriteFile(testFile, []byte(testData), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	events, err := Import(FromFile(testFile))
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	// Verify enrichment occurred
	if events[0].Resource.InvolvedObjectUID != "pod-uid-123" {
		t.Errorf("Expected InvolvedObjectUID 'pod-uid-123', got %q",
			events[0].Resource.InvolvedObjectUID)
	}
}

func TestBackwardCompatibility(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.json")
	testData := `{
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
	if err := os.WriteFile(testFile, []byte(testData), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test old API functions still work
	t.Run("ImportJSONFile", func(t *testing.T) {
		events, err := ImportJSONFile(testFile)
		if err != nil {
			t.Fatalf("ImportJSONFile failed: %v", err)
		}
		if len(events) != 1 {
			t.Errorf("Expected 1 event, got %d", len(events))
		}
		// Should return pointers
		if events[0] == nil {
			t.Error("Expected non-nil event pointer")
		}
	})

	t.Run("ImportJSONFileAsValues", func(t *testing.T) {
		events, err := ImportJSONFileAsValues(testFile)
		if err != nil {
			t.Fatalf("ImportJSONFileAsValues failed: %v", err)
		}
		if len(events) != 1 {
			t.Errorf("Expected 1 event, got %d", len(events))
		}
		// Should return values
		if events[0].ID != "event1" {
			t.Errorf("Expected event ID 'event1', got %q", events[0].ID)
		}
	})

	t.Run("ParseJSONEvents", func(t *testing.T) {
		reader := strings.NewReader(testData)
		events, err := ParseJSONEvents(reader)
		if err != nil {
			t.Fatalf("ParseJSONEvents failed: %v", err)
		}
		if len(events) != 1 {
			t.Errorf("Expected 1 event, got %d", len(events))
		}
		// Should return pointers
		if events[0] == nil {
			t.Error("Expected non-nil event pointer")
		}
	})

	t.Run("ConvertEventsToValues", func(t *testing.T) {
		ptrs := []*models.Event{
			{
				ID:        "test1",
				Timestamp: 123,
				Type:      models.EventTypeCreate,
			},
			{
				ID:        "test2",
				Timestamp: 456,
				Type:      models.EventTypeUpdate,
			},
		}

		values := ConvertEventsToValues(ptrs)
		if len(values) != 2 {
			t.Errorf("Expected 2 values, got %d", len(values))
		}
		if values[0].ID != "test1" {
			t.Errorf("Expected ID 'test1', got %q", values[0].ID)
		}
		if values[1].ID != "test2" {
			t.Errorf("Expected ID 'test2', got %q", values[1].ID)
		}
	})
}
