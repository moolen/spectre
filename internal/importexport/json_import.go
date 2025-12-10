package importexport

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"github.com/moolen/spectre/internal/storage"
)

// ProgressCallback is called during import to report progress
type ProgressCallback func(filename string, eventCount int)

// BatchEventImportRequest represents a JSON request to import a batch of events
type BatchEventImportRequest struct {
	Events []*models.Event `json:"events"`
}

// ImportReport contains the results of an import operation
type ImportReport struct {
	TotalFiles    int
	ImportedFiles int
	MergedHours   int
	SkippedFiles  int
	FailedFiles   int
	TotalEvents   int64
	Errors        []string
	Duration      time.Duration
}

// WalkAndImportJSON recursively walks a directory tree and imports all JSON files
// containing event arrays. It calls the progress callback for each file processed.
func WalkAndImportJSON(dirPath string, st *storage.Storage, opts storage.ImportOptions, progress ProgressCallback) (*ImportReport, error) {
	logger := logging.GetLogger("importexport")

	// Verify directory exists
	info, err := os.Stat(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to access import directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("import path is not a directory: %s", dirPath)
	}

	// Collect all JSON files
	var jsonFiles []string
	err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logger.Warn("Error accessing path %s: %v", path, err)
			return nil // Continue walking
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process .json files
		if strings.HasSuffix(strings.ToLower(info.Name()), ".json") {
			jsonFiles = append(jsonFiles, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory tree: %w", err)
	}

	if len(jsonFiles) == 0 {
		logger.Warn("No JSON files found in directory: %s", dirPath)
		return &ImportReport{
			TotalFiles:    0,
			ImportedFiles: 0,
			MergedHours:   0,
			SkippedFiles:  0,
			FailedFiles:   0,
			TotalEvents:   0,
			Errors:        []string{},
		}, nil
	}

	logger.Info("Found %d JSON files to import", len(jsonFiles))

	// Aggregate all events from all files
	var allEvents []*models.Event
	filesProcessed := 0

	for _, filePath := range jsonFiles {
		events, err := ImportJSONFile(filePath)
		if err != nil {
			logger.Error("Failed to parse %s: %v", filePath, err)
			return nil, fmt.Errorf("failed to parse %s: %w", filePath, err)
		}

		if len(events) == 0 {
			logger.Warn("No events in file: %s", filePath)
			continue
		}

		allEvents = append(allEvents, events...)
		filesProcessed++

		// Call progress callback
		if progress != nil {
			progress(filePath, len(events))
		}

		logger.Debug("Loaded %d events from %s", len(events), filePath)
	}

	if len(allEvents) == 0 {
		logger.Warn("No events found in any JSON files")
		return &ImportReport{
			TotalFiles:    filesProcessed,
			ImportedFiles: 0,
			MergedHours:   0,
			SkippedFiles:  0,
			FailedFiles:   0,
			TotalEvents:   0,
			Errors:        []string{},
		}, nil
	}

	logger.Info("Importing %d total events from %d files", len(allEvents), filesProcessed)

	// Use storage's batch import functionality
	storageReport, err := st.AddEventsBatch(allEvents, opts)
	if err != nil {
		return nil, fmt.Errorf("batch import failed: %w", err)
	}

	logger.Info("Import completed: %d events, %d hours merged", storageReport.TotalEvents, storageReport.MergedHours)

	// Convert storage report to our import report
	report := &ImportReport{
		TotalFiles:    filesProcessed,
		ImportedFiles: storageReport.ImportedFiles,
		MergedHours:   storageReport.MergedHours,
		SkippedFiles:  storageReport.SkippedFiles,
		FailedFiles:   storageReport.FailedFiles,
		TotalEvents:   storageReport.TotalEvents,
		Errors:        storageReport.Errors,
		Duration:      storageReport.Duration,
	}

	return report, nil
}

// ImportJSONFile reads and parses a single JSON file containing an events array
func ImportJSONFile(filePath string) ([]*models.Event, error) {
	file, err := os.Open(filePath) //nolint:gosec // filePath is validated before use
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			// Log error but don't fail the operation
		}
	}()

	return ParseJSONEvents(file)
}

// ParseJSONEvents parses a JSON events array from a reader
func ParseJSONEvents(r io.Reader) ([]*models.Event, error) {
	var req BatchEventImportRequest
	decoder := json.NewDecoder(r)

	if err := decoder.Decode(&req); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("empty file")
		}
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if len(req.Events) == 0 {
		return nil, fmt.Errorf("events array is empty")
	}

	return req.Events, nil
}
