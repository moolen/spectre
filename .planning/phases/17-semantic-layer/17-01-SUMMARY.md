---
phase: 17-semantic-layer
plan: 01
subsystem: graph
tags: [falkordb, promql, service-inference, semantic-layer]

# Dependency graph
requires:
  - phase: 16-ingestion-pipeline
    provides: PromQL parsing and label selector extraction
provides:
  - Service node type with cluster/namespace scoping
  - TRACKS edge linking metrics to services
  - Service inference logic with label priority (app > service > job)
affects: [17-02, 17-03, semantic-queries, service-exploration]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Service inference from PromQL label selectors
    - Label priority hierarchy (app > service > job)
    - Multiple service node creation when labels conflict
    - Unknown service fallback when no service labels present

key-files:
  created: []
  modified:
    - internal/graph/models.go
    - internal/integration/grafana/graph_builder.go
    - internal/integration/grafana/graph_builder_test.go

key-decisions:
  - "Service identity = {name, cluster, namespace} for proper scoping"
  - "Multiple service nodes when labels disagree instead of choosing one"
  - "Unknown service with empty cluster/namespace when no labels present"
  - "TRACKS edges from Metric to Service (not Query to Service)"

patterns-established:
  - "inferServiceFromLabels function with priority-based label extraction"
  - "ServiceInference struct for passing inferred service metadata"
  - "Graceful degradation: log errors but continue with other services"

# Metrics
duration: 4min
completed: 2026-01-23
---

# Phase 17 Plan 01: Service Inference Summary

**Service nodes inferred from PromQL label selectors with app/service/job priority and cluster/namespace scoping**

## Performance

- **Duration:** 4 min
- **Started:** 2026-01-22T23:27:30Z
- **Completed:** 2026-01-22T23:31:41Z
- **Tasks:** 1
- **Files modified:** 5

## Accomplishments
- Service node type added to graph with cluster/namespace scoping
- TRACKS edge type linking metrics to services
- Label priority logic (app > service > job) with multiple service support
- Unknown service fallback when no service labels present
- Comprehensive unit tests covering priority, scoping, and edge cases

## Task Commits

Each task was committed atomically:

1. **Task 1: Create Service node inference from label selectors** - `c9bd956` (feat)
   - Added Service node type and TRACKS edge type to models.go
   - Implemented inferServiceFromLabels with priority logic
   - Created createServiceNodes for graph operations
   - Integrated into createQueryGraph after metric creation
   - Added 7 comprehensive unit tests

**Test fixes:** `b7c47c8` (fix: update test signatures for Config parameter)

## Files Created/Modified
- `internal/graph/models.go` - Added NodeTypeService, EdgeTypeTracks, and ServiceNode struct
- `internal/integration/grafana/graph_builder.go` - Service inference logic and graph operations
- `internal/integration/grafana/graph_builder_test.go` - 7 unit tests for service inference
- `internal/integration/grafana/dashboard_syncer_test.go` - Fixed test signatures
- `internal/integration/grafana/integration_lifecycle_test.go` - Fixed test signatures

## Decisions Made

**Service identity includes cluster and namespace:** Services are scoped by {name, cluster, namespace} to distinguish the same service name across different clusters/namespaces.

**Multiple services when labels conflict:** When app="frontend" and service="backend" both exist, create two service nodes instead of choosing one. This preserves all label information.

**Unknown service fallback:** When no service-related labels (app/service/job) exist, create a single Unknown service to maintain graph connectivity.

**TRACKS edges from Metric to Service:** The edge direction is Metric-[:TRACKS]->Service (not Query-[:TRACKS]->Service) because metrics are the entities being tracked by services, and metrics are shared across queries.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

**Test signature incompatibility:** NewGraphBuilder and NewDashboardSyncer signatures changed to include Config parameter in concurrent work. Fixed by passing nil for Config in all test constructors.

Resolution: Updated test signatures in separate commit (b7c47c8).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Service inference foundation complete, ready for:
- Dashboard hierarchy classification (Plan 02)
- Variable classification (Plan 03)
- Semantic query capabilities using Service nodes

**Graph schema ready:** Service nodes and TRACKS edges can now be queried for service-to-metric relationships.

**Label whitelist enforced:** Only app, service, job, cluster, namespace labels used for inference as specified in CONTEXT.md.

---
*Phase: 17-semantic-layer*
*Completed: 2026-01-23*
