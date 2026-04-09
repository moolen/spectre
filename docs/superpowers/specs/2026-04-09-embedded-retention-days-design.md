# Embedded Retention Days Design

**Date:** 2026-04-09

**Status:** Approved in-session, documenting before implementation

## Goal

Add `--embedded-retention-days` so operators can cap embedded on-disk event age with one time-based policy.

The flag must control both:

- raw embedded event storage on disk
- checkpoint/projection history used for analysis and query fallbacks

`0` means retention is disabled.

## Problem

The embedded store currently has no general time-based raw retention. Old raw segments remain on disk until operators delete the PVC or the engine rewrites them for unrelated reasons.

The projection/checkpoint side already has a fixed 24 hour history window, but that window is:

- hardcoded
- not operator-configurable
- not tied to raw segment cleanup

That means disk usage and retained analysis/query history drift apart.

## Requirements

- Add a new server flag: `--embedded-retention-days`
- `0` disables retention
- Retention uses event timestamps, not file mtimes
- Raw segments older than the retention cutoff must be removed from the embedded store
- Mixed-age raw segments must be rewritten with only retained events
- Query/analysis history retained in checkpoints and projection state must follow the same window
- Startup must reconcile expired embedded data already on disk
- Retention enforcement may be checkpoint-paced rather than immediate on every ingest

## Non-Goals

- Instant per-event deletion during live ingest
- A second retention policy for raw data and another for checkpoint history
- Format changes that require a breaking migration

## Design

### Configuration

Add `EmbeddedRetentionDays` to embedded config and engine config.

- `0`: disabled
- `>0`: keep events whose timestamp is within `N * 24h` of `time.Now()`
- `<0`: invalid configuration

The server CLI, embedded runtime config, and Helm chart all expose the same flag/value.

### Retention Cutoff

Retention is computed from wall clock time, but compared against event timestamps:

- `cutoff = time.Now().Add(-retentionWindow)`
- an event is retained when `event.Timestamp >= cutoff`

This satisfies the requirement that expiry is based on event time instead of filesystem metadata.

### Raw Storage Retention

Retention is enforced in two places:

1. during startup reconciliation, before the engine builds serving state
2. after a successful checkpoint, before optional compaction

For raw segments:

- if `segment.MaxTimestamp < cutoff`, delete the segment
- if `segment.MinTimestamp >= cutoff`, keep it unchanged
- otherwise rewrite the segment with only retained events

For the active tail journal at startup:

- replay entries
- keep only retained events
- rewrite the active tail journal contiguously

Checkpoint-time enforcement does not need special tail handling because a successful checkpoint already rotates to a fresh tail and stale tails are deleted.

Hot in-memory events are also filtered during checkpoint-time retention so live queries stop serving expired raw events after the retention cycle completes.

### Checkpoint / Projection Retention

The existing fixed 24 hour checkpoint-history window becomes configurable.

Projection retention keeps enough state to preserve current resource visibility:

- versions inside the retention window are kept
- the newest version before the cutoff may be kept as a sentinel when needed to preserve current state or pre-existing query semantics
- deleted-only historical state older than retention can be dropped
- Kubernetes Event history older than retention is dropped outright

Checkpoint directories whose maximum timestamp is fully expired are deleted during startup/checkpoint retention.

Checkpoint count retention still applies, but only after time-expired checkpoints have been removed.

### Query / Analysis Semantics

When retention is enabled:

- analysis/history helpers use the configured retention window instead of the hardcoded 24 hour window
- raw timeline/export queries only see retained hot/cold data after the next retention cycle
- projection-history fallback also follows the same retained window after projection pruning

When retention is disabled:

- no time-based raw deletion occurs
- the hardcoded 24 hour projection history cap is removed

## Trade-Offs

### Advantages

- one operator-facing knob controls both disk growth and retained embedded history
- startup can reclaim old data even if the process was offline when it should have expired
- compaction remains useful, but retention no longer depends on compaction being triggered

### Costs

- checkpoint/startup retention cycles do extra IO when mixed-age segments must be rewritten
- live in-memory data may remain slightly beyond retention until the next checkpoint cycle
- keeping a pre-cutoff sentinel for active resource state means checkpoints may retain a minimal amount of older state metadata even when raw history is gone

## Acceptance Criteria

- `--embedded-retention-days=0` disables time-based retention
- `--embedded-retention-days=1` removes raw data older than one day on startup/checkpoint cycles
- mixed-age segments are rewritten to retained subsets
- analysis/query history windows align with the configured retention window
- focused Go tests cover config, checkpoint/history retention, mixed/raw segment pruning, and startup reconciliation
- Helm tests verify the deployment emits the new flag
