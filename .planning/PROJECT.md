# Spectre

## What This Is

A Kubernetes observability platform with an MCP server for AI assistants. Provides timeline-based event exploration, graph-based reasoning (FalkorDB), and pluggable integrations (VictoriaLogs, Logz.io, Grafana). AI assistants can explore logs progressively, use Grafana dashboards as structured operational knowledge, and investigate incidents systematically through signal intelligence.

## Core Value

Enable AI assistants to understand what's happening in Kubernetes clusters through a unified MCP interface—timeline queries, graph traversal, log exploration, metrics analysis, and incident investigation in one server.

## Current State: v1.5 Shipped

**Cumulative stats:** 26 phases, 83 plans, 207 requirements, ~164k LOC (Go + TypeScript)

**Available capabilities:**
- Timeline-based Kubernetes event exploration with FalkorDB graph
- Log exploration via VictoriaLogs and Logz.io with progressive disclosure
- Grafana metrics integration with dashboard sync, anomaly detection, and 3 MCP tools
- Grafana alerts integration with state tracking, flappiness analysis, and 3 MCP tools
- Observatory signal intelligence with 8 MCP tools for incident investigation

## Previous State: v1.5 Observatory (Shipped 2026-01-30)

**Shipped 2026-01-30:**
- Signal anchors with 7-role taxonomy (Availability, Latency, Errors, Traffic, Saturation, Churn, Novelty)
- 5-layer classification with confidence decay (0.95 → 0.85-0.9 → 0.7-0.8 → 0.5 → 0)
- Dashboard quality scoring (freshness, alerting, ownership, completeness) with alert boost
- Rolling baseline statistics using gonum/stat (median, P50/P90/P99, stddev)
- Hybrid anomaly detection (z-score + percentile) with sigmoid normalization, alert override
- Hierarchical MAX aggregation (signals → workloads → namespaces → clusters)
- 8 Observatory MCP tools: status, changes, scope, signals, signal_detail, compare, explain, evidence

**Total MCP tools:** 14 Grafana tools (3 metrics + 3 alerts + 8 observatory)

<details>
<summary>v1.4 Grafana Alerts Integration (Shipped 2026-01-23)</summary>

- Alert rule sync via Grafana Alerting API (incremental, version-based)
- Alert nodes in FalkorDB linked to Metrics/Services via PromQL extraction
- STATE_TRANSITION self-edges for 7-day timeline with TTL-based retention
- Flappiness detection with exponential scaling (0.7 threshold)
- Multi-label categorization: onset (NEW/RECENT/CHRONIC) + pattern (flapping/stable)
- AlertAnalysisService with 1000-entry LRU cache (5-minute TTL)
- `grafana_{name}_alerts_overview` — firing/pending counts by severity with flappiness indicators
- `grafana_{name}_alerts_aggregated` — specific alerts with 1h state timelines [F F N N]
- `grafana_{name}_alerts_details` — full 7-day state history with rule definition

**Stats:** 4 phases, 10 plans, 22 requirements

</details>

<details>
<summary>v1.3 Grafana Metrics Integration (Shipped 2026-01-23)</summary>

- Grafana dashboard ingestion via API (both Cloud and self-hosted)
- Full semantic graph storage in FalkorDB (dashboards→panels→queries→metrics→services)
- Dashboard hierarchy (overview/drill-down/detail) via Grafana tags + config fallback
- Best-effort PromQL parsing for metric names, labels, and variable classification
- Service inference from metric labels (job, service, app)
- Anomaly detection with 7-day historical baseline (z-score based, time-of-day matched)
- Three MCP tools: metrics_overview, metrics_aggregated, metrics_details
- UI configuration form for Grafana connection (URL, API token, hierarchy mapping)

**Stats:** 5 phases, 17 plans, 51 requirements

</details>

<details>
<summary>v1.2 Logz.io Integration + Secret Management (Shipped 2026-01-22)</summary>

- Logz.io as second log backend with 3 MCP tools (overview, logs, patterns)
- SecretWatcher with SharedInformerFactory for Kubernetes-native secret hot-reload
- Multi-region API support (US, EU, UK, AU, CA) with X-API-TOKEN authentication
- UI configuration form with region selector and SecretRef fields
- Helm chart documentation for Secret mounting with rotation workflow

**Stats:** 5 phases, 8 plans, 21 requirements

</details>

<details>
<summary>v1.1 Server Consolidation (Shipped 2026-01-21)</summary>

