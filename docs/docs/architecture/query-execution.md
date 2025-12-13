---
title: Query Execution
description: Query pipeline, optimization, and performance characteristics
keywords: [architecture, query, execution, cache, performance]
---

# Query Execution

This document explains how Spectre executes queries efficiently using multi-stage filtering, caching, and state snapshot integration.

## Query Pipeline Overview

```
┌─────────────────────────────────────────────────────────────────┐
│  API Request: GET /api/v1/query?kind=Pod&time=[10:00,11:00]     │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         v
┌─────────────────────────────────────────────────────────────────┐
│  Stage 1: Request Validation                                     │
│  - Parse query parameters                                        │
│  - Validate time range, filters                                  │
│  - Apply defaults (limit, ordering)                              │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         v
┌─────────────────────────────────────────────────────────────────┐
│  Stage 2: File Selection (by time)                               │
│  - List hourly files in data directory                           │
│  - Filter by hour overlap with query time range                  │
│  - Include one file before start (for state snapshots)           │
│  Result: 1-24 files (typical: 2-3 files)                         │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         v
┌─────────────────────────────────────────────────────────────────┐
│  Stage 3: Per-File Query (parallel)                              │
│  For each file:                                                   │
│    ├─ Read footer → index section                                │
│    ├─ Filter blocks (inverted index + Bloom filters + time)      │
│    ├─ Decompress candidate blocks (with cache)                   │
│    ├─ Parse events from protobuf                                 │
│    ├─ Apply in-memory filters                                    │
│    └─ Collect matching events                                    │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         v
┌─────────────────────────────────────────────────────────────────┐
│  Stage 4: Result Merging                                         │
│  - Combine events from all files                                 │
│  - Add state snapshot events (for pre-existing resources)        │
│  - Sort by timestamp (ascending)                                 │
│  - Apply limit                                                    │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         v
┌─────────────────────────────────────────────────────────────────┐
│  Stage 5: Response Serialization                                 │
│  - Convert events to API format                                  │
│  - Add query metadata (total, duration)                          │
│  - Return JSON response                                          │
└─────────────────────────────────────────────────────────────────┘
```

## Stage 1: Request Validation

**Input:** HTTP query parameters
**Output:** Validated QueryRequest

```go
type QueryRequest struct {
    StartTime int64  // Unix nanoseconds
    EndTime   int64  // Unix nanoseconds
    Kind      string // Optional filter
    Namespace string // Optional filter
    Group     string // Optional filter
    Limit     int32  // Max results (default: 1000)
    OrderBy   string // "asc" or "desc"
}
```

**Validation:**
- `StartTime` must be before `EndTime`
- Time range must be reasonable (not > 1 year)
- Limit must be 1-10,000
- Filter values must be valid (no SQL injection)

**Performance:** \<1 ms

## Stage 2: File Selection

**Goal:** Identify hourly files that overlap the query time range

```go
func SelectFiles(dataDir string, startTime, endTime int64) ([]string, error) {
    var selectedFiles []string

    // List all .bin files
    files, _ := os.ReadDir(dataDir)

    for _, file := range files {
        // Parse hour from filename: YYYY-MM-DD-HH.bin
        fileHour := ParseHourFromFilename(file.Name())

        // Check if file hour overlaps query range
        fileStart := fileHour.Unix()
        fileEnd := fileHour.Add(1 * time.Hour).Unix()

        if fileEnd >= startTime && fileStart \<= endTime {
            selectedFiles = append(selectedFiles, file.Name())
        }
    }

    // Include one file before start time (for state snapshots)
    if len(selectedFiles) > 0 {
        previousFile := GetPreviousHourFile(selectedFiles[0])
        if previousFile != "" {
            selectedFiles = append([]string{previousFile}, selectedFiles...)
        }
    }

    return selectedFiles, nil
}
```

**Example:**
```
Query: [2025-12-12 10:30:00, 2025-12-12 11:15:00]
Files:
  2025-12-12-09.bin (for state snapshots)
  2025-12-12-10.bin (overlaps query)
  2025-12-12-11.bin (overlaps query)
```

**Performance:** \<5 ms (directory listing + parsing)

## Stage 3: Per-File Query

**Most Complex Stage:** Multi-level filtering and decompression

### 3.1 Index Loading

```go
// Read footer to locate index
footer := ReadFileFooter(filePath)

// Read index section
indexData := ReadAt(filePath, footer.IndexOffset, footer.IndexLength)
index := json.Unmarshal(indexData)
```

**Performance:** ~10-20 ms per file (depends on index size)

### 3.2 Block Filtering (Inverted Indexes)

```go
// Build filter map from query
filters := map[string]string{
    "kind": query.Kind,
    "namespace": query.Namespace,
    "group": query.Group,
}

// Get candidate blocks using inverted indexes
candidateBlocks := GetCandidateBlocks(index.InvertedIndexes, filters)

// Example:
// kind=Pod → blocks [0, 1, 3, 7, 9]
// namespace=default → blocks [0, 1, 2, 3]
// Intersection → blocks [0, 1, 3]
```

