---
phase: 07-service-layer-extraction
plan: 01
subsystem: api
tags: [go, service-layer, timeline, mcp-tools, architecture]

# Dependency graph
requires:
  - phase: 06-consolidated-server
    provides: Single-port server with MCP endpoint and TimelineService foundation
provides:
  - Shared TimelineService used by both REST handlers and MCP tools
  - Direct service access for MCP tools eliminating HTTP self-calls
  - Service injection pattern for API server and MCP server
affects: [07-02, 07-03, 07-04, 07-05]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Service layer shared between REST and MCP via constructor injection"
    - "API server creates services, exposes via getter methods"
    - "MCP server accepts services in ServerOptions for tool initialization"

key-files:
  created: []
  modified:
    - internal/api/timeline_service.go
    - internal/api/handlers/register.go
    - internal/apiserver/server.go
    - internal/apiserver/routes.go
    - internal/mcp/server.go
    - internal/mcp/tools/resource_timeline.go
    - internal/mcp/tools/cluster_health.go
    - cmd/spectre/commands/server.go
    - internal/agent/tools/registry.go

key-decisions:
  - "Create API server before MCP server to access TimelineService"
  - "Add RegisterMCPEndpoint method for late MCP endpoint registration"
  - "Add WithClient constructors for backward compatibility with agent tools"

patterns-established:
  - "Service layer pattern: API server creates and owns services"
  - "Service sharing: Expose services via getter methods for external use"
  - "Tool dual-mode: Support both service injection and HTTP client fallback"

# Metrics
duration: 9min
completed: 2026-01-21
---

# Phase 07 Plan 01: Timeline Service Layer Extraction Summary

**MCP timeline tools now call shared TimelineService directly, eliminating HTTP overhead; REST handlers and MCP tools share same service instance**

## Performance

- **Duration:** 9 min
- **Started:** 2026-01-21T19:11:10Z
- **Completed:** 2026-01-21T19:19:51Z
- **Tasks:** 3 (1 skipped - work already complete)
- **Files modified:** 9

## Accomplishments
- TimelineService fully extracted with all REST handler business logic (found already complete from Phase 6)
- REST timeline handler already using TimelineService (found already complete from Phase 6)
- MCP tools refactored to use TimelineService directly via constructor injection
- Server initialization reordered to create API server first, enabling service sharing
- HTTP self-calls eliminated for timeline operations in MCP tools

## Task Commits

Each task was committed atomically:

1. **Task 1: Complete TimelineService** - Work already complete (no commit needed)
2. **Task 2: Refactor REST timeline handler** - Work already complete (no commit needed)
3. **Task 3: Wire MCP tools to use TimelineService** - `ad16758` (feat)

**Plan metadata:** (will be included in final metadata commit)

_Note: Tasks 1 and 2 were discovered to be already complete from Phase 6 work_

## Files Created/Modified
- `internal/apiserver/server.go` - Added timelineService field, creates service in constructor, added GetTimelineService() getter, added RegisterMCPEndpoint() for late registration
- `internal/apiserver/routes.go` - Pass timelineService to RegisterHandlers
- `internal/api/handlers/register.go` - Accept timelineService parameter instead of creating new instance
- `internal/mcp/server.go` - Added TimelineService to ServerOptions, store in SpectreServer, pass to timeline tools, added conditional tool creation (service vs client)
- `internal/mcp/tools/resource_timeline.go` - Accept TimelineService in primary constructor, added WithClient fallback constructor, Execute method already using service
- `internal/mcp/tools/cluster_health.go` - Accept TimelineService in primary constructor, added WithClient fallback constructor, refactored Execute to use service directly
- `cmd/spectre/commands/server.go` - Reordered initialization: create API server first, get TimelineService, create MCP server with service, register MCP endpoint late
- `internal/agent/tools/registry.go` - Updated to use WithClient constructors for backward compatibility

## Decisions Made

**1. Reorder server initialization**
- **Rationale:** TimelineService is created by API server, so API server must be created before MCP server to access it
- **Approach:** Create API server with nil MCP server, then create MCP server with TimelineService, then register MCP endpoint
- **Impact:** Enables direct service sharing without circular dependencies

**2. Add RegisterMCPEndpoint method**
- **Rationale:** MCP endpoint registration must happen after MCP server creation, but API server constructor previously required MCP server
- **Approach:** Add RegisterMCPEndpoint(mcpServer) method to apiserver for late registration
- **Impact:** Clean separation of API server construction and MCP endpoint registration

**3. WithClient constructors for backward compatibility**
- **Rationale:** Agent tools registry still uses HTTP client pattern
- **Approach:** Add NewClusterHealthToolWithClient and NewResourceTimelineToolWithClient constructors
- **Impact:** Both patterns supported during transition, agent tools continue working

**4. Move integration manager initialization**
- **Rationale:** Integration manager requires MCP registry, which requires MCP server
- **Approach:** Initialize integration manager after MCP server creation instead of before
- **Impact:** Integration tools can register with MCP server properly

## Deviations from Plan

**1. Tasks 1 and 2 already complete**
- **Found during:** Plan execution start
- **Issue:** TimelineService was already fully extracted with ParseQueryParameters, ParsePagination, ExecuteConcurrentQueries, and BuildTimelineResponse methods. REST timeline handler was already using TimelineService.
- **Root cause:** Phase 6 work included more service extraction than documented in Phase 6 plans
- **Action taken:** Verified existing implementation matches requirements, proceeded directly to Task 3
- **Impact:** Saved development time, no code changes needed for Tasks 1-2
- **Documentation:** Tasks 1-2 marked as "work already complete" in summary

---

**Total deviations:** 1 (discovered work already complete)
**Impact on plan:** Positive - work already done correctly, proceeded directly to MCP tool wiring

## Issues Encountered

**1. Circular dependency in server initialization**
- **Problem:** API server constructor required MCP server, but MCP server needed TimelineService from API server
- **Solution:** Refactored initialization order - create API server first with nil MCP server, then create MCP server with TimelineService, then register MCP endpoint via new RegisterMCPEndpoint method
- **Verification:** Server compiles and initializes properly with new order

**2. Integration manager requires MCP registry**
- **Problem:** Integration manager initialization moved too early (before MCP server), causing undefined err variable
- **Solution:** Moved integration manager initialization to after MCP server creation
- **Verification:** Server compiles without errors

**3. Agent tools registry compatibility**
- **Problem:** Agent tools registry expected tools to accept HTTP client, but refactored tools now expect TimelineService
- **Solution:** Added WithClient constructors for backward compatibility
- **Verification:** Agent tools compile and use client-based tools properly

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

**Ready for Phase 7 Plan 2:**
- TimelineService pattern established and working
- MCP tools successfully refactored to use service layer
- Server initialization order supports service sharing
- Pattern ready to replicate for GraphService, SearchService, MetadataService

**No blockers**

---
*Phase: 07-service-layer-extraction*
*Completed: 2026-01-21*
