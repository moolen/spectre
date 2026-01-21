# GSD State: Spectre Server Consolidation

## Project Reference

See: .planning/PROJECT.md (updated 2026-01-21)

**Core value:** Enable AI assistants to understand Kubernetes clusters through unified MCP interface
**Current focus:** v1.1 Server Consolidation — single-port deployment with in-process MCP

## Current Position

Phase: Phase 7 — Service Layer Extraction (2 of 4) — IN PROGRESS
Plan: 07-04 complete (4 of 5 plans in phase)
Status: In progress - Timeline, Graph, and Metadata services extracted
Last activity: 2026-01-21 — Completed 07-04-PLAN.md (MetadataService extraction)

Progress: ██████░░░░░░░░░░░░░░ 30% (6/20 total plans estimated)

## Milestone: v1.1 Server Consolidation

**Goal:** Single server binary serving REST API, UI, and MCP on one port (:8080)

**Phases:**
- Phase 6: Consolidated Server & Integration Manager (7 reqs) — COMPLETE (2/2 plans complete)
- Phase 7: Service Layer Extraction (5 reqs) — IN PROGRESS (4/5 plans complete)
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
- Plans complete: 6/20 (estimated)
- Requirements satisfied: 13/21 (SRVR-01 through INTG-03, SVCE-01 through SVCE-04)

**Session metrics:**
- Current session: 2026-01-21
- Plans executed this session: 6
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
| 07-04 | MetadataService returns cache hit status | Service returns (response, cacheHit bool, error) tuple | Handler uses cacheHit for X-Cache header, cleaner than handler inspecting cache |
| 07-04 | useCache hardcoded to true in handler | Metadata changes infrequently, always prefer cache | Simplifies API surface, cache fallback handled by service |
| 07-04 | Service handles both efficient and fallback query paths | Check for MetadataQueryExecutor interface, fallback if unavailable | Centralizes query path selection in service layer |

### Active TODOs

*Updated as work progresses*

### Deferred Issues

- DateAdded persistence (v1 debt, not blocking v1.1)
- GET /{name} endpoint usage (v1 debt, not blocking v1.1)

## Session Continuity

**Last command:** Executed 07-04-PLAN.md (MetadataService extraction)
**Last output:** 07-04-SUMMARY.md created, STATE.md updated
**Context preserved:** MetadataService created with cache integration, REST metadata handler refactored to thin adapter

**On next session:**
- Phase 7 IN PROGRESS — 4 of 5 plans complete (SVCE-01 through SVCE-04 satisfied)
- Service layer pattern complete for all core API operations (Timeline, Graph, Metadata)
- REST handlers follow thin adapter pattern, delegate all business logic to services
- Services encapsulate cache integration and query path selection
- Next: Plan 07-05 (final plan) - Wire MCP metadata tool to use MetadataService directly
- After Phase 7: Phase 8 cleanup and Helm chart updates

---
*Last updated: 2026-01-21 — Completed Phase 7 Plan 4*
