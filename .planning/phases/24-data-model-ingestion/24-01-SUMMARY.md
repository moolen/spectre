---
phase: 24-data-model-ingestion
plan: 01
milestone: v1.5
subsystem: signal-intelligence
completed: 2026-01-29
duration: 6m

requires:
  - internal/integration/grafana/promql_parser.go (QueryExtraction for Layer 2 classification)
  - internal/integration/grafana/types.go (GrafanaDashboard, GrafanaPanel structures)
  - internal/integration/grafana/graph_builder.go (existing graph patterns)

provides:
  - SignalAnchor data model with role classification and quality scoring
  - Layered classification engine (5 layers, 0.95 → 0 confidence)
  - Dashboard quality scorer (5 factors with alert boost)

affects:
  - Phase 24-02: Signal extraction will use ClassifyMetric and ComputeDashboardQuality
  - Phase 25: Baseline storage will reference SignalAnchor nodes
  - Phase 26: Observatory API will query SignalAnchor nodes by workload

tech-stack:
  added: []
  patterns:
    - Layered classification with confidence decay
    - Multi-factor quality scoring with alert incentive
    - TTL via expires_at timestamp (7 days, follows v1.4)

key-files:
  created:
    - internal/integration/grafana/signal_types.go (SignalAnchor, SignalRole enum, ClassificationResult)
    - internal/integration/grafana/signal_classifier.go (5-layer classification engine)
    - internal/integration/grafana/signal_classifier_test.go (comprehensive test coverage)
    - internal/integration/grafana/quality_scorer.go (dashboard quality computation)
    - internal/integration/grafana/quality_scorer_test.go (factor and formula tests)
  modified: []

decisions:
  - role-taxonomy: "7 roles: Availability, Latency, Errors, Traffic, Saturation, Churn (deprecated), Novelty"
  - classification-layers: "5 layers with decreasing confidence: 0.95, 0.85-0.9, 0.7-0.8, 0.5, 0"
  - quality-formula: "base = avg(4 factors), quality = min(1.0, base + 0.2*hasAlerts)"
  - quality-tiers: "high (>=0.7), medium (>=0.4), low (<0.4)"
  - ttl-duration: "7 days from LastSeen, query-time filtering via WHERE expires_at > $now"
  - composite-key: "metric_name + workload_namespace + workload_name for deduplication"

tags:
  - signal-intelligence
  - classification
  - quality-scoring
  - grafana
  - observability
---

# Phase 24 Plan 01: Signal Types and Classification Summary

**One-liner:** Created SignalAnchor types with 5-layer classification engine (0.95→0 confidence) and 5-factor dashboard quality scoring (alert boost formula).

## What Was Delivered

Established the foundation for signal intelligence: types, classification, and quality scoring. SignalAnchor links metrics to semantic roles (Availability, Latency, Errors, Traffic, Saturation, Novelty) with confidence scoring. Layered classifier applies hardcoded metrics → PromQL structure → metric name patterns → panel titles → unknown. Quality scorer evaluates dashboards via freshness, usage, alerting, ownership, and completeness.

### Components

**1. SignalAnchor Data Model** (`signal_types.go`)
- SignalRole enum with 7 roles (Google Four Golden Signals + extensions)
- SignalAnchor struct with 13 fields (metric, role, confidence, quality, workload, timestamps)
- ClassificationResult for internal classification tracking
- WorkloadInference for K8s workload linkage from PromQL labels
- Composite key: `metric_name + workload_namespace + workload_name`
- TTL via `expires_at` timestamp (7 days, follows v1.4 pattern)

**2. Layered Signal Classifier** (`signal_classifier.go`)
- **Layer 1:** Hardcoded known metrics (20+ core metrics, confidence 0.95)
  - Examples: `up` → Availability, `container_cpu_usage_seconds_total` → Saturation
- **Layer 2:** PromQL structure patterns (confidence 0.85-0.9)
  - `histogram_quantile` → Latency, `rate(errors)` → Errors, `rate(requests)` → Traffic
- **Layer 3:** Metric name patterns (confidence 0.7-0.8)
  - `*_latency*` → Latency, `*_error*` → Errors, `*_total` → Traffic
- **Layer 4:** Panel title patterns (confidence 0.5)
  - "Error Rate" → Errors, "Latency P95" → Latency, "QPS" → Traffic
- **Layer 5:** Unknown classification (confidence 0)

