---
phase: 20-alert-api-client
plan: 01
subsystem: api
tags: [grafana, alerting, graph-schema, api-client]

# Dependency graph
requires:
  - phase: 16-graph-ingestion
    provides: "Graph schema patterns for dashboard nodes and edges"
  - phase: 15-grafana-integration
    provides: "GrafanaClient with Bearer token authentication patterns"
provides:
  - "Alert node type (NodeTypeAlert) and MONITORS edge for graph schema"
  - "AlertNode struct with 9 metadata fields for alert rule storage"
  - "GrafanaClient methods for Grafana Alerting API (ListAlertRules, GetAlertRule)"
  - "AlertRule and AlertQuery structs for PromQL expression extraction"
affects: [20-02-sync, 21-alert-states, graph-ingestion, mcp-tools]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Alert rules API pattern following dashboard API conventions"
    - "AlertQuery.Model as json.RawMessage for PromQL extraction in next phase"

key-files:
  created: []
  modified:
    - internal/graph/models.go
    - internal/integration/grafana/client.go

key-decisions:
  - "Alert rule metadata stored in AlertNode (definition), state tracking deferred to Phase 21 (AlertStateChange nodes)"
  - "AlertQuery.Model stored as json.RawMessage for flexible PromQL parsing in Phase 20-02"
  - "Integration field added to AlertNode for multi-Grafana support"

patterns-established:
  - "Alert nodes follow dashboard node pattern with FirstSeen/LastSeen tracking"
  - "MONITORS edge type for Alert -> Metric/Service relationships"
  - "Alerting Provisioning API (/api/v1/provisioning/alert-rules) for rule definitions"

# Metrics
duration: 2min
completed: 2026-01-23
---

# Phase 20 Plan 01: Alert API Client & Graph Schema Summary

**Alert node types added to graph schema with GrafanaClient methods for fetching alert rules via Grafana Alerting Provisioning API**

## Performance

- **Duration:** 2 min
- **Started:** 2026-01-23T08:42:57Z
- **Completed:** 2026-01-23T08:44:49Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Alert node types (NodeTypeAlert, EdgeTypeMonitors, AlertNode struct) added to graph schema
- GrafanaClient extended with ListAlertRules() and GetAlertRule() methods
- AlertRule struct contains Data field with AlertQuery array for PromQL extraction
- All code compiles without errors, no test regressions

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Alert node type and MONITORS edge to graph schema** - `1d092f4` (feat)
2. **Task 2: Add alert rules API methods to GrafanaClient** - `67c3c3c` (feat)

## Files Created/Modified
- `internal/graph/models.go` - Added NodeTypeAlert constant, EdgeTypeMonitors constant, and AlertNode struct with 9 fields (UID, Title, FolderTitle, RuleGroup, Condition, Labels, Annotations, Updated, Integration)
- `internal/integration/grafana/client.go` - Added AlertRule and AlertQuery structs, ListAlertRules() and GetAlertRule() methods using /api/v1/provisioning/alert-rules endpoint

## Decisions Made

**1. Alert definition vs state separation**
- Alert rule metadata (title, condition, labels) stored in AlertNode
- Alert state tracking (firing/pending/normal) deferred to Phase 21 AlertStateChange nodes
- Rationale: Clean separation between rule definition (relatively static) and state (frequently changing)

**2. AlertQuery.Model as json.RawMessage**
- Model field stores raw JSON for flexible parsing
- Enables Phase 20-02 to extract PromQL expressions without coupling to exact Grafana model structure
- Rationale: Grafana query models vary by datasource type, raw storage enables type-specific parsing

**3. Integration field in AlertNode**
- Added Integration string field for multi-Grafana support
- Follows pattern from DashboardNode (no integration field there yet, but anticipated)
- Rationale: Enable future support for multiple Grafana instances with alert rule scoping

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - both tasks completed without issues.

## Next Phase Readiness

Ready for Phase 20-02 (Alert Rules Sync Service):
- Alert node types available for graph ingestion
- GrafanaClient can fetch alert rules from Grafana Alerting API
- AlertRule.Data contains PromQL queries for metric extraction
- No blockers identified

---
*Phase: 20-alert-api-client*
*Completed: 2026-01-23*
