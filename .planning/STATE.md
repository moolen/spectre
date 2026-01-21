# GSD State: Spectre Server Consolidation

## Project Reference

See: .planning/PROJECT.md (updated 2026-01-21)

**Core value:** Enable AI assistants to understand Kubernetes clusters through unified MCP interface
**Current focus:** v1.1 Server Consolidation — single-port deployment with in-process MCP

## Current Position

Phase: Phase 7 — Service Layer Extraction (2 of 4) — IN PROGRESS
Plan: 07-02 complete (2 of 5 plans in phase)
Status: In progress - Timeline and Graph services extracted, MCP tools wired
Last activity: 2026-01-21 — Completed 07-02-PLAN.md (GraphService extraction with MCP tool wiring)

Progress: ████░░░░░░░░░░░░░░░░ 20% (4/20 total plans estimated)

## Milestone: v1.1 Server Consolidation

**Goal:** Single server binary serving REST API, UI, and MCP on one port (:8080)

**Phases:**
- Phase 6: Consolidated Server & Integration Manager (7 reqs) — COMPLETE (2/2 plans complete)
- Phase 7: Service Layer Extraction (5 reqs) — IN PROGRESS (2/5 plans complete)
- Phase 8: Cleanup & Helm Chart Update (5 reqs) — Pending
- Phase 9: E2E Test Validation (4 reqs) — Pending

**Total requirements:** 21

## Milestone History

- **v1 MCP Plugin System + VictoriaLogs** — shipped 2026-01-21
  - 5 phases, 19 plans, 31 requirements
  - See .planning/milestones/v1-ROADMAP.md

## Open Blockers

None

## Tech Debt

- DateAdded field not persisted in integration config (from v1)
- GET /{name} endpoint unused by UI (from v1)

## Next Steps

1. `/gsd:plan-phase 7` — Plan service layer extraction
2. Execute Phase 7 plans (convert MCP tools to use direct service calls)
3. Continue through phases 8-9

## Performance Metrics

**v1.1 Milestone:**
- Phases complete: 1/4 (Phase 6 ✅)
- Plans complete: 4/20 (estimated)
- Requirements satisfied: 9/21 (SRVR-01 through INTG-03, SVCE-01 through SVCE-02)

**Session metrics:**
- Current session: 2026-01-21
- Plans executed this session: 4
- Blockers hit this session: 0

## Accumulated Context

### Key Decisions

| Phase | Decision | Rationale | Impact |
|-------|----------|-----------|--------|
| 06-01 | Use /v1/mcp instead of /mcp | API versioning consistency with /api/v1/* | Requirement docs specify /mcp, implementation uses /v1/mcp |
| 06-01 | Use --stdio flag instead of --transport=stdio | Simpler boolean vs enum | Requirement docs specify --transport=stdio, implementation uses --stdio |
| 06-01 | MCP server self-references localhost:8080 | Reuse existing tool implementations during transition | Phase 7 will eliminate HTTP overhead with direct service calls |
| 06-01 | StreamableHTTPServer with stateless mode | Client compatibility for session-less MCP clients | Each request includes full context |
| 06-02 | Phase 6 requirements fully validated | All 7 requirements verified working | Single-port deployment confirmed stable for production |
| 07-01 | Create API server before MCP server | TimelineService created by API server, needed by MCP tools | Enables direct service sharing, required init order change |
| 07-01 | Add RegisterMCPEndpoint for late registration | MCP endpoint must register after MCP server creation | Clean separation of API server construction and MCP registration |
| 07-01 | WithClient constructors for backward compatibility | Agent tools still use HTTP client pattern | Both patterns supported during transition |
| 07-02 | GraphService wraps existing analyzers | Facade pattern over PathDiscoverer, AnomalyDetector, Analyzer | Reuses proven logic, provides unified interface |
| 07-02 | Timeline integration deferred for detect_anomalies | TimelineService integration complex, uses HTTP for now | Keeps plan focused on graph operations |
| 07-02 | Dual constructors for MCP tools | NewTool(service) and NewToolWithClient(client) | Enables gradual migration, backward compatibility |

### Active TODOs

*Updated as work progresses*

### Deferred Issues

- DateAdded persistence (v1 debt, not blocking v1.1)
- GET /{name} endpoint usage (v1 debt, not blocking v1.1)

## Session Continuity

**Last command:** Executed 07-02-PLAN.md (GraphService extraction with MCP tool wiring)
**Last output:** 07-02-SUMMARY.md created, STATE.md updated
**Context preserved:** GraphService wraps analyzers, REST handlers refactored, MCP graph tools call services directly

**On next session:**
- Phase 7 IN PROGRESS — 2 of 5 plans complete (SVCE-01, SVCE-02 satisfied)
- Service layer pattern proven for Timeline and Graph operations
- MCP tools successfully using direct service calls (no HTTP for graph operations)
- Next: Continue Phase 7 service extractions (plans 03-05)
- REST handlers follow thin adapter pattern, MCP tools call services directly

---
*Last updated: 2026-01-21 — Completed Phase 7 Plan 2*
