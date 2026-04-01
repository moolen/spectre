---
phase: 24-data-model-ingestion
plan: 03
subsystem: grafana-signal-graph-integration
tags: [grafana, signals, falkordb, graph-persistence, ttl, incremental-sync]

requires: ["24-01-signal-types-classifier", "24-02-signal-extractor"]
provides: ["signal-graph-persistence", "signal-dashboard-integration"]
affects: ["25-baseline-storage", "26-observatory-api"]

tech-stack:
  added: []
  patterns: ["merge-upsert", "composite-key-deduplication", "graceful-degradation"]

key-files:
  created: []
  modified:
    - path: "internal/integration/grafana/graph_builder.go"
      lines: 1044
      description: "Added BuildSignalGraph method for SignalAnchor node creation"
    - path: "internal/integration/grafana/graph_builder_test.go"
      lines: 1636
      description: "Added 5 test cases for BuildSignalGraph (single, idempotency, multiple, no-workload, empty)"
    - path: "internal/integration/grafana/dashboard_syncer.go"
      lines: 468
      description: "Hooked signal extraction into syncDashboard with ingestSignals helper"

decisions:
  - id: "signal-graph-composite-key"
    choice: "metric_name + workload_namespace + workload_name + integration"
    rationale: "Allows same metric+workload per Grafana instance, deduplicates across dashboards"
    impact: "Idempotent signal ingestion, ON MATCH updates fields except first_seen"

  - id: "signal-relationships"
    choice: "SOURCED_FROM (Dashboard), REPRESENTS (Metric), MONITORS (ResourceIdentity)"
    rationale: "Links signals to dashboard graph and K8s workloads for traversal queries"
    impact: "Enables graph queries: signal->dashboard, signal->metric, signal->workload"

  - id: "graceful-signal-failure"
    choice: "Signal extraction errors logged but don't fail dashboard sync"
    rationale: "Dashboard sync is critical, signals are additive intelligence"
    impact: "Signal failures don't block core dashboard ingestion"

metrics:
  duration: "227s (3min 47sec)"
  completed: "2026-01-29"
  tasks: 2
  commits: 2
  tests-added: 5
  lines-modified: 583
---

# Phase 24 Plan 03: Signal Graph Integration Summary

**One-liner:** SignalAnchor nodes persisted to FalkorDB with MERGE upsert, linked to Dashboard/Metric/ResourceIdentity, triggered by hourly dashboard sync

## What Was Built

### 1. BuildSignalGraph Method (graph_builder.go)

Extended GraphBuilder with `BuildSignalGraph(ctx, signals)` for persisting SignalAnchor nodes:

**MERGE Upsert Semantics:**
- Composite key: `metric_name + workload_namespace + workload_name + integration`
- ON CREATE: Sets all fields including `first_seen`
- ON MATCH: Updates `role`, `confidence`, `quality_score`, `last_seen`, `expires_at` (preserves `first_seen`)

**Relationships Created:**
- `(SignalAnchor)-[:SOURCED_FROM]->(Dashboard)` — links to source dashboard
- `(SignalAnchor)-[:REPRESENTS]->(Metric)` — links to metric node (MERGE creates if missing)
- `(SignalAnchor)-[:MONITORS]->(ResourceIdentity)` — optional link to K8s workload (if exists)

**TTL Mechanism:**
- `expires_at = last_seen + 7 days` (nanosecond timestamp)
- Query-time filtering: `WHERE expires_at > $now`
- Follows v1.4 TTL pattern (state transitions, alert edges)

**Graceful Error Handling:**
- Relationship creation failures logged, don't fail entire batch
- Signal node still created if relationships fail
- Continues processing remaining signals

### 2. Dashboard Signal Ingestion (dashboard_syncer.go)

Modified `syncDashboard` to call `ingestSignals` after dashboard graph creation:

**ingestSignals Flow:**
1. Call stub methods `getAlertRuleCount`, `getViewsLast30Days` (return 0 for now)
2. Compute quality score via `ComputeDashboardQuality`
3. Extract signals via `ExtractSignalsFromDashboard`
4. Persist signals via `BuildSignalGraph`

**Graceful Failure:**
- Signal extraction errors logged with `Warn`
- Don't return error from `syncDashboard` if signal ingestion fails
- Dashboard sync succeeds independently of signal extraction

**Stub Methods:**
- `getAlertRuleCount(dashboardUID)` → returns 0
- `getViewsLast30Days(dashboardUID)` → returns 0
- TODO markers for future implementation (query Grafana API or graph)

