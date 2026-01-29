# GSD State: Spectre

## Project Reference

See: .planning/PROJECT.md (updated 2026-01-29)

**Core value:** Enable AI assistants to understand what's happening in Kubernetes clusters through unified MCP interface—timeline queries, graph traversal, log exploration, and metrics analysis.
**Current focus:** v1.5 Observatory — Phase 26: Observatory API and MCP Tools

## Current Position

Phase: 25 — Baseline & Anomaly Detection (COMPLETE)
Plan: 5 of 5 complete
Status: Phase 25 COMPLETE — Ready for Phase 26
Last activity: 2026-01-30 — Completed 25-05-PLAN.md

Progress: [█████████░░░░░░░░░░░] ~36% (Phase 24-25 complete, 9 plans shipped)

## Performance Metrics

**v1.5 Status (current):**
- Plans completed: 9
- Phase 24: 4/4 complete (24-01: 6 min, 24-02: 4 min, 24-03: 3.8 min, 24-04: 11 min) — PHASE COMPLETE
- Phase 25: 5/5 complete (25-01: 2 min, 25-02: 2.5 min, 25-03: 7 min, 25-04: 11 min, 25-05: 8 min) — PHASE COMPLETE
- Phase 26: Ready to start

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
- Total plans: 75 complete (v1.0-v1.4: 66, v1.5: 9)
- Milestones shipped: 5 (v1.0, v1.1, v1.2, v1.3, v1.4)
- v1.5 progress: 9/TBD plans complete

## Accumulated Context

### Decisions

| Decision | Context | Impact | When |
|----------|---------|--------|------|
| Layered classification with confidence decay | Need reliable metric → role mapping | 5 layers: 0.95 → 0.85-0.9 → 0.7-0.8 → 0.5 → 0 | 24-01 |
| Quality scoring with alert boost | Prioritize high-value dashboards | Formula: base + 0.2*hasAlerts, capped at 1.0 | 24-01 |
| Composite key for SignalAnchor | Deduplication across dashboards | metric_name + namespace + workload_name + integration | 24-01, 24-03 |
| 7-day TTL for signals | Stale metric cleanup | expires_at = last_seen + 7 days, query-time filtering | 24-01 |
| Namespace-only signal inference | Signals with namespace but no workload | Returns WorkloadInference with empty workload_name (confidence 0.7) | 24-02 |
| Low-confidence filter threshold | Filter unclassifiable metrics | Signals with confidence < 0.5 excluded from extraction | 24-02 |
| Workload label priority | K8s workload inference | deployment > app.kubernetes.io/name > app > service > job > pod | 24-02 |
| Deduplication winner selection | Multiple panels with same metric+workload | Highest quality signal wins, preserve FirstSeen timestamp | 24-02 |
| Signal graph relationships | Link signals to context | SOURCED_FROM (Dashboard), REPRESENTS (Metric), MONITORS (ResourceIdentity) | 24-03 |
| Graceful signal failure | Don't block dashboard sync | Signal extraction errors logged but don't fail syncDashboard | 24-03 |
| SignalBaseline composite key alignment | Match SignalAnchor identity | metric_name + namespace + workload + integration | 25-01 |
| MinSamplesRequired = 10 | Cold start baseline threshold | Per CONTEXT.md decision | 25-01 |
| Empty input returns zero RollingStats | Not error, just zero SampleCount | Error reserved for explicit cold start check | 25-01 |
| Z-score sigmoid normalization | Map unbounded z-score to 0-1 | 1 - exp(-|z|/2): z=2->0.63, z=3->0.78 | 25-02 |
| Hybrid anomaly MAX aggregation | Either method can flag anomaly | score = MAX(zScore, percentile) per CONTEXT.md | 25-02 |
| Alert firing override | Human decision takes precedence | score=1.0, confidence=1.0, method="alert-override" | 25-02 |
| MERGE upsert for SignalBaseline | Idempotent graph updates | ON CREATE/ON MATCH with composite key | 25-03 |
| Backfill rate limit 2 req/sec | Slower than forward (10 req/sec) | Protect Grafana during bulk ops | 25-04 |
| MAX aggregation for anomaly scores | Worst signal bubbles up | Per CONTEXT.md hierarchy | 25-04 |
| Quality tiebreaker | Equal scores need deterministic TopSource | Higher quality wins when scores equal | 25-04 |
| Aggregation cache 5min + jitter | Prevent thundering herd | Random 0-30s jitter on TTL | 25-04 |
| Welford's online algorithm | Incremental statistics without storing samples | Mean/variance update via delta formula | 25-03 |
| Rate limiting 10 req/sec | Protect Grafana API | 100ms ticker interval | 25-03 |
| BaselineCollector lifecycle pattern | Follow AlertStateSyncer | Start after analysis service, stop before stateSyncer | 25-05 |
| Non-fatal collector start | Warn but continue | Anomaly detection works with existing baselines | 25-05 |

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
| 24 | Signal anchors with role classification and quality scoring | 25 | 4/4 COMPLETE |
| 25 | Baseline storage and anomaly detection | 12 | 5/5 COMPLETE |
| 26 | Observatory API and 8 MCP tools | 24 | Ready to start |

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
- TestComputeDashboardQuality_Freshness has time-dependent failures (from v1.3)

## Session Continuity

**Last command:** /gsd:execute-plan 25-05
**Last session:** 2026-01-30
**Stopped at:** Completed 25-05-PLAN.md (Integration test & lifecycle)
**Resume file:** None
**Context preserved:** Phase 25 COMPLETE: All baseline storage and anomaly detection functionality implemented and tested. 10 total commits for phase 25.

**Next step:** Begin Phase 26 (Observatory API and MCP tools)

**Phase 25-05 Summary:**
- BaselineCollector wired into Grafana integration lifecycle
- End-to-end integration test suite (11 tests, 947 lines)
- Test coverage for cold start, alert override, aggregation, TTL
- All tests pass with race detector enabled
- Duration: 8 min

**Phase 25 Complete:**
- 25-01: SignalBaseline types + RollingStatistics (2 min)
- 25-02: Hybrid anomaly scorer with alert override (2.5 min)
- 25-03: Graph storage + BaselineCollector syncer (7 min)
- 25-04: BackfillService + AnomalyAggregator (11 min)
- 25-05: Integration test + lifecycle wiring (8 min)
- Total: ~30.5 min for full baseline & anomaly detection layer

---
*Last updated: 2026-01-30 — Phase 25 COMPLETE (baseline & anomaly detection ready for Observatory)*
