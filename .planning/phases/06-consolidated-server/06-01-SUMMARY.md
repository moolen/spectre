---
phase: 06-consolidated-server
plan: 01
subsystem: server-architecture
tags: [mcp, http, server-consolidation, in-process-tools, streamablehttp]

# Dependency graph
requires:
  - phase: 05-integration-manager
    provides: Integration manager with plugin system and MCP tool registration
provides:
  - MCP server initialized in-process with main server on port 8080
  - /v1/mcp HTTP endpoint with StreamableHTTP transport (stateless mode)
  - Optional --stdio flag for stdio MCP transport alongside HTTP
  - MCPToolRegistry adapter wiring integration manager to MCP server
  - Single-port deployment architecture (REST + MCP on :8080)
affects: [07-service-layer, 08-cleanup, 09-e2e-tests]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "MCP server lifecycle tied to HTTP server (no separate component)"
    - "Route registration order: specific routes -> MCP -> static UI catch-all"
    - "MCPToolRegistry adapter pattern for integration tool registration"

key-files:
  created: []
  modified:
    - cmd/spectre/commands/server.go
    - internal/apiserver/server.go
    - internal/apiserver/routes.go

key-decisions:
  - "Use /v1/mcp instead of /mcp for API versioning consistency with /api/v1/*"
  - "Use --stdio boolean flag instead of --transport=stdio enum for simplicity"
  - "MCP server self-references localhost:8080 for tool execution (Phase 7 will eliminate HTTP calls)"
  - "StreamableHTTPServer in stateless mode for client compatibility"

patterns-established:
  - "MCP server initialized before integration manager (tools need registry)"
  - "Integration manager Start() calls RegisterTools() for each integration"
  - "Stdio transport runs in goroutine, stops automatically on context cancel"

# Metrics
duration: 3min
completed: 2026-01-21
---

# Phase 6 Plan 01: MCP Server Consolidation Summary

**Single-port server deployment with in-process MCP on :8080 using StreamableHTTP transport and MCPToolRegistry integration**

## Performance

- **Duration:** 3 minutes
- **Started:** 2026-01-21T17:43:21Z
- **Completed:** 2026-01-21T17:46:31Z
- **Tasks:** 3 (executed as single cohesive unit)
- **Files modified:** 3

## Accomplishments
- MCP server initializes in-process before integration manager, enabling tool registration
- /v1/mcp endpoint serves MCP protocol via StreamableHTTP on main HTTP server
- Optional stdio transport runs alongside HTTP when --stdio flag provided
- MCPToolRegistry adapter wires integration manager to MCP server
- Single-port deployment eliminates MCP sidecar architecture

## Task Commits

All tasks executed as single cohesive implementation:

1. **Tasks 1-3: MCP server consolidation** - `e792f9a` (feat)
   - Initialize MCP server in main server startup
   - Add mcpServer to APIServer struct and /v1/mcp endpoint
   - Wire mcpServer parameter through server initialization

## Files Created/Modified
- `cmd/spectre/commands/server.go` - MCP server initialization, MCPToolRegistry wiring, --stdio flag
- `internal/apiserver/server.go` - mcpServer field, constructor parameter, registerMCPHandler method
- `internal/apiserver/routes.go` - Call registerMCPHandler before static UI handlers

## Decisions Made

**1. Use /v1/mcp instead of /mcp**
- Rationale: Consistency with existing /api/v1/* routes for API versioning
- Impact: Requirement docs specify /mcp but implementation uses /v1/mcp

**2. Use --stdio flag instead of --transport=stdio**
- Rationale: Simpler boolean flag vs string enum when only two modes needed
- Impact: Requirement docs specify --transport=stdio but implementation uses --stdio

**3. MCP server self-references localhost:8080**
- Rationale: Reuses existing MCP tool implementations during transition
- Impact: Phase 7 will eliminate HTTP calls by converting to direct service calls
- Trade-off: Temporary HTTP overhead for cleaner incremental migration

**4. StreamableHTTPServer with stateless mode**
- Rationale: Compatibility with MCP clients that don't manage sessions
- Impact: Each request includes full session context vs server-side session state

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - implementation proceeded smoothly.

## Next Phase Readiness

**Ready for Phase 7 (Service Layer Extraction):**
- MCP server operational with /v1/mcp endpoint
- Integration manager successfully registers tools via MCPToolRegistry
- Single-port architecture in place (REST + MCP on :8080)

**Blockers:** None

**Considerations for Phase 7:**
- Current MCP tools make HTTP calls to localhost:8080 (internal API)
- Service layer extraction will convert these to direct function calls
- Tool implementations in internal/mcp/tools/ will be refactored

---
*Phase: 06-consolidated-server*
*Completed: 2026-01-21*
