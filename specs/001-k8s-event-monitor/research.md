# Phase 0: Research & Findings

**Feature**: Kubernetes Event Monitoring and Storage System
**Date**: 2025-11-25
**Status**: Complete

## Research Tasks Completed

### 1. Kubernetes Watchers Implementation (FR-001, FR-002)

**Decision**: Use `kubernetes.io/client-go` library with informer pattern

**Rationale**: The Kubernetes Go client library is the official, well-maintained library for interacting with Kubernetes APIs. The informer pattern provides event-driven architecture with built-in caching and deduplication, ideal for reliable event capture.

**Alternatives Considered**:
- Direct REST API calls: Would require manual error handling, connection management, and deal with event loss on network failures
- Custom webhook implementation: Requires external accessibility and more complex setup; informers are simpler for in-cluster monitoring

**Implementation Notes**:
- Use `watch.Interface` from client-go for resource change detection
- Implement `ResourceEventHandler` interface to capture ADD/UPDATE/DELETE events
- Handle informer initialization across multiple resource types
- Manage watchers for built-in types (Pod, Deployment, Service, etc.) and CRDs

### 2. Disk-Based Storage Architecture (FR-003, FR-005, FR-006, FR-007)

**Decision**: Implement custom storage engine with hourly files, segment-based organization, and dual-layer indexing

**Rationale**: Custom storage allows precise control over compression, indexing, and query performance. Hourly files provide natural log rotation boundaries. Segment-based organization enables efficient filtering. Dual indexing (sparse timestamp index + segment metadata) provides fast lookups while minimizing I/O.

**Alternatives Considered**:
- Traditional database (PostgreSQL, SQLite): Adds operational complexity, requires connection pooling, slower for high-volume event ingestion; spec requires custom storage implementation
- Cloud object storage (S3): Inconsistent with "disk-based" requirement; higher latency and cost

**Storage Format**:
```
[HOURLY_FILE_YYYYMMDD_HH.bin]
├── [SPARSE_INDEX] → Maps timestamps to segment offsets
├── [SEGMENT_1]
│   ├── [METADATA_INDEX] → g/v/k/n presence bitmap
│   └── [COMPRESSED_DATA]
├── [SEGMENT_2]
│   ├── [METADATA_INDEX]
│   └── [COMPRESSED_DATA]
└── [FILE_FOOTER] → Index offsets and metadata
```

### 3. Compression Strategy (FR-003, FR-017)

**Decision**: Use github.com/klauspost/compress library with gzip format for segment data

**Rationale**: Klauspost's compress library is high-performance, well-tested, and includes multiple compression algorithms. Gzip provides good balance of compression ratio and decompression speed. Library specified in requirements.

**Compression Details**:
- Compress data at segment boundaries (chunk events into segments, compress each segment)
- Target: ≥30% compression ratio (typical for JSON event data)
- Decompress on-demand during queries (not cached in memory)

### 4. Multi-Dimensional Indexing Strategy (FR-006, FR-007, FR-013)

**Decision**: Implement two-level indexing approach:
1. **Sparse Timestamp Index**: Maps timestamp ranges to segment offsets within file
2. **Segment Metadata Index**: Bitmap/summary of which g/v/k/namespaces exist in segment

**Rationale**: Enables efficient query execution without full segment scans. Sparse index handles temporal filtering. Metadata index enables resource filtering at segment level, skipping irrelevant segments.

**Implementation**:
- Timestamp index: Sorted array of [timestamp, offset] pairs; one entry per segment or fixed interval
- Metadata index: Bloom filter or set-based summary of unique (group, version, kind, namespace) tuples in segment
- Both indexes stored in file footer for quick access

### 5. Query API Design (FR-008, FR-009, FR-010, FR-011, FR-012)

**Decision**: RESTful HTTP API with `/v1/search` endpoint accepting filter parameters

**Rationale**: Simple, standard HTTP interface matches specification requirements. Query parameters provide intuitive filtering. Synchronous request/response pattern specified in assumptions.

**API Design**:
```
GET /v1/search?start=<unix_ts>&end=<unix_ts>&kind=<k>&namespace=<ns>&group=<g>&version=<v>
```

- Query logic: Load sparse index → identify candidate segments → load metadata → apply filters → decompress matching segments → apply event-level filters
- Response: JSON array of matching events with event metadata and resource data

### 6. Data Pruning Strategy (FR-004)

**Decision**: Remove `metadata.managedFields` from events before storage

**Rationale**: managedFields is a Kubernetes internal field tracking field ownership; not needed for event history analysis. Removing it reduces data size by 10-20% typically.

**Implementation**:
- Process events during capture phase: delete event.Object.Metadata.ManagedFields
- Same for DeletedState events

### 7. Event Loss Prevention and Ordering (Edge Cases)

**Decision**:
- Use informer pattern with resync period to catch missed events
- Store events with received timestamp, not event.CreationTimestamp
- Handle out-of-order events with time-window-based buffering for cross-segment writes

**Rationale**: Informer pattern ensures reliability. Received timestamp reflects when monitoring system captured event. Time-window buffering prevents segment boundaries from causing event ordering issues.

### 8. Cross-File Query Handling (FR-014)

**Decision**: Query executor opens and searches multiple hourly files based on time window

**Rationale**: Queries naturally span hour boundaries. Iterator pattern allows efficient multi-file handling without loading all into memory.

**Implementation**: Time-based file discovery, sequential/parallel file processing based on time window.

### 9. Concurrent Request Handling (Edge Cases)

**Decision**: Use read-write locks on storage directory; individual segment reads are thread-safe

**Rationale**: Multiple API requests can safely read simultaneously. Writes are serialized by watcher goroutine. No lock contention expected for typical queries.

## Technology Stack Summary

| Component | Technology | Reason |
|-----------|-----------|--------|
| Language | Go 1.21+ | Efficient concurrency, cloud-native standard |
| K8s API | client-go | Official Kubernetes library |
| Compression | klauspost/compress | Specified, high-performance |
| Storage | Custom (disk files) | Specification requirement |
| HTTP API | net/http | Lightweight, standard library |
| Testing | Go testing + integration | Standard Go practices |
| Deployment | Helm | Kubernetes standard IaC |
| Build | Make | Simple, universal automation |

## Performance Targets Validation

| Target | Approach | Feasibility |
|--------|----------|------------|
| <5s event capture latency | Informer event handler in separate goroutine | High |
| <2s query response (24h) | Sparse indexing + segment metadata filtering | High |
| ≥30% compression | Klauspost gzip on JSON data | High (typical: 40-50%) |
| 1000+ events/min sustained | Buffered segment writes, async storage | High |
| 10GB/month for 100K events/day | ~100 bytes/event compressed | Achievable with field pruning |

## Known Limitations & Future Considerations

1. **Single Instance Only**: Specification defines single-instance monitoring; clustering deferred
2. **Local Disk Required**: No distributed storage support; assumes single pod with persistent volume
3. **Event Retention**: Manual cleanup/rotation expected; no automatic deletion policies
4. **No Auth**: API is unauthenticated; assumes internal cluster access only
5. **Memory Usage**: Entire segments loaded during queries; streaming not implemented

## Next Steps

This research completes Phase 0. Ready to proceed to Phase 1 (Design & Contracts) with:
- data-model.md: Entity definitions and relationships
- contracts/search-api.openapi.yaml: API specification
- quickstart.md: Development setup guide
