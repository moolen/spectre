# Embedded Near-Instant Startup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make embedded Spectre restart from on-disk state in under 15 seconds by loading a compact checkpoint plus a bounded tail journal instead of replaying post-checkpoint cold segments.

**Architecture:** Extend `internal/embeddedstore` so immutable segments remain the history source for query-time reads, while restart head state comes from a streaming-written compact checkpoint plus a durable post-checkpoint tail journal. Keep the current API startup sequence, but make `embeddedstore.Open(...)` fast by eliminating cold-segment replay from the normal restart path and by re-enabling safe, bounded checkpoint rotation.

**Tech Stack:** Go, existing `internal/embeddedstore` engine/query planner/projection code, Cobra server flags, Helm deployment defaults, Go unit tests, integration tests under `tests/integration/api`

---

## Spec Reference

- Spec: `/home/moritz/dev/spectre-via-ssh/.worktrees/embedded-memory-phase1/docs/superpowers/specs/2026-04-05-embedded-near-instant-startup-design.md`
- Implement with `@superpowers:test-driven-development`
- Before claiming success, use `@superpowers:verification-before-completion`

## Scope Boundary

Included:

- active checkpoint plus active tail-journal manifest model
- streaming checkpoint writer and loader
- durable tail-journal ingest and bounded restart recovery
- fast-path startup that skips cold-segment replay
- repair-mode replay IO cleanup for segment rebuilds
- readiness/config/metrics updates for the new startup model
- deployment defaults and restart validation for homelab-style datasets

Not included:

- changing segment bundle format
- removing repair-mode cold replay when no checkpoint exists
- replacing the planner/history query model
- new external storage dependencies

## File Map

**Create:**

- `internal/embeddedstore/tail_journal.go`
  - Active post-checkpoint journal wrapper with append, streamed replay, stats, and rotation helpers.
- `internal/embeddedstore/tail_journal_test.go`
  - Tail append/replay/rotate/crash-recovery tests.
- `internal/embeddedstore/checkpoint_stream_test.go`
  - Focused tests for streamed checkpoint encode/decode without full projection cloning.
- `tests/integration/api/embedded_runtime_fast_restart_test.go`
  - Integration coverage for restart from checkpoint + tail and immediate data availability.

**Modify:**

- `internal/embeddedstore/config.go`
  - Add bounds for tail size and shutdown checkpoint behavior.
- `internal/embeddedstore/backend.go`
  - Validate/default new embedded restart knobs.
- `internal/embeddedstore/manifest.go`
  - Track active checkpoint and active tail metadata, including backward-compatible manifest loading.
- `internal/embeddedstore/checkpoint.go`
  - Replace clone-based checkpoint bundle writing with streamed encode/decode and atomic publish.
- `internal/embeddedstore/checkpoint_test.go`
  - Update round-trip and corruption tests for the new bundle layout.
- `internal/embeddedstore/projection_snapshot.go`
  - Add deterministic streaming iteration/import helpers instead of full-heap snapshot export for checkpoint writing.
- `internal/embeddedstore/engine.go`
  - Hold active tail journal and startup mode metadata.
- `internal/embeddedstore/engine_state.go`
  - Load checkpoint + tail as the normal startup path; use cold replay only for repair mode.
- `internal/embeddedstore/engine_lifecycle.go`
  - Append durably to tail before mutating projection/hot state; checkpoint on bounded tail triggers and clean shutdown.
- `internal/embeddedstore/engine_persistence.go`
  - Wire checkpoint publish, tail rotation, and manifest updates.
- `internal/embeddedstore/engine_test.go`
  - Cover checkpoint publication, tail rotation, and ready-state behavior.
- `internal/embeddedstore/engine_replay_test.go`
  - Convert restart expectations from cold replay to checkpoint + tail recovery; keep repair-mode tests.
- `internal/embeddedstore/journal.go`
  - Reuse framing and add streamed replay callbacks if that is simpler than duplicating the format in `tail_journal.go`.
- `internal/embeddedstore/segment_replay.go`
  - Keep one file handle per cursor and stream sequentially for repair mode.
- `internal/embeddedstore/query_planner.go`
  - Ensure dedupe remains correct when recent events exist in both tail-recovered hot state and cold segments.
- `cmd/spectre/commands/server.go`
  - Add CLI flags for tail-bound checkpoint policy if they do not already exist.
