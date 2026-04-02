# Embedded Primary Storage Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the current journal-only embedded backend with a tiered storage engine that keeps recent data hot in memory, persists immutable cold segments on disk, checkpoints derived projection state, and serves Spectre's timeline, metadata, export, and analysis reads efficiently.

**Architecture:** Build the engine in layers inside `internal/embeddedstore`: a hot in-memory store for recent ingest and low-latency queries, an immutable segment store for cold historical data, and a checkpointed projection store for resource-centric analysis reads and fast restart. Keep the current `embeddedstore.Backend` API stable while incrementally swapping its internals from journal replay to manifest + segments + checkpoints + query planning.

**Tech Stack:** Go, existing `internal/embeddedstore` package, existing `models.Event` query and analysis contracts, Cobra server/runtime wiring already merged, on-disk files under `--data-dir`, Go unit tests, integration tests under `tests/integration/api`

---

## Scope Boundary

This plan builds the storage engine internals and validates them behind the existing embedded runtime.

Included:

- hot in-memory event and projection tier
- immutable cold segments on disk
- manifest and crash-safe file layout
- projection checkpoints for fast restart
- hot+cold query planning for timeline, metadata, and export
- analysis reads served from maintained projection state
- background flush and manual/automatic compaction

Not included in this plan:

- making embedded the default runtime in CLI flags
- removing FalkorDB
- observatory graph parity in embedded mode
- distributed storage or multi-node replication

## File Map

- Create: `internal/embeddedstore/config.go`
  Engine-level sizing and interval configuration for hot-memory limits, flush, checkpoint, and compaction.
- Create: `internal/embeddedstore/manifest.go`
  Persistent manifest model and atomic load/store helpers.
- Create: `internal/embeddedstore/manifest_test.go`
  Manifest load/store, versioning, and atomic replacement tests.
- Create: `internal/embeddedstore/segment_format.go`
  Segment metadata, file naming, bundle descriptors, and record encoding helpers.
- Create: `internal/embeddedstore/segment_writer.go`
  Immutable segment builder that writes `events.bin`, `time.idx`, `resource.idx`, `dim.idx`, and `stats.json`.
- Create: `internal/embeddedstore/segment_reader.go`
  Cold-segment range scan, UID scan, and dimension-pruning read helpers.
- Create: `internal/embeddedstore/segment_test.go`
  Segment write/read/pruning tests.
- Create: `internal/embeddedstore/hot_store.go`
  Hot in-memory event tier: recent event buffers, per-UID tails, metadata sets, and flush extraction.
- Create: `internal/embeddedstore/hot_store_test.go`
  Hot-tier query and memory-bound behavior tests.
- Create: `internal/embeddedstore/checkpoint.go`
  Projection checkpoint writer/reader and checkpoint metadata handling.
- Create: `internal/embeddedstore/checkpoint_test.go`
  Checkpoint round-trip and recovery tests.
- Create: `internal/embeddedstore/engine.go`
  Engine orchestration: startup recovery, ingest, flush scheduling, checkpoint scheduling, query planner entrypoints.
- Create: `internal/embeddedstore/engine_test.go`
  Engine-level recovery, hot+cold read, and flush/checkpoint tests.
- Create: `internal/embeddedstore/query_planner.go`
  Planning and merge logic for timeline/search/metadata/export across hot and cold tiers.
- Create: `internal/embeddedstore/query_planner_test.go`
  Query-planner pruning and merge tests.
- Create: `internal/embeddedstore/compaction.go`
  Segment compaction selection and rewrite orchestration.
- Create: `internal/embeddedstore/compaction_test.go`
  Compaction correctness and manifest swap tests.
- Modify: `internal/embeddedstore/backend.go`
  Replace journal-centric internals with engine orchestration while preserving public backend methods.
- Modify: `internal/embeddedstore/query_executor.go`
  Route timeline queries through the new query planner and hot+cold merge path.
- Modify: `internal/embeddedstore/analysis_store.go`
  Keep analysis reads on projection state while decoupling from assumptions that all history is fully resident in RAM.
- Modify: `internal/embeddedstore/projection.go`
  Add snapshot/export/import helpers needed for checkpoints and bounded hot history.
