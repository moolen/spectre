# Architecture: Kubernetes Event Monitoring System

**Document**: Architecture Overview
**Date**: 2025-11-25
**Version**: 1.0

## Table of Contents

1. [System Overview](#system-overview)
2. [Component Architecture](#component-architecture)
3. [Storage Design](#storage-design)
4. [Query Execution](#query-execution)
5. [Data Flow](#data-flow)
6. [Performance Characteristics](#performance-characteristics)

---

## System Overview

The Kubernetes Event Monitoring System captures all resource changes (CREATE, UPDATE, DELETE) from a Kubernetes cluster, stores them efficiently with compression and indexing, and provides a queryable API for retrieving historical events.

```
┌─────────────────────────────────────────────────────────────┐
│           Kubernetes Event Monitoring System                 │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │  K8s Watcher │  │ K8s Watcher  │  │ K8s Watcher  │      │
│  │  (Pods)      │  │ (Deployments)│  │  (Services)  │      │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │
│         └──────────────────┼──────────────────┘              │
│                            │ Events                          │
│                   ┌────────▼────────┐                        │
│                   │  Event Queue    │                        │
│                   │ (Concurrent)    │                        │
│                   └────────┬────────┘                        │
│                            │                                 │
│              ┌─────────────┴─────────────┐                  │
│              │ Pruning & Validation      │                  │
│              │ (Remove managedFields)    │                  │
│              └─────────────┬─────────────┘                  │
│                            │ Events                          │
│              ┌─────────────▼─────────────┐                  │
│              │  Storage Layer            │                  │
│              │  ┌──────────────────────┐ │                  │
│              │  │ Hourly Files         │ │                  │
│              │  │  ├─ File Header      │ │                  │
│              │  │  ├─ Blocks           │ │                  │
│              │  │  │  ├─ Compressed    │ │                  │
│              │  │  │  │   Data         │ │                  │
│              │  │  │  └─ Metadata      │ │                  │
│              │  │  ├─ Index Section    │ │                  │
│              │  │  │  ├─ Timestamp     │ │                  │
│              │  │  │  │   Index        │ │                  │
│              │  │  │  └─ Inverted      │ │                  │
│              │  │  │      Index        │ │                  │
│              │  │  └─ File Footer      │ │                  │
│              │  └──────────────────────┘ │                  │
│              └────────────┬────────────────┘                  │
│                           │                                 │
│              ┌────────────▼────────────┐                   │
│              │  Query Engine           │                   │
│              │  ├─ File Selection      │                   │
│              │  ├─ Block Filtering     │                   │
│              │  ├─ Decompression       │                   │
│              │  └─ Result Aggregation  │                   │
│              └────────────┬────────────┘                   │
│                           │ Query Results                   │
│              ┌────────────▼────────────┐                   │
│              │  HTTP API Server        │                   │
│              │  /v1/search             │                   │
│              └─────────────────────────┘                   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## Component Architecture

### 1. Watcher Component (internal/watcher/)

**Responsibility**: Capture Kubernetes resource changes

**Files**:
- `watcher.go` - Main watcher factory and registration
- `event_handler.go` - Resource event handler (ADD/UPDATE/DELETE)
- `event_queue.go` - Concurrent event buffering
- `pruner.go` - managedFields removal
- `validator.go` - Event validation and error handling

**Flow**:
```
K8s ResourceEventHandler
    ↓
Event (with managedFields)
    ↓ (Pruning)
Event (cleaned)
    ↓ (Validation)
Valid Event
    ↓ (Queue)
Event Queue Buffer
```

**Key Features**:
- Watches multiple resource types in parallel
- Handles concurrent events without loss
- Removes large metadata.managedFields for data reduction
- Validates events before storage

---

### 2. Storage Component (internal/storage/)

**Responsibility**: Store events with compression and indexing

**Core Modules**:

#### File Management
- `storage.go` - Hourly file creation and rotation
- `file.go` - File handling and metadata

#### Block-Based Storage
- `block_storage.go` - Block writer implementation
- `block_reader.go` - Block reader for decompression
- `block.go` - Block structures and compression
- `block_format.go` - Binary format definitions

#### Indexing
- `index.go` - Sparse timestamp index (O(log N) lookups)
- `segment_metadata.go` - Segment metadata tracking (kinds, namespaces, groups)
- `filter.go` - Bloom filters for 3-dimensional filtering

#### Compression
- `compression.go` - Gzip compression/decompression

#### Data Organization
```
Hourly File Structure:
┌────────────────────────────────────┐
│        File Header (77 bytes)       │
├────────────────────────────────────┤
│  Block 1 (compressed events)        │
├────────────────────────────────────┤
│  Block 2 (compressed events)        │
├────────────────────────────────────┤
│  Block N (compressed events)        │
├────────────────────────────────────┤
│     Index Section (JSON)            │
│  ├─ Block Metadata Array            │
│  ├─ Inverted Indexes                │
│  └─ Statistics                      │
├────────────────────────────────────┤
│      File Footer (324 bytes)        │
├────────────────────────────────────┤
```

**Key Features**:
- Fixed 256KB blocks with configurable size (32KB-1MB)
- Gzip compression (typically 90%+ reduction)
- Sparse timestamp index for fast block discovery
- Inverted indexes for multi-dimensional filtering
- MD5 checksums for corruption detection
- Format versioning for future compatibility

---

### 3. Query Component (internal/storage/)

**Responsibility**: Execute queries with filtering and optimization

**Files**:
- `query.go` - Query executor with multi-file support
- `filters.go` - Filter matching logic (AND semantics)

**Query Execution Flow**:
```
API Request (time window + filters)
    ↓
File Selection (by hour)
    ↓
Block Discovery (by timestamp index)
    ↓
Block Filtering (by inverted indexes)
    ↓ (Skip non-matching blocks)
Decompression (only candidates)
    ↓
Event Filtering (by resource attributes)
    ↓
Result Aggregation
    ↓
Response (events + metrics)
```

**Optimization**:
- **Segment Skipping**: Skip blocks that don't contain matching resources (50%+ reduction)
- **Binary Search**: O(log N) timestamp lookups in sparse index
- **Early Termination**: Stop reading when sufficient results obtained
- **Concurrent Reading**: Parallel file reads for multiple hours

---

### 4. API Component (internal/api/)

**Responsibility**: HTTP interface for queries

**Files**:
- `server.go` - HTTP server setup
- `search_handler.go` - /v1/search endpoint
- `response.go` - Response formatting and metrics
- `validators.go` - Parameter validation
- `errors.go` - Error response formatting

**API Specification**:
```
GET /v1/search

Query Parameters:
  start (required)    : Unix timestamp (start of time window)
  end (required)      : Unix timestamp (end of time window)
  kind (optional)     : Resource kind (e.g., "Pod", "Deployment")
  namespace (optional): Kubernetes namespace
  group (optional)    : API group (e.g., "apps")
  version (optional)  : API version (e.g., "v1")

Response:
  {
    "events": [...],
    "count": 100,
    "executionTimeMs": 45,
    "filesSearched": 24,
    "segmentsScanned": 12,
    "segmentsSkipped": 88
  }
```

---

## Storage Design

### File Organization

```
Data Directory Structure:
data/
├── 2025-11-25T00.bin  (00:00-01:00 UTC)
├── 2025-11-25T01.bin  (01:00-02:00 UTC)
├── 2025-11-25T02.bin  (02:00-03:00 UTC)
└── ... (one file per hour)
```

**Rationale**:
- One file per hour enables efficient time-based queries
- Immutable files after hour completion enable concurrent reads
- Clear namespace prevents file conflicts

### Compression

**Algorithm**: Gzip (via klauspost/compress)

**Performance**:
- Typical reduction: 90%+ (events are highly repetitive)
- Throughput: >100MB/sec compression
- Memory: <1MB overhead per block

**Example**:
```
100K Kubernetes events:
  Uncompressed: 22.44 MB
  Compressed:    1.63 MB
  Ratio:         7.28% (92.72% reduction)
  Savings:      20.81 MB
```

### Indexing Strategy

#### Sparse Timestamp Index

**Purpose**: Fast block discovery by event timestamp

**Structure**:
```
[
  {timestamp: 1700000000, blockOffset: 77},
  {timestamp: 1700000256, blockOffset: 50000},
  {timestamp: 1700000512, blockOffset: 100000}
]
```

**Complexity**: O(log N) via binary search

**Space**: ~100 bytes per block

#### Inverted Indexes

**Purpose**: Skip blocks without matching resources

**Indexes**:
1. Kind → Block IDs (e.g., "Pod" → [0, 2, 5])
2. Namespace → Block IDs (e.g., "default" → [0, 1, 3])
3. Group → Block IDs (e.g., "apps" → [1, 2, 4])

**Query Optimization**:
```
Query: kind=Deployment AND namespace=default
  ↓
Deployment blocks: [0, 1, 3, 4]
default blocks:    [0, 1, 2]
  ↓
Intersection: [0, 1]  (only 2 blocks to decompress!)
  ↓
Skip blocks: 2, 3, 4 (60% reduction)
```

#### Bloom Filters

**Purpose**: Additional false-positive filtering

**Configuration**:
- False positive rate: 5%
- Size: ~18KB per block
- Benefits from SIMD optimization in bits-and-blooms library

---

## Query Execution

### Single File Query

```
File: 2025-11-25T12.bin (12:00-13:00)

1. Read File Header & Footer
2. Load Index Section
   - Sparse timestamp index
   - Inverted indexes
   - Bloom filters

3. Filter by Time Window
   Binary search in timestamp index
   → Find candidate blocks

4. Filter by Resources
   Inverted index intersection
   → Narrow candidate set

5. Decompression
   For each candidate block:
   - Decompress (gzip)
   - Validate checksum (MD5)
   - Parse events (NDJSON)

6. Event Filtering
   For each event:
   - Check namespace
   - Check kind
   - Check group/version

7. Aggregate Results
   - Combine events
   - Count totals
   - Record metrics
```

### Multi-File Query

```
Query: timestamp 2025-11-25 09:00 to 2025-11-25 14:00

Files: 09.bin, 10.bin, 11.bin, 12.bin, 13.bin (5 files)

Parallel Execution:
┌─────────────┬─────────────┬─────────────┬─────────────┬─────────────┐
│  09.bin     │  10.bin     │  11.bin     │  12.bin     │  13.bin     │
│  100 events │  150 events │  120 events │  200 events │  80 events  │
└─────────────┴─────────────┴─────────────┴─────────────┴─────────────┘
                              ↓
                    Aggregate & Sort by Timestamp
                              ↓
                    Return combined results
```

---

## Data Flow

### Write Path (Event → Storage)

```
Kubernetes Event
    ↓
Watcher receives (ADD/UPDATE/DELETE)
    ↓
Event Queue (buffer)
    ↓
Pruning (remove managedFields)
    ↓
Validation (check required fields)
    ↓
Storage Write
    ├─ Accumulate in EventBuffer
    ├─ When full or hourly boundary:
    │   ├─ Create Block
    │   ├─ Compress with gzip
    │   ├─ Create metadata (bloom filters, sets)
    │   ├─ Compute checksum (MD5)
    │   └─ Write to file
    └─
    When hourly boundary:
    ├─ Build inverted indexes
    ├─ Create index section
    ├─ Write file footer
    └─ Seal file (immutable)
```

### Read Path (Query → Results)

```
HTTP API Request
    ↓
Validate parameters
    ↓
Select files (by time window)
    ↓
For each file:
    ├─ Load header/footer
    ├─ Load index section
    ├─ Filter blocks (timestamp + inverted index)
    ├─ Skip non-matching blocks
    ├─ Decompress candidates
    ├─ Validate checksums
    ├─ Filter events
    └─ Aggregate results
    ↓
Combine results from all files
    ↓
Sort by timestamp
    ↓
Format response (JSON)
    ↓
Return to client
```

---

## Performance Characteristics

### Storage Efficiency

| Metric | Value |
|--------|-------|
| Compression ratio | 7-10% (90-93% reduction) |
| Disk I/O | Optimized with block-based read |
| Index size | ~1% of compressed data |
| Bloom filter size | ~18KB per block |

### Query Performance

| Scenario | Latency | Notes |
|----------|---------|-------|
| Single hour (no filters) | <50ms | Load and decompress 1 file |
| Single hour (with filters) | 10-20ms | Segment skipping reduces I/O |
| 24-hour window (no filters) | <500ms | Load 24 files, simple merge |
| 24-hour window (filters) | 100-200ms | Significant block skipping |
| 7-day window | <2s | Parallel file reading |

### Memory Usage

| Component | Memory |
|-----------|--------|
| Base application | ~50MB |
| Per file (loaded) | ~10MB (headers + indexes) |
| Per decompressed block | ~256KB (configurable) |
| Event queue buffer | ~100MB (configurable) |

### Throughput

| Operation | Rate |
|-----------|------|
| Event ingestion | 139K events/sec |
| Compression | >100MB/sec |
| Decompression | >100MB/sec |
| Index lookup | O(log N), <1ms typical |

---

## Scalability Considerations

### Horizontal

The current design is **single-writer, multi-reader**:
- One application instance captures events
- Queries can be handled by multiple replicas (read files)
- File immutability after finalization enables concurrent reads

**Future**: Multi-writer sharding by namespace or resource type

### Vertical

Scaling up a single instance:
- Increase EventBuffer size for higher throughput
- Increase block size for better compression
- Add more CPU for parallel decompression

**Limits**:
- Storage I/O bandwidth (~100MB/sec)
- Network bandwidth (typical 1Gbps uplink)
- Memory for index caching

### Data Retention

Current design:
- No automatic rotation/cleanup
- Operator manages retention policy
- Files can be archived/deleted manually

**Future**: Implement TTL-based automatic cleanup

---

## Deployment Models

### Local Development

```
make run
  ├─ Builds binary
  ├─ Creates ./data directory
  └─ Starts server on :8080
```

### Docker

```
docker build -t k8s-event-monitor:latest .
docker run -p 8080:8080 -v $(pwd)/data:/data k8s-event-monitor:latest
```

### Kubernetes (Helm)

```
helm install k8s-event-monitor ./chart --namespace monitoring
  ├─ Creates ServiceAccount + RBAC
  ├─ Mounts PersistentVolume
  ├─ Exposes via Service
  └─ Configures health checks
```

---

## Future Enhancements

### Short Term (v1.1)

1. **Protobuf Encoding**: More efficient than JSON for storage
2. **Advanced Filtering**: Range queries, regex support
3. **Metrics Export**: Prometheus metrics endpoint
4. **WebUI**: Dashboard for event visualization

### Medium Term (v2.0)

1. **Multi-writer Clustering**: Horizontal scaling
2. **Automatic Rotation**: TTL-based cleanup
3. **S3 Integration**: Cloud storage backend
4. **Event Replay**: Reprocess historical data

### Long Term

1. **Machine Learning**: Anomaly detection
2. **Multi-cluster Federation**: Cross-cluster queries
3. **Real-time Streaming**: WebSocket support
4. **RBAC Integration**: Fine-grained access control

---

## Conclusion

The Kubernetes Event Monitoring System architecture emphasizes:

1. **Reliability**: No event loss, concurrent handling, corruption detection
2. **Performance**: Fast queries via indexing, compression >90%
3. **Simplicity**: Single-writer, file-based, no external dependencies
4. **Operability**: Kubernetes-native, Helm deployable, easy monitoring

The design scales from development to production clusters and provides a foundation for future enhancements.
