# Embedded Timeline-Only Mode Design

**Date:** 2026-04-01

**Status:** Approved in design conversation, pending written spec review

## Goal

Add an explicit `--embedded` mode to `spectre server` for single-binary, proof-of-concept deployments that:

- load a static dataset from `--import-path` at startup
- keep all imported data in memory
- support the existing timeline UI contract
- avoid any dependency on FalkorDB or other external services

This mode is intentionally narrow. It exists to make the timeline view usable from imported data in a self-contained binary, not to preserve the full Spectre feature set.

## Non-Goals

The following are out of scope for `--embedded`:

- live watcher ingestion
- audit-log ingestion
- runtime HTTP import
- namespace graph, causal graph, root-cause, anomaly, or observatory features
- MCP tools that rely on graph-backed reasoning
- durable on-disk embedded persistence

The implementation should prefer omission over partial emulation for unsupported capabilities.

## Existing Seams In The Repo

The repo already has the right boundary for this work:

- startup import already flows through `internal/importexport` in [server.go](/home/moritz/dev/spectre-via-ssh/cmd/spectre/commands/server.go)
- `/v1/timeline` already flows through [TimelineService](/home/moritz/dev/spectre-via-ssh/internal/api/timeline_service.go)
- `TimelineService` already depends on the narrow [QueryExecutor](/home/moritz/dev/spectre-via-ssh/internal/api/interfaces.go) interface
- the UI already consumes the existing timeline response shape and import workflow assumptions

That means `--embedded` should not invent a parallel timeline API. It should provide a different executor behind the current timeline service.

## Recommended Approach

Use a new in-memory embedded executor that implements `api.QueryExecutor` and wire it into `TimelineService` when `--embedded` is enabled.

This is preferred over special-casing handlers because:

- the timeline API stays unchanged
- the UI does not need mode-specific branching for timeline reads
- startup and capability differences stay localized to server wiring
- the embedded store remains easy to reason about because it serves one responsibility: read-only timeline queries over imported events

## Runtime Shape

`--embedded` introduces a third server runtime shape:

1. normal graph-backed mode
2. audit-only mode
3. embedded timeline-only mode

In embedded mode:

- `--import-path` is required
- imported data is loaded before the server begins serving requests
- graph service initialization is skipped
- graph sync pipeline initialization is skipped
- watcher startup is skipped
- reconciler startup is skipped
- HTTP import registration is skipped
- graph-backed API handlers are not registered
- graph-backed MCP surfaces are not registered

The result is a deliberately smaller server surface that matches the user expectation of a static, single-binary POC.

## Data Model

The embedded store should keep raw imported `models.Event` as its source of truth and build a small number of read-optimized indexes at startup.

### Source of truth

- raw imported `[]models.Event`

### Required indexes

- `eventsByResourceUID map[string][]models.Event`
- `resourceMetaByUID map[string]models.ResourceMetadata`
- `k8sEventsByInvolvedUID map[string][]models.Event`
- deterministic ordered resource key list for pagination
- global earliest and latest event timestamps

### Load-time normalization

At startup, the embedded store should:

- reject or skip malformed events the same way the importer already does
- sort each resource event slice by timestamp ascending
- retain the latest resource metadata per UID for filtering and display
- split Kubernetes `kind=Event` objects into the `k8sEventsByInvolvedUID` index via `InvolvedObjectUID`
- exclude raw Kubernetes `Event` objects from the primary resource timeline index
- log a warning for imported rows missing a resource UID, since they cannot participate in resource timelines correctly

The store should remain fully in memory. No persistence layer is needed.

## Query Model

The embedded executor should implement the current `api.QueryExecutor` contract and return `models.QueryResult` values shaped so [BuildTimelineResponse](/home/moritz/dev/spectre-via-ssh/internal/api/timeline_service.go) can continue to assemble the response.

### Resource query behavior

For non-`Event` resource queries:

- filter by time window
- filter by namespace, kind, group, and version
- group by resource UID
- paginate by resource, not by raw event row
- return each selected resource with its complete set of relevant timeline events

### Kubernetes Event query behavior

For the timeline service's secondary `kind=Event` query:

- filter by the same time window
- respect namespace filters
- return Kubernetes `Event` rows so they can be attached to their target resources by `InvolvedObjectUID`

### Pre-window anchor behavior

To preserve correct first-segment rendering, if a resource has state before the requested `start` timestamp, the executor should include the latest event before the window as a synthetic anchor event in the result set and mark it `PreExisting=true`.

That ensures:

