# Data Model: Block-based Storage Format

**Feature**: Block-based Storage Format with Advanced Indexing
**Version**: 1.0
**Date**: 2025-11-25

---

## Core Entities

### FileHeader

**Purpose**: Identifies and describes the storage file format, version, and configuration

**Fields**:
- `magic_bytes`: string (exactly "RPKBLOCK" for format identification)
- `format_version`: string (e.g., "1.0" for major.minor versioning)
- `created_at`: int64 (Unix timestamp in nanoseconds)
- `compression_algorithm`: string ("gzip" or "zstd")
- `block_size`: int32 (uncompressed block size limit in bytes)
- `encoding_format`: string ("json" or "protobuf")
- `checksum_enabled`: bool (whether checksums are computed)
- `reserved`: []byte (16 bytes for future extensions)

**Validation Rules**:
- magic_bytes MUST be "RPKBLOCK"
- format_version MUST match reader's supported versions
- created_at MUST be valid Unix timestamp
- compression_algorithm MUST be supported
- block_size MUST be between 32KB and 1MB

**Physical Layout**:
```
Offset  Size  Field
0       8     magic_bytes ("RPKBLOCK")
8       8     format_version (null-terminated string)
16      8     created_at (int64)
24      16    compression_algorithm (fixed string, null-padded)
40      4     block_size (int32)
44      16    encoding_format (fixed string, null-padded)
60      1     checksum_enabled (bool)
61      16    reserved (for future use)
Total: 77 bytes (fixed, must always be at position 0)
```

---

### Block

**Purpose**: Atomic unit of compressed event data with associated metadata

**Fields**:
- `id`: int32 (sequential block number within file, 0-based)
- `offset`: int64 (byte offset in file where block starts)
- `length`: int64 (byte length of compressed data)
- `uncompressed_length`: int64 (byte length before compression)
- `event_count`: int32 (number of events in block)
- `timestamp_min`: int64 (minimum event timestamp in block, nanoseconds)
- `timestamp_max`: int64 (maximum event timestamp in block, nanoseconds)
- `metadata`: BlockMetadata (see below)
- `compressed_data`: []byte (the actual gzip/zstd compressed event payload)

**Validation Rules**:
- id MUST be unique within file
- offset MUST be within file bounds
- length MUST match actual compressed data size
- uncompressed_length MUST be ≥ length (compression constraint)
- event_count MUST be > 0
- timestamp_min MUST be ≤ timestamp_max
- All event timestamps MUST fall within [timestamp_min, timestamp_max]

**State Transitions**:
- BUFFERED (events being added) → COMPRESSED (data compressed) → INDEXED (included in index)
- After INDEXED, block is immutable

**Memory Estimate**:
- Typical: 256KB uncompressed → ~60KB compressed (zstd)
- With metadata overhead: ~60KB + ~18KB metadata = ~78KB per finalized block

---

### BlockMetadata

**Purpose**: Tracks filtering-relevant information about block contents

**Fields**:
- `id`: int32 (same as Block.id for reference)
- `bloom_filter_kinds`: BloomFilter (resources.kinds present in block)
- `bloom_filter_namespaces`: BloomFilter (namespaces present in block)
- `bloom_filter_groups`: BloomFilter (resource groups present in block)
- `kind_set`: []string (exact set of unique kinds, for precise matching)
- `namespace_set`: []string (exact set of unique namespaces)
- `group_set`: []string (exact set of unique groups)
- `checksum`: string (CRC32 hex-encoded if enabled, empty if disabled)

**Validation Rules**:
- kind_set MUST match bloom_filter_kinds (no false negatives)
- namespace_set MUST match bloom_filter_namespaces
- group_set MUST match bloom_filter_groups
- Bloom filters MUST have <5% false positive rate
- checksum MUST be valid CRC32 if computed

**Purpose of Both Sets and Bloom Filters**:
- Bloom filters: Space-efficient filtering (18KB for 10K values), allows probabilistic "might contain"
- Sets: Exact list for definitive queries (used when inverted index available)
- Sets also used as fallback if bloom filter reading fails

---

### BloomFilter

**Purpose**: Probabilistic set membership test for space-efficient filtering

**Fields**:
- `bitset`: []byte (binary representation of bloom filter)
- `size_bits`: int32 (total bits in filter)
- `hash_functions`: int32 (number of hash functions)
- `false_positive_rate`: float32 (configured FP rate, e.g., 0.05 for 5%)

**Characteristics**:
- **Size**: ~0.6 bytes per added element (for 5% FP rate)
- **Operations**: Add(string), Contains(string)
- **Serialization**: bitset is base64-encoded in JSON, raw bytes in binary format

**Example Memory Usage**:
- 10,000 unique values at 5% FP rate: ~6KB
- 3 bloom filters per block (kinds, namespaces, groups): ~18KB total
- With 100 blocks in file: 1.8MB for all bloom filters (negligible)

