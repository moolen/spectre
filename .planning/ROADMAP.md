# Roadmap: Spectre

## Milestones

- ✅ **v1.0 MCP Plugin System + VictoriaLogs** - Phases 1-5 (shipped 2026-01-21)
- ✅ **v1.1 Server Consolidation** - Phases 6-9 (shipped 2026-01-21)
- ✅ **v1.2 Logz.io Integration + Secret Management** - Phases 10-14 (shipped 2026-01-22)
- ✅ **v1.3 Grafana Metrics Integration** - Phases 15-19 (shipped 2026-01-23)
- ✅ **v1.4 Grafana Alerts Integration** - Phases 20-23 (shipped 2026-01-23)
- 🚧 **v1.5 Observatory** - Phases 24-26 (in progress)

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

<details>
<summary>✅ v1.3 Grafana Metrics Integration (Phases 15-19) - SHIPPED 2026-01-23</summary>

**Milestone Goal:** Use Grafana dashboards as structured operational knowledge so Spectre can detect high-level anomalies, progressively drill down, and reason about services, clusters, and metrics.

#### ✅ Phase 15: Foundation - Grafana API Client & Graph Schema
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
**Completed**: 2026-01-22

Plans:
- [x] 15-01-PLAN.md — Grafana API client backend with SecretWatcher integration
- [x] 15-02-PLAN.md — FalkorDB Dashboard node schema with named graph support
- [x] 15-03-PLAN.md — UI configuration form and test connection handler

#### ✅ Phase 16: Ingestion Pipeline - Dashboard Sync & PromQL Parsing
**Goal**: Dashboards are ingested incrementally with full semantic structure extracted to graph.
**Depends on**: Phase 15
**Requirements**: FOUN-04, GRPH-02, GRPH-03, GRPH-04, GRPH-06, PROM-01, PROM-02, PROM-03, PROM-04, PROM-05, PROM-06, UICF-05
**Success Criteria** (what must be TRUE):
  1. DashboardSyncer detects changed dashboards via version field (incremental sync)
  2. PromQL parser extracts metric names, label selectors, and aggregation functions
  3. Graph contains Dashboard→Panel→Query→Metric relationships with CONTAINS/HAS/USES edges
  4. UI displays sync status and last sync time
  5. Parser handles Grafana variable syntax as passthrough (preserves $var, [[var]])
**Plans**: 3 plans
**Completed**: 2026-01-22

Plans:
- [x] 16-01-PLAN.md — PromQL parser with AST extraction (metrics, labels, aggregations)
- [x] 16-02-PLAN.md — Dashboard syncer with incremental sync and graph builder
- [x] 16-03-PLAN.md — UI sync status display and manual sync trigger

#### ✅ Phase 17: Semantic Layer - Service Inference & Dashboard Hierarchy
**Goal**: Dashboards are classified by hierarchy level, services are inferred from metrics, and variables are classified by type.
**Depends on**: Phase 16
**Requirements**: GRPH-05, SERV-01, SERV-02, SERV-03, SERV-04, HIER-01, HIER-02, HIER-03, HIER-04, VARB-01, VARB-02, VARB-03, UICF-04
**Success Criteria** (what must be TRUE):
  1. Service nodes are created from PromQL label extraction (job, service, app, namespace, cluster)
  2. Metric→Service relationships exist in graph (TRACKS edges)
  3. Dashboards are classified as overview, drill-down, or detail based on tags
  4. Variables are classified as scoping (cluster/region), entity (service/namespace), or detail (pod/instance)
  5. UI allows configuration of hierarchy mapping fallback (when tags not present)
**Plans**: 4 plans
**Completed**: 2026-01-23

Plans:
- [x] 17-01-PLAN.md — Service inference from PromQL label selectors
- [x] 17-02-PLAN.md — Variable classification (scoping/entity/detail)
- [x] 17-03-PLAN.md — Dashboard hierarchy classification with tag-first logic
- [x] 17-04-PLAN.md — UI hierarchy mapping configuration

#### ✅ Phase 18: Query Execution & MCP Tools Foundation
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
**Plans**: 3 plans
**Completed**: 2026-01-23

Plans:
- [x] 18-01-PLAN.md — GrafanaQueryService with Grafana /api/ds/query integration
- [x] 18-02-PLAN.md — Three MCP tools (overview, aggregated, details)
- [x] 18-03-PLAN.md — Tool registration and end-to-end verification

#### ✅ Phase 19: Anomaly Detection & Progressive Disclosure
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
**Plans**: 4 plans
**Completed**: 2026-01-23

Plans:
- [x] 19-01-PLAN.md — Statistical detector with z-score analysis (TDD)
- [x] 19-02-PLAN.md — Baseline cache with FalkorDB storage and TTL
- [x] 19-03-PLAN.md — Anomaly service orchestration and Overview tool integration
- [x] 19-04-PLAN.md — Integration wiring, tests, and verification

**Stats:** 5 phases, 17 plans, 51 requirements