- the first timeline segment starts with a known resource state
- the UI keeps working for resources that existed before the visible window
- `TimelineService` can continue inferring segments from ordered events without embedded-specific rendering logic

### Pagination behavior

Pagination must happen at the resource level because the UI expects complete resource timelines, not partial event fragments.

The resource ordering should be deterministic and stable for a fixed dataset and filter set. A simple sortable key is sufficient, for example:

- namespace
- kind
- name
- UID

The cursor format does not need to be user-friendly; it only needs to be stable and unambiguous.

## API Surface In Embedded Mode

### Supported

- timeline API backed by the embedded executor
- timeline metadata or other timeline-adjacent reads that can be satisfied from imported events
- existing UI timeline read path

### Not supported

- `/v1/storage/import`
- namespace graph handlers
- causal graph handlers
- observatory graph handlers
- graph-backed root-cause and anomaly features
- graph-backed MCP tools

The cleaner implementation is to omit unsupported handler registration in embedded mode instead of registering them and failing later.

## CLI And Startup Behavior

`--embedded` should be a hard mode switch in [server.go](/home/moritz/dev/spectre-via-ssh/cmd/spectre/commands/server.go).

### Required behavior

- fail startup if `--embedded` is set without `--import-path`
- fail startup if import parsing fails
- fail startup if the imported dataset yields zero usable events
- build the in-memory indexes before the HTTP server begins accepting requests
- create `TimelineService` with the embedded executor and no graph executor

### Incompatible behavior

Embedded mode should reject or ignore configuration that implies a live system. The implementation should choose the smallest, clearest rule set, but the intended runtime behavior is:

- no watcher activity
- no audit-log ingestion
- no graph database requirement
- no background synchronization loops

This mode is static by design.

## Import Path Behavior

Embedded mode should reuse the existing import parser from `internal/importexport` rather than introducing a second import format.

The startup flow is:

1. read events from `--import-path`
2. validate and enrich them through the existing import layer
3. build embedded indexes in memory
4. start serving timeline requests

There is no runtime mutation path after startup.

## Error Handling

Error handling should be explicit and boring.

### Startup errors

- missing `--import-path`
- unreadable import path
- parse failure
- zero usable imported events

These should fail startup immediately with clear messages.

### Warnings

- imported events with missing resource UIDs
- imported Kubernetes `Event` rows without usable `InvolvedObjectUID`

These should not necessarily abort startup, but they should be logged because they reduce timeline fidelity.

### Request-time errors

Request behavior for the timeline endpoint should remain consistent with the current handler:

- invalid query parameters return `400`
- executor failures return `500`

The goal is to preserve the current contract, not create a second error model.

## Testing Strategy

Verification should focus on the embedded contract instead of broad product parity.

### Unit tests

- embedded executor filtering by namespace, kind, group, and version
- resource-level pagination
- pre-window anchor event behavior
- Kubernetes `Event` attachment behavior via `InvolvedObjectUID`
- deterministic ordering and cursor behavior

### Command and wiring tests

- `--embedded` requires `--import-path`
- embedded mode does not require graph configuration
- embedded mode skips unsupported handler registration
- embedded mode skips watcher and graph startup paths

### API tests

- import fixture at startup, then query `/v1/timeline`
- verify the response shape still matches what the UI expects:
  - resources
  - status segments
  - attached Kubernetes events
  - pagination metadata behavior

The primary success criterion is that the existing timeline UI can load useful static data without graph infrastructure.

## Risks And Mitigations

### Risk: hidden graph assumptions leak into timeline reads

Some current code paths may assume graph-backed services exist even when only the timeline is being used.

Mitigation:

- keep embedded mode registration narrow
- construct only the services needed by the timeline UI
- add server wiring tests that prove graph-dependent paths are absent

### Risk: incomplete resource histories produce misleading first segments

If the executor only returns in-window events, long-lived resources will render incorrect initial state.

Mitigation:

- explicitly include the latest pre-window event as a `PreExisting` anchor
- test this behavior directly at executor level

### Risk: pagination by event instead of resource breaks the UI

Returning partial resource histories across pages would make segment rendering and detail inspection inconsistent.

Mitigation:

- paginate by resource identity only
- return complete event history for each selected resource within the request semantics

## Open Design Constraint

This design intentionally optimizes for cleanliness over feature breadth. If future POC needs expand beyond timeline-only playback, the correct next step is likely a broader backend abstraction. That refactor is intentionally deferred until there is a real need for more than:

- startup import
- in-memory indexes
- read-only timeline queries
