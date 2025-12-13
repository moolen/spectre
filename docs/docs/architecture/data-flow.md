---
title: Data Flow
description: End-to-end data flow through Spectre's write and read paths
keywords: [architecture, data flow, write path, read path, pipeline]
---

# Data Flow

This document traces the complete journey of data through Spectre - from Kubernetes events arriving at the watcher to query results returned to the user.

## System Overview

Spectre processes data through two independent paths:

1. **Write Path**: Kubernetes events → Watcher → Storage
2. **Read Path**: API request → Query engine → Response

These paths are designed to minimize interference - writes happen continuously in the background while queries execute concurrently without blocking writes.

## Write Path: Events to Storage

### Complete Write Flow

```
┌────────────────────────────────────────────────────────────────────┐
│                         Write Path                                  │
└────────────────────────────────────────────────────────────────────┘

Kubernetes API Server
     │ (Resource ADD/UPDATE/DELETE)
     v
┌────────────────────────────────────────────────────────────────────┐
│ 1. Watcher Component (internal/watcher/)                           │
├────────────────────────────────────────────────────────────────────┤
│   ResourceEventHandler.OnAdd/OnUpdate/OnDelete                     │
│   └─> Receives: Kubernetes runtime.Object                          │
│   └─> Converts: Object → ResourceEvent struct                      │
│        - ExtractMetadata(name, namespace, kind, apiVersion)        │
│        - Capture timestamp, operation type, UID                    │
│        - Marshal full object to JSON                               │
└────────────────────────────────────────────────────────────────────┘
     │ ResourceEvent (with managedFields ~10KB)
     v
┌────────────────────────────────────────────────────────────────────┐
│ 2. Pruning (internal/watcher/pruner.go)                            │
├────────────────────────────────────────────────────────────────────┤
│   Remove metadata.managedFields                                    │
│   └─> Typical size reduction: 80-90%                               │
│   └─> Example: 10KB → 1-2KB                                        │
└────────────────────────────────────────────────────────────────────┘
     │ Pruned ResourceEvent (~1-2KB)
     v
┌────────────────────────────────────────────────────────────────────┐
│ 3. Validation (internal/watcher/validator.go)                      │
├────────────────────────────────────────────────────────────────────┤
│   Check required fields exist:                                     │
│   - UID, timestamp, kind, namespace, name                          │
│   └─> Invalid events: Logged and discarded                         │
└────────────────────────────────────────────────────────────────────┘
     │ Valid ResourceEvent
     v
┌────────────────────────────────────────────────────────────────────┐
│ 4. Event Queue (internal/watcher/event_queue.go)                   │
├────────────────────────────────────────────────────────────────────┤
│   Concurrent buffering: Channel-based queue                        │
│   └─> Buffer size: 10000 events (configurable)                     │
│   └─> Backpressure: Blocks watcher if queue full                   │
│   └─> Batch drain: Worker goroutine processes events               │
└────────────────────────────────────────────────────────────────────┘
     │ Batched events (up to 100 at a time)
     v
┌────────────────────────────────────────────────────────────────────┐
│ 5. Storage Layer (internal/storage/)                               │
├────────────────────────────────────────────────────────────────────┤
│ storage.WriteEvent(event)                                          │
│   │                                                                 │
│   ├─> Get or Create Hourly File                                    │
│   │   - Check current hour: time.Now().Truncate(time.Hour)         │
│   │   - If hour changed: Close previous file, create new           │
│   │   - Carryover finalResourceStates to new file                  │
│   │                                                                 │
│   ├─> blockStorageFile.WriteEvent(event)                           │
│   │   │                                                             │
│   │   ├─> Marshal event to JSON                                    │
│   │   │   └─> json.Marshal(event) → []byte                         │
│   │   │                                                             │
│   │   ├─> Check buffer capacity                                    │
│   │   │   └─> if currentBuffer.IsFull(len(eventJSON)):             │
│   │   │       - Finalize current block (compress + write)          │
│   │   │       - Create new EventBuffer                             │
│   │   │                                                             │
│   │   ├─> Add to EventBuffer                                       │
│   │   │   - Append event JSON to buffer                            │
│   │   │   - Update metadata sets (kinds, namespaces, groups)       │
│   │   │   - Update Bloom filters                                   │
│   │   │   - Track timestamp min/max                                │
│   │   │                                                             │
│   │   └─> Update Final Resource States                             │
│   │       - Map key: group/version/kind/namespace/name             │
│   │       - Store: UID, EventType, Timestamp, ResourceData         │
│   │                                                                 │
│   └─> Return (non-blocking)                                        │
└────────────────────────────────────────────────────────────────────┘
     │ Event buffered in memory
     v
┌────────────────────────────────────────────────────────────────────┐
│ 6. Block Finalization (when buffer full)                           │
├────────────────────────────────────────────────────────────────────┤
│ currentBuffer.Finalize(blockID, "gzip")                            │
│   │                                                                 │
│   ├─> Encode events as Protobuf stream                             │
│   │   - Length-prefixed messages                                   │
│   │   - Each event: [length(4B)][protobuf_data]                    │
│   │                                                                 │
│   ├─> Compress with gzip                                           │
│   │   - Algorithm: klauspost/compress/gzip                         │
│   │   - Level: DefaultCompression (6)                              │
│   │   - Typical ratio: 75% reduction                               │
│   │                                                                 │
│   ├─> Create Block structure                                       │
│   │   - ID: sequential block number                                │
│   │   - Offset: current file position                              │
│   │   - Length: compressed data size                               │
│   │   - UncompressedLength: original size                          │
│   │   - EventCount: number of events                               │
│   │   - TimestampMin/Max: time range                               │
│   │   - Metadata: kinds, namespaces, groups, Bloom filters         │
│   │                                                                 │
│   ├─> Write compressed data to disk                                │
│   │   - Append to hourly file                                      │
│   │   - fsync() for durability (optional)                          │
│   │                                                                 │
│   └─> Store block metadata for index                               │
│       - Append to blockMetadataList (in-memory)                    │
└────────────────────────────────────────────────────────────────────┘
     │ Block written to disk
     v
┌────────────────────────────────────────────────────────────────────┐
│ 7. File Closing (hour boundary or shutdown)                        │
├────────────────────────────────────────────────────────────────────┤
│ blockStorageFile.Close()                                           │
│   │                                                                 │
│   ├─> Finalize last buffer (if non-empty)                          │
│   │                                                                 │
│   ├─> Build Inverted Indexes                                       │
│   │   - KindToBlocks: "Pod" → [0, 2, 5, ...]                       │
│   │   - NamespaceToBlocks: "default" → [0, 1, 3, ...]              │
│   │   - GroupToBlocks: "apps" → [1, 2, 4, ...]                     │
│   │                                                                 │
│   ├─> Create IndexSection                                          │
│   │   - BlockMetadata: Array of block metadata                     │
│   │   - InvertedIndexes: Kind/namespace/group maps                 │
│   │   - Statistics: Event counts, compression ratios               │
│   │   - FinalResourceStates: Last state of each resource           │
│   │                                                                 │
│   ├─> Write index section (JSON)                                   │
│   │   - Record indexOffset = currentFilePosition                   │
│   │   - Write JSON-encoded IndexSection                            │
│   │   - Record indexLength = bytes written                         │
│   │                                                                 │
│   ├─> Write File Footer (324 bytes)                                │
│   │   - IndexSectionOffset: int64 (8 bytes)                        │
│   │   - IndexSectionLength: int32 (4 bytes)                        │
│   │   - Checksum: MD5 hash (256 bytes)                             │
│   │   - Reserved: padding (16 bytes)                               │
│   │   - MagicBytes: "RPKEND" (8 bytes)                             │
│   │                                                                 │
│   └─> Close file handle                                            │
└────────────────────────────────────────────────────────────────────┘
     │ File sealed (immutable)
     v
  Ready for queries
```

