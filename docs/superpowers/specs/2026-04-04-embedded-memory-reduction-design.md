# Embedded Memory Reduction Design

## Context

Spectre's embedded mode still uses more memory than is justified by the on-disk dataset size. Two concrete reductions already landed:

1. Stop retaining duplicate raw payload copies in `resourceVersion.changeEvent.Data`.
2. Stop retaining parsed `map[string]any` objects in each `resourceVersion`, and lazy-parse only when needed.

Those changes materially improved live memory use, but the embedded projection still keeps historical event data in multiple in-memory forms:

- `projection.events`
- `eventsByResourceUID`
- `k8sRawEventsByInvolvedUID`
- `k8sEventsByInvolvedUID`
- `resourcesByUID`
- `resourceMetaByUID`
- `orderedResources`

The core problem is architectural: RAM is still acting as the primary store for historical timeline queries, even though the engine already has hot and cold persistent stores plus a query planner.

## Goal

Reduce embedded steady-state memory by making the in-memory projection analysis-oriented instead of history-oriented.

## Acceptance Criteria

Use the homelab `monitoring/spectre` dataset as the primary acceptance baseline.

Observed baselines before this design work:

- original embedded bootstrap image with cache enabled: OOM near the 4 GiB limit
- first projection-memory fix image: stable around 2.6-2.9 GiB steady-state after restore
- second lazy-parse fix image: materially lower during restore, but Phase 1 of this design is intended to remove the remaining historical-RAM dependency

Phase 1 is successful only if all of the following are true on the homelab dataset:

- no OOM or restart during restore or steady-state
- peak RSS during restore remains below 3.0 GiB
- steady-state RSS 10 minutes after readiness is at or below 1.8 GiB
- timeline/export/metadata behavior matches the parity contract below

If steady-state memory is still above 1.8 GiB after Phase 1, Phase 2 becomes mandatory rather than optional.

## Non-Goals

- Replacing the current segment/journal format.
- Changing external API semantics for timeline or namespace-graph queries.
- Introducing a new database or external dependency.
- Broad refactors outside embedded query and projection paths.

## Recommendation

Adopt a **slim projection + disk-backed history** design.

The projection should hold only the compact state needed for:

- current and recent analysis reads
- relationship traversal
- metadata lookup
- readiness / watcher bootstrap decisions

Historical timeline and export queries should read from:

- hot store for recent events
- persisted segments for older events
- the existing query planner to combine both

This is the best ROI path because it removes the largest remaining resident structures instead of trying to micro-optimize them in place.

## Target Architecture

### In-Memory Projection

Keep:

- `resourcesByUID`
  - but only with compact per-version data:
    - timestamp
    - event type
    - identity
    - raw `data` bytes only if still needed for lazy parsing or diffs
    - compact change-event summary
- `resourceMetaByUID`
- `orderedResources`
- `k8sEventsByInvolvedUID`
  - compact derived summaries only

Remove from steady-state projection memory:

- `projection.events`
- `eventsByResourceUID`
- `k8sRawEventsByInvolvedUID`

### Query Execution

#### Timeline / Export

Timeline and export paths should no longer fall back to `projection.events` or `eventsByResourceUID`.

Instead:

- use hot store for recent data
- use segment readers for persisted history
- use the planner to merge, dedupe, and order results

Projection should only provide:

- resource metadata
- resource ordering
- analysis-side current state

### Parity Contract

Phase 1 must preserve current externally observable timeline/export semantics:

- event ordering uses `compareEventOrder`
  - primary key: `Timestamp` ascending
  - tie-breaker: `ID` ascending
- time bounds remain inclusive
  - include events where `start <= event.Timestamp <= end`
- per-resource timeline queries keep the current pre-existing event behavior
  - include the latest event before `start` marked `PreExisting=true` when that event is not a delete
- merged results dedupe by event ID
  - keep the first event after deterministic merged ordering
- pagination semantics for resource queries remain unchanged
  - resource pagination is by ordered resource identity, not by raw event count
- export results remain deterministic across repeated runs on the same persisted dataset
- concurrent ingestion may add newer data, but identical persisted inputs must produce identical ordered outputs

#### Analysis

Analysis APIs keep using the compact projection:

- ownership chain
- related resources
- manager inference
- namespace graph
- metadata cache support

If a query needs a raw payload for a specific returned event, hydrate it on demand from `version.data` or persistent storage.

## Data Model Changes

### 1. Remove Full Event Arrays From Projection

Delete:

- `Projection.events []models.Event`
- `Projection.eventsByResourceUID map[string][]models.Event`
- `Projection.k8sRawEventsByInvolvedUID map[string][]models.Event`

Replace them with:

- no historical event retention in projection
- compact K8s event summaries only

### 2. Keep Compact Resource Version State

`resourceVersion` should remain compact and analysis-focused:

- timestamp
- eventType
- identity
- `data []byte` only if required for:
  - lazy relationship parsing
  - change-event payload hydration
  - spec diff computation

For Phase 1, `data []byte` remains required for exactly these paths:

- `specDiffWithinLookback()`
- `GetChangeEvents()` payload hydration
- lazy parsing used by ownership/reference/selector/spec-replica analysis

For Phase 1, `data []byte` is explicitly not the source of truth for:

