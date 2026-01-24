# Project Milestones: Spectre MCP Plugin System

## v1.4 Grafana Alerts Integration (Shipped: 2026-01-23)

**Delivered:** Alert rule ingestion from Grafana with state tracking, historical analysis, and progressive disclosure MCP tools—overview with flappiness indicators, aggregated with 1h state timelines, details with full 7-day history.

**Phases completed:** 20-23 (10 plans total)

**Key accomplishments:**

- Alert rule sync via Grafana Alerting API with incremental updates (version-based)
- STATE_TRANSITION self-edges for 7-day timeline with TTL-based retention
- Flappiness detection with exponential scaling (0.7 threshold)
- Multi-label categorization: onset (NEW/RECENT/CHRONIC) + pattern (flapping/stable)
- AlertAnalysisService with 1000-entry LRU cache (5-minute TTL)
- Three MCP tools: overview (severity grouping), aggregated (10-min bucket timelines), details (full history)
- 959 lines of integration tests with progressive disclosure workflow validation

**Stats:**

- ~4,630 LOC added
- 4 phases, 10 plans, 22 requirements
- Same-day execution (all 4 phases completed 2026-01-23)
- Total: 6 Grafana MCP tools (3 metrics + 3 alerts)

**Git range:** Phase 20 → Phase 23

**What's next:** Cross-signal correlation (alert↔log, alert↔metric anomaly) or additional integrations (Datadog, PagerDuty)

---

## v1.3 Grafana Metrics Integration (Shipped: 2026-01-23)

**Delivered:** Grafana dashboards as structured operational knowledge with PromQL parsing, semantic service inference, 7-day baseline anomaly detection, and progressive disclosure MCP tools—overview with ranked anomalies, aggregated with service focus, details with full dashboard execution.

**Phases completed:** 15-19 (17 plans total)

**Key accomplishments:**

- Grafana API client with Bearer token authentication and SecretWatcher hot-reload
- PromQL parser using official Prometheus library (metrics, labels, aggregations)
- Dashboard→Panel→Query→Metric graph relationships with incremental sync
- Service inference from PromQL labels with cluster/namespace scoping
- Dashboard hierarchy classification (overview, drilldown, detail)
- Statistical z-score detector with 7-day baseline (time-of-day, weekday/weekend matching)
- Three MCP tools with progressive disclosure and anomaly ranking

**Stats:**

- ~6,835 LOC added
- 5 phases, 17 plans, 51 requirements
- 2-day execution (2026-01-22 to 2026-01-23)

**Git range:** Phase 15 → Phase 19

---

## v1.2 Logz.io Integration + Secret Management (Shipped: 2026-01-22)

**Delivered:** Logz.io as second log backend with Kubernetes-native secret management—SecretWatcher with hot-reload, 3 MCP tools (overview, logs, patterns), UI configuration form, and Helm chart documentation for production deployment.

**Phases completed:** 11-14 (8 plans total)

**Key accomplishments:**

- SecretWatcher with SharedInformerFactory for zero-downtime credential rotation (< 2s detection)
- Thread-safe token access with sync.RWMutex and graceful degradation when secrets missing
- Logz.io HTTP client with X-API-TOKEN authentication and 5-region support (US, EU, UK, AU, CA)
- Three MCP tools with VictoriaLogs parity: overview (parallel aggregations), logs (100 limit), patterns (novelty detection)
- UI form with region selector and SecretRef fields (Secret Name, Key) in Authentication section
- Helm chart values.yaml with copy-paste Secret mounting example and 4-step rotation workflow

**Stats:**

- ~104k Go LOC, ~21k TypeScript LOC (cumulative)
- 4 phases, 8 plans, 21 requirements
- Same-day execution (all 4 phases completed 2026-01-22)
- Critical fix: Logzio factory import added during milestone audit

**Git range:** Phase 11 → Phase 14

**What's next:** Additional integrations (Grafana Cloud, Datadog) or advanced features (multi-account support, pattern alerting)

---

## v1.1 Server Consolidation (Shipped: 2026-01-21)

**Delivered:** Single-port deployment with in-process MCP execution—REST API, UI, and MCP all served on port 8080, eliminating MCP sidecar and HTTP overhead via shared service layer.

**Phases completed:** 6-9 (12 plans total)

**Key accomplishments:**

- Single-port deployment with REST API, UI, and MCP on port 8080 at /v1/mcp endpoint
- Service layer extracted: TimelineService, GraphService, MetadataService, SearchService shared by REST and MCP
- HTTP self-calls eliminated—MCP tools call services directly in-process
- 14,676 lines of dead code removed—standalone mcp/agent/mock commands and internal/agent package
- Helm chart simplified—single-container deployment, no MCP sidecar
- E2E tests validated for consolidated architecture

**Stats:**

- 154 files changed
- 9,589 insertions, 17,168 deletions (net -7,579 lines, cleaned dead code)
- 4 phases, 12 plans, 21 requirements
- 56 commits
- Same-day execution (all 4 phases completed 2026-01-21)

**Git range:** `607ad75` → `a359b53`

**What's next:** Additional integrations (Logz.io, Grafana Cloud, VictoriaMetrics) or advanced features (MCP authentication, long-term baseline tracking)

---

## v1 MCP Plugin System + VictoriaLogs (Shipped: 2026-01-21)

**Delivered:** AI assistants can now explore logs progressively via MCP tools—starting from high-level signals, drilling into patterns with novelty detection, and viewing raw logs when context is narrow.

**Phases completed:** 1-5 (19 plans total)

**Key accomplishments:**

- Plugin infrastructure with factory registry, config hot-reload (fsnotify), lifecycle manager with health monitoring and auto-recovery
- REST API + React UI for integration management with atomic YAML writes and health status enrichment
- VictoriaLogs client with LogsQL query builder, tuned connection pooling, backpressure pipeline
- Log template mining using Drain algorithm with namespace-scoped storage, SHA-256 hashing, persistence, auto-merge and pruning
- Progressive disclosure MCP tools (overview/patterns/logs) with novelty detection and high-volume sampling

**Stats:**

- 108 files created/modified
- ~17,850 lines of Go + TypeScript
- 5 phases, 19 plans, 31 requirements
- 1 day from start to ship

**Git range:** `feat(01-01)` → `docs(05)`

**What's next:** Additional integrations (Logz.io, Grafana Cloud) or advanced features (long-term baseline tracking, anomaly scoring)

---