### Write Path Timing

For a typical event processing cycle:

| Stage | Time | Notes |
|-------|------|-------|
| Kubernetes API → Watcher | \<1 ms | Watch stream notification |
| Event conversion | ~10 µs | Object → ResourceEvent struct |
| Pruning | ~5 µs | Remove managedFields |
| Validation | ~1 µs | Check required fields |
| Queue buffering | \<1 µs | Channel send |
| Queue drain | ~100 µs | Batch processing |
| JSON marshal | ~10 µs | Event → JSON bytes |
| Buffer accumulation | ~1 µs | Append + metadata update |
| **Total (per event)** | **~130 µs** | **Sustained: 7,500 events/sec** |

**Block finalization (periodic)**:

| Stage | Time | Notes |
|-------|------|-------|
| Protobuf encode | ~20 ms | 10 MB uncompressed |
| gzip compression | ~100 ms | 10 MB → 2.5 MB |
| Disk write | ~10 ms | Append to file |
| Metadata tracking | ~1 ms | Update indexes |
| **Total** | **~130 ms** | **Every ~800 events** |

**File closing (hourly)**:

| Stage | Time | Notes |
|-------|------|-------|
| Finalize last block | ~130 ms | Same as block finalization |
| Build inverted indexes | ~50 ms | Process ~300 blocks |
| Encode index JSON | ~20 ms | Serialize index |
| Write index | ~10 ms | Write to disk |
| Write footer | \<1 ms | 324 bytes |
| **Total** | **~210 ms** | **Once per hour** |

