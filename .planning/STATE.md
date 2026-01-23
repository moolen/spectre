# GSD State: Spectre

## Project Reference

See: .planning/PROJECT.md (updated 2026-01-23)

**Core value:** Enable AI assistants to understand what's happening in Kubernetes clusters through unified MCP interface—timeline queries, graph traversal, log exploration, and metrics analysis.
**Current focus:** v1.4 Grafana Alerts Integration

## Current Position

Phase: 22 (Historical Analysis) — COMPLETE ✅
Plan: 3/3 complete (22-03 DONE)
Status: Phase 22 fully complete - AlertAnalysisService integrated into lifecycle, tested, ready for Phase 23 MCP tools
Last activity: 2026-01-23 — Completed 22-03-PLAN.md (Integration lifecycle and end-to-end tests)

Progress: [██████████████>      ] 75% (3/4 phases)

## Performance Metrics

**v1.4 Velocity (current):**
- Plans completed: 7
- Phase 20 duration: ~10 min
- Phase 21-01 duration: 4 min
- Phase 21-02 duration: 8 min
- Phase 22-01 duration: 9 min
- Phase 22-02 duration: 6 min
- Phase 22-03 duration: 5 min (281s)

**v1.3 Velocity:**
- Total plans completed: 17
- Average duration: ~5 min
- Total execution time: ~1.8 hours

**Previous Milestones:**
- v1.2: 8 plans completed
- v1.1: 12 plans completed
- v1.0: 19 plans completed

**Cumulative:**
- Total plans: 63 complete (v1.0-v1.4 Phase 22-03)
- Milestones shipped: 4 (v1.0, v1.1, v1.2, v1.3)

## Accumulated Context

### Decisions

Recent decisions from PROJECT.md affecting v1.4:
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

From Phase 20:
- Alert rule metadata stored in AlertNode (definition), state tracking deferred to Phase 21 — 20-01
- AlertQuery.Model as json.RawMessage for flexible PromQL parsing — 20-01
- Integration field in AlertNode for multi-Grafana support — 20-01
- ISO8601 string comparison for timestamp-based incremental sync (no parse needed) — 20-02
- Shared GraphBuilder instance between Dashboard and Alert syncers — 20-02
- Integration name parameter in GraphBuilder constructor for consistent node tagging — 20-02
- First PromQL expression stored as condition field for alert display — 20-02
- Alert→Service relationships accessed transitively via Metrics (no direct edge) — 20-02

From Phase 21:
- Prometheus-compatible /api/prometheus/grafana/api/v1/rules endpoint for alert states — 21-01
- 7-day TTL via expires_at RFC3339 timestamp with WHERE filtering (no cleanup job) — 21-01
- State deduplication via getLastKnownState comparison before edge creation — 21-01
- Map "alerting" to "firing" state, normalize to lowercase — 21-01
- Extract UID from grafana_uid label in Prometheus response — 21-01
- Self-edge pattern for state transitions: (Alert)-[STATE_TRANSITION]->(Alert) — 21-01
- Return "unknown" for missing state (not error) to handle first sync gracefully — 21-01
- MERGE for Alert node in state sync to handle race with rule sync — 21-01
- Periodic state sync with 5-minute interval (independent from 1-hour rule sync) — 21-02
- State aggregation: worst-case across instances (firing > pending > normal) — 21-02
- Per-alert last_synced_at timestamp for staleness tracking (not global) — 21-02
- Partial failures OK: continue sync with other alerts on graph errors — 21-02
- strings.Contains for query detection in mocks (more reliable than parameter matching) — 21-02

From Phase 22:
- Exponential scaling for flappiness (1 - exp(-k*count)) instead of linear ratio — 22-01
- Duration multipliers penalize short-lived states (1.3x) vs long-lived (0.8x) — 22-01
- LOCF daily buckets with state carryover for multi-day baseline variance — 22-01
- 24h minimum data requirement for statistically meaningful baselines — 22-01
- Transitions at period boundaries are inclusive (careful timestamp logic) — 22-01
- Sample variance (N-1) via gonum.org/v1/gonum/stat.StdDev for unbiased estimator — 22-01
- 5-minute cache TTL with 1000-entry LRU for analysis results — 22-02
- Multi-label categorization: independent onset and pattern categories — 22-02
- LOCF interpolation for state duration computation fills gaps realistically — 22-02
- Chronic threshold: >80% firing over 7 days using LOCF — 22-02
- Flapping overrides trend patterns (flappiness > 0.7) — 22-02
- ErrInsufficientData with Available/Required fields for clear error messages — 22-02
- AlertAnalysisService created in Start after graphClient (no Start/Stop methods) — 22-03
- GetAnalysisService() getter returns nil when graph disabled (clear signal to MCP tools) — 22-03
- Service shares graphClient with AlertSyncer and AlertStateSyncer (no separate client) — 22-03

### Pending Todos

None yet.

### Blockers/Concerns

None yet.

## Milestone History

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

**Last command:** Execute plan 22-03
**Last session:** 2026-01-23
**Stopped at:** Completed 22-03-PLAN.md (Integration lifecycle and tests)
**Resume file:** None
**Context preserved:** Phase 22 COMPLETE ✅ - AlertAnalysisService integrated into GrafanaIntegration lifecycle, accessible via GetAnalysisService(), 5 integration tests verify end-to-end functionality (full history, flapping, insufficient data, cache, lifecycle). Service created in Start after graphClient init, shares graph client with syncers, no Start/Stop methods (stateless). ~71% test coverage (core logic >85%). Ready for Phase 23 MCP tools.

**Next step:** Execute Phase 23 plans to create MCP tools for alert analysis (list_alerts with filters, analyze_alert, get_flapping_alerts). Service access pattern: `integration.GetAnalysisService()` returns nil if graph disabled.

---
*Last updated: 2026-01-23 — Phase 22-03 complete (Integration lifecycle wiring)*
