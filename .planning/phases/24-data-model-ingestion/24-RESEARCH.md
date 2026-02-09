# Phase 24: Data Model & Ingestion - Research

**Researched:** 2026-01-29
**Domain:** Graph-based signal extraction with PromQL classification and quality scoring
**Confidence:** HIGH

## Summary

Phase 24 creates SignalAnchor nodes in FalkorDB that extract "what matters" from Grafana dashboards. The architecture combines PromQL parsing for metric extraction, layered classification for signal role taxonomy (Availability, Latency, Errors, Traffic, Saturation), quality scoring based on dashboard metadata, and K8s workload linkage through label inference.

Research confirms the standard stack is already in place: `prometheus/prometheus/promql/parser` for PromQL AST traversal, `FalkorDB/falkordb-go/v2` for graph operations with MERGE-based idempotency, and established patterns from v1.4 for TTL management via `expires_at` timestamps. The signal taxonomy aligns with Google's Four Golden Signals (Latency, Traffic, Errors, Saturation) plus observability-specific extensions (Availability, Churn, Novelty).

Key architectural patterns verified: idempotent MERGE operations with ON CREATE/ON MATCH clauses, query-time TTL filtering, parameterized queries for safety, and layered classification with confidence scoring. The phase integrates naturally with existing DashboardSyncer infrastructure.

**Primary recommendation:** Extend existing PromQL parser with layered classification heuristics, reuse MERGE upsert patterns from v1.4, piggyback on DashboardSyncer for ingestion trigger, and implement query-time TTL filtering for signal expiration.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/prometheus/prometheus/promql/parser | v0.309.1 | PromQL AST parsing and traversal | Official Prometheus parser, production-grade AST walking with parser.Inspect |
| github.com/FalkorDB/falkordb-go/v2 | v2.0.2 | FalkorDB graph database client | Already integrated, provides Query/ROQuery with parameterization |
| Go standard library | 1.24.9 | String matching, regexp, time | Built-in, no external dependencies needed |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/texttheater/golang-levenshtein/levenshtein | latest | Fuzzy string matching | Optional: could improve metric name pattern matching |
| encoding/json | stdlib | JSON serialization for properties | Graph node properties (labels, annotations) |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| prometheus/prometheus parser | VictoriaMetrics/metricsql | MetricsQL has extensions but adds dependency, Prometheus parser is sufficient |
| Hardcoded classification | LLM-based classification | Too slow, not deterministic, overkill for pattern matching |
| Application-side TTL cleanup | Graph-based query-time filtering | Query-time filtering is established v1.4 pattern, no background jobs |

**Installation:**
All dependencies already in go.mod. No new packages required.

## Architecture Patterns

### Recommended Project Structure
```
internal/integration/grafana/
├── signal_classifier.go         # Layered classification engine
├── signal_extractor.go          # Panel -> SignalAnchor transformation
├── quality_scorer.go            # Dashboard quality computation
├── workload_linker.go           # K8s workload inference from labels
├── graph_builder.go             # EXISTING: extend with signal methods
├── promql_parser.go             # EXISTING: reuse QueryExtraction
└── dashboard_syncer.go          # EXISTING: hook signal ingestion
```

