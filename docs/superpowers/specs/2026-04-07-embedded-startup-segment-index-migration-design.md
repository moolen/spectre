# Embedded Startup Segment Index Migration Design

**Date:** 2026-04-07

**Status:** Approved for implementation

## Goal

Backfill the new embedded `associated.idx` segment artifact onto existing historical segments by running a one-time startup migration when needed.

The migration must:

- run automatically during embedded startup
- be necessary only once for a given data directory
- be safe to retry after interruption or failure
- preserve query correctness throughout

## Context

The current embedded engine writes `associated.idx` for newly created segments. That index is used to accelerate Event-kind lookups for timeline resource attachments.

On the live homelab dataset, the code path is deployed and working, but nearly all historical segments predate the new artifact. In practice:

- startup is already fast because checkpoint loading is working well
- timeline metadata calls are already cheap
- timeline open is still dominated by resource-event history reads
- most historical associated-event reads still fall back to full segment scans

The missing piece is not query logic. The missing piece is adoption of the new segment format on old persisted segments.

## Non-Goals

- changing external API behavior
- changing the logical meaning of segment history
- introducing a general-purpose migration framework for all future storage changes
- rewriting segments in the background after readiness
- requiring an operator-triggered maintenance command

## Requirements

The startup migration must satisfy all of the following:

1. It runs only when the active embedded dataset has not yet been migrated to the new associated-event index generation.
2. It rewrites only persisted cold segments, not checkpoints or tail journals.
3. It preserves the existing `HighWaterMark` mapping in manifest segment metadata.
4. It does not expose a partially migrated manifest.
5. If interrupted, the next startup retries safely.
6. Once successful, later startups skip the migration entirely.

## Decision

Adopt a **manifest-gated one-shot startup migration**.

The manifest becomes the durable source of truth for whether the embedded data directory has already been migrated to the associated-event index generation. Startup checks that marker before building the query planner. If migration is needed, startup rewrites the active segment set once, updates the manifest atomically, and only then continues normal engine open.

This is preferred over heuristic scanning on every startup because it makes completion explicit and keeps later restarts cheap and predictable.

## Data Model Change

Extend the embedded manifest with a persisted segment index generation marker.

Example shape:

```json
{
  "format_version": 1,
  "segment_index_generation": 1
}
```

Semantics:

- `0` or omitted: legacy dataset, startup migration may be required
- `1`: dataset migrated to the associated-event index generation

This field is independent from `format_version`. The manifest format itself is still compatible; the new field only records the segment feature generation for this dataset.

## Migration Trigger

Startup runs the migration check after manifest load and before query planner refresh.

The engine should migrate if:

- embedded startup is using persisted data
- `manifest.SegmentIndexGeneration < associatedEventIndexGeneration`
- there is at least one active segment

If generation is already current, startup skips the migration without scanning segment contents.

## Migration Algorithm

For each active segment in manifest order:

1. Open the existing segment reader using the current manifest metadata.
2. Read the segment metadata to obtain the full timestamp bounds.
3. Scan the segment events across its full range.
4. Write a replacement segment using the current segment writer, which emits:
   - `events.bin`
   - `time.idx`
   - `resource.idx`
   - `associated.idx`
   - `dim.idx`
   - `stats.json`
5. Build a replacement `SegmentMeta` using:
   - the new segment bundle metadata
   - the original segment `HighWaterMark`

After all replacement segments are written successfully:

1. Build a new manifest value containing the replacement segment list.
2. Set `segment_index_generation` to the target generation.
3. Persist the manifest atomically.
4. Remove the old segment directories.
5. Continue startup using the replacement readers.

## Atomicity And Failure Handling

The manifest update is the commit point.

Before the manifest swap:

- old manifest still points at old segments
- replacement segments may exist on disk as temporary or newly written directories
- startup must not delete old segments yet

After the manifest swap:

- startup may delete the old segment directories
- if cleanup fails, startup should surface the error and leave the manifest on the new segment set
- future startups should succeed using the new segment set even if orphaned old segment directories remain

Failure policy:

- if any segment rewrite fails before manifest update, abort startup with an error and keep the manifest unchanged
- if startup is interrupted before manifest update, the next startup retries migration
- if startup is interrupted after manifest update but before old-segment cleanup, later startup uses the migrated segment set and may optionally ignore or clean up orphaned legacy directories

## Startup Flow

Revised embedded startup sequence:

1. Load manifest.
2. If `segment_index_generation` is stale, rewrite the active segment set.
3. Re-open segment readers from the committed segment set.
4. Load checkpoint.
5. Recover hot state and tail.
6. Refresh query planner.
7. Mark engine ready.

The migration is a startup prerequisite because the goal is to make the optimized query path available immediately after readiness.

## Logging And Observability

Startup logs should make migration behavior explicit:

- migration required / skipped
- number of segments to rewrite
- number of segments rewritten
- total duration
- failure reason if aborted

Recommended metrics:

- startup migration total
- startup migration errors total
- startup migration duration
- startup migration segments rewritten

The minimum requirement is clear logs even if metrics are deferred.

## Compatibility

Backward compatibility requirements:

- manifests without `segment_index_generation` remain readable
- legacy segments without `associated.idx` remain readable until migrated
- no checkpoint rewrite is required for this feature
- no API or Helm config change is required to enable the migration

## Risks

### Startup Time Spike During First Post-Deploy Boot

The first startup on a large dataset will be slower because it rewrites historical segments before readiness.

This is acceptable because:

- it is a one-time cost
- later startups stay fast
- it avoids making the query path pay the penalty forever

### Disk Amplification During Migration

Migration temporarily needs room for both old and rewritten segments.

This is acceptable if the migration writes replacement segments first and deletes old ones only after manifest commit. Operationally, this should be called out because low free space can block the migration.

### Partial Rewrite Confusion

If startup crashes mid-migration, orphaned rewritten segments may remain on disk.

This is acceptable as long as manifest commit remains atomic and startup only trusts the manifest, not directory enumeration.

## Acceptance Criteria

The design is successful only if all of the following are true:

- a legacy embedded dataset with old segments is migrated automatically on startup
- the migrated manifest records the new segment index generation
- a second startup does not rewrite the same dataset again
- migrated segments contain `associated.idx`
- timeline resource attachment queries use indexed lookups for migrated historical segments
- query correctness remains unchanged before and after migration
- interrupted or failed migration attempts leave the old manifest usable