- Modify: `internal/embeddedstore/backend_test.go`
  Update backend-level assertions from journal-specific behavior to engine behavior.
- Modify: `tests/integration/api/embedded_runtime_import_only_test.go`
  Assert restart from checkpoints and cold segments, not only in-memory state.
- Modify: `tests/integration/api/embedded_runtime_restart_test.go`
  Cover restart with flushed segments and checkpoints.
- Create: `tests/integration/api/embedded_runtime_cold_query_test.go`
  Verify older data is served after aging out of the hot tier.

## Task 1: Add Engine Config And Persistent Manifest

**Files:**
- Create: `internal/embeddedstore/config.go`
- Create: `internal/embeddedstore/manifest.go`
- Create: `internal/embeddedstore/manifest_test.go`
- Test: `internal/embeddedstore/manifest_test.go`

- [ ] **Step 1: Write failing manifest tests for create/load/store behavior**

```go
func TestManifestStore_LoadOrCreateInitialManifest(t *testing.T) {
	dir := t.TempDir()

	manifest, err := loadOrCreateManifest(dir)
	require.NoError(t, err)
	require.Equal(t, storageFormatVersion, manifest.FormatVersion)
	require.Empty(t, manifest.ActiveSegments)
	require.Empty(t, manifest.Checkpoints)
}

func TestManifestStore_AtomicReplace(t *testing.T) {
	dir := t.TempDir()
	manifest, err := loadOrCreateManifest(dir)
	require.NoError(t, err)

	manifest.ActiveSegments = append(manifest.ActiveSegments, SegmentMeta{ID: "seg-001"})
	require.NoError(t, storeManifest(dir, manifest))

	reloaded, err := loadOrCreateManifest(dir)
	require.NoError(t, err)
	require.Len(t, reloaded.ActiveSegments, 1)
	require.Equal(t, "seg-001", reloaded.ActiveSegments[0].ID)
}
```

- [ ] **Step 2: Run the manifest tests and verify they fail**

Run: `go test ./internal/embeddedstore -run 'TestManifestStore_' -v`
Expected: FAIL because manifest helpers do not exist yet.

- [ ] **Step 3: Implement engine config and manifest persistence**

In `config.go`, define:

```go
type EngineConfig struct {
	DataDir                 string
	HotMaxEvents            int
	HotMaxResourceVersions  int
	FlushInterval           time.Duration
	CheckpointInterval      time.Duration
	SegmentTargetBytes      int64
	CompactionMinSegments   int
}
```

In `manifest.go`, define:

```go
const storageFormatVersion = 1

type Manifest struct {
	FormatVersion     int              `json:"format_version"`
	ActiveSegments    []SegmentMeta    `json:"active_segments"`
	Checkpoints       []CheckpointMeta `json:"checkpoints"`
	FlushHighWaterMark uint64          `json:"flush_high_water_mark"`
}
```

Persist using temp file + fsync + atomic rename.

- [ ] **Step 4: Re-run manifest tests**

Run: `go test ./internal/embeddedstore -run 'TestManifestStore_' -v`
Expected: PASS.

- [ ] **Step 5: Commit the manifest/config slice**

```bash
git add internal/embeddedstore/config.go internal/embeddedstore/manifest.go internal/embeddedstore/manifest_test.go
git commit -m "feat: add embedded engine manifest and config"
```

## Task 2: Build Immutable Segment Format, Writer, And Reader

**Files:**
- Create: `internal/embeddedstore/segment_format.go`
- Create: `internal/embeddedstore/segment_writer.go`
- Create: `internal/embeddedstore/segment_reader.go`
- Create: `internal/embeddedstore/segment_test.go`
- Test: `internal/embeddedstore/segment_test.go`

- [ ] **Step 1: Write failing segment tests for write/read and pruning**

