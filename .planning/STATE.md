# GSD State: Spectre

## Project Reference

See: .planning/PROJECT.md (updated 2026-01-22)

**Core value:** Enable AI assistants to understand what's happening in Kubernetes clusters through unified MCP interface—timeline queries, graph traversal, log exploration, and metrics analysis.
**Current focus:** v1.3 Milestone Complete - Ready for Audit

## Current Position

Phase: 19 of 19 (v1.3 Grafana Metrics Integration)
Plan: All plans complete
Status: Phase 19 verified - All 6 success criteria passed
Last activity: 2026-01-23 — Phase 19 Anomaly Detection complete and verified

Progress: [████████████████] 100% (5 of 5 phases complete in v1.3)

## Performance Metrics

**v1.3 Velocity:**
- Total plans completed: 17
- Average duration: ~5 min
- Total execution time: ~1.8 hours

**Previous Milestones:**
- v1.2: 8 plans completed
- v1.1: 12 plans completed
- v1.0: 19 plans completed

**Cumulative:**
- Total plans: 56 complete (v1.0-v1.3 phase 19 plan 4)
- Milestones shipped: 3

## Accumulated Context

### Decisions

Recent decisions from PROJECT.md affecting v1.3:
- Query via Grafana API (not direct Prometheus) — simpler auth, variable handling
- No metric storage — query historical ranges on-demand
- Dashboards are intent, not truth — treat as fuzzy signals
- Progressive disclosure — overview → aggregated → details

From Phase 15:
- SecretWatcher duplication (temporary) - refactor to common package deferred — 15-01
- Dashboard access required for health check, datasource access optional — 15-01
- Follows VictoriaLogs integration pattern exactly for consistency — 15-01
- Generic factory pattern eliminates need for type-specific switch cases in test handler — 15-03
- Blank import pattern for factory registration via init() functions — 15-03

From Phase 16:
- Use official Prometheus parser instead of custom regex parsing — 16-01
- Detect variable syntax before parsing to handle unparseable queries gracefully — 16-01
- Return partial extraction for queries with variables instead of error — 16-01
- MERGE-based upsert semantics for all nodes — 16-02
- Full dashboard replace pattern - simpler than incremental panel updates — 16-02
- Graceful degradation: log parse errors but continue with other panels/queries — 16-02
- IntegrationStatus type in types.go - unified status representation — 16-03

From Phase 17:
- Service identity = {name, cluster, namespace} for proper scoping — 17-01
- Multiple service nodes when labels disagree instead of choosing one — 17-01
- Variable classification uses case-insensitive pattern matching — 17-02
- Per-tag HierarchyMap mapping - each tag maps to level, first match wins — 17-03
- Default to "detail" level when no hierarchy signals present — 17-03

From Phase 18:
- Query types defined in client.go alongside client methods — 18-01
- formatTimeSeriesResponse is package-private (called by query service) — 18-01
- Dashboard JSON fetched from graph (not Grafana API) since it's already synced — 18-01
- Only first target per panel executed (most panels have single target) — 18-01
- dashboardInfo type shared across all tools — 18-02
- Query service requires graph client (tools not registered without it) — 18-03
- Tool descriptions guide AI on progressive disclosure usage — 18-03

From Phase 19:
- Sample variance (n-1) for standard deviation computation — 19-01
- Error metrics use lower thresholds (2σ critical vs 3σ for normal metrics) — 19-01
- Absolute z-score for bidirectional anomaly detection — 19-01
- Pattern-based error metric detection (5xx, error, failed, failure) — 19-01
- TTL implementation via expires_at Unix timestamp in graph (no application-side cleanup) — 19-02
- Weekday/weekend separation for different baseline patterns — 19-02
- DataFrame parsing: ExecuteDashboard returns time-series data in Values arrays, not single snapshots — 19-03
- Metric name extraction via __name__ label with fallback to label pair construction — 19-03
- Omit dashboard results when anomalies found (minimal context optimization) — 19-03
- Run anomaly detection on first dashboard only (primary overview dashboard) — 19-03
- Integration tests focus on helper function validation rather than complex service mocking — 19-04
- Map iteration non-determinism handled via acceptAnyKey pattern in tests — 19-04
- Time-based tests use explicit date construction with day-of-week comments — 19-04

### Pending Todos

None yet.

### Blockers/Concerns

None yet.

## Milestone History

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

**Last command:** /gsd:execute-phase 19
**Last session:** 2026-01-23
**Stopped at:** Phase 19 execution and verification complete
**Resume file:** None
**Context preserved:** All 5 phases of v1.3 complete - Grafana Metrics Integration with anomaly detection

**Next step:** `/gsd:audit-milestone` to verify milestone completion

---
*Last updated: 2026-01-23 — Phase 19 Anomaly Detection complete and verified*
