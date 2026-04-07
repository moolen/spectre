# Embedded Startup Segment Index Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite legacy embedded cold segments once at startup so historical datasets gain `associated.idx` automatically and timeline Event-kind lookups stop relying on full segment scans.

**Architecture:** Extend the embedded manifest with a durable segment index generation marker and run a one-shot startup migration before planner initialization when the dataset is still on the legacy generation. The migration rewrites active segments using the current writer, atomically swaps the manifest to the rewritten segment set, and retries safely until it succeeds.

**Tech Stack:** Go, `internal/embeddedstore`, Cobra server startup path, Go unit tests, homelab rollout verification

---

## Spec Reference

- Spec: `/home/moritz/dev/spectre-via-ssh/.worktrees/checkpoint-compression/docs/superpowers/specs/2026-04-07-embedded-startup-segment-index-migration-design.md`
- Implement with `@superpowers:test-driven-development`
- Before claiming success, use `@superpowers:verification-before-completion`

## File Map

**Create:**

- `internal/embeddedstore/startup_segment_migration_test.go`
  - TDD coverage for one-shot migration, retry safety, and startup skip behavior.

**Modify:**

- `internal/embeddedstore/manifest.go`
  - Persist and normalize the segment index generation marker.
- `internal/embeddedstore/manifest_test.go`
  - Cover legacy manifest loading and generation persistence.
- `internal/embeddedstore/engine.go`
  - Invoke startup migration before planner refresh and normal readiness.
- `internal/embeddedstore/engine_state.go`
  - Re-open readers from the committed segment set after migration.
- `internal/embeddedstore/segment_reader.go`
  - Expose a capability/helper to detect whether a reader already has `associated.idx` without forcing repeated full scans.
- `internal/embeddedstore/segment_writer.go`
  - Reuse current writer for rewrite output and keep bundle metadata deterministic.
- `internal/embeddedstore/compaction_test.go`
  - Extend segment rewrite expectations if compaction semantics overlap with the new artifact generation.
- `internal/embeddedstore/engine_test.go`
  - Cover startup migration integration and manifest persistence.
- `internal/embeddedstore/query_metrics_test.go`
  - Update metric expectations if migrated segments change query-path counters in existing tests.

## Task 1: Persist Segment Index Generation In The Manifest

**Files:**
- Modify: `internal/embeddedstore/manifest.go`
- Modify: `internal/embeddedstore/manifest_test.go`
- Test: `internal/embeddedstore/manifest_test.go`

- [ ] **Step 1: Write the failing manifest generation tests**

```go
func TestManifest_LoadLegacyManifestDefaultsSegmentIndexGeneration(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, manifestFileName)
	require.NoError(t, os.WriteFile(manifestPath, []byte(`{"format_version":1,"active_segments":[],"checkpoints":[]}`), 0o600))

	manifest, err := loadOrCreateManifest(dir)
	require.NoError(t, err)
	require.Equal(t, 0, manifest.SegmentIndexGeneration)
}

func TestManifest_StoresSegmentIndexGeneration(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, storeManifest(dir, Manifest{
		FormatVersion:           storageFormatVersion,
		SegmentIndexGeneration:  1,
		ActiveSegments:          []SegmentMeta{},
		Checkpoints:             []CheckpointMeta{},
	}))

	reloaded, err := loadOrCreateManifest(dir)
	require.NoError(t, err)
	require.Equal(t, 1, reloaded.SegmentIndexGeneration)
}
```

- [ ] **Step 2: Run the focused manifest tests to verify they fail**

Run: `go test ./internal/embeddedstore -run 'TestManifest_(LoadLegacyManifestDefaultsSegmentIndexGeneration|StoresSegmentIndexGeneration)' -count=1`
Expected: FAIL because the manifest does not yet persist the generation marker.

- [ ] **Step 3: Implement manifest generation persistence**

Add a field like this to `internal/embeddedstore/manifest.go`:

```go
type Manifest struct {
	FormatVersion           int              `json:"format_version"`
	SegmentIndexGeneration  int              `json:"segment_index_generation,omitempty"`
	ActiveSegments          []SegmentMeta    `json:"active_segments"`
	ActiveCheckpoint        CheckpointMeta   `json:"active_checkpoint"`
	ActiveTail              TailJournalMeta  `json:"active_tail"`
	Checkpoints             []CheckpointMeta `json:"checkpoints"`
	FlushHighWaterMark      uint64           `json:"flush_high_water_mark"`
}
```

