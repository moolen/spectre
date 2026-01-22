# Roadmap: Spectre v1.2 Logz.io Integration

## Milestones

- ✅ **v1.0 MCP Plugin System + VictoriaLogs** - Phases 1-5 (shipped 2026-01-21)
- ✅ **v1.1 Server Consolidation** - Phases 6-9 (shipped 2026-01-21)
- 🚧 **v1.2 Logz.io Integration + Secret Management** - Phases 10-14 (in progress)

## Overview

v1.2 adds Logz.io as a second log integration with production-grade secret management infrastructure. The journey: build HTTP client with multi-region support → implement Kubernetes-native secret hot-reload → expose MCP tools for overview/logs → add pattern mining → finalize Helm chart and documentation for Kubernetes deployment.

## Phases

<details>
<summary>✅ v1.0 MCP Plugin System + VictoriaLogs (Phases 1-5) - SHIPPED 2026-01-21</summary>

### Phase 1: Plugin Infrastructure
**Goal**: Enable dynamic integration registration and lifecycle management
**Plans**: 3 plans

Plans:
- [x] 01-01: Factory registry with init-based registration
- [x] 01-02: Lifecycle management with hot-reload
- [x] 01-03: REST API + UI for integration config

### Phase 2: VictoriaLogs Client
**Goal**: Query VictoriaLogs with backpressure pipeline
**Plans**: 2 plans

Plans:
- [x] 02-01: LogsQL HTTP client with batching
- [x] 02-02: Integration tests with real VictoriaLogs

### Phase 3: Log Processing Pipeline
**Goal**: Extract log templates using Drain algorithm
**Plans**: 2 plans

Plans:
- [x] 03-01: Drain algorithm implementation
- [x] 03-02: Namespace-scoped template storage

### Phase 4: VictoriaLogs MCP Tools
**Goal**: Expose progressive disclosure tools for VictoriaLogs
**Plans**: 3 plans

Plans:
- [x] 04-01: Overview tool (severity summary)
- [x] 04-02: Patterns tool (template mining)
- [x] 04-03: Logs tool (raw log retrieval)

### Phase 5: Config Management
**Goal**: Hot-reload integration configuration without restarts
**Plans**: 2 plans

Plans:
- [x] 05-01: fsnotify-based config watcher
- [x] 05-02: Integration lifecycle restart

</details>

<details>
<summary>✅ v1.1 Server Consolidation (Phases 6-9) - SHIPPED 2026-01-21</summary>

### Phase 6: Service Layer Extraction
**Goal**: Shared service layer for REST and MCP
**Plans**: 2 plans

Plans:
- [x] 06-01: Extract TimelineService, GraphService, MetadataService
- [x] 06-02: MCP tools call services directly

### Phase 7: Single-Port Server
**Goal**: Consolidated server on port 8080 with /v1/mcp endpoint
**Plans**: 2 plans

Plans:
- [x] 07-01: MCP StreamableHTTP at /v1/mcp
- [x] 07-02: Remove standalone MCP command

### Phase 8: Helm Chart Update
**Goal**: Single-container deployment with no sidecar
**Plans**: 1 plan

Plans:
- [x] 08-01: Update Helm chart for consolidated server

### Phase 9: E2E Test Validation
**Goal**: E2E tests pass with consolidated architecture
**Plans**: 2 plans

Plans:
- [x] 09-01: Update E2E tests for single server
- [x] 09-02: Remove stdio transport tests

</details>

### 🚧 v1.2 Logz.io Integration + Secret Management (In Progress)

**Milestone Goal:** Add Logz.io as second log backend with Kubernetes-native secret hot-reload and multi-region API support.

#### Phase 10: Logz.io Client Foundation
**Goal**: HTTP client connects to Logz.io Search API with multi-region support and bearer token authentication
**Depends on**: Phase 9 (v1.1 complete)
**Requirements**: LZIO-01, LZIO-02, LZIO-03, LZIO-04, LZIO-05, CONF-01
**Success Criteria** (what must be TRUE):
  1. Client successfully connects to all 5 Logz.io regional endpoints (US, EU, UK, AU, CA)
  2. Health check validates API token with minimal test query
  3. Query builder generates valid Elasticsearch DSL from structured parameters
  4. Client handles rate limits with exponential backoff (returns helpful error on 429)
  5. Integration can be configured with region and API token path in config file
**Plans**: TBD

Plans:
- [ ] 10-01: TBD
- [ ] 10-02: TBD

#### ✅ Phase 11: Secret File Management
**Goal**: Kubernetes-native secret fetching with hot-reload for zero-downtime credential rotation
**Depends on**: Phase 10
**Requirements**: SECR-01, SECR-02, SECR-03, SECR-04, SECR-05
**Success Criteria** (what must be TRUE):
  1. Integration reads API token from Kubernetes Secret at startup (fetches via client-go API, not file mount)
  2. Kubernetes Watch API detects Secret rotation within 2 seconds without pod restart (SharedInformerFactory pattern)
  3. Token updates are thread-safe - concurrent queries continue with old token until update completes
  4. API token values never appear in logs, error messages, or HTTP debug output
  5. Watch re-establishes automatically after disconnection (Kubernetes informer pattern)
