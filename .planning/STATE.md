# GSD State: Spectre

## Project Reference

See: .planning/PROJECT.md (updated 2026-01-22)

**Core value:** Enable AI assistants to understand what's happening in Kubernetes clusters through unified MCP interface—timeline queries, graph traversal, log exploration, and metrics analysis.
**Current focus:** Phase 18 - Query Execution & MCP Tools Foundation

## Current Position

Phase: 18 of 19 (v1.3 Grafana Metrics Integration)
Plan: Ready to plan Phase 18
Status: Phase 17 verified, ready for Phase 18 planning
Last activity: 2026-01-23 — Phase 17 Semantic Layer verified (5/5 must-haves)

Progress: [██████░░░░░░░░░░] 60% (3 of 5 phases complete in v1.3)

## Performance Metrics

**v1.3 Velocity:**
- Total plans completed: 10
- Average duration: 4 min
- Total execution time: 0.76 hours

**Previous Milestones:**
- v1.2: 8 plans completed
- v1.1: 12 plans completed
- v1.0: 19 plans completed

**Cumulative:**
- Total plans: 49 complete (v1.0-v1.3 phase 17)
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
- Variable classification uses case-insensitive pattern matching — 17-02
- Unknown classification for unrecognized variable names — 17-02
- Graceful handling of malformed variables with warning logs — 17-02
- Variable nodes use composite key: dashboardUID + name — 17-02
- Per-tag HierarchyMap mapping (simplest, most flexible) - each tag maps to level, first match wins — 17-03
- Support both spectre:* and hierarchy:* tag formats for flexibility — 17-03
- Case-insensitive hierarchy tag matching for user convenience — 17-03
- Tags always override config mapping when both present — 17-03
- Default to "detail" level when no hierarchy signals present — 17-03
- Warning-only validation for hierarchy levels (allows save with invalid values) — 17-04
- Empty string values allowed in hierarchy mappings (cleanup on backend) — 17-04
- Inline IIFE pattern for validation warning rendering — 17-04

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

**Last command:** /gsd:execute-phase 17
**Context preserved:** Phase 17 verified (Semantic Layer), 13 requirements complete (GRPH-05, SERV-01-04, HIER-01-04, VARB-01-03, UICF-04)

**Next step:** `/gsd:discuss-phase 18` to gather context for Query Execution & MCP Tools planning

---
*Last updated: 2026-01-23 — Phase 17 Semantic Layer complete and verified*