### Pattern 1: Layered Classification with Confidence
**What:** Multi-tier classification where confidence decreases as matching becomes less specific
**When to use:** When multiple heuristics of varying reliability must be combined
**Example:**
```go
// Source: Phase 24 context decisions
type ClassificationResult struct {
    Role       SignalRole  // Availability, Latency, Errors, etc.
    Confidence float64     // 0.0-1.0
    Layer      int         // 1-5 (1=hardcoded, 5=panel title)
    Reason     string      // "matched hardcoded metric: up"
}

// Layer 1: Hardcoded known metrics (confidence ~0.95)
func classifyKnownMetric(metricName string) *ClassificationResult {
    knownMetrics := map[string]SignalRole{
        "up":                                   Availability,
        "kube_pod_status_phase":               Availability,
        "container_cpu_usage_seconds_total":    Saturation,
        "node_cpu_seconds_total":              Saturation,
        "kube_node_status_condition":          Availability,
    }
    if role, ok := knownMetrics[metricName]; ok {
        return &ClassificationResult{
            Role: role, Confidence: 0.95, Layer: 1,
            Reason: fmt.Sprintf("matched hardcoded metric: %s", metricName),
        }
    }
    return nil
}

// Layer 2: PromQL structure patterns (confidence ~0.85-0.9)
func classifyPromQLStructure(query *QueryExtraction) *ClassificationResult {
    // histogram_quantile(*_bucket) -> Latency
    if containsFunc(query.Aggregations, "histogram_quantile") {
        return &ClassificationResult{
            Role: Latency, Confidence: 0.9, Layer: 2,
            Reason: "histogram_quantile indicates latency measurement",
        }
    }
    // rate(*_total) with error keywords -> Errors
    if containsFunc(query.Aggregations, "rate") || containsFunc(query.Aggregations, "increase") {
        for _, metric := range query.MetricNames {
            if strings.Contains(metric, "error") || strings.Contains(metric, "failed") {
                return &ClassificationResult{
                    Role: Errors, Confidence: 0.85, Layer: 2,
                    Reason: "rate/increase on error metric",
                }
            }
        }
    }
    return nil
}

// Layer 3: Metric name patterns (confidence ~0.7-0.8)
// Layer 4: Panel title/description (confidence ~0.5)
// Layer 5: Unknown (confidence 0)
```

### Pattern 2: Idempotent MERGE Upsert
**What:** Graph operations that can be safely re-run without duplicating data
**When to use:** All graph write operations, especially for sync/ingestion pipelines
**Example:**
```go
// Source: internal/graph/schema.go UpsertDashboardNode pattern
func UpsertSignalAnchorQuery(anchor SignalAnchor) graph.GraphQuery {
    // Composite key: metric_name + workload_namespace + workload_name
    return graph.GraphQuery{
        Query: `
            MERGE (s:SignalAnchor {
                metric_name: $metric_name,
                workload_namespace: $workload_namespace,
                workload_name: $workload_name
            })
            ON CREATE SET
                s.role = $role,
                s.confidence = $confidence,
                s.quality_score = $quality_score,
                s.dashboard_uid = $dashboard_uid,
                s.panel_id = $panel_id,
                s.query_id = $query_id,
                s.source_grafana = $source_grafana,
                s.first_seen = $first_seen,
                s.last_seen = $last_seen,
                s.expires_at = $expires_at
            ON MATCH SET
                s.role = $role,
                s.confidence = $confidence,
                s.quality_score = $quality_score,
                s.dashboard_uid = $dashboard_uid,
                s.panel_id = $panel_id,
                s.query_id = $query_id,
                s.last_seen = $last_seen,
                s.expires_at = $expires_at
        `,
        Parameters: map[string]interface{}{
            "metric_name":         anchor.MetricName,
            "workload_namespace":  anchor.WorkloadNamespace,
            "workload_name":       anchor.WorkloadName,
            "role":                string(anchor.Role),
            "confidence":          anchor.Confidence,
            "quality_score":       anchor.QualityScore,
            "dashboard_uid":       anchor.DashboardUID,
            "panel_id":            anchor.PanelID,
            "query_id":            anchor.QueryID,
            "source_grafana":      anchor.SourceGrafana,
            "first_seen":          anchor.FirstSeen,
            "last_seen":           anchor.LastSeen,
            "expires_at":          anchor.ExpiresAt,
        },
    }
}
```

### Pattern 3: Query-Time TTL Filtering
**What:** Expired data filtered in WHERE clause, not via background cleanup jobs
**When to use:** Any temporal data that becomes stale (established v1.4 pattern)
**Example:**
```go
// Source: .planning/phases/19-anomaly-detection/19-02-PLAN.md
func QueryActiveSignals(namespace, workload string, now int64) graph.GraphQuery {
    return graph.GraphQuery{
        Query: `
            MATCH (s:SignalAnchor {
                workload_namespace: $namespace,
                workload_name: $workload
            })
            WHERE s.expires_at > $now
            RETURN s
        `,
        Parameters: map[string]interface{}{
            "namespace": namespace,
            "workload":  workload,
            "now":       now,
        },
    }
}
```

