# Implementation Plan: [FEATURE]

**Branch**: `[###-feature-name]` | **Date**: [DATE] | **Spec**: [link]
**Input**: Feature specification from `/specs/[###-feature-name]/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Replace the current segment-based storage format with an advanced block-based format featuring fixed-size blocks (256KB default), bloom filters for multi-dimensional filtering, inverted indexes for rapid block selection, and explicit file headers supporting format evolution. This achieves 50%+ compression (vs 30% current) and 90%+ block skipping for filtered queries (vs 50-70% current), while maintaining existing API compatibility and hourly file organization.

## Technical Context

**Language/Version**: Go 1.21+
**Primary Dependencies**: github.com/klauspost/compress (existing), new: bloom filter library (e.g., spaolac/bloom), proto compiler for optional protobuf support
**Storage**: Binary file format (custom block-based structure), local filesystem with hourly rotation
**Testing**: Go's standard testing framework (existing), new: block format validation tests, bloom filter tests, inverted index tests
**Target Platform**: Linux server (Kubernetes deployment)
**Project Type**: Single Go application (extends rpk CLI)
**Performance Goals**: 50%+ compression, 90%+ block skip rate for selective queries, <2s query response (24-hour windows), <500ms index finalization
**Constraints**: Single-writer/multi-reader, atomic file finalization, zero data loss, existing QueryResult API contract maintained
**Scale/Scope**: Kubernetes audit events (~1000+ events/minute), files up to 100GB+, queries spanning multiple hourly files

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Compliance Status**: No constitution violations. This feature extends existing rpk storage system and maintains all established patterns.

**Verification**:
- ✅ Extends existing single Go project (no new projects)
- ✅ Maintains existing internal/storage package structure
- ✅ Continues test-first approach (unit + integration tests)
- ✅ Maintains compatibility with existing models and API contracts
- ✅ Uses established dependency (klauspost/compress), adds only lightweight bloom filter library

## Project Structure

### Documentation (this feature)

```text
specs/002-block-storage-format/
├── spec.md              # Feature specification (completed)
├── plan.md              # This file (in progress)
├── research.md          # Phase 0 output (TBD)
├── data-model.md        # Phase 1 output (TBD)
├── quickstart.md        # Phase 1 output (TBD)
├── contracts/           # Phase 1 output (TBD)
├── tasks.md             # Phase 2 output (TBD - from /speckit.tasks)
└── checklists/          # Quality gates
    └── requirements.md
```

### Source Code (existing structure - no new packages)

**Modified/Extended Packages**:
```text
internal/storage/
├── file.go              # Modify for new block-based format
├── segment.go           # Refactor to Block with metadata
├── compression.go       # Extend for block-level compression
├── index.go             # Enhance with inverted indexes
├── query.go             # Update for block filtering + bloom filters
├── filter.go            # Extend with bloom filter evaluation
└── block_format.go      # NEW: File format reader/writer

tests/unit/storage/
├── block_format_test.go # NEW: Block format validation
├── bloom_filter_test.go # NEW: Bloom filter tests
├── inverted_index_test.go # NEW: Inverted index tests
└── [existing tests adapted for new format]

