# Embedded Storage Driver Design

**Date:** 2026-04-02

**Status:** Approved in design conversation

## Goal

Enable the embedded storage driver to support durable writes and serve as a live backend for Spectre when `--embedded` is set, while preserving the existing FalkorDB path when `--embedded` is not set.

The target outcome is:

- `--embedded=false`: keep the existing FalkorDB-backed runtime unchanged
- `--embedded=true` with watcher enabled: run a persistent embedded backend under `--data-dir`
- `--embedded=true` with import data and watcher disabled: serve imported data without starting the watcher

This is the migration path toward eventually replacing FalkorDB entirely, but FalkorDB removal is explicitly out of scope for this design.

## Constraints

- Do not remove the FalkorDB backend yet
- Backend choice must be controlled by the presence of `--embedded`
- Embedded persistent state must live under `--data-dir`
- There must be an embedded mode that only displays imported data and does not start the watcher
- Existing REST and MCP consumers should keep using the same API surface
- The design should minimize handler-level branching and keep backend decisions centralized in server startup

## Problem Statement

Today the repository has two different meanings attached to "embedded":

- `internal/embedded` and `internal/analysis/store/embedded` provide read-only, import-backed query and analysis behavior
- `cmd/spectre/commands/server.go` treats `--embedded` as an offline startup-import mode that disables watcher, graph service, and MCP startup

The live runtime still depends on the watcher writing into `graphservice`, which depends on `graph.Client`, which is implemented by FalkorDB.

That means the current embedded path is not a true backend alternative. It is a separate read-only execution mode.

To make embedded storage a real replacement candidate for FalkorDB, the system needs a backend split at runtime startup, not just a special-case import path.

## Architectural Decision

Introduce a first-class persistent embedded backend selected by `--embedded`, with its own durable event journal and derived read models, and wire the rest of the system to depend on backend-neutral contracts above the Falkor layer.

The embedded backend will be a peer of the Falkor-backed graph service, not an implementation of FalkorDB's query/client semantics.

This means:

- keep FalkorDB and `graphservice` for the non-embedded path
- add an embedded runtime that owns persistence, ingestion, query execution, and analysis reads
- change watcher and import paths to target a backend-neutral ingest contract instead of a Falkor-specific sync pipeline
- keep REST and MCP mostly unchanged by handing them backend-provided executors and stores

## Rejected Approaches

### Rejected: implement embedded as a new `graph.Client`

This would preserve more of `graphservice`, but it is the wrong seam.

The current Falkor path assumes:

- Cypher-like query execution through `ExecuteQuery`
- Falkor-backed observatory graph queries
- graph-service lifecycle tied to Falkor connection and schema semantics

Implementing a local `graph.Client` would effectively require building a Falkor/Cypher compatibility layer inside Spectre. That is high effort and high risk for little architectural benefit.

### Rejected: mutate the current in-memory embedded snapshot directly

The existing embedded code is shaped around imported, immutable data:

- `internal/embedded.QueryExecutor`
- `internal/analysis/store/embedded`

It is suitable as a read-optimized snapshot, but not as the long-term write path. Teaching it to mutate in place would conflate durability, replay, concurrency, and query responsibilities in code that was designed for static datasets.

### Chosen: persistent projection-based embedded backend

Persist raw events durably under `--data-dir`, build and update read models from those events, and expose the same high-level query/analysis contracts that the API layer already understands.

This keeps the durable source of truth simple and makes projections rebuildable.

## Runtime Modes

The runtime should be described by two axes:

- backend: `falkor` or `embedded`
- ingestion mode: `live`, `import-only`, or `audit-only`

### Falkor runtime

When `--embedded=false`, keep the current Falkor path:

- initialize `graphservice`
- start watcher when enabled
- write events through the graph sync pipeline
- use Falkor-backed query execution and analysis

### Embedded live runtime

When `--embedded=true` and watcher is enabled:

- initialize the persistent embedded backend under `--data-dir`
- start the watcher
- write watcher events into the embedded backend
- serve timeline, metadata, analysis, import/export, and MCP from the embedded backend

This is the durable live mode intended to replace FalkorDB behavior over time.

### Embedded import-only runtime

When `--embedded=true`, imported data is provided, and watcher startup is disabled:

- initialize the embedded backend
- import the dataset into the embedded backend
- do not start the watcher
- serve the imported dataset through the embedded query and analysis stack

This preserves the current "display imported data only" workflow, but places it on the same backend family as live embedded mode.