```go
func TestSegment_WriteReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	events := []models.Event{
		{ID: "1", Timestamp: 10, Resource: models.ResourceMetadata{Namespace: "default", Kind: "Pod", UID: "pod-1"}},
		{ID: "2", Timestamp: 20, Resource: models.ResourceMetadata{Namespace: "default", Kind: "Service", UID: "svc-1"}},
	}

	meta, err := writeSegment(dir, "seg-001", events)
	require.NoError(t, err)

	reader, err := openSegmentReader(dir, meta)
	require.NoError(t, err)

	got, err := reader.ScanTimeRange(context.Background(), 0, 30)
	require.NoError(t, err)
	require.Len(t, got, 2)
}

func TestSegment_PrunesByNamespaceKindStats(t *testing.T) {
	dir := t.TempDir()
	events := []models.Event{
		{ID: "1", Timestamp: 10, Resource: models.ResourceMetadata{Namespace: "flux-system", Kind: "HelmRelease", UID: "hr-1"}},
	}

	meta, err := writeSegment(dir, "seg-001", events)
	require.NoError(t, err)
	require.True(t, meta.MayContain("flux-system", "HelmRelease"))
	require.False(t, meta.MayContain("default", "Pod"))
}
```

- [ ] **Step 2: Run segment tests to verify they fail**

Run: `go test ./internal/embeddedstore -run 'TestSegment_' -v`
Expected: FAIL because segment writer/reader do not exist.

- [ ] **Step 3: Implement the segment bundle format**

Use the following bundle files:

```text
segments/<segment-id>/events.bin
segments/<segment-id>/time.idx
segments/<segment-id>/resource.idx
segments/<segment-id>/dim.idx
segments/<segment-id>/stats.json
```

Writer requirements:

- sort records by `(timestamp, sequence)`
- encode event records in an append-friendly binary framing
- build sparse indexes while writing
- write to `tmp/` and atomically rename into `segments/<segment-id>/`

Reader requirements:

- scan by time range
- scan by UID
- inspect stats for namespace/kind pruning

- [ ] **Step 4: Re-run segment tests**

Run: `go test ./internal/embeddedstore -run 'TestSegment_' -v`
Expected: PASS.

- [ ] **Step 5: Commit the segment slice**

```bash
git add internal/embeddedstore/segment_format.go internal/embeddedstore/segment_writer.go internal/embeddedstore/segment_reader.go internal/embeddedstore/segment_test.go
git commit -m "feat: add embedded cold segment storage"
```

## Task 3: Add The Hot In-Memory Store

**Files:**
- Create: `internal/embeddedstore/hot_store.go`
- Create: `internal/embeddedstore/hot_store_test.go`
- Modify: `internal/embeddedstore/projection.go`
- Test: `internal/embeddedstore/hot_store_test.go`

- [ ] **Step 1: Write failing tests for recent-window and per-UID hot tails**

```go
func TestHotStore_QueryRecentTimeRange(t *testing.T) {
	store := newHotStore(HotStoreConfig{MaxEvents: 10, MaxResourceVersions: 4})
	store.Append([]models.Event{
		{ID: "1", Timestamp: 10, Resource: models.ResourceMetadata{UID: "pod-1", Namespace: "default", Kind: "Pod"}},
		{ID: "2", Timestamp: 20, Resource: models.ResourceMetadata{UID: "pod-1", Namespace: "default", Kind: "Pod"}},
	})

	got := store.ScanTimeRange(0, 15)
	require.Len(t, got, 1)
	require.Equal(t, "1", got[0].ID)
}

func TestHotStore_BoundsResourceVersionHistory(t *testing.T) {
	store := newHotStore(HotStoreConfig{MaxEvents: 100, MaxResourceVersions: 2})
	for i := 0; i < 3; i++ {
		store.Append([]models.Event{{ID: fmt.Sprintf("%d", i), Timestamp: int64(i + 1), Resource: models.ResourceMetadata{UID: "pod-1", Namespace: "default", Kind: "Pod"}}})
	}

	require.Len(t, store.RecentEventsByUID("pod-1"), 2)
}
```

- [ ] **Step 2: Run hot-store tests and verify they fail**

Run: `go test ./internal/embeddedstore -run 'TestHotStore_' -v`
Expected: FAIL because the hot store does not exist.

- [ ] **Step 3: Implement the hot store**

Implement:

- append-friendly recent event buffer ordered by timestamp
- `map[uid]*RecentResourceLog`
- separate `Kind=Event` recent association keyed by involved UID
- hot metadata sets
- methods for flush extraction that hand off a stable batch without stopping readers

Also add projection helpers in `projection.go` to:

- export a snapshot for checkpoints
- import a snapshot during recovery