### Concurrency Model (Write Path)

```
┌─────────────────────────────────────────────────────────────────┐
│                         Goroutines                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Watcher Goroutines (one per resource type):                   │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │ Pod Watcher  │  │ Deploy Watch │  │ Service Watch│          │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘          │
│         └─────────────────┼─────────────────┘                  │
│                           │ Events                              │
│                           v                                     │
│                   ┌───────────────┐                             │
│                   │  Event Queue  │ (buffered channel)          │
│                   │  cap=10000    │                             │
│                   └───────┬───────┘                             │
│                           │                                     │
│                           v                                     │
│                   ┌───────────────┐                             │
│                   │ Queue Worker  │ (single goroutine)          │
│                   │ Drains queue  │                             │
│                   └───────┬───────┘                             │
│                           │                                     │
│                           v                                     │
│                   ┌───────────────┐                             │
│                   │ Storage Writer│ (synchronized)              │
│                   │ Single writer │                             │
│                   └───────────────┘                             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Key Points**:
- **Multiple watchers**: One goroutine per resource type (parallelism)
- **Single queue**: All watchers feed into one event queue (serialization)
- **Single writer**: Only one goroutine writes to storage (no locking)
- **Buffered channel**: Decouples watchers from storage (backpressure tolerance)

### Error Handling (Write Path)

| Error Type | Handling | Impact |
|------------|----------|--------|
| **Invalid event** | Log warning, discard event | Single event lost |
| **Queue full** | Block watcher until space available | Watcher backpressure |
| **Disk full** | Log error, return error to caller | Stops all writes |
| **Compression error** | Log error, skip block finalization | Block data lost |
| **File write error** | Retry 3 times with exponential backoff | Potential data loss if persistent |
| **Index build error** | Log error, file marked incomplete | File can't be queried |
| **Hour rotation error** | Log error, continue with same file | No new file created |

## Read Path: Query to Results

### Complete Read Flow

```
┌────────────────────────────────────────────────────────────────────┐
│                          Read Path                                  │
└────────────────────────────────────────────────────────────────────┘

HTTP Client
     │ GET /api/search?start=X&end=Y&kind=Pod&namespace=default
     v
┌────────────────────────────────────────────────────────────────────┐
│ 1. API Server (internal/api/)                                      │
├────────────────────────────────────────────────────────────────────┤
│ search_handler.go:ServeHTTP()                                      │
│   │                                                                 │
│   ├─> Parse query parameters                                       │
│   │   - start (required): Unix timestamp (seconds or ms)           │
│   │   - end (required): Unix timestamp                             │
│   │   - kind (optional): e.g., "Pod", "Deployment"                 │
│   │   - namespace (optional): e.g., "default", "kube-system"       │
│   │   - group (optional): e.g., "apps", ""                         │
│   │   - version (optional): e.g., "v1"                             │
│   │                                                                 │
│   ├─> Validate parameters                                          │
│   │   - start < end (time range valid)                             │
│   │   - Range not too large (max 30 days)                          │
│   │   - Timestamps in valid format                                 │
│   │                                                                 │
│   └─> Create Filter struct                                         │
│       - TimeRange: [start, end]                                    │
│       - ResourceFilters: {kind, namespace, group, version}         │
└────────────────────────────────────────────────────────────────────┘
     │ Filter{start, end, kind, namespace, group}
     v
┌────────────────────────────────────────────────────────────────────┐
│ 2. Query Engine (internal/storage/query.go)                        │
├────────────────────────────────────────────────────────────────────┤
│ storage.Query(filter)                                              │
│   │                                                                 │
│   ├─> Select Files by Time Window                                  │
│   │   - Hourly files: YYYY-MM-DD-HH.bin                            │
│   │   - Example: [10:00-14:00] → [10.bin, 11.bin, 12.bin, 13.bin] │
│   │   - Include one file before start (for state snapshots)        │
│   │                                                                 │
│   └─> For each file (sequential):                                  │
│       query_file.go:queryFile(path, filter)                        │
└────────────────────────────────────────────────────────────────────┘
     │ File paths: [file1.bin, file2.bin, ...]
     v