**Performance:** \<2 ms (map lookups + intersection)
**Skip Rate:** 90-98% of blocks (typical)

### 3.3 Time Range Filtering

```go
var timeFilteredBlocks []int32

for _, blockID := range candidateBlocks {
    blockMeta := index.BlockMetadata[blockID]

    // Check if block overlaps query time range
    if blockMeta.TimestampMax >= query.StartTime &&
       blockMeta.TimestampMin <= query.EndTime {
        timeFilteredBlocks = append(timeFilteredBlocks, blockID)
    }
}
```

**Performance:** \<1 ms (linear scan of candidate blocks)
**Additional Skip Rate:** 30-60% of candidates

### 3.4 Block Decompression (with Cache)

```go
for _, blockID := range timeFilteredBlocks {
    blockMeta := index.BlockMetadata[blockID]

    // Check cache first
    cacheKey := fmt.Sprintf("%s:%d", filePath, blockID)
    if cachedEvents, ok := cache.Get(cacheKey); ok {
        // Cache hit - use decompressed events
        events = cachedEvents
    } else {
        // Cache miss - read and decompress
        compressedData := ReadBlockData(filePath, blockMeta.Offset, blockMeta.CompressedLength)
        uncompressedData := gzip.Decompress(compressedData)
        events := protobuf.Parse(uncompressedData)

        // Store in cache
        cache.Set(cacheKey, events)
    }

    // Filter events in memory
    for _, event := range events {
        if MatchesFilters(event, query) {
            results = append(results, event)
        }
    }
}
```

**Performance per block:**
- Cache hit: ~1 ms
- Cache miss: ~30-50 ms (10 MB block)

**Cache Impact:**
- First query: 100% cache misses
- Repeated query: 80-90% cache hits
- Dashboard queries: 60-80% cache hits

## Stage 4: Result Merging

### 4.1 Combining Events

```go
var allEvents []*Event

// Collect from all files
for _, file := range files {
    fileEvents := QueryFile(file, query)
    allEvents = append(allEvents, fileEvents...)
}
```

### 4.2 State Snapshot Integration

```go
// Get state snapshots from previous hour's file
stateSnapshots := ReadStateSnapshots(files[0])

// For each resource in snapshots
for resourceKey, state := range stateSnapshots {
    // Check if resource has events in query range
    hasEvents := false
    for _, event := range allEvents {
        if event.Resource.Key() == resourceKey {
            hasEvents = true
            break
        }
    }

    // If no events but resource exists → create synthetic event
    if !hasEvents && state.EventType != "DELETE" {
        syntheticEvent := CreateStateEvent(state)
        allEvents = append(allEvents, syntheticEvent)
    }
}
```

**Purpose:** Show resources that exist but have no events in query window

**Example:**
```
Previous hour: Deployment "nginx" created
Query hour: No events for "nginx"
Result: Synthetic "state-" event shows Deployment still exists
```

### 4.3 Sorting and Limiting

```go
// Sort by timestamp
sort.Slice(allEvents, func(i, j int) bool {
    return allEvents[i].Timestamp < allEvents[j].Timestamp
})

// Apply limit
if len(allEvents) > query.Limit {
    allEvents = allEvents[:query.Limit]
}
```

**Performance:** O(N log N) where N = result count

## Stage 5: Response Serialization

```go
type QueryResponse struct {
    Events     []*Event  `json:"events"`
    Total      int32     `json:"total"`
    Duration   int64     `json:"duration_ms"`
    FilesScanned int32   `json:"files_scanned"`
    BlocksRead int32     `json:"blocks_read"`
}

// Serialize to JSON
responseJSON := json.Marshal(response)
```

**Performance:** ~10-50 ms (depends on result size)

## Block Cache

### LRU Cache Design

```go
type BlockCache struct {
    maxMemory int64                       // Max cache size (MB)
    cache     map[string]*CachedBlock     // Key → cached block
    lru       *list.List                  // LRU ordering
    mutex     sync.RWMutex                // Thread-safe access

    // Metrics
    hits              int64
    misses            int64
    evictions         int64
    bytesDecompressed int64
}

type CachedBlock struct {
    Key    string
    Events []*Event
    Size   int64
}
```

### Cache Operations

**Get (Read):**
```go
func (c *BlockCache) Get(key string) ([]*Event, bool) {
    c.mutex.RLock()
    defer c.mutex.RUnlock()

    if block, ok := c.cache[key]; ok {
        // Move to front of LRU
        c.lru.MoveToFront(block.lruElement)
        atomic.AddInt64(&c.hits, 1)
        return block.Events, true
    }

    atomic.AddInt64(&c.misses, 1)
    return nil, false
}
```

