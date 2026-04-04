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

- embedded pod steady-state memory drops materially below current post-fix baseline
- timeline/export results stay behaviorally equivalent

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
