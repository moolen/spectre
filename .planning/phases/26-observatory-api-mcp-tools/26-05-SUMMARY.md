---
phase: 26-observatory-api-mcp-tools
plan: 05
subsystem: mcp-tools
tags: [grafana, observatory, mcp, narrow-stage, anomaly-detection]

# Dependency graph
requires:
  - phase: 26-01
    provides: ObservatoryService with GetNamespaceAnomalies, GetWorkloadAnomalyDetail
  - phase: 26-02
    provides: ObservatoryInvestigateService with GetWorkloadSignals
provides:
  - ObservatoryScopeTool with namespace/workload scope filtering
  - ObservatorySignalsTool with workload signal enumeration
  - Unit tests for both Narrow stage tools
affects: [26-06, 26-07, 26-08, MCP registration]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - MCP tool composition with service layer
    - Flat list responses sorted by anomaly score descending
    - RFC3339 timestamps in all responses

key-files:
  created:
    - internal/integration/grafana/tools_observatory_scope.go
    - internal/integration/grafana/tools_observatory_signals.go
    - internal/integration/grafana/tools_observatory_narrow_test.go
  modified:
    - internal/integration/grafana/observatory_investigate_service.go

key-decisions:
  - "SignalSummary includes QualityScore for tool response completeness"
  - "Empty Workload field at signal level, populated at namespace level"
  - "Role field empty at namespace level (aggregation doesn't preserve role)"

patterns-established:
  - "Narrow tool pattern: Service composition with flat list response"
  - "Scope format: 'namespace' for namespace-level, 'namespace/workload' for workload-level"

# Metrics
duration: 4min
completed: 2026-01-30
---

# Phase 26 Plan 05: Narrow Stage MCP Tools Summary

**ObservatoryScopeTool and ObservatorySignalsTool for namespace/workload scoped anomaly investigation with 9 passing tests**

## Performance

- **Duration:** 4 min
- **Started:** 2026-01-30T00:26:24Z
- **Completed:** 2026-01-30T00:30:30Z
- **Tasks:** 3
- **Files created:** 3
- **Files modified:** 1

## Accomplishments

- Created ObservatoryScopeTool for namespace/workload anomaly scoping
- Created ObservatorySignalsTool for workload signal enumeration
- Added QualityScore to SignalSummary for complete tool response
- 9 test cases covering all required scenarios

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement observatory_scope tool** - `973d34f` (feat)
2. **Task 2: Implement observatory_signals tool** - `f2f5b12` (feat)
3. **Task 3: Add unit tests for Narrow tools** - `3d994ab` (test)

## Files Created/Modified

- `internal/integration/grafana/tools_observatory_scope.go` (122 lines) - Narrow stage scope tool
  - ObservatoryScopeTool struct with service composition
  - Execute method routing to GetNamespaceAnomalies or GetWorkloadAnomalyDetail
  - ScopedAnomaly response type with workload/metric/role/score/confidence

- `internal/integration/grafana/tools_observatory_signals.go` (99 lines) - Narrow stage signals tool
  - ObservatorySignalsTool struct with investigate service composition
  - Execute method calling GetWorkloadSignals
  - SignalState response type with quality_score included

- `internal/integration/grafana/tools_observatory_narrow_test.go` (430 lines) - Unit tests
  - 4 tests for ObservatoryScopeTool (namespace, workload, empty, missing params)
  - 5 tests for ObservatorySignalsTool (success, sorted, empty, missing params, timestamp)
  - Mock graph client with comprehensive query matching

- `internal/integration/grafana/observatory_investigate_service.go` (modified) - Added QualityScore to SignalSummary

## Decisions Made

1. **QualityScore in SignalSummary**: Added to SignalSummary type since the investigate service already queries it but wasn't exposing it. Tool response requires quality_score per plan specification.

2. **Empty Workload at signal level**: When scope is workload-level, the Workload field is omitted from ScopedAnomaly since it would be redundant.

3. **Empty Role at namespace level**: At namespace aggregation level, role information is not preserved (aggregated across all signals), so Role field is empty string.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added QualityScore to SignalSummary**
- **Found during:** Task 2 (ObservatorySignalsTool implementation)
- **Issue:** SignalSummary type didn't include quality_score, but tool response requires it
- **Fix:** Added QualityScore field to SignalSummary, updated GetWorkloadSignals to populate it
- **Files modified:** observatory_investigate_service.go
- **Verification:** Tool returns quality_score in response, tests pass
- **Committed in:** f2f5b12 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (missing critical functionality)
**Impact on plan:** Essential for complete API response. No scope creep.

## Issues Encountered

None - plan executed smoothly.

## User Setup Required

None - no external service configuration required.

## Key Links Verified

| From | To | Via | Pattern |
|------|-----|-----|---------|
| tools_observatory_scope.go | observatory_service.go | Service composition | `service.GetNamespaceAnomalies`, `service.GetWorkloadAnomalyDetail` |
| tools_observatory_signals.go | observatory_investigate_service.go | Service composition | `investigateService.GetWorkloadSignals` |

## Next Phase Readiness

- Narrow stage tools complete and tested
- Ready for Investigate stage tools (26-06: signal_detail, compare)
- Ready for Verify stage tools (26-07: changes, evidence)
- Ready for Hypothesize stage tools (26-08: explain)

**No blockers or concerns.**

---
*Phase: 26-observatory-api-mcp-tools*
*Completed: 2026-01-30*
