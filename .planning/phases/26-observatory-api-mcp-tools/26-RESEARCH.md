# Phase 26: Observatory API & MCP Tools - Research

**Researched:** 2026-01-30
**Domain:** MCP tool design, Go service layer patterns, observability API patterns
**Confidence:** HIGH

## Summary

Phase 26 builds 8 MCP tools for AI-driven incident investigation through progressive disclosure (Orient → Narrow → Investigate → Hypothesize → Verify). The phase leverages existing infrastructure from Phase 24 (SignalAnchors, classification, quality scoring) and Phase 25 (baselines, anomaly detection, aggregation).

The research reveals that the codebase already contains the core building blocks: `AnomalyAggregator` for hierarchical scoring, `SignalBaseline` for statistical baselines, `BaselineCollector` for metric ingestion, and graph queries for topology. The primary work is creating thin service/tool layers that compose these components, following the established patterns in `tools_alerts_aggregated.go` and `cluster_health.go`.

Key insight: The existing Grafana integration tools demonstrate the exact pattern needed - tools receive minimal params, query graph for data, compose services for computation, and return minimal JSON responses. This phase extends that pattern with anomaly-focused tools.

**Primary recommendation:** Build service layer (`ObservatoryService`) to encapsulate graph queries and business logic, then create thin MCP tool wrappers. Reuse existing `AnomalyAggregator`, `SignalBaseline`, and graph infrastructure. Follow progressive disclosure principle: each tool returns only what's needed for its investigation stage.

## Standard Stack

### Core Libraries (Already in Use)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| mark3labs/mcp-go | v0.43.2 | MCP server implementation | Already used for cluster_health, resource_timeline tools. Proven stable. |
| FalkorDB/falkordb-go/v2 | v2.0.2 | Graph database client | Already used throughout codebase. Cypher query support. |
| gonum.org/v1/gonum | v0.17.0 | Statistical computation | Already used for baseline statistics (z-score, percentiles). |
| github.com/moolen/spectre/internal/graph | internal | Graph client abstraction | Project's graph service layer. |
| github.com/moolen/spectre/internal/api | internal | Service layer patterns | Established patterns for TimelineService, GraphService. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| encoding/json | stdlib | JSON marshaling for tool params/responses | All MCP tool I/O |
| context | stdlib | Request scoping and cancellation | All service methods |
| time | stdlib | Time range parsing, duration handling | Time-based filtering |
| sync | stdlib | Thread-safe caching (sync.Map, sync.RWMutex) | AggregationCache pattern |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| mark3labs/mcp-go | modelcontextprotocol/go-sdk (official) | Official SDK is newer, but mark3labs is already integrated and stable in codebase |
| Service layer pattern | Direct graph queries in tools | Service layer enables testing, reuse, and cleaner separation |
| Separate services per tool | Single monolithic service | Separate services scale better but add complexity for this phase scope |

**Installation:**
```bash
# All dependencies already in go.mod - no new external dependencies needed
go mod download
```

## Architecture Patterns

### Recommended Project Structure

```
internal/integration/grafana/
├── observatory_service.go              # Core service layer (Orient/Narrow queries)
├── observatory_investigate_service.go  # Investigation-specific logic
├── observatory_evidence_service.go     # Evidence aggregation
├── observatory_tools.go                # MCP tool registrations
├── tools_observatory_status.go         # Tool: observatory_status
├── tools_observatory_changes.go        # Tool: observatory_changes
├── tools_observatory_scope.go          # Tool: observatory_scope
├── tools_observatory_signals.go        # Tool: observatory_signals
├── tools_observatory_signal_detail.go  # Tool: observatory_signal_detail
├── tools_observatory_compare.go        # Tool: observatory_compare
├── tools_observatory_explain.go        # Tool: observatory_explain
├── tools_observatory_evidence.go       # Tool: observatory_evidence
└── observatory_test.go                 # Integration tests
```

### Pattern 1: Service Layer with Tool Wrappers

**What:** Thin tool layer calls service layer for business logic. Service layer encapsulates graph queries, caching, and composition.

**When to use:** All 8 observatory tools follow this pattern.

