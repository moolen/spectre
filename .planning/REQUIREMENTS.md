# Requirements: Spectre v1.4 Grafana Alerts Integration

**Defined:** 2026-01-23
**Core Value:** Enable AI assistants to understand what's happening in Kubernetes clusters through unified MCP interface—timeline queries, graph traversal, log exploration, and metrics analysis.

## v1.4 Requirements

Requirements for Grafana alerts integration. Each maps to roadmap phases.

### Alert Sync

- [x] **ALRT-01**: Alert rules synced via Grafana Alerting API (incremental, version-based)
- [x] **ALRT-02**: Alert rule PromQL queries parsed to extract metrics (reuse existing parser)
- [x] **ALRT-03**: Alert state fetched (firing/pending/normal) with timestamps
- [x] **ALRT-04**: Alert state timeline stored in graph (state transitions over time)
- [x] **ALRT-05**: Periodic sync updates alert rules and current state

### Graph Schema

- [x] **GRPH-08**: Alert nodes in FalkorDB with metadata (name, severity, labels, state)
- [x] **GRPH-09**: Alert→Metric relationships via PromQL extraction (MONITORS edge)
- [x] **GRPH-10**: Alert→Service relationships via metric labels (transitive through Metric nodes)
- [x] **GRPH-11**: AlertStateChange nodes for state timeline (timestamp, from_state, to_state)

### Historical Analysis

- [x] **HIST-01**: 7-day baseline for alert state patterns (time-of-day matching)
- [x] **HIST-02**: Flappiness detection (frequent state transitions within window)
- [x] **HIST-03**: Trend analysis (alert started firing recently vs always firing)
- [x] **HIST-04**: State comparison with historical baseline (normal vs abnormal alert behavior)

### MCP Tools

- [x] **TOOL-10**: `grafana_{name}_alerts_overview` — counts by severity/cluster/service/namespace
- [x] **TOOL-11**: `grafana_{name}_alerts_overview` — accepts optional filters (severity, cluster, service, namespace)
- [x] **TOOL-12**: `grafana_{name}_alerts_overview` — includes flappiness indicator per group
- [x] **TOOL-13**: `grafana_{name}_alerts_aggregated` — specific alerts with 1h state progression
- [x] **TOOL-14**: `grafana_{name}_alerts_aggregated` — accepts lookback duration parameter
- [x] **TOOL-15**: `grafana_{name}_alerts_aggregated` — state change summary (started firing, was firing, flapping)
- [x] **TOOL-16**: `grafana_{name}_alerts_details` — full state timeline graph data
- [x] **TOOL-17**: `grafana_{name}_alerts_details` — includes alert rule definition and labels
- [x] **TOOL-18**: All alert tools are stateless (AI manages context)

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Advanced Alert Features

- **ALRT-V2-01**: Alert silencing/muting support
- **ALRT-V2-02**: Alert annotation ingestion
- **ALRT-V2-03**: Notification channel integration

### Cross-Signal Correlation

- **CORR-V2-01**: Alert↔Log correlation (time-based linking)
- **CORR-V2-02**: Alert↔Metric anomaly correlation
- **CORR-V2-03**: Root cause suggestion based on correlated signals

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Alert rule creation/editing | Read-only access, users manage alerts in Grafana |
| Alert acknowledgment | Would require write access and state management |
| Notification routing | Grafana handles notification channels |
| Alert dashboard rendering | Return structured data, not visualizations |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| ALRT-01 | Phase 20 | Complete |
| ALRT-02 | Phase 20 | Complete |
| ALRT-03 | Phase 21 | Complete |
| ALRT-04 | Phase 21 | Complete |
| ALRT-05 | Phase 21 | Complete |
| GRPH-08 | Phase 20 | Complete |
| GRPH-09 | Phase 20 | Complete |
| GRPH-10 | Phase 20 | Complete |
| GRPH-11 | Phase 21 | Complete |
| HIST-01 | Phase 22 | Complete |
| HIST-02 | Phase 22 | Complete |
| HIST-03 | Phase 22 | Complete |
| HIST-04 | Phase 22 | Complete |
| TOOL-10 | Phase 23 | Complete |
| TOOL-11 | Phase 23 | Complete |
| TOOL-12 | Phase 23 | Complete |
| TOOL-13 | Phase 23 | Complete |
| TOOL-14 | Phase 23 | Complete |
| TOOL-15 | Phase 23 | Complete |
| TOOL-16 | Phase 23 | Complete |
| TOOL-17 | Phase 23 | Complete |
| TOOL-18 | Phase 23 | Complete |

**Coverage:**
- v1.4 requirements: 22 total
- Mapped to phases: 22 (100%)
- Unmapped: 0

**Phase Distribution:**
- Phase 20: 5 requirements (Alert API Client & Graph Schema)
- Phase 21: 4 requirements (Alert Sync Pipeline)
- Phase 22: 4 requirements (Historical Analysis)
- Phase 23: 9 requirements (MCP Tools)

---
*Requirements defined: 2026-01-23*
*Last updated: 2026-01-23 — v1.4 milestone COMPLETE (22/22 requirements satisfied)*
