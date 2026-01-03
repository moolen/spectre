# Phase 3 & 4 Code Examples

This document provides code examples demonstrating the architectural improvements made in Phases 3 and 4.

## 1. Enrichment System

### Before: Tightly Coupled Enrichment
```go
// Old: Enrichment was embedded in the import logic
func enrichEventsWithInvolvedObjectUID(events []models.Event) {
    for i := range events {
        // Hard-coded enrichment logic...
    }
}
```

### After: Pluggable Enrichment
```go
// New: Enrichment is pluggable and extensible
import "github.com/moolen/spectre/internal/importexport/enrichment"

// Use default enrichment chain
enricher := enrichment.Default()
enricher.Enrich(events, logger)

// Or create custom chain
chain := enrichment.NewChain(
    enrichment.NewInvolvedObjectUIDEnricher(),
    // Add more enrichers here...
)
chain.Enrich(events, logger)
```

### Creating Custom Enrichers
```go
// Implement the Enricher interface
type MyCustomEnricher struct {}

func (e *MyCustomEnricher) Name() string {
    return "my-custom-enricher"
}

func (e *MyCustomEnricher) Enrich(events []models.Event, logger *logging.Logger) {
    for i := range events {
        // Custom enrichment logic
    }
}

// Use it
chain := enrichment.NewChain(
    enrichment.NewInvolvedObjectUIDEnricher(),
    &MyCustomEnricher{},
)
```

## 2. File I/O Operations

### Before: Scattered File Operations
```go
// Old: File operations mixed with business logic
file, err := os.Open(path)
if err != nil {
    return nil, fmt.Errorf("failed to open: %w", err)
}
defer file.Close()
```

### After: Centralized File I/O
```go
import "github.com/moolen/spectre/internal/importexport/fileio"

// FileReader with built-in validation
reader := fileio.NewFileReader(logger)
file, err := reader.ReadFile(path)
if err != nil {
    // Error message includes validation details
    return nil, err
}
defer file.Close()

// DirectoryWalker for recursive discovery
walker := fileio.NewDirectoryWalker(logger)
files, err := walker.WalkJSON(dirPath)
for _, file := range files {
    fmt.Printf("Found: %s (%d bytes)\n", file.FilePath, file.Size)
}

// Path type detection
pathType, err := fileio.DetectPathType(path)
switch pathType {
case fileio.PathTypeFile:
    // Handle file
case fileio.PathTypeDirectory:
    // Handle directory
}
```

## 3. Input Validation

### Before: No Validation
```go
// Old: Events were imported without validation
events, err := parseJSONEvents(reader)
// No check for invalid events
```

### After: Automatic Validation
```go
// New: Validation happens automatically in parseJSONEvents()
events, err := parseJSONEvents(reader, logger)
// Invalid events are filtered out with warnings

// Validation checks:
// - Event ID must not be empty
// - Timestamp must be positive
// - Event type must not be empty
// - Resource kind must not be empty
// - Resource name must not be empty

// Invalid events are logged:
// "Skipping event with empty ID"
// "Skipping event with invalid timestamp"
```

### Manual Validation
```go
// You can also call validateEvents directly
validEvents, invalidCount := validateEvents(events, logger)
if invalidCount > 0 {
    logger.Warn("Found %d invalid events", invalidCount)
}
```

## 4. Structured Logging

### Before: Minimal Logging
```go
// Old: Simple logging
logger.Info("Imported %d events from %s", len(events), path)
```

### After: Structured Logging with Decision Tracking
```go
// New: Structured logs with context
logger.InfoWithFields("Loading events from file",
    logging.Field("path", s.path))

logger.InfoWithFields("Successfully loaded events from file",
    logging.Field("path", s.path),
    logging.Field("event_count", len(events)))

logger.InfoWithFields("Event enrichment completed",
    logging.Field("enricher", e.Name()),
    logging.Field("enriched", enrichedCount),
    logging.Field("skipped", skippedCount),
    logging.Field("errors", errorCount))

logger.InfoWithFields("Successfully loaded events from directory",
    logging.Field("path", s.path),
    logging.Field("event_count", len(allEvents)),
    logging.Field("files_processed", successCount),
    logging.Field("files_failed", failureCount))
```

## 5. Complete Import Examples

### Basic File Import
```go
import "github.com/moolen/spectre/internal/importexport"

// Simple import
events, err := importexport.Import(
    importexport.FromFile("/path/to/events.json"),
)

// Import with custom logger
logger := logging.New("my-importer")
events, err := importexport.Import(
    importexport.FromFile("/path/to/events.json"),
    importexport.WithLogger(logger),
)
```

### Directory Import
```go
// Import all JSON files from directory
events, err := importexport.Import(
    importexport.FromDirectory("/path/to/events"),
)

// Logs will show:
// - Files discovered
// - Files processed successfully
// - Files that failed (continues processing others)
// - Total event count
```

