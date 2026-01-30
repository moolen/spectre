# Requirements: Spectre v1.5 Observatory

**Defined:** 2026-01-29
**Core Value:** Enable AI assistants to understand what's happening in Kubernetes clusters through unified MCP interface—signal anchors extract "what matters" from dashboards for systematic incident investigation.

## v1.5 Requirements

Requirements for Observatory signal intelligence layer. Each maps to roadmap phases.

### Signal Schema ✅

- [x] **SCHM-01**: SignalAnchor nodes exist in FalkorDB with links to source dashboard/panel
- [x] **SCHM-02**: SignalAnchor nodes link to metric(s) they represent
- [x] **SCHM-03**: SignalAnchor nodes have classified signal role from taxonomy
- [x] **SCHM-04**: SignalAnchor nodes have classification confidence score (0.0-1.0)
- [x] **SCHM-05**: SignalAnchor nodes have quality score derived from source dashboard
- [x] **SCHM-06**: SignalAnchor nodes track K8s workload scope (namespace + workload) when inferrable
- [x] **SCHM-07**: SignalAnchor nodes track source Grafana instance for multi-source support
- [x] **SCHM-08**: Graph relationships connect anchors to Dashboard, Panel, Metric, and K8s workload nodes

### Role Classification ✅

- [x] **CLAS-01**: Signal role taxonomy implemented (Availability, Latency, Errors, Traffic, Saturation, Churn, Novelty)
- [x] **CLAS-02**: Keyword/heuristic matching classifies metrics against panel titles, descriptions, metric names
- [x] **CLAS-03**: Hardcoded mappings for well-known metrics (kube_*, cadvisor, node-exporter, Go runtime, HTTP)
- [x] **CLAS-04**: Classification confidence computed based on match strength
- [x] **CLAS-05**: Panels with multiple metrics can have different roles per metric
- [x] **CLAS-06**: K8s workload scope inferred from PromQL label selectors (namespace, job, service, app)

### Dashboard Quality ✅

- [x] **QUAL-01**: Dashboard quality score computed (0.0-1.0) based on freshness, alerting, ownership, completeness
- [x] **QUAL-02**: Freshness scoring uses days since last modification with decay function
- [x] **QUAL-03**: Alerting bonus: dashboards with associated alert rules score higher
- [x] **QUAL-04**: Ownership bonus: dashboards in team-specific folders score higher than "General"
- [x] **QUAL-05**: Completeness bonus: dashboards with meaningful titles and descriptions score higher

### Ingestion Pipeline ✅

- [x] **INGT-01**: Panel -> SignalAnchor transformation extracts metrics and classifies to roles
- [x] **INGT-02**: Pipeline is idempotent (re-running updates existing anchors, not duplicates)
- [x] **INGT-03**: Pipeline runs as background goroutine on configurable schedule
- [x] **INGT-04**: Pipeline can be triggered manually via existing UI mechanism
- [x] **INGT-05**: Pipeline tracks last sync time per Grafana source
- [x] **INGT-06**: Pipeline integrates with existing Grafana dashboard sync mechanism

### Baseline Storage

- [x] **BASE-01**: Rolling statistics stored per SignalAnchor (median, P50, P90, P99)
- [x] **BASE-02**: Rolling statistics include standard deviation, min/max, sample count
- [x] **BASE-03**: Baseline tracks time window covered by samples
- [x] **BASE-04**: Forward-looking collection updates baselines periodically via Grafana queries
- [x] **BASE-05**: Opt-in catchup mode backfills baseline from historical data (rate-limited)
- [x] **BASE-06**: Alert rule thresholds bootstrap initial anomaly boundaries

### Anomaly Detection

- [x] **ANOM-01**: Anomaly score computed using z-score (standard deviations from mean)
- [x] **ANOM-02**: Anomaly score uses percentile comparison (current vs historical P99)
- [x] **ANOM-03**: Anomaly output includes score (0.0-1.0) and confidence (0.0-1.0)
- [x] **ANOM-04**: Cold start handled gracefully (returns "insufficient data" state)
- [x] **ANOM-05**: Anomalies aggregate from metrics -> signals -> workloads -> namespaces -> clusters
- [x] **ANOM-06**: Grafana alert state (firing/pending/normal) used as strong anomaly signal

### Observatory API ✅

- [x] **API-01**: GetAnomalies returns current anomalies optionally scoped by cluster/namespace/workload
- [x] **API-02**: GetWorkloadSignals returns all signals for a workload with current state
- [x] **API-03**: GetSignalDetail returns baseline, current value, anomaly score, source dashboard
- [x] **API-04**: ~~GetSignalsByRole returns anchors filtered by role across a scope~~ (SUPERSEDED: AI handles role filtering)
- [x] **API-05**: GetDashboardQuality returns dashboard quality rankings
- [x] **API-06**: ~~API response envelope includes scope, timestamp, summary, confidence, suggestions~~ (SUPERSEDED: minimal responses)
- [x] **API-07**: ~~Suggestions field guides progressive disclosure (what to query next)~~ (SUPERSEDED: AI handles next steps)
- [x] **API-08**: API integrates with GraphService for K8s topology queries

### MCP Tools - Orient ✅

