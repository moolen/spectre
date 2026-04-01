# Embedded Analysis Parity Design

**Date:** 2026-04-01

**Status:** Approved in design conversation

## Goal

Enable embedded mode to support the existing graph-oriented UI and analysis APIs from imported event data only, while staying startup-imported and read-only.

The target is eventual parity for:

- namespace graph / `/graph`
- causal graph
- anomaly detection
- causal paths
- the existing timeline and metadata APIs already supported in embedded mode

## Constraints

- Embedded mode remains `--embedded` plus `--import-path` startup import only
- Embedded mode remains read-only for now
- No live watchers, streaming ingest, or HTTP import in this design
- The implementation must work from imported JSONL event data
- The implementation must preserve the current Falkor-backed behavior while parity work is in flight

## Problem Statement

The current embedded implementation is timeline-oriented and does not expose the graph-analysis APIs used by the UI.

The missing functionality is not primarily an HTTP handler problem. The deeper issue is that the analysis stack is directly coupled to `graph.Client` and graph-database query semantics:

- root cause analysis in `internal/analysis`
- anomaly detection in `internal/analysis/anomaly`
- causal path discovery in `internal/analysis/causal_paths`
- namespace graph analysis in `internal/analysis/namespace_graph`

Today these packages assume that analysis is performed by issuing graph queries against Falkor. That makes it hard to add embedded support cleanly, because the embedded backend would have to either:

- emulate Falkor query behavior in memory, or
- provide a cleaner backend-neutral analysis substrate

The second approach is the preferred direction.

## Architectural Decision

Introduce a backend-neutral analysis store over a canonical materialized graph snapshot.

The analysis packages should stop depending on `graph.Client` directly and instead depend on a domain-oriented read interface that expresses the real analysis queries they need.

This creates two backend implementations:

- a Falkor-backed adapter that preserves existing behavior
- an embedded in-memory adapter built from imported events at startup

The canonical analysis substrate is the materialized graph/read model, not Cypher.

## Why This Approach

### Rejected: in-memory graph query emulation

Teaching embedded mode to mimic the exact existing `graph.Client` query shapes would likely get a partial demo working quickly, but it would deepen the existing coupling between analysis logic and storage-specific query behavior.

That would make future parity work harder to reason about and harder to test.

### Rejected: full greenfield rewrite of analysis first

Rewriting root cause, anomalies, causal paths, and namespace graph all at once around a brand-new engine would create too much surface area and too much behavior drift at the same time.

### Chosen: backend-neutral analysis store

This preserves the current Falkor path while creating a clean seam for embedded mode. It also makes parity testing possible at the analyzer layer and at the HTTP layer.

## Embedded Snapshot Model

At startup, embedded mode should parse imported events once and build an immutable in-memory snapshot.

The snapshot should contain:

- resource identities keyed by UID
- ordered change events keyed by resource UID
- ordered Kubernetes `Event` objects keyed by involved object UID
- relationship edges with type, endpoints, timestamps, and supporting metadata
- indexes by namespace, kind, label sets, selectors, service accounts, role bindings, and nodes
- enough temporal metadata to answer queries at a requested timestamp and lookback window

The snapshot is not a general-purpose database. It is a read-optimized analysis model for a fixed imported dataset.

## Shared Extraction Strategy

Relationship extraction should be shared with the existing graph sync code as much as possible.

The main reuse target is the resource-relationship extraction logic in `internal/graph/sync/builder.go` and its extractor registry:

- ownership edges
- selector edges
- scheduling edges
- service account edges
- RBAC edges
- Flux manager/reference edges
- Gateway, cert-manager, external-secrets, and other CRD extractors already present

Embedded mode should reuse extraction logic, but it should not reuse graph-database query execution as its read path.

## Analysis Store Responsibilities

The new analysis store should expose domain queries such as:

- fetch resource identity by UID
- fetch ownership chain for a resource at a timestamp
- fetch manager relationships for a set of resources
- fetch related resources for a set of resources within a lookback window
- fetch change events for resources within a lookback window
- fetch Kubernetes events for resources within a lookback window
- fetch latest events for a set of resources at a timestamp
- fetch spec changes or the event data needed to compute them
- fetch namespace graph resources and relationships

The interface should be derived from the real needs of the analyzers, not from storage-layer abstractions.

## Temporal Semantics

The biggest design challenge is temporal correctness.

The store must provide consistent answers for:

- which resource state is considered active at timestamp `T`
- which edges exist at timestamp `T`
- how deleted resources remain visible when they were deleted within the lookback window
- which event is considered the latest relevant event for a resource
- which earlier event should be used as the base for diff/spec-change computation