### Auto-Detection Import
```go
// Automatically detect if path is file or directory
events, err := importexport.Import(
    importexport.FromPath("/path/to/file_or_directory"),
)
```

### Reader Import
```go
import "strings"

data := `{"events": [...]}`
reader := strings.NewReader(data)

events, err := importexport.Import(
    importexport.FromReader(reader),
)
```

## 6. Performance Benchmarks

### Running Benchmarks
```bash
# Run all benchmarks
go test ./internal/importexport -bench=. -benchmem

# Run specific benchmark
go test ./internal/importexport -bench=BenchmarkParseJSONEvents/events_1000 -benchmem

# Compare before/after
go test ./internal/importexport -bench=BenchmarkImportFromFile -benchmem > after.txt
git checkout main
go test ./internal/importexport -bench=BenchmarkImportFromFile -benchmem > before.txt
benchcmp before.txt after.txt
```

### Example Benchmark Output
```
BenchmarkParseJSONEvents/events_100-16     10000    12345 ns/op    5678 B/op    123 allocs/op
BenchmarkImportFromFile/events_100-16       5000    23456 ns/op    8901 B/op    234 allocs/op
BenchmarkImportFromDirectory/files_10-16    1000   123456 ns/op   45678 B/op   1234 allocs/op
```

## 7. Error Handling Examples

### File I/O Errors
```go
// Empty path
_, err := reader.ReadFile("")
// Error: "file path cannot be empty"

// Non-existent file
_, err := reader.ReadFile("/nonexistent/file.json")
// Error: "file does not exist: /nonexistent/file.json"

// Directory instead of file
_, err := reader.ReadFile("/some/directory")
// Error: "path is a directory, not a file: /some/directory"

// Empty directory
_, err := walker.WalkJSON("/empty/directory")
// Error: "no JSON files found in directory: /empty/directory"
```

### Validation Errors
```go
// All events invalid
data := `{"events": [{"id": "", "timestamp": 0, ...}]}`
_, err := Import(FromReader(strings.NewReader(data)))
// Error: "all 1 events failed validation"

// Some events invalid
data := `{"events": [
    {"id": "valid", "timestamp": 123, ...},
    {"id": "", "timestamp": 456, ...}
]}`
events, err := Import(FromReader(strings.NewReader(data)))
// Warning logged: "Some events failed validation | valid_count=1 invalid_count=1"
// Returns 1 valid event
```

## 8. Testing Examples

### Unit Test with Enrichment
```go
func TestCustomEnrichment(t *testing.T) {
    logger := logging.GetLogger("test")
    enricher := enrichment.NewInvolvedObjectUIDEnricher()

    events := []models.Event{
        {
            ID: "test-event",
            Resource: models.ResourceMetadata{Kind: "Event"},
            Data: json.RawMessage(`{"involvedObject": {"uid": "test-uid"}}`),
        },
    }

    enricher.Enrich(events, logger)

    if events[0].Resource.InvolvedObjectUID != "test-uid" {
        t.Errorf("Expected uid 'test-uid', got %q", events[0].Resource.InvolvedObjectUID)
    }
}
```

### Integration Test with Validation
```go
func TestImportWithInvalidEvents(t *testing.T) {
    tmpDir := t.TempDir()
    testFile := filepath.Join(tmpDir, "test.json")

    // Create file with mixed valid/invalid events
    testData := `{
        "events": [
            {"id": "valid", "timestamp": 123, "type": "CREATE",
             "resource": {"kind": "Pod", "name": "test"}},
            {"id": "", "timestamp": 456, "type": "UPDATE",
             "resource": {"kind": "Deployment", "name": "test"}}
        ]
    }`
    os.WriteFile(testFile, []byte(testData), 0644)

    events, err := Import(FromFile(testFile))
    if err != nil {
        t.Fatalf("Import failed: %v", err)
    }

    // Should only get the valid event
    if len(events) != 1 {
        t.Errorf("Expected 1 valid event, got %d", len(events))
    }
    if events[0].ID != "valid" {
        t.Errorf("Expected valid event, got %q", events[0].ID)
    }
}
```

## 9. Backward Compatibility

All legacy APIs continue to work:

```go
// Legacy API (still works)
events, err := importexport.ParseJSONEvents(reader)
events, err := importexport.ImportJSONFile(filePath)
events, err := importexport.ImportJSONFileAsValues(filePath)
values := importexport.ConvertEventsToValues(eventPointers)

// New API (preferred)
events, err := importexport.Import(importexport.FromFile(filePath))
events, err := importexport.Import(importexport.FromReader(reader))
```

The new implementation adds:
- Validation (with warnings for invalid events)
- Structured logging
- Better error messages
- But maintains same behavior for valid inputs