- `cmd/spectre/commands/server_runtime_embedded.go`
  - Pass the new engine config, surface startup-mode logging, and keep readiness tied to the fast open path.
- `cmd/spectre/commands/server_embedded_config_test.go`
  - Validate new config rendering and defaults.
- `chart/values.yaml`
  - Add defaults for tail-bound checkpoint policy.
- `chart/templates/deployment.yaml`
  - Pass the new embedded restart flags when enabled.
- `chart/tests/deployment_embedded_test.yaml`
  - Assert the deployment emits the new arguments.
- `tests/integration/api/embedded_runtime_restart_test.go`
  - Keep restart parity coverage while switching the expected storage path.

## Task 1: Add Active Checkpoint And Tail Metadata

**Files:**
- Modify: `internal/embeddedstore/config.go`
- Modify: `internal/embeddedstore/backend.go`
- Modify: `internal/embeddedstore/manifest.go`
- Modify: `internal/embeddedstore/manifest_test.go`
- Modify: `cmd/spectre/commands/server.go`
- Modify: `cmd/spectre/commands/server_runtime_embedded.go`
- Modify: `cmd/spectre/commands/server_embedded_config_test.go`
- Test: `internal/embeddedstore/manifest_test.go`
- Test: `cmd/spectre/commands/server_embedded_config_test.go`

- [ ] **Step 1: Write failing manifest and config tests for the active head-state model**

```go
func TestManifest_LoadLegacyManifestWithoutTailMetadata(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, storeManifest(dir, Manifest{
		FormatVersion: storageFormatVersion,
		ActiveSegments: []SegmentMeta{{ID: "seg-001", HighWaterMark: 42}},
		Checkpoints: []CheckpointMeta{{ID: "chk-00000000000000000042-1", HighWaterMark: 42}},
	}))

	manifest, err := loadOrCreateManifest(dir)
	require.NoError(t, err)
	require.Equal(t, uint64(42), manifest.ActiveCheckpoint.HighWaterMark)
	require.Equal(t, uint64(42), manifest.ActiveTail.BaseHighWaterMark)
}

func TestConfig_EffectiveEngineConfigAppliesTailDefaults(t *testing.T) {
	cfg, err := Config{DataDir: t.TempDir()}.EffectiveEngineConfig()
	require.NoError(t, err)
	require.Equal(t, 2048, cfg.CheckpointMaxTailEvents)
	require.True(t, cfg.CheckpointOnShutdown)
}
```

- [ ] **Step 2: Run the focused config and manifest tests**

Run: `go test ./internal/embeddedstore ./cmd/spectre/commands -run 'TestManifest_|TestConfig_EffectiveEngineConfig|TestEmbeddedStoreConfig' -count=1`
Expected: FAIL because active checkpoint/tail fields and new config defaults do not exist yet.

- [ ] **Step 3: Implement manifest v2-compatible metadata and CLI/config plumbing**

Use this shape in `internal/embeddedstore/manifest.go`:

```go
type TailJournalMeta struct {
	ID                string `json:"id"`
	BaseHighWaterMark uint64 `json:"base_high_water_mark"`
	LastHighWaterMark uint64 `json:"last_high_water_mark"`
	EventCount        int    `json:"event_count"`
	SizeBytes         int64  `json:"size_bytes"`
}

type Manifest struct {
	FormatVersion      int              `json:"format_version"`
	ActiveSegments     []SegmentMeta    `json:"active_segments"`
	ActiveCheckpoint   CheckpointMeta   `json:"active_checkpoint"`
	ActiveTail         TailJournalMeta  `json:"active_tail"`
	Checkpoints        []CheckpointMeta `json:"checkpoints"`
	FlushHighWaterMark uint64           `json:"flush_high_water_mark"`
}
```

Add engine config defaults for:

- `CheckpointMaxTailEvents`
- `CheckpointMaxTailBytes`
- `CheckpointOnShutdown`

Keep legacy manifests readable by deriving `ActiveCheckpoint` from `latestCheckpointMeta(...)` and by synthesizing an empty tail rooted at that high-water mark.

- [ ] **Step 4: Re-run the focused config and manifest tests**

