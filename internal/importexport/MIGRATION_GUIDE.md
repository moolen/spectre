# Import/Export Package Migration Guide

## Overview

The `internal/importexport` package has been refactored to provide a cleaner, more intuitive API with better error handling and observability. This guide helps you migrate from the old API to the new one.

## What Changed

### Phase 1: Quick Wins
1. **Fixed Silent Error Handling**: File close errors are now properly logged instead of being silently ignored
2. **Removed Dead Code**: Unused `ProgressCallback` type has been removed
3. **Added Package Documentation**: Comprehensive documentation with usage examples
4. **Improved Comments**: Misleading comments have been corrected

### Phase 2: API Cleanup
1. **Standardized Event Types**: New API returns `[]models.Event` directly (no pointer conversion needed)
2. **Consolidated Functions**: Old 5-function API consolidated into single `Import()` function with source types
3. **Removed Unnecessary Conversions**: `ConvertEventsToValues()` is deprecated (no longer needed)
4. **Better API Design**: Strategy pattern with `ImportSource` interface for extensibility

## Migration Path

### Old API (Deprecated)

```go
// Import from file - returns pointers
events, err := importexport.ImportJSONFile("/path/to/events.json")
if err != nil {
    return err
}

// Convert to values for pipeline
eventValues := importexport.ConvertEventsToValues(events)
pipeline.ProcessBatch(ctx, eventValues)
```

### New API (Recommended)

```go
// Import from file - returns values directly
events, err := importexport.Import(importexport.FromFile("/path/to/events.json"))
if err != nil {
    return err
}

// Use directly with pipeline - no conversion needed
pipeline.ProcessBatch(ctx, events)
```

## API Comparison

### Single File Import

**Old:**
```go
events, err := importexport.ImportJSONFile(filePath)
eventValues := importexport.ConvertEventsToValues(events)
```

**New:**
```go
events, err := importexport.Import(importexport.FromFile(filePath))
```

### Directory Import

**Old:**
```go
events, err := importexport.ImportPathAsValues(dirPath)
```

**New:**
```go
events, err := importexport.Import(importexport.FromDirectory(dirPath))
```

### Reader Import

**Old:**
```go
events, err := importexport.ParseJSONEvents(reader)
eventValues := importexport.ConvertEventsToValues(events)
```

**New:**
```go
events, err := importexport.Import(importexport.FromReader(reader))
```

### Auto-detect Path Type

**Old:**
```go
events, err := importexport.ImportPathAsValues(path)
```

**New:**
```go
events, err := importexport.Import(importexport.FromPath(path))
```

### With Custom Logger

**Old:**
```go
// Not possible - no logging support
events, err := importexport.ImportJSONFile(filePath)
```

**New:**
```go
logger := logging.GetLogger("my-importer")
events, err := importexport.Import(
    importexport.FromFile(filePath),
    importexport.WithLogger(logger),
)
```

## Backward Compatibility

The old API is **deprecated but still functional**. All old functions are marked with `// Deprecated:` comments and will be removed in a future version.

To maintain backward compatibility during migration:

1. Old functions continue to work exactly as before
2. They internally use the new API with default logger
3. They perform pointer-to-value conversion automatically
4. No code changes are required immediately

## Benefits of New API

### 1. No Unnecessary Memory Copying
**Old:** Required converting `[]*models.Event` to `[]models.Event`
**New:** Returns `[]models.Event` directly

### 2. Better Error Handling
**Old:** File close errors silently ignored
**New:** File close errors logged with context

### 3. Clearer Intent
**Old:** 5 functions with overlapping responsibilities
**New:** Single `Import()` function with explicit sources

### 4. Extensible Design
**Old:** Adding new import sources required new top-level functions
**New:** Implement `ImportSource` interface

### 5. Observability
**Old:** No logging or visibility into import operations
**New:** Structured logging with configurable logger

## Migration Timeline

### Immediate (Backward Compatible)
- All old functions continue to work
- Start using new API for new code
- Update examples and documentation

### Short Term (Next Release)
- Add deprecation warnings to old functions
- Update all internal callers to new API
- Document breaking changes

### Long Term (Future Release)
- Remove deprecated functions
- Remove backward compatibility code
- Clean up internal APIs

## Examples

### Example 1: Simple File Import

```go
// Before
func loadTestFixture(path string) ([]models.Event, error) {
    events, err := importexport.ImportJSONFile(path)
    if err != nil {
        return nil, err
    }
    return importexport.ConvertEventsToValues(events), nil
}

// After
func loadTestFixture(path string) ([]models.Event, error) {
    return importexport.Import(importexport.FromFile(path))
}
```

### Example 2: Batch Processing

```go
// Before
func processImportedEvents(path string) error {
    events, err := importexport.ImportPathAsValues(path)
    if err != nil {
        return fmt.Errorf("import failed: %w", err)
    }

    return pipeline.ProcessBatch(ctx, events)
}

// After
func processImportedEvents(path string) error {
    logger := logging.GetLogger("processor")
    events, err := importexport.Import(
        importexport.FromPath(path),
        importexport.WithLogger(logger),
    )
    if err != nil {
        return fmt.Errorf("import failed: %w", err)
    }

    return pipeline.ProcessBatch(ctx, events)
}
```

### Example 3: Custom Import Source

```go
// New capability - not possible with old API
type httpSource struct {
    url string
}

func (s *httpSource) Load(logger *logging.Logger) ([]models.Event, error) {
    resp, err := http.Get(s.url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    return importexport.FromReader(resp.Body).Load(logger)
}

// Usage
events, err := importexport.Import(&httpSource{url: "https://..."})
```

## Testing

All existing tests pass with the new implementation. The test suite includes:

- Backward compatibility tests for old API
- New API functionality tests
- Edge case handling
- Error scenarios

Run tests:
```bash
go test -v ./internal/importexport/...
```

## Questions?

If you encounter issues during migration or have questions about the new API, please:

1. Check this migration guide
2. Review the package documentation (`go doc importexport`)
3. Look at example code in tests
4. File an issue if something is unclear

## Summary

The new API provides:
- ✅ Better error handling with proper logging
- ✅ Cleaner, more intuitive interface
- ✅ No unnecessary memory copying
- ✅ Extensible design for future features
- ✅ Full backward compatibility during migration
- ✅ Comprehensive documentation

Migrate at your own pace - the old API works but is deprecated.