</details>

<details>
<summary>✅ v1.4 Grafana Alerts Integration (Phases 20-23) - SHIPPED 2026-01-23</summary>

**Milestone Goal:** Extend Grafana integration with alert rule ingestion, graph linking, and progressive disclosure MCP tools for incident response.

#### ✅ Phase 20: Alert API Client & Graph Schema
**Goal**: Alert rules are synced from Grafana and stored in FalkorDB with links to existing Metrics and Services.
**Depends on**: Phase 19 (v1.3 complete)
**Requirements**: ALRT-01, ALRT-02, GRPH-08, GRPH-09, GRPH-10
**Success Criteria** (what must be TRUE):
  1. GrafanaClient can fetch alert rules via Grafana Alerting API
  2. Alert rules are synced incrementally based on version field (like dashboards)
  3. Alert nodes exist in FalkorDB with metadata (name, severity, labels, current state)
  4. PromQL parser extracts metrics from alert rule queries (reuses existing parser)
  5. Graph contains Alert→Metric relationships (MONITORS edges)
  6. Graph contains Alert→Service relationships (transitive through Metric nodes)
**Plans**: 2 plans
**Completed**: 2026-01-23

Plans:
- [x] 20-01-PLAN.md — Alert node schema and Grafana API client methods
- [x] 20-02-PLAN.md — AlertSyncer with incremental sync and graph relationships

#### ✅ Phase 21: Alert Sync Pipeline
**Goal**: Alert state is continuously tracked with full state transition timeline stored in graph.
**Depends on**: Phase 20
**Requirements**: ALRT-03, ALRT-04, ALRT-05, GRPH-11
**Success Criteria** (what must be TRUE):
  1. AlertSyncer fetches current alert state (firing/pending/normal) with timestamps
  2. AlertStateChange nodes are created for every state transition
  3. Graph stores full state timeline with from_state, to_state, and timestamp
  4. Periodic sync updates both alert rules and current state
  5. Sync gracefully handles Grafana API unavailability (logs error, continues with stale data)
**Plans**: 2 plans
**Completed**: 2026-01-23

Plans:
- [x] 21-01-PLAN.md — Alert state API client and graph storage with deduplication
- [x] 21-02-PLAN.md — AlertStateSyncer with periodic sync and lifecycle wiring

#### ✅ Phase 22: Historical Analysis
**Goal**: AI can identify flapping alerts and compare current alert behavior to 7-day baseline.
**Depends on**: Phase 21
**Requirements**: HIST-01, HIST-02, HIST-03, HIST-04
**Success Criteria** (what must be TRUE):
  1. AlertAnalysisService computes 7-day baseline for alert state patterns (rolling average)
  2. Flappiness detection identifies alerts with frequent state transitions within time window
  3. Trend analysis distinguishes recently-started alerts from always-firing alerts
  4. Historical comparison determines if current alert behavior is normal vs abnormal
  5. Analysis handles missing historical data gracefully (marks as unknown vs error)
**Plans**: 3 plans
**Completed**: 2026-01-23

Plans:
- [x] 22-01-PLAN.md — Statistical analysis foundation with TDD (flappiness, baseline)
- [x] 22-02-PLAN.md — AlertAnalysisService with categorization and cache
- [x] 22-03-PLAN.md — Integration lifecycle wiring and end-to-end tests

#### ✅ Phase 23: MCP Tools
**Goal**: AI can discover firing alerts, analyze state progression, and drill into full timeline through three progressive disclosure tools.
**Depends on**: Phase 22
**Requirements**: TOOL-10, TOOL-11, TOOL-12, TOOL-13, TOOL-14, TOOL-15, TOOL-16, TOOL-17, TOOL-18
**Success Criteria** (what must be TRUE):
  1. MCP tool `grafana_{name}_alerts_overview` returns firing/pending counts by severity/cluster/service/namespace
  2. Overview tool accepts optional filters (severity, cluster, service, namespace)
  3. Overview tool includes flappiness indicator for each alert group
  4. MCP tool `grafana_{name}_alerts_aggregated` shows specific alerts with 1h state progression
  5. Aggregated tool accepts lookback duration parameter
  6. Aggregated tool provides state change summary (started firing, was firing, flapping)
  7. MCP tool `grafana_{name}_alerts_details` returns full state timeline graph data
  8. Details tool includes alert rule definition and labels
  9. All alert tools are stateless (AI manages context across calls)
**Plans**: 3 plans
**Completed**: 2026-01-23

Plans:
- [x] 23-01-PLAN.md — Overview tool with filtering and flappiness counts
- [x] 23-02-PLAN.md — Aggregated and details tools with state timeline buckets
- [x] 23-03-PLAN.md — Integration tests and end-to-end verification

**Stats:** 4 phases, 10 plans, 22 requirements

</details>

<details open>
<summary>🚧 v1.5 Observatory (Phases 24-26) - IN PROGRESS</summary>

