---
phase: 26-observatory-api-mcp-tools
plan: 04
subsystem: api
tags: [mcp, grafana, observatory, orient, tools, anomaly-detection]

# Dependency graph
requires:
  - phase: 26-01
    provides: ObservatoryService with GetClusterAnomalies, AnomalyAggregator
provides:
  - ObservatoryStatusTool with Execute method for cluster-wide anomaly summary
  - ObservatoryChangesTool with Execute method for recent K8s changes
  - 10 unit tests for Orient stage tools
affects: [26-06, 26-07, 26-08]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "MCP tool pattern: struct with Execute(ctx, args) (interface{}, error)"
    - "Graph query for ChangeEvent nodes with deployment-related filters"

key-files:
  created:
    - internal/integration/grafana/tools_observatory_status.go
    - internal/integration/grafana/tools_observatory_changes.go
    - internal/integration/grafana/tools_observatory_orient_test.go
  modified: []

key-decisions:
  - "Query ChangeEvent nodes linked to ResourceIdentity for deployment changes"
  - "Filter by configChanged=true OR eventType=CREATE for meaningful changes"
  - "Include ReplicaSet in change-related kinds for deployment rollouts"
  - "Lookback default 1h, max 24h, max 20 changes returned"

patterns-established:
  - "Orient tools delegate to ObservatoryService for anomaly data"
  - "Empty results return empty arrays, not error or 'healthy' message"

# Metrics
duration: 7min
completed: 2026-01-30
---

# Phase 26 Plan 04: Orient Stage Tools Summary

**Two MCP tools for cluster-wide situation awareness: observatory_status returns top 5 anomaly hotspots, observatory_changes returns recent K8s deployment/config changes from graph**

## Performance

- **Duration:** 7 min
- **Started:** 2026-01-30T00:26:44Z
- **Completed:** 2026-01-30T00:33:32Z
- **Tasks:** 3
- **Files modified:** 3 created

## Accomplishments
- ObservatoryStatusTool provides cluster-wide anomaly summary via ObservatoryService
- ObservatoryChangesTool queries K8s graph for recent deployment/config changes
- 10 unit tests covering success, empty results, filtering, lookback parsing

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement observatory_status tool** - `505dedc` (feat)
2. **Task 2: Implement observatory_changes tool** - `de5f3a1` (feat)
3. **Task 3: Add unit tests for Orient tools** - `184e6d4` (test)

## Files Created/Modified
- `internal/integration/grafana/tools_observatory_status.go` - ObservatoryStatusTool delegating to ObservatoryService.GetClusterAnomalies
- `internal/integration/grafana/tools_observatory_changes.go` - ObservatoryChangesTool querying K8s graph for ChangeEvent nodes
- `internal/integration/grafana/tools_observatory_orient_test.go` - 10 unit tests for both tools

## Decisions Made
- **Query ChangeEvent via ResourceIdentity:** Instead of querying hypothetical Event nodes, use existing ChangeEvent nodes linked from ResourceIdentity via CHANGED relationship
- **Deployment-related kinds filter:** Deployment, HelmRelease, Kustomization, ConfigMap, Secret, StatefulSet, DaemonSet, ReplicaSet
- **configChanged OR CREATE filter:** Only show meaningful changes, not status-only updates
- **Response structure alignment:** Both tools return timestamp in RFC3339, changes/hotspots as arrays

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed undefined strings.Contains in tools_observatory_signal_detail.go**
- **Found during:** Task 3 (running tests revealed build failure)
- **Issue:** Previous plan's file used `contains()` instead of `strings.Contains()`
- **Fix:** Changed `contains(errStr, ...)` to `strings.Contains(errStr, ...)`
- **Files modified:** internal/integration/grafana/tools_observatory_signal_detail.go
- **Verification:** Build and tests pass
- **Committed in:** Not committed (file is untracked from prior plan - will be committed with that plan's completion)

---

**Total deviations:** 1 auto-fixed (1 bug in sibling file)
**Impact on plan:** Bug fix was necessary for tests to run. No scope creep.

## Issues Encountered
None - plan executed as written after fixing build issue in sibling file.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Orient stage tools complete (observatory_status, observatory_changes)
- Ready for Narrow stage tools (26-05: workloads, dashboards)
- Ready for Investigate stage tools (26-06: signal_detail)
- Untracked files from prior plans should be committed (tools_observatory_signal_detail.go, tools_observatory_compare.go)

---
*Phase: 26-observatory-api-mcp-tools*
*Completed: 2026-01-30*
