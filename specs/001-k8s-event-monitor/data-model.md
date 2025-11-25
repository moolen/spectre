# Phase 1: Data Model & Entities

**Feature**: Kubernetes Event Monitoring and Storage System
**Date**: 2025-11-25
**Status**: Complete

## Core Entities

### Event

**Purpose**: Represents a single Kubernetes resource change (CREATE, UPDATE, or DELETE)

**Fields**:
- `id`: string (unique identifier, e.g., UUID)
- `timestamp`: int64 (Unix nanoseconds when event was captured by monitor)
- `type`: enum (CREATE | UPDATE | DELETE)
- `resource`: ResourceMetadata (see below)
- `data`: object (full Kubernetes resource object with managedFields removed)
- `dataSize`: int32 (original uncompressed size in bytes)
- `compressedSize`: int32 (compressed size in bytes)

**Validation Rules**:
- timestamp must be valid Unix time (non-negative)
- type must be one of three valid values
- resource must have all required fields populated
- data must be valid JSON
- dataSize ≥ compressedSize (compression constraint)

**State Transitions**:
- Events are immutable once stored
- No updates or deletes of individual events (append-only log)

### ResourceMetadata

**Purpose**: Identifies a Kubernetes resource and provides filter dimensions

**Fields**:
- `group`: string (API group, e.g., "apps", "" for core)
- `version`: string (API version, e.g., "v1", "v1beta1")
- `kind`: string (resource kind, e.g., "Deployment", "Pod")
- `namespace`: string (Kubernetes namespace, "" for cluster-scoped)
- `name`: string (resource name)
- `uid`: string (unique identifier within cluster)

**Validation Rules**:
- All fields must be non-empty strings
- group, version, kind must match Kubernetes naming conventions
- namespace can be empty for cluster-scoped resources
- name and uid must be valid per Kubernetes spec

**Query Dimensions**:
- Used in filtering: can query by any combination of group/version/kind/namespace
- Form composite index key for segment metadata

### StorageSegment

**Purpose**: Atomic unit of compressed event data within a storage file

**Fields**:
- `id`: int32 (sequential segment number within file)
- `startTimestamp`: int64 (minimum event timestamp in segment)
- `endTimestamp`: int64 (maximum event timestamp in segment)
- `eventCount`: int32 (number of events in segment)
- `uncompressedSize`: int64 (total uncompressed bytes)
- `compressedSize`: int64 (total compressed bytes)
- `offset`: int64 (byte offset in file where segment starts)
- `length`: int64 (byte length of compressed segment)
- `metadata`: SegmentMetadata (see below)

**Validation Rules**:
- startTimestamp ≤ endTimestamp
- eventCount ≥ 1
- uncompressedSize ≥ compressedSize
- offset + length must be within file bounds
- All timestamps within segment must fall within [startTimestamp, endTimestamp]

**State Transitions**:
- WRITING → FINALIZED (immutable after completion)

### SegmentMetadata

**Purpose**: Enables efficient filtering by allowing queries to skip entire segments

**Fields**:
- `resourceSummary`: set of (group, version, kind, namespace) tuples present in segment
- `minTimestamp`: int64 (earliest event timestamp)
- `maxTimestamp`: int64 (latest event timestamp)
- `namespaceSet`: set of unique namespaces in segment
- `kindSet`: set of unique kinds in segment
- `compressionAlgorithm`: string (e.g., "gzip")

**Validation Rules**:
- All sets must be non-empty (segment contains at least one unique value per dimension)
- minTimestamp ≤ all event timestamps ≤ maxTimestamp
- All kinds/namespaces must exactly match events in segment

**Query Optimization**:
- Metadata is checked before decompressing segment
- If query filters for kind="Node" and kindSet doesn't contain "Node", segment is skipped entirely
- Same for namespace, group, version filters

### HourlyStorageFile

**Purpose**: Container for all events captured within a one-hour window

**Fields**:
- `path`: string (file path on disk, e.g., "/data/2025-11-25-14.bin")
- `hourTimestamp`: int64 (Unix timestamp of hour boundary)
- `segments`: array of StorageSegment
- `sparseIndex`: SparseTimestampIndex (see below)
- `metadata`: FileMetadata (see below)

**Validation Rules**:
- Exactly one file per calendar hour
- All events in file must have timestamps within [hourTimestamp, hourTimestamp + 3600 seconds)
- Files are write-once: once completed, never modified

**File Lifecycle**:
1. WRITING: Current hour, events being written
2. FINALIZED: Hour complete, indexes built, file closed
3. ARCHIVED: Available for queries (may be compressed for long-term storage - future)

### SparseTimestampIndex

**Purpose**: Maps timestamp ranges to segment offsets for fast temporal filtering

**Fields**:
- `entries`: array of IndexEntry (sorted by timestamp)
- `totalSegments`: int32 (number of segments in file)

**IndexEntry**:
- `timestamp`: int64 (representative timestamp)
- `segmentId`: int32 (points to segment with events near this time)
- `offset`: int64 (byte offset of segment in file)