Run: `go test ./internal/embeddedstore ./cmd/spectre/commands -run 'TestManifest_|TestConfig_EffectiveEngineConfig|TestEmbeddedStoreConfig' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit the config/manifest slice**

```bash
git add internal/embeddedstore/config.go \
        internal/embeddedstore/backend.go \
        internal/embeddedstore/manifest.go \
        internal/embeddedstore/manifest_test.go \
        cmd/spectre/commands/server.go \
        cmd/spectre/commands/server_runtime_embedded.go \
        cmd/spectre/commands/server_embedded_config_test.go
git commit -m "feat: add embedded active checkpoint and tail metadata"
```

## Task 2: Replace Clone-Based Checkpoints With Streaming Checkpoint Bundles

**Files:**
- Modify: `internal/embeddedstore/checkpoint.go`
- Modify: `internal/embeddedstore/checkpoint_test.go`
- Modify: `internal/embeddedstore/projection_snapshot.go`
- Modify: `internal/embeddedstore/projection.go`
- Create: `internal/embeddedstore/checkpoint_stream_test.go`
- Test: `internal/embeddedstore/checkpoint_test.go`
- Test: `internal/embeddedstore/checkpoint_stream_test.go`

- [ ] **Step 1: Write failing tests that prove checkpoint writing no longer requires full snapshot export**

```go
func TestCheckpoint_StreamRoundTripProjectionState(t *testing.T) {
	dir := t.TempDir()
	projection, err := BuildProjection(makeReplayHeavyEvents(500))
	require.NoError(t, err)

	meta, err := writeCheckpoint(dir, projection, 500)
	require.NoError(t, err)

	restored, highWaterMark, err := loadCheckpoint(dir, meta)
	require.NoError(t, err)
	require.Equal(t, uint64(500), highWaterMark)
	require.Equal(t, projection.ResourceCount(), restored.ResourceCount())
}

func TestCheckpoint_WritesStreamBundleFiles(t *testing.T) {
	dir := t.TempDir()
	projection, err := BuildProjection(makeReplayHeavyEvents(50))
	require.NoError(t, err)

	meta, err := writeCheckpoint(dir, projection, 7)
	require.NoError(t, err)

	checkpointDir := filepath.Join(dir, checkpointsDirName, meta.ID)
	require.FileExists(t, filepath.Join(checkpointDir, "meta.json"))
	require.FileExists(t, filepath.Join(checkpointDir, "resources.ndjson"))
	require.FileExists(t, filepath.Join(checkpointDir, "k8s-events.json"))
}
```

- [ ] **Step 2: Run the checkpoint-focused tests**

Run: `go test ./internal/embeddedstore -run 'TestCheckpoint_' -count=1`
Expected: FAIL because `writeCheckpoint(...)` still builds `Projection.ExportSnapshot()`.

- [ ] **Step 3: Implement streamed checkpoint encode/decode**

Write checkpoints as a bundle like:

```text
checkpoints/<checkpoint-id>/meta.json
checkpoints/<checkpoint-id>/resources.ndjson
checkpoints/<checkpoint-id>/k8s-events.json
```

In `projection_snapshot.go`, add deterministic iteration helpers such as:

```go
func (p *Projection) StreamCheckpointResources(emit func(ProjectionResourceSnapshot) error) error
func ProjectionFromCheckpointStream(meta checkpointState, resources io.Reader, k8s io.Reader) (*Projection, error)
```

Keep the commit sequence atomic:

- write temp bundle
- fsync files and temp dir
- rename into `checkpoints/`
- fsync checkpoint root

- [ ] **Step 4: Re-run the checkpoint-focused tests**

Run: `go test ./internal/embeddedstore -run 'TestCheckpoint_' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit the streaming checkpoint slice**

```bash
git add internal/embeddedstore/checkpoint.go \
        internal/embeddedstore/checkpoint_test.go \
        internal/embeddedstore/checkpoint_stream_test.go \
        internal/embeddedstore/projection_snapshot.go \
        internal/embeddedstore/projection.go
git commit -m "feat: stream embedded checkpoints without snapshot cloning"
```

## Task 3: Add Durable Tail Journal Ingest And Rotation

**Files:**
- Create: `internal/embeddedstore/tail_journal.go`
- Create: `internal/embeddedstore/tail_journal_test.go`
- Modify: `internal/embeddedstore/journal.go`
- Modify: `internal/embeddedstore/journal_test.go`
- Modify: `internal/embeddedstore/engine.go`
- Modify: `internal/embeddedstore/engine_lifecycle.go`
- Modify: `internal/embeddedstore/engine_persistence.go`
- Modify: `internal/embeddedstore/engine_test.go`
- Test: `internal/embeddedstore/tail_journal_test.go`
- Test: `internal/embeddedstore/engine_test.go`

