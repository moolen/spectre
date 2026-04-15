# Strict Hour Boundaries - Usage Examples

## Basic Usage

### Writing Events

Events are automatically routed to the correct hour file:

```go
// Create storage
storage, err := storage.New("/data", 256*1024)
if err != nil {
    log.Fatal(err)
}
defer storage.Close()

// Write event - automatically goes to correct hour file
event := &models.Event{
    UID:       "event-123",
    Timestamp: time.Date(2024, 12, 13, 14, 30, 0, 0, time.UTC).UnixNano(),
    Type:      "CREATE",
    // ... other fields
}

if err := storage.WriteEvent(event); err != nil {
    log.Printf("Failed to write event: %v", err)
}
// Event written to: /data/2024-12-13-14.bin
```

### Querying with Index

```go
// Create query executor
executor := storage.NewQueryExecutor(storage, nil)

// Query uses file index automatically
result, err := executor.Execute(ctx, &models.QueryRequest{
    StartTimestamp: time.Date(2024, 12, 13, 14, 0, 0, 0, time.UTC).Unix(),
    EndTimestamp:   time.Date(2024, 12, 13, 15, 0, 0, 0, time.UTC).Unix(),
    Filters: models.QueryFilters{
        Kind: "Pod",
    },
})

// File index selects only 2024-12-13-14.bin instantly
// No metadata reads from other files
```

### File Index Operations

```go
// Get file index
fileIndex := storage.GetFileIndex()

// Check index status
fmt.Printf("Indexed files: %d\n", fileIndex.Count())

// Get files for specific time range
startNs := time.Date(2024, 12, 13, 10, 0, 0, 0, time.UTC).Unix() * 1e9
endNs := time.Date(2024, 12, 13, 12, 0, 0, 0, time.UTC).Unix() * 1e9
files := fileIndex.GetFilesByTimeRange(startNs, endNs)
fmt.Printf("Files in range: %v\n", files)

// Get file before time (for state snapshots)
queryStart := time.Date(2024, 12, 13, 15, 0, 0, 0, time.UTC).Unix() * 1e9
fileBeforeQuery := fileIndex.GetFileBeforeTime(queryStart)
fmt.Printf("State snapshot file: %s\n", fileBeforeQuery)

// Manually save index
if err := fileIndex.Save(); err != nil {
    log.Printf("Failed to save index: %v", err)
}

// Rebuild index from disk
if err := fileIndex.Rebuild("/data", storage.ExtractHourFromFilename); err != nil {
    log.Printf("Failed to rebuild index: %v", err)
}
```

## Periodic Maintenance

### Close Old Files

Keep only recent hour files open:

```go
// In main application loop
ticker := time.NewTicker(30 * time.Minute)
defer ticker.Stop()

for range ticker.C {
    // Close files older than 2 hours
    if err := storage.CloseOldHourFiles(2 * time.Hour); err != nil {
        log.Printf("Failed to close old files: %v", err)
    }
    
    // Save file index
    if err := storage.GetFileIndex().Save(); err != nil {
        log.Printf("Failed to save index: %v", err)
    }
}
```

### Monitor Index Health

```go
// Get storage stats
stats, err := storage.GetStorageStats()
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Data directory: %s\n", stats["dataDir"])
fmt.Printf("Total files: %d\n", stats["fileCount"])
fmt.Printf("Indexed files: %d\n", stats["indexedFiles"])
fmt.Printf("Open hour files: %d\n", stats["openHourFiles"])
fmt.Printf("Total size: %.2f MB\n", stats["totalSizeMB"])

// Check if index is healthy
fileCount := stats["fileCount"].(int)
indexedFiles := stats["indexedFiles"].(int)
if indexedFiles < fileCount {
    log.Printf("Warning: Some files not indexed (%d/%d)", indexedFiles, fileCount)
}
```

## Advanced Configuration

### Disable Strict Mode (Legacy Compatibility)

For backward compatibility with old files:

```go
storage, _ := storage.New("/data", 256*1024)
fileIndex := storage.GetFileIndex()

// Disable strict mode - uses actual event timestamps instead of hour boundaries
fileIndex.SetStrictHours(false)

// Now queries check actual TimestampMin/Max from index
// instead of relying on hour boundaries
```

