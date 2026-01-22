# Phase 18: Query Execution & MCP Tools Foundation - Context

**Gathered:** 2026-01-23
**Status:** Ready for planning

<domain>
## Phase Boundary

AI can execute Grafana queries and discover dashboards through three MCP tools (overview, aggregated, details). Tools query via Grafana's /api/ds/query endpoint, accept scoping variables, and return time series data formatted for AI consumption. Progressive disclosure from overview → aggregated → details based on dashboard hierarchy levels established in Phase 17.

</domain>

<decisions>
## Implementation Decisions

### Response format
- Raw data points — full [timestamp, value] arrays, AI decides what matters
- Metadata inline with values — each metric includes labels, unit, panel title together
- Include PromQL query only on error/empty results — keep successful responses clean
- ISO timestamps for time ranges — precise, unambiguous (2026-01-23T10:00:00Z format)

### Tool parameters
- Absolute time range only — from/to timestamps, no relative shortcuts
- Scoping variables required always — cluster, region must be specified (prevents accidental broad queries)
- Aggregated tool accepts service OR namespace — covers common drill-down patterns
- Query all matching dashboards — tools find dashboards by hierarchy level automatically, no dashboard filter parameter

### Error handling
- Partial results + errors — return what worked, list what failed, AI proceeds with partial data
- Omit panels with no data — don't include empty panels, keeps response clean
- Empty success when no dashboards match — return success with no results, AI figures out next step
- Clear error messages on auth failures — "Grafana API returned 403: insufficient permissions for dashboard X"

### Progressive disclosure
- Overview = key metrics only — first 5 panels per overview-level dashboard
- Aggregated = drill-down dashboards — show all panels in drill-down hierarchy dashboards
- Details = detail dashboards — show all panels in detail hierarchy dashboards
- Tools select dashboards by hierarchy level (overview/drill-down/detail) established in Phase 17

### Claude's Discretion
- Exact response JSON structure
- How to handle panels without queries (text panels, etc.)
- Query batching/parallelization strategy
- Timeout values for Grafana API calls

</decisions>

<specifics>
## Specific Ideas

- Overview should be fast and focused — 5 panels is enough to spot anomalies without overload
- Scoping always required prevents "query all clusters" accidents that could be expensive
- Partial results are valuable — better to see 8/10 panels than fail completely

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 18-query-execution-mcp-tools*
*Context gathered: 2026-01-23*