- [ ] **Step 1: Write failing tests for durable post-checkpoint replay and checkpoint-driven tail rotation**

```go
func TestEngine_ReopenRestoresTailEventsWithoutColdReplay(t *testing.T) {
	dir := t.TempDir()
	engine, err := OpenEngine(EngineConfig{DataDir: dir, CheckpointMaxTailEvents: 32})
	require.NoError(t, err)

	require.NoError(t, engine.ProcessBatch(context.Background(), makeReplayHeavyEvents(20)))
	require.NoError(t, engine.Checkpoint(context.Background()))
	require.NoError(t, engine.ProcessBatch(context.Background(), makeReplayHeavyEventsFrom(21, 5)))
	require.NoError(t, engine.Close())

	restore := setApplyProjectionEventFnForTest(func(*Projection, models.Event) error {
		t.Fatal("normal restart must not rebuild head state from cold replay")
		return nil
	})
	t.Cleanup(restore)

	reopened, err := OpenEngine(EngineConfig{DataDir: dir})
	require.NoError(t, err)
	require.Equal(t, uint64(25), reopened.nextHighWaterMark)
}

func TestTailJournal_RotateAfterCheckpointPublish(t *testing.T) {
	dir := t.TempDir()
	journal, err := openTailJournal(dir, TailJournalMeta{ID: "tail-a", BaseHighWaterMark: 10})
	require.NoError(t, err)

	require.NoError(t, journal.AppendBatch(context.Background(), 11, sampleTailEvents(3)))
	nextMeta, err := journal.Rotate(13)
	require.NoError(t, err)
	require.Equal(t, uint64(13), nextMeta.BaseHighWaterMark)
	require.Zero(t, nextMeta.EventCount)
}
```

- [ ] **Step 2: Run the tail-journal and engine tests**

Run: `go test ./internal/embeddedstore -run 'TestTailJournal_|TestEngine_ReopenRestoresTailEventsWithoutColdReplay|TestEngine_' -count=1`
Expected: FAIL because ingest is not durable to a tail journal and checkpoint rotation does not exist.

- [ ] **Step 3: Implement tail journal append/replay/rotate and engine durability ordering**

In `tail_journal.go`, add an active wrapper like:

```go
type tailJournal struct {
	meta    TailJournalMeta
	journal *Journal
}

func (t *tailJournal) AppendBatch(ctx context.Context, nextHighWaterMark uint64, events []models.Event) (TailJournalMeta, error)
func (t *tailJournal) ReplaySince(ctx context.Context, afterHighWaterMark uint64, apply func(models.Event, uint64) error) error
func (t *tailJournal) Rotate(newBaseHighWaterMark uint64) (TailJournalMeta, error)
```

In `engine_lifecycle.go`, change `ProcessBatch(...)` ordering to:

1. append to tail journal durably
2. apply to projection
3. append to hot store
4. advance high-water mark

Do not truncate tail on hot flush. Tail is only rotated after a newer checkpoint is committed.

- [ ] **Step 4: Re-run the tail-journal and engine tests**

Run: `go test ./internal/embeddedstore -run 'TestTailJournal_|TestEngine_ReopenRestoresTailEventsWithoutColdReplay|TestEngine_' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit the tail durability slice**

```bash
git add internal/embeddedstore/tail_journal.go \
        internal/embeddedstore/tail_journal_test.go \
        internal/embeddedstore/journal.go \
        internal/embeddedstore/journal_test.go \
        internal/embeddedstore/engine.go \
        internal/embeddedstore/engine_lifecycle.go \
        internal/embeddedstore/engine_persistence.go \
        internal/embeddedstore/engine_test.go
