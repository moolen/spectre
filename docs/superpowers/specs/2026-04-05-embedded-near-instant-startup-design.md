# Embedded Near-Instant Startup Design

**Date:** 2026-04-05

**Status:** Drafted from homelab investigation

## Goal

When embedded data already exists on disk, Spectre should restart to a healthy, queryable state in under 15 seconds on the homelab `monitoring/spectre` dataset.

That target is specifically about the steady-state restart path:

- process restart with an existing embedded data directory
- API data available immediately after readiness
- no long replay window proportional to uptime
- no checkpoint-related OOM on a small cluster

## Current Problem

The live homelab dataset is about 2.1 GiB on disk. The current embedded startup path still takes roughly 84-90 seconds after the metadata fast path landed.

The remaining cost is structural:

- `cmd/spectre/commands/server_runtime_embedded.go` blocks API startup on `embeddedstore.Open(...)`
- `internal/embeddedstore/engine_state.go` restores the latest checkpoint and then replays every newer cold segment before `OpenEngine(...)` returns
- the live PVC currently has a latest checkpoint at high-water mark `113143`, while `flush_high_water_mark` is around `147.6k`
- that leaves about 34.5k events to replay across about 1.6k segment bundles on every restart
- `internal/embeddedstore/segment_replay.go` re-opens and re-seeks the events file for every decoded event, which makes the replay path slower than necessary
- `internal/embeddedstore/checkpoint.go` still builds checkpoints from `projection.ExportSnapshot()`, and `internal/embeddedstore/projection_snapshot.go` clones the full projection into memory first
- enabling periodic checkpoints on the live dataset caused OOMKilled, so live was rolled back to `--embedded-checkpoint-interval=0`

The main issue is not a missing micro-optimization. The main issue is that restart time still scales with "all events after the last checkpoint," and checkpoint creation is too memory-hungry to keep that replay window small.

## Non-Goals

- Replacing immutable segment storage as the history source of truth
- Introducing a new external database
- Making first-ever boot from a segment-only dataset fast
- Reworking API query semantics
- Solving every embedded memory optimization in the same change set

First boot from legacy segment-only state may still be slower. The required guarantee is: once a checkpointed embedded store exists on disk, restart must stay fast.

## Acceptance Criteria

The design is successful only if all of the following are true on the homelab `monitoring/spectre` dataset:

- restart from an already-populated PVC reaches `/health` success in under 15 seconds
- timeline, export, metadata, and analysis reads work immediately after readiness without a background "history catch-up" phase
- normal restart does not replay cold segments to rebuild head state
- startup replay work is bounded by a small post-checkpoint delta journal, not by hours of accumulated segment history
- periodic checkpoints no longer OOM at the current memory limit
- an unclean shutdown still recovers all committed events

## Architectural Decision

Adopt a **head checkpoint + tail journal** restart model.

### Head Checkpoint

Persist the compact projection state needed for:

- current resource state
- resource metadata
- ordered resource index
- analysis relationships derived from the compact resource versions
- compact Kubernetes event summaries

This checkpoint is the restart baseline for head-state reconstruction.

### Tail Journal

Persist every event after the current checkpoint in an append-only durable journal.

The tail journal is:

- written before mutating in-memory state
- retained across hot flushes to immutable segments
- rotated only after a newer checkpoint is durably committed

This guarantees crash recovery without forcing startup to replay cold segments.

### Cold Segments

Cold segments remain the history source for:

- timeline queries
- export queries
- Event-kind history queries
- older metadata presence checks

Segments are no longer the normal restart source for projection rebuilds.

## Why The Tail Must Survive Flushes

Hot flushes write immutable history segments, but they do not update the head checkpoint.

If Spectre flushed hot data to segments and then deleted the recent journal immediately, a restart would have this problem:

- checkpoint restores old head state
- post-checkpoint events exist only in cold segments
- startup would need to replay those cold segments again to rebuild current resource state

That is the exact failure mode causing the current 84-90 second restart.

So the invariant must be:

- **all events newer than the active checkpoint remain recoverable from the tail journal**

The journal may duplicate data that is also already present in segments. That duplication is intentional because it bounds restart cost.

## Target On-Disk Model

The embedded root keeps three persistent layers:

1. `segments/`
   Immutable historical segment bundles used for query-time reads.
2. `checkpoints/<id>/`
   Streaming-written compact head-state checkpoint bundle.
3. `tail/`
   The current post-checkpoint journal.

The manifest becomes the authoritative pointer to the active head-state set:

- active cold segments
- active checkpoint
- active tail journal
- checkpoint high-water mark
- tail high-water mark and bounded size metadata

Existing segment metadata stays valid. Existing checkpoint directories stay valid if they can be decoded into the new active-checkpoint pointer model.

## Streaming Checkpoint Format

Checkpoint creation must stop cloning the full projection.

The checkpoint writer should emit a bundle with small metadata plus streamed resource snapshots, for example:

- `meta.json`
- `resources.ndjson`
- `k8s-events.json`

