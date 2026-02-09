---
phase: 26-observatory-api-mcp-tools
plan: 07
subsystem: api
tags: [grafana, mcp, observatory, explain, evidence, root-cause-analysis, verify-stage]

# Dependency graph
requires:
  - phase: 26-03
    provides: ObservatoryEvidenceService with GetCandidateCauses and GetSignalEvidence
provides:
  - ObservatoryExplainTool for Hypothesize stage (root cause candidates)
  - ObservatoryEvidenceTool for Verify stage (raw metric values, alerts, logs)
  - Unit tests for both tools
affects: [26-mcp-tool-registration]

# Tech tracking
tech-stack:
  added: []
  patterns: [tool-service-composition, graceful-degradation, parameter-validation]

key-files:
  created:
    - internal/integration/grafana/tools_observatory_explain.go
    - internal/integration/grafana/tools_observatory_evidence.go
    - internal/integration/grafana/tools_observatory_verify_test.go
  modified:
    - internal/integration/grafana/live_state_test.go

key-decisions:
  - "Explain returns upstream deps and recent changes for AI interpretation"
  - "Evidence includes lookback parameter with 1h default"
  - "Both tools return raw data, no summaries or categorical labels"
  - "LogExcerpts gracefully empty when log integration not configured"

patterns-established:
  - "Tool-Service composition: tool wraps service method, adds validation"
  - "Required parameter validation with descriptive error messages"
  - "Lookback duration parsing with helpful format guidance"

# Metrics
duration: 8min
completed: 2026-01-30
---

# Phase 26 Plan 07: Hypothesize and Verify Stage Tools Summary

**observatory_explain tool returns K8s graph candidates (upstream deps, recent changes); observatory_evidence tool returns raw metrics, alerts, and logs for verification**

## Performance

- **Duration:** 8 min
- **Started:** 2026-01-30T00:26:40Z
- **Completed:** 2026-01-30T00:34:49Z
- **Tasks:** 3
- **Files created:** 3
- **Files modified:** 1 (bug fix)

## Accomplishments
- ObservatoryExplainTool wrapping ObservatoryEvidenceService.GetCandidateCauses
- ObservatoryEvidenceTool wrapping ObservatoryEvidenceService.GetSignalEvidence
- Input validation for required parameters (namespace, workload, metric_name)
- Lookback parsing with 1h default and helpful error messages
- Full unit test coverage (9 test cases)

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement observatory_explain tool** - `b16248a` (feat)
2. **Task 2: Implement observatory_evidence tool** - `0923435` (feat)
3. **Task 3: Add unit tests for Hypothesize/Verify tools** - `0f63ed0` (test)

## Files Created/Modified

- `internal/integration/grafana/tools_observatory_explain.go` (94 lines) - ObservatoryExplainTool with Execute method
- `internal/integration/grafana/tools_observatory_evidence.go` (120 lines) - ObservatoryEvidenceTool with Execute method
- `internal/integration/grafana/tools_observatory_verify_test.go` (633 lines) - 9 test cases for both tools
- `internal/integration/grafana/live_state_test.go` (modified) - Fix function name collision

## Decisions Made

1. **Raw data response pattern** - Both tools return raw data for AI interpretation, not summaries or categorical labels
2. **Default lookback of 1h** - Evidence tool uses 1 hour lookback when not specified, with duration parsing support
3. **Graceful log degradation** - LogExcerpts field is empty array when log integration not configured
4. **Service composition pattern** - Tools wrap service methods and add parameter validation

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Function name collision in live_state_test.go**
- **Found during:** Task 3 (test execution)
- **Issue:** `contains` function in `live_state_test.go` conflicted with same name in `tools_observatory_signal_detail.go`
- **Fix:** Renamed to `liveStateContains` and `liveStateContainsHelper`
- **Files modified:** internal/integration/grafana/live_state_test.go
- **Committed in:** 0f63ed0 (Task 3 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Minimal - pre-existing naming collision unrelated to plan scope.

## Issues Encountered
None - plan executed as specified after fixing pre-existing naming collision.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- observatory_explain and observatory_evidence tools ready for MCP registration
- Both tools follow established patterns from Wave 1
- Service composition pattern validated for remaining tools

---
*Phase: 26-observatory-api-mcp-tools*
*Completed: 2026-01-30*