**Milestone Goal:** Build a signal intelligence layer that extracts "what matters" from dashboards and exposes it for AI-driven incident investigation.

**Core insight:** Dashboards encode human knowledge about "what matters" — Observatory extracts, classifies, and exposes that knowledge so AI agents can investigate incidents systematically.

#### Phase 24: Data Model & Ingestion
**Goal**: Signal anchors exist in graph with role classification, quality scoring, and K8s workload linkage.
**Depends on**: Phase 23 (v1.4 complete)
**Requirements**: SCHM-01, SCHM-02, SCHM-03, SCHM-04, SCHM-05, SCHM-06, SCHM-07, SCHM-08, CLAS-01, CLAS-02, CLAS-03, CLAS-04, CLAS-05, CLAS-06, QUAL-01, QUAL-02, QUAL-03, QUAL-04, QUAL-05, INGT-01, INGT-02, INGT-03, INGT-04, INGT-05, INGT-06
**Success Criteria** (what must be TRUE):
  1. SignalAnchor nodes appear in FalkorDB linked to Dashboard, Panel, Metric, and K8s workload nodes
  2. Each anchor has a classified signal role (Availability, Latency, Errors, Traffic, Saturation, Churn, Novelty) with confidence score
  3. Each anchor has a quality score derived from its source dashboard (freshness, alerting, ownership, completeness)
  4. Ingestion pipeline transforms existing dashboards/panels into signal anchors idempotently
  5. Pipeline runs on schedule and can be triggered manually via existing UI sync mechanism
**Plans**: 4 plans

Plans:
- [ ] 24-01-PLAN.md — SignalAnchor types, layered classifier, quality scorer
- [ ] 24-02-PLAN.md — Signal extractor and K8s workload linker
- [ ] 24-03-PLAN.md — GraphBuilder integration and DashboardSyncer hook
- [ ] 24-04-PLAN.md — Integration tests and verification

#### Phase 25: Baseline & Anomaly Detection
**Goal**: Anomalies are detected against rolling baselines with alert-bootstrapped thresholds and hybrid collection.
**Depends on**: Phase 24
**Requirements**: BASE-01, BASE-02, BASE-03, BASE-04, BASE-05, BASE-06, ANOM-01, ANOM-02, ANOM-03, ANOM-04, ANOM-05, ANOM-06
**Success Criteria** (what must be TRUE):
  1. Rolling statistics (median, P50/P90/P99, stddev, min/max, sample count) are stored per SignalAnchor
  2. Forward collection updates baselines periodically; opt-in catchup backfills from historical data
  3. Anomaly score (0.0-1.0) computed via z-score and percentile comparison with confidence indicator
  4. Grafana alert state (firing/pending/normal) treated as strong anomaly signal
  5. Anomalies aggregate upward: metrics to signals to workloads to namespaces to clusters
**Plans**: TBD

#### Phase 26: Observatory API & MCP Tools
**Goal**: AI can investigate incidents through 8 progressive disclosure tools covering Orient, Narrow, Investigate, Hypothesize, and Verify stages.
**Depends on**: Phase 25
**Requirements**: API-01, API-02, API-03, API-04, API-05, API-06, API-07, API-08, TOOL-01, TOOL-02, TOOL-03, TOOL-04, TOOL-05, TOOL-06, TOOL-07, TOOL-08, TOOL-09, TOOL-10, TOOL-11, TOOL-12, TOOL-13, TOOL-14, TOOL-15, TOOL-16
**Success Criteria** (what must be TRUE):
  1. Observatory API returns anomalies, workload signals, signal details, and dashboard quality rankings
  2. API responses include scope, timestamp, summary, confidence, and suggestions for next query
  3. Orient tools (`observatory_status`, `observatory_changes`) show cluster-wide anomaly summary and recent changes
  4. Narrow tools (`observatory_scope`, `observatory_signals`) focus on specific namespace/workload with ranked signals
  5. Investigate/Hypothesize/Verify tools (`observatory_signal_detail`, `observatory_compare`, `observatory_explain`, `observatory_evidence`) provide deep analysis with K8s graph integration
**Plans**: TBD

**Stats:** 3 phases, TBD plans, 61 requirements

</details>

## Progress

| Milestone | Phases | Plans | Requirements | Status |
|-----------|--------|-------|--------------|--------|
| v1.0 | 1-5 | 19 | 31 | ✅ Shipped 2026-01-21 |
| v1.1 | 6-9 | 12 | 21 | ✅ Shipped 2026-01-21 |
| v1.2 | 10-14 | 8 | 21 | ✅ Shipped 2026-01-22 |
| v1.3 | 15-19 | 17 | 51 | ✅ Shipped 2026-01-23 |
| v1.4 | 20-23 | 10 | 22 | ✅ Shipped 2026-01-23 |
| v1.5 | 24-26 | TBD | 61 | 🚧 In Progress |

**Total:** 26 phases, 66+ plans, 207 requirements

---
*v1.5 roadmap created: 2026-01-29*
