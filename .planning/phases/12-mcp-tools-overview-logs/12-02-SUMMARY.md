---
phase: 12-mcp-tools-overview-logs
plan: 02
subsystem: mcp
tags: [logzio, mcp, elasticsearch, aggregations, tools]

# Dependency graph
requires:
  - phase: 12-01
    provides: Logzio integration bootstrap with Elasticsearch DSL builder and HTTP client
provides:
  - Two MCP tools for Logzio progressive disclosure (overview → logs)
  - Overview tool with parallel aggregations for namespace severity breakdown
  - Logs tool with filtering and 100-log limit enforcement
  - Tool registration via MCP protocol following victorialogs pattern
affects: [13-patterns, logzio-integration-tests, mcp-client-usage]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Parallel aggregation queries for reduced latency (3 goroutines with channel collection)"
    - "Truncation detection via Limit+1 fetch pattern"
    - "Tool naming convention: {backend}_{instance}_{tool}"
    - "ValidateQueryParams protects internal severity regex patterns only"

key-files:
  created:
    - internal/integration/logzio/tools_overview.go
    - internal/integration/logzio/tools_logs.go
  modified:
    - internal/integration/logzio/logzio.go

key-decisions:
  - "Logs tool max 100 entries (not 500 like VictoriaLogs) per CONTEXT.md"
  - "ValidateQueryParams only validates internal severity regex, not user parameters"
  - "Logs tool schema does NOT expose regex parameter - only structured filters"
  - "Overview tool validates severity patterns to prevent leading wildcard performance issues"

patterns-established:
  - "ToolContext struct for dependency injection (Client, Logger, Instance)"
  - "TimeRangeParams embedded in tool params with parseTimeRange helper"
  - "Namespace severity breakdown with Errors, Warnings, Other, Total"
  - "Parallel query pattern from VictoriaLogs for reduced latency"

# Metrics
duration: 3min
completed: 2026-01-22
---

# Phase 12 Plan 02: MCP Tools - Overview and Logs Summary

**Logzio MCP tools (overview + logs) with parallel aggregations, 100-log limit, and structured filtering only**

## Performance

- **Duration:** 3 min 19 sec
- **Started:** 2026-01-22T14:48:20Z
- **Completed:** 2026-01-22T14:51:39Z
- **Tasks:** 3
- **Files modified:** 3 (2 created, 1 modified)

## Accomplishments
- Overview tool returns namespace severity breakdown (errors, warnings, other) with parallel aggregation queries
- Logs tool returns up to 100 filtered log entries with namespace required
- Tool schemas registered with MCP protocol following victorialogs_{name}_{tool} naming pattern
- ValidateQueryParams protects overview tool's internal severity regex patterns from leading wildcard performance issues

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement overview tool with parallel severity aggregations** - `972c258` (feat)
   - OverviewTool with parallel execution of 3 aggregation queries (total, errors, warnings)
   - NamespaceSeverity response with Errors, Warnings, Other, Total
   - parseTimeRange helper with Unix seconds/milliseconds detection
   - ValidateQueryParams checks internal severity regex patterns

2. **Task 2: Implement logs tool with filtering and limits** - `f36613b` (feat)
   - LogsTool with namespace required validation
   - MaxLimit = 100, DefaultLimit = 100 per CONTEXT.md
   - Truncation detection via Limit+1 fetch pattern
   - NO wildcard validation needed (only structured filters exposed)

3. **Task 3: Wire tools into RegisterTools and update Health check** - `e3196fb` (feat)
   - RegisterTools with 2 tool registrations (overview, logs)
   - Tool schemas with parameter descriptions
   - Health() check reflects SecretWatcher status
   - Tool naming: logzio_{name}_overview, logzio_{name}_logs

## Files Created/Modified

### Created
- **internal/integration/logzio/tools_overview.go** (246 lines)
  - OverviewTool with parallel aggregation queries
  - ToolContext, TimeRangeParams, OverviewParams, OverviewResponse
  - NamespaceSeverity struct with Errors, Warnings, Other, Total
  - parseTimeRange and parseTimestamp helpers

- **internal/integration/logzio/tools_logs.go** (95 lines)
  - LogsTool with namespace required validation
  - LogsParams with structured filters (namespace, pod, container, level)
  - LogsResponse with truncation flag
  - MaxLimit = 100 enforcement

### Modified
- **internal/integration/logzio/logzio.go**
  - RegisterTools implementation (84 lines added)
  - Overview tool schema with start_time, end_time, namespace (all optional)
  - Logs tool schema with namespace required, other filters optional
  - Health() check updated (removed TODO comment)

## Decisions Made

**1. Logs tool limit: 100 max (not 500)**
- **Rationale:** Per CONTEXT.md decision for more conservative limit than VictoriaLogs
- **Impact:** Prevents AI assistant context overflow, encourages narrow filtering

**2. ValidateQueryParams scope: internal patterns only**
- **Rationale:** Overview tool uses internal severity regex patterns (GetErrorPattern, GetWarningPattern) which could have leading wildcards. Validation protects against performance issues.
- **Impact:** Logs tool does NOT need validation - it only exposes structured filters (namespace, pod, container, level), not raw regex queries to users.

**3. Logs tool schema: no regex parameter**
- **Rationale:** Per CONTEXT.md and plan, logs tool exposes only structured filters. Users cannot provide raw regex patterns.
- **Impact:** No leading wildcard exposure risk from user input. ValidateQueryParams protects internal severity detection patterns only.

**4. Parallel aggregation queries**
- **Rationale:** Copied VictoriaLogs pattern - reduces latency from ~16s sequential to ~10s parallel
- **Impact:** Better UX for AI assistants, faster overview responses

## Deviations from Plan

None - plan executed exactly as written.

All implementation matched plan specifications:
- Overview tool with 3 parallel queries (total, errors, warnings)
- Logs tool with namespace required, 100-log limit
- ValidateQueryParams called only for internal severity patterns
- Tool schemas match VictoriaLogs structure
- No regex parameter exposed in logs tool schema

## Issues Encountered

None - implementation proceeded smoothly. All code compiled on first attempt.

## User Setup Required

None - no external service configuration required.

Tools are automatically registered when Logzio integration is configured. See Phase 11 (Secret File Management) for Kubernetes Secret setup if using apiTokenRef.

## Validation Scope Clarification

**Important architectural decision documented:**

The plan specifies ValidateQueryParams validates "internal regex patterns" and that "logs tool does NOT expose regex parameter to users."

This means:
- **Overview tool:** Calls ValidateQueryParams to check GetErrorPattern() and GetWarningPattern() for leading wildcards (performance protection)
- **Logs tool:** Does NOT call ValidateQueryParams because it only exposes structured filters (namespace, pod, container, level) to users, not raw regex queries

This distinction protects against:
1. Performance issues from internal severity detection patterns (overview tool)
2. Does NOT create false sense of security - users cannot provide regex to logs tool, so no validation needed there

## Next Phase Readiness

**Ready for Phase 13 (Patterns tool):**
- Overview and logs tools provide progressive disclosure foundation
- Pattern mining can build on overview tool's namespace aggregations
- Logzio integration fully operational with 2 MCP tools registered

**Template limits deferred:**
Per plan scope note, template limits (max 50) are out of scope for Phase 12. They will be addressed in Phase 13 when pattern mining tool is implemented.

**No blockers.**

---
*Phase: 12-mcp-tools-overview-logs*
*Completed: 2026-01-22*
