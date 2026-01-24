---
phase: 16-ingestion-pipeline
plan: 02
subsystem: graph
tags: [grafana, falkordb, dashboard-sync, promql, cypher, graph-database]

# Dependency graph
requires:
  - phase: 16-01
    provides: PromQL parser with semantic extraction (metrics, labels, aggregations, variables)
provides:
  - Dashboard semantic graph with Panel/Query/Metric nodes and relationships
  - Incremental sync with version-based change detection
  - Full dashboard replace pattern preserving shared Metric nodes
  - Hourly periodic sync with graceful error handling
affects: [17-service-inference, 18-mcp-tools]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "MERGE-based upsert semantics for all graph nodes"
    - "Full dashboard replace pattern (delete panels/queries, preserve metrics)"
    - "Incremental sync via version field comparison"
    - "Periodic sync loop with ticker and cancellable context"
    - "Interface-based design for testability (GrafanaClientInterface, PromQLParserInterface)"
    - "Optional graph client injection via SetGraphClient method"

key-files:
  created:
    - internal/integration/grafana/graph_builder.go
    - internal/integration/grafana/graph_builder_test.go
    - internal/integration/grafana/dashboard_syncer.go
    - internal/integration/grafana/dashboard_syncer_test.go
    - internal/integration/grafana/integration_lifecycle_test.go
  modified:
    - internal/graph/models.go
    - internal/integration/grafana/grafana.go

key-decisions:
  - "MERGE-based upsert for all nodes - simpler than separate CREATE/UPDATE logic"
  - "Full dashboard replace pattern - simpler than incremental panel updates"
  - "Metric nodes preserved on dashboard delete - shared entities across dashboards"
  - "Graceful degradation: log parse errors but continue with other panels/queries"
  - "Dashboard sync optional - integration works without graph client"
  - "SetGraphClient injection pattern - transitional API for graph client access"

patterns-established:
  - "Interface-based testing: mock implementations for GrafanaClient and PromQLParser"
  - "Thread-safe status tracking with RWMutex for concurrent access"
  - "Periodic background workers with ticker and cancellable context"

# Metrics
duration: 10min
completed: 2026-01-22
---

# Phase 16 Plan 02: Dashboard Sync Summary

**Incremental dashboard synchronization with semantic graph storage (Dashboard→Panel→Query→Metric) using version-based change detection and hourly periodic sync**

## Performance

- **Duration:** 10 min
- **Started:** 2026-01-22T22:09:47Z
- **Completed:** 2026-01-22T22:19:52Z
- **Tasks:** 4
- **Files modified:** 7

## Accomplishments

- Panel, Query, Metric node types added to graph schema with CONTAINS/HAS/USES relationships
- GraphBuilder transforms Grafana dashboard JSON into graph nodes with MERGE-based upsert
- DashboardSyncer orchestrates incremental sync with version comparison and hourly periodic loop
- Integration lifecycle wiring with optional graph client via SetGraphClient injection
- Comprehensive test coverage with mock clients for Grafana API and graph operations

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Panel, Query, Metric Node Types to Graph Models** - `3acc36a` (feat)
2. **Task 2: Implement Graph Builder for Dashboard Structure** - `cedd268` (feat)
3. **Task 3: Implement Dashboard Syncer with Version-Based Change Detection** - `43feae6` (feat)
4. **Task 4: Integrate Dashboard Syncer into Grafana Integration Lifecycle** - `53a37df` (feat)

## Files Created/Modified

**Created:**
- `internal/integration/grafana/graph_builder.go` - Transforms Grafana dashboard JSON into graph nodes/edges with MERGE upsert
- `internal/integration/grafana/graph_builder_test.go` - Tests for simple panels, multiple queries, variables, graceful degradation
- `internal/integration/grafana/dashboard_syncer.go` - Orchestrates incremental sync with version comparison and periodic loop
- `internal/integration/grafana/dashboard_syncer_test.go` - Tests for new/updated/unchanged dashboards, error handling, lifecycle
- `internal/integration/grafana/integration_lifecycle_test.go` - Integration tests for lifecycle with/without graph client

