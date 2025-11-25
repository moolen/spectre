# Feature Specification: Block-based Storage Format with Advanced Indexing

**Feature Branch**: `002-block-storage-format`
**Created**: 2025-11-25
**Status**: Draft
**Input**: Adopt block-based storage architecture with bloom filters, inverted indexes, and explicit file headers for optimized query performance and compression

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Store Events with Optimal Compression (Priority: P1)

An operator wants events to be stored using an advanced block-based format that achieves better compression ratios than the current segment-based approach, enabling them to store more event history on limited disk space while maintaining fast access.

**Why this priority**: Compression efficiency directly impacts operational cost and the volume of historical data available for investigation. Better compression extends data retention windows.

**Independent Test**: Can be fully tested by writing a batch of events to storage files, measuring compression ratios against uncompressed data, and verifying that the stored format can be read back correctly.

**Acceptance Scenarios**:

1. **Given** events are continuously generated, **When** they are written to storage using the block format, **Then** compression achieves at least 50% reduction compared to uncompressed JSON (including both block-level compression and optional protobuf encoding)
2. **Given** a storage file contains 10,000 events, **When** the file is finalized, **Then** blocks of consistent size are created with proper compression applied to each block
3. **Given** events are stored with block-based layout, **When** the file is examined, **Then** a file header is present that identifies the format version and compression algorithm

---

### User Story 2 - Query Events with Rapid Block Filtering (Priority: P1)

An operator needs to search for specific events by resource type, namespace, and API group. They want queries to skip blocks that don't match their criteria without decompressing them, resulting in significantly faster query response times.

**Why this priority**: Query performance directly impacts operational effectiveness. Operators need to quickly diagnose issues across potentially thousands of events.

**Independent Test**: Can be fully tested by creating storage with events from multiple resource types and namespaces, then executing queries with various filter combinations and measuring how many blocks are skipped versus decompressed.

**Acceptance Scenarios**:

1. **Given** a storage file contains events from 10 different resource kinds across 5 namespaces, **When** a query filters for "Deployment in namespace default", **Then** blocks that don't contain Deployments or default namespace events are skipped without decompression
2. **Given** multiple blocks exist in a storage file, **When** inverted indexes are built during file finalization, **Then** queries can directly identify candidate blocks by resource kind, namespace, and API group
3. **Given** a query matches only 5% of total blocks, **When** the query executes, **Then** at least 90% of blocks are skipped without decompression
4. **Given** a query spans multiple storage files, **When** executing, **Then** inverted indexes enable rapid file selection without scanning all files

---

### User Story 3 - Detect Storage Corruption Early (Priority: P2)

An operator running the system in production wants to detect file corruption as early as possible to prevent serving invalid data and to implement recovery strategies proactively.

**Why this priority**: Data integrity is critical for reliability, but secondary to core query functionality. Corruption detection enables operational confidence.

**Independent Test**: Can be fully tested by intentionally corrupting portions of a storage file and verifying that checksums or corruption detection mechanisms identify the issue.

**Acceptance Scenarios**:

1. **Given** a storage file is finalized, **When** optional checksums are computed and stored, **Then** file integrity can be verified before serving results to queries
2. **Given** a block is corrupted during storage, **When** the file is read, **Then** the checksum verification identifies which block(s) are affected, enabling isolation of the problem

---

### User Story 4 - Support Future Format Evolution (Priority: P2)

A developer maintaining the system wants to evolve the storage format in the future (new compression algorithms, index strategies, encoding) without breaking existing production deployments reading old files.

**Why this priority**: Format evolution flexibility is important for long-term system sustainability, but secondary to initial implementation of the new format.

**Independent Test**: Can be fully tested by verifying that the explicit file header contains a version number that allows future readers to handle different format versions appropriately.

**Acceptance Scenarios**:

1. **Given** a storage file is created with format version 1.0, **When** a future version 2.0 reader encounters it, **Then** the version number enables appropriate handling (compatibility or graceful error)
2. **Given** format evolution occurs, **When** files are written, **Then** version information is explicitly encoded in the file header for future readers to check

---

### Edge Cases