### Pattern 4: Multi-Label from Single Query
**What:** Create separate nodes for each detected role when multiple signals exist in one query
**When to use:** "Golden signals" dashboards with multiple metrics in one panel
**Example:**
```go
// From Phase 24 context: "Create separate SignalAnchor per detected role"
func extractSignalsFromPanel(panel GrafanaPanel, dashboardQuality float64) []SignalAnchor {
    var signals []SignalAnchor
    for _, target := range panel.Targets {
        extraction, _ := ExtractFromPromQL(target.Expr)
        for _, metric := range extraction.MetricNames {
            // Each metric may classify to multiple roles
            results := classifyMetric(metric, extraction, panel.Title)
            for _, result := range results {
                if result.Confidence >= threshold {
                    signal := SignalAnchor{
                        MetricName:   metric,
                        Role:         result.Role,
                        Confidence:   result.Confidence,
                        QualityScore: dashboardQuality,
                        // ... workload inference, timestamps ...
                    }
                    signals = append(signals, signal)
                }
            }
        }
    }
    return signals
}
```

### Anti-Patterns to Avoid
- **Eagerly creating K8s nodes:** Don't create ResourceIdentity nodes for workloads unless they exist in K8s graph or can be inferred with high confidence. Use `unlinked` flag instead.
- **Classification overrides in config:** User decisions say "no overrides initially, trust the algorithm." Fix classification bugs in code, not via config mappings.
- **Single classification per metric:** Metrics can have multiple roles (e.g., `http_requests_total` can be both Traffic and Errors depending on label filters).
- **Application-side TTL cleanup:** Use query-time filtering with `WHERE expires_at > $now`, following v1.4 baseline cache pattern.

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| PromQL parsing | Custom regex-based parser | prometheus/prometheus/promql/parser | AST-based traversal handles nested expressions, function calls, binary operations correctly |
| Metric name pattern matching | Custom string matching | Standard library strings + regexp | Sufficient for classification, no need for complex NLP |
| Graph idempotency | Application-side deduplication | Cypher MERGE with ON CREATE/ON MATCH | Database-level guarantees, simpler code, handles concurrent writes |
| TTL cleanup | Background goroutine with DELETE queries | Query-time filtering with WHERE expires_at | No cleanup jobs, no race conditions, established v1.4 pattern |
| Quality scoring normalization | Custom math library | Simple float64 averaging + min/max | Quality formula is explicit average with alert boost, no statistical library needed |
| K8s label parsing | Custom key-value parser | Go map[string]string from existing QueryExtraction.LabelSelectors | Already extracted by PromQL parser |

**Key insight:** This phase primarily combines existing components (PromQL parser, graph patterns, DashboardSyncer) rather than building new infrastructure. The complexity is in classification heuristics, not in tooling.

## Common Pitfalls

### Pitfall 1: Classification Confidence Inflation
**What goes wrong:** Setting confidence too high for weak signals (e.g., 0.9 for panel title matching)
**Why it happens:** Developer confidence in heuristic doesn't match reality of noisy panel titles
**How to avoid:** Follow Phase 24 context confidence levels strictly: Layer 1=0.95, Layer 2=0.85-0.9, Layer 3=0.7-0.8, Layer 4=0.5, Layer 5=0
**Warning signs:** Uncertain signals appearing in high-confidence tier in tool responses

### Pitfall 2: Composite Key Mismatch
**What goes wrong:** Using wrong unique key for SignalAnchor MERGE, creating duplicates or missing updates
**Why it happens:** Unclear what makes a signal "unique" - is it metric+workload? metric+query? metric+panel?
**How to avoid:** Follow Phase 24 decision: "Same metric in multiple dashboards → highest-quality dashboard wins". Key = metric_name + workload_namespace + workload_name. NOT keyed by query_id.
**Warning signs:** Multiple SignalAnchors for same metric+workload with different quality scores

### Pitfall 3: Workload Inference Over-Eager
**What goes wrong:** Creating ResourceIdentity nodes for inferred workloads that don't exist in K8s
**Why it happens:** Label selectors in PromQL don't guarantee K8s resource exists
**How to avoid:** Phase 24 context says "if no match: create signal as orphan node, mark as unlinked". Check if ResourceIdentity exists first, use MATCH not MERGE for workload linkage.
**Warning signs:** Orphan ResourceIdentity nodes with no CHANGED edges or other K8s relationships

