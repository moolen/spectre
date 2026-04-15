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
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// BatchEventImportRequest represents a JSON request to import a batch of events.
type BatchEventImportRequest struct {
	Events []models.Event `json:"events"`
}

// ImportReport contains the results of an import operation.
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

// ImportSource represents the source of events to import.
type ImportSource interface {
	Load(logger *logging.Logger) ([]models.Event, error)
}

// ImportOptions configures the import behavior.
type ImportOptions struct {
	logger *logging.Logger
}

// ImportOption is a functional option for configuring imports.
type ImportOption func(*ImportOptions)

// WithLogger configures import operations to use the specified logger.
func WithLogger(logger *logging.Logger) ImportOption {
	return func(opts *ImportOptions) {
		opts.logger = logger
	}
}

// Import loads events from the specified source with optional configuration.
// This is the primary entry point for importing events.
func Import(source ImportSource, opts ...ImportOption) ([]models.Event, error) {
	var allEvents []models.Event
	if err := ImportInChunks(source, defaultImportChunkSize, func(chunk []models.Event) error {
		allEvents = append(allEvents, chunk...)
		return nil
	}, opts...); err != nil {
		return nil, err
	}

	return allEvents, nil
}