**3. Dashboard Quality Scorer** (`quality_scorer.go`)
- **Freshness:** 1.0 at <=90 days, linear decay to 0.0 at 365 days
- **RecentUsage:** 1.0 if views in last 30 days, 0 otherwise (graceful fallback)
- **HasAlerts:** 1.0 if alert rules attached, 0 otherwise
- **Ownership:** 1.0 for team folder, 0.5 for "General"
- **Completeness:** 0-1 based on description + meaningful panel titles (>50% threshold)
- **Formula:** `base = avg(4 factors)`, `quality = min(1.0, base + 0.2*hasAlerts)`
- **Tiers:** high (>=0.7), medium (>=0.4), low (<0.4)

## Task Breakdown

| Task | Description | Commit | Files | Duration |
|------|-------------|--------|-------|----------|
| 1 | Create SignalAnchor types and schema | 49aa933 | signal_types.go | ~2m |
| 2 | Implement layered signal classifier | bcee61e | signal_classifier.go, signal_classifier_test.go | ~2m |
| 3 | Implement dashboard quality scorer | 120a084 | quality_scorer.go, quality_scorer_test.go | ~2m |

Total implementation time: 6 minutes

## Decisions Made

### 1. Signal Role Taxonomy
**Decision:** Use 7-role taxonomy based on Google Four Golden Signals + observability extensions

**Context:** Need semantic classification that aligns with SRE best practices

**Roles:**
- **Availability:** Uptime/health (up, kube_pod_status_phase)
- **Latency:** Response time/duration (histogram_quantile, *_duration_*)
- **Errors:** Failure rates (*_error_*, *_failed_*)
- **Traffic:** Throughput/requests (rate(*_total), *_count)
- **Saturation:** Resource utilization (cpu, memory, disk)
- **Churn:** (deprecated) Workload restarts
- **Novelty:** Change events/deployments (replaces Churn in v1.5)

**Rationale:** Google's Four Golden Signals (Latency, Traffic, Errors, Saturation) are industry standard. Added Availability (basic health checks) and Novelty (change tracking) for observability completeness.

### 2. Layered Classification with Confidence Decay
**Decision:** Apply 5 classification layers with decreasing confidence (0.95 → 0.85-0.9 → 0.7-0.8 → 0.5 → 0)

**Context:** Single-layer classification either too rigid (hardcoded only) or too unreliable (fuzzy matching only)

**Implementation:**
1. Layer 1 (0.95): Exact metric name matching for 20+ core Prometheus metrics
2. Layer 2 (0.85-0.9): PromQL AST analysis (histogram_quantile, rate patterns)
3. Layer 3 (0.7-0.8): Metric name substring patterns (_latency, _error, _total)
4. Layer 4 (0.5): Panel title keyword matching (Error Rate, QPS, CPU)
5. Layer 5 (0): Unknown classification, confidence 0

**Rationale:** Confidence reflects classification reliability. Hardcoded metrics are near-certain (0.95), while panel titles are subjective/ambiguous (0.5). Agents can filter by confidence threshold and see "uncertain" signals separately.

### 3. Quality Scoring Formula with Alert Boost
**Decision:** Compute quality as `base = avg(4 factors)`, `quality = min(1.0, base + 0.2*hasAlerts)`

**Context:** Need to prioritize high-value dashboards and incentivize alerting

**Factors:**
- Freshness: Recent modification indicates maintenance
- RecentUsage: Views indicate relevance (graceful fallback if Stats API unavailable)
- Ownership: Team folders indicate responsibility vs "General" dumping ground
- Completeness: Description + meaningful titles indicate quality

**Alert Boost:** +0.2 quality score if dashboard has attached alert rules. Incentivizes teams to create alerts, not just dashboards.

**Rationale:** Simple average is interpretable. Alert boost prioritizes "production-ready" dashboards with actionable alerting. Capped at 1.0 to maintain 0-1 normalization.

### 4. Composite Key for Deduplication
**Decision:** Use `metric_name + workload_namespace + workload_name` as SignalAnchor unique key

**Context:** Same metric may appear in multiple dashboards → need conflict resolution

**Implementation:** MERGE node on composite key, highest quality dashboard wins via ON MATCH updates

**Rationale:** Metric+workload combination is semantically unique. If Team A and Team B both monitor `http_requests_total` for service `api`, they're the same signal. Quality-based conflict resolution ensures best source wins.