- [ ] **Step 4: Re-run hot-store tests**

Run: `go test ./internal/embeddedstore -run 'TestHotStore_' -v`
Expected: PASS.

- [ ] **Step 5: Commit the hot-store slice**

```bash
git add internal/embeddedstore/hot_store.go internal/embeddedstore/hot_store_test.go internal/embeddedstore/projection.go
git commit -m "feat: add embedded hot in-memory store"
```

## Task 4: Add Projection Checkpoints

**Files:**
- Create: `internal/embeddedstore/checkpoint.go`
- Create: `internal/embeddedstore/checkpoint_test.go`
- Modify: `internal/embeddedstore/projection.go`
- Test: `internal/embeddedstore/checkpoint_test.go`

- [ ] **Step 1: Write failing checkpoint tests**

```go
func TestCheckpoint_RoundTripProjectionState(t *testing.T) {
	dir := t.TempDir()
	projection, err := BuildProjection([]models.Event{
		{ID: "1", Timestamp: 10, Resource: models.ResourceMetadata{UID: "pod-1", Namespace: "default", Kind: "Pod", Name: "pod-1"}},
	})
	require.NoError(t, err)

	meta, err := writeCheckpoint(dir, projection, 123)
	require.NoError(t, err)

	restored, highWaterMark, err := loadCheckpoint(dir, meta)
	require.NoError(t, err)
	require.Equal(t, uint64(123), highWaterMark)

	resource, err := restored.GetResource(context.Background(), "pod-1")
	require.NoError(t, err)
	require.NotNil(t, resource)
}
```

- [ ] **Step 2: Run checkpoint tests and verify they fail**

Run: `go test ./internal/embeddedstore -run 'TestCheckpoint_' -v`
Expected: FAIL because checkpoint helpers do not exist.

- [ ] **Step 3: Implement checkpoint writer/reader**

Persist:

- latest resource state
- bounded resource versions
- relationship adjacency
- namespace state
- metadata state
- checkpoint high-water mark

Checkpoint writes must use:

- staging dir in `tmp/`
- fsync
- atomic rename

- [ ] **Step 4: Re-run checkpoint tests**

Run: `go test ./internal/embeddedstore -run 'TestCheckpoint_' -v`
Expected: PASS.

- [ ] **Step 5: Commit the checkpoint slice**

```bash
git add internal/embeddedstore/checkpoint.go internal/embeddedstore/checkpoint_test.go internal/embeddedstore/projection.go
git commit -m "feat: add embedded projection checkpoints"
```

## Task 5: Introduce The Engine Orchestrator And Startup Recovery

**Files:**
- Create: `internal/embeddedstore/engine.go`
- Create: `internal/embeddedstore/engine_test.go`
- Modify: `internal/embeddedstore/backend.go`
- Modify: `internal/embeddedstore/backend_test.go`
- Test: `internal/embeddedstore/engine_test.go`

- [ ] **Step 1: Write failing engine recovery tests**

```go
func TestEngine_OpenLoadsCheckpointAndReplaysNewerSegments(t *testing.T) {
	dir := t.TempDir()

	engine1, err := OpenEngine(EngineConfig{DataDir: dir, HotMaxEvents: 100, HotMaxResourceVersions: 4})
	require.NoError(t, err)
	require.NoError(t, engine1.ProcessBatch(context.Background(), []models.Event{
		{ID: "1", Timestamp: 10, Resource: models.ResourceMetadata{UID: "pod-1", Namespace: "default", Kind: "Pod", Name: "pod-1"}},
	}))
	require.NoError(t, engine1.Flush(context.Background()))
	require.NoError(t, engine1.Checkpoint(context.Background()))
	require.NoError(t, engine1.Close())

	engine2, err := OpenEngine(EngineConfig{DataDir: dir, HotMaxEvents: 100, HotMaxResourceVersions: 4})
	require.NoError(t, err)
	defer func() { require.NoError(t, engine2.Close()) }()

	result, err := engine2.QueryExecutor().Execute(context.Background(), &models.QueryRequest{StartTimestamp: 0, EndTimestamp: 100})
	require.NoError(t, err)
	require.Len(t, result.Events, 1)
}
```

- [ ] **Step 2: Run engine tests and verify they fail**

