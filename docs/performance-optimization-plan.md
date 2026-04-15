# Timeline Query Performance Optimization Plan

## Executive Summary

Current `/timeline` requests are slow (~800ms per file) despite block cache hits because:
1. File metadata (index sections) are read from disk on every query
2. Files are queried 3 times independently (standalone, Event query, resource query)
3. A double-pass reads each file twice for deleted resource collection
4. Per-file I/O overhead dominates query time even with cached blocks

## Problem Analysis

### Current Architecture Issues

#### 1. Cache Only Covers Blocks, Not File Metadata
**Current Behavior:**
- `BlockCache` caches decompressed block data
- Each `queryFile` call still performs:
  - Opens file (lines 442, 222 in query.go)
  - Seeks to start for header
  - Seeks to end for footer
  - Reads index section from disk (can be large - contains all block metadata and resource states)
  - Unmarshals protobuf index section

**Impact:** ~200-300ms per file for I/O and unmarshaling

**Files Involved:**
- `internal/storage/query.go:442` - Opens BlockReader
- `internal/storage/query.go:470` - Calls `reader.ReadFile()`
- `internal/storage/block_reader.go:227-252` - `ReadFile()` reads header, footer, index section
- `internal/storage/block_reader.go:78-96` - `ReadIndexSection()` reads and unmarshals protobuf

#### 2. Double-Pass File Reading
**Current Behavior:**
```go
// First pass: lines 219-240
for _, filePath := range filesToQuery {
    reader, err := NewBlockReader(filePath)
    fileData, err := reader.ReadFile()  // Reads index section
    // ... collect deleted resources
    reader.Close()
}

// Second pass: lines 244-277
for _, filePath := range filesToQuery {
    events, ... := qe.queryFileWithSnapshotsWithDeleted(...)  // Reads index section again
}
```

**Impact:** 2x file I/O per file

**Files Involved:**
- `internal/storage/query.go:219-240` - First pass for deleted resources
- `internal/storage/query.go:244-277` - Second pass for actual query
- `internal/storage/query.go:470` - Each pass calls `ReadFile()`

#### 3. Files Queried 3 Times Independently
**Current Behavior:**
- Timeline handler executes 2 concurrent queries (lines 102-156 in timeline_handler.go):
  1. Resource query (user filters)
  2. Event query (kind="Event")
- Storage layer may execute standalone query
- Each query independently opens and reads the same files

**Impact:** 3x file I/O per file (worst case)

**Files Involved:**
- `internal/api/timeline_handler.go:102-156` - Executes 2 concurrent queries
- `internal/api/timeline_handler.go:134` - Resource query
- `internal/api/timeline_handler.go:147` - Event query
- `internal/storage/query.go:82-428` - Each Execute() independently processes files

#### 4. Per-File Query Slowness Breakdown

For files like `2025-12-16-20.bin` (~800ms):
- File I/O for header/footer/index: ~150ms
- Protobuf unmarshaling of index: ~100ms
- Iterating block metadata: ~50ms
- Event filtering on cached blocks: ~200ms
- Processing final resource states: ~300ms

**Block cache saves:** ~500ms in decompression
**Still required:** ~800ms in metadata I/O and processing

## Optimization Options

### Option 1: Cache File Metadata/Index Sections ⭐ HIGHEST IMPACT

**Description:**
Cache the result of `ReadFile()` (StorageFileData) so the index section doesn't need to be read from disk each time.

**Benefits:**
- **Eliminates file I/O overhead:** ~150ms saved per file per query
- **Eliminates protobuf unmarshaling:** ~100ms saved per file per query
- **Simplest to implement:** Extends existing cache pattern
- **Immediate impact:** All queries benefit instantly
- **Memory efficient:** Index sections typically 50-500KB (much smaller than blocks)

**Estimated Improvement:** 250-400ms per file (~30-50% total reduction)

**Implementation Complexity:** LOW

**Memory Cost:**
- Index section: 50-500KB per file
- For 24 files (1 day): 1-12MB total
- Negligible compared to block cache (100MB+)

