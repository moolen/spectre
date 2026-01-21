# GSD State: Spectre Server Consolidation

## Project Reference

See: .planning/PROJECT.md (updated 2026-01-21)

**Core value:** Enable AI assistants to understand Kubernetes clusters through unified MCP interface
**Current focus:** v1.1 Server Consolidation — single-port deployment with in-process MCP

## Current Position

Phase: Phase 9 — E2E Test Validation (4 of 4) — IN PROGRESS
Plan: 09-01 complete (1 of 3 plans in phase)
Status: In progress
Last activity: 2026-01-21 — Completed 09-01-PLAN.md

Progress: ███████████░░░░░░░░░ 55% (11/20 total plans estimated)

## Milestone: v1.1 Server Consolidation

**Goal:** Single server binary serving REST API, UI, and MCP on one port (:8080)

**Phases:**
- Phase 6: Consolidated Server & Integration Manager (7 reqs) — COMPLETE (2/2 plans complete)
- Phase 7: Service Layer Extraction (5 reqs) — COMPLETE (5/5 plans complete)
- Phase 8: Cleanup & Helm Chart Update (5 reqs) — COMPLETE (3/3 plans complete)
- Phase 9: E2E Test Validation (4 reqs) — IN PROGRESS (1/3 plans complete)

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

1. Execute plan 09-02 (Run MCP HTTP tests)
2. Execute plan 09-03 (Run MCP failure scenario tests)
3. Verify v1.1 milestone complete

## Performance Metrics

**v1.1 Milestone:**
- Phases complete: 3/4 (Phase 6 ✅, Phase 7 ✅, Phase 8 ✅, Phase 9 in progress)
- Plans complete: 11/20 (estimated)
- Requirements satisfied: 18/21 (SRVR-01 through TEST-01)

**Session metrics:**
- Current session: 2026-01-21
- Plans executed this session: 11
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
| 08-03 | README MCP Integration section describes in-process architecture | Documentation must match actual Phase 6 implementation | Users understand MCP runs integrated on port 8080 at /v1/mcp |
| 08-03 | chart/README.md does not exist | Helm charts often document via values.yaml comments instead | No Helm chart README to update, values.yaml provides documentation |
| 08-02 | Remove MCP sidecar completely from Helm chart | After Phase 6, MCP runs in-process on port 8080 | Simplified deployment, lower resource usage, single-container architecture |
| 08-02 | Port consolidation: all HTTP traffic on port 8080 | Aligns with Phase 6 consolidated server | Simpler service definition, ingress routing, and firewall rules |
| 08-02 | Update test fixtures immediately | E2E tests in Phase 9 need correct architecture | Test fixtures ready, no follow-up work needed |
| 09-01 | E2E tests use /v1/mcp endpoint instead of /mcp | Aligns with Phase 6 decision for API versioning consistency | Test client sends requests to correct endpoint matching server implementation |
| 09-01 | E2E tests connect to port 8080 instead of 8082 | MCP now integrated on main server port after Phase 6-8 | Test infrastructure matches production consolidated architecture |
| 09-01 | Remove MCP Helm values from test deployment | MCP integrated by default, no separate config needed | Simplified test deployment configuration |

### Active TODOs

*Updated as work progresses*

### Deferred Issues

- DateAdded persistence (v1 debt, not blocking v1.1)
- GET /{name} endpoint usage (v1 debt, not blocking v1.1)

## Session Continuity

**Last command:** /gsd:execute-plan 09-01
**Last output:** Plan 09-01 complete — E2E test configuration updated
**Context preserved:** Test endpoints updated to /v1/mcp, ports updated to 8080, MCP Helm config removed

**On next session:**
- Phase 9 IN PROGRESS — Plan 09-01 complete (1/3)
- E2E tests configured for consolidated MCP architecture
- TEST-01 requirement satisfied ✓
- Tests connect to port 8080 at /v1/mcp endpoint
- Test deployment matches production architecture
- 18/21 v1.1 requirements satisfied (TEST-02 through TEST-04 remain)
- Next: Execute plan 09-02 (Run MCP HTTP tests)

---
*Last updated: 2026-01-21 — Completed 09-01-PLAN.md execution*