Run: `go test ./internal/embeddedstore -run 'TestEngine_' -v`
Expected: FAIL because engine orchestration does not exist.

- [ ] **Step 3: Implement the engine and rewire backend**

`engine.go` should own:

- config
- manifest
- hot store
- projection
- live segment readers
- startup recovery
- explicit `Flush`, `Checkpoint`, and `Compact` entrypoints

`backend.go` should delegate:

- `ProcessEvent`
- `ProcessBatch`
- `QueryExecutor`
- `AnalysisStore`
- `Start`
- `Stop`
- `Close`

to the engine.

- [ ] **Step 4: Re-run engine and backend tests**

Run: `go test ./internal/embeddedstore -run 'TestEngine_|TestBackend_' -v`
Expected: PASS.

- [ ] **Step 5: Commit the engine orchestration slice**

```bash
git add internal/embeddedstore/engine.go internal/embeddedstore/engine_test.go internal/embeddedstore/backend.go internal/embeddedstore/backend_test.go
git commit -m "refactor: route embedded backend through storage engine"
```

## Task 6: Build The Hot+Cold Query Planner

**Files:**
- Create: `internal/embeddedstore/query_planner.go`
- Create: `internal/embeddedstore/query_planner_test.go`
- Modify: `internal/embeddedstore/query_executor.go`
- Test: `internal/embeddedstore/query_planner_test.go`

- [ ] **Step 1: Write failing planner tests for hot+cold merge**

```go
func TestQueryPlanner_MergesHotAndColdTimeRangeResults(t *testing.T) {
	engine := newTestEngineWithColdSegment(t,
		[]models.Event{{ID: "cold-1", Timestamp: 10, Resource: models.ResourceMetadata{UID: "pod-1", Namespace: "default", Kind: "Pod", Name: "pod-1"}}},
		[]models.Event{{ID: "hot-1", Timestamp: 20, Resource: models.ResourceMetadata{UID: "pod-1", Namespace: "default", Kind: "Pod", Name: "pod-1"}}},
	)

	result, err := engine.QueryExecutor().Execute(context.Background(), &models.QueryRequest{
		StartTimestamp: 0,
		EndTimestamp:   30,
		Filters:        models.QueryFilters{},
	})
	require.NoError(t, err)
	require.Len(t, result.Events, 2)
}

func TestQueryPlanner_QueryDistinctMetadataUsesProjectionState(t *testing.T) {
	engine := newTestEngineWithEvents(t, []models.Event{
		{ID: "1", Timestamp: 10, Resource: models.ResourceMetadata{UID: "pod-1", Namespace: "default", Kind: "Pod"}},
	})

	namespaces, kinds, _, _, err := engine.QueryExecutor().QueryDistinctMetadata(context.Background(), 0, 20*1e9)
	require.NoError(t, err)
	require.Equal(t, []string{"default"}, namespaces)
	require.Equal(t, []string{"Pod"}, kinds)
}
```

- [ ] **Step 2: Run planner tests and verify they fail**

Run: `go test ./internal/embeddedstore -run 'TestQueryPlanner_' -v`
Expected: FAIL because the planner does not exist.

- [ ] **Step 3: Implement planner and query-executor integration**

Planner responsibilities:

- prune cold segments by time range
- prune by segment stats and dimension index
- consult hot store first
- merge hot and cold results in timestamp order
- preserve current resource-centric pagination semantics
- source metadata from maintained projection state, not cold scans

- [ ] **Step 4: Re-run planner and query executor tests**

Run: `go test ./internal/embeddedstore ./internal/embedded -run 'TestQueryPlanner_|TestQueryExecutor' -v`
Expected: PASS.

- [ ] **Step 5: Commit the planner slice**

```bash
git add internal/embeddedstore/query_planner.go internal/embeddedstore/query_planner_test.go internal/embeddedstore/query_executor.go
git commit -m "feat: add embedded hot and cold query planner"
```

## Task 7: Optimize Export And Preserve Analysis On Projection State

**Files:**
- Modify: `internal/embeddedstore/query_executor.go`
- Modify: `internal/embeddedstore/analysis_store.go`
- Modify: `internal/api/handlers/export_handler.go`
- Test: `internal/embeddedstore/engine_test.go`
- Test: `tests/integration/api/embedded_runtime_cold_query_test.go`