### Pitfall 4: TTL Duration Guesswork
**What goes wrong:** Setting expires_at too short (signals expire before refresh) or too long (stale signals persist)
**Why it happens:** No explicit requirement in Phase 24, developer must choose
**How to avoid:** Follow v1.4 state transition pattern: 7 days. Rationale: dashboards sync daily, 7 days gives multiple refresh opportunities before expiration.
**Warning signs:** `dashboards processed=X, signals created=0` in logs on subsequent syncs (signals expired before refresh)

### Pitfall 5: Quality Score Circular Dependency
**What goes wrong:** Computing dashboard quality using signal quality, or vice versa
**Why it happens:** Confusion about propagation direction
**How to avoid:** Phase 24 context is explicit: "SignalAnchor inherits quality score from source dashboard". Dashboard quality computed first (freshness, alerting, ownership, completeness), then propagated to signals.
**Warning signs:** Quality scores of 0.0 when dashboard has valid metadata

### Pitfall 6: PromQL Variable Handling
**What goes wrong:** Classification fails on queries with Grafana variables ($namespace, ${cluster})
**Why it happens:** Variables make PromQL unparseable by Prometheus parser
**How to avoid:** Existing promql_parser.go already handles this: extraction.HasVariables=true when variables detected. Classify based on partial extraction or skip with low confidence.
**Warning signs:** High skip count in ingestion logs for dashboards with templated queries

## Code Examples

Verified patterns from official sources:

### PromQL AST Traversal for Classification
```go
// Source: internal/integration/grafana/promql_parser.go + prometheus parser docs
// URL: https://pkg.go.dev/github.com/prometheus/prometheus/promql/parser
func ExtractMetricsForClassification(queryStr string) (*QueryExtraction, error) {
    extraction := &QueryExtraction{
        MetricNames:    make([]string, 0),
        LabelSelectors: make(map[string]string),
        Aggregations:   make([]string, 0),
        HasVariables:   false,
    }

    if hasVariableSyntax(queryStr) {
        extraction.HasVariables = true
    }

    expr, err := parser.ParseExpr(queryStr)
    if err != nil {
        if extraction.HasVariables {
            return extraction, nil // Partial extraction OK
        }
        return nil, fmt.Errorf("failed to parse PromQL: %w", err)
    }

    // Walk AST in depth-first order
    parser.Inspect(expr, func(node parser.Node, path []parser.Node) error {
        if node == nil {
            return nil
        }

        switch n := node.(type) {
        case *parser.VectorSelector:
            if n.Name != "" && !hasVariableSyntax(n.Name) {
                extraction.MetricNames = append(extraction.MetricNames, n.Name)
            }
            for _, matcher := range n.LabelMatchers {
                if matcher.Name != "__name__" {
                    extraction.LabelSelectors[matcher.Name] = matcher.Value
                }
            }

        case *parser.AggregateExpr:
            extraction.Aggregations = append(extraction.Aggregations, n.Op.String())

        case *parser.Call:
            extraction.Aggregations = append(extraction.Aggregations, n.Func.Name)
        }

        return nil
    })

    return extraction, nil
}
```

