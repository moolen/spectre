# Implementation Tasks: Block-based Storage Format with Advanced Indexing

**Feature**: Block-based Storage Format with Advanced Indexing
**Branch**: `002-block-storage-format`
**Date**: 2025-11-25
**Status**: Ready for Implementation (Phase 2 Output)

---

## Overview

This document contains all implementation tasks organized by user story and phase. Tasks are designed to be independently executable and testable. The critical path prioritizes P1 user stories (compression and query performance) before P2 enhancements (corruption detection and format evolution).

**Total Tasks**: 45
**Phase Breakdown**:
- Phase 1 (Setup): 3 tasks
- Phase 2 (Foundational): 8 tasks
- Phase 3 (US1 - Compression): 12 tasks
- Phase 4 (US2 - Query Performance): 12 tasks
- Phase 5 (US3 - Corruption Detection): 6 tasks
- Phase 6 (US4 - Format Evolution): 2 tasks
- Phase 7 (Polish & Integration): 2 tasks

**Parallel Opportunities**:
- Phase 1: All 3 tasks can run in parallel (independent setup)
- Phase 2: Tasks T004-T011 can mostly run in parallel (after T004)
- Phase 3: T012-T023 can run in parallel per component
- Phase 4: T024-T035 can run in parallel per component

---

## Phase 1: Setup & Project Initialization

- [x] T001 Add spaolac/bloom dependency to go.mod: `go get github.com/spaolac/bloom` (Using bits-and-blooms/bloom/v3 instead - more actively maintained)
- [x] T002 Update .specify/scripts/bash/update-agent-context.sh to document block-based storage terminology and bloom filter library
- [x] T003 Create docs/BLOCK_FORMAT_REFERENCE.md as operational reference for storage format (copy from quickstart.md)

---

## Phase 2: Foundational Infrastructure

These tasks establish shared infrastructure and core interfaces needed by all user stories.

- [x] T004 Define BloomFilter interface in internal/storage/filter.go with methods: Add(string), Contains(string), FalsePositiveRate() float32, Serialize() ([]byte, error), Deserialize([]byte) error
- [x] T005 [P] Implement BloomFilter using spaolac/bloom in internal/storage/filter.go with configurable false positive rate (default 5%)
- [x] T006 [P] Create Block data structure in internal/storage/block.go with fields: ID, Offset, Length, UncompressedLength, EventCount, TimestampMin, TimestampMax, CompressedData, Metadata
- [x] T007 [P] Create BlockMetadata structure in internal/storage/block.go with fields: ID, BloomFilterKinds, BloomFilterNamespaces, BloomFilterGroups, KindSet, NamespaceSet, GroupSet, Checksum
- [x] T008 Create FileHeader structure in internal/storage/block_format.go with fields: MagicBytes("RPKBLOCK"), FormatVersion, CreatedAt, CompressionAlgorithm, BlockSize, EncodingFormat, ChecksumEnabled, Reserved
- [x] T009 Create FileFooter structure in internal/storage/block_format.go with fields: IndexSectionOffset, IndexSectionLength, Checksum, Reserved, MagicBytes("RPKEND")
- [x] T010 Create IndexSection structure in internal/storage/block_format.go with fields: FormatVersion, BlockMetadata[], InvertedIndexes, Statistics
- [x] T011 Create InvertedIndex structure in internal/storage/block_format.go with fields: KindToBlocks map[string][]int32, NamespaceToBlocks map[string][]int32, GroupToBlocks map[string][]int32

---

## Phase 3: User Story 1 - Store Events with Optimal Compression (P1)

**Story Goal**: Events are stored using fixed-size blocks with compression, achieving 50%+ compression ratios.

**Independent Test**: Write batch of events, measure compression, read back and verify format.

**Acceptance Criteria**:
- Compression achieves at least 50% reduction vs uncompressed JSON
- File header is present identifying format and compression algorithm
- Blocks of consistent size are created with proper compression

### 3.1 Block Format Writer Implementation