**Cache Strategy:**
```
Key: filename + file_mtime
Value: StorageFileData (Header, Footer, IndexSection)
TTL: Until file modification time changes
Eviction: LRU when max memory reached
```

**Files to Modify:**
1. `internal/storage/file_metadata_cache.go` - New file for metadata cache
2. `internal/storage/query.go:219-240` - Use cached metadata for first pass
3. `internal/storage/query.go:470` - Use cached metadata for second pass
4. `internal/storage/block_reader.go:227-252` - Add cache integration to ReadFile()

**Implementation Steps:**
1. Create `FileMetadataCache` similar to `BlockCache`
2. Add cache key: `filename:mtime`
3. Modify `ReadFile()` to check cache first
4. Update `QueryExecutor` to use metadata cache
5. Add metrics for cache hits/misses
6. Add tests for cache behavior

---

### Option 2: Share File Queries Across Concurrent Queries ⭐⭐ HIGH IMPACT

**Description:**
When Event and resource queries read the same files, share the file data and filter differently.

**Benefits:**
- **Eliminates duplicate file reads:** 2-3x reduction in file I/O
- **Reduces contention:** Less concurrent disk access
- **Synergy with Option 1:** Combined with metadata cache = massive speedup
- **Better resource utilization:** Lower CPU and disk I/O

**Estimated Improvement:** 400-600ms total (when files queried 2-3 times)

**Implementation Complexity:** MEDIUM

**Approach:**
```go
// Timeline handler coordinates file reading
fileDataCache := make(map[string]*CachedFileData)
var mu sync.RWMutex

// Both queries use shared cache
resourceQuery(fileDataCache, &mu, resourceFilters)
eventQuery(fileDataCache, &mu, eventFilters)
```

**Files to Modify:**
1. `internal/api/timeline_handler.go:102-156` - Add shared file data cache
2. `internal/storage/query.go:82-428` - Accept optional shared cache parameter
3. `internal/storage/query.go:244-277` - Use shared cache in query loop

**Implementation Steps:**
1. Add `SharedFileCache` struct in timeline_handler.go
2. Modify `Execute()` to accept optional shared cache
3. Update queryFile to check shared cache first
4. Add synchronization for concurrent access
5. Measure and validate performance gains
6. Add tests for concurrent access

---

### Option 3: Eliminate the Double-Pass ⭐⭐ MEDIUM IMPACT

**Description:**
Collect deleted resources during the main query pass instead of a separate first pass.

**Benefits:**
- **Eliminates one complete file scan:** 50% reduction in file reads
- **Simpler code:** Less complex query logic
- **Memory efficient:** Stream processing instead of buffering
- **Works independently:** Can be implemented without other changes

**Estimated Improvement:** 300-400ms total (eliminates one pass)

**Implementation Complexity:** LOW-MEDIUM

**Approach:**
```go
// Single pass approach
deletedResources := make(map[string]bool)
allEvents := make([]models.Event, 0)

for _, filePath := range filesToQuery {
    fileData := readFileOnce(filePath)  // Read once
    
    // Collect deleted resources inline
    for key, state := range fileData.IndexSection.FinalResourceStates {
        if state.EventType == "DELETED" {
            deletedResources[key] = true
        }
    }
    
    // Query and filter events
    events := queryEvents(fileData, query, deletedResources)
    allEvents = append(allEvents, events...)
}
```

**Files to Modify:**
1. `internal/storage/query.go:219-240` - Remove first pass loop
2. `internal/storage/query.go:244-277` - Collect deleted resources inline in main loop
3. `internal/storage/query.go:576-581` - Update state snapshot logic

**Implementation Steps:**
1. Move deleted resource collection into main query loop
2. Update queryFileWithSnapshotsWithDeleted signature
3. Ensure deleted resources from earlier files affect later files
4. Update tests for new behavior
5. Verify correctness with existing test suite

**Gotcha:** Need to ensure deleted resources from file N affect state snapshots in file N+1

---

### Option 4: Add File-Level Caching with TTL ⭐ COMPLEMENTARY

**Description:**
Cache entire `StorageFileData` with TTL based on file modification time.