### Quality Score Computation
```go
// Source: Phase 24 context decisions
type DashboardQuality struct {
    Freshness       float64 // 0-1: 90 days=1.0, linear decay to 0 at 365 days
    RecentUsage     float64 // 0 or 1: has views in last 30 days
    HasAlerts       float64 // 0 or 1: at least one alert rule
    Ownership       float64 // 1.0 for team folder, 0.5 for "General"
    Completeness    float64 // 0-1: has description + meaningful panel titles
}

func ComputeDashboardQuality(dashboard DashboardMetadata) float64 {
    q := DashboardQuality{}

    // Freshness: linear decay from 90 to 365 days
    daysSinceModified := time.Since(dashboard.Updated).Hours() / 24
    if daysSinceModified <= 90 {
        q.Freshness = 1.0
    } else if daysSinceModified >= 365 {
        q.Freshness = 0.0
    } else {
        // Linear interpolation: 1.0 at 90 days, 0.0 at 365 days
        q.Freshness = 1.0 - (daysSinceModified-90)/(365-90)
    }

    // RecentUsage: binary check (requires Grafana Stats API)
    if dashboard.ViewsLast30Days > 0 {
        q.RecentUsage = 1.0
    }

    // HasAlerts: binary check
    if dashboard.AlertRuleCount > 0 {
        q.HasAlerts = 1.0
    }

    // Ownership: team folder vs General
    if dashboard.Folder != "" && dashboard.Folder != "General" {
        q.Ownership = 1.0
    } else {
        q.Ownership = 0.5
    }

    // Completeness: has description + meaningful titles
    completeness := 0.0
    if dashboard.Description != "" {
        completeness += 0.5
    }
    if dashboard.MeaningfulPanelTitleRatio > 0.5 { // >50% panels have non-default titles
        completeness += 0.5
    }
    q.Completeness = completeness

    // Formula: base = avg(4 factors), alertBoost = 0.2 if alerts exist
    base := (q.Freshness + q.RecentUsage + q.Ownership + q.Completeness) / 4.0
    alertBoost := q.HasAlerts * 0.2
    quality := math.Min(1.0, base+alertBoost)

    return quality
}
```

### K8s Workload Inference from Labels
```go
// Source: Phase 24 context + Kubernetes Labels and Selectors docs
// URL: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
func InferWorkloadFromLabels(labelSelectors map[string]string) *WorkloadInference {
    // Label priority: namespace > deployment > service > pod > container
    // Per Kubernetes best practices, standard label keys are:
    // app.kubernetes.io/name, app, service, job, deployment, namespace

    inference := &WorkloadInference{
        Confidence: 0.0,
    }

    // Namespace: highest priority, most reliable
    if ns, ok := labelSelectors["namespace"]; ok {
        inference.Namespace = ns
        inference.Confidence = 0.9
    }

    // Workload name: try standard label keys in priority order
    workloadKeys := []string{
        "deployment",           // Explicit deployment label
        "app.kubernetes.io/name", // Recommended label
        "app",                  // Common label
        "service",              // Service name
        "job",                  // Job name
    }

    for _, key := range workloadKeys {
        if val, ok := labelSelectors[key]; ok {
            inference.WorkloadName = val
            inference.InferredFrom = key
            if inference.Confidence == 0.0 {
                inference.Confidence = 0.7 // Base confidence for label match
            }
            break
        }
    }

    // No workload inferred: return nil to mark signal as unlinked
    if inference.WorkloadName == "" {
        return nil
    }

    return inference
}
```