git commit -m "feat: add durable embedded tail journal recovery"
```

## Task 4: Make Checkpoint Plus Tail The Normal Startup Path

**Files:**
- Modify: `internal/embeddedstore/engine_state.go`
- Modify: `internal/embeddedstore/engine_replay_test.go`
- Modify: `internal/embeddedstore/segment_replay.go`
- Modify: `internal/embeddedstore/query_planner.go`
- Modify: `internal/embeddedstore/query_planner_test.go`
- Test: `internal/embeddedstore/engine_replay_test.go`
- Test: `internal/embeddedstore/query_planner_test.go`

- [ ] **Step 1: Write failing tests for fast-path startup and repair-mode replay**

```go
func TestEngine_OpenUsesCheckpointAndTailOnNormalRestart(t *testing.T) {
	dir := t.TempDir()
	root := embeddedRootDir(dir)
	seedStoreWithCheckpointAndTail(t, root)

	restore := setApplyProjectionEventFnForTest(func(*Projection, models.Event) error {
		t.Fatal("normal restart should not apply cold segment replay")
		return nil
	})
	t.Cleanup(restore)

	engine, err := OpenEngine(EngineConfig{DataDir: dir})
	require.NoError(t, err)
	require.True(t, engine.IsReady())
}

func TestSegmentReplayCursor_KeepsFileOpenAcrossSequentialReads(t *testing.T) {
	reader := writeReplayTestSegment(t)
	cursor := newSegmentReplayCursor(replaySegmentReader{segmentID: "seg-1", reader: reader})

	first, ok, err := cursor.next(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
	second, ok, err := cursor.next(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
	require.NotEqual(t, first.ID, second.ID)
}
```

- [ ] **Step 2: Run the replay and planner tests**

Run: `go test ./internal/embeddedstore -run 'TestEngine_Open|TestSegmentReplayCursor_|TestQueryPlanner_' -count=1`
Expected: FAIL because `loadEngineState(...)` still replays cold segments on normal restart.

- [ ] **Step 3: Rework startup recovery and repair-mode replay**

In `engine_state.go`, make the normal load path:

```go
func loadEngineState(rootDir string, manifest Manifest) ([]*segmentReader, *tailJournal, uint64, *Projection, *hotStore, startupMode, error)
```

Behavior:

- always open segment readers for query serving
- if `manifest.ActiveCheckpoint` is usable:
  - load checkpoint
  - replay only `manifest.ActiveTail`
  - rebuild hot store from the same tail events
  - return `startupModeFast`
- otherwise:
  - replay cold segments with the repair path
  - return `startupModeRepair`

In `segment_replay.go`, hold an open `*os.File` on the cursor and close it when the cursor is exhausted.

In `query_planner.go`, keep dedupe-by-ID stable so events present in both tail-recovered hot state and flushed cold segments still merge deterministically.

- [ ] **Step 4: Re-run the replay and planner tests**

Run: `go test ./internal/embeddedstore -run 'TestEngine_Open|TestSegmentReplayCursor_|TestQueryPlanner_' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit the fast-startup recovery slice**

```bash
git add internal/embeddedstore/engine_state.go \
        internal/embeddedstore/engine_replay_test.go \
        internal/embeddedstore/segment_replay.go \
        internal/embeddedstore/query_planner.go \
        internal/embeddedstore/query_planner_test.go
git commit -m "feat: make checkpoint plus tail the embedded restart path"
```

## Task 5: Bound Tail Growth, Re-Enable Safe Checkpointing, And Validate Runtime Behavior

**Files:**
- Modify: `internal/embeddedstore/engine_lifecycle.go`
- Modify: `internal/embeddedstore/engine_persistence.go`
- Modify: `internal/embeddedstore/metrics.go`
- Modify: `internal/embeddedstore/metrics_test.go`
- Modify: `tests/integration/api/embedded_runtime_restart_test.go`
- Create: `tests/integration/api/embedded_runtime_fast_restart_test.go`
- Modify: `chart/values.yaml`
- Modify: `chart/templates/deployment.yaml`
- Modify: `chart/tests/deployment_embedded_test.yaml`
- Test: `internal/embeddedstore/metrics_test.go`
- Test: `tests/integration/api/embedded_runtime_restart_test.go`
- Test: `tests/integration/api/embedded_runtime_fast_restart_test.go`

- [ ] **Step 1: Write failing tests for tail-bound checkpoint triggers and immediate post-restart data availability**

```go
func TestEngine_AutoCheckpointWhenTailExceedsEventBudget(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{
		DataDir:                 t.TempDir(),
		CheckpointMaxTailEvents: 4,
		CheckpointOnShutdown:    true,
	})
	require.NoError(t, err)

	require.NoError(t, engine.ProcessBatch(context.Background(), makeReplayHeavyEvents(5)))
	require.GreaterOrEqual(t, len(engine.manifest.Checkpoints), 1)
	require.LessOrEqual(t, engine.manifest.ActiveTail.EventCount, 1)
}

func TestEmbeddedRuntimeFastRestartServesTimelineImmediately(t *testing.T) {
	dir := t.TempDir()
	seedEmbeddedRuntimeForFastRestart(t, dir)

	backend, err := embeddedstore.Open(embeddedstore.Config{DataDir: dir})
	require.NoError(t, err)
	require.True(t, backend.IsReady())

	server := newEmbeddedRuntimeServer(t, backend)
	response := queryEmbeddedTimeline(t, server, 0, 1_000_000)
	require.NotEmpty(t, response.Resources)
}
```

- [ ] **Step 2: Run metrics, integration, and chart tests**

Run: `go test ./internal/embeddedstore -run 'TestEngine_AutoCheckpointWhenTailExceedsEventBudget|TestMetrics_' -count=1`
Run: `go test ./tests/integration/api -run 'TestEmbeddedRuntime(Restart|FastRestart)' -count=1`
Run: `helm unittest ./chart`
Expected: FAIL because tail-bound checkpoint triggers, integration timing, and chart args are not implemented yet.

- [ ] **Step 3: Implement checkpoint triggers, metrics, and deployment defaults**

Add engine logic that checkpoints when any bound is exceeded:

- `CheckpointMaxTailEvents`
- `CheckpointMaxTailBytes`
- `CheckpointInterval`
- clean shutdown

Expose metrics/logging for:

- startup mode (`fast` vs `repair`)
- replayed tail event count
- active tail event count/bytes
- checkpoint duration

Update chart args only after the engine supports safe streaming checkpoints again. Keep the rollout defaults conservative but non-zero, for example:

- `--embedded-checkpoint-interval=15m`
- `--embedded-checkpoint-max-tail-events=2048`

- [ ] **Step 4: Re-run metrics, integration, and chart tests**

Run: `go test ./internal/embeddedstore -run 'TestEngine_AutoCheckpointWhenTailExceedsEventBudget|TestMetrics_' -count=1`
Run: `go test ./tests/integration/api -run 'TestEmbeddedRuntime(Restart|FastRestart)' -count=1`
Run: `helm unittest ./chart`
Expected: PASS.

- [ ] **Step 5: Commit the runtime validation slice**

```bash
git add internal/embeddedstore/engine_lifecycle.go \
        internal/embeddedstore/engine_persistence.go \
        internal/embeddedstore/metrics.go \
        internal/embeddedstore/metrics_test.go \
        tests/integration/api/embedded_runtime_restart_test.go \
        tests/integration/api/embedded_runtime_fast_restart_test.go \
        chart/values.yaml \
        chart/templates/deployment.yaml \
        chart/tests/deployment_embedded_test.yaml
git commit -m "feat: bound embedded restart tail and validate fast startup"
```

## Verification Checklist

- [ ] Run the focused embedded-store unit suites from Tasks 1-5.
- [ ] Run the embedded runtime integration tests:
  `go test ./tests/integration/api -run 'TestEmbeddedRuntime(Restart|FastRestart)' -count=1`
- [ ] Run chart validation:
  `helm unittest ./chart`
- [ ] Build the binary:
  `make build`
- [ ] On homelab, deploy the resulting image and confirm:
  - pod reaches readiness in under 15 seconds after a restart
  - logs report `startup_mode=fast`
  - reported tail replay count stays within the configured bound
  - timeline data is available immediately after readiness

## Manual Homelab Validation

After implementation, use the existing `homelab-admin@kubernetes` context and `monitoring/spectre` release:

1. Build and push an image for the branch.
2. Patch the deployment image and embedded flags.
3. Restart the pod once to create a fresh streaming checkpoint.
4. Restart it again and measure the true steady-state restart path.

Suggested commands:

```bash
kubectl --context homelab-admin@kubernetes -n monitoring rollout restart deploy/spectre
kubectl --context homelab-admin@kubernetes -n monitoring rollout status deploy/spectre --timeout=5m
kubectl --context homelab-admin@kubernetes -n monitoring logs deploy/spectre --since=10m | rg 'startup_mode|tail|checkpoint'
```

Success is not "startup improved." Success is:

- second restart after the new checkpoint path is active is under 15 seconds
- no checkpoint OOM
- data is queryable immediately
