# Feature Landscape: Grafana Metrics Integration via MCP Tools

**Domain:** AI-assisted metrics exploration through Grafana dashboards
**Researched:** 2026-01-22
**Confidence:** MEDIUM (verified with official Grafana docs, WebSearch for emerging patterns)

## Executive Summary

Grafana metrics integration via MCP tools represents the next evolution of Spectre's progressive disclosure pattern (overview→patterns→logs becomes overview→aggregated→details for metrics). The feature landscape divides into four distinct categories:

1. **Table Stakes:** Dashboard execution, basic variable handling, RED/USE metrics
2. **Differentiators:** AI-driven anomaly detection with severity ranking, intelligent variable scoping, correlation with logs/traces
3. **Anti-Features:** Full dashboard UI replication, custom dashboard creation, user-specific dashboard management
4. **Phase-Specific:** Progressive disclosure implementation that mirrors log exploration patterns

This research informs v1.3 roadmap structure with clear MVP boundaries and competitive advantages over direct Grafana usage.

---

## Table Stakes

Features users expect from any Grafana metrics integration. Missing these = product feels incomplete.

### 1. Dashboard Execution via API

| Feature | Why Expected | Complexity | Implementation Notes |
|---------|--------------|------------|---------------------|
| Fetch dashboard JSON by UID | Core requirement for any programmatic access | Low | GET `/api/dashboards/uid/<uid>` - official API |
| Execute panel queries | Required to get actual metric data | Medium | POST `/api/tsdb/query` with targets array from dashboard JSON |
| Parse dashboard structure | Need to understand panels, variables, rows | Low | Dashboard JSON is well-documented schema |
| Handle multiple data sources | Real dashboards use Prometheus, CloudWatch, etc. | Medium | Extract `datasourceId` per panel, route queries appropriately |
| Time range parameterization | AI tools need to specify "last 1h" or custom ranges | Low | Standard `from`/`to` timestamp parameters |

