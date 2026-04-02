# Embedded Primary Storage Engine Design

**Date:** 2026-04-02

**Status:** Approved in design conversation

## Goal

Evolve Spectre's embedded backend from an in-memory projection over a durable journal into the primary storage engine for the product, optimized for:

- fast recent-window UX
- medium-scale single-node operation
- asynchronous durability with tolerated recent-data loss on crash
- bounded restart time
- efficient timeline/search/export reads
- efficient resource-centric analysis reads

The engine should keep recent data hot in memory, move older history into immutable on-disk segments, and persist enough derived state to avoid replaying the full corpus on every restart.

## Constraints

- Embedded storage is intended to become the primary backend for Spectre
- Optimize for medium-scale single-instance deployments
  - roughly up to millions of events per day
  - tens of GB of local data
- Durability model is throughput-first
  - accepted writes may initially live only in memory
  - small recent-data loss on crash is acceptable
- Recent UX must stay fast
- Older history may be slower, but still interactive
  - cold-history queries in the 200 ms to 2 s range are acceptable
- Observatory graph parity is not part of this design
- The design should remain understandable and operable by Spectre engineers without introducing a generic database platform inside the repo

## Problem Statement

The current embedded backend is suitable as a first replacement step for FalkorDB-backed reads, but it is still shaped as:

- an in-memory mutable projection
- a durable append/replay journal
- restart recovery by replaying the event journal into RAM

That is enough for correctness at modest scale, but not enough for a primary long-lived engine because:

- restart cost grows with total history
- all historical reads depend on in-memory materialization
- raw event history, recent working set, and derived analysis state are not tiered separately
- the backend does not yet have a cold on-disk query structure with bounded memory
- compaction, checkpointing, and cold-query planning are absent

The primary engine needs to separate hot mutable state from cold immutable history and to persist derived read models in a form that supports fast restart and fast resource-centric analysis.

## Observed Query Patterns

The existing Spectre codebase already reveals the real access patterns the engine must serve.

### Timeline and search

Primary paths:

- `/v1/timeline`
- `/v1/search`
- Connect/gRPC timeline service
- MCP timeline tools

Observed shape:

- time-window queries dominate
- optional filtering by `namespace`, `kind`, `name`, `uid`, `group`, `version`
- timeline pagination is effectively resource-centric, not raw-event-centric
- recent windows are the dominant interactive path
- timeline service also issues a second query for Kubernetes `Event` objects in the same time range

Implication:

- the engine must optimize recent time-window scans
- resource-local recent event tails should stay hot
- Kubernetes Event association should be separately indexed by involved UID

### Metadata

Primary path:

- `/v1/metadata`

Observed shape:

- distinct namespaces
- distinct kinds
- global time range

Implication:

- this must be served from maintained metadata state, not by scanning segments

### Analysis and graph-like resource reads

Primary paths:

- `/v1/causal-graph`
- `/v1/causal-paths`
- `/v1/anomalies`
- `/v1/namespace-graph`
- root cause analysis and anomaly detection internals

Observed shape:

- point lookup by resource UID
- bounded recent history per UID
- short traversals over ownership, managers, references, selectors, and namespace membership
- recent change events and K8s events for a small set of UIDs

Implication:

- these queries should read a materialized resource/relationship projection
- they should not rebuild state by scanning raw history

### Export

Primary path:

- `/v1/storage/export`

Observed shape:

- full time-range scan
- potentially many pages
- light filtering compared to timeline and analysis

Implication:

- immutable segment scans by time should be a first-class path

## Architectural Decision

Adopt a tiered embedded engine with three coordinated layers:

1. **Hot Store**
   - mutable in-memory ingest buffer and recent working set
2. **Cold Segment Store**
   - immutable on-disk event segments with sparse indexes
3. **Projection Store**
   - materialized resource and relationship state for analysis and metadata

The projection store exists both:

- in memory for hot reads
- on disk as checkpoints for restart

The raw event plane and the derived projection plane are deliberately separate. Timeline/export queries may consult segments directly, while analysis queries should resolve mostly through the projection store.

## Rejected Approaches

### Rejected: keep the current journal + full replay model and just optimize replay

This delays the core issue rather than solving it. Faster replay still leaves:

- restart time proportional to total retained history
- no true cold-query structure
- no memory bound on historical materialization