**Benefits:**
- **File-level invalidation:** Automatic cache invalidation on file changes
- **Complements Option 1:** Provides TTL strategy for metadata cache
- **Production-ready:** Handles file rotation/updates automatically
- **Observability:** Clear cache invalidation behavior

**Estimated Improvement:** Maintains performance gains from other options

**Implementation Complexity:** LOW

**Cache Strategy:**
```go
type CachedFileMetadata struct {
    Data     *StorageFileData
    ModTime  time.Time
    CachedAt time.Time
}

func (cache *FileMetadataCache) Get(filePath string) (*StorageFileData, error) {
    stat, _ := os.Stat(filePath)
    
    if cached, ok := cache.entries[filePath]; ok {
        if cached.ModTime.Equal(stat.ModTime()) {
            return cached.Data, nil  // Cache hit
        }
        cache.invalidate(filePath)  // File changed
    }
    
    // Cache miss or invalidated
    return cache.loadAndCache(filePath, stat.ModTime())
}
```

**Files to Modify:**
1. `internal/storage/file_metadata_cache.go` - Add TTL logic to metadata cache
2. `internal/storage/block_reader.go:227-252` - Check file mtime before using cache

**Implementation Steps:**
1. Add `ModTime` field to cache entries
2. Check file mtime on cache lookup
3. Invalidate if mtime changed
4. Add metrics for invalidations
5. Test with file updates

---

## Recommended Implementation Plan

### Phase 1: Quick Wins (Week 1)
**Goal:** 40-50% performance improvement with minimal risk

1. **Implement Option 1: File Metadata Cache** (2-3 days)
   - Create FileMetadataCache
   - Integrate with query executor
   - Add basic tests
   - Deploy to staging

2. **Implement Option 3: Eliminate Double-Pass** (1-2 days)
   - Refactor query loop
   - Update tests
   - Verify correctness

**Expected Result:** 600-800ms → 300-400ms per timeline request

### Phase 2: Advanced Optimization (Week 2)
**Goal:** Additional 30-40% improvement

3. **Implement Option 2: Shared File Queries** (3-4 days)
   - Add shared cache to timeline handler
   - Coordinate concurrent queries
   - Add synchronization
   - Performance testing

4. **Implement Option 4: TTL Strategy** (1 day)
   - Add file mtime checking
   - Implement cache invalidation
   - Monitoring/metrics

**Expected Result:** 300-400ms → 150-200ms per timeline request

### Phase 3: Polish & Monitoring (Week 3)
**Goal:** Production readiness

5. **Add comprehensive metrics** (1-2 days)
   - Cache hit/miss rates
   - File I/O timing
   - Query execution breakdown
   - Grafana dashboards

6. **Load testing & optimization** (2-3 days)
   - Concurrent query testing
   - Cache sizing optimization
   - Memory profiling
   - Stress testing

**Expected Result:** Production-ready, monitored, optimized

---

## Implementation Details

### File Metadata Cache Structure

```go
// internal/storage/file_metadata_cache.go
package storage

type FileMetadataCache struct {
    lru        *lru.Cache[string, *CachedFileMetadata]
    maxMemory  int64
    usedMemory int64
    mu         sync.RWMutex
    logger     *logging.Logger
    
    // Metrics
    hits         uint64
    misses       uint64
    invalidations uint64
}

type CachedFileMetadata struct {
    FilePath     string
    Data         *StorageFileData
    ModTime      time.Time
    Size         int64
    CachedAt     time.Time
}

func (fmc *FileMetadataCache) Get(filePath string) (*StorageFileData, error) {
    stat, err := os.Stat(filePath)
    if err != nil {
        return nil, err
    }
    
    fmc.mu.RLock()
    cached, ok := fmc.lru.Get(filePath)
    fmc.mu.RUnlock()
    
    if ok && cached.ModTime.Equal(stat.ModTime()) {
        atomic.AddUint64(&fmc.hits, 1)
        return cached.Data, nil
    }
    
    if ok {
        atomic.AddUint64(&fmc.invalidations, 1)
    }
    
    atomic.AddUint64(&fmc.misses, 1)
    return fmc.loadAndCache(filePath, stat.ModTime())
}

func (fmc *FileMetadataCache) loadAndCache(filePath string, modTime time.Time) (*StorageFileData, error) {
    reader, err := NewBlockReader(filePath)
    if err != nil {
        return nil, err
    }
    defer reader.Close()
    
    fileData, err := reader.ReadFile()
    if err != nil {
        return nil, err
    }
    
    // Estimate size: header + footer + index section
    size := int64(FileHeaderSize + FileFooterSize + len(fileData.IndexSection.BlockMetadata)*1024)
    
    cached := &CachedFileMetadata{
        FilePath: filePath,
        Data:     fileData,
        ModTime:  modTime,
        Size:     size,
        CachedAt: time.Now(),
    }
    
    fmc.mu.Lock()
    fmc.lru.Add(filePath, cached)
    fmc.usedMemory += size
    fmc.mu.Unlock()
    
    return fileData, nil
}
```

