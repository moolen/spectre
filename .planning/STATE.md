# GSD State: Spectre

## Project Reference

See: .planning/PROJECT.md (updated 2026-01-22)

**Core value:** Enable AI assistants to understand what's happening in Kubernetes clusters through unified MCP interface—timeline queries, graph traversal, log exploration, and metrics analysis.
**Current focus:** Phase 17 - Semantic Layer (Service Inference & Dashboard Hierarchy)

## Current Position

Phase: 17 of 19 (v1.3 Grafana Metrics Integration)
Plan: 1 of 3 (Service Inference)
Status: In progress - 17-01 complete
Last activity: 2026-01-23 — Completed 17-01-PLAN.md (Service node inference)

Progress: [█████░░░░░░░░░░░] 43% (2 of 5 phases complete, 1 of 3 plans in phase 17)

## Performance Metrics

**v1.3 Velocity:**
- Total plans completed: 7
- Average duration: 5 min
- Total execution time: 0.6 hours

**Previous Milestones:**
- v1.2: 8 plans completed
- v1.1: 12 plans completed
- v1.0: 19 plans completed

**Cumulative:**
- Total plans: 46 complete (v1.0-v1.3 phase 17 plan 1)
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
- Check for variables in both metric names and label selector values — 16-01
- MERGE-based upsert semantics for all nodes - simpler than separate CREATE/UPDATE logic — 16-02
- Full dashboard replace pattern - simpler than incremental panel updates — 16-02
- Metric nodes preserved on dashboard delete - shared entities across dashboards — 16-02
- Graceful degradation: log parse errors but continue with other panels/queries — 16-02
- Dashboard sync optional - integration works without graph client — 16-02
- SetGraphClient injection pattern - transitional API for graph client access — 16-02
- IntegrationStatus type in types.go - unified status representation for all integrations — 16-03
- Interface-based type assertion for optional integration features (Syncer, StatusProvider) — 16-03
- SSE stream includes sync status for real-time updates — 16-03

From Phase 17:
- Service identity = {name, cluster, namespace} for proper scoping — 17-01
- Multiple service nodes when labels disagree instead of choosing one — 17-01
- Unknown service with empty cluster/namespace when no labels present — 17-01
- TRACKS edges from Metric to Service (not Query to Service) — 17-01

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

**Last session:** 2026-01-23T00:31:41Z
**Stopped at:** Completed 17-01-PLAN.md (Service node inference)
**Resume file:** None

**Context preserved:** Phase 17 Plan 01 complete - Service inference with label priority, cluster/namespace scoping, and TRACKS edges

**Next step:** Continue to 17-02 (Dashboard Hierarchy Classification) or 17-03 (Variable Classification)

---
*Last updated: 2026-01-23 — Phase 17 Plan 01 complete*