Key properties:

- no `Projection.ExportSnapshot()` of the entire dataset into a second giant Go object
- resource records are encoded directly while iterating the projection in deterministic order
- load path restores resources incrementally from the checkpoint files
- checkpoint commit is atomic: write to temp dir, fsync, rename, update manifest

The checkpoint still represents the same logical compact projection as today. The change is how it is encoded and committed.

## Startup Flow

### Normal Fast Path

1. Load manifest.
2. Open active segment readers for query serving.
3. Load the active checkpoint into a fresh projection.
4. Replay only the active tail journal entries newer than the checkpoint high-water mark into:
   - projection
   - hot store
5. Build the query planner from:
   - projection metadata/state
   - recovered hot store
   - active cold segments
6. Mark engine ready.

This path must not read cold segments to rebuild head state.

### Repair Path

If there is no usable checkpoint or the tail journal is inconsistent, the engine falls back to repair mode:

- rebuild from cold segments using the existing replay machinery
- keep repair mode explicit in logs and metrics
- write a fresh streaming checkpoint as soon as possible after recovery

Repair mode is acceptable for first boot, migration recovery, or corrupted local state. It is not acceptable as the steady-state restart path.

## Ingest Flow

For each new event:

1. append to the active tail journal and fsync
2. apply to the compact projection
3. append to the hot store
4. expose it to queries immediately
5. flush hot batches to immutable segments asynchronously as today

This makes the tail journal the durable write-ahead log for post-checkpoint state.

## Checkpoint Rotation Flow

To keep restart bounded, checkpointing must be driven by tail size, not only by wall clock.

Recommended triggers:

- max tail events
- max tail bytes
- optional max tail age
- clean shutdown

Checkpoint rotation sequence:

1. freeze the current checkpoint high-water mark target
2. stream-write a new checkpoint from the live projection
3. atomically publish the new checkpoint in the manifest
4. rotate the tail journal so new events go to an empty journal rooted at the new checkpoint high-water mark
5. garbage-collect superseded checkpoint bundles and stale tail files

This is what makes restart cost proportional to "small bounded tail" instead of "all time since the last safe checkpoint."

## Query Availability

The user requirement is not just "health goes green quickly." Data must also be available immediately.

That means:

- head-state reads come from checkpoint + tail replay
- timeline/export/history reads come from hot + cold planner immediately after startup
- no asynchronous backfill job is allowed to gate historical queries

This is already directionally aligned with the embedded memory work: projection stores current analytical state, while history queries come from hot and cold persisted event stores.

## Replay IO Fix

Even after normal startup stops depending on cold replay, the repair path still matters.

`internal/embeddedstore/segment_replay.go` should keep a segment file handle open per replay cursor and decode sequentially instead of:

- opening the same file per event
- seeking for every record
- closing it again

This does not solve the main steady-state startup problem by itself, but it makes repair mode and tests materially faster and less wasteful.

## Readiness Semantics

Readiness should continue to mean:

- embedded engine opened successfully
- head state loaded
- immediate query serving is available

The change is that `OpenEngine(...)` becomes fast enough that the current startup wiring is acceptable. There is no need to make the API "ready before the data exists."

Operationally, Spectre should expose enough logging and metrics to distinguish:

- fast-path startup
- repair startup
- checkpoint age
- tail replay count
- tail replay bytes
- checkpoint duration

## Migration And Rollout

### Existing Data Directories

- existing `segments/` stay readable
- existing checkpoint bundles stay readable if they can be mapped to the active checkpoint pointer
- if a store has segments but no usable checkpoint, the first restart after upgrade may still enter repair mode
- after the first successful streaming checkpoint, subsequent restarts use the fast path

### Live Rollout

1. implement streaming checkpoints and tail-journal recovery
2. verify periodic checkpoints are safe again on homelab
3. re-enable checkpoint scheduling in deployment defaults only after memory verification
4. deploy to homelab and measure restart time against the existing 84-90 second baseline

## Risks

### Manifest migration complexity

The manifest is becoming the pointer to active checkpoint plus active tail. Incomplete migration logic could strand valid old data or trigger unnecessary repair.

Mitigation:

- add explicit backward-compat tests for old manifest layouts
- treat repair mode as a safe fallback, not as silent corruption handling

### Tail duplication across hot and cold storage

Recent events will exist in both the tail journal and cold segments until the next checkpoint rotation.

Mitigation:

- continue deduping by event ID in planner merge paths
- keep the tail small via checkpoint triggers

### Concurrent checkpoint rotation and ingest

Checkpoint publication and tail rotation must not lose post-checkpoint writes.

Mitigation:

- define a strict lock/commit order inside the engine
- use atomic manifest swap only after the checkpoint bundle and new tail are durable

## Result

This design changes restart from:

- load checkpoint
- replay every cold segment newer than the checkpoint

to:

- load checkpoint
- replay only a bounded tail journal

That is the only credible path to the user requirement that persisted embedded data should restart in under 15 seconds and be queryable immediately.