### Custom Index Location

```go
// File index is automatically stored at: {dataDir}/.file_index.json
// To use custom location, modify the FileIndex initialization in storage.New()
```

## Error Handling

### Event Outside Hour Boundaries

```go
// This will fail validation
event := &models.Event{
    UID:       "event-456",
    Timestamp: time.Date(2024, 12, 13, 14, 30, 0, 0, time.UTC).UnixNano(),
}

// Try to write to different hour file
// Will return error: "event timestamp X is outside hour boundaries [Y, Z)"
```

### Index Corruption

```go
// On startup, if index is corrupted or missing
storage, _ := storage.New("/data", 256*1024)

// Logs will show:
// "Could not load file index (will create new): ..."
// "File index is empty, rebuilding..."
// "Rebuilt file index with N files"

// Index rebuilds automatically by scanning all .bin files
```

### Late-Arriving Events

```go
// Event from 2 hours ago
oldEvent := &models.Event{
    UID:       "late-event",
    Timestamp: time.Now().Add(-2 * time.Hour).UnixNano(),
}

// Will be written to the 2-hour-old file
// File will be opened if needed, or reused if still open
if err := storage.WriteEvent(oldEvent); err != nil {
    log.Printf("Failed to write late event: %v", err)
}

// Periodically close old files to avoid keeping too many open
storage.CloseOldHourFiles(1 * time.Hour)
```

## Performance Testing

### Benchmark Query Times

```go
// Before: With metadata reads
start := time.Now()
result, _ := executor.Execute(ctx, query)
fmt.Printf("Query time (old): %v\n", time.Since(start))
// Output: Query time (old): 8.5s

// After: With file index
start = time.Now()
result, _ = executor.Execute(ctx, query)
fmt.Printf("Query time (new): %v\n", time.Since(start))
// Output: Query time (new): 850ms

// Speedup: 10x
```

### Monitor Index Hit Rate

```go
// Query metrics include index usage
// Check traces/logs for:
// - "storage.used_index: true"
// - "File selection via index completed in Xms"
// - "storage.indexed_files: N"

// If not using index, check:
fileIndex := storage.GetFileIndex()
if fileIndex.Count() == 0 {
    log.Println("Index is empty, triggering rebuild")
    fileIndex.Rebuild("/data", storage.ExtractHourFromFilename)
}
```

## Migration Examples

### Clean Slate Migration

```bash
#!/bin/bash
# Stop application
systemctl stop spectre

# Backup old data (optional)
tar -czf /backup/spectre-data-$(date +%Y%m%d).tar.gz /data/*.bin

# Delete old files
rm -f /data/*.bin

# Start application
systemctl start spectre

# Verify new files are created with strict boundaries
tail -f /var/log/spectre.log | grep "Created new storage file"
```

### Gradual Migration

```go
// Step 1: Enable non-strict mode temporarily
storage, _ := storage.New("/data", 256*1024)
storage.GetFileIndex().SetStrictHours(false)

// Step 2: New events go to strict-boundary files
// Old files continue to work

// Step 3: After old files expire (based on retention policy)
// Step 4: Enable strict mode
storage.GetFileIndex().SetStrictHours(true)

// Step 5: Rebuild index
storage.GetFileIndex().Rebuild("/data", storage.ExtractHourFromFilename)
```

## Monitoring & Observability

### Prometheus Metrics (if integrated)

```
# File index health
spectre_file_index_count 142
spectre_file_index_hits_total 1523
spectre_file_index_misses_total 3

# Query performance
spectre_query_file_selection_duration_seconds{method="index"} 0.001
spectre_query_file_selection_duration_seconds{method="fallback"} 0.045

# File management
spectre_storage_open_hour_files 3
spectre_storage_total_files 142
```

### Logging

```
INFO  Storage initialized with directory: /data
INFO  Loaded file index with 142 files
INFO  Created new storage file: /data/2024-12-13-15.bin
DEBUG File selection via index completed in 1ms, selected 2 files
DEBUG Closing old hour file: 2024-12-13 13:00
```

### Distributed Tracing

Spans include attributes:
- `storage.used_index`: boolean
- `storage.indexed_files`: count
- `storage.files_to_query`: count
- `storage.total_files`: count
