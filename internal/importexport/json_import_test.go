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