**Set (Write):**
```go
func (c *BlockCache) Set(key string, events []*Event, size int64) {
    c.mutex.Lock()
    defer c.mutex.Unlock()

    // Evict if over capacity
    for c.currentSize+size > c.maxMemory && c.lru.Len() > 0 {
        oldest := c.lru.Back()
        delete(c.cache, oldest.Value.Key)
        c.currentSize -= oldest.Value.Size
        c.lru.Remove(oldest)
        atomic.AddInt64(&c.evictions, 1)
    }

    // Add to cache
    c.cache[key] = &CachedBlock{Key: key, Events: events, Size: size}
    c.currentSize += size
}
```

### Cache Metrics

```go
type CacheMetrics struct {
    MaxMemory         int64   `json:"max_memory_mb"`
    UsedMemory        int64   `json:"used_memory_mb"`
    Items             int64   `json:"items"`
    Hits              int64   `json:"hits"`
    Misses            int64   `json:"misses"`
    HitRate           float64 `json:"hit_rate"`
    Evictions         int64   `json:"evictions"`
    BytesDecompressed int64   `json:"bytes_decompressed"`
}
```

**API Endpoint:**
```bash
curl http://localhost:8080/api/v1/cache/stats

{
  "max_memory_mb": 100,
  "used_memory_mb": 85,
  "items": 42,
  "hits": 1250,
  "misses": 180,
  "hit_rate": 0.87,
  "evictions": 15,
  "bytes_decompressed": 420000000
}
```

### Cache Hit Rate Scenarios

| Scenario                 | Hit Rate | Explanation                     |
| ------------------------ | -------- | ------------------------------- |
| Repeated query           | 90-95%   | Same blocks accessed repeatedly |
| Dashboard (5min refresh) | 80-85%   | Recent blocks stay hot          |
| Time-series query        | 60-70%   | Some overlap, some new blocks   |
| Historical analysis      | 20-30%   | Old blocks not in cache         |
| Ad-hoc exploration       | 10-20%   | Random access pattern           |

## Performance Metrics

### Query Response Time

**Breakdown by stage:**

| Stage              | Latency     | Percentage |
| ------------------ | ----------- | ---------- |
| Request validation | \<1 ms      | \<1%       |
| File selection     | ~5 ms       | ~2%        |
| Index loading      | ~20 ms      | ~8%        |
| Block filtering    | ~3 ms       | ~1%        |
| Decompression      | ~240 ms     | ~85%       |
| Result merging     | ~10 ms      | ~4%        |
| **Total**          | **~280 ms** | **100%**   |

**Decompression dominates query time** (85% of latency)

### Query Performance by Time Range

| Time Range | Files | Blocks Scanned | Decompression | Total Time |
| ---------- | ----- | -------------- | ------------- | ---------- |
| 1 hour     | 1-2   | 5-10           | ~150 ms       | ~200 ms    |
| 6 hours    | 6-7   | 20-30          | ~600 ms       | ~700 ms    |
| 24 hours   | 24-25 | 80-120         | ~2400 ms      | ~2500 ms   |
| 7 days     | 168+  | 500-800        | ~15000 ms     | ~16000 ms  |

**Key Insight:** Query time scales linearly with blocks scanned (not total data size)

### Cache Impact on Performance

**Scenario:** Repeated 1-hour query

| Attempt | Cache Hit Rate | Decompression Time | Total Time |
| ------- | -------------- | ------------------ | ---------- |
| First   | 0%             | 150 ms             | 200 ms     |
| Second  | 90%            | 15 ms              | 65 ms      |
| Third+  | 95%            | 7 ms               | 57 ms      |

**Improvement:** 3.5× faster with warm cache

## Optimization Strategies

### ✅ Do

- **Enable caching** (`--cache-enabled=true`) - 3× faster repeated queries
- **Increase cache size** (`--cache-max-mb=200+`) - For read-heavy workloads
- **Use specific filters** - Reduces blocks scanned (kind + namespace better than kind alone)
- **Limit time ranges** - Query last hour instead of last week when possible
- **Apply limits** - Use `limit=100` for dashboards vs unbounded queries

### ❌ Don't

- **Don't query without filters** - Scans all blocks (slow)
- **Don't use very wide time ranges** - 7+ days takes 10+ seconds
- **Don't disable cache** - Repeated queries will be slow
- **Don't set limit too high** - Large result sets take time to serialize
- **Don't query deleted resources** - Filter `event_type != DELETE` for active resources

## Related Documentation

- [Storage Design](./storage-design.md) - Overall architecture
- [Indexing Strategy](./indexing-strategy.md) - Block filtering techniques
- [Compression](./compression.md) - Decompression performance
- [Storage Settings](../configuration/storage-settings.md) - Cache configuration

<!-- Source: internal/storage/query.go, internal/storage/block_cache.go -->