Keep omitted legacy values normalized to `0`.

- [ ] **Step 4: Re-run the focused manifest tests**

Run: `go test ./internal/embeddedstore -run 'TestManifest_(LoadLegacyManifestDefaultsSegmentIndexGeneration|StoresSegmentIndexGeneration)' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit the manifest generation slice**

```bash
git add internal/embeddedstore/manifest.go \
        internal/embeddedstore/manifest_test.go
git commit -m "persist embedded segment index generation"
```

## Task 2: Add Segment Rewrite Helpers And Migration Detection

**Files:**
- Create: `internal/embeddedstore/startup_segment_migration_test.go`
- Modify: `internal/embeddedstore/segment_reader.go`
- Modify: `internal/embeddedstore/segment_writer.go`
- Modify: `internal/embeddedstore/engine_state.go`
- Test: `internal/embeddedstore/startup_segment_migration_test.go`

- [ ] **Step 1: Write the failing migration-helper tests**

```go
func TestStartupMigration_RewritesLegacySegmentsWithAssociatedIndex(t *testing.T) {
	engine := newCompactionTestEngine(t, EngineConfig{
		HotMaxEvents: 128,
		HotMaxResourceVersions: 16,
	})

	require.NoError(t, engine.ProcessBatch(context.Background(), makeLegacyAssociatedEventBatch("legacy", 0, 3)))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.Close())

	rootDir := embeddedRootDir(engine.config.DataDir)
	manifest, err := loadOrCreateManifest(rootDir)
	require.NoError(t, err)
	manifest.SegmentIndexGeneration = 0
	require.NoError(t, storeManifest(rootDir, manifest))

	reopened, err := OpenEngine(EngineConfig{DataDir: engine.config.DataDir, HotMaxEvents: 128, HotMaxResourceVersions: 16})
	require.NoError(t, err)
	defer reopened.Close()

	require.Equal(t, 1, reopened.manifest.SegmentIndexGeneration)
	reader := reopened.segmentReaders[0]
	_, indexed, err := reader.ScanAssociatedUIDs(context.Background(), []string{"pod-1"})
	require.NoError(t, err)
	require.True(t, indexed)
}
```

- [ ] **Step 2: Run the focused migration-helper test to verify it fails**

Run: `go test ./internal/embeddedstore -run TestStartupMigration_RewritesLegacySegmentsWithAssociatedIndex -count=1`
Expected: FAIL because startup does not yet rewrite legacy segments.

- [ ] **Step 3: Implement migration detection and segment rewrite helpers**

Add helpers that:

- detect whether a reader already has `associated.idx`
- rewrite one segment by scanning its full event range and writing a new segment bundle
- build a replacement `SegmentMeta` preserving the original `HighWaterMark`

Prefer a dedicated helper rather than embedding rewrite logic directly in `OpenEngine(...)`.

- [ ] **Step 4: Re-run the focused migration-helper test**

Run: `go test ./internal/embeddedstore -run TestStartupMigration_RewritesLegacySegmentsWithAssociatedIndex -count=1`
Expected: PASS.

- [ ] **Step 5: Commit the migration-helper slice**

```bash
git add internal/embeddedstore/startup_segment_migration_test.go \
        internal/embeddedstore/segment_reader.go \
        internal/embeddedstore/segment_writer.go \
        internal/embeddedstore/engine_state.go
git commit -m "add embedded legacy segment rewrite helpers"
```

## Task 3: Run One-Shot Migration During Startup With Atomic Manifest Swap

**Files:**
- Modify: `internal/embeddedstore/engine.go`
- Modify: `internal/embeddedstore/engine_state.go`
- Modify: `internal/embeddedstore/startup_segment_migration_test.go`
- Modify: `internal/embeddedstore/engine_test.go`
- Test: `internal/embeddedstore/startup_segment_migration_test.go`
- Test: `internal/embeddedstore/engine_test.go`

- [ ] **Step 1: Write the failing startup migration behavior tests**

Add tests for:

- manifest generation `1` skips rewrite on second startup
- failure before manifest swap leaves the old manifest usable
- startup reopens readers from the migrated segment set

Example skeleton:

```go
func TestStartupMigration_SkipsWhenGenerationAlreadyCurrent(t *testing.T) {}

