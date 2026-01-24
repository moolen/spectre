# Phase 23: MCP Tools - Context

**Gathered:** 2026-01-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Three progressive disclosure MCP tools for AI-driven alert analysis: overview (counts/grouping), aggregated (specific alerts with timeline), details (full state history and rule definition). Tools consume AlertAnalysisService from Phase 22. No new alert storage or analysis logic—tools expose existing capabilities.

</domain>

<decisions>
## Implementation Decisions

### Overview Aggregation
- Primary grouping by severity (Critical, Warning, Info)
- Within each severity: both cluster counts AND service names
- Default scope shows ALL states with counts (Firing: X, Pending: Y, Normal: Z)
- Include alert names + firing duration in each severity bucket (e.g., "HighErrorRate (2h)")

### Flappiness Presentation
- Show flapping count in summary per severity: "Critical: 5 (2 flapping)"
- No dedicated flapping tool—AI uses aggregated tool to investigate
- In aggregated view: show raw transition count (e.g., "12 state changes in 1h")
- Flapping threshold: Claude's discretion (use Phase 22 computed flappiness score)

### State Progression Format
- Time bucket display: [F F N N F F] format with 10-minute buckets (6 per hour)
- Single letters: F=firing, N=normal, P=pending
- Aggregated view includes analysis category inline: "HighErrorRate: CHRONIC [F F F F F F]"

### Filter Parameters
- Overview accepts all four filters: severity, cluster, service, namespace
- All filters optional—no filters returns all alerts
- Aggregated tool default lookback: 1 hour (parameter to extend)
- Details tool can accept single alert_uid OR filter by service/cluster for multiple alerts

### Claude's Discretion
- Exact flapping threshold for overview count
- How to handle missing analysis data (insufficient history)
- Tool description wording for AI guidance
- Response formatting details beyond specified structure

</decisions>

<specifics>
## Specific Ideas

- "Names + duration" in overview helps AI triage without extra tool calls
- Time buckets should read left-to-right as oldest→newest for natural timeline reading
- Analysis category (CHRONIC, NEW_ONSET, etc.) from Phase 22 should appear inline in aggregated view

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 23-mcp-tools*
*Context gathered: 2026-01-23*
