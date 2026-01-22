# Requirements: Spectre v1.3 Grafana Metrics Integration

**Defined:** 2026-01-22
**Core Value:** Use Grafana dashboards as structured operational knowledge so Spectre can detect high-level anomalies, progressively drill down, and reason about services, clusters, and metrics.

## v1.3 Requirements

Requirements for Grafana metrics integration. Each maps to roadmap phases.

### Foundation

- [ ] **FOUN-01**: Grafana API client supports both Cloud and self-hosted authentication
- [ ] **FOUN-02**: Client can list all dashboards via Grafana search API
- [ ] **FOUN-03**: Client can retrieve full dashboard JSON by UID
- [x] **FOUN-04**: Incremental sync detects changed dashboards via version field
- [ ] **FOUN-05**: Client integrates with SecretWatcher for API token hot-reload
- [ ] **FOUN-06**: Integration follows factory registry pattern (compile-time registration)

### Graph Schema

- [ ] **GRPH-01**: FalkorDB schema includes Dashboard nodes with metadata (uid, title, tags, folder)
- [x] **GRPH-02**: FalkorDB schema includes Panel nodes with query references
- [x] **GRPH-03**: FalkorDB schema includes Query nodes with raw PromQL expressions
- [x] **GRPH-04**: FalkorDB schema includes Metric nodes (metric name templates)
- [x] **GRPH-05**: FalkorDB schema includes Service nodes inferred from metric labels
- [x] **GRPH-06**: Relationships: Dashboard CONTAINS Panel, Panel HAS Query, Query USES Metric, Metric TRACKS Service
- [ ] **GRPH-07**: Graph indexes on Dashboard.uid, Metric.name, Service.name for efficient queries

### PromQL Parsing

- [x] **PROM-01**: PromQL parser uses official Prometheus library (prometheus/promql/parser)
- [x] **PROM-02**: Parser extracts metric names from VectorSelector nodes
- [x] **PROM-03**: Parser extracts label selectors (key-value matchers)
- [x] **PROM-04**: Parser extracts aggregation functions (sum, avg, rate, etc.)
- [x] **PROM-05**: Parser handles variable syntax ($var, ${var}, [[var]]) as passthrough
- [x] **PROM-06**: Parser uses best-effort extraction (complex expressions may partially parse)

### Service Inference

- [x] **SERV-01**: Service inference extracts from job, service, app labels in PromQL
- [x] **SERV-02**: Service inference extracts namespace and cluster for scoping
- [x] **SERV-03**: Service nodes link to Metric nodes via TRACKS relationship
- [x] **SERV-04**: Service inference uses whitelist approach (known-good labels only)

### Dashboard Hierarchy

- [x] **HIER-01**: Dashboards classified as overview, drill-down, or detail level
- [x] **HIER-02**: Hierarchy read from Grafana tags (spectre:overview, spectre:drilldown, spectre:detail)
- [x] **HIER-03**: Hierarchy fallback to config mapping when tags not present
- [x] **HIER-04**: Hierarchy level stored as Dashboard node property

### Variable Handling

- [x] **VARB-01**: Variables extracted from dashboard JSON template section
- [x] **VARB-02**: Variables classified as scoping (cluster, region), entity (service, namespace), or detail (pod, instance)
- [x] **VARB-03**: Variable classification stored in graph for smart defaults
- [ ] **VARB-04**: Single-value variable substitution supported for query execution
- [ ] **VARB-05**: Variables passed to Grafana API via scopedVars (not interpolated locally)

### Query Execution

- [ ] **EXEC-01**: Queries executed via Grafana /api/ds/query endpoint
- [ ] **EXEC-02**: Query service handles time range parameters (from, to, interval)
- [ ] **EXEC-03**: Query service formats Prometheus time series response for MCP tools
- [ ] **EXEC-04**: Query service supports scoping variable substitution (AI provides values)

### MCP Tools

- [ ] **TOOL-01**: `grafana_{name}_metrics_overview` executes overview dashboards only
- [ ] **TOOL-02**: `grafana_{name}_metrics_overview` detects anomalies vs 7-day baseline
- [ ] **TOOL-03**: `grafana_{name}_metrics_overview` returns ranked anomalies with severity
- [ ] **TOOL-04**: `grafana_{name}_metrics_aggregated` focuses on specified service or cluster
- [ ] **TOOL-05**: `grafana_{name}_metrics_aggregated` executes related dashboards for correlation
- [ ] **TOOL-06**: `grafana_{name}_metrics_details` executes full dashboard with all panels
- [ ] **TOOL-07**: `grafana_{name}_metrics_details` supports deep variable expansion
- [ ] **TOOL-08**: All tools accept scoping variables (cluster, region) as parameters
- [ ] **TOOL-09**: All tools are stateless (AI manages context across calls)

### Anomaly Detection

