---
phase: 26-observatory-api-mcp-tools
plan: 01
subsystem: api
tags: [grafana, anomaly-detection, observatory, mcp-tools, signal-classification]

# Dependency graph
requires:
  - phase: 25-baseline-anomaly-detection
    provides: AnomalyAggregator, SignalBaseline, anomaly scoring infrastructure
provides:
  - ObservatoryService with GetClusterAnomalies, GetNamespaceAnomalies, GetWorkloadAnomalyDetail, GetDashboardQuality
  - Response types for Orient/Narrow/Investigate stages
  - Internal 0.5 anomaly threshold constant
  - Unit tests for all service methods
affects: [26-02, 26-03, 26-04, 26-05, MCP tools]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Service layer composition with AnomalyAggregator
    - Threshold-based filtering for anomaly results
    - Hierarchical anomaly aggregation (signal -> workload -> namespace -> cluster)

key-files:
  created:
    - internal/integration/grafana/observatory_service.go
    - internal/integration/grafana/observatory_service_test.go
  modified: []

key-decisions:
  - "Internal anomaly threshold = 0.5 per CONTEXT.md"
  - "Top 5 hotspots for cluster-wide queries"
  - "Top 20 workloads for namespace queries"
  - "Top 20 dashboards for quality queries"
  - "Confidence as tiebreaker when scores are equal"

patterns-established:
  - "ObservatoryService pattern: Service layer composing AnomalyAggregator + graph queries"
  - "Threshold filtering: Filter results where Score >= anomalyThreshold (0.5)"
  - "Response types: Minimal facts only, numeric scores, RFC3339 timestamps"

# Metrics
duration: 9min
completed: 2026-01-30
---

# Phase 26 Plan 01: Observatory Service Core Summary

**ObservatoryService with hierarchical anomaly queries for cluster/namespace/workload scopes using AnomalyAggregator composition**

## Performance

- **Duration:** 9 min
- **Started:** 2026-01-30T00:11:50Z
- **Completed:** 2026-01-30T00:20:49Z
- **Tasks:** 3
- **Files created:** 2

## Accomplishments
- Created ObservatoryService with 4 core methods for MCP tool foundation
- Implemented hierarchical anomaly filtering with 0.5 threshold
- Added comprehensive unit tests (10 test cases) with race detector

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement ObservatoryService core** - `6c220d1` (feat)
2. **Task 2: Add unit tests for ObservatoryService** - `a2c7f5a` (test)
3. **Task 3: Implement GetDashboardQuality method** - (included in Tasks 1 & 2)

## Files Created

- `internal/integration/grafana/observatory_service.go` (561 lines)
  - ObservatoryService struct with graphClient, anomalyAgg, integrationName, logger
  - GetClusterAnomalies: Returns top 5 hotspots filtered by 0.5 threshold
  - GetNamespaceAnomalies: Returns top 20 workloads with anomaly details
  - GetWorkloadAnomalyDetail: Returns signal-level anomalies with roles
  - GetDashboardQuality: Returns top 20 dashboards ranked by quality score
  - Response types: ClusterAnomaliesResult, NamespaceAnomaliesResult, WorkloadAnomalyDetailResult, DashboardQualityResult

- `internal/integration/grafana/observatory_service_test.go` (604 lines)
  - Mock graph client implementing graph.Client interface
  - 10 test cases covering success, threshold filtering, empty results, limits
  - All tests pass with race detector enabled

## Decisions Made

1. **Internal threshold = 0.5**: Per CONTEXT.md "Fixed anomaly score threshold internally"
2. **Top 5 hotspots**: Per RESEARCH.md recommendation for Orient stage
3. **Top 20 workloads/dashboards**: Per RESEARCH.md recommendation for Narrow stage
4. **Confidence tiebreaker**: When anomaly scores are equal, higher confidence wins
5. **RFC3339 timestamps**: All response types include RFC3339 formatted timestamp field

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- **Mock query matching**: Initial test mock incorrectly matched AnomalyAggregator's namespace workloads query as cluster namespace query due to overlapping patterns. Fixed by using more specific query pattern matching (checking "AS workload_name" vs "AS namespace").

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- ObservatoryService provides foundation for all 8 MCP tools
- GetClusterAnomalies ready for observatory_status tool
- GetNamespaceAnomalies ready for observatory_scope tool
- GetWorkloadAnomalyDetail ready for observatory_signals tool
- GetDashboardQuality ready for API-05 requirement

**No blockers or concerns.**

---
*Phase: 26-observatory-api-mcp-tools*
*Completed: 2026-01-30*