### Audit-only runtime

Audit-only remains a non-embedded special case for the Falkor-disabled path. It should not be mixed into the embedded backend selection logic beyond sharing high-level runtime-mode resolution.

## Component Boundaries

The embedded backend should be implemented as a focused package, for example `internal/embeddedstore`, with four responsibilities.

### Event store

Persistent append-only event storage under `--data-dir`.

Responsibilities:

- durably append one event or a batch of events
- expose replay/read APIs for startup recovery
- track checkpoints or high-water marks for snapshotting

This is the source of truth for embedded mode.

### Projection store

Durable read models derived from the event store.

Responsibilities:

- maintain indexes needed for timeline and metadata queries
- maintain the state needed by `analysisstore.AnalysisStore`
- maintain exportable event/query views
- optionally maintain observatory read models where supported

Projection state is rebuildable from the event journal.

### Ingestor

Single backend write entrypoint for watcher and import flows.

Responsibilities:

- accept one event or a batch
- append durably to the journal first
- update projections second
- report degraded state if append succeeds but projection update fails

### Runtime facade

Backend-facing service surface for the rest of Spectre.

Responsibilities:

- provide an `api.QueryExecutor`
- provide an `analysisstore.AnalysisStore`
- provide a write interface for watcher/import
- expose startup/shutdown/recovery lifecycle hooks

## Why the Runtime Facade Lives Above `graph.Client`

The stable contracts in the current codebase are already above the Falkor layer:

- HTTP handlers consume `api.QueryExecutor`
- analysis consumes `analysisstore.AnalysisStore`
- watcher only needs an event-processing target
- import only needs batch ingestion

Those are the right seams for backend substitution.

The embedded backend should therefore integrate at those seams instead of trying to emulate FalkorDB internals.

## Data Flow

### Embedded live mode

1. Kubernetes watcher receives add, update, and delete events.
2. `watcher.EventCaptureHandler` forwards the normalized `models.Event` to an embedded ingestor instead of a Falkor-specific pipeline.
3. The embedded ingestor appends the event durably to the journal under `--data-dir`.
4. The ingestor updates persistent projections used by timeline, metadata, analysis, export, and any supported observatory views.
5. REST and MCP serve reads from the embedded executor/store pair.

### Embedded import-only mode

1. Startup import reads events from `--import-path`.
2. The same embedded ingestor processes the imported batch.
3. The watcher is not started.
4. API requests read the imported dataset through the same embedded executor/store interfaces.

The important property is that live and import-only embedded modes share the same backend family and query surfaces.

## Server Wiring Changes

### `cmd/spectre/commands/server_mode.go`

Replace the current binary `embedded` special case with a richer runtime-mode resolver that returns:

- selected backend
- selected ingestion mode
- whether watcher should start
- whether MCP should start
- whether import-only serving is active

The resolver should stop treating embedded mode as "disable runtime components unconditionally."

### `cmd/spectre/commands/server.go`

Refactor startup into backend-specific branches:

- Falkor branch: current behavior
- embedded branch: initialize embedded backend, select live or import-only behavior, and wire API/MCP/watcher against embedded services

The rest of startup should be assembled from backend-provided contracts rather than direct knowledge of FalkorDB.

### `internal/watcher/event_handler.go`

Replace the Falkor-specific `GraphPipeline` dependency with a backend-neutral ingest interface.

The event handler should not care whether the downstream write target is:

- Falkor graph pipeline
- embedded ingestor
- audit log only

### `internal/apiserver/server.go`

Generalize server construction so import/export registration depends on backend capabilities rather than on the presence of a Falkor graph pipeline alone.

The server should continue to receive:

- storage/query executor
- analysis store
- optional import/export capability
- readiness checker
- optional MCP server

### `internal/api/handlers/register.go`

Keep handler behavior mostly unchanged by continuing to register endpoints based on available query/analysis capabilities.

The main change is that embedded live mode should now satisfy the same contracts Falkor mode uses for:

- timeline
- metadata
- causal graph
- anomalies
- causal paths
- namespace graph
- import/export where supported

## Persistence Model

Embedded persistent state lives under `--data-dir`.

The minimal layout is:

- append-only event journal
- persisted projection snapshot(s)
- metadata describing the snapshot watermark and replay position

### Source of truth

The event journal is the source of truth.

### Recovery model

On startup:

