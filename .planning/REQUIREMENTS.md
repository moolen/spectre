# Requirements: Spectre v1.5 Observatory

**Defined:** 2026-01-29
**Core Value:** Enable AI assistants to understand what's happening in Kubernetes clusters through unified MCP interface—signal anchors extract "what matters" from dashboards for systematic incident investigation.

## v1.5 Requirements

Requirements for Observatory signal intelligence layer. Each maps to roadmap phases.

### Signal Schema

- [ ] **SCHM-01**: SignalAnchor nodes exist in FalkorDB with links to source dashboard/panel
- [ ] **SCHM-02**: SignalAnchor nodes link to metric(s) they represent
- [ ] **SCHM-03**: SignalAnchor nodes have classified signal role from taxonomy
- [ ] **SCHM-04**: SignalAnchor nodes have classification confidence score (0.0-1.0)
- [ ] **SCHM-05**: SignalAnchor nodes have quality score derived from source dashboard
- [ ] **SCHM-06**: SignalAnchor nodes track K8s workload scope (namespace + workload) when inferrable
- [ ] **SCHM-07**: SignalAnchor nodes track source Grafana instance for multi-source support
- [ ] **SCHM-08**: Graph relationships connect anchors to Dashboard, Panel, Metric, and K8s workload nodes

### Role Classification

- [ ] **CLAS-01**: Signal role taxonomy implemented (Availability, Latency, Errors, Traffic, Saturation, Churn, Novelty)
- [ ] **CLAS-02**: Keyword/heuristic matching classifies metrics against panel titles, descriptions, metric names
- [ ] **CLAS-03**: Hardcoded mappings for well-known metrics (kube_*, cadvisor, node-exporter, Go runtime, HTTP)
- [ ] **CLAS-04**: Classification confidence computed based on match strength
- [ ] **CLAS-05**: Panels with multiple metrics can have different roles per metric
- [ ] **CLAS-06**: K8s workload scope inferred from PromQL label selectors (namespace, job, service, app)

### Dashboard Quality

- [ ] **QUAL-01**: Dashboard quality score computed (0.0-1.0) based on freshness, alerting, ownership, completeness
- [ ] **QUAL-02**: Freshness scoring uses days since last modification with decay function
- [ ] **QUAL-03**: Alerting bonus: dashboards with associated alert rules score higher
- [ ] **QUAL-04**: Ownership bonus: dashboards in team-specific folders score higher than "General"
- [ ] **QUAL-05**: Completeness bonus: dashboards with meaningful titles and descriptions score higher

### Ingestion Pipeline

- [ ] **INGT-01**: Panel → SignalAnchor transformation extracts metrics and classifies to roles
- [ ] **INGT-02**: Pipeline is idempotent (re-running updates existing anchors, not duplicates)
- [ ] **INGT-03**: Pipeline runs as background goroutine on configurable schedule
- [ ] **INGT-04**: Pipeline can be triggered manually via existing UI mechanism
- [ ] **INGT-05**: Pipeline tracks last sync time per Grafana source
- [ ] **INGT-06**: Pipeline integrates with existing Grafana dashboard sync mechanism

### Baseline Storage

- [ ] **BASE-01**: Rolling statistics stored per SignalAnchor (median, P50, P90, P99)
- [ ] **BASE-02**: Rolling statistics include standard deviation, min/max, sample count
- [ ] **BASE-03**: Baseline tracks time window covered by samples
- [ ] **BASE-04**: Forward-looking collection updates baselines periodically via Grafana queries
- [ ] **BASE-05**: Opt-in catchup mode backfills baseline from historical data (rate-limited)
- [ ] **BASE-06**: Alert rule thresholds bootstrap initial anomaly boundaries

### Anomaly Detection

- [ ] **ANOM-01**: Anomaly score computed using z-score (standard deviations from mean)
- [ ] **ANOM-02**: Anomaly score uses percentile comparison (current vs historical P99)
- [ ] **ANOM-03**: Anomaly output includes score (0.0-1.0) and confidence (0.0-1.0)
- [ ] **ANOM-04**: Cold start handled gracefully (returns "insufficient data" state)
- [ ] **ANOM-05**: Anomalies aggregate from metrics → signals → workloads → namespaces → clusters
- [ ] **ANOM-06**: Grafana alert state (firing/pending/normal) used as strong anomaly signal

### Observatory API

- [ ] **API-01**: GetAnomalies returns current anomalies optionally scoped by cluster/namespace/workload
- [ ] **API-02**: GetWorkloadSignals returns all signals for a workload with current state
- [ ] **API-03**: GetSignalDetail returns baseline, current value, anomaly score, source dashboard
- [ ] **API-04**: GetSignalsByRole returns anchors filtered by role across a scope
- [ ] **API-05**: GetDashboardQuality returns dashboard quality rankings
- [ ] **API-06**: API response envelope includes scope, timestamp, summary, confidence, suggestions
- [ ] **API-07**: Suggestions field guides progressive disclosure (what to query next)
- [ ] **API-08**: API integrates with GraphService for K8s topology queries

### MCP Tools - Orient

- [ ] **TOOL-01**: `observatory_status` returns cluster/namespace anomaly summary
- [ ] **TOOL-02**: `observatory_status` returns top 5 hotspots with severity
- [ ] **TOOL-03**: `observatory_changes` returns recent Flux deployments, config changes, image updates
- [ ] **TOOL-04**: `observatory_changes` leverages existing K8s graph for change events