**Plans**: 4 plans in 2 waves

Plans:
- [x] 11-01-PLAN.md — SecretWatcher with SharedInformerFactory (Wave 1)
- [x] 11-02-PLAN.md — Config types with SecretRef field (Wave 1)
- [x] 11-03-PLAN.md — Integration wiring and client token auth (Wave 2)
- [x] 11-04-PLAN.md — RBAC setup in Helm chart (Wave 1)

#### ✅ Phase 12: MCP Tools - Overview and Logs
**Goal**: MCP tools expose Logz.io data with progressive disclosure (overview → logs)
**Depends on**: Phase 11
**Requirements**: TOOL-01, TOOL-02, TOOL-04, TOOL-05
**Success Criteria** (what must be TRUE):
  1. `logzio_{name}_overview` returns namespace-level severity summary (errors, warnings, total)
  2. `logzio_{name}_logs` returns raw logs with filters (namespace, pod, container, level, time range)
  3. Tools enforce result limits - max 100 logs to prevent MCP client overload
  4. Tools reject leading wildcard queries with helpful error message (Logz.io API limitation)
  5. MCP tools handle authentication failures gracefully with degraded status
**Plans**: 2 plans in 2 waves

Plans:
- [x] 12-01-PLAN.md — Logzio foundation (bootstrap, client, query builder) (Wave 1)
- [x] 12-02-PLAN.md — MCP tools (overview + logs with progressive disclosure) (Wave 2)

#### ✅ Phase 13: MCP Tools - Patterns
**Goal**: Pattern mining tool exposes log templates with novelty detection
**Depends on**: Phase 12
**Requirements**: TOOL-03
**Success Criteria** (what must be TRUE):
  1. `logzio_{name}_patterns` returns log templates with occurrence counts
  2. Pattern mining reuses existing Drain algorithm from VictoriaLogs (integration-agnostic)
  3. Pattern storage is namespace-scoped (same template in different namespaces tracked separately)
  4. Tool enforces result limits - max 50 templates to prevent MCP client overload
  5. Novelty detection compares current patterns to previous time window
**Plans**: 1 plan in 1 wave

Plans:
- [x] 13-01-PLAN.md — Patterns tool with VictoriaLogs parity (Wave 1)

#### Phase 14: UI and Helm Chart
**Goal**: UI configuration form and Helm chart support for Kubernetes secret mounting
**Depends on**: Phase 13
**Requirements**: CONF-02, CONF-03, HELM-01, HELM-02, HELM-03
**Success Criteria** (what must be TRUE):
  1. UI displays Logz.io configuration form with region selector dropdown (5 regions)
  2. Connection test validates API token before saving configuration (test query to Search API)
  3. Helm values.yaml includes extraVolumes example for mounting Kubernetes Secrets
  4. Documentation covers complete secret rotation workflow (create Secret → mount → rotate → verify)
  5. Example Kubernetes Secret manifest provided in docs with correct file structure
**Plans**: 1 plan in 2 waves

Plans:
- [ ] 14-01-PLAN.md — Logzio UI form and Helm Secret documentation (Wave 1: auto tasks, Wave 2: human-verify checkpoint)

## Progress

**Execution Order:**
Phases execute in numeric order: 10 → 11 → 12 → 13 → 14

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Plugin Infrastructure | v1.0 | 3/3 | Complete | 2026-01-21 |
| 2. VictoriaLogs Client | v1.0 | 2/2 | Complete | 2026-01-21 |
| 3. Log Processing Pipeline | v1.0 | 2/2 | Complete | 2026-01-21 |
| 4. VictoriaLogs MCP Tools | v1.0 | 3/3 | Complete | 2026-01-21 |
| 5. Config Management | v1.0 | 2/2 | Complete | 2026-01-21 |
| 6. Service Layer Extraction | v1.1 | 2/2 | Complete | 2026-01-21 |
| 7. Single-Port Server | v1.1 | 2/2 | Complete | 2026-01-21 |
| 8. Helm Chart Update | v1.1 | 1/1 | Complete | 2026-01-21 |
| 9. E2E Test Validation | v1.1 | 2/2 | Complete | 2026-01-21 |
| 10. Logz.io Client Foundation | v1.2 | 0/TBD | Not started | - |
| 11. Secret File Management | v1.2 | 4/4 | Complete | 2026-01-22 |
| 12. MCP Tools - Overview and Logs | v1.2 | 2/2 | Complete | 2026-01-22 |
| 13. MCP Tools - Patterns | v1.2 | 1/1 | Complete | 2026-01-22 |
| 14. UI and Helm Chart | v1.2 | 0/1 | Not started | - |

---
*Created: 2026-01-22*
*Last updated: 2026-01-22 - Phase 13 complete, Phase 14 planned*
