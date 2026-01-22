# Phase 12: MCP Tools - Overview and Logs - Context

**Gathered:** 2026-01-22
**Status:** Ready for planning

<domain>
## Phase Boundary

Expose Logz.io data through MCP tools with progressive disclosure. Two tools: overview (severity summary with top error sources) and logs (raw logs with filters). Pattern mining tool is Phase 13.

</domain>

<decisions>
## Implementation Decisions

### Tool Naming & Structure
- Follow VictoriaLogs naming pattern: `logzio_{name}_overview`, `logzio_{name}_logs`
- Each tool defines its own complete parameter schema (no shared base)
- Support optional query string parameter for full-text search
- Normalize response to common schema (timestamp, message, level, namespace, pod) matching VictoriaLogs format

### Overview Response Format
- Severity breakdown: error, warn, info, debug, trace + total (match VictoriaLogs)
- Totals only (no time-based histogram)
- Include top 5 namespaces/pods with highest error counts
- Default time range: last 1 hour

### Logs Filtering & Limits
- Namespace is required, all other filters optional (pod, container, level, query)
- Maximum limit: 100 logs per request (more conservative than 500)
- Default sort: newest first
- No pagination - single request, rely on filters to narrow scope

### Error Handling
- Auth failures: clear error message explaining authentication issue, suggest checking token
- Rate limits (429): immediate error returned to caller (no retry)
- Leading wildcard queries: reject with helpful error explaining Logz.io limitation + suggestion
- No debug metadata in responses (no took_ms, keep minimal)

### Claude's Discretion
- Exact parameter naming within tools
- Field mapping details from Logz.io to common schema
- Error message wording
- Default limit value (if user doesn't specify)

</decisions>

<specifics>
## Specific Ideas

- Match VictoriaLogs tool UX so AI assistants can use both backends consistently
- Overview should help triage by showing where errors are concentrated

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 12-mcp-tools-overview-logs*
*Context gathered: 2026-01-22*
