# Startup Import Timeline Only Design

**Date:** 2026-04-01

**Status:** Approved in design conversation

## Goal

Add an explicit startup-import mode that makes the timeline view usable as quickly as possible by importing only the graph data required for timeline rendering.

## Definition

`timeline only` means:

- keep `ResourceIdentity` nodes
- keep `ChangeEvent` and `K8sEvent` nodes
- keep `CHANGED` and `EMITTED_EVENT` structural edges
- skip semantic relationship extraction such as `OWNS`, `SELECTS`, `MOUNTS`, `USES_SERVICE_ACCOUNT`
- skip causality inference

## Scope

### In Scope

- explicit startup-import-only flag
- pipeline override that skips semantic relationship extraction and causality
- tests proving structural timeline edges are still written

### Out Of Scope

- changing live watcher ingestion
- changing timeline queries
- background backfill of deferred relationships or causality

## Design

### CLI

Add:

- `--startup-import-timeline-only`

Behavior:

- only applies when `--import-path` is set
- leaves normal ingestion untouched
- implies deferring both semantic relationships and causality during startup import

### Pipeline

Extend the existing startup-import batch override context with:

- `TimelineOnly bool`

When enabled in `ProcessBatch`:

- Phase 1 continues unchanged
- structural edges collected from `BuildResourceNodes` are still applied
- `BuildRelationshipEdges` is not called
- causality inference is skipped

This keeps the optimization local to the existing pipeline and avoids introducing a second ingest implementation.

## Expected Impact

This should improve startup-import throughput beyond the causality-only mode. The causality win is already strongly evidenced; the incremental relationship-extraction win is expected but workload-dependent.

## Risks

- Graph and causal-path views will be incomplete until deferred work is recomputed later
- This mode must remain explicit so operators do not accidentally trade graph completeness for speed