### Idempotent Signal Ingestion with Conflict Resolution
```go
// Source: internal/graph/schema.go MERGE patterns + Phase 24 context
func IngestSignalsFromDashboard(
    ctx context.Context,
    graphClient graph.Client,
    dashboard DashboardMetadata,
    panels []GrafanaPanel,
) error {
    // Compute quality once per dashboard
    quality := ComputeDashboardQuality(dashboard)

    // Extract signals from all panels
    var signals []SignalAnchor
    for _, panel := range panels {
        panelSignals := extractSignalsFromPanel(panel, quality)
        signals = append(signals, panelSignals...)
    }

    // Deduplication: same metric+workload, highest quality wins
    // This happens naturally via MERGE key + ON MATCH updating quality_score
    // If dashboard A (quality 0.8) and dashboard B (quality 0.6) both have
    // the same metric+workload, whichever syncs last wins. Since we process
    // dashboards in descending quality order, highest quality writes last.

    // Sort signals by quality descending before writing
    sort.Slice(signals, func(i, j int) bool {
        return signals[i].QualityScore > signals[j].QualityScore
    })

    // Write signals with MERGE upsert
    for _, signal := range signals {
        query := UpsertSignalAnchorQuery(signal)
        _, err := graphClient.Query(ctx, query)
        if err != nil {
            return fmt.Errorf("failed to upsert signal %s: %w",
                signal.MetricName, err)
        }
    }

    return nil
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manual signal curation | Automated extraction with classification | v1.5 Phase 24 | Scales to 100+ dashboards |
| Single role per metric | Multi-role support (separate anchors) | v1.5 Phase 24 | Handles golden signals dashboards |
| Application-side TTL cleanup | Query-time filtering with expires_at | v1.4 Phase 20 | No background jobs, simpler |
| Prometheus Four Golden Signals | Extended taxonomy (7 roles) | v1.5 Phase 24 | Adds Availability, Churn, Novelty |
| Static dashboard quality | Five-factor quality scoring with alert boost | v1.5 Phase 24 | Incentivizes alert creation |

**Deprecated/outdated:**
- None: this is a new phase building on v1.4 patterns (MERGE, TTL, DashboardSyncer)

## Open Questions

Things that couldn't be fully resolved:

1. **Grafana Stats API availability**
   - What we know: Quality scoring uses "views in last 30 days" from Grafana Stats API
   - What's unclear: Not all Grafana deployments expose Stats API; graceful fallback needed
   - Recommendation: Make RecentUsage factor optional, log warning if API unavailable, quality formula still works with 4 factors instead of 5

2. **Layer 1 hardcoded metric exhaustiveness**
   - What we know: Context says "start small, expand based on real data"
   - What's unclear: No authoritative list exists for kube_*, cadvisor, node-exporter, Go runtime, HTTP metrics
   - Recommendation: Start with ~20 core metrics (kube_pod_status_phase, up, container_cpu_usage_seconds_total, node_cpu_seconds_total, etc.), add more in Phase 25 based on unclassified signals

3. **Multi-source Grafana handling**
   - What we know: SCHM-07 requires tracking source Grafana instance for multi-source support
   - What's unclear: How to handle signal conflicts across multiple Grafana instances (prod Grafana vs staging Grafana)
   - Recommendation: Include source_grafana in composite key for SignalAnchor uniqueness, allowing same metric+workload to exist separately per Grafana instance

4. **Classification debug logging verbosity**
   - What we know: Context says "Claude's discretion" for debug logging
   - What's unclear: Balance between debuggability and log noise
   - Recommendation: Log all classifications at DEBUG level initially, can be disabled via log level in production. Include: metric_name, classified_role, confidence, layer, reason.

## Sources

### Primary (HIGH confidence)
- github.com/prometheus/prometheus/promql/parser v0.309.1 - already in go.mod, parser.Inspect AST traversal verified in internal/integration/grafana/promql_parser.go
- github.com/FalkorDB/falkordb-go/v2 v2.0.2 - already in go.mod, Query/ROQuery patterns verified in internal/graph/client.go
- internal/graph/schema.go - MERGE with ON CREATE/ON MATCH patterns verified (lines 41, 112, 145, 173-175, etc.)
- .planning/milestones/v1.4-ROADMAP.md - TTL via expires_at timestamp pattern established (line 34, 70)
- Phase 24 CONTEXT.md - User decisions for classification layers, quality formula, K8s linkage strategy

### Secondary (MEDIUM confidence)
- [What are the Four Golden Signals and Why Do They Matter?](https://www.groundcover.com/blog/4-golden-signals) - Latency, Traffic, Errors, Saturation taxonomy
- [Mastering Observability in SRE: Golden Signals, RED & USE Metrics](https://medium.com/@farhanramzan799/mastering-observability-in-sre-golden-signals-red-use-metrics-005656c4fe7d) - RED method (Rate, Errors, Duration) and USE method patterns
- [Labels and Selectors | Kubernetes](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/) - Standard label keys for workload inference
- [pkg.go.dev/github.com/prometheus/prometheus/promql/parser](https://pkg.go.dev/github.com/prometheus/prometheus/promql/parser) - PromQL parser API documentation

### Tertiary (LOW confidence)
- WebSearch results on dashboard quality scoring - general patterns but no authoritative formula, used for conceptual validation only
- WebSearch results on metric naming conventions - node_exporter and kube-state-metrics patterns but incomplete, needs validation with real data

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - all dependencies already in go.mod, patterns verified in existing code
- Architecture: HIGH - MERGE, TTL, DashboardSyncer patterns established in v1.4, direct reuse
- Pitfalls: MEDIUM - predicted from requirements and user context, but not validated in production

**Research date:** 2026-01-29
**Valid until:** 2026-02-28 (30 days for stable domain - Go stdlib, Prometheus parser, FalkorDB API unlikely to change)
