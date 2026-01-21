# Phase 3: VictoriaLogs Client & Basic Pipeline - Context

**Gathered:** 2026-01-21
**Status:** Ready for planning

<domain>
## Phase Boundary

MCP server ingests and queries logs from VictoriaLogs with backpressure handling. Supports time range filtering, aggregation by namespace/pod/deployment, and histogram queries. Template mining and progressive disclosure tools are separate phases.

</domain>

<decisions>
## Implementation Decisions

### Query Interface Design
- Structured parameters only (no raw LogsQL exposed to MCP tools)
- K8s-focused filter fields: namespace, pod, container, level, time range
- Default time range: last 1 hour when not specified
- Log level filtering: exact match only (level=warn returns only warn, not warn+error+fatal)

### Error Handling & Resilience
- Fail fast with clear error when VictoriaLogs unreachable (no retries)
- Query timeout: 30 seconds
- Include full VictoriaLogs error details in error messages (helpful for debugging)
- When integration is in degraded state: attempt queries anyway (might work even if health check failed)

### Response Formatting
- Maximum 1000 log lines per query
- Include 'hasMore' flag and total count when results exceed limit
- Histogram/aggregation data grouped by dimension: `{namespace: [{timestamp, count}], ...}`
- Timestamps in ISO 8601 format: "2026-01-21T10:30:00Z"

### Pipeline Behavior
- Channel buffer size: 1000 items (medium - balanced memory vs throughput)
- Backpressure handling: block and wait until space available (no data loss)
- Batching: fixed size of 100 logs before sending to VictoriaLogs
- Expose pipeline metrics via Prometheus: queue depth, batch count, throughput

### Claude's Discretion
- HTTP client configuration details (connection pooling, keep-alive)
- Exact Prometheus metric names and labels
- Internal batch flush timing edge cases
- LogsQL query construction from structured parameters

</decisions>

<specifics>
## Specific Ideas

- Pipeline should feel production-ready with proper observability from day 1
- Error messages should be actionable - AI assistant needs enough detail to understand what went wrong

</specifics>

<deferred>
## Deferred Ideas

None - discussion stayed within phase scope

</deferred>

---

*Phase: 03-victorialogs-client-pipeline*
*Context gathered: 2026-01-21*
