# Project Research Summary

**Project:** Spectre v1.3 Grafana Metrics Integration
**Domain:** AI-assisted metrics observability through Grafana dashboards
**Researched:** 2026-01-22
**Confidence:** HIGH

## Executive Summary

The v1.3 Grafana Metrics Integration extends Spectre's progressive disclosure pattern from logs to metrics. Research recommends using custom HTTP client for Grafana API (official clients are immature), Prometheus official PromQL parser for metric extraction, existing FalkorDB patterns for graph storage, and custom statistical baseline for anomaly detection. This approach prioritizes production-ready libraries, avoids dependency bloat, and aligns with Spectre's existing architecture (FalkorDB integration, plugin system, MCP tools).

The key architectural insight is to parse PromQL at ingestion time (not query time) to build a semantic graph of Dashboard→Panel→Query→Metric→Service relationships. This enables intelligent queries like "show me all dashboards tracking pod memory" without re-parsing queries. The progressive disclosure model (overview→aggregated→details) mirrors the proven log exploration pattern and provides AI-driven anomaly detection with severity ranking as a competitive differentiator.

Critical risks include Grafana API version breaking changes (mitigated by storing raw dashboard JSON and defensive parsing), service account token scope confusion (mitigated by separate auth paths for Cloud vs self-hosted), and graph schema cardinality explosion (mitigated by storing structure only, not time-series data). The recommended approach avoids handwritten PromQL parsing (use official library), prevents variable interpolation edge cases (store separately, pass to API), and handles baseline drift with time-of-day matching for seasonality.

## Key Findings

### Recommended Stack

The technology stack prioritizes production-ready libraries with active maintenance, compatibility with Go 1.24+, and alignment with Spectre's existing patterns. No external services are required beyond Grafana API access and the already-deployed FalkorDB instance.

**Core technologies:**
- **Custom HTTP client (net/http)**: Grafana API access — official Go clients are deprecated or immature; custom client provides production control and matches existing integration patterns (VictoriaLogs, Logz.io)
- **prometheus/promql/parser**: PromQL parsing — official Prometheus library, production-proven, 556+ dependents; avoids handwritten parser complexity
- **FalkorDB (existing v2.0.2)**: Graph storage — already integrated; reuse existing patterns for Dashboard→Panel→Query→Metric relationships
- **Custom statistical baseline (stdlib math)**: Anomaly detection — z-score with time-of-day matching; simple, effective, no dependencies; defers ML complexity to future versions
- **SecretWatcher (existing pattern)**: Token management — Kubernetes-native hot-reload for Grafana API tokens; proven pattern from VictoriaLogs/Logz.io

**New dependencies needed:**
```bash
go get github.com/prometheus/prometheus/promql/parser@latest
```

All other components use stdlib (net/http, encoding/json, math, time) or existing dependencies.

### Expected Features

Research divides features into four categories: table stakes (users expect this), differentiators (competitive advantage), anti-features (explicitly avoid), and phase-specific (builds on foundation).

**Must have (table stakes):**
- Dashboard execution via API (fetch, parse, execute queries with time ranges)
- Basic variable support (single-value, simple substitution)
- RED method metrics (rate, errors, duration for request-driven services)
- USE method metrics (utilization, saturation, errors for resources)

**Should have (competitive differentiators):**
- AI-driven anomaly detection with severity ranking (statistical baseline, z-score, correlation)
- Intelligent variable scoping (classify as scope/entity/detail, auto-set defaults per tool level)
- Cross-signal correlation (metrics↔logs linking via shared namespace/time)
- Progressive disclosure pattern (overview→aggregated→details mirrors log exploration)

**Defer (v2+):**
- Advanced variable support (multi-value with pipe syntax, chained variables 3+ levels deep, query variables)
- Sophisticated anomaly detection (ML models, LSTM, adaptive baselines, root cause analysis)
- Trace linking (requires OpenTelemetry adoption)
- Dashboard management (create/edit/provision dashboards)