### MCP Tools - Narrow

- [ ] **TOOL-05**: `observatory_scope` accepts namespace/workload filter parameters
- [ ] **TOOL-06**: `observatory_scope` returns signals and anomalies ranked by severity
- [ ] **TOOL-07**: `observatory_signals` returns all anchors for a workload grouped by role
- [ ] **TOOL-08**: `observatory_signals` includes current state per anchor

### MCP Tools - Investigate

- [ ] **TOOL-09**: `observatory_signal_detail` returns baseline, current value, anomaly score
- [ ] **TOOL-10**: `observatory_signal_detail` returns source dashboard and confidence
- [ ] **TOOL-11**: `observatory_compare` accepts two signal IDs or signal + event
- [ ] **TOOL-12**: `observatory_compare` returns correlation analysis result

### MCP Tools - Hypothesize

- [ ] **TOOL-13**: `observatory_explain` accepts anomalous signal ID
- [ ] **TOOL-14**: `observatory_explain` returns candidate causes from K8s graph (upstream deps, recent changes)

### MCP Tools - Verify

- [ ] **TOOL-15**: `observatory_evidence` returns raw metric values for a signal
- [ ] **TOOL-16**: `observatory_evidence` returns log snippets when relevant

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Advanced Classification

- **CLAS-V2-01**: ML-based role classification (fine-tuned model)
- **CLAS-V2-02**: Automatic role taxonomy expansion from patterns
- **CLAS-V2-03**: Cross-dashboard deduplication (same metric in multiple dashboards)

### Advanced Anomaly Detection

- **ANOM-V2-01**: Rate of change detection (derivative analysis)
- **ANOM-V2-02**: Seasonal baseline adjustment (weekday vs weekend)
- **ANOM-V2-03**: Root cause ranking with causal inference

### Cross-Signal Correlation

- **CORR-V2-01**: Alert↔Log automatic correlation (time-based linking)
- **CORR-V2-02**: Alert↔Metric anomaly correlation
- **CORR-V2-03**: Cascade detection (alert A causes alert B)

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Dashboard creation/editing | Read-only access, users manage dashboards in Grafana |
| Custom role taxonomy | Fixed 7-role taxonomy sufficient for v1.5 |
| Real-time streaming | Polling-based, not push-based anomaly detection |
| ML-based classification | Keyword heuristics sufficient for v1.5, ML deferred |
| Multi-tenant isolation | Single-tenant deployment assumed |
| Log storage in Observatory | Use existing VictoriaLogs/Logz.io integrations |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| SCHM-01 | — | Pending |
| SCHM-02 | — | Pending |
| SCHM-03 | — | Pending |
| SCHM-04 | — | Pending |
| SCHM-05 | — | Pending |
| SCHM-06 | — | Pending |
| SCHM-07 | — | Pending |
| SCHM-08 | — | Pending |
| CLAS-01 | — | Pending |
| CLAS-02 | — | Pending |
| CLAS-03 | — | Pending |
| CLAS-04 | — | Pending |
| CLAS-05 | — | Pending |
| CLAS-06 | — | Pending |
| QUAL-01 | — | Pending |
| QUAL-02 | — | Pending |
| QUAL-03 | — | Pending |
| QUAL-04 | — | Pending |
| QUAL-05 | — | Pending |
| INGT-01 | — | Pending |
| INGT-02 | — | Pending |
| INGT-03 | — | Pending |
| INGT-04 | — | Pending |
| INGT-05 | — | Pending |
| INGT-06 | — | Pending |
| BASE-01 | — | Pending |
| BASE-02 | — | Pending |
| BASE-03 | — | Pending |
| BASE-04 | — | Pending |
| BASE-05 | — | Pending |
| BASE-06 | — | Pending |
| ANOM-01 | — | Pending |
| ANOM-02 | — | Pending |
| ANOM-03 | — | Pending |
| ANOM-04 | — | Pending |
| ANOM-05 | — | Pending |
| ANOM-06 | — | Pending |
| API-01 | — | Pending |
| API-02 | — | Pending |
| API-03 | — | Pending |
| API-04 | — | Pending |
| API-05 | — | Pending |
| API-06 | — | Pending |
| API-07 | — | Pending |
| API-08 | — | Pending |
| TOOL-01 | — | Pending |
| TOOL-02 | — | Pending |
| TOOL-03 | — | Pending |
| TOOL-04 | — | Pending |
| TOOL-05 | — | Pending |
| TOOL-06 | — | Pending |
| TOOL-07 | — | Pending |
| TOOL-08 | — | Pending |
| TOOL-09 | — | Pending |
| TOOL-10 | — | Pending |
| TOOL-11 | — | Pending |
| TOOL-12 | — | Pending |
| TOOL-13 | — | Pending |
| TOOL-14 | — | Pending |
| TOOL-15 | — | Pending |
| TOOL-16 | — | Pending |

**Coverage:**
- v1.5 requirements: 54 total
- Mapped to phases: 0 (pending roadmap)
- Unmapped: 54

---
*Requirements defined: 2026-01-29*
*Last updated: 2026-01-29 after initial definition*