- What happens when a block is partially written but the file is not finalized (crash during write)?
- How does the system handle mixed block sizes in legacy files if evolution occurs?
- What occurs if inverted indexes become corrupted during finalization?
- How are queries handled when files are missing or unreadable?
- What happens if file header is missing or malformed?

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST organize event storage into fixed-size blocks (32KB–1MB configurable)
- **FR-002**: System MUST include an explicit file header at the beginning of each storage file containing format version, creation timestamp, and compression algorithm identifier
- **FR-003**: System MUST compress each block independently using a configured algorithm (gzip or zstd)
- **FR-004**: System MUST track per-block metadata including: uncompressed length, compressed length, event count, and bloom filters for resource kinds, namespaces, and API groups
- **FR-005**: System MUST implement bloom filters for each block representing: (a) set of resource kinds in block, (b) set of namespaces in block, (c) set of API groups in block
- **FR-006**: System MUST build inverted indexes during file finalization mapping: (a) resource kind → list of candidate blocks, (b) namespace → list of candidate blocks, (c) API group → list of candidate blocks
- **FR-007**: System MUST write all blocks, metadata, indexes, and footer to the end of the file in a structured index section after all event data
- **FR-008**: System MUST include an explicit footer containing: pointer to index section offset, index section length, optional checksum, and magic bytes for format validation
- **FR-009**: System MUST compute optional checksums (CRC32 or MD5) for corruption detection if enabled
- **FR-010**: System MUST support querying without decompressing blocks that don't match filter criteria, using bloom filters and inverted indexes for rapid elimination
- **FR-011**: System MUST support both JSON and protobuf encoding for events (with JSON as default for human readability)
- **FR-012**: System MUST maintain the existing hourly file organization (one file per hour) from the current architecture
- **FR-013**: System MUST provide backward-compatible query results maintaining the same QueryResult structure

### Key Entities

- **FileHeader**: Metadata at file start identifying format version, creation timestamp, and compression algorithm
- **Block**: Fixed-size unit of compressed event data with associated metadata and bloom filters
- **BlockMetadata**: Information about a block including uncompressed/compressed sizes, event count, and bloom filters
- **BloomFilter**: Probabilistic set for resource kinds, namespaces, and API groups in a block
- **InvertedIndex**: Maps resource kinds/namespaces/API groups to list of candidate blocks for fast filtering
- **IndexSection**: Structured collection of all block metadata, inverted indexes, and offset information
- **FileFooter**: Contains pointers to index section, optional checksum, and magic bytes for validation

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Storage compression achieves at least 50% reduction in total data size compared to uncompressed JSON (from 30% with current segment approach + optional protobuf encoding)
- **SC-002**: Queries skip at least 90% of blocks without decompression when filtering for less than 5% of resource kinds/namespaces in a file
- **SC-003**: Query response time for filtered results remains under 2 seconds for typical time windows (24 hours) on standard hardware
- **SC-004**: File header and footer structure enable format version identification, supporting at least 5 future format versions
- **SC-005**: Optional checksum verification completes in under 100ms for a 100MB storage file
- **SC-006**: Block size remains consistent and configurable (default 256KB), enabling predictable memory usage during decompression
- **SC-007**: Inverted indexes can be built and written during finalization in under 500ms for files with 100,000 events
- **SC-008**: System maintains compatibility with existing query API without requiring client-side changes
- **SC-009**: Bloom filter false positive rate remains below 5% for resource kind/namespace/group filtering
- **SC-010**: Backward compatibility layer correctly handles reading events from current segment-based files during migration (if applicable in future)

## Assumptions

- **Block Size**: Default block size of 256KB uncompressed is reasonable for typical Kubernetes event volumes; configurable per deployment
- **Encoding Default**: JSON encoding is used by default for human readability in operations; protobuf encoding is optional for advanced use cases requiring maximum compression
- **Compression Algorithm**: zstd is preferred over gzip for better compression ratio and streaming support; gzip remains supported for compatibility
- **Bloom Filter False Positive Rate**: 5% false positive rate is acceptable for resource filtering; provides good balance between space efficiency and correctness
- **Query Performance**: Maintaining <2 second query times for 24-hour windows is achievable with block-level filtering on standard hardware (single-instance deployment)
- **Single-writer Architecture**: File finalization is atomic; a single writer controls block creation and index building; multiple concurrent readers are supported after finalization
- **No Backward Compatibility**: Current segment-based storage files are not required to be readable; existing data is not migrated
- **Checksum Optional**: Integrity checking via checksums is optional and configurable; can be disabled if performance is prioritized over corruption detection
- **File Structure**: All structure (blocks, metadata, indexes, footer) written to file sequentially; files are append-only until finalization

## Scope Boundaries

**In Scope**:
- Block-based file format specification with fixed-size blocks and metadata
- Explicit file header and footer structure
- Bloom filters for resource kind/namespace/API group filtering
- Inverted indexes for rapid block candidate selection
- Optional checksums for corruption detection
- Support for both JSON and protobuf event encoding
- Query optimization to skip non-matching blocks
- Format version support for future evolution
- Maintaining existing hourly file organization
- Compatibility with existing query API

**Out of Scope**:
- Backward compatibility with current segment-based storage format
- Data migration from old to new format
- Multi-writer scenarios or distributed consensus
- Automatic compression algorithm selection
- Specialized compression dictionaries (dictionary learning)
- Web UI or visualization of storage structure
- Detailed performance benchmarking against other storage systems
- Custom index types beyond bloom filters and inverted indexes