- [ ] **ANOM-01**: Baseline computed from 7-day historical data
- [ ] **ANOM-02**: Baseline uses time-of-day matching (compare Monday 10am to previous Mondays 10am)
- [ ] **ANOM-03**: Anomaly detection uses z-score comparison against baseline
- [ ] **ANOM-04**: Anomalies classified by severity (info, warning, critical)
- [ ] **ANOM-05**: Baseline cached in graph with TTL (1-hour refresh)
- [ ] **ANOM-06**: Anomaly detection handles missing metrics gracefully (check scrape status)

### UI Configuration

- [ ] **UICF-01**: Integration form includes Grafana URL field
- [ ] **UICF-02**: Integration form includes API token field (SecretRef: name + key)
- [ ] **UICF-03**: Integration form validates connection on save (health check)
- [x] **UICF-04**: Integration form includes hierarchy mapping configuration
- [x] **UICF-05**: UI displays sync status and last sync time

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Advanced Variables

- **VARB-V2-01**: Multi-value variable support with pipe syntax
- **VARB-V2-02**: Chained variables (3+ levels deep)
- **VARB-V2-03**: Query variables (dynamic options from data source)

### Advanced Anomaly Detection

- **ANOM-V2-01**: ML-based anomaly detection (LSTM, adaptive baselines)
- **ANOM-V2-02**: Root cause analysis across correlated metrics
- **ANOM-V2-03**: Anomaly pattern learning (reduce false positives over time)

### Cross-Signal Correlation

- **CORR-V2-01**: Trace linking with OpenTelemetry integration
- **CORR-V2-02**: Automatic correlation of metrics with log patterns
- **CORR-V2-03**: Event correlation (K8s events + metric spikes)

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Dashboard UI replication | Return structured data, not rendered visualizations |
| Dashboard creation/editing | Read-only access, users manage dashboards in Grafana |
| Direct Prometheus queries | Use Grafana API as proxy for simpler auth |
| Metric value storage | Query on-demand, avoid time-series DB complexity |
| Per-user dashboard state | Stateless MCP architecture, no session state |
| Alert rule sync | Different API, defer to future milestone |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| FOUN-01 | Phase 15 | Complete |
| FOUN-02 | Phase 15 | Complete |
| FOUN-03 | Phase 15 | Complete |
| FOUN-04 | Phase 16 | Complete |
| FOUN-05 | Phase 15 | Complete |
| FOUN-06 | Phase 15 | Complete |
| GRPH-01 | Phase 15 | Complete |
| GRPH-02 | Phase 16 | Complete |
| GRPH-03 | Phase 16 | Complete |
| GRPH-04 | Phase 16 | Complete |
| GRPH-05 | Phase 17 | Complete |
| GRPH-06 | Phase 16 | Complete |
| GRPH-07 | Phase 15 | Complete |
| PROM-01 | Phase 16 | Complete |
| PROM-02 | Phase 16 | Complete |
| PROM-03 | Phase 16 | Complete |
| PROM-04 | Phase 16 | Complete |
| PROM-05 | Phase 16 | Complete |
| PROM-06 | Phase 16 | Complete |
| SERV-01 | Phase 17 | Complete |
| SERV-02 | Phase 17 | Complete |
| SERV-03 | Phase 17 | Complete |
| SERV-04 | Phase 17 | Complete |
| HIER-01 | Phase 17 | Complete |
| HIER-02 | Phase 17 | Complete |
| HIER-03 | Phase 17 | Complete |
| HIER-04 | Phase 17 | Complete |
| VARB-01 | Phase 17 | Complete |
| VARB-02 | Phase 17 | Complete |
| VARB-03 | Phase 17 | Complete |
| VARB-04 | Phase 18 | Pending |
| VARB-05 | Phase 18 | Pending |
| EXEC-01 | Phase 18 | Pending |
| EXEC-02 | Phase 18 | Pending |
| EXEC-03 | Phase 18 | Pending |
| EXEC-04 | Phase 18 | Pending |
| TOOL-01 | Phase 18 | Pending |
| TOOL-02 | Phase 19 | Pending |
| TOOL-03 | Phase 19 | Pending |
| TOOL-04 | Phase 18 | Pending |
| TOOL-05 | Phase 18 | Pending |
| TOOL-06 | Phase 18 | Pending |
| TOOL-07 | Phase 18 | Pending |
| TOOL-08 | Phase 18 | Pending |
| TOOL-09 | Phase 18 | Pending |
| ANOM-01 | Phase 19 | Pending |
| ANOM-02 | Phase 19 | Pending |
| ANOM-03 | Phase 19 | Pending |
| ANOM-04 | Phase 19 | Pending |
| ANOM-05 | Phase 19 | Pending |
| ANOM-06 | Phase 19 | Pending |
| UICF-01 | Phase 15 | Complete |
| UICF-02 | Phase 15 | Complete |
| UICF-03 | Phase 15 | Complete |
| UICF-04 | Phase 17 | Complete |
| UICF-05 | Phase 16 | Complete |

**Coverage:**
- v1.3 requirements: 51 total
- Mapped to phases: 51
- Unmapped: 0 ✓

---
*Requirements defined: 2026-01-22*
*Last updated: 2026-01-22 after v1.3 roadmap creation*