- [ ] T012 [US1] Implement FileHeader serialization in internal/storage/block_format.go: WriteFileHeader(w io.Writer, header *FileHeader) error (77 bytes fixed)
- [ ] T013 [US1] Implement FileFooter serialization in internal/storage/block_format.go: WriteFileFooter(w io.Writer, footer *FileFooter) error (324 bytes fixed)
- [ ] T014 [US1] Implement Block compression in internal/storage/block.go: CompressBlock(block *Block, algorithm string) (*Block, error) using klauspost/compress with zstd default
- [ ] T015 [US1] Implement EventBuffer in internal/storage/block.go for accumulating events with methods: AddEvent(event Event), IsFull(blockSize) bool, Finalize(algorithm string) (*Block, error)
- [ ] T016 [US1] Implement BlockWriter in internal/storage/block_format.go: WriteBlock(w io.Writer, block *Block, offset int64) (*Block, error) that tracks offsets and writes compressed data
- [ ] T017 [US1] Implement IndexSection serialization in internal/storage/block_format.go: WriteIndexSection(w io.Writer, section *IndexSection) error with JSON encoding
- [ ] T018 [US1] Modify internal/storage/file.go: Update StorageFile.WriteEvent() to accumulate events in EventBuffer instead of Segment
- [ ] T019 [US1] Modify internal/storage/file.go: Update StorageFile.Finalize() to flush EventBuffer, create blocks, compress them, write IndexSection and FileFooter
- [ ] T020 [US1] Add compression ratio calculation in internal/storage/block_format.go: GetCompressionMetrics(file *File) (ratio float32, uncompressed, compressed int64)
- [ ] T021 [P] [US1] Write unit tests in tests/unit/storage/block_format_test.go: TestWriteFileHeader, TestWriteFileFooter, TestBlockCompression, TestEventBuffer (4 tests)
- [ ] T022 [P] [US1] Write unit tests in tests/unit/storage/block_writer_test.go: TestWriteBlock, TestIndexSectionSerialization (2 tests)
- [ ] T023 [US1] Write integration test in tests/integration/block_storage_write_test.go: TestWriteReadRoundTrip that writes 10K events, measures compression (target 50%+), reads back and verifies

---

## Phase 4: User Story 2 - Query Events with Rapid Block Filtering (P1)

**Story Goal**: Queries skip 90%+ of blocks without decompression using inverted indexes and bloom filters.

**Independent Test**: Create storage with mixed resource types/namespaces, execute queries, measure block skip rate.

**Acceptance Criteria**:
- Inverted indexes built during file finalization identify candidate blocks
- Queries skip at least 90% of blocks when filtering for <5% selectivity
- Query execution on 24-hour windows completes in <2 seconds

### 4.1 Bloom Filter Integration

- [ ] T024 [US2] Implement bloom filter creation in internal/storage/block.go: CreateBlockBloomFilters(events []Event) (*BlockMetadata, error) for kinds, namespaces, groups
- [ ] T025 [US2] Implement bloom filter query in internal/storage/filter.go: QueryBloomFilters(metadata *BlockMetadata, filters QueryFilters) bool for filtering candidate blocks
- [ ] T026 [P] [US2] Write unit tests in tests/unit/storage/bloom_filter_test.go: TestBloomFilterAdd, TestBloomFilterContains, TestFalsePositiveRate (3 tests)

### 4.2 Inverted Index Building

- [ ] T027 [US2] Implement InvertedIndex builder in internal/storage/block_format.go: BuildInvertedIndexes(blocks []BlockMetadata) (*InvertedIndex, error)
- [ ] T028 [US2] Implement block candidate selection in internal/storage/block_format.go: GetCandidateBlocks(index *InvertedIndex, filters QueryFilters) []int32 with intersection logic
- [ ] T029 [P] [US2] Write unit tests in tests/unit/storage/inverted_index_test.go: TestBuildInvertedIndexes, TestGetCandidateBlocks, TestIntersectionLogic (3 tests)

### 4.3 Block Format Reader Implementation

- [ ] T030 [US2] Implement FileHeader deserialization in internal/storage/block_format.go: ReadFileHeader(r io.Reader) (*FileHeader, error)
- [ ] T031 [US2] Implement FileFooter deserialization in internal/storage/block_format.go: ReadFileFooter(r io.Reader) (*FileFooter, error) with backward seeking
- [ ] T032 [US2] Implement IndexSection deserialization in internal/storage/block_format.go: ReadIndexSection(r io.Reader, offset int64, length int32) (*IndexSection, error)
- [ ] T033 [US2] Implement Block decompression in internal/storage/block.go: DecompressBlock(block *Block, algorithm string) ([]byte, error) with gzip/zstd support
- [ ] T034 [US2] Implement BlockReader in internal/storage/block_format.go: ReadBlock(f *os.File, metadata BlockMetadata) (*Block, error) with compression validation

