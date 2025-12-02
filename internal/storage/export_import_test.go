package storage

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
)

func TestExportImport(t *testing.T) {
	// Create temporary directories for source and destination
	sourceDir, err := os.MkdirTemp("", "spectre-export-test-source-*")
	if err != nil {
		t.Fatalf("Failed to create source temp dir: %v", err)
	}
	defer os.RemoveAll(sourceDir)

	destDir, err := os.MkdirTemp("", "spectre-export-test-dest-*")
	if err != nil {
		t.Fatalf("Failed to create dest temp dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	// Create source storage and write some test events
	sourceStorage, err := New(sourceDir, 256*1024)
	if err != nil {
		t.Fatalf("Failed to create source storage: %v", err)
	}

	// Write test events across multiple hours
	baseTime := time.Date(2024, 12, 1, 10, 0, 0, 0, time.UTC)
	eventCount := 100

	for i := 0; i < eventCount; i++ {
		event := &models.Event{
			ID:        "test-event-" + string(rune(i)),
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute).UnixNano(),
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Group:     "apps",
				Version:   "v1",
				Kind:      "Deployment",
				Namespace: "default",
				Name:      "test-deployment",
			},
			Data: json.RawMessage(`{"test": "data"}`),
		}

		if err := sourceStorage.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event %d: %v", i, err)
		}
	}

	// Close source storage to finalize files
	if err := sourceStorage.Close(); err != nil {
		t.Fatalf("Failed to close source storage: %v", err)
	}

	// Export the data
	var exportBuffer bytes.Buffer
	exportOpts := ExportOptions{
		StartTime:   0,
		EndTime:     0,
		Compression: true,
		ClusterID:   "test-cluster",
		InstanceID:  "test-instance",
	}

	// Reopen storage for export
	sourceStorage, err = New(sourceDir, 256*1024)
	if err != nil {
		t.Fatalf("Failed to reopen source storage: %v", err)
	}

	if err := sourceStorage.Export(&exportBuffer, exportOpts); err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	t.Logf("Export size: %d bytes", exportBuffer.Len())

	// Import into destination
	destStorage, err := New(destDir, 256*1024)
	if err != nil {
		t.Fatalf("Failed to create dest storage: %v", err)
	}

	importOpts := ImportOptions{
		ValidateFiles:     true,
		OverwriteExisting: true,
	}

	report, err := destStorage.Import(&exportBuffer, importOpts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Verify import report
	if report.TotalFiles == 0 {
		t.Error("Expected at least one file to be imported")
	}

	if report.FailedFiles > 0 {
		t.Errorf("Expected no failed files, got %d", report.FailedFiles)
		for _, errMsg := range report.Errors {
			t.Logf("Import error: %s", errMsg)
		}
	}

	if report.TotalEvents != int64(eventCount) {
		t.Errorf("Expected %d events, got %d", eventCount, report.TotalEvents)
	}

	t.Logf("Import report: files=%d, merged_hours=%d, events=%d, duration=%v",
		report.ImportedFiles, report.MergedHours, report.TotalEvents, report.Duration)

	// Verify that we can query the imported data
	queryExecutor := NewQueryExecutor(destStorage)
	queryResult, err := queryExecutor.Execute(&models.QueryRequest{
		StartTimestamp: baseTime.Unix(),
		EndTimestamp:   baseTime.Add(2 * time.Hour).Unix(),
		Filters: models.QueryFilters{
			Kind: "Deployment",
		},
	})

	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(queryResult.Events) != eventCount {
		t.Errorf("Expected %d events from query, got %d", eventCount, len(queryResult.Events))
	}
}

func TestExportTimeRange(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "spectre-export-timerange-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Use current time to ensure files are created properly
	now := time.Now()
	currentHour := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	// Create storage and write events for current hour
	storage, err := New(tempDir, 256*1024)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Write events for the current hour
	for i := 0; i < 10; i++ {
		event := &models.Event{
			ID:        "test-event",
			Timestamp: currentHour.Add(time.Duration(i) * time.Minute).UnixNano(),
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Group:     "apps",
				Version:   "v1",
				Kind:      "Deployment",
				Namespace: "default",
				Name:      "test-deployment",
			},
			Data: json.RawMessage(`{"test": "data"}`),
		}

		if err := storage.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}

	if err := storage.Close(); err != nil {
		t.Fatalf("Failed to close storage: %v", err)
	}

	// Reopen storage for export
	storage, err = New(tempDir, 256*1024)
	if err != nil {
		t.Fatalf("Failed to reopen storage: %v", err)
	}

	// Export the current hour
	var exportBuffer bytes.Buffer
	exportOpts := ExportOptions{
		StartTime:   currentHour.Unix(),
		EndTime:     currentHour.Add(1 * time.Hour).Unix(),
		Compression: false,
	}

	if err := storage.Export(&exportBuffer, exportOpts); err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Verify that export succeeded
	if exportBuffer.Len() == 0 {
		t.Error("Export buffer is empty")
	}

	t.Logf("Exported %d bytes for hour %s", exportBuffer.Len(), currentHour.Format("2006-01-02-15"))
}