┌────────────────────────────────────────────────────────────────────┐
│ 3. Per-File Query (internal/storage/query_file.go)                 │
├────────────────────────────────────────────────────────────────────┤
│ For each file:                                                     │
│   │                                                                 │
│   ├─> Read File Header (77 bytes)                                  │
│   │   - Validate magic bytes: "RPKBLOCK"                           │
│   │   - Check format version                                       │
│   │   - Read compression algorithm                                 │
│   │                                                                 │
│   ├─> Read File Footer (324 bytes from end)                        │
│   │   - Validate magic bytes: "RPKEND"                             │
│   │   - Extract indexSectionOffset                                 │
│   │   - Extract indexSectionLength                                 │
│   │                                                                 │
│   ├─> Read Index Section                                           │
│   │   - Seek to indexSectionOffset                                 │
│   │   - Read indexSectionLength bytes                              │
│   │   - Parse JSON → IndexSection struct                           │
│   │   - Load: BlockMetadata, InvertedIndexes, FinalResourceStates  │
│   │                                                                 │
│   ├─> Filter Blocks (by inverted indexes)                          │
│   │   - If filter.kind specified:                                  │
│   │     candidates = InvertedIndexes.KindToBlocks[filter.kind]     │
│   │   - If filter.namespace specified:                             │
│   │     candidates ∩= InvertedIndexes.NamespaceToBlocks[ns]        │
│   │   - If filter.group specified:                                 │
│   │     candidates ∩= InvertedIndexes.GroupToBlocks[group]         │
│   │   - Result: Subset of block IDs to decompress                  │
│   │                                                                 │
│   ├─> Binary Search Timestamp Index                                │
│   │   - BlockMetadata sorted by timestampMin                       │
│   │   - Find first block: block.timestampMax >= filter.start       │
│   │   - Find last block: block.timestampMin <= filter.end          │
│   │   - Intersect with candidates from inverted indexes            │
│   │                                                                 │
│   ├─> For each candidate block:                                    │
│   │   │                                                             │
│   │   ├─> Read Block Data                                          │
│   │   │   - Seek to block.offset                                   │
│   │   │   - Read block.length bytes                                │
│   │   │                                                             │
│   │   ├─> Decompress Block                                         │
│   │   │   - gzip.Decompress(compressedData)                        │
│   │   │   - Result: Protobuf-encoded event stream                  │
│   │   │                                                             │
│   │   ├─> Decode Protobuf Events                                   │
│   │   │   - Read length-prefixed messages                          │
│   │   │   - Unmarshal each: protobuf → ResourceEvent               │
│   │   │                                                             │
│   │   ├─> Filter Events (exact match)                              │
│   │   │   - Check timestamp in [filter.start, filter.end]          │
│   │   │   - Check kind == filter.kind (if specified)               │
│   │   │   - Check namespace == filter.namespace (if specified)     │
│   │   │   - Check group == filter.group (if specified)             │
│   │   │                                                             │
│   │   └─> Collect matching events                                  │
│   │       - Append to results array                                │
│   │                                                                 │
│   └─> Include Final Resource States (if in time range)             │
│       - For resources with no events in window but state exists    │
│       - Generate synthetic "state-" events                         │
└────────────────────────────────────────────────────────────────────┘
     │ Events from all files
     v
┌────────────────────────────────────────────────────────────────────┐
│ 4. Result Aggregation (internal/storage/query.go)                  │
├────────────────────────────────────────────────────────────────────┤
│   Combine results from all files                                  │
│   │                                                                 │
│   ├─> Merge event arrays                                           │
│   │   - Concatenate results from each file                         │
│   │                                                                 │
│   ├─> Sort by timestamp (ascending)                                │
│   │   - Sort all events chronologically                            │
│   │                                                                 │
│   ├─> Apply result limit (if specified)                            │
│   │   - Truncate to max results (e.g., 10000)                      │
│   │                                                                 │
│   └─> Collect metrics                                              │
│       - Total events returned                                      │
│       - Files searched                                             │
│       - Blocks scanned (decompressed)                              │
│       - Blocks skipped (via indexes)                               │
│       - Execution time (milliseconds)                              │
└────────────────────────────────────────────────────────────────────┘
     │ Sorted events + metrics
     v
