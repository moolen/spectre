# Spectre MCP Plugin System + VictoriaLogs Integration

## What This Is

A plugin system for Spectre's MCP server that enables dynamic loading of observability integrations (Logz.io, VictoriaMetrics, Grafana Cloud, etc.). Each integration provides its own MCP tools. The first integration is VictoriaLogs, implementing a progressive disclosure approach for log exploration: global overview → aggregated view → full logs.

## Core Value

Enable AI assistants to explore logs progressively—starting from high-level signals (errors, panics, timeouts) aggregated by namespace, then drilling into patterns, and finally viewing raw logs only when context is narrow.

## Requirements

### Validated

- ✓ MCP server exists with tool registration — existing
- ✓ REST API backend exists — existing
- ✓ React UI exists for configuration — existing
- ✓ FalkorDB integration pattern established — existing

### Active

- [ ] Plugin system for MCP integrations
- [ ] Config hot-reload in MCP server
- [ ] REST API endpoints for integration management
- [ ] UI for enabling/configuring integrations
- [ ] VictoriaLogs integration with progressive disclosure
- [ ] Log template mining package (reusable across integrations)
- [ ] Canonical template storage in MCP

### Out of Scope

- Logz.io integration — defer to later milestone
- Grafana Cloud integration — defer to later milestone
- VictoriaMetrics (metrics) integration — defer to later milestone
- Long-term pattern baseline tracking — keep simple, compare to previous time window only
- Authentication for VictoriaLogs — no auth needed (just base URL)
- Mobile UI — web-first

## Context

**Existing codebase:**
- MCP server at `internal/mcp/` with tool registration pattern
- REST API at `internal/api/` using Connect/gRPC
- React UI at `ui/src/` with existing configuration patterns
- Go 1.24+, TypeScript 5.8, React 19

**VictoriaLogs API:**
- HTTP API documented at https://docs.victoriametrics.com/victorialogs/querying/#http-api
- No authentication required, just base URL

**Progressive disclosure model:**
1. **Global Overview** — errors/panics/timeouts aggregated by namespace over time (default: last 60min, min: 15min)
2. **Aggregated View** — log templates via client-side mining (Drain/IPLoM/Spell), highlight high-volume patterns and new patterns (vs previous window)
3. **Full Logs** — raw logs once scope is narrowed

**Template mining considerations:**
- Algorithm research needed (Drain vs IPLoM vs Spell)
- Stable template hashing: normalize (lowercase, remove numbers/UUIDs/IPs) → hash
- Store canonical templates in MCP for cross-client consistency
- Sampling for high-volume namespaces
- Time-window batching

**Integration config flow:**
- User enables/configures via UI
- UI sends to REST API
- API persists to disk
- MCP server watches/reloads config dynamically
- Tools become available to AI assistants

## Constraints

- **Tech stack**: Go backend, TypeScript/React frontend — established patterns
- **No auth**: VictoriaLogs uses no authentication, just base URL
- **Client-side mining**: Template mining happens in Go (not dependent on log store features)
- **Reusability**: Log processing package must be integration-agnostic

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Client-side template mining | Independence from log store features, works across integrations | — Pending |
| Previous-window pattern comparison | Simplicity over long-term baseline tracking | — Pending |
| Config via REST API + disk | Matches existing architecture, enables hot-reload | — Pending |
| Template algorithm TBD | Need to research Drain vs IPLoM vs Spell tradeoffs | — Pending |

---
*Last updated: 2026-01-20 after initialization*