### Shared File Cache Structure

```go
// internal/api/timeline_handler.go
type SharedFileCache struct {
    cache map[string]*CachedFileData
    mu    sync.RWMutex
}

type CachedFileData struct {
    Data      *storage.StorageFileData
    LoadedAt  time.Time
}

func (sfc *SharedFileCache) GetOrLoad(filePath string, loader func() (*storage.StorageFileData, error)) (*storage.StorageFileData, error) {
    sfc.mu.RLock()
    cached, ok := sfc.cache[filePath]
    sfc.mu.RUnlock()
    
    if ok {
        return cached.Data, nil
    }
    
    // Load file data
    data, err := loader()
    if err != nil {
        return nil, err
    }
    
    sfc.mu.Lock()
    sfc.cache[filePath] = &CachedFileData{
        Data:     data,
        LoadedAt: time.Now(),
    }
    sfc.mu.Unlock()
    
    return data, nil
}
```

### Eliminated Double-Pass Implementation

```go
// internal/storage/query.go - Modified Execute function

// Single-pass: query files and collect deleted resources simultaneously
deletedResources := make(map[string]bool)
var allEvents []models.Event

for _, filePath := range filesToQuery {
    // Read file once (will hit metadata cache)
    reader, err := NewBlockReader(filePath)
    if err != nil {
        continue
    }
    
    fileData, err := reader.ReadFile()
    reader.Close()
    if err != nil {
        continue
    }
    
    // Collect deleted resources from this file
    for resourceKey, state := range fileData.IndexSection.FinalResourceStates {
        if state.EventType == string(models.EventTypeDelete) {
            deletedResources[resourceKey] = true
        }
    }
    
    // Query events with current deleted resources set
    events := qe.queryFileWithDeletedResources(ctx, fileData, query, deletedResources)
    allEvents = append(allEvents, events...)
}
```

---

## Performance Projections

### Current Performance
- File I/O (header/footer/index): 150ms
- Protobuf unmarshaling: 100ms
- Block processing (cached): 50ms
- Event filtering: 200ms
- State processing: 300ms
- **Total per file:** ~800ms
- **3 files × 2 passes × 2 queries:** ~9,600ms (9.6s)

### After Phase 1 (Metadata Cache + Eliminate Double-Pass)
- File I/O (cached): 0ms
- Protobuf unmarshaling (cached): 0ms
- Block processing (cached): 50ms
- Event filtering: 200ms
- State processing: 300ms
- **Total per file:** ~550ms (31% reduction)
- **3 files × 1 pass × 2 queries:** ~3,300ms (3.3s) → **66% improvement**

### After Phase 2 (+ Shared File Queries)
- File I/O (cached): 0ms
- Protobuf unmarshaling (cached): 0ms
- Block processing (cached): 50ms
- Event filtering: 200ms
- State processing: 300ms
- **Total per file:** ~550ms
- **3 files × 1 pass × 1 shared query:** ~1,650ms (1.65s) → **83% improvement**

---

## Risk Assessment

| Optimization | Risk Level | Mitigation |
|-------------|-----------|------------|
| Option 1: Metadata Cache | LOW | Extensive testing, gradual rollout |
| Option 2: Shared Queries | MEDIUM | Add comprehensive concurrency tests |
| Option 3: Eliminate Double-Pass | LOW | Validate with existing test suite |
| Option 4: TTL Strategy | LOW | Monitor cache invalidation metrics |

