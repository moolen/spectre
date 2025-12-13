---
title: Storage Design
description: Deep dive into Spectre's storage architecture and design decisions
keywords: [architecture, storage, design, blocks, hourly files]
---

# Storage Design

This document provides a comprehensive overview of Spectre's storage architecture, explaining the design philosophy, implementation details, and performance characteristics.

## Design Philosophy

Spectre's storage engine is designed with three primary goals:

1. **High Write Throughput:** Handle continuous streams of Kubernetes audit events with minimal latency
2. **Fast Query Access:** Execute filtered queries efficiently without scanning entire dataset
3. **Storage Efficiency:** Compress data effectively while maintaining read performance

The design draws inspiration from log-structured storage systems like Loki and VictoriaMetrics, adapted specifically for Kubernetes resource state tracking.

## File Organization

### Hourly File Strategy

Events are organized into **hourly files** with the naming convention:

```
YYYY-MM-DD-HH.bin
```

**Examples:**
- `2025-12-12-10.bin` - Events from 10:00-10:59
- `2025-12-12-11.bin` - Events from 11:00-11:59
- `2025-12-12-12.bin` - Events from 12:00-12:59

### Why Hourly Files?

| Benefit              | Description                                                              |
|----------------------|--------------------------------------------------------------------------|
| **Retention Granularity** | Delete old data by hour, not day or all-at-once                     |
| **Query Optimization**    | Skip entire files that fall outside query time range                |
| **Crash Recovery**        | Limit blast radius - only current hour affected if crash occurs     |
| **Parallelization**       | Future: Query multiple hourly files concurrently                    |
| **Manageability**         | Smaller files are easier to backup, move, or analyze               |

### Directory Structure

```
/data/
├── 2025-12-11-10.bin                    # Complete file (with footer)
├── 2025-12-11-11.bin                    # Complete file
├── 2025-12-11-12.bin                    # Incomplete (currently writing)
├── 2025-12-11-09.bin.incomplete.1733915200  # Backup from crash
└── 2025-12-10-15.bin.corrupted.1733828800   # Corrupted file backup
```

**File States:**
- **Complete:** Has valid header and footer, can be reopened for appending
- **Incomplete:** Missing footer (crash during write), renamed with `.incomplete.<timestamp>`
- **Corrupted:** Invalid header or structure, renamed with `.corrupted.<timestamp>`

### File Lifecycle

```
┌──────────────┐
│   Created    │ Write header (77 bytes)
└──────┬───────┘
       │
       v
┌──────────────┐
│   Writing    │ Buffer events → Finalize blocks → Write compressed data
└──────┬───────┘
       │
       v
┌──────────────┐
│    Closing   │ Finalize last block → Build indexes → Write index → Write footer
└──────┬───────┘
       │
       v
┌──────────────┐
│   Complete   │ Has footer, can be queried or reopened for appending
└──────┬───────┘
       │
       v
┌──────────────┐
│   Archived   │ Outside query window, ready for deletion
└──────────────┘
```

### Hour Rotation

When the clock advances to a new hour:

1. **Finalize current file:** Flush buffer, build indexes, write footer
2. **Extract state snapshots:** Capture final resource states
3. **Create new file:** Generate filename for new hour
4. **Carry over states:** Transfer state snapshots to new file
5. **Continue writing:** New events go to new file

**Code Path:**
```
storage.go:getOrCreateCurrentFile()
  → Check if currentHour has changed
  → Close previous file (extracts finalResourceStates)
  → Create new file for current hour
  → Transfer finalResourceStates to new file
```

## Block-Based Architecture

### Block Lifecycle

Events are buffered in memory until they reach the configured block size, then compressed and written to disk.

