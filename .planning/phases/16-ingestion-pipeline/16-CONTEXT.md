# Phase 16: Ingestion Pipeline - Context

**Gathered:** 2026-01-22
**Status:** Ready for planning

<domain>
## Phase Boundary

Incremental dashboard sync with full semantic structure extraction to graph. Detect changed dashboards via version field, parse PromQL queries to extract metrics/labels/functions, and build Dashboard→Panel→Query→Metric relationships. UI displays sync status and provides manual sync trigger.

</domain>

<decisions>
## Implementation Decisions

### Sync Behavior
- Sync on startup + hourly interval (automatic periodic sync)
- Sync all dashboards the API token can access (no folder filtering)
- Full replace on dashboard update — delete all existing Panel/Query nodes for that dashboard, recreate from scratch
- Orphan cleanup for deleted dashboards — remove Dashboard node but keep Metric nodes if used by other dashboards

### PromQL Parsing
- Full AST parsing — extract metric names, label selectors, and aggregation functions
- Use existing Go PromQL library (prometheus/prometheus or similar)
- Log + skip unparseable queries — log warning, skip the query, continue syncing
- Store aggregation functions as properties on Query node (not separate Function nodes)

### Variable Handling
- Extract variables as placeholders — replace variable syntax with marker, store variable reference separately
- Store variable definitions as JSON property on Dashboard node (not separate Variable nodes)
- Capture variable default values during sync
- Query→Metric relationship with variables: Claude's discretion based on what's useful for downstream MCP tools

### UI Feedback
- Summary status display: last sync time + dashboard count + success/error indicator
- Live progress during sync: "Syncing dashboard 5 of 23..."
- Errors shown in status area with click-to-see-details
- Sync status displayed inline in integrations list (not just detail view)
- Manual sync button in integrations table row

### Claude's Discretion
- Query→Metric relationship when metric name contains variable (pattern vs no node)
- Exact progress indicator implementation
- Error detail format and storage

</decisions>

<specifics>
## Specific Ideas

- Follow existing VictoriaLogs integration pattern for consistency
- Sync button should be visually distinct in the table row (not hidden in menu)

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 16-ingestion-pipeline*
*Context gathered: 2026-01-22*