### 4.4 Query Executor Integration

- [ ] T035 [US2] Modify internal/storage/query.go: UpdateQueryExecutor.Execute() to use inverted indexes for block selection, decompress candidates, apply filters
- [ ] T036 [P] [US2] Write unit tests in tests/unit/storage/block_reader_test.go: TestReadFileHeader, TestReadFileFooter, TestDecompressBlock (3 tests)
- [ ] T037 [US2] Write integration test in tests/integration/block_storage_query_test.go: TestQueryBlockFiltering that creates file with 10 kinds × 5 namespaces, executes query, verifies 90%+ skip rate
- [ ] T038 [US2] Write performance test in tests/integration/block_storage_perf_test.go: BenchmarkQueryPerformance measuring 24-hour window queries (target <2s)

---

## Phase 5: User Story 3 - Detect Storage Corruption Early (P2)

**Story Goal**: Optional checksums detect file corruption, isolated to affected blocks.

**Independent Test**: Intentionally corrupt portions, verify detection, ensure isolation.

**Acceptance Criteria**:
- Optional checksums computed and stored during finalization
- Corrupted blocks identified, other blocks remain queryable
- Checksum verification completes in <100ms for 100MB files

### 5.1 Checksum Implementation

- [ ] T039 [US3] Implement CRC32 checksum computation in internal/storage/block_format.go: ComputeChecksum(data []byte) string using stdlib crc32
- [ ] T040 [US3] Add checksum calculation to BlockMetadata and FileFooter structures (optional fields)
- [ ] T041 [US3] Implement checksum validation in internal/storage/block_format.go: VerifyBlockChecksum(block *Block, metadata BlockMetadata) error
- [ ] T042 [US3] Implement file checksum validation in internal/storage/block_format.go: VerifyFileChecksum(footer *FileFooter, f *os.File) error
- [ ] T043 [P] [US3] Write unit tests in tests/unit/storage/checksum_test.go: TestComputeChecksum, TestVerifyBlockChecksum, TestVerifyFileChecksum (3 tests)
- [ ] T044 [US3] Write integration test in tests/integration/block_storage_corruption_test.go: TestCorruptionDetection that intentionally corrupts block, verifies detection and isolation

---

## Phase 6: User Story 4 - Support Future Format Evolution (P2)

**Story Goal**: Version field in file header enables future format changes.

**Independent Test**: Verify version identification allows handling different format versions.

**Acceptance Criteria**:
- File header contains explicit version number (major.minor)
- Future reader can check version and handle appropriately
- Support documented for at least 5 future versions

### 6.1 Format Versioning

- [ ] T045 [US4] Document version strategy in internal/storage/block_format.go: Version constants (V1_0 = "1.0", V1_1 = "1.1", etc.) and migration notes
- [ ] T046 [US4] Implement version-aware file reader in internal/storage/block_format.go: ReadFile(path string) (*File, error) with version check and appropriate handler selection

---

## Phase 7: Polish & Cross-Cutting Concerns

- [ ] T047 Update internal/storage/storage.go to use new block-based StorageFile implementation
- [ ] T048 Write comprehensive integration test in tests/integration/block_storage_e2e_test.go: TestEndToEnd covering full lifecycle (write 100K events, compress, query, verify all success criteria)

---

## Dependency Graph & Execution Order

### Critical Path (Must Complete in Order)

```
T001-T003 (Setup) →
T004-T011 (Foundational) →
T012-T019 (Block Writer) →
T024-T025 (Bloom Integration) →
T027-T028 (Inverted Index) →
T030-T035 (Block Reader + Query) →
T039-T042 (Checksums) →
T045-T046 (Versioning) →
T047-T048 (Integration)
```

### Parallelizable Task Groups

**After T004 (Foundational)**:
- T005: BloomFilter implementation
- T006-T007: Block/BlockMetadata structures
- T008-T009: FileHeader/FileFooter
- T010-T011: IndexSection/InvertedIndex

**After T019 (Block Writer Complete)**:
- T024-T026: Bloom filter integration & tests
- T027-T029: Inverted index building & tests
- T030-T036: Block reader & tests (parallel)

**After T035 (Query Complete)**:
- T039-T044: Corruption detection (parallel)
- T045-T046: Versioning (parallel)

---