**Index Strategy**:
- Entry per segment (exact: one entry per segment boundary)
- Binary search finds candidate segments for time window
- Enables O(log N) segment discovery

### FileMetadata

**Purpose**: Stores information about entire hourly file for quick access

**Fields**:
- `createdAt`: int64 (Unix timestamp when file created)
- `finalizedAt`: int64 (Unix timestamp when hour completed)
- `totalEvents`: int64 (total events in file)
- `totalUncompressedBytes`: int64 (sum of all events' uncompressed sizes)
- `totalCompressedBytes`: int64 (sum of all events' compressed sizes)
- `compressionRatio`: float32 (totalCompressedBytes / totalUncompressedBytes)
- `resourceTypes`: set of unique kinds in file
- `namespaces`: set of unique namespaces in file

**Validation Rules**:
- finalizedAt ≥ createdAt
- totalEvents ≥ 0 (file may be empty)
- compressionRatio between 0.0 and 1.0
- Resource/namespace sets must match actual file contents

### QueryRequest

**Purpose**: Represents an API query for historical events

**Fields**:
- `startTimestamp`: int64 (Unix timestamp, inclusive)
- `endTimestamp`: int64 (Unix timestamp, inclusive)
- `filters`: QueryFilters (see below)

**Validation Rules**:
- startTimestamp ≤ endTimestamp
- Both timestamps must be valid Unix times
- Filters are optional (empty filters = return all)

### QueryFilters

**Purpose**: Specifies which events to return based on resource dimensions

**Fields**:
- `group`: string (optional, "" means match all)
- `version`: string (optional, "" means match all)
- `kind`: string (optional, "" means match all)
- `namespace`: string (optional, "" means match all)

**Matching Rules**:
- All specified filters must match (AND logic)
- Empty string = wildcard (match all)
- If namespace is specified alone, return all resources in that namespace regardless of g/v/k
- If g/v/k are specified without namespace, return matches from all namespaces

### QueryResult

**Purpose**: Response containing matching events from a query

**Fields**:
- `events`: array of Event
- `count`: int32 (number of events)
- `executionTimeMs`: int32 (query execution duration)
- `segmentsScanned`: int32 (total segments examined)
- `segmentsSkipped`: int32 (segments skipped via metadata filtering)
- `filesSearched`: int32 (number of hourly files searched)

**Validation Rules**:
- events.length == count
- All events must match query filters and time window
- executionTimeMs ≥ 0
- segmentsScanned + segmentsSkipped = total segments examined

## Entity Relationships

```
HourlyStorageFile
├── contains 1..N StorageSegment
│   └── contains SegmentMetadata
│       └── references N ResourceMetadata (unique tuples)
│           └── used by 1..N Event
├── contains SparseTimestampIndex
└── contains FileMetadata

QueryRequest
├── references QueryFilters
└── matched by Event (via ResourceMetadata)

QueryResult
└── contains 0..N Event (matching filters)
```

## Storage Layout on Disk

```
/data/
├── 2025-11-25-00.bin    # Events from 2025-11-25 00:00:00-01:00:00 UTC
├── 2025-11-25-01.bin    # Events from 2025-11-25 01:00:00-02:00:00 UTC
├── 2025-11-25-02.bin
...
└── 2025-11-26-00.bin    # Most recent hour

Each file internal structure:
[SEGMENT_1_COMPRESSED_DATA] [SEGMENT_2_COMPRESSED_DATA] ... [SPARSE_INDEX] [FILE_METADATA] [FILE_FOOTER]
```

## Index Optimization Notes

1. **Segment Metadata Index** (bloom filter or set):
   - Allows O(1) checking if resource type exists in segment
   - Skips entire segment decompression if filter doesn't match

2. **Sparse Timestamp Index** (sorted array):
   - Binary search: O(log N) to find candidate segments
   - Enables queries to skip segments entirely outside time window

3. **File Metadata Index** (in-memory):
   - Quick lookup of which files to search (index by hour)
   - Allows skipping entire files if they're outside time window

## Compression Details

- **Algorithm**: gzip (via klauspost/compress)
- **Unit**: Per-segment compression
- **Format**: Raw gzip stream, no wrapper
- **Target**: ≥30% compression ratio

Example:
```
Uncompressed: [Event1 JSON] [Event2 JSON] [Event3 JSON] ...
↓ compress
Compressed: [gzip header] [compressed data...] [gzip footer]
↓ store with metadata
Segment: [compressed_data] {metadata: {event_count: 3, uncompressed_size: XXX, compressed_size: YYY}}
```

## Version & Evolution

**Current Version**: 1.0

**Backward Compatibility Considerations**:
- File format: Use versioning in file header for future compatibility
- API: Use semantic versioning for endpoint changes
- Entity fields: Mark additions as optional in queries, required fields in updates

## Next Steps

Ready for Phase 1 contract generation:
- OpenAPI specification for /v1/search endpoint
- Quickstart guide for setup and development