**Source:** [Grafana Dashboard HTTP API](https://grafana.com/docs/grafana/latest/developer-resources/api-reference/http-api/dashboard/), [Getting Started with the Grafana API](https://last9.io/blog/getting-started-with-the-grafana-api/)

**Implementation Priority:** Phase 1 (foundation)
- Dashboard retrieval and JSON parsing
- Query extraction from panels
- Basic query execution with time ranges

### 2. Variable Templating Support

| Feature | Why Expected | Complexity | Implementation Notes |
|---------|--------------|------------|---------------------|
| Read dashboard variables | 90%+ of dashboards use variables | Medium | Extract from `templating` field in dashboard JSON |
| Substitute variable values | Queries contain `${variable}` placeholders | Medium | String replacement before query execution |
| Handle multi-value variables | Common pattern: `${namespace:pipe}` for filtering | High | Requires expansion logic for different formats |
| Support variable chaining | Variables depend on other variables (hierarchical) | High | Dependency resolution, 5-10 levels deep possible |
| Query variables (dynamic) | Variables populated from queries (most common type) | Medium | Execute variable query against data source |

**Source:** [Grafana Variables Documentation](https://grafana.com/docs/grafana/latest/visualizations/dashboards/variables/), [Chained Variables Guide](https://signoz.io/guides/how-to-make-grafana-template-variable-reference-another-variable-prometheus-datasource/)

**Implementation Priority:** Phase 2 (variable basics), Phase 3 (advanced chaining)
- Phase 2: Single-value variables, simple substitution
- Phase 3: Multi-value, chained variables, query variables

### 3. RED Method Metrics (Request-Driven Services)

| Feature | Why Expected | Complexity | Implementation Notes |
|---------|--------------|------------|---------------------|
| Rate (requests/sec) | Core SLI for services | Low | Typically `rate(http_requests_total[5m])` |
| Errors (error rate %) | Critical health indicator | Low | `rate(http_requests_total{status=~"5.."}[5m])` |
| Duration (latency p50/p95/p99) | User experience metric | Medium | `histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))` |

**Source:** [RED Method Monitoring](https://last9.io/blog/monitoring-with-red-method/), [RED Metrics Guide](https://www.splunk.com/en_us/blog/learn/red-monitoring.html)

**Why table stakes:** Google SRE's Four Golden Signals and RED method are industry-standard. Any metrics tool that doesn't surface these immediately feels incomplete for microservices monitoring.

### 4. USE Method Metrics (Resource-Centric Monitoring)

| Feature | Why Expected | Complexity | Implementation Notes |
|---------|--------------|------------|---------------------|
| Utilization (% busy) | Infrastructure health | Low | CPU/memory/disk utilization metrics |
| Saturation (queue depth) | Overload detection | Medium | Queue lengths, wait times |
| Errors (error count) | Hardware/resource failures | Low | Error counters at infrastructure level |

**Source:** [Mastering Observability: RED & USE](https://medium.com/@farhanramzan799/mastering-observability-in-sre-golden-signals-red-use-metrics-005656c4fe7d), [Four Golden Signals](https://www.sysdig.com/blog/golden-signals-kubernetes)

**Why table stakes:** RED for services, USE for infrastructure = complete coverage. Both needed for full-stack observability.

---

## Differentiators

Features that set Spectre apart from just using Grafana directly. Not expected, but highly valued.

### 1. AI-Driven Anomaly Detection with Severity Ranking

| Feature | Value Proposition | Complexity | Implementation Strategy |
|---------|-------------------|------------|------------------------|
| Automated anomaly detection | AI finds issues without writing PromQL | High | Statistical analysis on time series (z-score, IQR, rate-of-change) |
| Severity classification | Rank anomalies by impact | High | Score based on: deviation magnitude, metric criticality, error correlation |
| Node-level correlation | Connect anomalies across related metrics | Very High | TraceID/context propagation, shared labels (namespace, pod) |
| Novelty detection | Flag new metric patterns (like log patterns) | Medium | Compare current window to historical baseline (reuse pattern from logs) |
| Root cause hints | Surface likely causes based on correlation | Very High | Multi-metric correlation, temporal analysis |

**Source:** [Netdata Anomaly Detection](https://learn.netdata.cloud/docs/netdata-ai/anomaly-detection), [AWS Lookout for Metrics](https://aws.amazon.com/lookout-for-metrics/), [Anomaly Detection Metrics Research](https://arxiv.org/abs/2408.04817)

**Why differentiator:**
- Grafana shows data, you find anomalies manually
- Spectre + AI: "Show me the top 5 anomalies in prod-api namespace" → AI ranks by severity
- Competitive advantage: Proactive discovery vs reactive dashboard staring

**Implementation Approach:**
```
metrics_overview tool:
1. Execute overview dashboards (tagged "overview")
2. For each time series:
   - Calculate baseline (mean, stddev from previous window)
   - Detect deviations (z-score > 3, or rate-of-change > threshold)
   - Score severity: (deviation magnitude) × (metric weight) × (correlation to errors)
3. Return ranked anomalies with:
   - Metric name, current value, expected range
   - Severity score (0-100)
   - Correlated metrics (e.g., high latency + high error rate)
   - Suggested drill-down (link to aggregated/detail dashboards)
```

**Confidence:** MEDIUM - Statistical methods well-established, severity ranking is heuristic-based (needs tuning)

### 2. Intelligent Variable Scoping (Entity/Scope/Detail Classification)

| Feature | Value Proposition | Complexity | Implementation Strategy |
|---------|-------------------|------------|------------------------|
| Auto-classify variable types | AI understands namespace vs time_range vs detail_level | Medium | Heuristic analysis: common names, query patterns, cardinality |
| Scope variables (filtering) | namespace, cluster, region - reduce data volume | Low | Multi-value variables that filter entire dashboard |
| Entity variables (identity) | service_name, pod_name - what you're looking at | Low | Single-value variables that identify the subject |
| Detail variables (resolution) | aggregation_interval, percentile - how deep to look | Medium | Control granularity without changing what you're viewing |
| Smart defaults per tool level | overview=5m aggregation, details=10s aggregation | Medium | Tool-specific variable overrides based on progressive disclosure |

**Source:** [Grafana Variable Templating](https://grafana.com/docs/grafana/latest/visualizations/dashboards/variables/), [Chained Variables](https://signoz.io/guides/how-to-make-grafana-template-variable-reference-another-variable-prometheus-datasource/)

**Why differentiator:**
- Grafana requires manual variable selection
- Spectre: "Show metrics for prod-api service" → AI sets namespace=prod-api, time_range=1h, aggregation=5m automatically
- Progressive disclosure: overview tool uses coarse aggregation, details tool uses fine aggregation

**Implementation Approach:**
```
Variable classification (one-time per dashboard):
- Scope variables: Multi-value, used in WHERE clauses, low cardinality (<50 values)
  Examples: namespace, cluster, environment

- Entity variables: Single-value, identifies subject, medium cardinality (50-500)
  Examples: service_name, pod_name, node_name

- Detail variables: Control query resolution, very low cardinality (<10)
  Examples: interval, aggregation_window, percentile

Progressive disclosure defaults:
- overview: interval=5m, limit=10 panels
- aggregated: interval=1m, limit=50 panels, scope to single namespace
- details: interval=10s, all panels, scope to single service
```

**Confidence:** HIGH - Variable types are common patterns, defaults are configurable

### 3. Cross-Signal Correlation (Metrics ↔ Logs ↔ Traces)

| Feature | Value Proposition | Complexity | Implementation Strategy |
|---------|-------------------|------------|------------------------|
| Metrics → Logs drill-down | "High error rate" → show error logs from that time | Medium | Share namespace, time_range; call logs_overview with error filter |
| Logs → Metrics context | "Error spike in logs" → show related metrics (latency, CPU) | Medium | Reverse lookup: namespace in log → fetch service dashboards |
| Trace ID linking | Connect metric anomaly to distributed traces | High | Requires OpenTelemetry context propagation in metrics labels |
| Unified context object | Single time_range + namespace across all signals | Low | MCP tools already use this pattern (stateless with context) |
| Temporal correlation | Detect when metrics and logs spike together | Medium | Align time windows, compute correlation scores |

**Source:** [Three Pillars of Observability](https://www.ibm.com/think/insights/observability-pillars), [OpenTelemetry Correlation](https://www.dash0.com/knowledge/logs-metrics-and-traces-observability), [Unified Observability 2026](https://platformengineering.org/blog/10-observability-tools-platform-engineers-should-evaluate-in-2026)

**Why differentiator:**
- Grafana has separate metrics/logs/traces UIs, manual context switching
- Spectre: AI orchestrates across signals → "Show me metrics and logs for prod-api errors" executes both, correlates results
- 2026 trend: Unified observability is expected from modern tools

**Implementation Approach:**
```
Correlation via shared context:
1. AI provides context to each tool call: {namespace, time_range, filters}
2. metrics_overview detects anomaly at 14:32 UTC in prod-api namespace
3. AI automatically calls:
   - logs_overview(namespace=prod-api, time_range=14:30-14:35, severity=error)
   - metrics_aggregated(namespace=prod-api, time_range=14:30-14:35, dashboard=service-health)
4. AI synthesizes: "Latency spike (p95: 500ms→2000ms) coincides with 250 error logs"

Trace linking (future):
- Require OpenTelemetry semantic conventions: http.response.status_code, trace.id
- Store trace IDs in logs (already supported via VictoriaLogs)
- Link metrics label→trace ID→log trace_id field
```

**Confidence:** HIGH for metrics↔logs (already proven pattern), LOW for traces (needs OTel adoption)

### 4. Progressive Disclosure Pattern for Metrics

| Feature | Value Proposition | Complexity | Implementation Strategy |
|---------|-------------------|------------|------------------------|
| Overview dashboards (10k ft view) | See all services/clusters at a glance | Low | Execute dashboards tagged "overview", limit to summary panels |
| Aggregated dashboards (service-level) | Focus on one service, see all its metrics | Medium | Execute dashboards tagged "aggregated" or "service", filter to namespace |
| Detail dashboards (deep dive) | Full metrics for troubleshooting | High | Execute all panels, full variable expansion, fine granularity |
| Dashboard hierarchy via tags | User-configurable levels (not hardcoded) | Medium | Tag dashboards: `overview`, `aggregated`, `detail` |
| Auto-suggest next level | "High errors in prod-api" → suggest aggregated dashboard for prod-api | Medium | Anomaly detection triggers drill-down suggestion |

**Source:** [Progressive Disclosure UX](https://www.interaction-design.org/literature/topics/progressive-disclosure), [Grafana Dashboard Best Practices](https://grafana.com/docs/grafana/latest/visualizations/dashboards/build-dashboards/best-practices/), [Observability 2026 Trends](https://grafana.com/blog/2026-observability-trends-predictions-from-grafana-labs-unified-intelligent-and-open/)

**Why differentiator:**
- Grafana: flat list of dashboards, users navigate manually
- Spectre: structured exploration → overview finds problem → aggregated narrows scope → details diagnose root cause
- Mirrors proven log exploration pattern (overview→patterns→logs)

**Implementation Approach:**
```
Tool hierarchy (user provides context, tool determines scope):

metrics_overview:
  - Dashboards: tagged "overview" (cluster-level, namespace summary)
  - Variables: namespace=all, interval=5m
  - Panels: Limit to 10 most important (e.g., RED metrics only)
  - Anomaly detection: YES (rank namespaces/services by severity)
  - Output: List of namespaces with anomaly scores, suggest drill-down

metrics_aggregated:
  - Dashboards: tagged "aggregated" or "service"
  - Variables: namespace=<specific>, interval=1m
  - Panels: All panels for this service (RED, USE, custom metrics)
  - Correlation: YES (link to related dashboards, e.g., DB metrics if service uses DB)
  - Output: Time series for all metrics, correlated dashboards

metrics_details:
  - Dashboards: tagged "detail" or all dashboards for service
  - Variables: Full expansion (namespace, pod, container)
  - Panels: All panels, full resolution (interval=10s or as configured)
  - Variable expansion: Multi-value variables expanded (show per-pod metrics)
  - Output: Complete dashboard execution results

Dashboard tagging (user configuration):
- Users tag dashboards in Grafana: "overview", "aggregated", "detail"
- Spectre reads tags from dashboard JSON
- Flexible: One dashboard can have multiple tags (e.g., both aggregated and detail)
```

**Confidence:** HIGH - Pattern proven with logs, dashboard tagging is standard Grafana feature

---

## Anti-Features

Features to explicitly NOT build in v1.3. Common mistakes or out-of-scope for AI-assisted exploration.

### 1. Dashboard UI Replication

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Render dashboard visualizations | Grafana UI already exists; duplication is wasteful | Return structured data (JSON), let AI or user choose visualization |
| Build chart/graph rendering | Not the value prop; increases complexity 10x | Focus on data extraction and anomaly detection |
| Support all panel types | 50+ panel types (gauge, heatmap, etc.) = maintenance nightmare | Support query execution, ignore panel type (return raw time series) |

**Rationale:** Spectre is an MCP server for AI assistants, not a Grafana replacement. AI consumes structured data (time series arrays), not rendered PNGs. If users want pretty graphs, they open Grafana.

**Confidence:** HIGH - Clear product boundary

### 2. Custom Dashboard Creation/Editing

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|---|
| Create new dashboards via API | Out of scope for v1.3; users manage dashboards in Grafana | Read-only dashboard access, point users to Grafana for editing |
| Modify dashboard JSON | Requires full schema understanding, error-prone | Dashboards are immutable from Spectre's perspective |
| Save user preferences (default time ranges, etc.) | Adds state management, complicates architecture | Stateless tools: AI provides all context per call |

**Rationale:** Dashboards-as-code is a separate workflow (Terraform, Ansible, Grafana Provisioning). Spectre reads dashboards, doesn't manage them. Keep architecture stateless.

**Source:** [Observability as Code](https://grafana.com/docs/grafana/latest/as-code/observability-as-code/), [Dashboard Provisioning](https://grafana.com/tutorials/provision-dashboards-and-data-sources/)

**Confidence:** HIGH - Aligns with stateless MCP tool design

### 3. User-Specific Dashboard Management

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|---|
| Per-user dashboard favorites | Requires user identity, persistent storage | Global dashboard discovery via tags/folders |
| Personal dashboard customization | State management anti-pattern for MCP | AI remembers context within conversation, not across sessions |
| Dashboard sharing/collaboration | Grafana already has teams, folders, permissions | Respect Grafana's RBAC, use service account for read access |

**Rationale:** Spectre is a backend service, not a user-facing app. User identity and preferences belong in the frontend (AI assistant or UI), not the MCP server.

**Confidence:** HIGH - Architectural principle

### 4. Full Variable Dependency Resolution (Overly Complex Chaining)

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|---|
| Arbitrary depth variable chaining (10+ levels) | Complexity explosion; rare in practice | Support 2-3 levels (common case); warn if deeper |
| Circular dependency detection | Edge case; indicates misconfigured dashboard | Fail gracefully with error message |
| Variable value validation | Not Spectre's job; dashboards should be pre-validated | Trust dashboard configuration, surface query errors |

**Rationale:** 90% of dashboards use simple variables (0-3 levels deep). Supporting pathological cases (10-level chains, circular deps) adds complexity with minimal value. Focus on common patterns.

**Source:** [Chained Variables Guide](https://signoz.io/guides/how-to-make-grafana-template-variable-reference-another-variable-prometheus-datasource/) - mentions "5-10 levels deep technically possible" but warns about query load.

**Confidence:** MEDIUM - Need to validate with real-world dashboard corpus (could be MVP blocker if deep chaining is common)

---

## Feature Dependencies

Visualizing how features build on each other:

```
Foundation Layer (Phase 1):
  ├─ Dashboard JSON fetching
  ├─ Panel query extraction
  └─ Basic query execution (time range only)
       ↓
Variable Layer (Phase 2):
  ├─ Read dashboard variables
  ├─ Simple substitution (single-value)
  └─ Query variable execution
       ↓
Progressive Disclosure (Phase 3):
  ├─ Dashboard tagging/classification
  ├─ Tool-level scoping (overview/aggregated/details)
  ├─ Variable scoping (scope/entity/detail)
  └─ Smart defaults per tool
       ↓
Anomaly Detection (Phase 4):
  ├─ Statistical analysis on time series
  ├─ Severity scoring
  ├─ Correlation across metrics
  └─ Drill-down suggestions
       ↓
Cross-Signal Integration (Phase 5):
  ├─ Metrics → Logs linking
  ├─ Shared context object
  └─ Temporal correlation

Advanced Features (Post-v1.3):
  ├─ Multi-value variables
  ├─ Chained variables (3+ levels)
  ├─ Trace linking (requires OTel)
  └─ Custom anomaly algorithms
```

**Critical Path:** Foundation → Variables → Progressive Disclosure
- Can't do progressive disclosure without variables (need to scope dashboards)
- Can't do useful anomaly detection without progressive disclosure (need to limit search space)

**Parallelizable:** Anomaly detection and cross-signal correlation can develop in parallel once progressive disclosure is stable.

---

## MVP Recommendation

For v1.3 MVP, prioritize features that deliver immediate value while establishing foundation for future work.

### Include in v1.3 MVP:

1. **Dashboard Execution (Foundation)**
   - Fetch dashboard JSON by UID
   - Parse panels and extract queries
   - Execute queries with time range parameters
   - Return raw time series data

2. **Basic Variable Support**
   - Read single-value variables from dashboard
   - Simple string substitution (`${variable}` → value)
   - AI provides variable values (no query variables yet)

3. **Progressive Disclosure Structure**
   - Three MCP tools: `metrics_overview`, `metrics_aggregated`, `metrics_details`
   - Dashboard discovery via tags: "overview", "aggregated", "detail"
   - Tool-specific variable defaults (interval, limit)

4. **Simple Anomaly Detection**
   - Z-score analysis on time series (baseline from previous window)
   - Severity ranking by deviation magnitude
   - Return top N anomalies with current vs expected values

5. **Cross-Signal Context**
   - Shared context object: `{namespace, time_range, filters}`
   - AI orchestrates metrics + logs calls
   - Return correlation hints (temporal overlap)

**Why this scope:**
- Delivers core value: AI-assisted metrics exploration with anomaly detection
- Establishes progressive disclosure pattern (proven with logs)
- Enables cross-signal correlation (competitive advantage)
- Avoids complexity pitfalls (multi-value variables, deep chaining)

### Defer to Post-MVP:

1. **Advanced Variable Support**
   - Multi-value variables (`${namespace:pipe}` → `prod|staging|dev`)
   - Chained variables (3+ levels deep)
   - Query variables (execute queries to populate variable options)
   - **Reason:** 20% of dashboards use these; can work around with AI providing values

2. **Sophisticated Anomaly Detection**
   - Machine learning models (LSTM, isolation forests)
   - Root cause analysis (multi-metric correlation graphs)
   - Adaptive baselines (seasonality detection)
   - **Reason:** Statistical methods (z-score, IQR) provide 80% of value with 20% of complexity

3. **Trace Linking**
   - OpenTelemetry trace ID correlation
   - Distributed tracing integration
   - **Reason:** Requires instrumentation adoption; logs+metrics already valuable

4. **Dashboard Management**
   - Create/edit dashboards
   - Dashboard provisioning
   - **Reason:** Out of scope; users manage dashboards in Grafana

**Validation Criteria for MVP:**
- [ ] AI can ask: "Show metrics overview for prod cluster" → gets top 5 anomalies ranked by severity
- [ ] AI can drill down: "Show aggregated metrics for prod-api namespace" → gets service-level RED metrics
- [ ] AI can correlate: "Show metrics and logs for prod-api errors" → executes both, identifies temporal overlap
- [ ] Users can configure: Tag dashboards with "overview"/"aggregated"/"detail" → Spectre respects hierarchy

---

## Dashboard Operations Expected

Based on research, here's what operations should be available at each progressive disclosure level:

### Overview Level (Cluster/Multi-Namespace View)

| Operation | Input | Output | Use Case |
|-----------|-------|--------|----------|
| List namespaces with health | time_range, cluster | Namespace list with RED metrics summary | "Which namespaces have issues?" |
| Detect top anomalies | time_range, limit | Ranked anomalies across all dashboards | "What's broken right now?" |
| Compare namespaces | time_range, metric_type (RED/USE) | Side-by-side comparison table | "Which service is most impacted?" |
| Trend summary | time_range, aggregation | Time series for cluster-wide metrics | "Is error rate increasing over time?" |

**Dashboard Type:** Cluster overview, multi-namespace summary
**Example Dashboards:** "Kubernetes Cluster Overview", "Service Mesh Overview", "Platform RED Metrics"

### Aggregated Level (Single Namespace/Service)

| Operation | Input | Output | Use Case |
|-----------|-------|--------|----------|
| Service health deep-dive | namespace, time_range | All RED metrics for this service | "How is prod-api performing?" |
| Resource utilization | namespace, time_range | USE metrics for pods/containers | "Is prod-api resource-starved?" |
| Dependency metrics | namespace, time_range | Related services (DB, cache, downstream) | "Is the database slowing down prod-api?" |
| Historical comparison | namespace, time_range_current, time_range_baseline | Current vs baseline (e.g., same time yesterday) | "Is this normal for Monday morning?" |

**Dashboard Type:** Service-specific, namespace-scoped
**Example Dashboards:** "Service Health Dashboard", "Application Metrics", "Database Performance"

### Details Level (Single Pod/Full Resolution)

| Operation | Input | Output | Use Case |
|-----------|-------|--------|----------|
| Per-pod metrics | namespace, pod_name, time_range | All metrics for specific pod | "Why is pod-1234 failing?" |
| Full dashboard execution | dashboard_uid, variables, time_range | Complete time series for all panels | "Show me everything for this dashboard" |
| Variable expansion | dashboard_uid, variable_name | All possible values for this variable | "What pods exist in prod-api?" |
| Query-level execution | promql_query, time_range | Raw Prometheus query results | "Run this specific query" |

**Dashboard Type:** Full dashboards with all panels and variables
**Example Dashboards:** "Node Exporter Full", "Pod Metrics Detailed", "JVM Detailed Metrics"

---

## Variable Handling (Scoping, Entity, Detail Classifications)

Based on research, variables fall into three categories that map to progressive disclosure:

### Scope Variables (Filtering)

**Purpose:** Reduce data volume by filtering to a subset of entities

| Variable Name Examples | Cardinality | Type | How Used |
|----------------------|-------------|------|----------|
| `namespace`, `cluster`, `environment` | Low (5-50) | Multi-value | Filters entire dashboard to specific namespaces |
| `region`, `datacenter`, `availability_zone` | Low (3-20) | Multi-value | Geographic filtering |
| `team`, `owner`, `product` | Medium (10-100) | Multi-value | Organizational filtering |

**AI Behavior:**
- Overview tool: `namespace=all` (or top 10 by volume)
- Aggregated tool: `namespace=<specific>` (user/AI specifies)
- Details tool: `namespace=<specific>` (required)

**Implementation:**
- Multi-value variables use `|` separator in Prometheus: `{namespace=~"prod|staging"}`
- AI provides single value or list: `["prod", "staging"]`
- Tool expands to query syntax

### Entity Variables (Identity)

**Purpose:** Identify the specific thing being examined

| Variable Name Examples | Cardinality | Type | How Used |
|----------------------|-------------|------|----------|
| `service_name`, `app_name`, `deployment` | Medium (50-500) | Single-value | Identifies which service's metrics to show |
| `pod_name`, `container_name`, `node_name` | High (100-10k) | Single-value | Identifies specific instance |
| `job`, `instance` | Medium (20-1000) | Single-value | Prometheus scrape target identification |

**AI Behavior:**
- Overview tool: Not used (aggregate across all entities)
- Aggregated tool: `service_name=<specific>` (filters to one service)
- Details tool: `pod_name=<specific>` (filters to one pod)

**Implementation:**
- Single-value: `{service_name="prod-api"}`
- AI provides one value: `"prod-api"`
- Tool substitutes directly

### Detail Variables (Resolution Control)

**Purpose:** Control granularity and depth of data without changing scope

| Variable Name Examples | Cardinality | Type | How Used |
|----------------------|-------------|------|----------|
| `interval`, `aggregation_window`, `resolution` | Very Low (3-10) | Single-value | Controls Prometheus `rate()` window: `[5m]` vs `[10s]` |
| `percentile` | Very Low (3-5) | Single-value | Controls which percentile: `p50`, `p95`, `p99` |
| `aggregation_function` | Very Low (3-5) | Single-value | `sum`, `avg`, `max` for grouping |
| `limit`, `topk` | Very Low (5-20) | Single-value | How many results to return |

**AI Behavior:**
- Overview tool: `interval=5m`, `limit=10` (coarse, limited)
- Aggregated tool: `interval=1m`, `limit=50` (medium, broader)
- Details tool: `interval=10s`, `limit=all` (fine, complete)

**Implementation:**
- Substitution in query: `rate(metric[${interval}])`
- Tool-specific defaults override dashboard defaults
- AI can override for specific queries ("Show per-second rate" → `interval=1s`)

### Variable Classification Algorithm

For automatic classification of dashboard variables:

```
For each variable in dashboard:

1. Check variable name (heuristic):
   - Scope: contains "namespace", "cluster", "environment", "region"
   - Entity: contains "service", "pod", "container", "node", "app", "job"
   - Detail: contains "interval", "percentile", "resolution", "limit", "topk"

2. Check cardinality (execute variable query):
   - Low (<50): Likely scope or detail
   - Medium (50-500): Likely entity
   - High (>500): Likely entity (pod/container level)

3. Check multi-value flag:
   - Multi-value enabled: Likely scope
   - Single-value only: Likely entity or detail

4. Check usage in queries:
   - Used in WHERE clauses: Scope or entity
   - Used in function parameters: Detail
   - Used in aggregation BY: Scope

Final classification:
- If scope heuristic + multi-value → Scope
- If entity heuristic + single-value + medium/high cardinality → Entity
- If detail heuristic + low cardinality → Detail
- Else: Default to Scope (safest assumption)
```

**Confidence:** MEDIUM - Heuristics work for 80% of dashboards; edge cases need manual tagging

---

## Anomaly Detection (Types, Ranking, Surfacing)

Based on research into modern anomaly detection approaches:

### Anomaly Types to Detect

| Anomaly Type | Detection Method | Example | Severity Factor |
|--------------|------------------|---------|-----------------|
| **Threshold violation** | Current value > threshold | Error rate >5% | High if RED metric, Medium otherwise |
| **Deviation from baseline** | Z-score >3 or IQR outlier | Latency 2x higher than yesterday same time | High if >5σ, Medium if 3-5σ |
| **Rate-of-change spike** | Delta >X% per minute | CPU jumped 50% in 1 minute | High if critical resource (CPU/memory) |
| **Novel metric pattern** | New time series appears | New pod started emitting errors | Medium (investigate but may be expected) |
| **Missing data (flatline)** | No data points in window | Service stopped reporting metrics | Critical (likely outage) |
| **Correlated anomalies** | Multiple metrics spike together | High latency + high CPU + high error rate | Critical (systemic issue) |

**Source:** [Netdata Anomaly Detection](https://learn.netdata.cloud/docs/netdata-ai/anomaly-detection), [AWS Lookout for Metrics](https://aws.amazon.com/lookout-for-metrics/), [Anomaly Detection Research](https://arxiv.org/abs/2408.04817)

### Severity Ranking Algorithm

Rank anomalies using weighted scoring:

```python
def calculate_severity(anomaly, context):
    score = 0

    # 1. Deviation magnitude (0-40 points)
    if anomaly.type == "threshold_violation":
        score += 40  # Hard limit exceeded = max points
    elif anomaly.type == "deviation_from_baseline":
        z_score = anomaly.z_score
        score += min(40, z_score * 8)  # 5σ = 40 points
    elif anomaly.type == "rate_of_change":
        percent_change = anomaly.percent_change
        score += min(40, percent_change / 2)  # 100% change = 40 points

    # 2. Metric criticality (0-30 points)
    if anomaly.metric_type in ["error_rate", "success_rate"]:
        score += 30  # RED metrics = critical
    elif anomaly.metric_type in ["latency_p95", "latency_p99"]:
        score += 25  # Latency = important
    elif anomaly.metric_type in ["cpu_utilization", "memory_utilization"]:
        score += 20  # Resources = moderate
    else:
        score += 10  # Custom metrics = lower priority

    # 3. Correlation with errors (0-20 points)
    if context.has_error_logs:
        score += 20  # Logs confirm issue
    elif context.has_correlated_anomalies:
        score += 15  # Multiple metrics affected

    # 4. Duration (0-10 points)
    if anomaly.duration > 5 minutes:
        score += 10  # Sustained issue = higher severity
    elif anomaly.duration > 1 minute:
        score += 5  # Brief spike = moderate

    return min(100, score)  # Cap at 100
```

**Output Format:**
```json
{
  "anomalies": [
    {
      "metric": "http_request_duration_seconds_p95",
      "namespace": "prod-api",
      "severity_score": 85,
      "type": "deviation_from_baseline",
      "current_value": 2.5,
      "expected_range": [0.1, 0.5],
      "z_score": 8.2,
      "correlated_metrics": ["error_rate", "cpu_utilization"],
      "has_error_logs": true,
      "suggested_action": "Drill down to metrics_aggregated for prod-api namespace"
    }
  ]
}
```

**Confidence:** MEDIUM - Scoring weights are heuristic-based; need tuning with real data

### Surfacing Strategy

How to present anomalies to AI and users:

| Level | Strategy | Limit | Rationale |
|-------|----------|-------|-----------|
| **Overview** | Top 5 anomalies across all namespaces | 5 | AI attention is limited; show only critical issues |
| **Aggregated** | Top 10 anomalies for this namespace | 10 | More context available, can handle more detail |
| **Details** | All anomalies for this service/pod | No limit | Full diagnostic mode |

**Ranking Order:**
1. Sort by severity_score (desc)
2. Within same score, prioritize:
   - Correlated anomalies (multi-metric issues)
   - RED metrics (user-facing impact)
   - Sustained anomalies (duration >5 min)

**Progressive Disclosure Pattern:**
```
AI: "Show metrics overview for prod cluster"
→ metrics_overview returns top 5 anomalies
→ AI: "prod-api has high latency (severity 85)"

User: "Tell me more about prod-api"
→ AI calls metrics_aggregated(namespace=prod-api)
→ Returns top 10 anomalies for prod-api specifically
→ AI: "Latency correlates with high CPU and error rate spike at 14:32"

User: "Show full details"
→ AI calls metrics_details(namespace=prod-api, service=api-deployment)
→ Returns all metrics, all anomalies, full time series
→ AI: "Pod api-deployment-abc123 is using 95% CPU, causing cascading failures"
```

---

## Research Gaps and Open Questions

### HIGH Priority (Blockers for MVP)

1. **Variable chaining depth in real dashboards**
   - **Question:** What % of production dashboards use >3 levels of variable chaining?
   - **Why it matters:** Determines if we can defer complex chaining to post-MVP
   - **How to resolve:** Survey sample dashboards from Grafana community library
   - **Impact:** Could force Phase 2 scope expansion

2. **Dashboard tagging adoption**
   - **Question:** Do users already tag dashboards, or is this a new practice we're introducing?
   - **Why it matters:** Affects onboarding friction (existing vs new workflow)
   - **How to resolve:** Check Grafana community dashboards for tag usage patterns
   - **Impact:** May need fallback discovery method (folder-based hierarchy)

### MEDIUM Priority (Post-MVP Validation)

3. **Anomaly detection accuracy**
   - **Question:** Do statistical methods (z-score, IQR) produce acceptable false positive rates?
   - **Why it matters:** Too many false positives = users ignore anomaly detection
   - **How to resolve:** A/B test with real metrics data, tune thresholds
   - **Impact:** May need ML-based detection sooner than planned

4. **Query execution latency**
   - **Question:** Can we execute 10-50 dashboard panels in <5 seconds?
   - **Why it matters:** AI user experience requires fast responses
   - **How to resolve:** Benchmark with production Prometheus/Grafana instances
   - **Impact:** May need query batching, caching, or parallel execution

### LOW Priority (Future Work)

5. **Multi-data source support**
   - **Question:** How common are dashboards that mix Prometheus + CloudWatch + InfluxDB?
   - **Why it matters:** Affects data source abstraction layer complexity
   - **How to resolve:** Survey enterprise Grafana deployments
   - **Impact:** Deferred to v1.4 or later

---

## Sources

### Official Documentation (HIGH confidence)
- [Grafana Dashboard HTTP API](https://grafana.com/docs/grafana/latest/developer-resources/api-reference/http-api/dashboard/)
- [Grafana Variables Documentation](https://grafana.com/docs/grafana/latest/visualizations/dashboards/variables/)
- [Grafana Dashboard Best Practices](https://grafana.com/docs/grafana/latest/visualizations/dashboards/build-dashboards/best-practices/)
- [Dashboard JSON Model](https://grafana.com/docs/grafana/latest/visualizations/dashboards/build-dashboards/view-dashboard-json-model/)
- [Observability as Code](https://grafana.com/docs/grafana/latest/as-code/observability-as-code/)

### Industry Best Practices (MEDIUM confidence)
- [RED Method Monitoring | Last9](https://last9.io/blog/monitoring-with-red-method/)
- [RED Metrics Guide | Splunk](https://www.splunk.com/en_us/blog/learn/red-monitoring.html)
- [Four Golden Signals | Sysdig](https://www.sysdig.com/blog/golden-signals-kubernetes)
- [Mastering Observability: RED & USE | Medium](https://medium.com/@farhanramzan799/mastering-observability-in-sre-golden-signals-red-use-metrics-005656c4fe7d)
- [Getting Started with Grafana API | Last9](https://last9.io/blog/getting-started-with-the-grafana-api/)

### Research and Emerging Patterns (MEDIUM confidence)
- [Netdata Anomaly Detection](https://learn.netdata.cloud/docs/netdata-ai/anomaly-detection)
- [AWS Lookout for Metrics](https://aws.amazon.com/lookout-for-metrics/)
- [Anomaly Detection Severity Levels Research | ArXiv](https://arxiv.org/abs/2408.04817)
- [Three Pillars of Observability | IBM](https://www.ibm.com/think/insights/observability-pillars)
- [OpenTelemetry Correlation | Dash0](https://www.dash0.com/knowledge/logs-metrics-and-traces-observability)

### 2026 Trends (LOW-MEDIUM confidence - WebSearch)
- [2026 Observability Trends | Grafana Labs](https://grafana.com/blog/2026-observability-trends-predictions-from-grafana-labs-unified-intelligent-and-open/)
- [10 Observability Tools for 2026 | Platform Engineering](https://platformengineering.org/blog/10-observability-tools-platform-engineers-should-evaluate-in-2026)
- [Observability Predictions 2026 | Middleware](https://middleware.io/blog/observability-predictions/)
- [AI Trends for Autonomous IT 2026 | LogicMonitor](https://www.logicmonitor.com/blog/observability-ai-trends-2026)

### MCP and AI Patterns (MEDIUM confidence)
- [Building Smarter Dashboards with AI (MCP)](https://www.nobs.tech/blog/building-smarter-datadog-dashboards-with-ai)
- [Top 10 MCP Servers & Clients | DataCamp](https://www.datacamp.com/blog/top-mcp-servers-and-clients)
- [Microsoft Clarity MCP Server](https://clarity.microsoft.com/blog/introducing-the-microsoft-clarity-mcp-server-a-smarter-way-to-fetch-analytics-with-ai/)
- [Google Analytics MCP Server](https://ppc.land/google-analytics-experimental-mcp-server-enables-ai-conversations-with-data/)

### High Cardinality and Performance (MEDIUM confidence)
- [Managing High Cardinality Metrics | Grafana Labs](https://grafana.com/blog/2022/10/20/how-to-manage-high-cardinality-metrics-in-prometheus-and-kubernetes/)
- [Cardinality Management Dashboards | Grafana](https://grafana.com/docs/grafana-cloud/cost-management-and-billing/analyze-costs/metrics-costs/prometheus-metrics-costs/cardinality-management/)
- [Prometheus Cardinality in Practice | Medium](https://medium.com/@dotdc/prometheus-performance-and-cardinality-in-practice-74d5d9cd6230)

### Dashboard as Code and Organization (MEDIUM confidence)
- [Grafana Dashboards: Complete Guide | Grafana Labs](https://grafana.com/blog/2022/06/06/grafana-dashboards-a-complete-guide-to-all-the-different-types-you-can-build/)
- [Dashboards as Code Best Practices | Andreas Sommer](https://andidog.de/blog/2022-04-21-grafana-dashboards-best-practices-dashboards-as-code)
- [Three Years of Dashboards as Code | Kévin Gomez](https://blog.kevingomez.fr/2023/03/07/three-years-of-grafana-dashboards-as-code/)
- [Chained Variables Guide | SigNoz](https://signoz.io/guides/how-to-make-grafana-template-variable-reference-another-variable-prometheus-datasource/)

---

**End of FEATURES.md**