**Example:**
```go
// Service layer (testable, reusable)
type ObservatoryService struct {
    graphClient     graph.Client
    anomalyAgg      *AnomalyAggregator
    integrationName string
    logger          *logging.Logger
}

func (s *ObservatoryService) GetClusterAnomalies(ctx context.Context, opts ScopeOptions) (*ClusterAnomaliesResult, error) {
    // Business logic: query graph, aggregate scores, filter, rank
    result, err := s.anomalyAgg.AggregateClusterAnomaly(ctx)
    if err != nil {
        return nil, err
    }
    // Apply filters, rank by score
    return formatForOrientStage(result), nil
}

// Tool layer (thin MCP wrapper)
type ObservatoryStatusTool struct {
    service *ObservatoryService
}

func (t *ObservatoryStatusTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
    var params StatusParams
    if err := json.Unmarshal(args, &params); err != nil {
        return nil, fmt.Errorf("invalid parameters: %w", err)
    }
    // Validate, call service, return response
    return t.service.GetClusterAnomalies(ctx, params.ToScopeOptions())
}
```

**Source:** Existing `cluster_health.go` and `tools_alerts_aggregated.go` demonstrate this exact pattern.

### Pattern 2: Progressive Disclosure Response Design

**What:** Each tool returns minimal data for its investigation stage. No suggestions, no verbose explanations. Let AI interpret.

**When to use:** All 8 tools. Per CONTEXT.md: "Minimal responses — facts only, AI interprets meaning."

**Example:**
```go
// Orient stage: High-level summary
type ClusterAnomaliesResult struct {
    TopHotspots []Hotspot `json:"top_hotspots"`      // Top 5 only
    TotalAnomalousSignals int `json:"total_anomalous_signals"`
    Timestamp string `json:"timestamp"`              // ISO8601
}

type Hotspot struct {
    Namespace string  `json:"namespace"`
    Workload  string  `json:"workload"`
    Score     float64 `json:"score"`           // 0.0-1.0 numeric only
    Confidence float64 `json:"confidence"`     // 0.0-1.0
}

// NO: suggestions, next_steps, severity labels ("critical"), URLs
```

