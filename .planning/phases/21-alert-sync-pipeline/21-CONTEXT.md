# Phase 21: Alert Sync Pipeline - Context

**Gathered:** 2026-01-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Continuously track alert state transitions and store state change history in the graph. AlertSyncer fetches current state (firing/pending/normal), creates AlertStateChange edges for transitions, and handles API unavailability gracefully. This phase builds on Phase 20's alert rule sync.

</domain>

<decisions>
## Implementation Decisions

### Sync frequency & triggers
- Periodic sync only (no on-demand triggers from MCP tools)
- 5-minute sync interval
- Independent timer from dashboard/alert rule sync (allows different frequencies later)
- On Grafana API errors: skip cycle and log warning, try again next interval (no backoff)

### State transition storage
- State changes stored as edge properties (not separate nodes)
- 3-state model: firing, pending, normal (no silenced/paused tracking)
- Deduplicate consecutive same-state syncs — only store actual transitions
- Minimal metadata per transition: from_state, to_state, timestamp

### Timeline retention
- 7-day retention window (matches Phase 22 baseline analysis window)
- TTL via expires_at timestamp in graph (same pattern as baseline cache)
- All edges use TTL including current state — refreshed on each sync
- Cascade delete when alert rule is deleted in Grafana — remove node and all state edges

### Staleness handling
- last_synced_at timestamp field on each alert node (per-alert granularity)
- When API unavailable: leave existing data as-is, don't update timestamps
- No explicit stale flag — AI interprets timestamp age
- No staleness warnings in MCP tool responses — AI checks timestamps if needed

### Claude's Discretion
- Edge property schema design
- Exact Grafana API endpoint selection for state queries
- State comparison logic implementation
- Logging verbosity and message format

</decisions>

<specifics>
## Specific Ideas

- Follows existing patterns: TTL implementation from Phase 19 baseline cache
- Independent timers allow future optimization (state could sync more frequently than rules)
- Per-alert timestamps enable granular staleness detection

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 21-alert-sync-pipeline*
*Context gathered: 2026-01-23*
