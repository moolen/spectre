# Phase 5: Progressive Disclosure MCP Tools - Context

**Gathered:** 2026-01-21
**Status:** Ready for planning

<domain>
## Phase Boundary

AI assistants explore logs progressively via MCP tools: overview → patterns → details. Three core tools per integration instance, namespaced by integration type and name. Stateless design where each tool call is independent.

</domain>

<decisions>
## Implementation Decisions

### Tool Granularity
- One tool per level: overview, patterns, detail
- Tool naming: `{integration-type}_{name}_{tool}` (e.g., `victorialogs_dev_overview`, `victorialogs_prod_patterns`)
- Each integration instance gets its own set of 3 tools
- Just the 3 core tools — no additional helper tools
- Overview params: time range + optional namespace filter + optional severity filter
- Detail params: namespace + time range + limit (no template-based drill-down)

### Response Format
- Compact by default — minimal data, counts, IDs, short summaries
- Overview response: counts + anomalies (novel/unusual patterns flagged)
- Patterns response: template + count + one sample raw log
- No pagination — return all results up to reasonable limit, truncate if too many
- No suggested next actions in responses — just data

### Drill-down State
- Stateless — each tool call is independent, AI must re-specify all filters
- Absolute timestamps for time ranges (RFC3339 format)
- Default time range: last 1 hour when not specified

### Novelty Presentation
- Compare current period to previous period of same duration
- Boolean `is_novel` flag per template
- Comparison window matches query duration (query last 1h → compare to hour before that)

### Claude's Discretion
- Novelty count threshold (minimum occurrences to flag as novel)
- Exact response field names and structure
- Error response format
- Template limit per response

</decisions>

<specifics>
## Specific Ideas

- Tool naming convention mirrors multi-environment deployment pattern (dev/staging/prod)
- Compact responses keep AI context window usage low
- Stateless design simplifies server implementation and enables horizontal scaling

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 05-progressive-disclosure-mcp-tools*
*Context gathered: 2026-01-21*
