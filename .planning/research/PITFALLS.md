# Domain Pitfalls: Grafana Metrics Integration

**Domain:** Grafana API integration, PromQL parsing, graph schema for observability, anomaly detection, progressive disclosure
**Researched:** 2026-01-22
**Confidence:** MEDIUM (WebSearch verified with official Grafana docs, PromQL GitHub issues, research papers)

## Critical Pitfalls

Mistakes that cause rewrites or major issues.

---

### Pitfall 1: Grafana API Version Breaking Changes

**What goes wrong:** Dashboard JSON schema changes between major Grafana versions break parsing logic. The dashboard schema changed significantly in v11 (URL structure for repeated panels) and v12 (new schema format).

**Why it happens:** Grafana's HTTP API follows alpha/beta/GA stability levels. Alpha APIs can have breaking changes without notice. GA APIs are stable but dashboard schema evolves independently.

**Consequences:**
- Dashboard ingestion fails silently when new schema fields appear
- Panel parsing breaks when `gridPos` structure changes
- Variable interpolation fails when template syntax evolves
- Repeated panel URLs (`&viewPanel=panel-5` → `&viewPanel=panel-3-clone1`) become invalid across versions

**Prevention:**
1. **Store raw dashboard JSON** — Always persist complete JSON before parsing. When parsing fails, fall back to raw storage and log for investigation.
2. **Version detection** — Check `schemaVersion` field (integer in dashboard JSON) and handle known versions explicitly.
3. **Defensive parsing** — Use optional field extraction. If `gridPos` is missing, infer from panel order. If `targets` array is empty, log warning but continue.
4. **Schema evolution tests** — Test against Grafana v9, v10, v11, v12 dashboard exports. Create fixture files for each.

**Detection:**
- Dashboard ingestion succeeds but panels array is empty
- Queries array exists but metric extraction returns zero results
- `schemaVersion` in logs is higher than tested versions

**Affected phases:** Phase 1 (Grafana client), Phase 2 (graph schema), Phase 6 (MCP tools)