This temporal behavior is the core substrate required for parity. Parsing events is comparatively straightforward; recreating the graph-read semantics correctly is the hard part.

## Component Boundaries

### `internal/analysis/store`

New package containing:

- the backend-neutral store interface
- shared domain types returned by the interface
- small helper utilities that are backend-agnostic

### `internal/analysis/store/falkor`

Adapter package that translates store interface calls into the current Falkor-backed queries.

This adapter exists to preserve current behavior while the analyzers are refactored away from direct `graph.Client` usage.

### `internal/analysis/store/embedded`

Embedded snapshot implementation containing:

- immutable snapshot data structures
- startup materialization logic from imported events
- in-memory implementations of the analysis store queries

### Refactored analyzers

These packages should be updated to depend on the new store interface:

- `internal/analysis`
- `internal/analysis/anomaly`
- `internal/analysis/causal_paths`
- `internal/analysis/namespace_graph`

## Migration Strategy

The migration should proceed in this order:

1. Define the backend-neutral analysis store interface from the current analyzer query needs.
2. Implement a Falkor adapter for that interface.
3. Refactor root cause analysis to use the new interface with no intended behavior change.
4. Refactor anomaly detection and causal path discovery to build on the refactored root cause path.
5. Refactor namespace graph analysis to use the same interface.
6. Implement the embedded snapshot builder and embedded store adapter.
7. Register the graph-analysis HTTP endpoints in embedded mode once the underlying analyzers work.
8. Add dual-backend parity tests using the same imported fixture datasets.

This order keeps Falkor as the compatibility baseline and avoids introducing embedded-specific behavior changes too early.

## HTTP/API Outcome

Once the analyzers are store-backed instead of Falkor-backed, embedded mode should be able to serve:

- `/v1/metadata`
- `/v1/timeline`
- `/v1/namespace-graph`
- `/v1/causal-graph`
- `/v1/anomalies`
- `/v1/causal-paths`

The `/graph` UI route should then work in embedded mode because it depends on `/v1/metadata` and `/v1/namespace-graph`.

## Testing Strategy

The primary semantic test source should be the real fixture data already present in:

- `tests/integration/fixtures`
- `tests/integration/fixtures/golden`

These fixtures contain real Kubernetes resources and real relationships and are more suitable than the synthetic generator for parity verification.

Testing should be layered:

### Store adapter tests

- Falkor adapter tests for each store query shape
- embedded snapshot/store tests for the same query shapes
- normalized result comparison where feasible

### Analyzer parity tests

Run the same root cause, anomaly, causal path, and namespace graph scenarios against:

- Falkor-backed store
- embedded store built from the same fixture

Compare normalized outputs rather than brittle byte-for-byte responses when ordering is incidental.

### HTTP integration tests

Add endpoint-level tests that boot embedded mode from import fixtures and verify:

- namespace graph responses
- causal graph responses
- anomaly responses
- causal path responses

Existing Falkor integration tests should remain as the reference behavior.

### Performance smoke tests

Use synthetic data only for:

- import-time scaling checks
- memory-growth observation
- coarse latency smoke tests

Synthetic fixtures should not be treated as the primary semantic oracle for parity.

## Risk Mitigation

- Keep Falkor-backed behavior working through a dedicated adapter before enabling embedded parity
- Preserve current degraded fallback behavior, especially symptom-only responses when graph assembly fails
- Roll out endpoint support incrementally instead of flipping all embedded graph APIs at once
- Reuse existing relationship extractors to avoid a second source of truth for edge semantics
- Normalize parity assertions so tests fail on real behavioral differences rather than unstable ordering
- Constrain the first parity milestone to startup-imported, read-only data

## Biggest Refactor

The largest refactor is the introduction of a backend-neutral temporal analysis substrate.

That work includes:

- isolating analyzer read needs into an explicit interface
- removing direct `graph.Client` dependencies from the analysis packages
- rebuilding temporal relationship and event lookup semantics in embedded mode

This is a larger change than adding handlers, but it is the right seam if Spectre may later gain a fully integrated non-Falkor backend.

## Deferred Work

This design intentionally does not include:

- write support in embedded mode
- live watcher ingestion in embedded mode
- HTTP import in embedded mode
- background incremental snapshot mutation

If the read-only embedded approach proves viable, a later design can extend the same substrate to support writes and live ingestion.

## Expected Outcome

After this refactor, Spectre should be able to analyze imported event datasets through a single backend-neutral analysis layer.

In the short term, that enables embedded mode to support the graph UI and analysis APIs from startup-imported data.

In the longer term, it creates a cleaner architecture for full backend parity without forcing all analysis logic to remain graph-database-specific.
