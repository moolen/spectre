package importexport

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/models"
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

// ConvertEventsToValues converts a slice of event pointers to a slice of event values.
// This is needed because pipeline.ProcessBatch() expects []models.Event, not []*models.Event.
func ConvertEventsToValues(events []*models.Event) []models.Event {
	eventValues := make([]models.Event, len(events))
	for i, event := range events {
		eventValues[i] = *event
	}
	return eventValues
}

// ImportJSONFileAsValues reads and parses a JSON file and returns events as values (not pointers).
// This is a convenience function for code that needs []models.Event for pipeline processing.
func ImportJSONFileAsValues(filePath string) ([]models.Event, error) {
	events, err := ImportJSONFile(filePath)
	if err != nil {
		return nil, err
	}
	return ConvertEventsToValues(events), nil
}

// ImportPathAsValues reads and parses events from a file or directory and returns events as values.
// If the path is a file, it imports that file. If it's a directory, it walks recursively
// and imports all JSON files found. Returns all events combined from all files.
func ImportPathAsValues(path string) ([]models.Event, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	if !info.IsDir() {
		// Single file import
		return ImportJSONFileAsValues(path)
	}

	// Directory import - walk recursively
	var allEvents []*models.Event
	err = filepath.Walk(path, func(filePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking path %s: %w", filePath, err)
		}

		// Skip directories
		if fileInfo.IsDir() {
			return nil
		}

		// Only process JSON files
		if !strings.HasSuffix(strings.ToLower(filePath), ".json") {
			return nil
		}

		// Import the file
		events, err := ImportJSONFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to import file %s: %w", filePath, err)
		}

		allEvents = append(allEvents, events...)
		return nil
	})

	if err != nil {
		return nil, err
	}

	if len(allEvents) == 0 {
		return nil, fmt.Errorf("no events found in path %s", path)
	}

	return ConvertEventsToValues(allEvents), nil
}
