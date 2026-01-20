// Package importexport provides utilities for importing Kubernetes events from JSON files.
//
// This package is primarily used for:
//   - Testing: Loading event fixtures for integration and E2E tests
//   - Debugging: Importing captured events for offline analysis
//   - Batch Processing: Ingesting historical event data from exports
//
// The package handles:
//   - JSON parsing and validation
//   - Kubernetes Event enrichment (involvedObject UID extraction)
//   - Single file and directory traversal imports
//
// # Basic Usage
//
// Import from a single file:
//
//	events, err := importexport.Import(importexport.FromFile("/path/to/events.json"))
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Import from a directory (recursive):
//
//	events, err := importexport.Import(importexport.FromDirectory("/path/to/events"))
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Import from an io.Reader with logging:
//
//	logger := logging.New("importer")
//	events, err := importexport.Import(
//	    importexport.FromReader(reader),
//	    importexport.WithLogger(logger),
//	)
//
// # Event Format
//
// Events must be in JSON format with the following structure:
//
//	{
//	  "events": [
//	    {
//	      "id": "unique-event-id",
//	      "timestamp": 1234567890000000000,
//	      "type": "CREATE",
//	      "resource": {
//	        "group": "apps",
//	        "version": "v1",
//	        "kind": "Deployment",
//	        "namespace": "default",
//	        "name": "my-app",
//	        "uid": "abc-123"
//	      },
//	      "data": { ... }
//	    }
//	  ]
//	}
//
// # Enrichment
//
// The package automatically enriches Kubernetes Event resources by extracting
// the involvedObject.uid from the event data and populating the InvolvedObjectUID
// field in the resource metadata. This matches the behavior of the live watcher.
package importexport

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/moolen/spectre/internal/importexport/enrichment"
	"github.com/moolen/spectre/internal/importexport/fileio"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// BatchEventImportRequest represents a JSON request to import a batch of events
type BatchEventImportRequest struct {
	Events []models.Event `json:"events"`
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

// ImportSource represents the source of events to import
type ImportSource interface {
	Load(logger *logging.Logger) ([]models.Event, error)
}

// ImportOptions configures the import behavior
type ImportOptions struct {
	logger *logging.Logger
}

// ImportOption is a functional option for configuring imports
type ImportOption func(*ImportOptions)

// WithLogger configures import operations to use the specified logger
func WithLogger(logger *logging.Logger) ImportOption {
	return func(opts *ImportOptions) {
		opts.logger = logger
	}
}

// Import loads events from the specified source with optional configuration.
// This is the primary entry point for importing events.
//
// Example:
//
//	events, err := Import(FromFile("events.json"), WithLogger(logger))
func Import(source ImportSource, opts ...ImportOption) ([]models.Event, error) {
	options := &ImportOptions{
		logger: logging.GetLogger("importexport"),
	}

	for _, opt := range opts {
		opt(options)
	}

	return source.Load(options.logger)
}

// fileSource imports events from a single JSON file
type fileSource struct {
	path string
}

// FromFile creates an import source for a single JSON file
func FromFile(path string) ImportSource {
	return &fileSource{path: path}
}

func (s *fileSource) Load(logger *logging.Logger) ([]models.Event, error) {
	logger.InfoWithFields("Loading events from file",
		logging.Field("path", s.path))

	reader := fileio.NewFileReader(logger)
	file, err := reader.ReadFile(s.path)
	if err != nil {
		logger.ErrorWithFields("Failed to read file",
			logging.Field("path", s.path),
			logging.Field("error", err))
		return nil, err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Warn("Failed to close file %s: %v", s.path, closeErr)
		}
	}()

	events, err := parseJSONEvents(file, logger)
	if err != nil {
		logger.ErrorWithFields("Failed to parse JSON events",
			logging.Field("path", s.path),
			logging.Field("error", err))
		return nil, err
	}

	logger.InfoWithFields("Successfully loaded events from file",
		logging.Field("path", s.path),
		logging.Field("event_count", len(events)))

	return events, nil
}

// readerSource imports events from an io.Reader
type readerSource struct {
	reader io.Reader
}

// FromReader creates an import source for an io.Reader
func FromReader(reader io.Reader) ImportSource {
	return &readerSource{reader: reader}
}