**Anti-features (explicitly avoid):**
- Dashboard UI replication (return structured data, not rendered visualizations)
- Custom dashboard creation via API (read-only access, users manage dashboards in Grafana)
- User-specific dashboard management (stateless MCP architecture, no per-user state)
- Full variable dependency resolution (support 2-3 levels, warn on deeper chaining)

### Architecture Approach

The Grafana integration follows Spectre's existing plugin architecture, extending it with six new components: dashboard sync, PromQL parser, graph storage schema, query executor, anomaly detector, and MCP tools. The design prioritizes incremental sync (only changed dashboards), structured graph queries (semantic relationships), and integration with existing infrastructure (FalkorDB, MCP server, plugin system).

**Major components:**
1. **GrafanaClient**: HTTP API wrapper for Grafana — handles authentication (Bearer token for Cloud, optional Basic auth for self-hosted), dashboard retrieval, query execution via `/api/ds/query`, rate limiting with exponential backoff
2. **DashboardSyncer**: Ingestion pipeline — incremental sync based on dashboard version, concurrent fetching with worker pool, change detection, batch graph writes in transactions
3. **PromQLParser**: Semantic extraction — uses Prometheus official parser to extract metric names, label selectors, aggregations, functions; stores results in graph for semantic queries
4. **GraphSchema**: Semantic relationships — Dashboard→Panel→Query→Metric→Service edges with CONTAINS, QUERIES, TRACKS relationships; stores structure only (no time-series data, no label values)
5. **QueryService**: Query execution — executes PromQL via Grafana API, formats results for MCP tools, performs graph queries for dashboard discovery ("show dashboards tracking this pod")
6. **AnomalyService**: Statistical detection — computes baselines (7-day history with time-of-day matching), calculates z-scores, classifies severity (info/warning/critical), caches baselines in graph (1-hour TTL)

**Data flow:**
- Ingestion: Poll Grafana API → parse dashboards → extract PromQL → build graph (Dashboard→Panel→Query→Metric→Service)
- Query: MCP tool → QueryService → Grafana API → format time series
- Anomaly: MCP tool → AnomalyService → compute baseline (cached) → query current → compare → rank by severity

**Graph schema strategy:**
Store structure (what exists), not data (metric values). Avoid cardinality explosion by creating nodes for Dashboard (dozens), Panel (hundreds), Query (hundreds), Metric template (thousands), Service (dozens) — NOT for individual time series (millions). Query actual metric values on-demand via Grafana API.

### Critical Pitfalls

Research identified 13 pitfalls ranging from critical (rewrites) to minor (annoyance). Top 5 require explicit mitigation in roadmap phases.

1. **Grafana API version breaking changes** — Dashboard JSON schema evolves between major versions (v11 URL changes, v12 schema format). Prevention: Store raw dashboard JSON before parsing, version detection via `schemaVersion` field, defensive parsing with optional fields, test against multiple Grafana versions (v9-v12 fixtures).

2. **Service account token scope confusion** — Cloud vs self-hosted have different auth methods (Bearer vs Basic) and permission scopes (service accounts lack Admin API access). Prevention: Detect Cloud via URL pattern, separate auth paths, minimal permissions (`dashboards:read` only), graceful degradation if optional APIs fail, clear error messages mapping 403 to actionable guidance.

3. **PromQL parser handwritten complexity** — PromQL has no formal grammar, official parser is handwritten with edge cases. Prevention: Use official `prometheus/promql/parser` library (do NOT write custom parser), best-effort extraction (complex expressions may not fully parse), variable interpolation passthrough (preserve `$var`, `[[var]]` as-is), focus on metric name extraction only.

4. **Graph schema cardinality explosion** — Creating nodes for every time series (metric × labels) explodes to millions of nodes. Prevention: Store structure only (Dashboard→Panel→Query→Metric template), do NOT create nodes for label values or time-series data, query actual metric values on-demand via Grafana API, limit to dozens of Dashboards/Services, hundreds of Panels/Queries, thousands of Metric templates.

5. **Anomaly detection baseline drift** — Simple rolling average ignores seasonality (weekday vs weekend) and concept drift (deployments change baseline). Prevention: Time-of-day matching (compare Monday 10am to previous Mondays at 10am), minimum deviation thresholds (absolute + relative), baseline staleness detection (warn if >14 days old), trend analysis for gradual degradation.

