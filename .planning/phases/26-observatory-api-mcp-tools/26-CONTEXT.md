# Phase 26: Observatory API & MCP Tools - Context

**Gathered:** 2026-01-30
**Status:** Ready for planning

<domain>
## Phase Boundary

8 MCP tools enabling AI-driven incident investigation through progressive disclosure stages (Orient → Narrow → Investigate → Hypothesize → Verify). Tools expose signal anchors, anomaly scores, baselines, and evidence from Phase 24-25 infrastructure. Eventually replaces separate grafana_alerts_* and log tools.

</domain>

<decisions>
## Implementation Decisions

### Response Structure
- Minimal responses — facts only, AI interprets meaning
- Always include confidence indicators (0-1) for anomaly scores based on sample count/freshness
- Anomaly severity as numeric score only (0.0-1.0), no categorical labels
- No URLs in MCP responses — keep responses data-only

### Tool Boundaries
- Two Orient tools: `observatory_status` (current state) separate from `observatory_changes` (recent deltas)
- Narrow tools return ranked flat lists sorted by anomaly score, not grouped
- Compare tool (`observatory_compare`) compares across time only (current vs N hours/days ago)
- Explain tool (`observatory_explain`) provides both signal context AND anomaly reasoning

### Investigation Flow
- No next-step suggestions in responses — AI decides flow independently
- Evidence tool (`observatory_evidence`) includes inline alert states and log excerpts directly
- Empty results when nothing anomalous (no "healthy" message, no low-score padding)
- No enforcement of stage ordering — tools are stateless, AI can call any tool anytime

### Filtering & Scoping
- Time range: support both relative (lookback duration) and absolute (from/to timestamps)
- Fixed anomaly score threshold internally — no configurable min_score param
- Scope filters (cluster, namespace, workload) all optional, any combination accepted
- No role filtering — return all signal roles, AI ignores in reasoning if needed

### Claude's Discretion
- Internal threshold value for anomaly filtering
- Response pagination / limit defaults
- Exact field naming in responses
- Error response structure

</decisions>

<specifics>
## Specific Ideas

- "I want to eventually remove the other alert/logs tools and only use the observatory_* tools" — design evidence tool to be self-contained
- Keep responses minimal so AI context window isn't bloated with verbose tool output

</specifics>

<deferred>
## Deferred Ideas

- Workload-to-workload comparison (compare tool does time comparison only for now)
- Role-based signal filtering (may add later if needed)
- Deprecation of grafana_alerts_* tools — future cleanup phase

</deferred>

---

*Phase: 26-observatory-api-mcp-tools*
*Context gathered: 2026-01-30*
