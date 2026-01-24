---
phase: 17-semantic-layer
plan: 03
subsystem: integration
tags: [grafana, graph, neo4j, hierarchy, dashboard-classification]

# Dependency graph
requires:
  - phase: 16-ingestion-pipeline
    provides: Dashboard sync infrastructure and graph builder pattern
provides:
  - Dashboard hierarchy classification (overview/drilldown/detail)
  - HierarchyMap config for tag-based fallback mapping
  - hierarchyLevel property on Dashboard nodes
affects: [18-mcp-tools, semantic-layer, progressive-disclosure]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Tag-first classification with fallback config mapping
    - Case-insensitive hierarchy tag detection
    - Per-tag HierarchyMap for flexible classification

key-files:
  created: []
  modified:
    - internal/integration/grafana/types.go
    - internal/integration/grafana/graph_builder.go
    - internal/integration/grafana/dashboard_syncer.go
    - internal/integration/grafana/grafana.go
    - internal/integration/grafana/graph_builder_test.go

key-decisions:
  - "Per-tag HierarchyMap mapping (simplest, most flexible) - each tag maps to a level, first match wins"
  - "Tag patterns: spectre:* and hierarchy:* both supported for flexibility"
  - "Case-insensitive tag matching for user convenience"
  - "Tags always override config mapping when both present"

patterns-established:
  - "Classification priority: explicit tags → config mapping → default"
  - "Config validation in Validate() method for all map fields"
  - "Graph node properties include semantic metadata (hierarchyLevel)"

# Metrics
duration: 5min
completed: 2026-01-23
---

# Phase 17 Plan 03: Dashboard Hierarchy Classification Summary

**Dashboard hierarchy classification via tags (spectre:overview/drilldown/detail) with HierarchyMap config fallback, enabling progressive disclosure in MCP tools**

## Performance

- **Duration:** 5 min
- **Started:** 2026-01-23T23:27:30Z
- **Completed:** 2026-01-23T23:32:21Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- Dashboard nodes now include hierarchyLevel property (overview/drilldown/detail)
- Config supports HierarchyMap for tag-based fallback when explicit hierarchy tags absent
- Classification uses tag-first logic with case-insensitive matching
- Comprehensive test coverage for all classification paths

## Task Commits

Each task was committed atomically:

1. **Task 1: Add HierarchyMap to Config and extend Validate** - `86e43f6` (feat)
   - Added HierarchyMap field to Config struct with JSON/YAML tags
   - Extended Validate() to check map values are valid levels
   - Documented mapping semantics in struct comments

2. **Task 2: Implement dashboard hierarchy classification** - `3e14320` (feat)
   - Added config field to GraphBuilder struct
   - Implemented classifyHierarchy method with tag-first, fallback, default logic
   - Updated CreateDashboardGraph to classify and store hierarchyLevel
   - Updated NewGraphBuilder signature to accept config parameter
   - Updated NewDashboardSyncer to pass config to GraphBuilder
   - Updated grafana.go integration to pass config when creating syncer
   - Added comprehensive unit tests for all classification paths
   - Updated all test call sites for new signatures

## Files Created/Modified
- `internal/integration/grafana/types.go` - Added HierarchyMap field and validation
- `internal/integration/grafana/graph_builder.go` - Added classifyHierarchy method, config field, hierarchyLevel to Dashboard nodes
- `internal/integration/grafana/dashboard_syncer.go` - Updated NewDashboardSyncer signature to accept config
- `internal/integration/grafana/grafana.go` - Pass config when creating syncer
- `internal/integration/grafana/graph_builder_test.go` - Added hierarchy classification tests

## Decisions Made

1. **Per-tag mapping granularity:** Used per-tag mapping (each tag maps to a level) as simplest and most flexible approach. Dashboard with multiple tags uses first matching tag.

2. **Tag pattern support:** Support both `spectre:*` and `hierarchy:*` tag formats for flexibility. Users can choose their preferred convention.

3. **Case-insensitive matching:** Tag matching is case-insensitive (`SPECTRE:OVERVIEW` works same as `spectre:overview`) for user convenience and robustness.

4. **Tags override mapping:** Explicit hierarchy tags always take priority over HierarchyMap lookup. This ensures explicit intent is honored.

5. **Default to detail:** When no hierarchy signals present (no tags, no mapping), default to "detail" level as most conservative choice.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - implementation was straightforward following the established graph builder pattern.

## User Setup Required

None - no external service configuration required.

HierarchyMap is optional config. If not specified, all dashboards default to "detail" level unless they have explicit hierarchy tags (spectre:* or hierarchy:*).

Example config usage:
```yaml
integrations:
  - name: production-grafana
    type: grafana
    config:
      url: https://grafana.example.com
      hierarchyMap:
        prod: overview
        staging: drilldown
        dev: detail
```

## Next Phase Readiness

- Dashboard hierarchy classification complete and tested
- hierarchyLevel property available on Dashboard nodes in graph
- Ready for Phase 18 MCP tools to leverage hierarchy for progressive disclosure
- Can filter/order dashboards by hierarchy level in tool responses

**Blockers:** None

**Notes:**
- Classification is deterministic: same tags always produce same level
- Config validation ensures only valid levels (overview/drilldown/detail) in HierarchyMap
- All existing tests pass, no regressions
- 44.4% test coverage for grafana integration package

---
*Phase: 17-semantic-layer*
*Completed: 2026-01-23*