**Additional key pitfalls:**
- **Variable interpolation edge cases**: Multi-value variables use different formats per data source (`{job=~"(api|web)"}` for Prometheus). Store variables separately, do NOT interpolate during ingestion, pass to Grafana API during query.
- **Rate limiting**: Grafana Cloud has 600 requests/hour limit. Implement exponential backoff on 429, incremental ingestion (overview dashboards first), cache dashboard JSON, background sync.
- **Progressive disclosure state leakage**: Stateless MCP tools prevent concurrent session interference. Require scoping variables (cluster, namespace), AI manages context across calls, document drill-down pattern in tool descriptions.

## Implications for Roadmap

Based on research, v1.3 should follow a 5-phase structure that builds incrementally from foundation (HTTP client, graph schema) through ingestion (PromQL parsing, dashboard sync) to value delivery (MCP tools, anomaly detection). Each phase addresses specific features from FEATURES.md and mitigates pitfalls from PITFALLS.md.

### Phase 1: Foundation — Grafana API Client & Graph Schema
**Rationale:** Establish HTTP client and graph structure before ingestion logic. Grafana client handles auth complexity (Cloud vs self-hosted). Graph schema design prevents cardinality explosion (store structure, not data).

**Delivers:**
- GrafanaClient with authentication (Bearer token for Cloud, SecretWatcher integration)
- Graph schema nodes (Dashboard, Panel, Query, Metric, Service) with indexes
- Health checks and connectivity validation
- Integration lifecycle (Start/Stop/Health) and factory registration

**Addresses features:**
- Table stakes: Dashboard execution API access, basic connectivity
- Foundation for all other features

**Avoids pitfalls:**
- Pitfall 2 (token scope): Separate auth paths for Cloud vs self-hosted, minimal permissions
- Pitfall 4 (cardinality): Graph schema stores structure only, no time-series nodes
- Pitfall 7 (rate limiting): HTTP client with rate limiter, exponential backoff

**Confidence:** HIGH — HTTP client patterns proven in VictoriaLogs/Logz.io, graph schema extends existing FalkorDB patterns.

---

### Phase 2: Ingestion Pipeline — Dashboard Sync & PromQL Parsing
**Rationale:** Build ingestion before MCP tools. PromQL parsing enables semantic graph queries ("show dashboards tracking this metric"). Incremental sync handles large Grafana instances (100+ dashboards).

**Delivers:**
- DashboardSyncer with incremental sync (version-based change detection)
- PromQLParser using official Prometheus library (metric extraction)
- Dashboard→Panel→Query→Metric graph population
- Concurrent fetching (worker pool), batch graph writes (transactions)

**Addresses features:**
- Table stakes: Dashboard parsing, panel/query extraction
- Foundation for anomaly detection (need metrics in graph)

**Avoids pitfalls:**
- Pitfall 1 (API breaking changes): Store raw dashboard JSON, defensive parsing, version detection
- Pitfall 3 (PromQL parser): Use official library, best-effort extraction, variable passthrough
- Pitfall 6 (variable edge cases): Store variables separately, do NOT interpolate during ingestion
- Pitfall 7 (rate limiting): Incremental sync, concurrent fetching with QPS limit

**Uses stack:**
- `prometheus/promql/parser` (new dependency)
- FalkorDB batch writes via existing graph.Client

**Confidence:** HIGH — Incremental sync is standard pattern, PromQL parser is production-proven official library.

---

### Phase 3: Service Inference & Dashboard Hierarchy
**Rationale:** Build semantic relationships (Metric→Service, Dashboard hierarchy) before MCP tools. Service inference enables "show metrics for this service" queries. Dashboard hierarchy (overview/aggregated/detail tags) structures progressive disclosure.

**Delivers:**
- Service inference from PromQL labels (job, service, app, namespace, cluster)
- Metric→Service linking with confidence scores (TRACKS edges)
- Dashboard hierarchy classification (via tags: overview, aggregated, detail)
- Variable classification (scope/entity/detail) for smart defaults