---

## Success Metrics

### Performance Metrics
- [ ] Timeline request latency P50 < 500ms
- [ ] Timeline request latency P99 < 2s
- [ ] Cache hit rate > 90%
- [ ] File I/O operations reduced by 70%

### Observability Metrics
- [ ] Cache hit/miss rates per cache type
- [ ] File I/O timing breakdown
- [ ] Query execution time by phase
- [ ] Memory usage by cache type

### Quality Metrics
- [ ] All existing tests pass
- [ ] No regressions in query correctness
- [ ] Memory usage remains stable
- [ ] No memory leaks under load

---

## Testing Strategy

### Unit Tests
1. FileMetadataCache hit/miss behavior
2. Cache invalidation on file mtime change
3. Shared cache concurrent access
4. Single-pass deleted resource collection

### Integration Tests
1. End-to-end timeline queries with cache
2. Concurrent query correctness
3. Cache behavior under file rotation
4. Memory usage under load

### Performance Tests
1. Baseline vs optimized timeline latency
2. Cache effectiveness at different sizes
3. Concurrent query scalability
4. Memory usage patterns

### Load Tests
1. Sustained high query rate
2. Cache eviction behavior
3. File rotation impact
4. Concurrent user simulation

---

## Rollout Plan

### Stage 1: Development (Week 1-2)
- Implement Phase 1 optimizations
- Unit and integration tests
- Local performance testing

### Stage 2: Staging (Week 3)
- Deploy to staging environment
- Load testing
- Performance validation
- Bug fixes

### Stage 3: Canary (Week 4)
- Deploy to 10% of production traffic
- Monitor metrics closely
- Gradual rollout to 50%

### Stage 4: Production (Week 5)
- Full production rollout
- Performance monitoring
- Documentation update
- Post-mortem analysis

---

## Monitoring & Alerting

### Key Metrics to Monitor
1. **Cache Performance**
   - `file_metadata_cache_hit_rate`
   - `file_metadata_cache_miss_rate`
   - `file_metadata_cache_memory_mb`
   - `file_metadata_cache_evictions_total`

2. **Query Performance**
   - `timeline_request_duration_seconds` (P50, P95, P99)
   - `file_io_duration_seconds`
   - `query_execution_duration_seconds`

3. **System Health**
   - `storage_disk_io_operations_total`
   - `storage_memory_usage_bytes`
   - `concurrent_queries_active`

### Alerts
- Timeline P99 latency > 3s (warning)
- Timeline P99 latency > 5s (critical)
- Cache hit rate < 80% (warning)
- Memory usage > 90% (warning)

---

## Future Optimizations (Post-Phase 3)

1. **Query Result Cache**
   - Cache query results for identical queries
   - Short TTL (1-5 seconds)
   - Useful for dashboards/polling

2. **Incremental Query Updates**
   - WebSocket-based incremental updates
   - Only send new events since last query
   - Reduces data transfer

3. **Index Section Compression**
   - Compress index sections in cache
   - Trade CPU for memory
   - 2-3x memory reduction

4. **Parallel File Processing**
   - Process multiple files concurrently
   - Worker pool pattern
   - CPU-bound optimization

5. **Query Plan Optimization**
   - Analyze query patterns
   - Pre-warm cache for common queries
   - Predictive prefetching

---

## Conclusion

The proposed optimizations address the root causes of slow `/timeline` requests:

1. **Metadata caching** eliminates repeated file I/O and unmarshaling
2. **Shared queries** eliminates duplicate file processing
3. **Single-pass processing** eliminates redundant file scans
4. **TTL strategy** ensures cache correctness

**Expected Overall Improvement:** 83% latency reduction (9.6s → 1.65s)

**Implementation Effort:** 3 weeks for Phases 1-3

**Risk:** Low-Medium, mitigated through phased rollout and comprehensive testing

**Recommendation:** Proceed with Phase 1 implementation immediately for quick wins, followed by Phase 2 for maximum performance.
