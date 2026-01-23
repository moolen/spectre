---
phase: 20-alert-api-client
plan: 02
subsystem: graph-ingestion
tags: [grafana, alerts, promql, falkordb, graph-sync]

# Dependency graph
requires:
  - phase: 20-01
    provides: AlertRule types and ListAlertRules API method
  - phase: 16-02
    provides: DashboardSyncer pattern and GraphBuilder framework
  - phase: 16-01
    provides: PromQL parser for metric extraction
provides:
  - AlertSyncer with incremental timestamp-based synchronization
  - BuildAlertGraph method for Alert node and MONITORS edge creation
  - Automatic alert rule ingestion from Grafana hourly
  - Alert→Metric→Service transitive graph relationships
affects: [20-03, 21-alert-state-sync]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Incremental sync via Updated timestamp comparison (ISO8601 string compare)"
    - "Shared GraphBuilder instance between Dashboard and Alert syncers"
    - "Integration field in all nodes for multi-Grafana support"

key-files:
  created:
    - internal/integration/grafana/alert_syncer.go
    - internal/integration/grafana/alert_syncer_test.go
  modified:
    - internal/integration/grafana/graph_builder.go
    - internal/integration/grafana/grafana.go
    - internal/integration/grafana/dashboard_syncer.go

key-decisions:
  - "ISO8601 string comparison for timestamp-based incremental sync (no parse needed)"
  - "Shared GraphBuilder instance for both dashboard and alert syncing"
  - "Integration name parameter added to GraphBuilder constructor for node tagging"
  - "First PromQL expression stored as condition field for alert display"
  - "Alert→Service relationships accessed transitively via Metrics (no direct edge)"

patterns-established:
  - "Syncer pattern: Start/Stop lifecycle with cancellable context and ticker loop"
  - "needsSync method: query graph for existing node, compare version/timestamp"
  - "Graceful degradation: log parse errors and continue with other queries"

# Metrics
duration: 7min
completed: 2026-01-23
---

# Phase 20 Plan 02: Alert Rules Sync Service Summary

**AlertSyncer implements hourly incremental sync of Grafana alert rules with PromQL-based metric extraction and transitive Alert→Metric→Service graph relationships**

## Performance

- **Duration:** 7 minutes
- **Started:** 2026-01-23T08:47:32Z
- **Completed:** 2026-01-23T08:54:50Z
- **Tasks:** 3
- **Files modified:** 7

## Accomplishments
- AlertSyncer with incremental timestamp-based sync (compares Updated field, skips unchanged alerts)
- BuildAlertGraph method extracts PromQL expressions from AlertQuery.Model JSON and creates MONITORS edges
- Alert rules automatically synced every hour when graph client available
- Transitive Alert→Metric→Service relationships enable incident response reasoning

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement AlertSyncer with incremental sync** - `e5c0c24` (feat)
2. **Task 2: Extend GraphBuilder with alert graph methods** - `d3f4c78` (feat)
3. **Task 3: Wire AlertSyncer into Grafana integration lifecycle** - `2b9e265` (feat)

## Files Created/Modified
- `internal/integration/grafana/alert_syncer.go` - AlertSyncer orchestrates incremental alert rule synchronization with ticker loop
- `internal/integration/grafana/alert_syncer_test.go` - Comprehensive test coverage for all sync scenarios (new/updated/unchanged/errors/lifecycle)
- `internal/integration/grafana/graph_builder.go` - BuildAlertGraph method creates Alert nodes and MONITORS edges from alert rules
- `internal/integration/grafana/grafana.go` - Wired AlertSyncer into integration Start/Stop lifecycle with shared GraphBuilder
- `internal/integration/grafana/dashboard_syncer.go` - Updated to accept integrationName parameter for node tagging
- `internal/integration/grafana/graph_builder_test.go` - Updated all test usages to pass integrationName
- `internal/integration/grafana/dashboard_syncer_test.go` - Updated NewDashboardSyncer calls with integrationName parameter

## Decisions Made
- **ISO8601 string comparison for timestamps:** Alert.Updated timestamps compared as RFC3339 strings, simpler than parsing to time.Time
- **Integration name in GraphBuilder:** Added integrationName field to GraphBuilder for consistent node tagging across syncers
- **Shared GraphBuilder instance:** Single GraphBuilder serves both DashboardSyncer and AlertSyncer to ensure consistent integration field
- **First PromQL as condition:** Extract first PromQL expression from alert queries as condition field for display purposes
- **Transitive service relationships:** No direct Alert→Service edges; services accessed via (Alert)-[:MONITORS]->(Metric)-[:TRACKS]->(Service) path

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - implementation followed DashboardSyncer pattern closely with expected integration points.

## User Setup Required

None - no external service configuration required. Alert syncing starts automatically when graph client is configured.

## Next Phase Readiness
- Alert rule metadata ingestion complete
- Graph contains Alert nodes with MONITORS relationships to Metrics
- Transitive Alert→Metric→Service paths enable incident response queries
- Ready for Phase 20-03 (Alert Query Tools) to expose alert data via MCP
- Alert state tracking (firing/pending) deferred to Phase 21

---
*Phase: 20-alert-api-client*
*Completed: 2026-01-23*