- timeline history scans
- export time-range scans
- raw Event-kind history scans

Those reads must come from hot store + persisted segments through the planner.

Longer-term optional improvement:

- replace `data []byte` with extracted compact fields plus optional persisted payload lookup

### 3. De-Duplicate Stringy Identity Data

After the disk-backed history change, add a smaller second-phase optimization:

- intern common strings:
  - namespace
  - kind
  - group
  - resource names
  - label keys
  - common label values

This is explicitly secondary because it does not address the main structural duplication.

## Query Planner Changes

### Planner Capability Matrix

Phase 1 requires the planner to support the following operations explicitly:

| Query / API Path | Required Source | Required Planner Capability |
|---|---|---|
| per-resource timeline query | hot + segments | fetch events by resource UID, merge hot/cold, order by `compareEventOrder`, inject pre-existing event, dedupe by ID |
| export time range | hot + segments | scan by time range + filters, merge hot/cold, deterministic global ordering, dedupe by ID |
| metadata query | projection metadata + planner-backed event presence check | determine whether a resource has events in range without requiring projection history arrays |
| Event-kind timeline query | hot + segments | fetch Event-kind records by involved UID and/or time range, convert to API shape, deterministic ordering |
| K8s events attached to resource query results | hot + segments or compact summary cache | return bounded Event-kind records for returned resources without relying on `k8sRawEventsByInvolvedUID` |

Any implementation plan must account for these capabilities separately; “make planner mandatory” is not enough.

### Resource Event Queries

Today, `resourceEvents()` falls back to projection-resident history when planner support is absent.

Change it so that:

- planner-backed reads are mandatory in normal engine operation
- projection fallback is removed or restricted to tests / synthetic projections

### Export Queries

`ExportTimeRange()` should never iterate over `projection.events`.

It should:

- ask the planner for the relevant hot/cold events
- return merged ordered results

### K8s Event Queries

`executeEventQuery()` and `collectK8sEventsForResources()` should stop depending on `k8sRawEventsByInvolvedUID`.

Options:

1. Read Event-kind resources from hot/cold stores like other timeline data.
2. Keep compact pre-parsed K8s event summaries only for the analysis use-cases.

Recommendation:

- use option 1 for timeline/export
- keep only compact summaries for analysis

## Rollout Plan

### Phase 1

Make timeline/export/history fully planner-backed and remove projection history retention.

Success criteria:

- acceptance criteria above are met
- no segment or manifest schema migration is required
- existing persisted hot/segment data remains readable without rewrite

Operational safety requirements:

- add a temporary kill switch for rollout and bisecting
  - recommended form: a server flag that preserves the old projection-history query path for one release window
- default the kill switch to the new planner-backed mode in test and canary validation only after parity tests pass
- remove the kill switch only after live validation shows stable behavior

### Phase 2

If needed, reduce remaining per-version payload cost:

- compact extracted fields instead of raw `data []byte`
- string interning / dedup

## Verification

Required before rollout:

- embeddedstore targeted tests
- watcher + command focused tests
- regression tests for:
  - timeline behavior parity
  - export behavior parity
  - metadata query parity
  - namespace graph parity
  - deterministic ordering for timestamp/ID ties
  - pre-existing event injection semantics
  - Event-kind query parity

Required test scenarios:

- resource history fully in hot store
- resource history fully in persisted segments
- mixed hot + persisted history for the same resource
- multiple resources sharing overlapping timestamps
- Event-kind records attached to returned resources
- deleted-resource window queries that rely on latest pre-start state

Required measurable assertions:

- output equality against existing behavior for golden fixtures
- stable ordering over repeated identical runs
- no projection fallback usage in normal engine-open path
- memory baselines sampled with the method below

Memory sampling methodology:

- sample `kubectl top pod` every 10 seconds from process start until 10 minutes after readiness
- record:
  - first sample after process start
  - peak restore RSS
  - RSS at readiness
  - median of the last 3 samples at the 10-minute mark
- capture restart count and readiness state alongside each run

Required during rollout:

- successful rollout on homelab PVC
- zero restarts
- readiness green
- memory sampling across restore and steady state

## Risks

### Risk: Query Parity Regressions

Removing projection-resident history changes the fallback path.

Mitigation:

- parity tests against current behavior
- keep planner-backed merge ordering deterministic

### Risk: Slower Cold Queries

Disk-backed historical reads may be slower than fully in-memory scans.

Mitigation:

- rely on hot store for recent reads
- keep segment dimension cache
- measure before adding more indexes

### Risk: Hidden Dependencies On Raw Arrays

Some code paths may implicitly rely on `projection.events` or `eventsByResourceUID`.

Mitigation:

- compile-time removal instead of soft deprecation
- update tests to assert absence of those dependencies

## Alternatives Considered

### Blob Dedup First

Rejected as the primary approach.

It adds complexity but leaves the architecture unchanged: historical queries would still be RAM-first.

### String Interning First

Useful but too incremental for the size of the current footprint.

### Leave History In Memory And Add More Compression

Rejected.

Compression trades CPU for RAM but keeps the wrong ownership model.

## Decision

Proceed with:

1. slim projection
2. disk-backed timeline/export/history reads
3. compact K8s event summaries only in memory
4. optional second-phase string/payload dedup if still needed