func TestStartupMigration_FailedRewriteLeavesLegacyManifest(t *testing.T) {}
```

- [ ] **Step 2: Run the focused startup migration tests to verify they fail**

Run: `go test ./internal/embeddedstore -run 'TestStartupMigration_' -count=1`
Expected: FAIL because startup has no one-shot migration orchestration yet.

- [ ] **Step 3: Implement startup migration orchestration**

Update `internal/embeddedstore/engine.go` so startup:

1. loads the manifest
2. checks `SegmentIndexGeneration`
3. rewrites active segments if needed
4. atomically stores the migrated manifest
5. removes old segment directories after manifest commit
6. continues opening the engine from the committed segment set

Keep the commit point at `storeManifest(...)`. Do not delete old segments before manifest persistence succeeds.

- [ ] **Step 4: Re-run the focused startup migration tests**

Run: `go test ./internal/embeddedstore -run 'TestStartupMigration_' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit the startup migration orchestration slice**

```bash
git add internal/embeddedstore/engine.go \
        internal/embeddedstore/engine_state.go \
        internal/embeddedstore/startup_segment_migration_test.go \
        internal/embeddedstore/engine_test.go
git commit -m "run embedded segment index migration at startup"
```

## Task 4: Verify Query Behavior And Roll Out

**Files:**
- Modify: `internal/embeddedstore/query_metrics_test.go`
- Modify: `internal/embeddedstore/compaction_test.go`
- Test: `internal/embeddedstore/query_metrics_test.go`
- Test: `internal/embeddedstore/compaction_test.go`

- [ ] **Step 1: Write or update failing verification tests for migrated query behavior**

Add or extend tests so migrated historical segments show indexed associated lookups rather than scan-only fallback behavior.

- [ ] **Step 2: Run the focused query verification tests to verify they fail first**

Run: `go test ./internal/embeddedstore -run 'Test(QueryMetrics_|Compaction_)' -count=1`
Expected: FAIL until the new migration path and expectations are wired together.

- [ ] **Step 3: Implement the minimal test-aligned changes**

Only update behavior that is actually affected by migrated historical segments:

- metrics expectations
- any compaction assumptions about segment artifacts

- [ ] **Step 4: Re-run the focused query verification tests**

Run: `go test ./internal/embeddedstore -run 'Test(QueryMetrics_|Compaction_)' -count=1`
Expected: PASS.

- [ ] **Step 5: Run package verification**

Run: `go test ./internal/embeddedstore/... ./cmd/spectre/commands -count=1`
Expected: PASS.

- [ ] **Step 6: Build and push a rollout image**

Run:

```bash
TAG=startup-segment-migration-$(date +%Y%m%d-%H%M%S)
docker buildx build --platform linux/amd64 -t ghcr.io/moolen/spectre:$TAG --push .
```

Expected: image successfully pushed to GHCR.

- [ ] **Step 7: Deploy to homelab**

Run:

```bash
kubectl --context homelab-admin@kubernetes -n monitoring set image deploy/spectre spectre=ghcr.io/moolen/spectre:$TAG
kubectl --context homelab-admin@kubernetes -n monitoring rollout status deploy/spectre --timeout=10m
```

Expected: rollout succeeds, first startup may take longer because of one-time migration.

- [ ] **Step 8: Verify one-shot migration and measure**

Collect:

- startup logs showing migration ran
- segment counts with `associated.idx` after rollout
- second restart logs showing migration skipped
- timeline timing for 1h and 4h windows
- memory after steady state

Commands:

```bash
kubectl --context homelab-admin@kubernetes -n monitoring logs deploy/spectre --since=15m | rg 'migration|startup_mode|checkpoint loaded|engine opened'
kubectl --context homelab-admin@kubernetes -n monitoring top pod -l app.kubernetes.io/name=spectre
```

- [ ] **Step 9: Commit the rollout-ready verification slice**

```bash
git add internal/embeddedstore/query_metrics_test.go \
        internal/embeddedstore/compaction_test.go \
        internal/embeddedstore/startup_segment_migration_test.go \
        internal/embeddedstore/engine_test.go
git commit -m "verify embedded startup segment migration"
```