┌────────────────────────────────────────────────────────────────────┐
│ 5. Response Formatting (internal/api/)                             │
├────────────────────────────────────────────────────────────────────┤
│ Format JSON response:                                              │
│   {                                                                 │
│     "events": [...],           // Array of ResourceEvent objects   │
│     "count": 150,               // Total events returned           │
│     "executionTimeMs": 45,      // Query duration                  │
│     "filesSearched": 4,         // Hourly files accessed           │
│     "segmentsScanned": 12,      // Blocks decompressed             │
│     "segmentsSkipped": 88       // Blocks skipped (indexes)        │
│   }                                                                 │
└────────────────────────────────────────────────────────────────────┘
     │ JSON response
     v
HTTP Client receives results
```

### Read Path Timing

**Single-hour query with filters** (best case):

| Stage | Time | Notes |
|-------|------|-------|
| Parameter parsing | \<1 ms | Parse query string |
| File selection | \<1 ms | Calculate hourly file names |
| Read header/footer | ~1 ms | 77 + 324 bytes |
| Read index section | ~10 ms | ~2 MB JSON, parse |
| Binary search timestamp | \<1 ms | O(log N) on ~300 blocks |
| Inverted index intersection | ~1 ms | Set operations |
| **Block filtering result** | **2-10 blocks** | **90-98% skipped** |
| Read block data | ~2 ms | Seek + read compressed |
| Decompress blocks | ~30 ms | gzip decompress 10 MB total |
| Decode protobuf | ~20 ms | Parse events |
| Event filtering | ~5 ms | Exact match checks |
| Sort + format | ~5 ms | Sort by timestamp |
| **Total** | **~75 ms** | **Typical filtered query** |

**24-hour query with filters** (multi-file):

| Stage | Time | Notes |
|-------|------|-------|
| File selection | ~1 ms | 24 hourly files |
| Per-file processing | ~75 ms × 24 | Sequential file reads |
| Result merge | ~50 ms | Combine + sort results |
| **Total** | **~1.8 seconds** | **24 files** |

**Optimization opportunity**: Parallel file reads (planned v2.0)

### Concurrency Model (Read Path)

```
┌─────────────────────────────────────────────────────────────────┐
│                     Concurrent Queries                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  HTTP Request 1                HTTP Request 2                  │
│         │                              │                        │
│         v                              v                        │
│  ┌─────────────┐              ┌─────────────┐                  │
│  │ Query 1     │              │ Query 2     │                  │
│  │ Goroutine   │              │ Goroutine   │                  │
│  └─────┬───────┘              └─────┬───────┘                  │
│        │                            │                           │
│        v                            v                           │
│  ┌─────────────────────────────────────────┐                   │
│  │   Read Files (immutable)                │                   │
│  │   - No locking required                 │                   │
│  │   - Each query reads independently      │                   │
│  │   - OS page cache shared                │                   │
│  └─────────────────────────────────────────┘                   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Key Points**:
- **One goroutine per request**: Each HTTP request handled independently
- **No coordination**: Files are immutable, no locks needed
- **Shared page cache**: OS caches frequently accessed blocks
- **Unlimited concurrency**: Only limited by OS resources

### Query Optimization Examples

#### Example 1: No Filters (Full Scan)

```
Query: All events from 10:00-11:00

File: 2025-12-12-10.bin
- Total blocks: 300
- Blocks matching time range: 300 (all)
- Blocks to decompress: 300
- Events scanned: 60,000
- Events returned: 60,000

Execution time: ~400ms (decompress all blocks)
```

#### Example 2: Kind Filter

```
Query: kind=Pod, 10:00-11:00

File: 2025-12-12-10.bin
- Inverted index: KindToBlocks["Pod"] = [0, 2, 5, 7, ..., 290] (30 blocks)
- Blocks to decompress: 30 (90% skip rate!)
- Events scanned: 6,000
- Events returned: 6,000

Execution time: ~50ms (decompress 10% of blocks)
```

#### Example 3: Kind + Namespace Filter

```
Query: kind=Pod, namespace=default, 10:00-11:00

File: 2025-12-12-10.bin
- KindToBlocks["Pod"] = [0, 2, 5, 7, ..., 290] (30 blocks)
- NamespaceToBlocks["default"] = [0, 1, 2, 3, 4, 5] (50 blocks)
- Intersection: [0, 2, 5] (3 blocks, 99% skip rate!)
- Blocks to decompress: 3
- Events scanned: 600
- Events returned: 200 (after exact match)

Execution time: ~15ms (decompress 1% of blocks)
```

