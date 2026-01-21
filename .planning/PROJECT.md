# Spectre MCP Plugin System + VictoriaLogs Integration

## What This Is

A plugin system for Spectre's MCP server that enables dynamic loading of observability integrations. The first integration is VictoriaLogs, implementing progressive disclosure for log exploration: overview (severity counts) → patterns (template mining with novelty detection) → raw logs.

## Core Value

Enable AI assistants to explore logs progressively—starting from high-level signals aggregated by namespace, then drilling into patterns with novelty detection, and finally viewing raw logs only when context is narrow.

## Current State (v1 Shipped)

**Shipped 2026-01-21:**
- Plugin infrastructure with factory registry, config hot-reload, lifecycle management
- REST API + React UI for integration configuration
- VictoriaLogs integration with LogsQL client and backpressure pipeline
- Log template mining using Drain algorithm with namespace-scoped storage
- Three progressive disclosure MCP tools: overview, patterns, logs

**Stats:** 5 phases, 19 plans, 31 requirements, ~17,850 LOC (Go + TypeScript)

## Requirements

### Validated

- ✓ MCP server exists with tool registration — existing
- ✓ REST API backend exists — existing
- ✓ React UI exists for configuration — existing
- ✓ FalkorDB integration pattern established — existing
- ✓ Plugin system for MCP integrations — v1
- ✓ Config hot-reload in MCP server — v1
- ✓ REST API endpoints for integration management — v1
- ✓ UI for enabling/configuring integrations — v1
- ✓ VictoriaLogs integration with progressive disclosure — v1
- ✓ Log template mining package (reusable across integrations) — v1
- ✓ Canonical template storage in MCP — v1

### Active

(None — milestone complete, new requirements to be defined in next milestone)

### Out of Scope

- Logz.io integration — defer to later milestone
- Grafana Cloud integration — defer to later milestone
- VictoriaMetrics (metrics) integration — defer to later milestone
- Long-term pattern baseline tracking — keep simple, compare to previous time window only
- Authentication for VictoriaLogs — no auth needed (just base URL)
- Mobile UI — web-first

## Context

**Current codebase:**
- Plugin system at `internal/integration/` with factory registry and lifecycle manager
- VictoriaLogs client at `internal/integration/victorialogs/`
- Log processing at `internal/logprocessing/` (Drain algorithm, template storage)
- MCP tools at `internal/integration/victorialogs/tools_*.go`
- Config management at `internal/config/` with hot-reload via fsnotify
- REST API at `internal/api/handlers/integration_config_handler.go`
- React UI at `ui/src/pages/IntegrationsPage.tsx`
- Go 1.24+, TypeScript 5.8, React 19

**VictoriaLogs API:**
- HTTP API documented at https://docs.victoriametrics.com/victorialogs/querying/#http-api
- No authentication required, just base URL

**Progressive disclosure model (implemented):**
1. **Overview** — error/warning counts by namespace (QueryAggregation with level filter)
2. **Patterns** — log templates via Drain with novelty detection (compare to previous window)
3. **Logs** — raw logs with limit enforcement (max 500)

**Template mining (implemented):**
- Drain algorithm via github.com/faceair/drain
- SHA-256 hashing for stable template IDs
- Namespace-scoped storage with periodic persistence
- Rebalancing with count-based pruning and similarity-based auto-merge

## Constraints

- **Tech stack**: Go backend, TypeScript/React frontend — established patterns
- **No auth**: VictoriaLogs uses no authentication, just base URL
- **Client-side mining**: Template mining happens in Go (not dependent on log store features)
- **Reusability**: Log processing package is integration-agnostic

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| In-tree integrations (not external plugins) | Simplifies deployment, eliminates version compatibility issues | ✓ Good |
| Client-side template mining with Drain | Independence from log store features, works across integrations | ✓ Good |
| Previous-window pattern comparison | Simplicity over long-term baseline tracking | ✓ Good |
| Config via REST API + disk | Matches existing architecture, enables hot-reload | ✓ Good |
| Drain algorithm (not IPLoM/Spell) | Research showed Drain is industry standard, O(log n) matching | ✓ Good |
| Factory registry pattern | Compile-time discovery via init(), clean lifecycle | ✓ Good |
| Atomic YAML writes (temp-then-rename) | Prevents config corruption on crashes | ✓ Good |
| Namespace-scoped templates | Multi-tenant support, same pattern in different namespaces has different semantics | ✓ Good |
| Stateless MCP tools | AI passes filters per call, no server-side session state | ✓ Good |

## Tech Debt

- DateAdded field not persisted in integration config (uses time.Now() on each GET request)
- GET /{name} endpoint available but unused by UI (uses list endpoint instead)

---
*Last updated: 2026-01-21 after v1 milestone*