func (s *readerSource) Load(logger *logging.Logger) ([]models.Event, error) {
	logger.Debug("Loading events from reader")

	events, err := parseJSONEvents(s.reader, logger)
	if err != nil {
		logger.ErrorWithFields("Failed to parse JSON events from reader",
			logging.Field("error", err))
		return nil, err
	}

	logger.InfoWithFields("Successfully loaded events from reader",
		logging.Field("event_count", len(events)))

	return events, nil
}

// directorySource imports events from all JSON files in a directory (recursive)
type directorySource struct {
	path string
}

// FromDirectory creates an import source that recursively imports all JSON files
// in the specified directory
func FromDirectory(path string) ImportSource {
	return &directorySource{path: path}
}

func (s *directorySource) Load(logger *logging.Logger) ([]models.Event, error) {
	logger.InfoWithFields("Loading events from directory",
		logging.Field("path", s.path))

	// Use the fileio walker to find JSON files
	walker := fileio.NewDirectoryWalker(logger)
	files, err := walker.WalkJSON(s.path)
	if err != nil {
		logger.ErrorWithFields("Failed to walk directory",
			logging.Field("path", s.path),
			logging.Field("error", err))
		return nil, err
	}

	logger.InfoWithFields("Found JSON files in directory",
		logging.Field("path", s.path),
		logging.Field("file_count", len(files)))

	var allEvents []models.Event
	successCount := 0
	failureCount := 0

	// Import each file
	for _, file := range files {
		logger.DebugWithFields("Importing file",
			logging.Field("path", file.FilePath),
			logging.Field("size_bytes", file.Size))

		events, err := FromFile(file.FilePath).Load(logger)
		if err != nil {
			failureCount++
			logger.WarnWithFields("Failed to import file, skipping",
				logging.Field("path", file.FilePath),
				logging.Field("error", err))
			continue
		}

		allEvents = append(allEvents, events...)
		successCount++
	}

	if len(allEvents) == 0 {
		logger.ErrorWithFields("No events imported from directory",
			logging.Field("path", s.path),
			logging.Field("files_found", len(files)),
			logging.Field("failures", failureCount))
		return nil, fmt.Errorf("no events found in directory %s (processed %d files, %d failures)", s.path, len(files), failureCount)
	}

	logger.InfoWithFields("Successfully loaded events from directory",
		logging.Field("path", s.path),
		logging.Field("event_count", len(allEvents)),
		logging.Field("files_processed", successCount),
		logging.Field("files_failed", failureCount))

	return allEvents, nil
}

// pathSource automatically detects whether the path is a file or directory
type pathSource struct {
	path string
}

// FromPath creates an import source that automatically detects whether the path
// is a file or directory and imports accordingly
func FromPath(path string) ImportSource {
	return &pathSource{path: path}
}

func (s *pathSource) Load(logger *logging.Logger) ([]models.Event, error) {
	logger.DebugWithFields("Detecting path type",
		logging.Field("path", s.path))

	pathType, err := fileio.DetectPathType(s.path)
	if err != nil {
		logger.ErrorWithFields("Failed to detect path type",
			logging.Field("path", s.path),
			logging.Field("error", err))
		return nil, err
	}

	switch pathType {
	case fileio.PathTypeDirectory:
		logger.Debug("Path is a directory, using directory import")
		return FromDirectory(s.path).Load(logger)
	case fileio.PathTypeFile:
		logger.Debug("Path is a file, using file import")
		return FromFile(s.path).Load(logger)
	case fileio.PathTypeUnknown:
		return nil, fmt.Errorf("unknown path type for %s", s.path)
	default:
		return nil, fmt.Errorf("unknown path type for %s", s.path)
	}
}