- Single-port deployment with REST API, UI, and MCP on port 8080 (/v1/mcp endpoint)
- Service layer extracted: TimelineService, GraphService, MetadataService, SearchService
- MCP tools call services directly in-process (no HTTP self-calls)
- 14,676 lines of dead code removed (standalone commands and internal/agent package)
- Helm chart simplified for single-container deployment
- E2E tests validated for consolidated architecture

**Stats:** 4 phases, 12 plans, 21 requirements

</details>

<details>
<summary>v1.0 MCP Plugin System + VictoriaLogs (Shipped 2026-01-21)</summary>

- Plugin infrastructure with factory registry, config hot-reload, lifecycle management
- REST API + React UI for integration configuration
- VictoriaLogs integration with LogsQL client and backpressure pipeline
- Log template mining using Drain algorithm with namespace-scoped storage
- Three progressive disclosure MCP tools: overview, patterns, logs

**Stats:** 5 phases, 19 plans, 31 requirements

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
- ✓ Grafana API client for dashboard ingestion (both Cloud and self-hosted) — v1.3
- ✓ FalkorDB graph schema for dashboards, panels, queries, metrics, services — v1.3
- ✓ Dashboard hierarchy support (overview/drill-down/detail levels) — v1.3
- ✓ PromQL parser for metric extraction (best-effort) — v1.3
- ✓ Variable classification (scoping vs entity vs detail) — v1.3
- ✓ Service inference from metric labels — v1.3
- ✓ Anomaly detection with 7-day historical baseline — v1.3
- ✓ MCP tool: metrics_overview (overview dashboards, ranked anomalies) — v1.3
- ✓ MCP tool: metrics_aggregated (service/cluster focus, correlations) — v1.3
- ✓ MCP tool: metrics_details (full dashboard, deep expansion) — v1.3
- ✓ UI form for Grafana configuration (URL, API token, hierarchy mapping) — v1.3
- ✓ Alert rule sync via Grafana Alerting API (incremental, version-based) — v1.4
- ✓ Alert nodes in FalkorDB linked to existing Metrics/Services via PromQL extraction — v1.4
- ✓ Alert state timeline storage (STATE_TRANSITION edges with 7-day TTL) — v1.4
- ✓ Flappiness detection with exponential scaling and historical baseline — v1.4
- ✓ MCP tool: alerts_overview (firing/pending counts by severity with flappiness indicators) — v1.4
- ✓ MCP tool: alerts_aggregated (specific alerts with 1h state timelines) — v1.4
- ✓ MCP tool: alerts_details (full 7-day state history with rule definition) — v1.4
- ✓ Signal anchors linking metrics to roles to workloads — v1.5
- ✓ 7-role classification taxonomy (Availability, Latency, Errors, Traffic, Saturation, Churn, Novelty) — v1.5
- ✓ Dashboard quality scoring (freshness, alerting, ownership, completeness) — v1.5
- ✓ Rolling baseline statistics per signal (median, P50/P90/P99, stddev) — v1.5
- ✓ Hybrid anomaly detection (z-score + percentile) with alert override — v1.5
- ✓ Hierarchical anomaly aggregation (signals → workloads → namespaces → clusters) — v1.5
- ✓ 8 Observatory MCP tools for progressive disclosure incident investigation — v1.5

### Out of Scope

- VictoriaMetrics (metrics) integration — defer to later milestone
- Long-term pattern baseline tracking for logs — keep simple, compare to previous time window only
- Authentication for VictoriaLogs — no auth needed (just base URL)
- Mobile UI — web-first
- Standalone MCP server command — consolidated architecture is the deployment model
- Metric value storage — query Grafana on-demand instead of storing time-series locally
- Direct Prometheus/Mimir queries — use Grafana API as proxy for simpler auth
- ML-based role classification — keyword heuristics sufficient, ML deferred to v2
- Real-time streaming anomaly detection — polling-based for v1.5

## Context

**Current codebase:**
- Consolidated server at `internal/apiserver/` serving REST, UI, and MCP on port 8080
- Service layer at `internal/api/` — TimelineService, GraphService, MetadataService, SearchService
- MCP server at `internal/mcp/server.go` with StreamableHTTP at /v1/mcp
- MCP tools at `internal/mcp/tools/` use services directly (no HTTP)
- Plugin system at `internal/integration/` with factory registry and lifecycle manager
- VictoriaLogs client at `internal/integration/victorialogs/`
- Grafana integration at `internal/integration/grafana/` with dashboard, metrics, alerts, and observatory
- Log processing at `internal/logprocessing/` (Drain algorithm, template storage)
- Config management at `internal/config/` with hot-reload via fsnotify
- REST API handlers at `internal/api/handlers/`
- React UI at `ui/src/pages/`
- Go 1.24+, TypeScript 5.8, React 19