---

### InvertedIndex

**Purpose**: Maps resource metadata values to candidate blocks for rapid filtering

**Fields**:
- `kind_to_blocks`: map[string][]int32 (kind → list of block IDs)
- `namespace_to_blocks`: map[string][]int32 (namespace → list of block IDs)
- `group_to_blocks`: map[string][]int32 (group → list of block IDs)

**Semantics**:
- kind_to_blocks["Pod"] = [0, 2, 5, 7] means blocks 0, 2, 5, 7 MAY contain Pod events
- Querying for kind="Pod" AND namespace="default":
  - Get candidates for Pod: [0, 2, 5, 7]
  - Get candidates for default: [0, 1, 5, 6]
  - Intersect: [0, 5] (blocks to decompress)

**Validation Rules**:
- Block IDs MUST exist in file
- Entries MUST be exhaustive (every unique value appears at least once)
- Can have false positives (bloom filter fallback), not false negatives

**Corruption Handling**:
- If inverted index corrupted, fall back to linear block scan with bloom filters
- Bloom filters provide guaranteed "all candidates" (no false negatives)

---

### IndexSection

**Purpose**: Collection of metadata and indexes written to end of file for fast access

**Fields**:
- `format_version`: string (matches FileHeader.format_version)
- `block_metadata`: []BlockMetadata (metadata for each block)
- `inverted_indexes`: InvertedIndex (maps values to block IDs)
- `statistics`: IndexStatistics (file-level stats)

**Sub-entity: IndexStatistics**:
- `total_blocks`: int32 (number of blocks in file)
- `total_events`: int64 (sum of all event_count from blocks)
- `total_uncompressed_bytes`: int64 (sum of block uncompressed sizes)
- `total_compressed_bytes`: int64 (sum of block compressed sizes)
- `compression_ratio`: float32 (total_compressed / total_uncompressed)
- `unique_kinds`: int32 (count of unique resource kinds)
- `unique_namespaces`: int32 (count of unique namespaces)
- `unique_groups`: int32 (count of unique groups)
- `timestamp_min`: int64 (earliest event timestamp in file)
- `timestamp_max`: int64 (latest event timestamp in file)

---

### FileFooter

**Purpose**: Enables backward seeking to find index section and validates file integrity

**Fields**:
- `index_section_offset`: int64 (byte offset where IndexSection starts in file)
- `index_section_length`: int32 (byte length of IndexSection)
- `checksum`: string (CRC32 of entire file before footer, if enabled)
- `reserved`: []byte (16 bytes for future extensions)
- `magic_bytes`: string (exactly "RPKEND" for EOF validation)

**Validation Rules**:
- index_section_offset MUST point to valid IndexSection
- index_section_length MUST be > 0
- magic_bytes MUST be "RPKEND"
- checksum MUST validate entire file if enabled

**Physical Layout** (from end of file, backward):
```
Offset From End  Size  Field
-76              8     magic_bytes ("RPKEND")
-68              8     reserved
-60              4     index_section_length (int32)
-56              8     index_section_offset (int64)
-48              256   checksum (if enabled, hex-encoded CRC32, null-padded)
Total: 324 bytes (fixed, must always be at position filesize-324)
```

---

## File Layout (Physical Structure)

Complete file structure from disk perspective:

```
┌─────────────────────────────────┐
│ FileHeader (77 bytes)           │  Offset 0
│  - magic_bytes: "RPKBLOCK"      │
│  - format_version: "1.0"        │
│  - compression: "zstd"          │
│  - etc.                         │
├─────────────────────────────────┤
│ Block 0 Data (256KB uncompressed)
│  ├─ Event 1 (JSON)              │  Offset: header_size
│  ├─ Event 2 (JSON)              │  Compressed size: ~60KB
│  └─ Event N (JSON)              │
├─────────────────────────────────┤
│ Block 1 Data (256KB uncompressed)
│  ├─ Event N+1 (JSON)            │  Offset: header_size + block0_length
│  ├─ Event N+2 (JSON)            │
│  └─ Event M (JSON)              │
├─────────────────────────────────┤
│ ... (more blocks) ...           │
├─────────────────────────────────┤
│ IndexSection (JSON)             │  <-- IndexSection offset stored in footer
│ {                               │
│   "format_version": "1.0",      │
│   "block_metadata": [           │
│     { "id": 0, "bloom_...", ... },
│     { "id": 1, "bloom_...", ... },
│     ...                         │
│   ],                            │
│   "inverted_indexes": {         │
│     "kind_to_blocks": { "Pod": [0, 2, 5], ... },
│     "namespace_to_blocks": { "default": [0, 1], ... },
│     "group_to_blocks": { "apps": [1, 2], ... }
│   },                            │
│   "statistics": { ... }         │
│ }                               │
├─────────────────────────────────┤
│ FileFooter (324 bytes)          │  Offset: filesize - 324
│  - index_section_offset         │
│  - index_section_length         │
│  - checksum (optional)          │
│  - magic_bytes: "RPKEND"        │
└─────────────────────────────────┘
```