### Rejected: make SQLite the primary engine

SQLite could work as a persistence layer, but it does not align as cleanly with Spectre's mixed workload:

- recent event windows
- resource-local history tails
- derived graph-like resource relationships
- namespace graph traversal

Using SQLite would still require substantial custom projection and hot-cache machinery, without providing the engine shape we actually want. It remains a valid fallback option, but it is not the recommended primary direction.

### Rejected: segment-only event storage with minimal projections

This favors append and export, but it pushes too much work to read time:

- causal analysis would repeatedly reconstruct state
- namespace graph would become scan-heavy
- recent UX would depend on merging too many raw structures

Spectre's workload is not a pure time-series export system. It is a resource-centric event + analysis system.

## Chosen Engine Shape

### Layer 1: Hot Store

Purpose:

- absorb ingest cheaply
- serve recent queries with minimal latency
- maintain recent event tails and current resource state

Primary contents:

- newest raw events in memory
- hot recent event windows
- hot per-resource recent history
- hot metadata sets
- hot resource and relationship projection state

### Layer 2: Cold Segment Store

Purpose:

- persist older history compactly
- support historical timeline and export queries
- provide bounded restart and bounded memory growth

Primary contents:

- immutable event segments on disk
- sparse segment indexes
- per-segment stats for pruning

### Layer 3: Projection Store

Purpose:

- answer resource-centric analysis reads efficiently
- serve metadata and namespace-state reads without scanning raw history
- make restart proportional to recent tail replay, not total retention

Primary contents:

- latest resource identity and state by UID
- bounded version history per UID
- ownership, manager, reference, selector, and namespace membership state
- recent change event summaries
- recent K8s events keyed by involved UID

## In-Memory Data Structures

The hot store should be resource-first, with time as a secondary access path.

### Hot raw-event structures

- `recentEventsByTime`
  - append-friendly chunked slices or ring-backed buffers
  - ordered by `(timestamp, sequence)`
  - source for recent time-window reads and flush batching
- `recentEventsByResourceUID`
  - `map[uid]*RecentResourceLog`
  - keeps the recent tail for each resource hot
- `recentK8sEventsByInvolvedUID`
  - separate store for Kubernetes `Event` objects
  - prevents noisy event traffic from polluting general resource-history traversal

### Hot resource state

- `resourceLatestByUID`
  - latest identity, labels, status summary, timestamps
- `resourceVersionsByUID`
  - bounded recent version history used by recent change queries and analysis windows
- `resourceNameIndex`
  - `(namespace, kind, name) -> uid or uid-set`

### Hot relationship indexes

Maintain adjacency explicitly:

- `ownersByUID`
- `ownedByUID`
- `managesByUID`
- `referencesFromUID`
- `referencedByUID` if profiling shows bidirectional traversal is worth the memory
- `selectorsToTargets`
- `namespaceActiveUIDs`

### Hot metadata indexes

- `activeNamespaces`
- `activeKinds`
- `globalTimeRange`

### Memory policy

The engine should not keep all history fully indexed in memory.

Instead:

- keep recent raw history hot
- keep current resource/relationship state hot
- keep bounded recent version/change/K8s-event tails hot
- spill older raw history into cold segments

This preserves fast UX while enforcing a stable memory ceiling.

## On-Disk Layout

All embedded engine state lives under:

- `data-dir/embedded/`

Directory layout:

- `manifest.json`
- `hot/`
- `segments/<segment-id>/`
- `checkpoints/<checkpoint-id>/`
- `tmp/`

### Manifest

`manifest.json` tracks:

- storage engine version
- live segment set
- newest durable checkpoint
- flush high-water mark
- compaction bookkeeping

Manifest updates must be atomic.

### Segment bundle

Each sealed segment is immutable and represents a bounded size slice of historical data.

Initial target:

- roughly 128 MB to 512 MB per segment, tuned later

Files:

- `events.bin`
  - encoded event records
  - sorted by `(timestamp, sequence)`
- `time.idx`
  - sparse time-to-offset mapping
- `resource.idx`
  - `uid -> offset ranges`
- `dim.idx`
  - sparse postings for `(namespace, kind)` and optionally `(namespace, kind, name)` if profiling justifies it
- `stats.json`
  - min/max timestamp
  - event count
  - resource count
  - namespaces and kinds present

### Checkpoint bundle