```
┌─────────────────────────────────────────────────────────────────┐
│                        Event Flow                                │
└─────────────────────────────────────────────────────────────────┘

Watcher Event
     │
     v
Storage.WriteEvent()
     │
     v
BlockStorageFile.WriteEvent()
     │
     v
EventBuffer.AddEvent()
     │
     │  ┌─────────────────────────────────────┐
     │  │ Buffer Events (JSON)                │
     │  │ - Track metadata (kinds, ns, etc)  │
     │  │ - Update Bloom filters             │
     │  │ - Monitor buffer size              │
     │  └─────────────────────────────────────┘
     │
     v
  Buffer Full?
     │
     ├─ No  ─> Continue buffering
     │
     └─ Yes ─> Finalize Block
                     │
                     v
               Encode Protobuf
                     │
                     v
               Compress (gzip)
                     │
                     v
               Write to Disk
                     │
                     v
               Store Metadata
                     │
                     v
               Create New Buffer
```

### EventBuffer Design

The `EventBuffer` accumulates events until the block size threshold is reached:

```go
type EventBuffer struct {
    events       [][]byte              // JSON-encoded events
    blockSize    int64                 // Target uncompressed size
    currentSize  int64                 // Current uncompressed size

    // Metadata tracking
    timestampMin int64
    timestampMax int64
    kindSet      map[string]bool
    namespaceSet map[string]bool
    groupSet     map[string]bool

    // Bloom filters (built incrementally)
    bloomKinds      *StandardBloomFilter
    bloomNamespaces *StandardBloomFilter
    bloomGroups     *StandardBloomFilter
}
```

**Key Behaviors:**
- **Incremental Metadata:** Kinds, namespaces, groups tracked as events arrive
- **Bloom Filter Building:** Filters updated with each event for space efficiency
- **Size Monitoring:** Checks if adding next event would exceed block size
- **First Event Exception:** Never full on first event (prevents zero-event blocks)

### Block Structure

Once finalized, a block contains:

```go
type Block struct {
    ID                 int32            // Sequential within file
    Offset             int64            // Byte offset in file
    Length             int64            // Compressed data length
    UncompressedLength int64            // Original size
    EventCount         int32            // Number of events
    TimestampMin       int64            // Time range for filtering
    TimestampMax       int64
    CompressedData     []byte           // gzip-compressed protobuf stream
    Metadata           *BlockMetadata   // For indexing
}
```

**Block Size Trade-offs:**

| Block Size    | Pros                                  | Cons                                |
|---------------|---------------------------------------|-------------------------------------|
| Small (1 MB)  | Fine-grained filtering, fast decompress | More blocks, larger index overhead |
| Medium (10 MB)| Balanced compression and granularity  | Moderate decompression latency      |
| Large (100 MB)| Fewer blocks, better compression      | Slow decompression, coarse filtering|

**Default:** 10 MB (configurable via `--segment-size` flag)

## Write Path

### Complete Write Flow

```go
// 1. Application calls WriteEvent
storage.WriteEvent(event)
    ↓
// 2. Get or create hourly file (rotates at hour boundary)
getOrCreateCurrentFile()
    ↓
// 3. Write to block storage file
blockStorageFile.WriteEvent(event)
    ↓
// 4. Serialize event to JSON
eventJSON := json.Marshal(event)
    ↓
// 5. Check if buffer is full
if currentBuffer.IsFull(len(eventJSON)) {
    finalizeBlock()     // Flush current buffer
    currentBuffer = NewEventBuffer(blockSize)
}
    ↓
// 6. Add to buffer
currentBuffer.AddEvent(eventJSON)
    ↓
// 7. Update metadata
- Add kind/namespace/group to sets
- Add to Bloom filters
- Update timestamp min/max
    ↓
// 8. Write buffered (returns immediately)
```

### Block Finalization

When buffer is full (triggered by next event):

```go
finalizeBlock()
    ↓
// 1. Create block from buffer
block := currentBuffer.Finalize(blockID, "gzip")
    ↓
// 2. Encode events as protobuf stream
protobufData := encodeProtobuf(events)
    ↓
// 3. Compress with gzip
compressedData := gzip.Compress(protobufData)
    ↓
// 4. Get current file offset
offset := file.Seek(0, SEEK_CUR)
    ↓
// 5. Write compressed data to disk
file.Write(compressedData)
    ↓
// 6. Store metadata for index
blockMetadataList.append(block.Metadata)
    ↓
// 7. Increment block ID
blockID++
```

