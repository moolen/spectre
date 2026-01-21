---
phase: 05-progressive-disclosure-mcp-tools
plan: 01
subsystem: integration
tags: [mcp, tools, registry, lifecycle]

# Dependency graph
requires:
  - phase: 01-plugin-infrastructure-foundation
    provides: Integration interface with RegisterTools placeholder
  - phase: 03-victorialogs-client-pipeline
    provides: VictoriaLogs client and pipeline ready for tool integration
provides:
  - MCPToolRegistry adapter bridging integration.ToolRegistry to mcp-go server
  - Manager lifecycle integration calling RegisterTools() after instance startup
  - VictoriaLogs integration storing registry reference for Plans 2-4
affects: [05-02, 05-03, 05-04]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Tool registration via adapter pattern"
    - "RegisterTools() called after Start() regardless of health status"
    - "Registry stored in integration for deferred tool implementation"

key-files:
  created: []
  modified:
    - internal/mcp/server.go
    - internal/integration/manager.go
    - internal/integration/victorialogs/victorialogs.go

key-decisions:
  - "MCPToolRegistry uses generic JSON schema, delegating validation to integration handlers"
  - "RegisterTools() called for all instances including degraded ones (tools can return service unavailable)"
  - "NewManagerWithMCPRegistry constructor for MCP-enabled servers, preserving backward compatibility"
  - "Tool registration errors logged but don't fail startup (resilience pattern)"

patterns-established:
  - "Tool naming convention: {integration_type}_{instance_name}_{tool}"
  - "Adapter pattern: integration.ToolHandler -> server.ToolHandlerFunc"
  - "Registry stored in integration struct for deferred tool implementations"

# Metrics
duration: 2min
completed: 2026-01-21
---

# Phase 5 Plan 1: MCP Tool Registration Infrastructure Summary

**MCPToolRegistry adapter enables dynamic tool registration with lifecycle integration and backward-compatible Manager constructor**

## Performance

- **Duration:** 2 min
- **Started:** 2026-01-21T15:26:58Z
- **Completed:** 2026-01-21T15:29:02Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments

- Created MCPToolRegistry adapter implementing integration.ToolRegistry interface
- Wired RegisterTools() into Manager lifecycle after instance startup
- VictoriaLogs integration stores registry reference for Plans 2-4 tool implementations
- Adapter converts integration.ToolHandler to mcp-go server.ToolHandlerFunc format
- Generic JSON schema allows integrations to provide their own argument validation

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement ToolRegistry adapter in MCP server** - `4470562` (feat)
2. **Task 2: Wire RegisterTools into integration lifecycle** - `1c5a63d` (feat)
3. **Task 3: Update VictoriaLogs integration to use registry** - `2a731d5` (feat)

## Files Created/Modified

- `internal/mcp/server.go` - Added MCPToolRegistry struct and NewMCPToolRegistry constructor
- `internal/integration/manager.go` - Added mcpRegistry field, NewManagerWithMCPRegistry constructor, RegisterTools() call in lifecycle
- `internal/integration/victorialogs/victorialogs.go` - Added registry field, store reference in RegisterTools()

## Decisions Made

**MCPToolRegistry uses generic JSON schema:**
- Rationale: Integration handlers validate their own arguments, keeping adapter simple and flexible
- Impact: Each tool implementation provides specific schema and validation in Plans 2-4

**RegisterTools() called for all instances including degraded ones:**
- Rationale: Degraded backends can still expose tools that return service unavailable errors
- Impact: AI assistants can discover available tools even when backends are temporarily down

**NewManagerWithMCPRegistry constructor added:**
- Rationale: Preserves backward compatibility for callers that don't need MCP integration
- Impact: Existing code continues to work unchanged, only MCP-enabled servers use new constructor

**Tool registration errors logged but don't fail startup:**
- Rationale: Resilience - one integration's tool registration failure shouldn't crash entire server
- Impact: Server continues with partial tool availability, logged for debugging

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - all tasks completed successfully.

**Note:** One pre-existing test failure (TestManagerConfigReload) is unrelated to these changes. The test is timing-dependent and was already failing before modifications. All other tests pass.

## Next Phase Readiness

Foundation complete for MCP tool implementations:
- Plans 2-4 can call `v.registry.RegisterTool()` to add tools
- Tool naming convention established: `victorialogs_{name}_{tool}`
- Adapter handles marshaling/unmarshaling between integration and mcp-go formats

Ready for Plan 2: Overview Tool implementation.

---
*Phase: 05-progressive-disclosure-mcp-tools*
*Completed: 2026-01-21*
