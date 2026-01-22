# Spectre

## What This Is

A Kubernetes observability platform with an MCP server for AI assistants. Provides timeline-based event exploration, graph-based reasoning (FalkorDB), and pluggable integrations (VictoriaLogs). AI assistants can explore logs progressively: overview → patterns → raw logs.

## Core Value

Enable AI assistants to understand what's happening in Kubernetes clusters through a unified MCP interface—timeline queries, graph traversal, and log exploration in one server.

## Current State (v1.2 Shipped)

**Shipped 2026-01-22:**
- Logz.io as second log backend with 3 MCP tools (overview, logs, patterns)
- SecretWatcher with SharedInformerFactory for Kubernetes-native secret hot-reload
- Multi-region API support (US, EU, UK, AU, CA) with X-API-TOKEN authentication
- UI configuration form with region selector and SecretRef fields
- Helm chart documentation for Secret mounting with rotation workflow

**Cumulative stats:** 14 phases, 39 plans, 73 requirements, ~125k LOC (Go + TypeScript)

## Previous State (v1.1 Shipped)

**Shipped 2026-01-21:**
- Single-port deployment with REST API, UI, and MCP on port 8080 (/v1/mcp endpoint)
- Service layer extracted: TimelineService, GraphService, MetadataService, SearchService
- MCP tools call services directly in-process (no HTTP self-calls)
- 14,676 lines of dead code removed (standalone commands and internal/agent package)
- Helm chart simplified for single-container deployment
- E2E tests validated for consolidated architecture

**Cumulative stats:** 9 phases, 31 plans, 52 requirements, ~121k LOC (Go + TypeScript)

<details>
<summary>v1 Shipped Features (2026-01-21)</summary>

- Plugin infrastructure with factory registry, config hot-reload, lifecycle management
- REST API + React UI for integration configuration
- VictoriaLogs integration with LogsQL client and backpressure pipeline
- Log template mining using Drain algorithm with namespace-scoped storage
- Three progressive disclosure MCP tools: overview, patterns, logs

**Stats:** 5 phases, 19 plans, 31 requirements, ~17,850 LOC

</details>

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
- ✓ Single-port server serving REST, UI, and MCP at :8080 — v1.1
- ✓ MCP endpoint at /v1/mcp path on main server — v1.1
- ✓ Shared service layer for timeline/graph queries — v1.1
- ✓ In-process MCP tool execution (no HTTP self-calls) — v1.1
- ✓ Remove `mcp` command from CLI — v1.1
- ✓ Remove MCP sidecar from Helm chart deployment — v1.1
- ✓ Integration manager works with consolidated server — v1.1
- ✓ E2E tests updated for single-server architecture — v1.1
- ✓ Logz.io integration with Elasticsearch DSL query client — v1.2
- ✓ Secret management infrastructure (Kubernetes-native SecretWatcher) — v1.2
- ✓ Logz.io progressive disclosure tools (overview, patterns, logs) — v1.2
- ✓ Multi-region API endpoint support (US, EU, UK, AU, CA) — v1.2
- ✓ UI for Logz.io configuration (region selector, SecretRef fields) — v1.2
- ✓ Helm chart updates for secret mounting (extraVolumes example) — v1.2

### Active

(No active requirements — ready for next milestone)

### Out of Scope

- Grafana Cloud integration — defer to later milestone
- VictoriaMetrics (metrics) integration — defer to later milestone
- Long-term pattern baseline tracking — keep simple, compare to previous time window only
- Authentication for VictoriaLogs — no auth needed (just base URL)
- Mobile UI — web-first
- Standalone MCP server command — consolidated architecture is the deployment model

## Context

**Current codebase:**
- Consolidated server at `internal/apiserver/` serving REST, UI, and MCP on port 8080
- Service layer at `internal/api/` — TimelineService, GraphService, MetadataService, SearchService
- MCP server at `internal/mcp/server.go` with StreamableHTTP at /v1/mcp
- MCP tools at `internal/mcp/tools/` use services directly (no HTTP)
- Plugin system at `internal/integration/` with factory registry and lifecycle manager
- VictoriaLogs client at `internal/integration/victorialogs/`
- Log processing at `internal/logprocessing/` (Drain algorithm, template storage)
- Config management at `internal/config/` with hot-reload via fsnotify
- REST API handlers at `internal/api/handlers/`
- React UI at `ui/src/pages/`
- Go 1.24+, TypeScript 5.8, React 19

**Architecture (v1.1):**
- Single `spectre server` command serves everything on port 8080
- MCP tools call TimelineService/GraphService directly in-process
- No standalone MCP/agent commands (removed in v1.1)
- Helm chart deploys single container

**Progressive disclosure model (implemented):**
1. **Overview** — error/warning counts by namespace (QueryAggregation with level filter)
2. **Patterns** — log templates via Drain with novelty detection (compare to previous window)
3. **Logs** — raw logs with limit enforcement (max 500)

## Constraints

- **Tech stack**: Go backend, TypeScript/React frontend — established patterns
- **No auth for VictoriaLogs**: VictoriaLogs uses no authentication, just base URL
- **API token for Logz.io**: Requires X-API-TOKEN header, Pro/Enterprise plan only
- **Client-side mining**: Template mining happens in Go (not dependent on log store features)
- **Reusability**: Log processing package is integration-agnostic
- **Logz.io rate limit**: 100 concurrent API requests per account
- **Logz.io result limits**: 1,000 aggregated results, 10,000 non-aggregated results per query

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
| Single-port consolidated server (v1.1) | Simpler deployment, single Helm container, no sidecar coordination | ✓ Good |
| MCP endpoint at /v1/mcp (v1.1) | API versioning consistency with existing /api/v1/* routes | ✓ Good |
| Service layer shared by REST and MCP (v1.1) | Eliminates code duplication, single source of truth for business logic | ✓ Good |
| Delete HTTP client entirely (v1.1) | Service-only architecture is cleaner, HTTP self-calls were wasteful | ✓ Good |
| StreamableHTTP stateless mode (v1.1) | Compatibility with MCP clients that don't manage sessions | ✓ Good |
| SharedInformerFactory for secrets (v1.2) | Kubernetes best practice, auto-reconnection, namespace-scoped | ✓ Good |
| X-API-TOKEN header for Logz.io (v1.2) | Per Logz.io API spec, not Bearer token | ✓ Good |
| VictoriaLogs parity for Logz.io tools (v1.2) | Consistent AI experience across backends | ✓ Good |
| Region selector (not freeform URL) (v1.2) | Prevents misconfiguration, maps to regional endpoints | ✓ Good |
| SecretRef split (Name + Key) (v1.2) | Clearer UX than single reference string | ✓ Good |

## Tech Debt

- DateAdded field not persisted in integration config (uses time.Now() on each GET request)
- GET /{name} endpoint available but unused by UI (uses list endpoint instead)

---
*Last updated: 2026-01-22 after v1.2 milestone shipped*