- [x] **TOOL-01**: `observatory_status` returns cluster/namespace anomaly summary
- [x] **TOOL-02**: `observatory_status` returns top 5 hotspots with severity
- [x] **TOOL-03**: `observatory_changes` returns recent Flux deployments, config changes, image updates
- [x] **TOOL-04**: `observatory_changes` leverages existing K8s graph for change events

### MCP Tools - Narrow ✅

- [x] **TOOL-05**: `observatory_scope` accepts namespace/workload filter parameters
- [x] **TOOL-06**: `observatory_scope` returns signals and anomalies ranked by severity
- [x] **TOOL-07**: `observatory_signals` returns all anchors for a workload grouped by role
- [x] **TOOL-08**: `observatory_signals` includes current state per anchor

### MCP Tools - Investigate ✅

- [x] **TOOL-09**: `observatory_signal_detail` returns baseline, current value, anomaly score
- [x] **TOOL-10**: `observatory_signal_detail` returns source dashboard and confidence
- [x] **TOOL-11**: `observatory_compare` accepts two signal IDs or signal + event
- [x] **TOOL-12**: `observatory_compare` returns correlation analysis result

### MCP Tools - Hypothesize ✅

- [x] **TOOL-13**: `observatory_explain` accepts anomalous signal ID
- [x] **TOOL-14**: `observatory_explain` returns candidate causes from K8s graph (upstream deps, recent changes)

### MCP Tools - Verify ✅

- [x] **TOOL-15**: `observatory_evidence` returns raw metric values for a signal
- [x] **TOOL-16**: `observatory_evidence` returns log snippets when relevant

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

- **CORR-V2-01**: Alert<->Log automatic correlation (time-based linking)
- **CORR-V2-02**: Alert<->Metric anomaly correlation
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
| SCHM-01 | Phase 24 | Complete |
| SCHM-02 | Phase 24 | Complete |
| SCHM-03 | Phase 24 | Complete |
| SCHM-04 | Phase 24 | Complete |
| SCHM-05 | Phase 24 | Complete |
| SCHM-06 | Phase 24 | Complete |
| SCHM-07 | Phase 24 | Complete |
| SCHM-08 | Phase 24 | Complete |
| CLAS-01 | Phase 24 | Complete |
| CLAS-02 | Phase 24 | Complete |
| CLAS-03 | Phase 24 | Complete |
| CLAS-04 | Phase 24 | Complete |
| CLAS-05 | Phase 24 | Complete |
| CLAS-06 | Phase 24 | Complete |
| QUAL-01 | Phase 24 | Complete |
| QUAL-02 | Phase 24 | Complete |
| QUAL-03 | Phase 24 | Complete |
| QUAL-04 | Phase 24 | Complete |
| QUAL-05 | Phase 24 | Complete |
| INGT-01 | Phase 24 | Complete |
| INGT-02 | Phase 24 | Complete |
| INGT-03 | Phase 24 | Complete |
| INGT-04 | Phase 24 | Complete |
| INGT-05 | Phase 24 | Complete |
| INGT-06 | Phase 24 | Complete |
| BASE-01 | Phase 25 | Complete |
| BASE-02 | Phase 25 | Complete |
| BASE-03 | Phase 25 | Complete |
| BASE-04 | Phase 25 | Complete |
| BASE-05 | Phase 25 | Complete |
| BASE-06 | Phase 25 | Complete |
| ANOM-01 | Phase 25 | Complete |
| ANOM-02 | Phase 25 | Complete |
| ANOM-03 | Phase 25 | Complete |
| ANOM-04 | Phase 25 | Complete |
| ANOM-05 | Phase 25 | Complete |
| ANOM-06 | Phase 25 | Complete |
| API-01 | Phase 26 | Pending |
| API-02 | Phase 26 | Pending |
| API-03 | Phase 26 | Pending |
| API-04 | Phase 26 | Pending |
| API-05 | Phase 26 | Pending |
| API-06 | Phase 26 | Pending |
| API-07 | Phase 26 | Pending |
| API-08 | Phase 26 | Pending |
| TOOL-01 | Phase 26 | Pending |
| TOOL-02 | Phase 26 | Pending |
| TOOL-03 | Phase 26 | Pending |
| TOOL-04 | Phase 26 | Pending |
| TOOL-05 | Phase 26 | Pending |
| TOOL-06 | Phase 26 | Pending |
| TOOL-07 | Phase 26 | Pending |
| TOOL-08 | Phase 26 | Pending |
| TOOL-09 | Phase 26 | Pending |
| TOOL-10 | Phase 26 | Pending |
| TOOL-11 | Phase 26 | Pending |
| TOOL-12 | Phase 26 | Pending |
| TOOL-13 | Phase 26 | Pending |
| TOOL-14 | Phase 26 | Pending |
| TOOL-15 | Phase 26 | Pending |
| TOOL-16 | Phase 26 | Pending |

**Coverage:**
- v1.5 requirements: 61 total
- Mapped to phases: 61
- Phase 24: 25 requirements (SCHM-*, CLAS-*, QUAL-*, INGT-*)
- Phase 25: 12 requirements (BASE-*, ANOM-*)
- Phase 26: 24 requirements (API-*, TOOL-*)
- Unmapped: 0

---
*Requirements defined: 2026-01-29*
*Last updated: 2026-01-29 after Phase 24 completion (25/61 complete)*