- if a valid projection snapshot exists, load it
- replay any journal tail newer than the snapshot watermark
- if snapshot load fails, rebuild projections from the full journal

This keeps restart time reasonable without making projections authoritative.

## Write Semantics

The embedded backend should have clear durability and failure semantics.

### Success path

1. append event to journal
2. fsync or otherwise make the append durable
3. update projections
4. report success

### Failure after durable append

If the durable append succeeds but projection update fails:

- the write should not be silently discarded
- the backend should mark itself degraded
- logs and readiness should surface the mismatch
- restart or explicit rebuild should recover by replaying the journal

### Failure before durable append

If durable append fails, return an error immediately and do not claim the event was accepted.

This keeps correctness biased toward "accepted durably or rejected," which is the right behavior for a backend intended to replace FalkorDB.

## Query and Analysis Contracts

The embedded backend should satisfy the same service-layer contracts that the API already consumes.

### Timeline and metadata

Provide an `api.QueryExecutor` implementation backed by embedded projections and event replay-aware indexes.

This becomes the read source for:

- `/v1/timeline`
- `/v1/metadata`
- search
- export

### Analysis

Provide an `analysisstore.AnalysisStore` implementation backed by embedded projections.

This becomes the read source for:

- causal graph
- anomaly detection
- causal paths
- namespace graph

### Import

Expose a batch-ingest entrypoint so `/v1/storage/import` and startup import can both target the same embedded write path.

## Observatory Graph

Observatory graph is the main known compatibility gap.

Today it depends directly on `graph.Client` and query shapes tailored to Falkor/Cypher semantics. That does not fit the embedded backend seam selected in this design.

This design therefore treats observatory support as a follow-on choice:

- either add a dedicated embedded observatory read model, or
- explicitly report observatory graph as unsupported in embedded mode until parity work is implemented

The important constraint is to avoid pretending that `graph.Client` compatibility comes for free.

## Error Handling

### Startup

- If `--embedded` import-only mode receives an unreadable or unusable import path, fail startup.
- If the event journal is corrupt, fail startup loudly instead of serving partial data.
- If a projection snapshot is corrupt but the journal is valid, log the snapshot failure and rebuild from the journal.

### Runtime

- If embedded backend recovery has not completed, readiness should remain false.
- If the backend enters degraded mode because projection updates failed after durable append, readiness should reflect that degraded state.
- Reads should serve from the last consistent projection state; writes should fail fast when durable append cannot be completed.

## Testing Strategy

Testing should be layered so the backend can become a credible Falkor replacement.

### Unit tests

- event journal append and replay
- projection update behavior
- snapshot load and fallback rebuild
- restart recovery under `--data-dir`
- concurrent reads during write activity

### Contract tests

Run backend-neutral tests against Falkor and embedded implementations where possible for:

- timeline query behavior
- metadata responses
- `analysisstore` query behavior

### Runtime-mode tests

Cover:

- `--embedded=true` live mode starts API and watcher, and uses embedded services
- `--embedded=true` import-only mode serves imported data and does not start watcher
- `--embedded=false` continues using Falkor-backed startup

### Integration tests

- import fixture data into embedded import-only mode and validate graph-analysis endpoints
- run embedded live mode, ingest events, restart Spectre, and verify state persists across restart from `--data-dir`

## Migration Strategy

Implement the change in this order:

1. Introduce backend-neutral runtime mode resolution for Falkor vs embedded live/import-only startup.
2. Define the embedded ingest/runtime contracts used by watcher, import, API, and MCP.
3. Implement durable event journal persistence under `--data-dir`.
4. Implement projection loading, replay, and rebuild behavior.
5. Adapt timeline/metadata/search/export to use embedded query execution in live mode.
6. Adapt analysis consumers to use embedded live projections through `analysisstore.AnalysisStore`.
7. Rewire watcher and import paths to target the backend-neutral ingest contract.
8. Add runtime and restart persistence tests.
9. Handle observatory graph explicitly: either embedded read-model support or a clear unsupported response.

This sequence keeps the Falkor path intact while embedded grows into a real backend rather than another special-case import mode.

## Expected Outcome

After this design is implemented:

- embedded mode is no longer synonymous with startup-import-only
- Spectre can run against a persistent local backend under `--data-dir`
- the watcher can write to embedded storage in live mode
- an import-only embedded mode still exists for read-only dataset inspection
- FalkorDB remains available when `--embedded` is not set

That is the right intermediate state before fully removing FalkorDB later.