- [ ] **Step 1: Write failing tests for cold export and cold-history analysis**

```go
func TestEngine_ExportReadsFlushedColdSegment(t *testing.T) {
	engine := newFlushedTestEngine(t, []models.Event{
		{ID: "1", Timestamp: 10, Resource: models.ResourceMetadata{UID: "pod-1", Namespace: "default", Kind: "Pod", Name: "pod-1"}},
	})

	result, err := engine.QueryExecutor().Execute(context.Background(), &models.QueryRequest{
		StartTimestamp: 0,
		EndTimestamp:   20,
		Filters:        models.QueryFilters{},
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.Events)
}
```

- [ ] **Step 2: Run targeted tests and verify they fail**

Run: `go test ./internal/embeddedstore ./tests/integration/api -run 'TestEngine_ExportReadsFlushedColdSegment|TestEmbeddedRuntime' -v`
Expected: FAIL before export and cold-history behavior are wired through the new engine fully.

- [ ] **Step 3: Implement export optimization and analysis guarantees**

Changes:

- keep export on sequential segment scans with time pruning
- do not route analysis through raw segment scans in the normal path
- if bounded recent tails are needed for analysis beyond current hot memory, load them from checkpointed projection state rather than replaying arbitrary old segments during query execution

- [ ] **Step 4: Re-run targeted tests**

Run: `go test ./internal/embeddedstore ./tests/integration/api -run 'TestEngine_ExportReadsFlushedColdSegment|TestEmbeddedRuntime' -v`
Expected: PASS.

- [ ] **Step 5: Commit the export/analysis slice**

```bash
git add internal/embeddedstore/query_executor.go internal/embeddedstore/analysis_store.go internal/api/handlers/export_handler.go tests/integration/api/embedded_runtime_cold_query_test.go
git commit -m "perf: serve cold export and analysis from embedded engine tiers"
```

## Task 8: Add Background Flush And Compaction

**Files:**
- Create: `internal/embeddedstore/compaction.go`
- Create: `internal/embeddedstore/compaction_test.go`
- Modify: `internal/embeddedstore/engine.go`
- Test: `internal/embeddedstore/compaction_test.go`

- [ ] **Step 1: Write failing compaction tests**

```go
func TestCompaction_MergesOldSegmentsAndPreservesQueryResults(t *testing.T) {
	engine := newCompactionTestEngine(t)
	require.NoError(t, engine.ProcessBatch(context.Background(), makeTestEvents("seg-a", 0, 100)))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.ProcessBatch(context.Background(), makeTestEvents("seg-b", 100, 200)))
	require.NoError(t, engine.Flush(context.Background()))

	before, err := engine.QueryExecutor().Execute(context.Background(), &models.QueryRequest{StartTimestamp: 0, EndTimestamp: 500})
	require.NoError(t, err)

	require.NoError(t, engine.Compact(context.Background()))

	after, err := engine.QueryExecutor().Execute(context.Background(), &models.QueryRequest{StartTimestamp: 0, EndTimestamp: 500})
	require.NoError(t, err)
	require.Equal(t, before.Events, after.Events)
}
```

- [ ] **Step 2: Run compaction tests and verify they fail**

Run: `go test ./internal/embeddedstore -run 'TestCompaction_' -v`
Expected: FAIL because compaction does not exist.

- [ ] **Step 3: Implement background flush and compaction orchestration**

Requirements:

- periodic flush from hot tier to cold segment when interval or size threshold is crossed
- compaction only for sealed old segments
- manifest swap must be atomic
- old segments removed only after manifest update succeeds

- [ ] **Step 4: Re-run compaction tests**

Run: `go test ./internal/embeddedstore -run 'TestCompaction_' -v`
Expected: PASS.

- [ ] **Step 5: Commit the compaction slice**

```bash
git add internal/embeddedstore/compaction.go internal/embeddedstore/compaction_test.go internal/embeddedstore/engine.go
git commit -m "feat: add embedded segment flush and compaction"
```

## Task 9: Expand Runtime And Integration Coverage

