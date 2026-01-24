# Phase 15: Foundation - Grafana API Client & Graph Schema - Context

**Gathered:** 2026-01-22
**Status:** Ready for planning

<domain>
## Phase Boundary

Build the foundational Grafana integration: UI configuration form, API client that authenticates to Grafana instances (Cloud or self-hosted), health check validation, and FalkorDB graph schema for storing dashboard structure. Each Grafana integration instance gets its own isolated graph database.

</domain>

<decisions>
## Implementation Decisions

### Connection config
- Multiple Grafana integrations allowed, each pointing to a single Grafana endpoint
- Full base URL required (e.g., https://myorg.grafana.net or https://grafana.internal:3000) — no Cloud shorthand
- Integration name is manual entry (used in MCP tool names like grafana_{name}_metrics_overview)
- Minimal form fields: name, URL, API token only — no description field

### Auth handling
- API token via K8s Secret reference only (consistent with Logz.io) — no direct token entry
- Health check validates both dashboard read AND datasource access
- If datasource access fails but dashboard works: warn but allow save (don't block)
- Treat Grafana Cloud as just another URL — no special Cloud-aware handling

### Graph schema design
- Each Grafana integration gets its own separate FalkorDB graph database
- Graph naming convention: `spectre_grafana_{name}` (e.g., spectre_grafana_prod)
- Dashboard nodes store: uid, title, version, tags, folder — enough for sync and hierarchy prep
- When integration is deleted, delete its entire graph database (clean delete)

### Error UX
- Health check errors display inline in the form below the failing field
- Detailed error messages showing HTTP status, Grafana error message, specific failure reason
- Status displayed in existing integrations table status indicator column
- Status updates via existing server push events (SSE)

### Claude's Discretion
- Exact FalkorDB index strategy for Dashboard nodes
- Error message formatting details
- API client retry/timeout configuration

</decisions>

<specifics>
## Specific Ideas

- Follow existing integration patterns (Logz.io, VictoriaLogs) for UI form and SecretWatcher
- Leverage existing SSE push mechanism for status updates
- Integration table already has status indicator — use it

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 15-foundation*
*Context gathered: 2026-01-22*
