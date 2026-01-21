# Phase 6: Consolidated Server & Integration Manager - Context

**Gathered:** 2026-01-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Single server binary that serves REST API, UI, and MCP on port 8080 with in-process integration manager. Replaces the current MCP sidecar architecture. Service layer extraction is Phase 7.

</domain>

<decisions>
## Implementation Decisions

### MCP Endpoint Design
- Use SSE (Server-Sent Events) transport, not WebSocket
- No authentication required (matches current REST API — relies on network-level security)
- Versioned URL path: `/v1/mcp` (future-proofs for protocol changes)
- CORS enabled for browser-based MCP clients

### Transport Switching
- HTTP server always runs by default
- `--stdio` flag adds stdio MCP alongside HTTP (not mutually exclusive)
- MCP endpoint is always on — no `--no-mcp` flag
- Logs tagged by transport source: `[http-mcp]`, `[stdio-mcp]`, `[rest]` for debugging

### Integration Lifecycle
- Integrations initialize AFTER server starts listening (fast startup, tools appear gradually)
- Server sends MCP notifications when tools change (not polling-based discovery)
- Failed integrations retry with exponential backoff in background
- Config hot-reload debounced at 500ms (wait for changes to settle)

### Shutdown & Signals
- 10 second graceful shutdown timeout
- Verbose shutdown logging: "Closing MCP...", "Stopping integrations...", etc.
- Force exit after timeout (ensures clean container restarts)

### Claude's Discretion
- Shutdown order (stop accepting → drain → close integrations, or other)
- Exact exponential backoff parameters for integration retry
- SSE implementation details (heartbeat interval, reconnection hints)

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches for SSE, signal handling, and integration management patterns.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 06-consolidated-server*
*Context gathered: 2026-01-21*