**Architecture (v1.5):**
- Single `spectre server` command serves everything on port 8080
- MCP tools call TimelineService/GraphService/ObservatoryService directly in-process
- Grafana integration provides 14 MCP tools (3 metrics + 3 alerts + 8 observatory)
- Observatory uses FalkorDB for signal anchors and baselines with TTL-based cleanup

**Progressive disclosure model:**
1. **Overview** — cluster/namespace anomaly summary (Orient stage)
2. **Scope** — namespace/workload focus with ranked signals (Narrow stage)
3. **Detail** — signal baseline, anomaly score, evidence (Investigate/Verify stages)

## Constraints

- **Tech stack**: Go backend, TypeScript/React frontend — established patterns
- **No auth for VictoriaLogs**: VictoriaLogs uses no authentication, just base URL
- **API token for Logz.io**: Requires X-API-TOKEN header, Pro/Enterprise plan only
- **Client-side mining**: Template mining happens in Go (not dependent on log store features)
- **Reusability**: Log processing package is integration-agnostic
- **Logz.io rate limit**: 100 concurrent API requests per account
- **Logz.io result limits**: 1,000 aggregated results, 10,000 non-aggregated results per query
- **Grafana API token**: Requires Bearer token with dashboard read permissions
- **PromQL parsing best-effort**: Complex expressions may not fully parse, extract what's possible
- **Graph storage for structure only**: FalkorDB stores dashboard structure, not metric values
- **Baseline collection rate limit**: 10 req/sec forward, 2 req/sec backfill

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
| Query via Grafana API (v1.3) | Simpler auth, variable handling vs direct Prometheus | ✓ Good |
| No metric storage (v1.3) | Query historical ranges on-demand via Grafana | ✓ Good |
| Dashboards as fuzzy signals (v1.3) | AI treats structure as intent, not strict truth | ✓ Good |
| Progressive disclosure for metrics (v1.3) | Overview → aggregated → details pattern | ✓ Good |
| Z-score with time-of-day matching (v1.3) | Better anomaly detection vs simple rolling average | ✓ Good |
| Error metrics use lower thresholds (v1.3) | Errors deserve attention at 2σ vs 3σ for normal | ✓ Good |
| Baseline cache in graph with TTL (v1.3) | Performance optimization, 1-hour refresh | ✓ Good |
| Self-edge pattern for state transitions (v1.4) | (Alert)-[STATE_TRANSITION]->(Alert) simpler than separate node | ✓ Good |
| 7-day TTL via expires_at timestamp (v1.4) | Query-time filtering, no cleanup job needed | ✓ Good |
| 5-minute state sync interval (v1.4) | More responsive than 1-hour rule sync | ✓ Good |
| Exponential flappiness scaling (v1.4) | Penalizes rapid transitions more than linear | ✓ Good |
| LOCF interpolation for timelines (v1.4) | Fills gaps realistically in state buckets | ✓ Good |
| Optional filter parameters (v1.4) | Maximum flexibility for AI alert queries | ✓ Good |
| 10-minute timeline buckets (v1.4) | Compact notation [F F N N], 6 buckets per hour | ✓ Good |
| Layered classification with confidence decay (v1.5) | 5 layers from hardcoded to unknown | ✓ Good |
| Quality scoring with alert boost (v1.5) | +0.2 for dashboards with alerts | ✓ Good |
| Composite key for SignalAnchor (v1.5) | metric + namespace + workload + integration | ✓ Good |
| Z-score sigmoid normalization (v1.5) | Maps unbounded to 0-1 range | ✓ Good |
| Hybrid MAX aggregation (v1.5) | Either z-score or percentile can flag anomaly | ✓ Good |
| Alert firing override (v1.5) | Human decision takes precedence, score=1.0 | ✓ Good |
| Hierarchical MAX aggregation (v1.5) | Worst signal bubbles up through hierarchy | ✓ Good |
| Progressive disclosure for incidents (v1.5) | Orient → Narrow → Investigate → Hypothesize → Verify | ✓ Good |

## Tech Debt

- DateAdded field not persisted in integration config (uses time.Now() on each GET request)
- GET /{name} endpoint available but unused by UI (uses list endpoint instead)
- TestComputeDashboardQuality_Freshness has time-dependent failures
- Quality scoring stubs (getAlertRuleCount, getViewsLast30Days return 0)
- Dashboard metadata extraction TODOs (updated time, folder title, description)
- QueryService stub methods (FetchCurrentValue, FetchHistoricalValue use baseline fallback)

---
*Last updated: 2026-01-30 after v1.5 Observatory milestone shipped*