**Modified:**
- `internal/graph/models.go` - Added NodeTypePanel, NodeTypeQuery, NodeTypeMetric, EdgeTypeContains, EdgeTypeHas, EdgeTypeUses
- `internal/integration/grafana/grafana.go` - Added syncer field, SetGraphClient method, Start/Stop lifecycle integration

## Decisions Made

**Graph Schema Design:**
- MERGE-based upsert semantics for all nodes - simpler than separate CREATE/UPDATE logic, handles both initial creation and updates
- Full dashboard replace pattern - delete all panels/queries on update, then recreate - simpler than incremental panel updates
- Metric nodes preserved when dashboard deleted - metrics are shared entities used by multiple dashboards

**Sync Strategy:**
- Version-based change detection - query graph for existing version, compare with Grafana current version, skip if unchanged
- Hourly periodic sync - balance between data freshness and API load
- Graceful degradation - log parse errors but continue with other panels/queries (don't fail entire sync for one dashboard)

**Architecture:**
- SetGraphClient injection pattern - transitional API for graph client access without changing Integration interface
- Dashboard sync optional - integration works without graph client (sync simply disabled)
- Interface-based design - GrafanaClientInterface and PromQLParserInterface for testability with mocks

## Deviations from Plan

**1. [Minor Enhancement] Added PromQLParserInterface for testability**
- **Found during:** Task 2 (GraphBuilder implementation)
- **Issue:** Direct use of PromQLParser struct made testing difficult - needed to inject mock parser
- **Fix:** Created PromQLParserInterface with Parse method, defaultPromQLParser implementation wraps ExtractFromPromQL
- **Files modified:** internal/integration/grafana/graph_builder.go
- **Verification:** Tests use mockPromQLParser that implements interface
- **Committed in:** cedd268 (Task 2 commit)

**2. [Minor Enhancement] Added GrafanaClientInterface for testability**
- **Found during:** Task 3 (DashboardSyncer implementation)
- **Issue:** Direct use of GrafanaClient pointer made testing difficult - needed to inject mock client
- **Fix:** Created GrafanaClientInterface with ListDashboards and GetDashboard methods
- **Files modified:** internal/integration/grafana/dashboard_syncer.go
- **Verification:** Tests use mockGrafanaClient that implements interface
- **Committed in:** 43feae6 (Task 3 commit)

**3. [Architectural Adjustment] SetGraphClient injection pattern**
- **Found during:** Task 4 (Integration lifecycle)
- **Issue:** Integration factory doesn't receive graph client parameter - factory signature is (name, config)
- **Fix:** Added SetGraphClient method to GrafanaIntegration, documented as transitional API
- **Files modified:** internal/integration/grafana/grafana.go
- **Verification:** Tests validate SetGraphClient works, integration starts syncer when graph client available
- **Committed in:** 53a37df (Task 4 commit)

---

**Total deviations:** 3 enhancements (2 testability interfaces, 1 architectural adjustment)
**Impact on plan:** All deviations necessary for clean testing and pragmatic graph client access. No scope creep - all planned functionality delivered.

## Issues Encountered

None - plan executed smoothly with minor testability enhancements.

## User Setup Required

None - no external service configuration required. Dashboard sync is automatic once Grafana integration is configured and graph client is set.

## Next Phase Readiness

**Ready for Phase 17 (Service Inference):**
- Dashboard semantic graph fully populated with Panel/Query/Metric relationships
- Metric nodes contain names for service inference algorithms
- Query nodes contain label selectors for service correlation
- Periodic sync ensures graph stays current with Grafana changes

**Ready for Phase 18 (MCP Tools):**
- Dashboard sync status available via GetSyncStatus for UI display
- Graph contains complete dashboard structure for MCP tool queries
- Incremental sync minimizes API load and graph operations

**No blockers or concerns.**

---
*Phase: 16-ingestion-pipeline*
*Completed: 2026-01-22*