**Files:**
- Modify: `tests/integration/api/embedded_runtime_import_only_test.go`
- Modify: `tests/integration/api/embedded_runtime_restart_test.go`
- Create: `tests/integration/api/embedded_runtime_cold_query_test.go`
- Test: `tests/integration/api/embedded_runtime_import_only_test.go`
- Test: `tests/integration/api/embedded_runtime_restart_test.go`
- Test: `tests/integration/api/embedded_runtime_cold_query_test.go`

- [ ] **Step 1: Write failing integration tests for cold-history behavior**

```go
func TestEmbeddedRuntimeColdQueryReadsFlushedSegment(t *testing.T) {
	backend := openEmbeddedBackendWithTinyHotWindow(t)
	require.NoError(t, backend.ProcessBatch(context.Background(), fixtureEventsForColdQuery()))
	require.NoError(t, backend.Flush(context.Background()))

	server := newEmbeddedRuntimeServer(t, backend)
	response := queryEmbeddedTimeline(t, server, 0, 4102444800)
	require.NotEmpty(t, response.Resources)
}
```

- [ ] **Step 2: Run integration tests and verify they fail**

Run: `go test ./tests/integration/api -run 'TestEmbeddedRuntime' -v`
Expected: FAIL before the new engine is fully represented in tests.

- [ ] **Step 3: Update runtime tests to validate the new engine shape**

Add coverage for:

- restart from checkpoint + cold segments
- import-only serving from persisted cold state with watcher disabled
- cold history still queryable after hot-tier eviction
- readiness remains correct during flush/checkpoint cycles

- [ ] **Step 4: Re-run integration tests**

Run: `go test ./tests/integration/api -run 'TestEmbeddedRuntime' -v`
Expected: PASS.

- [ ] **Step 5: Commit the integration coverage slice**

```bash
git add tests/integration/api/embedded_runtime_import_only_test.go tests/integration/api/embedded_runtime_restart_test.go tests/integration/api/embedded_runtime_cold_query_test.go
git commit -m "test: cover embedded hot and cold storage runtime"
```

## Task 10: Final Verification And Rollout Notes

**Files:**
- Modify: `internal/embeddedstore/backend.go`
- Modify: `cmd/spectre/commands/server.go`
- Modify: `docs/superpowers/specs/2026-04-02-embedded-primary-storage-engine-design.md` (only if implementation decisions drift from spec)
- Test: broader verification commands

- [ ] **Step 1: Add any remaining guardrails for engine configuration defaults**

Examples:

- reject invalid hot-memory limits
- reject invalid segment target bytes
- log configured flush/checkpoint/compaction settings clearly

- [ ] **Step 2: Run embeddedstore-focused verification**

Run: `go test ./internal/embeddedstore ./internal/embedded ./internal/analysis/store/embedded -v`
Expected: PASS.

- [ ] **Step 3: Run runtime and integration verification**

Run: `go test ./cmd/... ./internal/... ./tests/integration/api/...`
Expected: PASS.

- [ ] **Step 4: Re-read the approved storage-engine spec and reconcile any drift**

If implementation needed a justified change, update:

`docs/superpowers/specs/2026-04-02-embedded-primary-storage-engine-design.md`

and re-run targeted tests.

- [ ] **Step 5: Commit the final verification or drift-adjustment slice**

```bash
git add internal/embeddedstore/backend.go cmd/spectre/commands/server.go docs/superpowers/specs/2026-04-02-embedded-primary-storage-engine-design.md
git commit -m "chore: finalize embedded primary storage engine rollout"
```

## Verification Notes

Expected running checkpoints during implementation:

- `go test ./internal/embeddedstore -run 'TestManifestStore_|TestSegment_|TestHotStore_|TestCheckpoint_|TestEngine_|TestQueryPlanner_|TestCompaction_' -v`
- `go test ./internal/embeddedstore ./internal/embedded ./internal/analysis/store/embedded -v`
- `go test ./tests/integration/api -run 'TestEmbeddedRuntime' -v`
- `go test ./cmd/... ./internal/... ./tests/integration/api/...`

## Rollout Guidance

After this plan lands, the codebase should still keep backend selection explicit. Making the embedded engine the default runtime or removing FalkorDB should be a follow-on rollout plan after:

- real dataset profiling
- memory and cold-query latency measurements
- confidence in compaction and restart behavior