**Sync Integration:**
- Signal ingestion piggybacks on existing hourly dashboard sync
- Inherits incremental sync pattern (only syncs changed dashboards)
- No new scheduler or background job needed

## Test Coverage

Added 5 test cases for `BuildSignalGraph`:

| Test | What It Validates |
|------|-------------------|
| `TestBuildSignalGraph_SingleSignal` | Creates SignalAnchor node with all 4 relationships (node, SOURCED_FROM, REPRESENTS, MONITORS) |
| `TestBuildSignalGraph_MERGEIdempotency` | Same composite key updates fields on second insert, preserves first_seen |
| `TestBuildSignalGraph_MultipleSignals` | Batch processing of 3 signals with different metrics and workloads |
| `TestBuildSignalGraph_NoWorkloadName` | Namespace-only signal (empty workload_name) doesn't create MONITORS edge |
| `TestBuildSignalGraph_EmptySignals` | Empty array handled gracefully, no queries executed |

All existing DashboardSyncer tests still pass (lifecycle, start/stop).

## Deviations from Plan

None - plan executed exactly as written.

## Implementation Notes

**Composite Key Design:**
- Integration name included in key to support multi-Grafana setups
- Same metric+workload can exist per Grafana instance
- Enables deduplication across dashboards within one Grafana

**Relationship Creation Pattern:**
- Each relationship created in separate query (not atomic batch)
- Allows partial success: signal node useful even if edges fail
- MONITORS edge uses OPTIONAL MATCH (ResourceIdentity may not exist)

**Quality Score Defaults:**
- Alert count and views default to 0 (stub methods)
- Quality formula still works: base = (Freshness + 0 + Ownership + Completeness) / 4
- Alert boost disabled until stubs replaced

**TTL Expiration:**
- Follows v1.4 pattern: expires_at timestamp, query-time WHERE clause
- No background cleanup job (query filters expired nodes)
- 7-day window matches alert state transition TTL

## Next Phase Readiness

**Phase 25 (Baseline Storage) Requirements:**
- ✅ SignalAnchor nodes available in graph
- ✅ Composite key enables deduplication
- ✅ TTL mechanism ready (expires_at field)
- ✅ Graph relationships enable traversal queries
- ⚠️ Quality scores partially available (alert/views stubs return 0)

**Phase 26 (Observatory API) Requirements:**
- ✅ SignalAnchor nodes queryable via FalkorDB
- ✅ SOURCED_FROM enables dashboard context lookup
- ✅ REPRESENTS enables metric rollup queries
- ✅ MONITORS enables workload filtering

**Blockers:**
- None - all Phase 25/26 requirements met
- Quality score accuracy will improve when stubs replaced (non-blocking)

## Performance Characteristics

**Signal Ingestion Overhead:**
- Per signal: 4 graph queries (node + 3 relationships)
- Typical dashboard: 10-30 signals → 40-120 queries
- Piggybacks on hourly sync (no new background job)
- Graceful failure prevents blocking dashboard sync

**Graph Query Complexity:**
- MERGE with composite key: O(1) with index on (metric_name, workload_namespace, workload_name, integration)
- OPTIONAL MATCH for MONITORS: Safe for missing ResourceIdentity nodes
- Relationship creation: O(1) lookups with node indexes

**Memory Usage:**
- In-memory signal deduplication during extraction (per dashboard)
- Batch processing of all signals for one dashboard at once
- No persistent cache or state

## Commit History

| Commit | Description | Files | Tests |
|--------|-------------|-------|-------|
| `53152be` | feat(24-03): add BuildSignalGraph with MERGE upsert | graph_builder.go, graph_builder_test.go | +5 |
| `210c4fb` | feat(24-03): hook signal extraction into DashboardSyncer | dashboard_syncer.go | 0 |

## Files Modified

```
internal/integration/grafana/
├── graph_builder.go (+181 lines)
│   └── BuildSignalGraph method with MERGE upsert and relationship creation
├── graph_builder_test.go (+320 lines)
│   └── 5 test cases for BuildSignalGraph (single, idempotency, multiple, no-workload, empty)
└── dashboard_syncer.go (+82 lines)
    └── ingestSignals helper + stub methods for quality scoring
```

## Success Criteria

- [x] GraphBuilder has BuildSignalGraph method with MERGE upsert
- [x] Composite key: metric_name + workload_namespace + workload_name + integration
- [x] ON MATCH preserves first_seen, updates other fields
- [x] DashboardSyncer calls signal extraction after dashboard sync
- [x] Signal failures don't fail dashboard sync
- [x] TTL: 7 days via expires_at

**All success criteria met.**