func TestImportMerge(t *testing.T) {
	// Create temporary directories
	sourceDir, err := os.MkdirTemp("", "spectre-merge-test-source-*")
	if err != nil {
		t.Fatalf("Failed to create source temp dir: %v", err)
	}
	defer os.RemoveAll(sourceDir)

	destDir, err := os.MkdirTemp("", "spectre-merge-test-dest-*")
	if err != nil {
		t.Fatalf("Failed to create dest temp dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	// Create source storage with some events
	sourceStorage, err := New(sourceDir, 256*1024)
	if err != nil {
		t.Fatalf("Failed to create source storage: %v", err)
	}

	baseTime := time.Date(2024, 12, 1, 10, 0, 0, 0, time.UTC)

	// Write 50 events to source
	for i := 0; i < 50; i++ {
		event := &models.Event{
			ID:        "source-event",
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute).UnixNano(),
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Group:     "apps",
				Version:   "v1",
				Kind:      "Deployment",
				Namespace: "source",
				Name:      "test-deployment",
			},
			Data: json.RawMessage(`{"source": "data"}`),
		}

		if err := sourceStorage.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write source event: %v", err)
		}
	}

	if err := sourceStorage.Close(); err != nil {
		t.Fatalf("Failed to close source storage: %v", err)
	}

	// Create destination storage with different events in the same hour
	destStorage, err := New(destDir, 256*1024)
	if err != nil {
		t.Fatalf("Failed to create dest storage: %v", err)
	}

	// Write 50 different events to destination
	for i := 0; i < 50; i++ {
		event := &models.Event{
			ID:        "dest-event",
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute).UnixNano(),
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Group:     "apps",
				Version:   "v1",
				Kind:      "Deployment",
				Namespace: "dest",
				Name:      "test-deployment",
			},
			Data: json.RawMessage(`{"dest": "data"}`),
		}

		if err := destStorage.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write dest event: %v", err)
		}
	}

	if err := destStorage.Close(); err != nil {
		t.Fatalf("Failed to close dest storage: %v", err)
	}

	// Export from source
	sourceStorage, err = New(sourceDir, 256*1024)
	if err != nil {
		t.Fatalf("Failed to reopen source storage: %v", err)
	}

	var exportBuffer bytes.Buffer
	exportOpts := ExportOptions{
		Compression: true,
	}

	if err := sourceStorage.Export(&exportBuffer, exportOpts); err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// Import into destination (should merge)
	destStorage, err = New(destDir, 256*1024)
	if err != nil {
		t.Fatalf("Failed to reopen dest storage: %v", err)
	}

	importOpts := ImportOptions{
		ValidateFiles:     true,
		OverwriteExisting: true,
	}

	report, err := destStorage.Import(&exportBuffer, importOpts)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Should have merged 100 events total (50 from source + 50 from dest)
	if report.TotalEvents != 100 {
		t.Errorf("Expected 100 merged events, got %d", report.TotalEvents)
	}

	// Verify we can query both sets of events
	queryExecutor := NewQueryExecutor(destStorage)

	// Query for source events
	sourceResult, err := queryExecutor.Execute(&models.QueryRequest{
		StartTimestamp: baseTime.Unix(),
		EndTimestamp:   baseTime.Add(2 * time.Hour).Unix(),
		Filters: models.QueryFilters{
			Namespace: "source",
		},
	})

	if err != nil {
		t.Fatalf("Query for source events failed: %v", err)
	}

	if len(sourceResult.Events) != 50 {
		t.Errorf("Expected 50 source events, got %d", len(sourceResult.Events))
	}

	// Query for dest events
	destResult, err := queryExecutor.Execute(&models.QueryRequest{
		StartTimestamp: baseTime.Unix(),
		EndTimestamp:   baseTime.Add(2 * time.Hour).Unix(),
		Filters: models.QueryFilters{
			Namespace: "dest",
		},
	})

	if err != nil {
		t.Fatalf("Query for dest events failed: %v", err)
	}

	if len(destResult.Events) != 50 {
		t.Errorf("Expected 50 dest events, got %d", len(destResult.Events))
	}
}

func TestExportEmptyStorage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "spectre-export-empty-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage, err := New(tempDir, 256*1024)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	var exportBuffer bytes.Buffer
	exportOpts := ExportOptions{
		Compression: true,
	}

	err = storage.Export(&exportBuffer, exportOpts)
	if err == nil {
		t.Error("Expected error when exporting empty storage, got nil")
	}
}

func TestImportValidation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "spectre-import-validation-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage, err := New(tempDir, 256*1024)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Try to import invalid data
	invalidData := bytes.NewBufferString("invalid tar data")
	importOpts := ImportOptions{
		ValidateFiles: true,
	}

	_, err = storage.Import(invalidData, importOpts)
	if err == nil {
		t.Error("Expected error when importing invalid data, got nil")
	}
}

func TestExtractHourFromFilename(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "spectre-hour-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage, err := New(tempDir, 256*1024)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	tests := []struct {
		filename string
		expected int64
		wantErr  bool
	}{
		{
			filename: filepath.Join(tempDir, "2024-12-01-10.bin"),
			expected: time.Date(2024, 12, 1, 10, 0, 0, 0, time.UTC).Unix(),
			wantErr:  false,
		},
		{
			filename: filepath.Join(tempDir, "2024-01-15-23.bin"),
			expected: time.Date(2024, 1, 15, 23, 0, 0, 0, time.UTC).Unix(),
			wantErr:  false,
		},
		{
			filename: filepath.Join(tempDir, "invalid.bin"),
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result, err := storage.extractHourFromFilename(tt.filename)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %d, got %d", tt.expected, result)
				}
			}
		})
	}
}