## Testing Strategy

### Test Coverage by Story

**US1 (Compression)**: 6 tests (T021-T023)
- Unit: FileHeader/FileFooter serialization, Block compression, EventBuffer
- Integration: Full write/read roundtrip with compression measurement

**US2 (Query Performance)**: 10 tests (T026, T029, T036-T038)
- Unit: Bloom filters, Inverted indexes, Block decompression
- Integration: Query block filtering, Performance benchmarks

**US3 (Corruption Detection)**: 4 tests (T043-T044)
- Unit: Checksum computation and validation
- Integration: Corruption detection and isolation

**US4 (Format Evolution)**: 0 tests (structural verification only)

**Total**: 20+ unit and integration tests providing comprehensive coverage

---

## Implementation Strategy

### MVP Scope (Phase 3-4)

Complete User Stories 1 & 2 to achieve:
- ✅ 50%+ compression ratio
- ✅ 90%+ block skip rate on queries
- ✅ <2 second query response times

This provides immediate value for storage optimization and query performance.

### Enhancement Scope (Phase 5-6)

Add User Stories 3 & 4 for:
- ✅ Corruption detection capability
- ✅ Format evolution support (future-proofing)

These enhance operational reliability and long-term sustainability.

### Post-v1.0 (Future)

- Protobuf encoding option (FR-011 deferred)
- Dictionary learning for compression (out of scope)
- Advanced query optimization patterns
- Distributed/multi-writer scenarios (out of scope for this feature)

---

## Success Metrics & Validation

### Compression (US1)

- [ ] Compression ratio ≥ 50% on typical Kubernetes events (vs uncompressed JSON)
- [ ] Measured: Ratio = CompressedSize / UncompressedSize
- [ ] Test: T023 integration test with 10K events

### Query Performance (US2)

- [ ] Block skip rate ≥ 90% for queries matching <5% of blocks
- [ ] Query latency <2 seconds for 24-hour windows
- [ ] Test: T037-T038 performance benchmarks

### Corruption Detection (US3)

- [ ] Checksum verification <100ms for 100MB files
- [ ] Corrupted blocks isolated, others queryable
- [ ] Test: T044 corruption detection test

### Format Evolution (US4)

- [ ] Version field present in file header
- [ ] Documentation supports 5+ future versions
- [ ] Test: Manual verification in T046

---

## File Changes Summary

### New Files

- `internal/storage/block.go` - Block and BlockMetadata structures
- `internal/storage/block_format.go` - FileHeader, FileFooter, IndexSection readers/writers
- `internal/storage/filter.go` - Enhanced BloomFilter implementation
- `tests/unit/storage/block_format_test.go` - Block format unit tests
- `tests/unit/storage/bloom_filter_test.go` - Bloom filter tests
- `tests/unit/storage/inverted_index_test.go` - Inverted index tests
- `tests/unit/storage/checksum_test.go` - Checksum tests
- `tests/unit/storage/block_reader_test.go` - Block reader tests
- `tests/integration/block_storage_write_test.go` - Write path integration test
- `tests/integration/block_storage_query_test.go` - Query path integration test
- `tests/integration/block_storage_perf_test.go` - Performance tests
- `tests/integration/block_storage_corruption_test.go` - Corruption detection test
- `tests/integration/block_storage_e2e_test.go` - End-to-end integration test
- `docs/BLOCK_FORMAT_REFERENCE.md` - Operational reference

### Modified Files

- `internal/storage/file.go` - Update StorageFile for block-based format
- `internal/storage/query.go` - Update query executor for block filtering
- `internal/storage/storage.go` - Use new block-based StorageFile
- `go.mod` - Add spaolac/bloom dependency

---

## Notes for Implementation

1. **Start with Phase 1-2**: Setup and foundational infrastructure first
2. **Prioritize US1 & US2**: These provide immediate value; complete before US3 & US4
3. **Test as you go**: Each task should have corresponding tests before moving to next task
4. **Reference design artifacts**: Use data-model.md for entity specs, quickstart.md for examples
5. **Maintain compatibility**: QueryResult API contract must remain unchanged (FR-013)
6. **Single-writer guarantee**: File finalization is atomic (FR-007, FR-008 ordering)
7. **Memory efficiency**: Bloom filters ~18KB/block, decompressed block ~256KB (configurable)

---

**Status**: Ready for Phase 2 Implementation (First Task: T001)