**Performance Characteristics:**
- **Event buffering:** O(1) per event
- **Block finalization:** O(N) where N = events in block (protobuf encode + gzip compress)
- **Typical latency:** \<50ms for 10MB block on modern hardware

### File Closing

When hour changes or application shuts down:

```go
blockStorageFile.Close()
    ↓
// 1. Finalize last buffer (if non-empty)
if currentBuffer.EventCount > 0 {
    finalizeBlock()
}
    ↓
// 2. Build inverted indexes from block metadata
index := BuildInvertedIndexes(blockMetadataList)
    ↓
// 3. Extract final resource states
finalResourceStates := extractFinalResourceStates()
    ↓
// 4. Create index section
indexSection := IndexSection{
    BlockMetadata: blockMetadataList,
    InvertedIndexes: index,
    Statistics: stats,
    FinalResourceStates: finalResourceStates,
}
    ↓
// 5. Write index section (JSON)
indexOffset := file.CurrentOffset()
indexLength := WriteIndexSection(file, indexSection)
    ↓
// 6. Write footer
footer := FileFooter{
    IndexSectionOffset: indexOffset,
    IndexSectionLength: indexLength,
    MagicBytes: "RPKEND",
}
WriteFileFooter(file, footer)
    ↓
// 7. Close file handle
file.Close()
```

## State Snapshots

### Problem: Pre-Existing Resources

Consider this scenario:

```
Hour 10:00-10:59:  Deployment "nginx" created
Hour 11:00-11:59:  No events for "nginx"
Query [11:30-12:00]: Should "nginx" appear in results?
```

**Answer:** Yes! The Deployment still exists, even if no events occurred.

### Solution: Final Resource States

Each file stores the **final state** of every resource at file close time:

```go
type ResourceLastState struct {
    UID          string          // Resource UID
    EventType    string          // CREATE, UPDATE, or DELETE
    Timestamp    int64           // Last observed timestamp
    ResourceData json.RawMessage // Full resource object (null for DELETE)
}
```

**Map Key:** `group/version/kind/namespace/name`
**Example:** `apps/v1/Deployment/default/nginx`

### State Carryover

When creating a new hourly file:

```go
// Close previous file
previousFile.Close()
    ↓
// Extract its final states
carryoverStates := previousFile.finalResourceStates
    ↓
// Create new file
newFile := NewBlockStorageFile(path, timestamp, blockSize)
    ↓
// Transfer states to new file
newFile.finalResourceStates = carryoverStates
```

**Why This Works:**
- Resources that exist but have no events: Carried forward hour-to-hour
- Resources that are deleted: State shows `EventType = "DELETE"`
- Resources with new events: State updated during event processing

### Query Integration

When querying `[startTime, endTime]`:

1. **Identify files** that overlap the time range
2. **Include one file before** `startTime` (for state snapshots)
3. **Merge events** from files with state snapshots
4. **Generate synthetic "state-" events** for resources that exist but have no events in range

**Example:**
```
Query: [11:30, 12:30]
Files: 2025-12-12-10.bin (for states)
       2025-12-12-11.bin (events + states)
       2025-12-12-12.bin (events + states)
```

## File Restoration

### Reopening Complete Files

When starting the application, existing complete files can be reopened for appending:

