# Startup Import Defer Causality Design

**Date:** 2026-04-01

**Status:** Approved in design conversation

## Goal

Reduce startup import time for `spectre server --import-path` when the user only needs the timeline view to become usable quickly.

The optimization must be explicitly enabled and must not change normal live ingestion behavior.

## Problem Statement

The current startup import path uses the normal graph sync pipeline for every chunk. That preserves semantics, but it also runs causality inference during bulk import.

Measured startup-import logs on the current branch show that the dominant cost is after Phase 2, where causality inference runs. Timeline queries do not require causality edges, but they do require:

- `ResourceIdentity` nodes
- `CHANGED` edges to `ChangeEvent`
- `EMITTED_EVENT` edges to `K8sEvent`

Those timeline-critical edges are created during Phase 1, while semantic resource relationships and causality are not required for timeline rendering.

## Scope

### In Scope

- Add an explicit startup-import flag to defer causality during startup import
- Keep the timeline view functional
- Keep normal watcher and live ingestion behavior unchanged
- Add tests that prove the override is scoped to startup import only

### Out Of Scope

- Changing timeline queries
- Making deferred causality the default
- Deferring semantic relationship extraction in this change
- Adding background catch-up processing for deferred work in this change

## Design

### CLI

Add a new explicit flag on `spectre server`:

- `--startup-import-disable-causality`

Behavior:

- no effect unless `--import-path` is set
- when enabled, only startup-import batch processing skips causality inference
- watcher-driven and other normal pipeline usage continue to use the configured pipeline defaults

### Pipeline Control

Introduce a per-call batch processing override in `internal/graph/sync` using context.

The override is intentionally narrow:

- default behavior remains unchanged
- startup import can derive a context that disables causality for `ProcessBatch`
- later follow-up work can reuse the same seam for additional startup-import-only deferrals

### Why Context Override Instead Of A Second Pipeline

This keeps the change small and avoids:

- creating and managing a second pipeline lifecycle
- duplicating graph schema/bootstrap work
- broadening the CLI-to-graphservice wiring

The override remains explicit because it is only activated when the new CLI flag is passed.

## Testing

- Command test: server command exposes `--startup-import-disable-causality`
- Pipeline unit test: default config with causality enabled still skips causality when the override context disables it
- Existing startup import and integration tests continue to pass

## Risks

- Semantic relationship extraction still runs during startup import, so this is not the maximum possible speedup
- If future startup-import tuning also disables relationship extraction, timeline dependencies must still preserve `CHANGED` and `EMITTED_EVENT`

## Expected Outcome

This should produce an immediate startup-import throughput improvement because the measured dominant cost is causality inference, while keeping timeline data intact.