### Error Handling (Read Path)

| Error Type | Handling | Response |
|------------|----------|----------|
| **Invalid parameters** | Return 400 Bad Request | Error message to client |
| **File not found** | Skip file, continue query | Partial results returned |
| **Corrupted header** | Skip file, log error | Partial results |
| **Corrupted footer** | Skip file, log error | Partial results |
| **Invalid index JSON** | Skip file, log error | Partial results |
| **Decompression error** | Skip block, log error | Partial results |
| **Protobuf decode error** | Skip event, log error | Partial results |
| **Timeout** | Return 504 Gateway Timeout | Client can retry |

**Philosophy**: Partial results better than no results. Errors logged for debugging.

## Data Transformations

### Event Size Transformations

```
Original Kubernetes Object (with managedFields)
     │  Size: ~10 KB
     │  Format: runtime.Object (Go struct)
     v
Pruned ResourceEvent (managedFields removed)
     │  Size: ~1-2 KB (80-90% reduction)
     │  Format: ResourceEvent struct
     v
JSON-encoded Event
     │  Size: ~1.5 KB
     │  Format: JSON bytes
     v
Protobuf-encoded Event
     │  Size: ~1.2 KB (20% smaller than JSON)
     │  Format: Protobuf bytes
     v
Compressed Block (gzip)
     │  Size: ~300 bytes per event (~75% reduction)
     │  Format: gzip-compressed protobuf stream
     v
Stored on Disk
     │  Size: ~300 bytes per event
     │  Includes: Event data + metadata + indexes
```

**Total reduction: 10 KB → 300 bytes = 97% savings**

### Metadata Evolution

```
ResourceEvent (original)
     │  Fields: UID, Kind, Namespace, Name, Timestamp, Operation, Object
     v
EventBuffer Metadata (aggregated)
     │  Fields: KindSet, NamespaceSet, GroupSet, TimestampMin/Max
     │  Purpose: Track block contents for indexing
     v
Block Metadata (per block)
     │  Fields: ID, Offset, Length, EventCount, TimestampRange, Sets, Bloom filters
     │  Purpose: Enable filtering without decompression
     v
Inverted Indexes (per file)
     │  Fields: KindToBlocks, NamespaceToBlocks, GroupToBlocks
     │  Purpose: Map filters → candidate blocks
     v
Query Results
     │  Fields: Events[], Count, ExecutionTimeMs, FilesSearched, SegmentsScanned
     │  Purpose: Provide results + performance metrics
```

## Performance Characteristics

### Write Path Throughput

| Component | Throughput | Bottleneck |
|-----------|------------|------------|
| Watcher | 10,000 events/sec | Kubernetes API rate limit |
| Pruning | 50,000 events/sec | CPU (JSON parsing) |
| Validation | 100,000 events/sec | Minimal overhead |
| Event queue | Unlimited | Memory-based channel |
| JSON marshal | 20,000 events/sec | CPU (serialization) |
| Buffer accumulation | 50,000 events/sec | Memory operations |
| Block finalization | ~800 events/130ms | gzip compression (CPU) |
| **Sustained write rate** | **7,500 events/sec** | **Bottleneck: compression** |

### Read Path Latency

| Query Type | Latency (P50) | Latency (P99) | Bottleneck |
|------------|---------------|---------------|------------|
| 1-hour, no filters | 350 ms | 500 ms | Decompression |
| 1-hour, kind filter | 45 ms | 80 ms | I/O (block reads) |
| 1-hour, multi-filter | 12 ms | 25 ms | Inverted index ops |
| 24-hour, filtered | 180 ms | 400 ms | Sequential file reads |
| 7-day, filtered | 1.2 s | 2.5 s | File count |

**Optimization**: Parallel file reads can reduce 24-hour query to ~50ms (planned v2.0)

## Related Documentation

- [Architecture Overview](./overview.md) - System design and components
- [Storage Design](./storage-design.md) - File organization and blocks
- [Query Execution](./query-execution.md) - Query optimization details
- [Indexing Strategy](./indexing-strategy.md) - Inverted indexes and bloom filters
- [Compression](./compression.md) - Compression algorithms and performance

<!-- Source: docs-backup/ARCHITECTURE.md, internal/storage/, internal/watcher/, internal/api/ -->