```go
// 1. Check if file exists
if fileExists(path) {
    // 2. Read footer
    footer := ReadFileFooter(path)

    // 3. Verify magic bytes
    if footer.MagicBytes != "RPKEND" {
        // Incomplete file - rename and create new
        os.Rename(path, path + ".incomplete." + timestamp)
        return createNewFile(path)
    }

    // 4. Read index section
    index := ReadIndexSection(path, footer.IndexOffset, footer.IndexLength)

    // 5. Restore state
    blockMetadata := index.BlockMetadata
    finalResourceStates := index.FinalResourceStates
    nextBlockID := len(blockMetadata)

    // 6. Truncate at blocks end (remove old index + footer)
    file.Truncate(footer.IndexSectionOffset)

    // 7. Seek to end for appending
    file.Seek(footer.IndexSectionOffset, SEEK_SET)

    // 8. Continue writing new blocks
    return blockStorageFile
}
```

**Use Cases:**
- **Application restart:** Resume writing to current hour's file
- **Hot reload:** Reload configuration without losing buffered data
- **Testing:** Inject historical events into existing files

### Crash Recovery

#### Incomplete Files

If the application crashes before closing a file:

**Detection:**
- Footer is missing (can't read 324 bytes from end)
- Footer magic bytes != "RPKEND"

**Recovery:**
```go
timestamp := time.Now().Unix()
backupPath := fmt.Sprintf("%s.incomplete.%d", path, timestamp)
os.Rename(path, backupPath)

// Create new empty file
createNewFile(path)
```

**Result:**
- Original incomplete file preserved for debugging/recovery
- New empty file created for writing
- No data loss for previously closed files

#### Corrupted Files

If the header or structure is invalid:

**Detection:**
- Can't read 77-byte header
- Header magic bytes != "RPKBLOCK"
- Version is unsupported

**Recovery:**
```go
timestamp := time.Now().Unix()
backupPath := fmt.Sprintf("%s.corrupted.%d", path, timestamp)
os.Rename(path, backupPath)

// Create new empty file
createNewFile(path)
```

## Performance Characteristics

### Write Performance

| Operation            | Complexity | Typical Latency |
|----------------------|------------|-----------------|
| Event buffering      | O(1)       | \<1 µs           |
| JSON marshal         | O(N)       | ~10 µs          |
| Bloom filter update  | O(k)       | ~1 µs           |
| Block finalization   | O(N)       | ~50 ms (10 MB)  |
| Protobuf encode      | O(N)       | ~20 ms          |
| gzip compress        | O(N)       | ~100 ms         |
| Disk write           | O(1)       | ~10 ms          |
| Index build (close)  | O(N × M)   | ~500 ms         |

**Throughput:** 10,000+ events/second (typical Kubernetes cluster)

### Space Efficiency

For a typical hourly file with 60,000 events:

| Component             | Size        | Percentage |
|-----------------------|-------------|------------|
| Compressed Events     | 18 MB       | ~94%       |
| Block Metadata        | 800 KB      | ~4%        |
| Inverted Indexes      | 200 KB      | ~1%        |
| Bloom Filters         | 150 KB      | ~0.8%      |
| State Snapshots       | 100 KB      | ~0.5%      |
| Header + Footer       | 401 bytes   | \<0.001%    |
| **Total**             | **~19 MB**  | **100%**   |

**Compression Ratio:** 0.25 (75% reduction from uncompressed)

### Read Performance

| Operation         | Complexity | Typical Latency |
|-------------------|------------|-----------------|
| Header read       | O(1)       | \<1 ms           |
| Footer read       | O(1)       | \<1 ms           |
| Index read        | O(N)       | ~10 ms (2 MB)   |
| Index parse (JSON)| O(N)       | ~20 ms          |
| Block read        | O(1) seek  | ~5 ms           |
| Block decompress  | O(M)       | ~30 ms (10 MB)  |
| Protobuf decode   | O(M)       | ~20 ms          |

**Query Performance:** See [Query Execution](./query-execution.md) for details

## Design Decisions (Q&A)

### Q: Why 10MB default block size?

**A:** Balances three competing factors:

1. **Compression Ratio:** Larger blocks compress better (more context for gzip)
   - 1 MB: ~65% reduction
   - 10 MB: ~75% reduction
   - 100 MB: ~78% reduction

2. **Query Granularity:** Smaller blocks enable finer filtering
   - 1 MB block = ~80 events → better filtering precision
   - 10 MB block = ~800 events → balanced
   - 100 MB block = ~8000 events → coarse filtering

3. **Decompression Latency:** Smaller blocks decompress faster
   - 1 MB: ~3 ms
   - 10 MB: ~30 ms
   - 100 MB: ~300 ms

**10 MB provides good compression (75%) with acceptable latency (\<50ms).**

### Q: Why gzip over zstd?

**A:** Implementation maturity and compatibility:

- **gzip:**
  - Excellent Go library support (`klauspost/compress/gzip`)
  - Universal compatibility
  - Good compression ratio (75%)
  - Battle-tested in production

- **zstd** (planned for v2.0):
  - Slightly better compression (78%)
  - Faster compression (~2x)
  - Faster decompression (~1.5x)
  - Requires migration strategy for existing files

**Current choice: gzip for stability. Future: zstd as opt-in with migration path.**

### Q: Why hourly files instead of daily?

**A:** Operational flexibility:

| Hourly                        | Daily                           |
|-------------------------------|---------------------------------|
| Delete specific hours         | Delete entire days only         |
| Smaller files (~20 MB)        | Larger files (~500 MB)          |
| Hour-level query optimization | Day-level only                  |
| Fast rotation (low risk)      | Rotation once/day (higher risk) |
| Easier backup/restore         | Harder to manage                |

**Hourly provides finer control without excessive file count.**

### Q: Why JSON index instead of binary?

**A:** Developer experience and flexibility:

- **Pros:**
  - Human-readable (debugging, inspection)
  - Easy schema evolution (add fields without breaking)
  - Standard tooling (jq, JSON parsers)
  - Compact enough for typical indexes (\<2 MB)

- **Cons:**
  - Slightly larger than binary (10-20%)
  - Slightly slower to parse (~20ms vs ~5ms)

**Trade-off: Flexibility and debuggability over ~15ms latency.**

### Q: Why not use a database (SQLite, RocksDB)?

**A:** Specialized requirements and simplicity:

- **Append-only workload:** Blocks never modified after write
- **Compression at block level:** Databases compress at page level (less efficient)
- **Custom indexing:** Inverted indexes + Bloom filters tailored for our queries
- **No dependencies:** Single binary deployment
- **Portability:** Files can be copied, archived, analyzed offline

**Custom storage provides better compression and simpler deployment.**

## Future Enhancements

### Version 1.1 (Planned)

- **Automatic Retention Policies:** Delete files older than N days via configuration
- **Background Compaction:** Merge small blocks from low-traffic hours
- **Enhanced Metadata:** Track more dimensions (verbs, users, source IPs)

### Version 2.0 (Planned)

- **zstd Compression:** Opt-in faster compression with migration tool
- **Concurrent File Writing:** Parallel writes to multiple hourly files
- **Block-Level Encryption:** Encrypt sensitive events at rest
- **Multi-Tier Storage:** Hot (SSD), warm (HDD), cold (object storage)
- **Distributed Queries:** Query across multiple Spectre instances

### Beyond 2.0

- **Column-Oriented Blocks:** Store fields separately for better compression
- **Dictionary Learning:** Pre-build compression dictionaries for common patterns
- **Adaptive Block Sizing:** Tune block size based on event rate
- **Incremental Indexes:** Update indexes without rebuilding entire file

## Related Documentation

- [Block Format Reference](./block-format.md) - Binary file format specification
- [Indexing Strategy](./indexing-strategy.md) - Query optimization techniques
- [Compression](./compression.md) - Compression algorithms and performance
- [Query Execution](./query-execution.md) - Query pipeline and optimization
- [Storage Settings](../configuration/storage-settings.md) - Configuration guide

<!-- Source: internal/storage/block_storage.go, internal/storage/storage.go, internal/storage/README.md -->