tests/integration/
├── block_storage_test.go # NEW: End-to-end format tests
└── query_block_format_test.go # NEW: Query with block filtering
```

**Structure Decision**: Single-package extension within existing `internal/storage`. No new packages required. All changes confined to storage layer and corresponding tests. Maintains compatibility with existing watcher, API, and query layers through interface contracts.

## Complexity Tracking

> No constitution violations or complexity justifications needed

---

## Phase 0: Research & Analysis

**Gate Status**: ✅ PASSED - No NEEDS CLARIFICATION markers in spec. All design decisions are clear.

**Research Topics** (parallel investigation):

1. **Bloom Filter Library Selection**
   - Evaluate: github.com/spaolac/bloom vs golang-set alternatives
   - Goal: <1MB memory overhead for 10K unique values, <5% false positive rate
   - Decision: Use spaolac/bloom (battle-tested, minimal dependencies)

2. **Protobuf Optional Support**
   - Research: proto3 schema for Kubernetes events subset
   - Goal: 20-30% size reduction vs JSON baseline
   - Decision: Protobuf optional (JSON default for v1.0, protobuf in v1.1)

3. **zstd vs gzip Trade-offs**
   - Compression ratio, speed, streaming support, dictionary learning
   - Goal: Inform default algorithm choice
   - Decision: zstd for new files, gzip backwards-compatible reader

4. **Checksum Algorithm**
   - CRC32 vs MD5 vs xxHash trade-offs
   - Goal: Balance speed (<100ms) with collision resistance
   - Decision: CRC32 for checksums (fast, sufficient for storage), optional MD5

5. **Concurrent Reader Safety**
   - Memory mapping vs buffered I/O strategies
   - Goal: Support 10+ concurrent query readers per file
   - Decision: Buffered I/O with file-level read lock on index section

**Output**: research.md (consolidates all findings with decisions and rationales)

---

## Phase 1: Design & Contracts

### 1.1 Data Model (`data-model.md`)

**Entities to define**:
- `FileHeader`: Version, creation timestamp, compression algorithm, magic bytes
- `Block`: Event batch with compression metadata and bloom filters
- `BlockMetadata`: Uncompressed/compressed sizes, event count, bloom filters for 3 dimensions
- `BloomFilter`: Resource kinds, namespaces, API groups (serialized bitsets)
- `InvertedIndex`: Kind→blocks, namespace→blocks, group→blocks mappings
- `IndexSection`: All block metadata + inverted indexes + offsets
- `FileFooter`: Index section pointer, checksum, magic bytes

**Relationships**:
- File contains 1..N Blocks
- File contains 1 IndexSection (at end)
- IndexSection contains block metadata + inverted indexes
- Inverted indexes reference blocks by ID

**State Transitions**:
- File: WRITING → FINALIZED (immutable after)
- Block: BUFFERED → COMPRESSED → INDEXED

### 1.2 API Contracts (`contracts/`)

**No public API changes** - block format is internal to storage layer. Existing endpoints maintain compatibility:

- `QueryRequest` → `QueryResult` interface unchanged
- Internal storage interfaces:
  - `StorageFile.WriteEvent()` → same signature, new block-based implementation
  - `QueryExecutor.Execute()` → same signature, new block filtering strategy
  - `NewStorageFile()` → same signature, creates block-based file

**Internal Contracts**:
- `Block interface`: AddEvent(), Finalize(), GetMetadata(), GetBloomFilters()
- `BloomFilter interface`: Add(string), Contains(string), FalsePositiveRate()
- `InvertedIndex interface`: GetCandidateBlocks(kind/namespace/group)

### 1.3 Quickstart (`quickstart.md`)

Developer guide covering:
- Block format overview (visual layout)
- File structure walkthrough
- Bloom filter tuning (block size, FP rate)
- Performance benchmarking setup
- Integration testing patterns

### 1.4 Agent Context Update

Run `.specify/scripts/bash/update-agent-context.sh claude` to update context with:
- New bloom filter library (spaolac/bloom)
- Block-based storage terminology
- Inverted index patterns
- File format versioning approach

---

## Phase 2: Task Generation

**Prerequisite**: Phase 0 (research.md) and Phase 1 (data-model.md, contracts/, quickstart.md) complete

**Next Command**: `/speckit.tasks` generates `tasks.md` with implementation steps:
- Block format writer implementation
- Bloom filter integration
- Inverted index builder
- Query executor updates
- File reader/validation
- Unit tests
- Integration tests
- Migration strategy (data migration not required per spec)

---

## Implementation Priority

**Critical Path** (must complete first for working storage):
1. Block format writer (file structure, compression)
2. Block format reader (deserialization)
3. Query executor updates (block filtering)
4. Basic tests (can write/read blocks)

**High Priority** (enables performance goals):
5. Bloom filters (multi-dimensional filtering)
6. Inverted indexes (rapid block candidate selection)
7. Performance tests (validate 50% compression, 90% skip rate)

**Nice-to-Have** (post-v1.0):
8. Protobuf encoding option
9. Checksum validation
10. File format migration tools
