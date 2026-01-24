# Phase 1: Plugin Infrastructure Foundation - Context

**Gathered:** 2026-01-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Integration instance management with config hot-reload. Integrations are in-tree (compiled into Spectre), not external plugins. Multiple instances of the same integration type can run with different configs (e.g., victorialogs-prod, victorialogs-staging).

**Key clarification:** HashiCorp go-plugin is NOT needed. This phase delivers in-tree integration management with instance lifecycle and config reload.

</domain>

<decisions>
## Implementation Decisions

### Instance configuration
- Integration code lives in Spectre codebase (in-tree, not external binaries)
- Config file defines instances with unique names
- Each instance has its own connection details
- Multiple instances of same integration type supported (e.g., two VictoriaLogs: prod + staging)

### Lifecycle & health
- Failed connections mark instance as **degraded** (not crash server)
- Degraded instances stay registered but MCP tools return errors for that instance
- **Auto-recovery**: periodic health checks, auto-mark healthy when backend responds
- **Full isolation**: errors in instance A never affect instance B
- **Graceful shutdown** with timeout: wait for in-flight requests, then force stop

### Config reload
- **File watch** using fsnotify triggers reload
- **Full restart** on config change: all instances restart to pick up new state
- **Reject invalid config**: log error, keep running with previous valid config
- **Short debounce** (500ms-1s) to handle editor save storms

### Config versioning
- Config file has explicit **schema version** field
- **In-memory migration**: use migrated config at runtime, don't modify file on disk
- **Support N versions back**: support last 2-3 config versions, deprecate older ones

### Claude's Discretion
- Exact health check interval
- Graceful shutdown timeout duration
- Precise debounce timing
- Migration implementation details

</decisions>

<specifics>
## Specific Ideas

No specific requirements — standard Go patterns and Koanf for config management.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 01-plugin-infrastructure-foundation*
*Context gathered: 2026-01-21*
