# GSD State: Spectre

## Project Reference

See: .planning/PROJECT.md (updated 2026-01-29)

**Core value:** Enable AI assistants to understand what's happening in Kubernetes clusters through unified MCP interface—timeline queries, graph traversal, log exploration, and metrics analysis.
**Current focus:** v1.5 Observatory — Phase 24: Data Model & Ingestion

## Current Position

Phase: 24 — Data Model & Ingestion
Plan: 02 of 3 complete
Status: In progress — Signal extraction and workload linkage complete
Last activity: 2026-01-29 — Completed 24-02-PLAN.md

Progress: [██░░░░░░░░░░░░░░░░░░░] ~8% (Phase 24/26, Plan 2 of 3)

## Performance Metrics

**v1.5 Status (current):**
- Plans completed: 2
- Phase 24: 2/3 complete (24-01: 6 min, 24-02: 4 min)
- Phase 25: Blocked by Phase 24
- Phase 26: Blocked by Phase 25

**v1.4 Velocity (previous):**
- Plans completed: 10 (COMPLETE)
- Phase 20 duration: ~10 min
- Phase 21-01 duration: 4 min
- Phase 21-02 duration: 8 min
- Phase 22-01 duration: 9 min
- Phase 22-02 duration: 6 min
- Phase 22-03 duration: 5 min (281s)
- Phase 23-01 duration: 2 min
- Phase 23-02 duration: 3 min
- Phase 23-03 duration: 3 min (215s)

**v1.3 Velocity:**
- Total plans completed: 17
- Average duration: ~5 min
- Total execution time: ~1.8 hours

**Previous Milestones:**
- v1.2: 8 plans completed
- v1.1: 12 plans completed
- v1.0: 19 plans completed

**Cumulative:**
- Total plans: 66 complete (v1.0-v1.4)
- Milestones shipped: 5 (v1.0, v1.1, v1.2, v1.3, v1.4)

## Accumulated Context

### Decisions

| Decision | Context | Impact | When |
|----------|---------|--------|------|
| Layered classification with confidence decay | Need reliable metric → role mapping | 5 layers: 0.95 → 0.85-0.9 → 0.7-0.8 → 0.5 → 0 | 24-01 |
| Quality scoring with alert boost | Prioritize high-value dashboards | Formula: base + 0.2*hasAlerts, capped at 1.0 | 24-01 |
| Composite key for SignalAnchor | Deduplication across dashboards | metric_name + namespace + workload_name | 24-01 |
| 7-day TTL for signals | Stale metric cleanup | expires_at = last_seen + 7 days, query-time filtering | 24-01 |
| Namespace-only signal inference | Signals with namespace but no workload | Returns WorkloadInference with empty workload_name (confidence 0.7) | 24-02 |
| Low-confidence filter threshold | Filter unclassifiable metrics | Signals with confidence < 0.5 excluded from extraction | 24-02 |
| Workload label priority | K8s workload inference | deployment > app.kubernetes.io/name > app > service > job > pod | 24-02 |
| Deduplication winner selection | Multiple panels with same metric+workload | Highest quality signal wins, preserve FirstSeen timestamp | 24-02 |

Recent decisions from PROJECT.md affecting v1.5:
- Signal anchors link metrics to signal roles to workloads
- Role taxonomy: Availability, Latency, Errors, Traffic, Saturation, Churn, Novelty
- Dashboard quality scoring: freshness, usage, alerting, ownership, completeness
- Hybrid collection: forward-looking periodic + opt-in catchup backfill
- Progressive disclosure: Orient -> Narrow -> Investigate -> Hypothesize -> Verify

From v1.4 (relevant to v1.5):
- Self-edge pattern for state transitions works well
- TTL via expires_at timestamp with query-time filtering
- Exponential scaling for flappiness detection
- LOCF interpolation for timeline bucketization
- 5-minute cache TTL with LRU for analysis results

### Pending Todos

None yet.

### Blockers/Concerns

None yet.

## v1.5 Phase Overview

| Phase | Goal | Requirements | Status |
|-------|------|--------------|--------|
| 24 | Signal anchors with role classification and quality scoring | 25 | 2/3 plans complete (24-01: types+classification, 24-02: extraction+linkage) |
| 25 | Baseline storage and anomaly detection | 12 | Blocked by 24 |
| 26 | Observatory API and 8 MCP tools | 24 | Blocked by 25 |

## Milestone History

- **v1.5 Observatory** — in progress
  - 3 phases (24-26), TBD plans, 61 requirements
  - Signal intelligence layer for AI-driven incident investigation

- **v1.4 Grafana Alerts Integration** — shipped 2026-01-23
  - 4 phases (20-23), 10 plans, 22 requirements
  - Alert rule sync, state tracking, flappiness analysis, three MCP tools with progressive disclosure

- **v1.3 Grafana Metrics Integration** — shipped 2026-01-23
  - 5 phases (15-19), 17 plans, 51 requirements
  - Grafana dashboards as structured knowledge with anomaly detection

- **v1.2 Logz.io Integration + Secret Management** — shipped 2026-01-22
  - 5 phases (10-14), 8 plans, 21 requirements
  - Logz.io as second log backend with SecretWatcher

- **v1.1 Server Consolidation** — shipped 2026-01-21
  - 4 phases (6-9), 12 plans, 21 requirements
  - Single-port deployment with in-process MCP

- **v1.0 MCP Plugin System + VictoriaLogs** — shipped 2026-01-21
  - 5 phases (1-5), 19 plans, 31 requirements
  - Plugin infrastructure + VictoriaLogs integration

## Tech Debt

- DateAdded field not persisted in integration config (from v1)
- GET /{name} endpoint unused by UI (from v1)

## Session Continuity

**Last command:** /gsd:execute-phase 24-02
**Last session:** 2026-01-29
**Stopped at:** Completed 24-02-PLAN.md (Signal extraction and workload linkage)
**Resume file:** None
**Context preserved:** Phase 24-02 complete: Signal extractor (panel-to-SignalAnchor transformation, multi-query support, deduplication), workload linker (K8s inference with label priority, namespace-only signals). 2 commits (1babed5, 48eee9c). 24 test cases passing. Duration: 4 minutes.

**Next step:** Continue Phase 24 (Plan 03: Graph integration with SignalAnchor nodes and edges)

---
*Last updated: 2026-01-29 — Phase 24-02 complete (signal extraction + workload linkage)*
