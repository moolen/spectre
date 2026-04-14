package importexport

import (
	"bytes"
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

// parseResult contains parsed events and non-fatal warnings.
type parseResult struct {
	Events   []*models.Event
	Warnings []string
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
	Warnings      []string
	Duration      time.Duration
}

// WalkAndImportJSON recursively walks a directory tree and imports all supported
// import files. It calls the progress callback for each file processed.
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

	// Collect all supported import files.
	var importFiles []string
	err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logger.Warn("Error accessing path %s: %v", path, err)
			return nil // Continue walking
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		if isSupportedImportFile(info.Name()) {
			importFiles = append(importFiles, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory tree: %w", err)
	}

	if len(importFiles) == 0 {
		logger.Warn("No supported import files found in directory: %s", dirPath)
		return &ImportReport{
			TotalFiles:    0,
			ImportedFiles: 0,
			MergedHours:   0,
			SkippedFiles:  0,
			FailedFiles:   0,
			TotalEvents:   0,
			Errors:        []string{},
			Warnings:      []string{},
		}, nil
	}

	logger.Info("Found %d supported import files to import", len(importFiles))

	// Aggregate all events from all files
	var allEvents []*models.Event
	var allWarnings []string
	filesProcessed := 0

	for _, filePath := range importFiles {
		events, warnings, err := ImportJSONFile(filePath)
		if err != nil {
			logger.Error("Failed to parse %s: %v", filePath, err)
			return nil, fmt.Errorf("failed to parse %s: %w", filePath, err)
		}
		filesProcessed++

		if len(warnings) > 0 {
			for _, warning := range warnings {
				allWarnings = append(allWarnings, fmt.Sprintf("%s: %s", filePath, warning))
			}
		}

		if len(events) == 0 {
			logger.Warn("No events in file: %s", filePath)
			continue
		}

		allEvents = append(allEvents, events...)

		// Call progress callback
		if progress != nil {
			progress(filePath, len(events))
		}

		logger.Debug("Loaded %d events from %s", len(events), filePath)
	}

	if len(allEvents) == 0 {
		logger.Warn("No events found in any supported import files")
		return &ImportReport{
			TotalFiles:    filesProcessed,
			ImportedFiles: 0,
			MergedHours:   0,
			SkippedFiles:  0,
			FailedFiles:   0,
			TotalEvents:   0,
			Errors:        []string{},
			Warnings:      allWarnings,
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
		Warnings:      allWarnings,
		Duration:      storageReport.Duration,
	}

	return report, nil
}

func isSupportedImportFile(name string) bool {
	lower := strings.ToLower(name)

	switch {
	case strings.HasSuffix(lower, ".json"):
		return true
	case strings.HasSuffix(lower, ".jsonl"):
		return true
	case strings.HasSuffix(lower, ".log"):
		return true
	default:
		return false
	}
}

// ImportJSONFile reads and parses a single JSON file containing an events array
func ImportJSONFile(filePath string) ([]*models.Event, []string, error) {
	file, err := os.Open(filePath) //nolint:gosec // filePath is validated before use
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			// Log error but don't fail the operation
		}
	}()

	return ParseImportPayload(file)
}

// ParseImportPayload parses a supported import payload format into events and warnings.
func ParseImportPayload(r io.Reader) ([]*models.Event, []string, error) {
	result, err := parseImportPayload(r)
	if err != nil {
		return nil, nil, err
	}

	return result.Events, result.Warnings, nil
}

func parseImportPayload(r io.Reader) (*parseResult, error) {
	payload, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read import payload: %w", err)
	}
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty file")
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &root); err == nil {
		if _, hasEvents := root["events"]; hasEvents {
			events, parseErr := ParseJSONEvents(bytes.NewReader(trimmed))
			if parseErr != nil {
				return nil, parseErr
			}
			return &parseResult{
				Events:   events,
				Warnings: []string{},
			}, nil
		}

		var kind string
		if err := json.Unmarshal(root["kind"], &kind); err == nil {
			var apiVersion string
			_ = json.Unmarshal(root["apiVersion"], &apiVersion)
			if (kind == "Event" || kind == "EventList") && isAuditAPIVersion(apiVersion) {
				return parseAuditObjectPayload(trimmed)
			}
		}

		// Keep existing behavior for unknown JSON objects.
		events, parseErr := ParseJSONEvents(bytes.NewReader(trimmed))
		if parseErr != nil {
			return nil, parseErr
		}
		return &parseResult{
			Events:   events,
			Warnings: []string{},
		}, nil
	}

	result, err := parseAuditJSONLPayload(trimmed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return result, nil
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

	// Enrich Kubernetes Event resources with involvedObject UID
	enrichEventsWithInvolvedObjectUID(req.Events)

	return req.Events, nil
}

// enrichEventsWithInvolvedObjectUID extracts the involvedObject.uid from Kubernetes Event resources
// and populates the InvolvedObjectUID field in resource metadata.
// This matches the behavior of the live watcher for file imports.
func enrichEventsWithInvolvedObjectUID(events []*models.Event) {
	for _, event := range events {
		// Only process Kubernetes Event resources
		if !strings.EqualFold(event.Resource.Kind, "Event") {
			continue
		}

		// Skip if data is empty
		if len(event.Data) == 0 {
			continue
		}

		if event.Resource.InvolvedObjectUID != "" {
			continue
		}

		// Extract involvedObject.uid from the JSON data
		var eventData map[string]interface{}
		if err := json.Unmarshal(event.Data, &eventData); err != nil {
			// Skip events that can't be parsed - this shouldn't happen but handle gracefully
			continue
		}

		// Navigate to involvedObject.uid
		involvedObject, ok := eventData["involvedObject"].(map[string]interface{})
		if !ok {
			continue
		}

		uid, ok := involvedObject["uid"].(string)
		if !ok || uid == "" {
			continue
		}

		// Populate the InvolvedObjectUID field
		event.Resource.InvolvedObjectUID = uid
	}
}