**Addresses features:**
- Differentiator: Intelligent variable scoping (auto-classify variables)
- Foundation for progressive disclosure (need hierarchy)

**Avoids pitfalls:**
- Pitfall 5 (baseline drift): Service nodes enable per-service baselines (future)
- Pitfall 9 (label cardinality): Whitelist labels for service inference (job, service, app, namespace, cluster only)
- Pitfall 8 (gridPos): Use dashboard tags for hierarchy, not panel position

**Confidence:** MEDIUM-HIGH — Heuristic-based classification (80% accuracy expected), configurable via manual tags.

---

### Phase 4: Query Execution & MCP Tools Foundation
**Rationale:** Deliver basic MCP tools before anomaly detection. Enables AI to query metrics and discover dashboards. Tests end-to-end flow (client → parser → graph → tools).

**Delivers:**
- GrafanaQueryService (execute PromQL via Grafana API, format results)
- MCP tools: `grafana_{name}_dashboards` (list/search with filters)
- MCP tool: `grafana_{name}_query` (execute PromQL, return time series)
- MCP tool: `grafana_{name}_metrics_for_resource` (reverse lookup: resource → dashboards)

**Addresses features:**
- Table stakes: Dashboard execution, query execution with time ranges
- Progressive disclosure structure: Three tools (dashboards, query, metrics-for-resource)

**Avoids pitfalls:**
- Pitfall 10 (state leakage): Stateless MCP tools, require scoping variables, AI manages context
- Pitfall 6 (variable interpolation): Pass variables to Grafana API via `scopedVars`, not interpolated locally

**Uses stack:**
- GrafanaClient (query execution)
- FalkorDB (semantic queries for dashboard discovery)

**Confidence:** HIGH — MCP tool pattern proven in VictoriaLogs/Logz.io, stateless architecture established.

---

### Phase 5: Anomaly Detection & Progressive Disclosure
**Rationale:** Deliver competitive differentiator (anomaly detection) after foundation is stable. Progressive disclosure tools (overview/aggregated/details) complete the value proposition.

**Delivers:**
- GrafanaAnomalyService (baseline computation with time-of-day matching, z-score comparison)
- Baseline caching in graph (MetricBaseline nodes, 1-hour TTL)
- MCP tool: `grafana_{name}_detect_anomalies` (rank by severity)
- Progressive disclosure defaults per tool level (interval, limit)

**Addresses features:**
- Differentiator: AI-driven anomaly detection with severity ranking
- Differentiator: Progressive disclosure pattern (overview→aggregated→details)
- Differentiator: Cross-signal correlation (metrics + logs via shared namespace/time)

**Avoids pitfalls:**
- Pitfall 5 (baseline drift): Time-of-day matching, minimum thresholds, staleness detection
- Pitfall 13 (absent metrics): Check scrape status first (`up` metric), use `or vector(0)` pattern
- Pitfall 12 (histogram quantile): Validate `histogram_quantile()` wraps `sum() by (le)`

**Uses stack:**
- GrafanaQueryService (historical queries for baseline)
- stdlib math (mean, stddev, percentile calculations)
- FalkorDB (cache baselines)

**Confidence:** MEDIUM-HIGH — Statistical methods well-established, severity ranking heuristic needs tuning with real data.

---

### Phase Ordering Rationale

**Why this order:**
1. **Foundation first (Phase 1-2)**: HTTP client and graph schema are prerequisites for all other features. PromQL parsing enables semantic queries.
2. **Semantic layer (Phase 3)**: Service inference and hierarchy classification add intelligence to the graph before building tools on top.
3. **Basic tools (Phase 4)**: Deliver value early (query metrics, discover dashboards) before advanced features. Tests end-to-end flow.
4. **Differentiators last (Phase 5)**: Anomaly detection and progressive disclosure require stable foundation. These are competitive advantages, not MVP blockers.

**Why this grouping:**
- **Phase 1**: Auth complexity is separate concern from ingestion (different failure modes)
- **Phase 2**: Dashboard sync and PromQL parsing are tightly coupled (sync needs parser)
- **Phase 3**: Service inference depends on PromQL parsing (needs label extraction)
- **Phase 4**: MCP tools depend on query service (needs execution layer)
- **Phase 5**: Anomaly detection depends on query service (needs historical data)