// parseJSONEvents parses a JSON events array from a reader
func parseJSONEvents(r io.Reader, logger *logging.Logger) ([]models.Event, error) {
	logger.Debug("Parsing JSON events")

	var req BatchEventImportRequest
	decoder := json.NewDecoder(r)

	if err := decoder.Decode(&req); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("empty file or reader")
		}
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if len(req.Events) == 0 {
		return nil, fmt.Errorf("events array is empty")
	}

	logger.DebugWithFields("Parsed JSON structure",
		logging.Field("raw_event_count", len(req.Events)))

	// Validate events
	validEvents, invalidCount := validateEvents(req.Events, logger)
	if len(validEvents) == 0 {
		return nil, fmt.Errorf("all %d events failed validation", invalidCount)
	}

	if invalidCount > 0 {
		logger.WarnWithFields("Some events failed validation",
			logging.Field("valid_count", len(validEvents)),
			logging.Field("invalid_count", invalidCount))
	}

	// Apply enrichment strategies
	enricher := enrichment.Default()
	enricher.Enrich(validEvents, logger)

	logger.InfoWithFields("JSON parsing completed",
		logging.Field("valid_events", len(validEvents)),
		logging.Field("invalid_events", invalidCount))

	return validEvents, nil
}

// validateEvents validates event data and filters out invalid events
// Returns: (validEvents, invalidCount)
func validateEvents(events []models.Event, logger *logging.Logger) ([]models.Event, int) {
	validEvents := make([]models.Event, 0, len(events))
	invalidCount := 0

	for i, event := range events {
		// Validate required fields
		if event.ID == "" {
			logger.WarnWithFields("Skipping event with empty ID",
				logging.Field("index", i))
			invalidCount++
			continue
		}

		if event.Timestamp <= 0 {
			logger.WarnWithFields("Skipping event with invalid timestamp",
				logging.Field("event_id", event.ID),
				logging.Field("timestamp", event.Timestamp))
			invalidCount++
			continue
		}

		// Validate event type
		if event.Type == "" {
			logger.WarnWithFields("Skipping event with empty type",
				logging.Field("event_id", event.ID))
			invalidCount++
			continue
		}

		// Validate resource metadata
		if event.Resource.Kind == "" {
			logger.WarnWithFields("Skipping event with empty resource kind",
				logging.Field("event_id", event.ID))
			invalidCount++
			continue
		}

		if event.Resource.Name == "" {
			logger.WarnWithFields("Skipping event with empty resource name",
				logging.Field("event_id", event.ID),
				logging.Field("kind", event.Resource.Kind))
			invalidCount++
			continue
		}

		// Event is valid
		validEvents = append(validEvents, event)
	}

	return validEvents, invalidCount
}

// Legacy API - Deprecated but maintained for backward compatibility
// These functions will be removed in a future version.

// ParseJSONEvents parses a JSON events array from a reader.
//
// Deprecated: Use Import(FromReader(r)) instead.
func ParseJSONEvents(r io.Reader) ([]*models.Event, error) {
	logger := logging.GetLogger("importexport")
	events, err := parseJSONEvents(r, logger)
	if err != nil {
		return nil, err
	}

	// Convert to pointers for backward compatibility
	eventPtrs := make([]*models.Event, len(events))
	for i := range events {
		eventPtrs[i] = &events[i]
	}
	return eventPtrs, nil
}

// ImportJSONFile reads and parses a single JSON file containing an events array.
//
// Deprecated: Use Import(FromFile(filePath)) instead.
func ImportJSONFile(filePath string) ([]*models.Event, error) {
	logger := logging.GetLogger("importexport")
	events, err := FromFile(filePath).Load(logger)
	if err != nil {
		return nil, err
	}

	// Convert to pointers for backward compatibility
	eventPtrs := make([]*models.Event, len(events))
	for i := range events {
		eventPtrs[i] = &events[i]
	}
	return eventPtrs, nil
}

// ConvertEventsToValues converts a slice of event pointers to a slice of event values.
//
// Deprecated: The new API returns []models.Event directly. This function is no longer needed.
func ConvertEventsToValues(events []*models.Event) []models.Event {
	eventValues := make([]models.Event, len(events))
	for i, event := range events {
		eventValues[i] = *event
	}
	return eventValues
}

// ImportJSONFileAsValues reads and parses a JSON file and returns events as values.
//
// Deprecated: Use Import(FromFile(filePath)) instead.
func ImportJSONFileAsValues(filePath string) ([]models.Event, error) {
	logger := logging.GetLogger("importexport")
	return FromFile(filePath).Load(logger)
}

// ImportPathAsValues reads and parses events from a file or directory.
//
// Deprecated: Use Import(FromPath(path)) instead.
func ImportPathAsValues(path string) ([]models.Event, error) {
	logger := logging.GetLogger("importexport")
	return FromPath(path).Load(logger)
}