### 5. TTL Duration and Query-Time Filtering
**Decision:** 7-day TTL via `expires_at` timestamp, query-time filtering with `WHERE expires_at > $now`

**Context:** Dashboards may be deleted or metrics removed → signals become stale

**Implementation:** Set `expires_at = last_seen + 7 days` on every sync. Query filters expired signals automatically.

**Rationale:** Follows v1.4 pattern (state transitions, baseline cache). 7 days allows multiple sync cycles before expiration (dashboards sync daily). No background cleanup jobs needed.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed duplicate keys in known metrics map**
- **Found during:** Task 2 (classifier implementation)
- **Issue:** `grpc_server_handled_total` and `apiserver_request_total` appeared in both Traffic and Errors sections of Layer 1 map, causing Go compilation error
- **Root cause:** These metrics are context-dependent (can be Traffic or Errors based on status/code labels), but Layer 1 requires unambiguous classification
- **Fix:** Removed duplicates from Layer 1. Added comment noting these metrics should be classified at Layer 2 (PromQL structure) based on label context.
- **Files modified:** `signal_classifier.go`
- **Commit:** bcee61e (part of classifier implementation)
- **Rationale:** Layer 1 is for high-confidence, unambiguous metrics only. Context-dependent metrics belong in Layer 2 where PromQL label filters can inform classification.

**2. [Rule 1 - Bug] Fixed test using Layer 1 metrics to test Layer 2 classification**
- **Found during:** Task 2 (running classifier tests)
- **Issue:** Test `rate(requests_total) → Traffic` used `http_requests_total` (hardcoded in Layer 1), so classifier returned Layer 1 result (0.95 confidence) instead of Layer 2 (0.85 confidence)
- **Root cause:** Test design flaw - testing Layer 2 behavior with Layer 1 metric
- **Fix:** Changed test metric from `http_requests_total` → `api_requests_total` (not in Layer 1 hardcoded list). Similarly changed `http_request_errors_total` → `api_errors_total`.
- **Files modified:** `signal_classifier_test.go`
- **Commit:** bcee61e (part of classifier implementation)
- **Rationale:** Tests must use metrics NOT in higher-priority layers to validate layer-specific behavior.

## Test Coverage

### Classifier Tests (`signal_classifier_test.go`)
- **Layer 1:** 6 tests covering hardcoded metrics across all roles (Availability, Saturation, Traffic, Novelty)
- **Layer 2:** 4 tests for PromQL structure patterns (histogram_quantile, rate/increase)
- **Layer 3:** 6 tests for metric name patterns (latency, error, traffic, saturation indicators)
- **Layer 4:** 5 tests for panel title patterns (Error Rate, Latency, QPS, CPU, Health)
- **Layer 5:** 2 tests for unknown classification
- **Layer priority:** 3 tests verifying Layer 1 > Layer 2 > Layer 3 > Layer 4 precedence
- **Edge cases:** 1 test verifying error metrics with "_total" classify as Errors (not Traffic)

**Total:** 27 test cases

### Quality Scorer Tests (`quality_scorer_test.go`)
- **Freshness:** 7 tests covering 0-500 days old (linear decay validation)
- **RecentUsage:** 3 tests for view counts (0, 1, 100 views)
- **HasAlerts:** 3 tests for alert rule counts (0, 1, 5 alerts)
- **Ownership:** 4 tests for folder types (General, empty, team folders)
- **Completeness:** 7 tests for description + panel title combinations
- **Formula:** 1 test verifying alert boost caps at 1.0, 1 test for full formula
- **Tiers:** 8 tests for quality tier mapping (high/medium/low boundaries)
- **Helper functions:** 9 tests for isMeaningfulTitle edge cases

**Total:** 43 test cases

### Coverage Summary
- **Total test cases:** 70
- **All tests passing:** ✓
- **Build verification:** ✓ (`go build ./internal/integration/grafana`)

## Integration Points

### Inputs (Dependencies)
1. **internal/integration/grafana/promql_parser.go**
   - `QueryExtraction` struct used in Layer 2 classification
   - `ExtractFromPromQL` provides metric names, aggregations, label selectors
   - Used by: `classifyPromQLStructure()` in `signal_classifier.go`