---

## Entity Relationships

```
File (hourly, immutable after finalization)
├── FileHeader (at offset 0)
├── 1..N Blocks
│   └── BlockMetadata (for filtering)
│       ├── BloomFilter (kinds)
│       ├── BloomFilter (namespaces)
│       └── BloomFilter (groups)
├── IndexSection (at end)
│   ├── block_metadata[] (copy of metadata from blocks)
│   ├── InvertedIndex
│   │   ├── kind → [block_ids]
│   │   ├── namespace → [block_ids]
│   │   └── group → [block_ids]
│   └── Statistics
└── FileFooter (at offset filesize-324)
    └── Points to IndexSection
```

---

## State Transitions

### File Lifecycle

```
WRITING
  ├─ Events added to current block
  ├─ Block full (256KB) → finalize block, create new one
  └─ Hour boundary reached
       → finalize current block
       → finalize file (atomic)
              → compute inverted indexes
              → write IndexSection
              → write FileFooter
              → mark as FINALIZED

FINALIZED (immutable)
  ├─ Available for queries
  ├─ No writes allowed
  ├─ Multiple concurrent readers supported
  └─ Rotates off (if older than retention policy)
```

### Block Lifecycle

```
BUFFERED
  └─ Events added via AddEvent()
       → timestamp tracking
       → metadata accumulation
       → bloom filter updates

COMPRESSED
  └─ Finalize() called
       → Serialize events to JSON
       → Compress with zstd/gzip
       → Compute checksum (optional)
       → Write to file
       → Create BlockMetadata

INDEXED
  └─ File finalization
       → Include in IndexSection
       → Add to inverted indexes
       → File sealed (immutable)
```

---

## Validation & Constraints

### Per-Block Constraints

- **Size**: Must stay under configured block_size (default 256KB uncompressed)
- **Timestamp ordering**: Events can arrive out-of-order within block, timestamp min/max computed
- **Compression**: Compressed size MUST be ≤ uncompressed size (sanity check)
- **Event count**: Must be > 0 (cannot have empty blocks)

### Per-File Constraints

- **Exactly one file per hour**: File path encodes hour boundary
- **Immutable after finalization**: No appends or modifications after IndexSection written
- **All events valid**: Every event must pass Event.Validate() before storage
- **Ordered finalization**: IndexSection written before FileFooter

### Query Constraints

- **Time range queries**: Can span multiple files (hourly boundaries)
- **Filter queries**: AND logic on filters (kind AND namespace AND group)
- **Empty results valid**: Query returning 0 events is valid (not an error)

---

## Backward Compatibility

### Version 1.0 → 1.1 Compatibility

- Reader checks FileHeader.format_version
- Unknown versions cause error (fail-safe)
- v1.1 additions (e.g., protobuf encoding) are additive fields in IndexSection
- v1.0 files readable by v1.1+ readers (forward compatible)
- v1.1 files NOT readable by v1.0 readers (requires version check)

### Cross-Version Strategy

```go
func ReadFile(path string) (*File, error) {
  header := ReadFileHeader(path)

  switch header.FormatVersion {
    case "1.0": return ReadV1_0File(path)
    case "1.1": return ReadV1_1File(path)
    default: return nil, ErrUnsupportedVersion
  }
}
```

---

## Implementation Notes

### Bloom Filter Serialization

Bloom filters are serialized as:
```json
{
  "size_bits": 65536,
  "hash_functions": 5,
  "bitset": "base64-encoded-binary-data",
  "false_positive_rate": 0.05
}
```

### Inverted Index Serialization

Inverted indexes use block IDs for space efficiency:
```json
{
  "kind_to_blocks": {
    "Pod": [0, 1, 3, 5],
    "Deployment": [1, 2],
    "Service": [0, 4]
  },
  "namespace_to_blocks": {
    "default": [0, 1],
    "kube-system": [2, 3, 4],
    "kube-public": [5]
  },
  "group_to_blocks": {
    "": [0, 1, 3],
    "apps": [2, 4, 5],
    "rbac.authorization.k8s.io": [3]
  }
}
```

### Compression Details

Events in block are serialized as one JSON object per line, then compressed:
```
{"id":"evt-001","timestamp":1000,"type":"CREATE","resource":{...},"data":{...}}
{"id":"evt-002","timestamp":1001,"type":"UPDATE","resource":{...},"data":{...}}
...
```

Then entire block is compressed with zstd (default) or gzip.

---

**Status**: Ready for contract generation and implementation planning