**Source:** [Progressive Disclosure Matters: Applying 90s UX Wisdom to 2026 AI Agents](https://aipositive.substack.com/p/progressive-disclosure-matters) discusses the Agent Skills standard by Anthropic.

### Pattern 3: Cached Aggregation with Jitter

**What:** Cache aggregated anomaly scores at each hierarchy level (signal → workload → namespace → cluster) with 5-minute TTL + jitter to prevent stampede.

**When to use:** All aggregation queries (Orient, Narrow scopes).

**Example:**
```go
// Already implemented in anomaly_aggregator.go
type AggregationCache struct {
    data      sync.Map
    ttl       time.Duration    // 5 minutes per CONTEXT.md
    jitterMax time.Duration    // 30 seconds
}

func (c *AggregationCache) Set(key string, result *AggregatedAnomaly) {
    jitter := time.Duration(rand.Int63n(int64(c.jitterMax)))
    expiresAt := time.Now().Add(c.ttl + jitter)
    c.data.Store(key, &cacheEntry{result: result, expiresAt: expiresAt})
}
```

**Source:** Existing `AggregationCache` in `anomaly_aggregator.go`. Pattern documented in [API Design Best Practices - Azure Architecture Center](https://learn.microsoft.com/en-us/azure/architecture/best-practices/api-design).

### Pattern 4: Hybrid Cypher + In-Memory Filtering

**What:** Use Cypher for structural queries (relationships, topology), then filter/rank in-memory (anomaly scores, thresholds).

**When to use:** Queries that need both graph structure and computed scores.

**Example:**
```go
// Cypher: fetch signals with baselines
query := `
    MATCH (s:SignalAnchor {workload_namespace: $namespace})
    WHERE s.expires_at > $now
    OPTIONAL MATCH (s)-[:HAS_BASELINE]->(b:SignalBaseline)
    RETURN s.metric_name, s.quality_score, b.mean, b.std_dev, b.sample_count
`
result, err := graphClient.ExecuteQuery(ctx, graph.GraphQuery{...})

// In-memory: compute anomaly scores, filter by threshold
for _, row := range result.Rows {
    score, err := ComputeAnomalyScore(currentValue, baseline, qualityScore)
    if err != nil || score.Score < 0.5 { // Threshold per CONTEXT.md
        continue
    }
    anomalies = append(anomalies, score)
}
```

**Source:** Existing pattern in `anomaly_aggregator.go` getWorkloadSignals() method.

### Anti-Patterns to Avoid

- **Verbose responses with explanations:** Tools should return facts only. No "The workload is healthy because..." text. AI interprets.
- **Next-step suggestions in responses:** Per CONTEXT.md: "No next-step suggestions in responses — AI decides flow independently."
- **Categorical severity labels:** Return numeric scores (0.0-1.0) only. No "critical", "warning", "info" strings (violates CONTEXT.md).
- **URLs in responses:** Per CONTEXT.md: "No URLs in MCP responses — keep responses data-only."
- **Empty result padding:** Per CONTEXT.md: "Empty results when nothing anomalous (no 'healthy' message, no low-score padding)."

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Anomaly score computation | Custom z-score/percentile logic | Existing `ComputeAnomalyScore()` in `anomaly_scorer.go` | Already implements hybrid z-score + percentile with sigmoid normalization, confidence decay, alert override |
| Baseline statistics | Custom mean/stddev/percentile | Existing `ComputeRollingStatistics()` using gonum/stat | gonum handles edge cases (N-1 formula, percentile interpolation), already tested |
| Aggregation caching | Custom cache with TTL | Existing `AggregationCache` pattern | Handles jitter, thread safety, expiration cleanup |
| Signal classification | Regex-based metric name parsing | Existing `SignalClassifier` with layered confidence | 5-layer classification with confidence decay already implemented and tuned |
| Graph queries for topology | Manual Cypher construction | Existing `GraphService` patterns from K8s graph | Handles pagination, error cases, column mapping |
| Time range parsing | String splitting | `time.Parse()` with RFC3339 | Handles timezones, validation, duration calculation |
| Workload inference from labels | Custom label parsing | Existing `WorkloadInference` in signal extraction | Prioritizes deployment > app > service labels with confidence scores |

**Key insight:** Phase 24-25 built the anomaly detection infrastructure. Phase 26 is primarily about exposing it through MCP tools with minimal new logic.

## Common Pitfalls

### Pitfall 1: Over-Engineering Tool Responses

**What goes wrong:** Adding verbose explanations, suggestions, categorical labels to make responses "helpful" for LLMs.

**Why it happens:** Instinct to provide context, but this bloats AI context window and violates progressive disclosure.

**How to avoid:** Return raw numeric scores (0.0-1.0) and identifiers only. Let AI reason about meaning. Follow CONTEXT.md strictly.

**Warning signs:**
- Response contains strings like "This workload is experiencing high error rates"
- Responses include "next_steps" or "recommendations" fields
- Using "critical"/"warning"/"info" instead of numeric scores

### Pitfall 2: Ignoring Cold Start (InsufficientSamplesError)

**What goes wrong:** Attempting anomaly detection on signals with < 10 baseline samples causes errors or incorrect scores.

**Why it happens:** Baseline collection is asynchronous. New signals don't have history yet.

**How to avoid:** Check `baseline.SampleCount < MinSamplesRequired` and skip signal gracefully. Don't return error to user.

**Warning signs:**
- Tool returns 500 errors during startup
- All anomaly queries fail when baselines are cold
- Tests fail without waiting for baseline warmup

**Example:**
```go
score, err := ComputeAnomalyScore(value, baseline, quality)
if err != nil {
    var insufficientErr *InsufficientSamplesError
    if errors.As(err, &insufficientErr) {
        continue // Skip signal silently
    }
    return nil, err // Other errors should fail
}
```

### Pitfall 3: Cache Stampede on Aggregation Queries

**What goes wrong:** Multiple concurrent requests for same aggregation (e.g., namespace anomaly) hit cache expiration simultaneously, causing thundering herd to graph/computation layer.

**Why it happens:** Naive TTL expiration without jitter.

**How to avoid:** Use existing `AggregationCache` pattern with 30-second jitter. Already implemented in `anomaly_aggregator.go`.

**Warning signs:**
- Spikes in graph query latency at 5-minute intervals
- Multiple concurrent expensive aggregations for same scope
- Cache hit rate drops periodically

### Pitfall 4: Missing Expires_at Filtering in Graph Queries

**What goes wrong:** Queries return stale SignalAnchors/SignalBaselines that should have expired (> 7 days old).

**Why it happens:** Forgetting `WHERE s.expires_at > $now` clause in Cypher queries.

**How to avoid:** Always include TTL filtering. Follow pattern from existing queries in `anomaly_aggregator.go`.

**Warning signs:**
- Anomaly counts don't decrease when signals age out
- Graph queries return increasing result counts over time
- Stale metrics from deleted dashboards appear in results

**Example:**
```go
query := `
    MATCH (s:SignalAnchor {integration: $integration})
    WHERE s.expires_at > $now  // CRITICAL: filter expired signals
    RETURN s.metric_name, s.workload_name
`
```

### Pitfall 5: Time Range Validation Bypass

**What goes wrong:** Tools accept arbitrary time ranges without validation, allowing 30-day queries that overwhelm Grafana or return meaningless results.

**Why it happens:** Assuming LLM will always provide sensible ranges.

**How to avoid:** Validate time ranges per CONTEXT.md: support relative (lookback duration) AND absolute (from/to), but enforce max duration (7 days per existing `TimeRange.Validate()`).

**Warning signs:**
- Grafana API timeouts on tool calls
- Baseline queries taking > 30 seconds
- Out-of-memory errors during metric processing

## Code Examples

Verified patterns from existing codebase:

### Orient Stage: Cluster-Wide Anomaly Summary

```go
// Source: Adapted from anomaly_aggregator.go AggregateClusterAnomaly()
type ObservatoryStatusResponse struct {
    TopHotspots []Hotspot `json:"top_hotspots"`
    TotalAnomalousSignals int `json:"total_anomalous_signals"`
    Timestamp string `json:"timestamp"` // ISO8601
}

type Hotspot struct {
    Namespace string `json:"namespace"`
    Workload string `json:"workload,omitempty"` // Optional: may be namespace-level
    Score float64 `json:"score"`         // 0.0-1.0
    Confidence float64 `json:"confidence"` // 0.0-1.0
    SignalCount int `json:"signal_count"`
}

func (s *ObservatoryService) GetClusterAnomalies(ctx context.Context) (*ObservatoryStatusResponse, error) {
    // Query cluster-level aggregation with caching
    result, err := s.anomalyAgg.AggregateClusterAnomaly(ctx)
    if err != nil {
        return nil, err
    }

    // Query all namespace aggregations for hotspots
    namespaces, err := s.getClusterNamespaces(ctx)
    if err != nil {
        return nil, err
    }

    hotspots := make([]Hotspot, 0)
    for _, ns := range namespaces {
        nsResult, err := s.anomalyAgg.AggregateNamespaceAnomaly(ctx, ns)
        if err != nil || nsResult == nil {
            continue
        }
        if nsResult.Score >= 0.5 { // Threshold per CONTEXT.md
            hotspots = append(hotspots, Hotspot{
                Namespace: ns,
                Score: nsResult.Score,
                Confidence: nsResult.Confidence,
                SignalCount: nsResult.SourceCount,
            })
        }
    }

    // Rank by score descending, limit to top 5
    sort.Slice(hotspots, func(i, j int) bool {
        return hotspots[i].Score > hotspots[j].Score
    })
    if len(hotspots) > 5 {
        hotspots = hotspots[:5]
    }

    return &ObservatoryStatusResponse{
        TopHotspots: hotspots,
        TotalAnomalousSignals: result.SourceCount,
        Timestamp: time.Now().Format(time.RFC3339),
    }, nil
}
```

### Narrow Stage: Scoped Signal Ranking

```go
// Source: Pattern from anomaly_aggregator.go getWorkloadSignals()
type ObservatorySignalsResponse struct {
    Signals []SignalSummary `json:"signals"`
    Scope string `json:"scope"` // "namespace/workload"
}

type SignalSummary struct {
    MetricName string `json:"metric_name"`
    Role string `json:"role"` // Availability, Latency, etc.
    Score float64 `json:"score"` // 0.0-1.0
    Confidence float64 `json:"confidence"` // 0.0-1.0
}

func (s *ObservatoryService) GetWorkloadSignals(ctx context.Context, namespace, workload string) (*ObservatorySignalsResponse, error) {
    // Query graph for signals with baselines
    query := `
        MATCH (s:SignalAnchor {
            workload_namespace: $namespace,
            workload_name: $workload,
            integration: $integration
        })
        WHERE s.expires_at > $now
        OPTIONAL MATCH (s)-[:HAS_BASELINE]->(b:SignalBaseline)
        RETURN s.metric_name, s.role, s.quality_score,
               b.mean, b.std_dev, b.sample_count
    `

    result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
        Query: query,
        Parameters: map[string]interface{}{
            "namespace": namespace,
            "workload": workload,
            "integration": s.integrationName,
            "now": time.Now().Unix(),
        },
    })
    if err != nil {
        return nil, err
    }

    signals := make([]SignalSummary, 0)
    for _, row := range result.Rows {
        // Parse row (column mapping logic)
        metricName := row[0].(string)
        role := row[1].(string)
        qualityScore := parseFloat64(row[2])

        // Compute anomaly score (skip if baseline missing)
        if row[5] == nil { // sample_count is nil
            continue
        }
        baseline := SignalBaseline{
            Mean: parseFloat64(row[3]),
            StdDev: parseFloat64(row[4]),
            SampleCount: parseInt(row[5]),
        }

        score, err := ComputeAnomalyScore(baseline.Mean, baseline, qualityScore)
        if err != nil {
            continue // Skip cold-start signals
        }

        if score.Score >= 0.5 {
            signals = append(signals, SignalSummary{
                MetricName: metricName,
                Role: role,
                Score: score.Score,
                Confidence: score.Confidence,
            })
        }
    }

    // Rank by score descending
    sort.Slice(signals, func(i, j int) bool {
        if signals[i].Score != signals[j].Score {
            return signals[i].Score > signals[j].Score
        }
        // Tiebreaker: higher confidence wins
        return signals[i].Confidence > signals[j].Confidence
    })

    return &ObservatorySignalsResponse{
        Signals: signals,
        Scope: fmt.Sprintf("%s/%s", namespace, workload),
    }, nil
}
```

### Investigate Stage: Signal Detail with Baseline Context

```go
// Source: Pattern from signal_baseline.go and anomaly_scorer.go
type ObservatorySignalDetailResponse struct {
    MetricName string `json:"metric_name"`
    CurrentValue float64 `json:"current_value"`
    Baseline BaselineStats `json:"baseline"`
    AnomalyScore float64 `json:"anomaly_score"` // 0.0-1.0
    Confidence float64 `json:"confidence"` // 0.0-1.0
    SourceDashboard string `json:"source_dashboard"` // Dashboard UID
}

type BaselineStats struct {
    Mean float64 `json:"mean"`
    StdDev float64 `json:"std_dev"`
    P50 float64 `json:"p50"`
    P90 float64 `json:"p90"`
    P99 float64 `json:"p99"`
    SampleCount int `json:"sample_count"`
}

func (s *ObservatoryService) GetSignalDetail(ctx context.Context, namespace, workload, metricName string) (*ObservatorySignalDetailResponse, error) {
    // Query for SignalAnchor with baseline
    query := `
        MATCH (s:SignalAnchor {
            metric_name: $metric_name,
            workload_namespace: $namespace,
            workload_name: $workload,
            integration: $integration
        })
        WHERE s.expires_at > $now
        MATCH (s)-[:HAS_BASELINE]->(b:SignalBaseline)
        MATCH (s)-[:EXTRACTED_FROM]->(q:Query)-[:BELONGS_TO]->(d:Dashboard)
        RETURN s.quality_score, d.uid AS dashboard_uid,
               b.mean, b.std_dev, b.p50, b.p90, b.p99, b.sample_count
    `

    result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{...})
    if err != nil {
        return nil, err
    }
    if len(result.Rows) == 0 {
        return nil, fmt.Errorf("signal not found")
    }

    row := result.Rows[0]
    baseline := SignalBaseline{
        Mean: parseFloat64(row[2]),
        StdDev: parseFloat64(row[3]),
        P50: parseFloat64(row[4]),
        P90: parseFloat64(row[5]),
        P99: parseFloat64(row[6]),
        SampleCount: parseInt(row[7]),
    }

    // Fetch current value from Grafana (via queryService)
    currentValue, err := s.fetchCurrentValue(ctx, namespace, workload, metricName)
    if err != nil {
        return nil, err
    }

    // Compute anomaly score
    score, err := ComputeAnomalyScore(currentValue, baseline, parseFloat64(row[0]))
    if err != nil {
        return nil, err
    }

    return &ObservatorySignalDetailResponse{
        MetricName: metricName,
        CurrentValue: currentValue,
        Baseline: BaselineStats{
            Mean: baseline.Mean,
            StdDev: baseline.StdDev,
            P50: baseline.P50,
            P90: baseline.P90,
            P99: baseline.P99,
            SampleCount: baseline.SampleCount,
        },
        AnomalyScore: score.Score,
        Confidence: score.Confidence,
        SourceDashboard: row[1].(string),
    }, nil
}
```

### MCP Tool Registration

```go
// Source: Adapted from mcp/server.go registerTools()
func (s *SpectreServer) registerObservatoryTools(observatoryService *ObservatoryService) {
    // Register observatory_status tool (Orient stage)
    s.registerTool(
        "observatory_status",
        "Get cluster-wide anomaly summary with top 5 hotspots by namespace/workload",
        NewObservatoryStatusTool(observatoryService),
        map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "cluster": map[string]interface{}{
                    "type": "string",
                    "description": "Optional: cluster name filter",
                },
            },
        },
    )

    // Register observatory_scope tool (Narrow stage)
    s.registerTool(
        "observatory_scope",
        "Get anomalous signals for a specific namespace or workload, ranked by severity",
        NewObservatoryScopeTool(observatoryService),
        map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "namespace": map[string]interface{}{
                    "type": "string",
                    "description": "Kubernetes namespace",
                },
                "workload": map[string]interface{}{
                    "type": "string",
                    "description": "Optional: workload name within namespace",
                },
            },
            "required": []string{"namespace"},
        },
    )

    // ... register remaining 6 tools
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manual alert investigation | AI-driven progressive disclosure | 2025-2026 | LLMs can now navigate investigation stages autonomously |
| Verbose API responses with guidance | Minimal fact-only responses | 2026 (Agent Skills standard) | Reduces context bloat, lets AI reason |
| Separate metrics/logs/traces tools | Unified observatory tools with evidence aggregation | Phase 26 | Single investigation flow vs. context-switching |
| Static anomaly thresholds | Hybrid z-score + percentile with confidence decay | Phase 25 | Adapts to cold-start and data quality |
| Hardcoded investigation workflows | Stateless tools, AI chooses sequence | Phase 26 | Flexibility for different incident types |

**Deprecated/outdated:**
- Separate `grafana_alerts_*` tools: Will be superseded by observatory tools (per CONTEXT.md: "eventually remove the other alert/logs tools")
- Categorical severity labels: Replaced by numeric scores 0.0-1.0 (per CONTEXT.md)
- Tool response suggestions: Removed to follow progressive disclosure (per CONTEXT.md)

## Open Questions

Things that couldn't be fully resolved:

1. **Internal anomaly score threshold**
   - What we know: CONTEXT.md specifies "Fixed anomaly score threshold internally" but leaves value to "Claude's discretion"
   - What's unclear: Exact threshold (0.5 seems reasonable based on scoring math, but needs validation)
   - Recommendation: Start with 0.5 (halfway point in 0-1 range), make it a const in service layer for easy tuning

2. **Response pagination defaults**
   - What we know: CONTEXT.md leaves "Response pagination / limit defaults" to discretion
   - What's unclear: Top N for Orient stage (5 hotspots?), max signals for Narrow (50? 100?)
   - Recommendation: Top 5 for Orient (per CONTEXT.md hotspot requirement), top 20 for Narrow (matches existing anomaly detection limit in `anomaly_service.go`)

3. **Evidence tool log excerpt strategy**
   - What we know: TOOL-16 requires "log snippets when relevant"
   - What's unclear: How to determine "relevant" (time proximity? error-level logs only?)
   - Recommendation: Fetch logs for anomalous signal's namespace/workload from graph's existing log nodes, filter to ERROR level within 5-minute window of anomaly timestamp

4. **Compare tool time window defaults**
   - What we know: TOOL-11 "accepts two signal IDs or signal + event", CONTEXT.md specifies "current vs N hours/days ago"
   - What's unclear: Default N if not specified (1 hour? 1 day?)
   - Recommendation: Default to 24 hours for workload-level comparison (captures daily patterns), expose as optional parameter

5. **Explain tool K8s graph depth**
   - What we know: TOOL-14 "returns candidate causes from K8s graph (upstream deps, recent changes)"
   - What's unclear: How many hops upstream? (direct parents only? transitive closure?)
   - Recommendation: 2-hop upstream traversal (workload -> service -> ingress/deployment), plus recent changes (last 1 hour) from graph's timeline

## Sources

### Primary (HIGH confidence)

- **Existing Codebase**: `/home/moritz/dev/spectre-via-ssh/internal/integration/grafana/`
  - `anomaly_aggregator.go`: Hierarchical aggregation with caching, MAX score pattern
  - `anomaly_scorer.go`: Hybrid z-score + percentile, confidence decay, alert override
  - `signal_baseline.go`: Statistical computation with gonum, cold-start handling
  - `baseline_collector.go`: Periodic collection loop with rate limiting
  - `tools_alerts_aggregated.go`: MCP tool pattern with service layer
  - `query_service.go`: Grafana API interaction, time range handling
- **Existing Codebase**: `/home/moritz/dev/spectre-via-ssh/internal/mcp/`
  - `server.go`: Tool registration patterns
  - `tools/cluster_health.go`: Service + tool layer separation
- **Context Document**: `.planning/phases/26-observatory-api-mcp-tools/26-CONTEXT.md`
  - User decisions on response structure, tool boundaries, investigation flow
- [mcp-go GitHub](https://github.com/mark3labs/mcp-go) - MCP server implementation patterns
- [FalkorDB GitHub](https://github.com/FalkorDB/FalkorDB) - Graph database design and patterns
- [gonum.org/v1/gonum](https://pkg.go.dev/gonum.org/v1/gonum/stat) - Statistical computation library

### Secondary (MEDIUM confidence)

- [Progressive Disclosure | AI Design Patterns](https://www.aiuxdesign.guide/patterns/progressive-disclosure) - Progressive disclosure in AI UX
- [Progressive Disclosure Matters: Applying 90s UX Wisdom to 2026 AI Agents](https://aipositive.substack.com/p/progressive-disclosure-matters) - Agent Skills standard by Anthropic
- [Web API Design Best Practices - Azure Architecture Center](https://learn.microsoft.com/en-us/azure/architecture/best-practices/api-design) - Caching and pagination patterns
- [Clean Architecture in Go](https://pkritiotis.io/clean-architecture-in-golang/) - Service layer design patterns
- [GitHub - evrone/go-clean-template](https://github.com/evrone/go-clean-template) - Clean architecture template for Go services

### Tertiary (LOW confidence - marked for validation)

- [11 Key Observability Best Practices You Should Know in 2026](https://spacelift.io/blog/observability-best-practices) - AI-powered anomaly detection trends
- [Graph Database Guide for AI Architects | 2026 - FalkorDB](https://www.falkordb.com/blog/graph-database-guide/) - GraphRAG patterns

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All libraries already in use, proven in codebase
- Architecture: HIGH - Service layer pattern established in existing tools, well-documented
- Pitfalls: HIGH - Derived from existing code analysis and documented issues (cold-start, caching, TTL filtering)
- Code examples: HIGH - Adapted directly from working codebase patterns
- Open questions: MEDIUM - Discretion areas per CONTEXT.md, need validation during planning

**Research date:** 2026-01-30
**Valid until:** 2026-02-27 (30 days - stable domain, established patterns)
