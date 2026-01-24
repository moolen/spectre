---
phase: 17-semantic-layer
plan: 02
subsystem: graph
tags: [grafana, neo4j, dashboard, variables, classification]

# Dependency graph
requires:
  - phase: 16-ingestion-pipeline
    provides: Dashboard graph structure with panels and queries
provides:
  - Variable nodes with semantic classification (scoping/entity/detail/unknown)
  - HAS_VARIABLE edges linking dashboards to variables
  - Pattern-based variable classification logic
affects: [17-04-fallback-mapping-ui]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Pattern-based classification for dashboard variables
    - Graceful degradation for malformed variables

key-files:
  created: []
  modified:
    - internal/graph/models.go
    - internal/integration/grafana/graph_builder.go
    - internal/integration/grafana/graph_builder_test.go

key-decisions:
  - "Variable classification uses case-insensitive pattern matching"
  - "Unknown classification for unrecognized variable names"
  - "Graceful handling of malformed variables with warning logs"
  - "Variable nodes use composite key: dashboardUID + name"

patterns-established:
  - "Pattern-based semantic classification: multiple pattern lists checked in order"
  - "MERGE upsert semantics for variable nodes"
  - "Comprehensive test coverage for all classification categories"

# Metrics
duration: 7min
completed: 2026-01-23
---

# Phase 17 Plan 02: Variable Classification Summary

**Pattern-based variable classification with scoping/entity/detail/unknown categories for semantic dashboard queries**

## Performance

- **Duration:** 7 min
- **Started:** 2026-01-23T00:27:29Z
- **Completed:** 2026-01-23T00:34:29Z
- **Tasks:** 1
- **Files modified:** 3

## Accomplishments
- Variable node type and HAS_VARIABLE edge added to graph schema
- Pattern-based classification function with 4 categories (scoping/entity/detail/unknown)
- Variable node creation integrated into dashboard sync workflow
- Comprehensive test coverage for all classification patterns and edge cases
- Graceful handling of malformed variables (not a map, missing name, empty name)

## Task Commits

**Note:** This plan's implementation was included in commit c9bd956 (feat(17-01)) alongside Service node inference. The variable classification code was added together with the service inference feature as part of the broader semantic layer implementation.

1. **Task 1: Parse dashboard variables and classify by type** - `c9bd956` (feat) - included in 17-01

## Files Created/Modified
- `internal/graph/models.go` - Added NodeTypeVariable and EdgeTypeHasVariable constants, VariableNode struct
- `internal/integration/grafana/graph_builder.go` - Added classifyVariable() and createVariableNodes() functions, integrated into CreateDashboardGraph
- `internal/integration/grafana/graph_builder_test.go` - Added comprehensive tests for variable classification (scoping/entity/detail/unknown), malformed variable handling, and edge creation

## Decisions Made

**Variable classification patterns:**
- Scoping: cluster, region, env, environment, datacenter, zone
- Entity: service, namespace, app, application, deployment, pod, container
- Detail: instance, node, host, endpoint, handler, path
- Unknown: default for unrecognized patterns

**Malformed variable handling:**
- Variables must be JSON maps with a "name" field
- Missing or empty names skip the variable with a warning log
- Type field is optional, defaults to "unknown"
- Graceful degradation ensures dashboard sync continues despite malformed variables

**Classification approach:**
- Case-insensitive substring matching (converts to lowercase before matching)
- First match wins (scoping checked first, then entity, then detail)
- Simple and fast - no regex, just strings.Contains()

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - implementation proceeded smoothly with all tests passing on first run.

## Next Phase Readiness

**Ready for Phase 17-04 (Fallback Mapping UI):**
- Variable classification logic complete and tested
- Graph schema includes Variable nodes and HAS_VARIABLE edges
- Classification results can be queried from graph for UI display
- HierarchyMap pattern established (from 17-03) provides model for variable fallback mapping

**No blockers** - Variable classification working correctly and integrated into dashboard sync.

---
*Phase: 17-semantic-layer*
*Completed: 2026-01-23*