**References:**
- [Grafana v11 Breaking Changes](https://grafana.com/docs/grafana/latest/breaking-changes/breaking-changes-v11-0/)
- [Dashboard JSON Schema](https://grafana.com/docs/grafana/latest/visualizations/dashboards/build-dashboards/view-dashboard-json-model/)
- [Schema V2 Resource](https://grafana.com/docs/grafana/latest/as-code/observability-as-code/schema-v2/)

---

### Pitfall 2: Service Account Token Scope Confusion

**What goes wrong:** Service account tokens created in Grafana Cloud have different permissions than self-hosted Grafana. Tokens work for dashboard reads but fail for Admin/User API endpoints. Authentication method (Basic auth vs Bearer) varies between Cloud and self-hosted.

**Why it happens:** Service accounts are limited to an organization and organization role. They cannot be granted Grafana server administrator permissions. Admin HTTP API and User HTTP API require Basic authentication with server admin role.

**Consequences:**
- Token works in development (self-hosted with admin user) but fails in production (Cloud with service account)
- User attempts to list all dashboards via Admin API but gets 403 Forbidden
- Dashboard export works but version history API fails (requires `dashboards:write` since Grafana v11)

**Prevention:**
1. **Separate auth paths** — Detect Grafana Cloud vs self-hosted via base URL pattern (`grafana.com` vs custom domain). Use Bearer token for Cloud, optionally support Basic auth for self-hosted.
2. **Minimal permissions** — Document required scopes: `dashboards:read` for ingestion. Do NOT require Admin API access.
3. **Graceful degradation** — If dashboard versions API fails (403), fall back to current version only. Log warning about missing permissions.
4. **Clear error messages** — Map 403 responses to actionable errors: "Service account needs 'dashboards:read' scope" vs "This endpoint requires server admin permissions (not available for service accounts)".

**Detection:**
- 403 Forbidden responses on API calls that worked in testing
- Error message contains "service account" or "organization role"
- Admin/User API endpoints fail while Dashboard API succeeds

**Affected phases:** Phase 1 (Grafana client), Phase 8 (UI configuration)

**References:**
- [Grafana API Authentication](https://grafana.com/docs/grafana/latest/developer-resources/api-reference/http-api/authentication/)
- [User HTTP API Limitations](https://grafana.com/docs/grafana/latest/developer-resources/api-reference/http-api/user/)
- [Dashboard Versions API Issue #100970](https://github.com/grafana/grafana/issues/100970)

---

### Pitfall 3: PromQL Parser Handwritten Complexity

**What goes wrong:** PromQL has no formal grammar definition. The official parser is a handwritten recursive-descent parser with "hidden features and edge cases that nobody is aware of." Building a custom parser leads to incompatibilities with valid PromQL.

**Why it happens:** PromQL evolved organically. The Prometheus team acknowledges that "none of the active members has a deep understanding of the parser code." Third-party parsers (Go, C#, Python) handle different edge cases differently.

**Consequences:**
- Valid PromQL from Grafana dashboard fails to parse: `rate(http_requests_total[5m])` works but `rate(http_requests_total{job=~"$job"}[5m])` breaks on variable interpolation
- Binary expression constraints are inconsistent: "comparisons between scalars must use BOOL modifier" but not enforced everywhere
- Nested function calls parse incorrectly: `histogram_quantile(0.95, sum(rate(...)) by (le))` loses grouping context

**Prevention:**
1. **Best-effort parsing** — Accept PROJECT.md constraint: "Complex expressions may not fully parse, extract what's possible." Do NOT attempt 100% PromQL compatibility.
2. **Use official parser** — Import `github.com/prometheus/prometheus/promql/parser` for Go. Do NOT write custom parser.
3. **Variable interpolation passthrough** — Detect Grafana variables (`$var`, `[[var]]`) and preserve as-is. Do NOT attempt to resolve during parsing.
4. **Metric name extraction only** — Focus on extracting metric names, label matchers (simple equality only), and aggregation functions. Skip complex binary expressions.
5. **Test with real dashboards** — Parse actual Grafana dashboard queries (from fixtures), not synthetic examples.

**Detection:**
- Parser returns error on query that works in Grafana
- Metric names extracted are empty when query clearly contains `rate(metric_name...)`
- Label filters are lost: `{job="api"}` becomes just the metric name

**Affected phases:** Phase 3 (PromQL parsing), Phase 4 (metric extraction)

**References:**
- [Prometheus Issue #6256: Replacing the PromQL Parser](https://github.com/prometheus/prometheus/issues/6256)
- [PromQL Parser Source](https://github.com/prometheus/prometheus/blob/main/promql/parser/parse.go)
- [VictoriaMetrics: PromQL Edge Cases](https://victoriametrics.com/blog/prometheus-monitoring-function-operator-modifier/)

---

### Pitfall 4: Graph Schema Cardinality Explosion

**What goes wrong:** Creating a node for every metric time series (e.g., `http_requests_total{job="api", status="200"}`) explodes graph size. 10K metrics × 100 label combinations = 1M nodes. FalkorDB traversals become slow.

**Why it happens:** Observability data has high cardinality. A single Prometheus instance can have millions of unique time series. Treating each series as a graph node ignores that time-series databases are purpose-built for this scale.

**Consequences:**
- Graph ingestion takes minutes instead of seconds
- Cypher queries timeout when traversing metric relationships
- Memory usage grows unbounded (1M nodes × avg 500 bytes = 500MB just for metric nodes)
- Dashboard hierarchy traversal (Overview→Detail) is slower than querying Grafana directly

**Prevention:**
1. **Schema hierarchy** — Store structure, not data:
   - **Dashboard** node (dozens): `{uid, title, tags, level: overview|aggregated|detail}`
   - **Panel** node (hundreds): `{id, title, type, gridPos}`
   - **Query** node (hundreds): `{refId, expr: raw PromQL, datasource}`
   - **Metric template** node (thousands): `{name: "http_requests_total", labels: ["job", "status"]}` — no label values
   - **Service** node (dozens): `{name: inferred from job/service label}`

2. **Do NOT create nodes for:**
   - Individual time series (e.g., `http_requests_total{job="api"}`)
   - Metric values or timestamps
   - Label value combinations

3. **Relationships:**
   - `(Dashboard)-[:CONTAINS]->(Panel)`
   - `(Panel)-[:QUERIES]->(Query)`
   - `(Query)-[:MEASURES]->(MetricTemplate)`
   - `(MetricTemplate)-[:BELONGS_TO]->(Service)` — inferred from labels

4. **Service inference** — Extract `job`, `service`, or `app` label from PromQL. Create single Service node per unique value.

5. **Metric values in Grafana** — Query actual time-series data via Grafana API on-demand. Graph only stores "what exists" not "what the values are."

**Detection:**
- Graph ingestion for 10 dashboards takes >30 seconds
- Node count exceeds 100K after ingesting <100 dashboards
- Memory usage grows proportional to number of unique label combinations

**Affected phases:** Phase 2 (graph schema design), Phase 5 (service inference)

**References:**
- [FalkorDB Design](https://docs.falkordb.com/design/)
- [Time Series Database Fundamentals](https://www.tigergraph.com/blog/time-series-database-fundamentals-in-modern-analytics/)
- [Graph Database Schema Best Practices](https://www.falkordb.com/blog/how-to-build-a-knowledge-graph/)

---

### Pitfall 5: Anomaly Detection Baseline Drift

**What goes wrong:** Anomaly detection compares current metrics to 7-day average but doesn't account for seasonality (weekday vs weekend) or concept drift (deployment changes baseline). Results in false positives ("CPU is high!" but it's Monday morning) or false negatives (gradual memory leak is "normal").

**Why it happens:** Time-series data has multiple seasonal patterns (hourly, daily, weekly). Simple rolling average doesn't distinguish "10am on Monday" from "2am on Sunday." Systems change over time (new features, scaling events) so old baselines become invalid.

**Consequences:**
- High false positive rate: 40% of anomalies are "it's just peak hours"
- Users ignore alerts (alert fatigue)
- Gradual degradation goes undetected: 2% daily memory leak over 7 days looks "normal"
- Seasonal patterns (Black Friday, end-of-quarter) trigger false alarms

**Prevention:**
1. **Time-of-day matching** — Compare current value to same time-of-day in previous 7 days:
   - Current: Monday 10:15am
   - Baseline: Average of [last Monday 10:15am, 2 Mondays ago 10:15am, ...]
   - Use 1-hour window around target time to handle small time shifts

2. **Minimum deviation threshold** — Only flag as anomaly if:
   - Absolute deviation: `|current - baseline| > threshold` (e.g., 1000 requests/sec)
   - AND relative deviation: `|(current - baseline) / baseline| > percentage` (e.g., 50%)
   - This prevents "CPU is 0.1% higher!" false positives

3. **Baseline staleness detection** — If baseline data is >14 days old or has gaps, warn "insufficient data for anomaly detection" instead of showing false confidence.

4. **Trend analysis (future enhancement)** — Detect monotonic increase/decrease over 7 days using linear regression. If slope is significant, flag "trending up" instead of "anomaly."

5. **Manual thresholds** — Allow users to set expected ranges per metric in dashboard tags (e.g., `threshold:cpu_90%`). Use as override for ML-based detection.

6. **STL decomposition (advanced)** — For high-confidence metrics, use Seasonal-Trend decomposition (Loess) to separate trend, seasonality, and residuals. Detect anomalies in residuals only.

**Detection:**
- Anomaly alerts correlate with known patterns (time of day, day of week)
- False positive rate >20% when validating against known incidents
- Users report "anomaly detection is always wrong"

**Affected phases:** Phase 7 (anomaly detection), Phase 6 (MCP tools show anomaly scores)

**References:**
- [Dealing with Trends and Seasonality](https://www.oreilly.com/library/view/anomaly-detection-for/9781492042341/ch04.html)
- [OpenSearch: Reducing False Positives](https://opensearch.org/blog/reducing-false-positives-through-algorithmic-improvements/)
- [Anomaly Detection: How to Tell Good from Bad](https://towardsdatascience.com/anomaly-detection-how-to-tell-good-performance-from-bad-b57116d71a10/)
- [Time Series Anomaly Detection Seasonality](https://milvus.io/ai-quick-reference/how-does-anomaly-detection-handle-seasonal-patterns)

---

## Moderate Pitfalls

Mistakes that cause delays or technical debt.

---

### Pitfall 6: Grafana Variable Interpolation Edge Cases

**What goes wrong:** Grafana template variables have multiple syntaxes (`$var` vs `[[var]]`) and formatting options (`${var:csv}`, `${var:regex}`). Multi-value variables interpolate differently per data source (Prometheus uses regex, InfluxDB uses OR clauses). Custom "All" values (`*` vs concatenated values) break when used incorrectly.

**Why it happens:** Variable interpolation happens at Grafana query time, not dashboard storage time. Different data sources have different query languages, so Grafana transforms variables differently. The `[[var]]` syntax is deprecated but still appears in old dashboards.

**Consequences:**
- Query stored as `{job=~"$job"}` but when executed with multi-select, becomes `{job=~"(api|web)"` (correct) or `{job=~"api,web"}` (broken regex)
- Custom "All" value of `.*` works for Prometheus but breaks for exact-match databases
- Variable extraction from PromQL during parsing returns `$job` instead of actual values, breaking service inference

**Prevention:**
1. **Store variables separately** — Extract dashboard `templating.list` into separate Variable nodes in graph: `{name: "job", type: "query", multi: true, includeAll: true}`
2. **Do NOT interpolate during ingestion** — Keep queries as-is with `$var` placeholders. Grafana API handles interpolation during query execution.
3. **Pass variables to Grafana API** — When querying metrics, include `scopedVars` in `/api/ds/query` request body with AI-provided values.
4. **Document variable types** — In graph schema, classify variables:
   - **Scoping** (namespace, cluster, region): AI provides per MCP call
   - **Entity** (pod, service): Used for drill-down
   - **Detail** (time range, aggregation): Controls visualization

5. **Test multi-value variables** — Create fixture dashboard with `job` variable set to multi-select. Verify query execution returns results for all selected values.

**Detection:**
- Queries return zero results when variable is multi-select
- Service inference extracts `$job` as literal string instead of recognizing as variable
- Regex errors in Prometheus logs: "invalid regexp: api,web"

**Affected phases:** Phase 3 (PromQL parsing), Phase 4 (variable classification), Phase 6 (MCP query execution)

**References:**
- [Prometheus Template Variables](https://grafana.com/docs/grafana/latest/datasources/prometheus/template-variables/)
- [Variable Syntax](https://grafana.com/docs/grafana/latest/visualizations/dashboards/variables/variable-syntax/)
- [GitHub Issue #93776: Variable Formatter](https://github.com/grafana/grafana/issues/93776)

---

### Pitfall 7: Rate Limiting and Pagination Gaps

**What goes wrong:** Grafana Cloud API has rate limits (600 requests/hour for access policies). Large organizations with hundreds of dashboards hit limits during initial ingestion. Grafana API lacks pagination for dashboard list endpoints (default max 5000 data sources, no explicit dashboard limit documented).

**Why it happens:** Grafana API evolved for interactive use (humans clicking UI) not bulk automation. Rate limits prevent API abuse but block legitimate batch operations like "ingest all dashboards."

**Consequences:**
- Initial ingestion of 200 dashboards × 5 panels × 3 queries = 3000 API calls, hits rate limit
- Dashboard list returns first 5000 results, silently truncates rest
- Concurrent dashboard ingestion from multiple Spectre instances triggers rate limit

**Prevention:**
1. **Batch dashboard fetching** — Use `/api/search` endpoint with `type=dash-db` to list all dashboards, then fetch each full dashboard via `/api/dashboards/uid/:uid`. Do NOT fetch via `/api/dashboards/db/:slug` (deprecated).
2. **Rate limit backoff** — Detect 429 Too Many Requests response. Implement exponential backoff: wait 60s, then 120s, then 240s. Log "rate limited, retrying..." to UI.
3. **Incremental ingestion** — On first run, ingest dashboards tagged `overview` only (typically <20). Full ingestion happens in background with rate limiting.
4. **Cache dashboard JSON** — After initial fetch, only re-fetch if dashboard `version` changed (check via lightweight `/api/search` which includes version field).
5. **Pagination detection** — Check if dashboard list length equals suspected page size (e.g., 1000, 5000). Log warning "possible truncation, verify all dashboards ingested."

**Detection:**
- 429 response codes in logs
- Dashboard count in Spectre doesn't match Grafana UI count
- Ingestion stops midway with "rate limit exceeded" error

**Affected phases:** Phase 1 (Grafana client), Phase 2 (dashboard ingestion)

**References:**
- [Grafana Cloud API Rate Limiting](https://drdroid.io/stack-diagnosis/grafana-grafana-api-rate-limiting)
- [Data Source HTTP API Pagination](https://grafana.com/docs/grafana/latest/developers/http_api/data_source/)
- [Infinity Datasource Pagination Limits](https://github.com/grafana/grafana-infinity-datasource/discussions/601)

---

### Pitfall 8: Panel gridPos Negative Gravity

**What goes wrong:** Dashboard panel layout uses `gridPos` with coordinates `{x, y, w, h}` where the grid has "negative gravity" — panels automatically move upward to fill gaps. When programmatically modifying layouts or calculating panel importance, Y-coordinate alone doesn't indicate visual hierarchy.

**Why it happens:** Grafana UI auto-arranges panels to eliminate whitespace. When a panel is deleted, panels below move up. The stored `gridPos.y` reflects final position after gravity, not intended hierarchy.

**Consequences:**
- Importance ranking "first panel is overview" breaks when top panel is full-width (y=0) but second panel also has y=0 (placed to the right, not below)
- Panel reconstruction from graph fails to maintain visual layout
- Drill-down relationships inferred from position are incorrect: "panel at y=5 drills into panel at y=10" but they're actually side-by-side

**Prevention:**
1. **Sort by y then x** — When ranking panels by importance: sort by `gridPos.y` ascending, then `gridPos.x` ascending. This gives reading order (left-to-right, top-to-bottom).
2. **Use panel type as signal** — "Row" panels (type: "row") group related panels. Panel immediately after a row is child of that row.
3. **Rely on dashboard tags** — Use Grafana tags or dashboard JSON `tags` field for explicit hierarchy (`overview`, `detail`), not inferred from layout.
4. **Store original gridPos** — When saving to graph, preserve exact `gridPos` for reconstruction. Do NOT recalculate positions.

**Detection:**
- Panel importance ranking shows "graph" panel before "singlestat" panel when visual hierarchy is opposite
- Dashboard reconstruction places panels in wrong positions
- Drill-down links go to unrelated panels

**Affected phases:** Phase 2 (graph schema), Phase 5 (dashboard hierarchy inference)

**References:**
- [Dashboard JSON Model](https://grafana.com/docs/grafana/latest/visualizations/dashboards/build-dashboards/view-dashboard-json-model/)
- [Dashboard JSON Structure](https://yasoobhaider.medium.com/using-grafana-json-model-howto-509aca3cf9a9)

---

### Pitfall 9: PromQL Label Cardinality Mistakes

**What goes wrong:** Developers add high-cardinality labels to metrics (`user_id`, `request_id`, `trace_id`) causing millions of time series. Queries like `rate(http_requests{trace_id=~".*"}[5m])` timeout or OOM. Service inference from labels fails when label values are unbounded.

**Why it happens:** Prometheus best practices warn against high cardinality but Grafana dashboards may query external systems (Thanos, Mimir) with poor label hygiene. Every unique label combination creates a new time series in memory.

**Consequences:**
- Queries timeout after 30 seconds
- Prometheus memory usage spikes to 10GB+ for simple `rate()` query
- Service inference extracts 100K "services" from `trace_id` label instead of 10 services from `job` label
- Grafana API returns partial results or errors

**Prevention:**
1. **Label validation during ingestion** — When parsing PromQL, extract label matchers. If label name matches high-cardinality patterns (`*_id`, `trace_*`, `span_*`, `uuid`, `session`), log warning: "High-cardinality label detected in dashboard '{dashboard}', panel '{panel}'"
2. **Service inference whitelist** — Only infer services from known-good labels: `job`, `service`, `app`, `application`, `namespace`, `cluster`. Ignore all other labels.
3. **Query timeout enforcement** — Set Grafana query timeout to 30s (via `/api/ds/query` request). If query times out, show "query too slow" instead of crashing.
4. **Pre-aggregation hints** — Detect queries missing aggregation: `http_requests_total` without `sum()` or `rate()`. Log warning "query may return too many series."

**Detection:**
- Grafana queries return 429 "too many series" errors
- Service node count in graph is >1000 (should be <100 for typical setup)
- Query execution logs show "timeout" or "OOM"

**Affected phases:** Phase 3 (PromQL parsing), Phase 5 (service inference), Phase 7 (anomaly detection queries)

**References:**
- [3 Common Mistakes with PromQL](https://home.robusta.dev/blog/3-common-mistakes-with-promql-and-kubernetes-metrics)
- [PromQL Best Practices](https://last9.io/blog/promql-cheat-sheet/)

---

### Pitfall 10: Progressive Disclosure State Leakage

**What goes wrong:** Progressive disclosure (overview → aggregated → details) requires maintaining context across MCP tool calls. If state is stored server-side (e.g., "user selected cluster X in overview, now calling aggregated"), concurrent AI sessions interfere. If state is AI-managed, AI forgets context and calls details tool without scoping variables.

**Why it happens:** Spectre follows stateless MCP architecture (per PROJECT.md: "AI passes filters per call, no server-side session state"). But progressive disclosure implies stateful flow: overview picks service → aggregated shows correlations → details expands dashboard.

**Consequences:**
- AI calls `metrics_aggregated` without cluster/namespace, returns aggregated results across ALL clusters (too broad, slow)
- Concurrent Claude sessions: User A selects "prod" cluster, User B selects "staging", both get same results (state collision)
- AI forgets to pass scoping variables from overview to details: "show me details for service X" but doesn't include `cluster=prod` from previous call

**Prevention:**
1. **Stateless MCP tools** — Already implemented. Each tool call is independent, all filters passed as parameters.
2. **AI context management** — Document in MCP tool descriptions: "To drill down, copy scoping variables (cluster, namespace, service) from overview response and pass to aggregated/details calls."
3. **Require scoping variables** — Make `cluster` or `namespace` a required parameter for `metrics_aggregated` and `metrics_details` tools. Return error if missing.
4. **Prompt engineering** — MCP tool response includes reminder: "To see details for service 'api', call metrics_details with cluster='prod', namespace='default', service='api'."
5. **Test multi-turn conversations** — E2E test: AI calls overview → picks service → calls aggregated with correct scoping → calls details. Verify no state leakage.

**Detection:**
- AI calls `metrics_details` without scoping, returns "too many results" or timeout
- Multiple AI sessions report unexpected results (sign of shared state)
- Logs show tool calls with missing required parameters

**Affected phases:** Phase 6 (MCP tool design), Phase 8 (UI integration)

**References:**
- [Progressive Disclosure NN/G](https://www.nngroup.com/articles/progressive-disclosure/)
- [Progressive Disclosure Pitfalls](https://userpilot.com/blog/progressive-disclosure-examples/)
- [B2B SaaS UX 2026](https://www.onething.design/post/b2b-saas-ux-design)

---

## Minor Pitfalls

Mistakes that cause annoyance but are fixable.

---

### Pitfall 11: Dashboard JSON Comment and Whitespace Loss

**What goes wrong:** Dashboard JSON may include comments (via `__comment` fields) or custom formatting (indentation, field ordering). When parsing dashboard → storing in graph → reconstructing JSON, comments and formatting are lost.

**Why it happens:** JSON parsers discard comments and reformat. Grafana dashboard export uses custom field ordering (e.g., `id` before `title`) but Go `json.Marshal` uses alphabetical order.

**Consequences:**
- Users export dashboard from Spectre, lose original comments and formatting
- Git diffs show entire file changed even when only one panel modified (due to field reordering)
- Minor annoyance, not functionality break

**Prevention:**
1. **Store raw JSON** — Always preserve original dashboard JSON in graph or database. When exporting, return raw JSON instead of reconstructed.
2. **Do NOT reconstruct JSON** — Parsing is for graph population only, not for round-trip export.
3. **Document limitation** — If export is needed, add note: "Exported dashboards may have different formatting than original."

**Detection:**
- User reports "exported dashboard lost my comments"
- Git diff shows reformatted JSON

**Affected phases:** Phase 2 (dashboard storage)

**References:**
- [PromQL Parser C# Limitations](https://github.com/djluck/PromQL.Parser)

---

### Pitfall 12: Histogram Quantile Misuse

**What goes wrong:** Developers use `histogram_quantile()` on already-aggregated data or forget `le` label, producing nonsensical results. Example: `histogram_quantile(0.95, rate(http_duration_bucket[5m]))` without `sum() by (le)`.

**Why it happens:** Histogram metrics require specific aggregation patterns. Prometheus histograms use `_bucket` suffix with `le` (less than or equal) labels. Incorrect aggregation loses bucket boundaries.

**Consequences:**
- 95th percentile shows 0.0 or NaN
- Anomaly detection on latency percentiles fails

**Prevention:**
1. **Template detection** — When parsing PromQL, detect `histogram_quantile()`. Verify it wraps `sum(...) by (le)` or `rate(...[...]) by (le)`. Log warning if missing.
2. **Documentation** — When displaying histogram metrics in MCP tools, show note: "Percentile calculated from histogram buckets."

**Detection:**
- PromQL contains `histogram_quantile` without `by (le)`
- Query returns NaN or 0 for percentile metrics

**Affected phases:** Phase 3 (PromQL parsing validation)

**References:**
- [PromQL Tutorial: Histograms](https://coralogix.com/blog/promql-tutorial-5-tricks-to-become-a-prometheus-god/)
- [PromQL Cheat Sheet](https://promlabs.com/promql-cheat-sheet/)

---

### Pitfall 13: Absent Metric False Positives

**What goes wrong:** Anomaly detection flags "metric missing" when metric is legitimately zero (e.g., `error_count=0` during healthy period). Using `absent()` function detects truly missing metrics but doesn't distinguish from zero values.

**Why it happens:** Prometheus doesn't store zero-value counters. If `http_errors_total` has no errors, the metric doesn't exist in TSDB. `absent(metric)` returns 1 (true) both when metric never existed and when it's currently zero.

**Consequences:**
- Alert fatigue: "error_count missing!" every time there are no errors
- Cannot distinguish "scrape failed" from "no errors"

**Prevention:**
1. **Check scrape status first** — Query `up{job="..."}` metric. If 0, scrape failed. If 1 but metric missing, it's legitimately zero.
2. **Use `or vector(0)`** — PromQL pattern: `metric_name or vector(0)` returns 0 when metric absent.
3. **Baseline staleness** — Only flag missing if metric existed in previous 7 days. New services won't trigger false alerts.

**Detection:**
- Anomaly alerts during healthy periods: "error rate missing"
- `absent()` queries return 1 constantly

**Affected phases:** Phase 7 (anomaly detection)

**References:**
- [PromQL Tricks: Absent](https://last9.io/blog/promql-tricks-you-should-know/)

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| **Phase 1: Grafana Client** | Service account token vs Basic auth confusion | Detect Cloud vs self-hosted via URL pattern. Use Bearer token for Cloud. Document required scopes. |
| **Phase 2: Graph Schema** | Cardinality explosion from storing time-series nodes | Store structure only: Dashboard→Panel→Query→MetricTemplate. NO nodes for label values or metric data. |
| **Phase 3: PromQL Parsing** | Handwritten parser incompatibilities | Use official `prometheus/promql/parser` package. Best-effort extraction. Preserve variables as-is. |
| **Phase 4: Variable Classification** | Multi-value variable interpolation breaks | Store variables separately. Do NOT interpolate during ingestion. Pass to Grafana API during query. |
| **Phase 5: Service Inference** | High-cardinality labels (trace_id) become "services" | Whitelist: only infer from `job`, `service`, `app`, `namespace`, `cluster` labels. |
| **Phase 6: MCP Tools** | Progressive disclosure state leakage | Stateless tools. Require scoping variables. AI manages context. Test multi-turn conversations. |
| **Phase 7: Anomaly Detection** | Seasonality false positives | Time-of-day matching. Minimum deviation thresholds. Trend detection. Manual overrides. |
| **Phase 8: UI Configuration** | Rate limit exhaustion during initial ingestion | Incremental ingestion. Backoff on 429. Cache dashboards. Background sync. |

---

## Integration with Existing Spectre Patterns

### Patterns to Apply from v1.2 (Logz.io) and v1.1

**Secret management (v1.2):**
- SecretWatcher with SharedInformerFactory for Kubernetes-native hot-reload
- Grafana API token can use same pattern: store in Secret, reference via `SecretRef{Name, Key}`
- **Apply to:** Phase 1 (Grafana client auth)

**Hot-reload with fsnotify (v1.1):**
- IntegrationWatcher with debouncing (500ms) prevents reload storms
- Invalid configs logged but don't crash watcher
- **Apply to:** Phase 8 (Grafana config updates trigger re-ingestion)

**Best-effort parsing (VictoriaLogs):**
- LogsQL query builder gracefully handles missing fields
- Falls back to defaults when validation fails
- **Apply to:** Phase 3 (PromQL parsing — not all expressions need to parse perfectly)

**Progressive disclosure (v1.2):**
- overview → patterns → logs model already implemented for VictoriaLogs and Logz.io
- Stateless MCP tools with AI-managed context
- **Apply to:** Phase 6 (metrics_overview → metrics_aggregated → metrics_details)

**Graph storage (v1):**
- FalkorDB already stores Kubernetes resource relationships
- Node-edge model for hierarchical data
- **Apply to:** Phase 2 (Dashboard→Panel→Query→Metric graph schema)

### New Patterns for Grafana Integration

**Time-of-day baseline matching:**
- New requirement for anomaly detection
- VictoriaLogs pattern comparison is simpler (previous window only)
- **Implement in:** Phase 7 with time bucketing logic

**Variable classification:**
- Distinguish scoping (cluster, namespace) from entity (pod, service) from detail (time range)
- New concept not needed for log integrations
- **Implement in:** Phase 4 as metadata on Variable nodes

**Service inference from labels:**
- Graph schema needs Service nodes inferred from PromQL labels
- Kubernetes resources have explicit Service objects, metrics do not
- **Implement in:** Phase 5 with label whitelist

---

## Verification Checklist

Before proceeding to roadmap creation:

- [ ] Grafana client handles both Cloud (Bearer token) and self-hosted (Basic auth optional)
- [ ] Graph schema stores structure (Dashboard/Panel/Query/Metric) not time-series data
- [ ] PromQL parsing uses official `prometheus/promql/parser` package
- [ ] Variable interpolation preserved, passed to Grafana API during query execution
- [ ] Service inference only from whitelisted labels (job, service, app, namespace, cluster)
- [ ] Anomaly detection uses time-of-day baseline matching with minimum thresholds
- [ ] MCP tools are stateless, require scoping variables, AI manages context
- [ ] Rate limiting handled with backoff, incremental ingestion, caching
- [ ] Dashboard JSON stored raw for version compatibility
- [ ] E2E tests include multi-value variables, histogram metrics, high-cardinality label detection

---

## Sources

**Grafana API & Authentication:**
- [Grafana API Authentication Methods](https://grafana.com/docs/grafana/latest/developer-resources/api-reference/http-api/authentication/)
- [User HTTP API Limitations](https://grafana.com/docs/grafana/latest/developer-resources/api-reference/http-api/user/)
- [Breaking Changes in Grafana v11](https://grafana.com/docs/grafana/latest/breaking-changes/breaking-changes-v11-0/)
- [Dashboard Versions API Issue #100970](https://github.com/grafana/grafana/issues/100970)
- [Grafana API Rate Limiting](https://drdroid.io/stack-diagnosis/grafana-grafana-api-rate-limiting)
- [Azure Managed Grafana Limitations](https://learn.microsoft.com/en-us/azure/managed-grafana/known-limitations)

**Dashboard JSON Schema:**
- [Dashboard JSON Model](https://grafana.com/docs/grafana/latest/visualizations/dashboards/build-dashboards/view-dashboard-json-model/)
- [Dashboard JSON Schema V2](https://grafana.com/docs/grafana/latest/as-code/observability-as-code/schema-v2/)
- [Using Grafana JSON Model](https://yasoobhaider.medium.com/using-grafana-json-model-howto-509aca3cf9a9)
- [Dashboard Spec GitHub](https://github.com/grafana/dashboard-spec)

**PromQL Parsing:**
- [Prometheus Issue #6256: Parser Replacement](https://github.com/prometheus/prometheus/issues/6256)
- [PromQL Parser Source Code](https://github.com/prometheus/prometheus/blob/main/promql/parser/parse.go)
- [VictoriaMetrics: PromQL Functions and Edge Cases](https://victoriametrics.com/blog/prometheus-monitoring-function-operator-modifier/)
- [3 Common PromQL Mistakes](https://home.robusta.dev/blog/3-common-mistakes-with-promql-and-kubernetes-metrics)
- [PromQL Cheat Sheet](https://promlabs.com/promql-cheat-sheet/)
- [21 PromQL Tricks](https://last9.io/blog/promql-tricks-you-should-know/)

**Grafana Variables:**
- [Prometheus Template Variables](https://grafana.com/docs/grafana/latest/datasources/prometheus/template-variables/)
- [Variable Syntax](https://grafana.com/docs/grafana/latest/visualizations/dashboards/variables/variable-syntax/)
- [Variable Formatter Issue #93776](https://github.com/grafana/grafana/issues/93776)

**Graph Database Schema:**
- [FalkorDB Design](https://docs.falkordb.com/design/)
- [How to Build a Knowledge Graph](https://www.falkordb.com/blog/how-to-build-a-knowledge-graph/)
- [Graph Database Guide for AI](https://www.falkordb.com/blog/graph-database-guide/)
- [Time Series Database Fundamentals](https://www.tigergraph.com/blog/time-series-database-fundamentals-in-modern-analytics/)
- [Schema Design for Time Series](https://cloud.google.com/bigtable/docs/schema-design-time-series)

**Anomaly Detection:**
- [Dealing with Trends and Seasonality](https://www.oreilly.com/library/view/anomaly-detection-for/9781492042341/ch04.html)
- [OpenSearch: Reducing False Positives](https://opensearch.org/blog/reducing-false-positives-through-algorithmic-improvements/)
- [Anomaly Detection: Good vs Bad Performance](https://towardsdatascience.com/anomaly-detection-how-to-tell-good-performance-from-bad-b57116d71a10/)
- [Handling Seasonal Patterns](https://milvus.io/ai-quick-reference/how-does-anomaly-detection-handle-seasonal-patterns)
- [Time Series Anomaly Detection in Python](https://www.turing.com/kb/time-series-anomaly-detection-in-python)
- [Digital Twin Anomaly Detection Under Drift](https://www.sciencedirect.com/science/article/abs/pii/S0957417425036784)

**Progressive Disclosure:**
- [Progressive Disclosure (NN/G)](https://www.nngroup.com/articles/progressive-disclosure/)
- [Progressive Disclosure Examples](https://userpilot.com/blog/progressive-disclosure-examples/)
- [B2B SaaS UX Design 2026](https://www.onething.design/post/b2b-saas-ux-design)
- [Progressive Disclosure in UX](https://blog.logrocket.com/ux-design/progressive-disclosure-ux-types-use-cases/)

**Observability Trends:**
- [2026 Observability Trends from Grafana Labs](https://grafana.com/blog/2026-observability-trends-predictions-from-grafana-labs-unified-intelligent-and-open/)
- [What is Observability in 2026](https://clickhouse.com/resources/engineering/what-is-observability)
- [Observability Predictions for 2026](https://middleware.io/blog/observability-predictions/)