2. **internal/integration/grafana/types.go**
   - `GrafanaDashboard` struct provides Panels array
   - `GrafanaPanel` struct provides Title field
   - Used by: `ComputeDashboardQuality()` in `quality_scorer.go`

3. **internal/integration/grafana/graph_builder.go**
   - Provides existing MERGE patterns for graph operations
   - ServiceInference pattern for workload linkage
   - Used by: Future signal extraction (Phase 24-02)

### Outputs (Provides)
1. **SignalAnchor Data Model**
   - Will be stored as graph nodes in Phase 24-02 (signal extraction)
   - Links: `(SignalAnchor)-[:EXTRACTED_FROM]->(Query)`, `(SignalAnchor)-[:MONITORS]->(ResourceIdentity)`
   - TTL: 7 days via `expires_at` timestamp

2. **ClassifyMetric Function**
   - Public API: `func ClassifyMetric(metricName string, extraction *QueryExtraction, panelTitle string) ClassificationResult`
   - Returns role, confidence, layer, reason
   - Used by: Signal extraction in Phase 24-02

3. **ComputeDashboardQuality Function**
   - Public API: `func ComputeDashboardQuality(dashboard *GrafanaDashboard, alertRuleCount int, viewsLast30Days int, updated time.Time, folderTitle string, description string) float64`
   - Returns quality score (0.0-1.0)
   - Used by: Signal extraction in Phase 24-02

### Affects (Downstream)
1. **Phase 24-02: Signal Extraction**
   - Will call `ClassifyMetric()` for each PromQL query in dashboard panels
   - Will call `ComputeDashboardQuality()` once per dashboard
   - Will create SignalAnchor graph nodes with MERGE upsert

2. **Phase 25: Baseline Storage**
   - Will query SignalAnchor nodes to identify which metrics need baselines
   - Will filter by confidence threshold (e.g., >= 0.7 for high-confidence signals)

3. **Phase 26: Observatory API**
   - MCP tools will query SignalAnchor nodes by workload (namespace + name)
   - Will filter by quality tier (high/medium/low) for prioritization
   - Will return uncertain signals in separate response section

## Next Phase Readiness

### Ready for Phase 24-02
- ✓ SignalAnchor types defined
- ✓ Classification engine implemented and tested
- ✓ Quality scorer implemented and tested
- ✓ Confidence thresholds defined (0.95, 0.85-0.9, 0.7-0.8, 0.5, 0)
- ✓ Quality tiers defined (high >= 0.7, medium >= 0.4, low < 0.4)
- ✓ TTL pattern established (7 days, query-time filtering)
- ✓ Composite key pattern defined (metric + namespace + workload)

### Blockers
None. Phase 24-02 can proceed with signal extraction implementation.

### Open Questions
1. **Layer 1 metric exhaustiveness:** Started with 20 core metrics. May need expansion based on real dashboard data in Phase 24-02.
2. **Grafana Stats API availability:** Quality scorer gracefully handles absence of Stats API, but unknown if this is common in deployments.
3. **Multi-source Grafana handling:** SignalAnchor includes `source_grafana` field, but conflict resolution across multiple Grafana instances not fully specified. May need clarification in Phase 24-02.

## Performance Notes

- All operations O(1) or O(n) complexity (no nested loops or graph traversals)
- Classifier: 5 sequential layer checks, early exit on first match
- Quality scorer: 5 independent factor computations, no I/O
- No external dependencies added (uses stdlib only)
- Test execution: <20ms for 70 test cases

## Files Changed

**Created:**
- `internal/integration/grafana/signal_types.go` (138 lines)
- `internal/integration/grafana/signal_classifier.go` (280 lines)
- `internal/integration/grafana/signal_classifier_test.go` (407 lines)
- `internal/integration/grafana/quality_scorer.go` (146 lines)
- `internal/integration/grafana/quality_scorer_test.go` (458 lines)

**Total:** 1,429 lines of code and tests

**Modified:** None

## Commits

| Hash | Message | Files |
|------|---------|-------|
| 49aa933 | feat(24-01): create SignalAnchor types and schema | signal_types.go |
| bcee61e | feat(24-01): implement layered signal classifier | signal_classifier.go, signal_classifier_test.go |
| 120a084 | feat(24-01): implement dashboard quality scorer | quality_scorer.go, quality_scorer_test.go |

---

**Phase:** 24-data-model-ingestion
**Plan:** 01
**Completed:** 2026-01-29
**Duration:** 6 minutes