**How this avoids pitfalls:**
- Early defensive parsing (Phase 2) catches API breaking changes before they block later phases
- Incremental sync (Phase 2) prevents rate limit exhaustion during initial ingestion
- Stateless tools (Phase 4) prevent progressive disclosure state leakage
- Time-of-day matching (Phase 5) mitigates baseline drift before anomaly detection ships

### Research Flags

**Phases likely needing deeper research during planning:**
- **Phase 3**: Service inference heuristics need validation with real-world dashboard corpus. Question: What % of dashboards use standard labels (job, service, app) vs custom labels? May need fallback discovery method (folder-based hierarchy) if tag adoption is low.
- **Phase 5**: Anomaly detection thresholds (z-score cutoffs, severity classification weights) are heuristic-based. Will need A/B testing with real metrics data to tune false positive rates.

**Phases with standard patterns (skip research-phase):**
- **Phase 1**: HTTP client follows VictoriaLogs/Logz.io pattern exactly. SecretWatcher is copy-paste.
- **Phase 2**: PromQL parser is well-documented official library. Incremental sync is standard pattern.
- **Phase 4**: MCP tool pattern proven in VictoriaLogs/Logz.io. Stateless architecture established.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Official Prometheus parser is production-proven (556+ dependents). FalkorDB already integrated. Custom HTTP client follows proven pattern. Only new dependency is PromQL parser. |
| Features | MEDIUM-HIGH | Table stakes verified with official Grafana docs. Differentiators (anomaly detection, progressive disclosure) based on industry best practices (RED/USE metrics, statistical baselines). MVP scope validated against competitive tools (Netdata, AWS Lookout). |
| Architecture | HIGH | Graph schema extends existing FalkorDB patterns (ResourceIdentity, ChangeEvent). MCP tool pattern proven in VictoriaLogs/Logz.io. Service layer follows existing TimelineService/GraphService design. Integration lifecycle matches plugin system. |
| Pitfalls | MEDIUM-HIGH | Critical pitfalls verified with official Grafana docs (API breaking changes, auth scope) and Prometheus GitHub issues (parser complexity). Anomaly detection seasonality is well-researched (O'Reilly book, research papers). Variable interpolation edge cases documented in Grafana issues. Some pitfalls (baseline tuning, variable chaining depth) need validation with real dashboards. |

**Overall confidence:** HIGH

The recommended stack is production-ready with minimal new dependencies. The architecture aligns perfectly with Spectre's existing patterns (FalkorDB, MCP tools, plugin system). The main uncertainties are heuristic-based (service inference, anomaly thresholds) which are tunable parameters, not architectural risks.

### Gaps to Address

**Validation needed during implementation:**
- **Variable chaining depth**: Research suggests 90% of dashboards use 0-3 levels of variable chaining, but this needs validation with real-world dashboard corpus (Grafana community library sample). If >10% use deeper chaining, Phase 2 may need scope expansion.
- **Dashboard tagging adoption**: Research shows tags are standard Grafana feature, but need to verify users already tag dashboards or if this is new practice. If low adoption, Phase 3 needs fallback discovery method (folder-based hierarchy).
- **Anomaly detection false positive rate**: Statistical methods (z-score, IQR) are well-established but thresholds (2.5 sigma vs 3.0 sigma) need tuning with production data. Plan for A/B testing in Phase 5.

**How to handle during planning:**
- Phase 2 planning: Include fixture dashboards with multi-value variables (2-3 levels deep) to validate parsing. Log warning if deeper chaining detected.
- Phase 3 planning: Document manual tagging workflow in UI. Design fallback: if no tags, classify by folder name patterns (overview, service, detail).
- Phase 5 planning: Make sensitivity thresholds configurable. Include "tune anomaly detection" task for post-MVP based on false positive feedback.

**Known limitations (document, do NOT block):**
- Multi-value variables deferred to post-MVP (can work around by AI providing single value)
- Query variables (dynamic) deferred to post-MVP (AI provides static values)
- Trace linking deferred (requires OpenTelemetry adoption, metrics+logs already valuable)

## Sources

### Primary (HIGH confidence)

**Grafana Official Documentation:**
- [Dashboard HTTP API](https://grafana.com/docs/grafana/latest/developers/http_api/dashboard/) — API endpoints, authentication, dashboard JSON structure
- [Data Source HTTP API](https://grafana.com/docs/grafana/latest/developers/http_api/data_source/) — Query execution, `/api/ds/query` format
- [Grafana Authentication](https://grafana.com/docs/grafana/latest/developer-resources/api-reference/http-api/authentication/) — Service accounts, Bearer tokens, permissions
- [Variables Documentation](https://grafana.com/docs/grafana/latest/visualizations/dashboards/variables/) — Template syntax, multi-value, chained variables
- [Dashboard Best Practices](https://grafana.com/docs/grafana/latest/visualizations/dashboards/build-dashboards/best-practices/) — Tags, organization, hierarchy

**Prometheus Official Documentation:**
- [PromQL Parser pkg.go.dev](https://pkg.go.dev/github.com/prometheus/prometheus/promql/parser) — API reference, AST structure
- [Prometheus Parser Source](https://github.com/prometheus/prometheus/blob/main/promql/parser/ast.go) — VectorSelector, AggregateExpr, Call structures

**FalkorDB Official Documentation:**
- [FalkorDB Design](https://docs.falkordb.com/design/) — Architecture, GraphBLAS backend, string interning
- [Cypher Support](https://docs.falkordb.com/cypher/cypher-support.html) — Supported Cypher syntax, indexes, transactions

### Secondary (MEDIUM confidence)

**Industry Best Practices:**
- [RED Method Monitoring](https://last9.io/blog/monitoring-with-red-method/) — Rate, errors, duration (table stakes for microservices)
- [Four Golden Signals](https://www.sysdig.com/blog/golden-signals-kubernetes) — USE method for resources
- [Getting Started with Grafana API](https://last9.io/blog/getting-started-with-the-grafana-api/) — Practical examples, authentication patterns

**Anomaly Detection Research:**
- [Netdata Anomaly Detection](https://learn.netdata.cloud/docs/netdata-ai/anomaly-detection) — Real-world implementation, severity ranking
- [AWS Lookout for Metrics](https://aws.amazon.com/lookout-for-metrics/) — Commercial product approach, baseline strategies
- [Time Series Anomaly Detection – ACM SIGMOD](https://wp.sigmod.org/?p=3739) — Statistical methods vs ML

**Progressive Disclosure UX:**
- [Progressive Disclosure (NN/G)](https://www.nngroup.com/articles/progressive-disclosure/) — UX patterns, drill-down hierarchy
- [Three Pillars of Observability](https://www.ibm.com/think/insights/observability-pillars) — Metrics, logs, traces correlation

### Tertiary (LOW-MEDIUM confidence)

**Grafana API Workarounds:**
- [Medium: Reverse Engineering Grafana API](https://medium.com/@mattam808/reverse-engineering-the-grafana-api-to-get-the-data-from-a-dashboard-48c2a399f797) — `/api/ds/query` undocumented structure
- [Grafana Community: Query /api/ds/query](https://community.grafana.com/t/query-data-from-grafanas-api-api-ds-query/143474) — Response format verification

**PromQL Edge Cases:**
- [Prometheus Issue #6256](https://github.com/prometheus/prometheus/issues/6256) — Parser complexity discussion, lack of formal grammar
- [VictoriaMetrics MetricsQL](https://github.com/VictoriaMetrics/metricsql) — Alternative parser, PromQL compatibility notes

**Emerging Patterns (2026 Trends):**
- [2026 Observability Trends](https://grafana.com/blog/2026-observability-trends-predictions-from-grafana-labs-unified-intelligent-and-open/) — Unified observability, AI integration
- [10 Observability Tools for 2026](https://platformengineering.org/blog/10-observability-tools-platform-engineers-should-evaluate-in-2026) — Industry direction

---
*Research completed: 2026-01-22*
*Ready for roadmap: yes*
