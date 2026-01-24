# GSD State: Spectre Server Consolidation

## Project Reference

See: .planning/PROJECT.md (updated 2026-01-21)

**Core value:** Enable AI assistants to understand Kubernetes clusters through unified MCP interface
**Current focus:** v1.1 Server Consolidation — single-port deployment with in-process MCP

## Current Position

Phase: Phase 8 — Cleanup & Helm Chart Update (3 of 4) — IN PROGRESS
Plan: 08-01 complete (1 of 2 plans in phase)
Status: In progress - Dead code cleanup complete, Helm chart updates next
Last activity: 2026-01-21 — Completed 08-01-PLAN.md (removed standalone commands)

Progress: ████████░░░░░░░░░░░░ 40% (8/20 total plans estimated)

## Milestone: v1.1 Server Consolidation

**Goal:** Single server binary serving REST API, UI, and MCP on one port (:8080)

**Phases:**
- Phase 6: Consolidated Server & Integration Manager (7 reqs) — COMPLETE (2/2 plans complete)
- Phase 7: Service Layer Extraction (5 reqs) — COMPLETE (5/5 plans complete)
- Phase 8: Cleanup & Helm Chart Update (5 reqs) — IN PROGRESS (1/2 plans complete)
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

1. Execute 08-02-PLAN.md — Update Helm chart for consolidated server
2. Phase 9: E2E test validation

## Performance Metrics

**v1.1 Milestone:**
- Phases complete: 2/4 (Phase 6 ✅, Phase 7 ✅)
- Plans complete: 8/20 (estimated)
- Requirements satisfied: 19/21 (SRVR-01 through CLNP-01)

**Session metrics:**
- Current session: 2026-01-21
- Plans executed this session: 8
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
| 07-05 | Delete HTTP client completely | HTTP client only used for self-calls in integrated server | Eliminates localhost HTTP overhead, cleaner service-only architecture |
| 07-05 | Disable standalone MCP and agent commands | Commands require HTTP to remote server, out of scope for Phase 7 | Breaking change acceptable, can refactor with gRPC/Connect in future |
| 07-05 | Build constraints on agent package | Agent depends on deleted HTTP client | Excludes agent from compilation, documents need for refactoring |
| 08-01 | Complete deletion approach for dead code | No TODO comments or deprecation stubs | Clean removal per Phase 8 context, deleted 14,676 lines (74 files) |
| 08-01 | Keep debug command even without subcommands | Future debug utilities may be added | Appears in Additional Help Topics, ready for future use |

### Active TODOs

*Updated as work progresses*

### Deferred Issues

- DateAdded persistence (v1 debt, not blocking v1.1)
- GET /{name} endpoint usage (v1 debt, not blocking v1.1)

## Session Continuity

**Last command:** /gsd:execute-plan .planning/phases/08-cleanup-helm-update/08-01-PLAN.md
**Last output:** Plan 08-01 complete - Dead code cleanup finished
**Context preserved:** Deleted 14,676 lines (74 files), CLI cleaned to server+debug commands only

**On next session:**
- Phase 8 IN PROGRESS — Plan 08-01 complete (dead code cleanup)
- Deleted commands: mcp, agent, mock
- Deleted package: internal/agent/ (entire package with 70 files)
- Removed tech debt: standalone MCP/agent commands and build-disabled agent package
- CLI surface: only `spectre server` and `spectre debug` commands
- Next: Execute 08-02-PLAN.md for Helm chart updates

---
*Last updated: 2026-01-21 — Completed 08-01-PLAN.md execution*
