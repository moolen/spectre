# Roadmap: Spectre

## Milestones

- ✅ **v1.0 MCP Plugin System + VictoriaLogs** - Phases 1-5 (shipped 2026-01-21)
- ✅ **v1.1 Server Consolidation** - Phases 6-9 (shipped 2026-01-21)
- ✅ **v1.2 Logz.io Integration + Secret Management** - Phases 10-14 (shipped 2026-01-22)
- 🚧 **v1.3 Grafana Metrics Integration** - Phases 15-19 (in progress)

## Phases

<details>
<summary>✅ v1.0 MCP Plugin System + VictoriaLogs (Phases 1-5) - SHIPPED 2026-01-21</summary>

See `.planning/milestones/v1-ROADMAP.md` for details.

**Stats:** 5 phases, 19 plans, 31 requirements

</details>

<details>
<summary>✅ v1.1 Server Consolidation (Phases 6-9) - SHIPPED 2026-01-21</summary>

See `.planning/milestones/v1.1-ROADMAP.md` for details.

**Stats:** 4 phases, 12 plans, 21 requirements

</details>

<details>
<summary>✅ v1.2 Logz.io Integration + Secret Management (Phases 10-14) - SHIPPED 2026-01-22</summary>

See `.planning/milestones/v1.2-ROADMAP.md` for details.

**Stats:** 5 phases, 8 plans, 21 requirements

</details>

### 🚧 v1.3 Grafana Metrics Integration (In Progress)

**Milestone Goal:** Use Grafana dashboards as structured operational knowledge so Spectre can detect high-level anomalies, progressively drill down, and reason about services, clusters, and metrics.

#### Phase 15: Foundation - Grafana API Client & Graph Schema
**Goal**: Grafana integration can authenticate, retrieve dashboards, and store structure in FalkorDB graph.
**Depends on**: Nothing (first phase of v1.3)
**Requirements**: FOUN-01, FOUN-02, FOUN-03, FOUN-05, FOUN-06, GRPH-01, GRPH-07, UICF-01, UICF-02, UICF-03
**Success Criteria** (what must be TRUE):
  1. User can configure Grafana URL and API token via UI form
  2. Integration validates connection on save with health check
  3. GrafanaClient can authenticate to both Cloud and self-hosted instances
  4. GrafanaClient can list all dashboards via search API
  5. FalkorDB schema includes Dashboard nodes with indexes on uid
**Plans**: 3 plans

Plans:
- [ ] 15-01-PLAN.md — Grafana API client backend with SecretWatcher integration
- [ ] 15-02-PLAN.md — FalkorDB Dashboard node schema with named graph support
- [ ] 15-03-PLAN.md — UI configuration form and test connection handler

#### Phase 16: Ingestion Pipeline - Dashboard Sync & PromQL Parsing
**Goal**: Dashboards are ingested incrementally with full semantic structure extracted to graph.
**Depends on**: Phase 15
**Requirements**: FOUN-04, GRPH-02, GRPH-03, GRPH-04, GRPH-06, PROM-01, PROM-02, PROM-03, PROM-04, PROM-05, PROM-06, UICF-05
**Success Criteria** (what must be TRUE):
  1. DashboardSyncer detects changed dashboards via version field (incremental sync)
  2. PromQL parser extracts metric names, label selectors, and aggregation functions
  3. Graph contains Dashboard→Panel→Query→Metric relationships with CONTAINS/QUERIES/USES edges
  4. UI displays sync status and last sync time
  5. Parser handles Grafana variable syntax as passthrough (preserves $var, [[var]])
**Plans**: TBD

Plans:
- [ ] 16-01: TBD

#### Phase 17: Semantic Layer - Service Inference & Dashboard Hierarchy
**Goal**: Dashboards are classified by hierarchy level, services are inferred from metrics, and variables are classified by type.
**Depends on**: Phase 16
**Requirements**: GRPH-05, SERV-01, SERV-02, SERV-03, SERV-04, HIER-01, HIER-02, HIER-03, HIER-04, VARB-01, VARB-02, VARB-03, UICF-04
**Success Criteria** (what must be TRUE):
  1. Service nodes are created from PromQL label extraction (job, service, app, namespace, cluster)
  2. Metric→Service relationships exist in graph (TRACKS edges)
  3. Dashboards are classified as overview, drill-down, or detail based on tags
  4. Variables are classified as scoping (cluster/region), entity (service/namespace), or detail (pod/instance)
  5. UI allows configuration of hierarchy mapping fallback (when tags not present)
**Plans**: TBD

Plans:
- [ ] 17-01: TBD

#### Phase 18: Query Execution & MCP Tools Foundation
**Goal**: AI can execute Grafana queries and discover dashboards through three MCP tools.
**Depends on**: Phase 17
**Requirements**: VARB-04, VARB-05, EXEC-01, EXEC-02, EXEC-03, EXEC-04, TOOL-01, TOOL-04, TOOL-05, TOOL-06, TOOL-07, TOOL-08, TOOL-09
**Success Criteria** (what must be TRUE):
  1. GrafanaQueryService executes PromQL via Grafana /api/ds/query endpoint
  2. Query service handles time range parameters (from, to, interval) and formats time series response
  3. MCP tool `grafana_{name}_metrics_overview` executes overview dashboards only
  4. MCP tool `grafana_{name}_metrics_aggregated` focuses on specified service or cluster
  5. MCP tool `grafana_{name}_metrics_details` executes full dashboard with all panels
  6. All tools accept scoping variables (cluster, region) as parameters and pass to Grafana API
**Plans**: TBD

Plans:
- [ ] 18-01: TBD

#### Phase 19: Anomaly Detection & Progressive Disclosure
**Goal**: AI can detect anomalies vs 7-day baseline with severity ranking and progressively disclose from overview to details.
**Depends on**: Phase 18
**Requirements**: TOOL-02, TOOL-03, ANOM-01, ANOM-02, ANOM-03, ANOM-04, ANOM-05, ANOM-06
**Success Criteria** (what must be TRUE):
  1. AnomalyService computes baseline from 7-day historical data with time-of-day matching
  2. Anomalies are detected using z-score comparison against baseline
  3. Anomalies are classified by severity (info, warning, critical)
  4. MCP tool `grafana_{name}_metrics_overview` returns ranked anomalies with severity
  5. Anomaly detection handles missing metrics gracefully (checks scrape status, uses fallback)
  6. Baselines are cached in graph with 1-hour TTL for performance
**Plans**: TBD

Plans:
- [ ] 19-01: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 15 → 16 → 17 → 18 → 19

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 15. Foundation | 0/3 | Ready to execute | - |
| 16. Ingestion Pipeline | 0/TBD | Not started | - |
| 17. Semantic Layer | 0/TBD | Not started | - |
| 18. Query Execution & MCP Tools | 0/TBD | Not started | - |
| 19. Anomaly Detection | 0/TBD | Not started | - |

---
*v1.3 roadmap created: 2026-01-22*