Checkpoint stores the derived projection state, not raw history.

Files:

- `resource_state.bin`
- `resource_versions.bin`
- `relationships.bin`
- `namespace_state.bin`
- `metadata.bin`
- `checkpoint.json`

Checkpoint metadata records:

- covered segments
- covered sequence range/high-water mark
- schema version

## Write Path

Because throughput-first durability is acceptable, the write path should be memory-first.

### Ingest flow

1. watcher/import normalizes event
2. event enters hot in-memory ingest buffer
3. hot recent indexes and hot projections update immediately
4. write is acknowledged
5. background flusher groups buffered events into a sealed segment
6. periodic checkpoint persists projection state
7. background compactor rewrites older segments

### Consequence of chosen durability model

Accepted writes may be lost if process or node crashes before flush. This is acceptable by design and should be explicit in docs and readiness/operational guidance.

## Read Path

### Timeline/search planning

1. consult hot store first
2. prune cold segments by min/max timestamp
3. further prune by segment stats and `dim.idx`
4. use `resource.idx` when query resolves to a specific UID or resource name
5. merge hot and cold results by resource

### Metadata planning

Serve from projection/checkpoint metadata state:

- namespaces
- kinds
- global time range

Do not perform raw segment scans in the normal metadata path.

### Analysis planning

Serve primarily from projection store:

- `GetResource`
- `GetOwnershipChain`
- `GetManagers`
- `GetRelatedResources`
- `GetChangeEvents`
- `GetK8sEvents`
- namespace graph

Raw segment fallback should be limited to recent tail recovery or edge cases, not the primary path.

### Export planning

Export should stream sealed segments in time order using `time.idx` and sequential reads from `events.bin`.

This is the one path where raw segment scans are the intended normal behavior.

## Required Indexes

### Must-have

- time range sparse index
- `uid -> offsets` index
- `(namespace, kind)` segment presence index
- projection-side `(namespace, kind, name) -> uid`
- projection-side `uid -> latest state`
- projection-side `uid -> bounded version history`
- projection-side relationship adjacency maps
- projection-side `namespace -> active uid set`
- projection-side `uid -> recent change events`
- projection-side `uid -> recent K8s events`

### Nice-to-have later

- bloom filters for UID pruning
- postings for `(namespace, kind, name)` inside cold segments
- per-segment compression tuning and vectorized decode

### Explicitly out of scope for v1

- full-text indexing
- arbitrary composite secondary indexes
- columnar storage
- separate on-disk graph engine
- observatory graph storage parity

## Recovery Model

On startup:

1. load manifest
2. load newest valid checkpoint
3. load sealed segments newer than the checkpoint high-water mark
4. replay only the uncovered tail into hot memory
5. recover interrupted flush or compaction work using atomic rename discipline

This keeps restart cost proportional to recent uncheckpointed history, not total retention.

## Compaction Model

Compaction merges multiple old segments into one larger immutable segment.

Goals:

- reduce segment fan-out
- improve cold-query pruning efficiency
- preserve time ordering
- rebuild sparse indexes

Process:

1. select candidate old segments
2. write merged bundle into `tmp/`
3. fsync staged files
4. atomically update manifest
5. remove obsolete segments

Checkpointing and compaction should be coordinated but independent:

- compaction rewrites raw history
- checkpointing rewrites derived state

## Operational Targets

For the selected constraints, the engine should target:

- very fast recent-query latency from hot memory
- acceptable interactive cold-history queries
- bounded RAM independent of total retained history
- restart that scales with tail replay plus checkpoint load
- segment and checkpoint files that remain inspectable and operationally understandable

## Migration Strategy

The engine should evolve incrementally from the current embedded backend:

1. keep current in-memory projection semantics for hot reads
2. replace journal-only durability with segment writer and manifest
3. add projection checkpoints
4. split query planner into hot + cold paths
5. add compaction
6. tune indexes based on measured query patterns

This preserves working behavior while moving toward a real primary embedded engine.

## Final Recommendation

Promote embedded storage by building:

- a hot mutable memory tier for recent data
- immutable cold on-disk segments for history
- persisted projection checkpoints for resource-centric reads and fast restart

This is the right engine shape for Spectre's actual workload. It optimizes the user-visible hot path while keeping historical data queryable, restart bounded, and implementation complexity aligned with the product rather than with a generic database ambition.
