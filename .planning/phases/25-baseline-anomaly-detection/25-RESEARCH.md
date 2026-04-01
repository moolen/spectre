# Phase 25: Baseline & Anomaly Detection - Research

**Researched:** 2026-01-29
**Domain:** Rolling statistical baselines with z-score/percentile anomaly detection and hierarchical aggregation
**Confidence:** HIGH

## Summary

Phase 25 implements rolling baselines per SignalAnchor for anomaly detection using z-score and percentile comparison. The architecture stores rolling statistics (median, P50/P90/P99, stddev, min/max, sample count) in FalkorDB graph nodes, computes anomaly scores (0.0-1.0) by combining z-score and percentile methods, treats Grafana alert state as a strong anomaly signal (firing = 1.0), and aggregates anomalies upward through the entity hierarchy (signals -> workloads -> namespaces -> clusters).

Research confirms the standard stack is already in place: `gonum.org/v1/gonum/stat` v0.17.0 for statistical functions (already used in baseline.go and flappiness.go), FalkorDB for graph storage with established MERGE/TTL patterns from Phase 24, and the existing Grafana client for querying metrics. The key extension is adding a new `SignalBaseline` node type to store rolling statistics per SignalAnchor, with periodic updates from forward collection and opt-in historical backfill.

The anomaly scoring algorithm combines z-score (distance from mean in standard deviations) with percentile comparison (current value vs historical P99) using MAX of both methods. This aligns with the CONTEXT.md decision: "anomaly if EITHER method flags it." Cold start handling returns "unknown" state with confidence=0 until minimum 10 samples are collected, per user decisions.

**Primary recommendation:** Extend FalkorDB schema with SignalBaseline nodes linked to SignalAnchor, use gonum/stat for statistical computations (already proven in codebase), implement periodic forward collection syncer similar to AlertStateSyncer pattern, and aggregate anomaly scores using MAX upward through entity hierarchy.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| gonum.org/v1/gonum/stat | v0.17.0 | Statistical functions (Mean, StdDev, Quantile) | Already in go.mod, proven patterns in baseline.go/flappiness.go |
| github.com/FalkorDB/falkordb-go/v2 | v2.0.2 | Graph database for baseline storage | Already integrated, MERGE/TTL patterns established |
| github.com/beorn7/perks/quantile | v1.0.1 | Streaming quantile estimation (indirect dep) | Already in go.sum, efficient for rolling percentiles |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| sort | stdlib | Sorting slices for quantile calculation | Required before stat.Quantile call |
| math | stdlib | Min/Max/Abs for score computation | Score normalization, threshold comparison |
| time | stdlib | TTL calculation, window management | Baseline expiration, collection scheduling |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| gonum/stat.Quantile | github.com/spenczar/tdigest | T-Digest is memory-efficient for streaming but adds dependency; gonum sufficient for 7-day window |
| Full sample storage | Reservoir sampling | Reservoir sampling reduces memory but loses precision; 7-day window with 5-min intervals = ~2016 samples, manageable |
| Graph-stored statistics | Redis with TTL | Redis faster but adds infrastructure; FalkorDB already handles TTL pattern well |

**Installation:**
All dependencies already in go.mod. No new packages required.

## Architecture Patterns

### Recommended Project Structure
```
internal/integration/grafana/
├── signal_baseline.go           # NEW: SignalBaseline type and operations
├── signal_baseline_store.go     # NEW: FalkorDB storage for baselines
├── anomaly_scorer.go            # NEW: Z-score + percentile scoring
├── baseline_collector.go        # NEW: Forward collection syncer
├── baseline_backfill.go         # NEW: Historical backfill service
├── anomaly_aggregator.go        # NEW: Hierarchical aggregation
├── graph_builder.go             # EXTEND: Add SignalBaseline methods
├── baseline.go                  # EXISTING: Alert baseline (different from signal baseline)
├── anomaly_service.go           # EXISTING: Metric anomaly detection
└── statistical_detector.go      # EXISTING: Z-score computation patterns
```

### Pattern 1: Rolling Statistics Storage in Graph
**What:** Store baseline statistics per SignalAnchor as linked graph node with TTL
**When to use:** Any signal that needs anomaly detection with historical context
**Example:**
```go
// Source: Extends Phase 24 SignalAnchor pattern
type SignalBaseline struct {
    // Identity (links to SignalAnchor composite key)
    MetricName        string
    WorkloadNamespace string
    WorkloadName      string
    Integration       string

    // Rolling statistics (7-day window per CONTEXT.md)
    Median      float64
    P50         float64
    P90         float64
    P99         float64
    Mean        float64
    StdDev      float64
    Min         float64
    Max         float64
    SampleCount int

    // Window metadata
    WindowStart int64 // Unix timestamp of oldest sample
    WindowEnd   int64 // Unix timestamp of newest sample

    // Timestamps
    LastUpdated int64 // Unix timestamp of last update
    ExpiresAt   int64 // TTL: LastUpdated + 7 days
}

// Graph query to store baseline (MERGE for idempotent upsert)
func UpsertSignalBaselineQuery(baseline SignalBaseline) graph.GraphQuery {
    return graph.GraphQuery{
        Query: `
            MATCH (s:SignalAnchor {
                metric_name: $metric_name,
                workload_namespace: $workload_namespace,
                workload_name: $workload_name,
                integration: $integration
            })
            MERGE (b:SignalBaseline {
                metric_name: $metric_name,
                workload_namespace: $workload_namespace,
                workload_name: $workload_name,
                integration: $integration
            })
            ON CREATE SET
                b.median = $median,
                b.p50 = $p50,
                b.p90 = $p90,
                b.p99 = $p99,
                b.mean = $mean,
                b.stddev = $stddev,
                b.min = $min,
                b.max = $max,
                b.sample_count = $sample_count,
                b.window_start = $window_start,
                b.window_end = $window_end,
                b.last_updated = $last_updated,
                b.expires_at = $expires_at
            ON MATCH SET
                b.median = $median,
                b.p50 = $p50,
                b.p90 = $p90,
                b.p99 = $p99,
                b.mean = $mean,
                b.stddev = $stddev,
                b.min = $min,
                b.max = $max,
                b.sample_count = $sample_count,
                b.window_start = $window_start,
                b.window_end = $window_end,
                b.last_updated = $last_updated,
                b.expires_at = $expires_at
            MERGE (s)-[:HAS_BASELINE]->(b)
        `,
        Parameters: map[string]interface{}{
            "metric_name":         baseline.MetricName,
            "workload_namespace":  baseline.WorkloadNamespace,
            "workload_name":       baseline.WorkloadName,
            "integration":         baseline.Integration,
            "median":              baseline.Median,
            "p50":                 baseline.P50,
            "p90":                 baseline.P90,
            "p99":                 baseline.P99,
            "mean":                baseline.Mean,
            "stddev":              baseline.StdDev,
            "min":                 baseline.Min,
            "max":                 baseline.Max,
            "sample_count":        baseline.SampleCount,
            "window_start":        baseline.WindowStart,
            "window_end":          baseline.WindowEnd,
            "last_updated":        baseline.LastUpdated,
            "expires_at":          baseline.ExpiresAt,
        },
    }
}
```

### Pattern 2: Hybrid Anomaly Scoring (Z-Score + Percentile)
**What:** Compute anomaly score using MAX of z-score and percentile methods
**When to use:** Computing anomaly score for any signal value
**Example:**
```go
// Source: CONTEXT.md decision + statistical_detector.go patterns
type AnomalyScore struct {
    Score      float64 // 0.0-1.0 (anomaly if >= 0.5 per CONTEXT.md)
    Confidence float64 // 0.0-1.0 = min(sampleConfidence, qualityScore)
    Method     string  // "z-score", "percentile", or "alert-override"
    ZScore     float64 // Raw z-score for debugging
}

// Cold start handling per CONTEXT.md
type InsufficientSamplesError struct {
    Available int
    Required  int
}

func (e InsufficientSamplesError) Error() string {
    return fmt.Sprintf("insufficient samples: have %d, need %d", e.Available, e.Required)
}

// ComputeAnomalyScore computes anomaly score using hybrid z-score + percentile
// Returns InsufficientSamplesError if sample_count < 10 (cold start)
func ComputeAnomalyScore(currentValue float64, baseline SignalBaseline, qualityScore float64) (*AnomalyScore, error) {
    // Cold start check per CONTEXT.md: minimum 10 samples
    if baseline.SampleCount < 10 {
        return nil, InsufficientSamplesError{
            Available: baseline.SampleCount,
            Required:  10,
        }
    }

    // Compute z-score (existing pattern from statistical_detector.go)
    var zScore float64
    if baseline.StdDev > 0 {
        zScore = (currentValue - baseline.Mean) / baseline.StdDev
    }

    // Z-score to normalized score (sigmoid-like mapping)
    // z=2 -> ~0.5, z=3 -> ~0.75, z=4 -> ~0.9
    zScoreNormalized := 1.0 - math.Exp(-math.Abs(zScore)/2.0)

    // Percentile-based score: compare to P99
    // If current > P99, score increases with distance
    var percentileScore float64
    if currentValue > baseline.P99 && baseline.P99 > baseline.P50 {
        excess := currentValue - baseline.P99
        range99 := baseline.P99 - baseline.P50
        percentileScore = math.Min(1.0, 0.5 + (excess / range99) * 0.5)
    } else if currentValue < baseline.Min {
        // Below minimum is also anomalous
        deficit := baseline.Min - currentValue
        rangeLow := baseline.P50 - baseline.Min
        if rangeLow > 0 {
            percentileScore = math.Min(1.0, 0.5 + (deficit / rangeLow) * 0.5)
        }
    }

    // MAX of both methods per CONTEXT.md
    score := math.Max(zScoreNormalized, percentileScore)

    // Compute confidence = min(sampleConfidence, qualityScore) per CONTEXT.md
    // sampleConfidence scales from 0.5 at 10 samples to 1.0 at 100+ samples
    sampleConfidence := math.Min(1.0, 0.5 + float64(baseline.SampleCount-10) / 180.0)
    confidence := math.Min(sampleConfidence, qualityScore)

    method := "z-score"
    if percentileScore > zScoreNormalized {
        method = "percentile"
    }

    return &AnomalyScore{
        Score:      score,
        Confidence: confidence,
        Method:     method,
        ZScore:     zScore,
    }, nil
}
```

### Pattern 3: Alert State Override
**What:** Grafana alert firing state overrides computed anomaly score to 1.0
**When to use:** When signal has an associated alert rule in firing state
**Example:**
```go
// Source: CONTEXT.md decision: "Grafana alert firing -> override anomaly score to 1.0"
func ApplyAlertOverride(score *AnomalyScore, alertState string) *AnomalyScore {
    if alertState == "firing" {
        return &AnomalyScore{
            Score:      1.0, // Human already decided this is anomalous
            Confidence: 1.0, // Alert = definitive signal
            Method:     "alert-override",
            ZScore:     score.ZScore, // Preserve for debugging
        }
    }
    return score
}

// Query to check alert state for signal's metric
func GetAlertStateForMetricQuery(metricName, integration string) graph.GraphQuery {
    return graph.GraphQuery{
        Query: `
            MATCH (a:Alert {integration: $integration})-[:MONITORS]->(m:Metric {name: $metric_name})
            RETURN a.state as state
            LIMIT 1
        `,
        Parameters: map[string]interface{}{
            "metric_name": metricName,
            "integration": integration,
        },
    }
}
```

### Pattern 4: Forward Collection Syncer
**What:** Periodic syncer that queries Grafana for current metric values and updates baselines
**When to use:** Continuous baseline maintenance (5-minute intervals per CONTEXT.md)
**Example:**
```go
// Source: alert_state_syncer.go pattern + CONTEXT.md decisions
type BaselineCollector struct {
    grafanaClient   *GrafanaClient
    queryService    *GrafanaQueryService
    graphClient     graph.Client
    integrationName string
    logger          *logging.Logger

    syncInterval time.Duration // 5 minutes per CONTEXT.md
    rateLimiter  *time.Ticker  // Hardcoded limit per CONTEXT.md

    ctx     context.Context
    cancel  context.CancelFunc
    stopped chan struct{}
}

// NewBaselineCollector creates a collector with 5-minute sync interval
func NewBaselineCollector(
    grafanaClient *GrafanaClient,
    queryService *GrafanaQueryService,
    graphClient graph.Client,
    integrationName string,
    logger *logging.Logger,
) *BaselineCollector {
    return &BaselineCollector{
        grafanaClient:   grafanaClient,
        queryService:    queryService,
        graphClient:     graphClient,
        integrationName: integrationName,
        logger:          logger,
        syncInterval:    5 * time.Minute,
        rateLimiter:     time.NewTicker(100 * time.Millisecond), // 10 req/sec
        stopped:         make(chan struct{}),
    }
}

// syncLoop pattern follows alert_state_syncer.go
func (c *BaselineCollector) syncLoop(ctx context.Context) {
    defer close(c.stopped)
    ticker := time.NewTicker(c.syncInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := c.collectAndUpdate(); err != nil {
                c.logger.Warn("Baseline collection failed: %v", err)
            }
        }
    }
}
```

### Pattern 5: Hierarchical Aggregation (MAX Score)
**What:** Aggregate anomaly scores upward through entity hierarchy using MAX
**When to use:** Computing workload/namespace/cluster level anomaly status
**Example:**
```go
// Source: CONTEXT.md decision: "MAX score - workload anomaly = worst signal anomaly"
type AggregatedAnomaly struct {
    Scope       string  // "signal", "workload", "namespace", "cluster"
    ScopeKey    string  // e.g., "default/nginx" for workload
    Score       float64 // MAX of child scores
    Confidence  float64 // MIN of child confidences (most uncertain)
    SourceCount int     // Number of signals contributing
    TopSource   string  // Signal with highest score (for debugging)
}

// Query: Aggregate signals to workload level
func AggregateWorkloadAnomalyQuery(namespace, workloadName, integration string) graph.GraphQuery {
    return graph.GraphQuery{
        Query: `
            MATCH (s:SignalAnchor {
                workload_namespace: $namespace,
                workload_name: $workload_name,
                integration: $integration
            })
            WHERE s.expires_at > $now
            OPTIONAL MATCH (s)-[:HAS_BASELINE]->(b:SignalBaseline)
            WHERE b.sample_count >= 10
            RETURN
                s.metric_name as metric,
                s.quality_score as quality,
                b.mean as mean,
                b.stddev as stddev,
                b.p99 as p99
        `,
        Parameters: map[string]interface{}{
            "namespace":     namespace,
            "workload_name": workloadName,
            "integration":   integration,
            "now":           time.Now().Unix(),
        },
    }
}

// AggregateWorkloadAnomaly computes MAX anomaly score across signals
func AggregateWorkloadAnomaly(signals []SignalWithAnomaly) *AggregatedAnomaly {
    if len(signals) == 0 {
        return nil
    }

    maxScore := 0.0
    minConfidence := 1.0
    topSource := ""

    for _, sig := range signals {
        if sig.AnomalyScore > maxScore {
            maxScore = sig.AnomalyScore
            topSource = sig.MetricName
        }
        // Quality weighting for tiebreaker per CONTEXT.md
        // Same score prefers high-quality signal as source
        if sig.AnomalyScore == maxScore && sig.QualityScore > signals[findByMetric(signals, topSource)].QualityScore {
            topSource = sig.MetricName
        }
        if sig.Confidence < minConfidence {
            minConfidence = sig.Confidence
        }
    }

    return &AggregatedAnomaly{
        Scope:       "workload",
        Score:       maxScore,
        Confidence:  minConfidence,
        SourceCount: len(signals),
        TopSource:   topSource,
    }
}
```

### Anti-Patterns to Avoid
- **Storing raw samples in graph:** Don't store all 2016 samples (7d * 288 intervals/day). Store only computed statistics (median, P50/P90/P99, mean, stddev, min, max, count).
- **Application-side TTL cleanup:** Use query-time filtering with `WHERE expires_at > $now`, not background cleanup jobs. This is the established v1.4 pattern.
- **Time-of-day bucketing:** CONTEXT.md explicitly says "no time-of-day bucketing - single rolling baseline per signal." Don't implement hour-based baselines.
- **Recursive aggregation queries:** Don't try to aggregate from cluster -> namespace -> workload -> signal in one query. Compute each level separately and cache results.
- **Alert threshold bootstrapping in code:** Alert thresholds come from Grafana alert rules, not from code configuration. The "bootstrap" is using existing alert state as anomaly signal, not computing thresholds.

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Mean/StdDev calculation | Custom sum/variance | gonum/stat.Mean, stat.StdDev | Off-by-one errors (N vs N-1), tested implementation already in baseline.go |
| Percentile computation | Manual sorting + indexing | gonum/stat.Quantile | Interpolation edge cases, stat.Quantile handles all cases |
| Rolling window storage | Custom sliding buffer | Graph node with periodic update | FalkorDB handles persistence, TTL, concurrent access |
| Syncer lifecycle | Custom goroutine management | Copy AlertStateSyncer pattern | Graceful shutdown, error handling already proven |
| Graph upsert | SELECT then INSERT/UPDATE | MERGE with ON CREATE/ON MATCH | Race conditions, duplicate handling at DB level |
| Rate limiting | Custom token bucket | time.Ticker (simple case) | For hardcoded fixed rate per CONTEXT.md, Ticker sufficient |

**Key insight:** This phase builds on established v1.4 patterns (AlertStateSyncer, baseline.go, graph MERGE). The novelty is in the anomaly scoring algorithm and hierarchical aggregation, not in infrastructure.

## Common Pitfalls

### Pitfall 1: Sample Variance vs Population Variance
**What goes wrong:** Using N divisor instead of N-1 for sample standard deviation
**Why it happens:** Different libraries default to different estimators
**How to avoid:** gonum/stat.StdDev uses N-1 (sample variance, unbiased) which is correct for baselines. Don't use stat.PopVariance.
**Warning signs:** Systematically understated stddev, leading to inflated z-scores

### Pitfall 2: Empty Baseline During Cold Start
**What goes wrong:** Division by zero in z-score computation, NaN scores
**Why it happens:** Forgot to check sample_count before computation
**How to avoid:** Per CONTEXT.md: return InsufficientSamplesError when sample_count < 10. Check BEFORE computing z-score.
**Warning signs:** NaN or Inf in anomaly scores, panic on first signal ingestion

### Pitfall 3: Percentile on Unsorted Data
**What goes wrong:** Wrong percentile values
**Why it happens:** stat.Quantile requires sorted input, easy to forget
**How to avoid:** Always sort.Float64s(values) before calling stat.Quantile
**Warning signs:** P50 > P99, P90 < Median

### Pitfall 4: Stale Baseline After Signal Expiration
**What goes wrong:** SignalAnchor expires but SignalBaseline persists, orphaned data
**Why it happens:** Forgot to link baseline TTL to signal TTL
**How to avoid:** Set SignalBaseline.ExpiresAt = SignalAnchor.ExpiresAt. Use query-time filtering on both.
**Warning signs:** Growing count of SignalBaseline nodes without corresponding SignalAnchors

### Pitfall 5: Rate Limit Exhaustion During Backfill
**What goes wrong:** Grafana API rate limits hit, backfill fails or blocks forward collection
**Why it happens:** Backfill of 7 days of history for many signals overwhelms API
**How to avoid:** Per CONTEXT.md: "Rate limiting: fixed hardcoded limit to protect Grafana API." Use separate rate limiter for backfill (slower than forward collection). Backfill is opt-in.
**Warning signs:** HTTP 429 responses from Grafana, forward collection delayed

### Pitfall 6: Aggregation Cache Stampede
**What goes wrong:** All cached aggregations expire simultaneously, thundering herd on graph queries
**Why it happens:** All caches set with same TTL from startup time
**How to avoid:** Add jitter to cache TTL: `ttl + random(0, 30s)`. Use sync.Map for thread-safe cache access.
**Warning signs:** Periodic CPU/latency spikes at fixed intervals

### Pitfall 7: Alert Override Without Fallback
**What goes wrong:** Alert is in "firing" state but signal baseline doesn't exist yet, lose anomaly context
**Why it happens:** Alert fires before baseline has 10 samples
**How to avoid:** Return score=1.0 with confidence=1.0 for firing alerts regardless of baseline existence. Alert state is definitive.
**Warning signs:** New alerts showing "insufficient data" despite being firing

## Code Examples

Verified patterns from official sources:

### Statistical Computation with gonum/stat
```go
// Source: gonum.org/v1/gonum/stat documentation + existing baseline.go
import (
    "sort"
    "gonum.org/v1/gonum/stat"
)

// ComputeRollingStatistics computes all statistics for a sample window
func ComputeRollingStatistics(values []float64) *RollingStats {
    if len(values) == 0 {
        return &RollingStats{SampleCount: 0}
    }

    // Sort for quantile computation (stat.Quantile requires sorted input)
    sorted := make([]float64, len(values))
    copy(sorted, values)
    sort.Float64s(sorted)

    // Compute statistics using gonum/stat
    mean := stat.Mean(values, nil)

    var stddev float64
    if len(values) >= 2 {
        stddev = stat.StdDev(values, nil) // Uses N-1 (sample variance)
    }

    // Quantiles: stat.Empirical for exact percentile at data points
    median := stat.Quantile(0.5, stat.Empirical, sorted, nil)
    p50 := median // Same as median
    p90 := stat.Quantile(0.90, stat.Empirical, sorted, nil)
    p99 := stat.Quantile(0.99, stat.Empirical, sorted, nil)

    // Min/Max from sorted array
    min := sorted[0]
    max := sorted[len(sorted)-1]

    return &RollingStats{
        Mean:        mean,
        StdDev:      stddev,
        Median:      median,
        P50:         p50,
        P90:         p90,
        P99:         p99,
        Min:         min,
        Max:         max,
        SampleCount: len(values),
    }
}

type RollingStats struct {
    Mean        float64
    StdDev      float64
    Median      float64
    P50         float64
    P90         float64
    P99         float64
    Min         float64
    Max         float64
    SampleCount int
}
```

### Backfill Service with Rate Limiting
```go
// Source: CONTEXT.md decisions + alert_state_syncer.go pattern
type BackfillService struct {
    grafanaClient   *GrafanaClient
    queryService    *GrafanaQueryService
    graphClient     graph.Client
    integrationName string
    logger          *logging.Logger

    maxBackfillDays int           // 7 per CONTEXT.md
    rateLimiter     *time.Ticker  // Slower than forward collection
}

// BackfillSignal fetches 7 days of history for a new signal
// Called automatically on signal creation per CONTEXT.md
func (s *BackfillService) BackfillSignal(ctx context.Context, signal SignalAnchor) error {
    // Calculate time range: 7 days ago to now
    now := time.Now()
    from := now.Add(-time.Duration(s.maxBackfillDays) * 24 * time.Hour)

    s.logger.Debug("Backfilling signal %s from %s to %s",
        signal.MetricName, from.Format(time.RFC3339), now.Format(time.RFC3339))

    // Fetch dashboard containing this signal
    dashboard, err := s.fetchDashboardJSON(ctx, signal.DashboardUID)
    if err != nil {
        return fmt.Errorf("fetch dashboard: %w", err)
    }

    // Find the query that produces this metric
    query, err := s.findQueryForMetric(dashboard, signal.MetricName, signal.PanelID)
    if err != nil {
        return fmt.Errorf("find query: %w", err)
    }

    // Rate limit before API call
    <-s.rateLimiter.C

    // Execute historical query via Grafana
    timeRange := TimeRange{
        From: from.Format(time.RFC3339),
        To:   now.Format(time.RFC3339),
    }

    result, err := s.queryService.ExecuteDashboard(
        ctx,
        signal.DashboardUID,
        timeRange,
        nil, // No scoped vars for backfill
        1,   // Only the panel containing this metric
    )
    if err != nil {
        return fmt.Errorf("query historical data: %w", err)
    }

    // Extract values for our specific metric
    var values []float64
    for _, panel := range result.Panels {
        for _, metric := range panel.Metrics {
            if extractMetricName(metric.Labels) == signal.MetricName {
                for _, dp := range metric.Values {
                    values = append(values, dp.Value)
                }
            }
        }
    }

    if len(values) < 10 {
        s.logger.Debug("Insufficient historical data for %s: got %d samples",
            signal.MetricName, len(values))
        return nil // Not an error, just cold start
    }

    // Compute statistics and store baseline
    stats := ComputeRollingStatistics(values)
    baseline := SignalBaseline{
        MetricName:        signal.MetricName,
        WorkloadNamespace: signal.WorkloadNamespace,
        WorkloadName:      signal.WorkloadName,
        Integration:       signal.SourceGrafana,
        Median:            stats.Median,
        P50:               stats.P50,
        P90:               stats.P90,
        P99:               stats.P99,
        Mean:              stats.Mean,
        StdDev:            stats.StdDev,
        Min:               stats.Min,
        Max:               stats.Max,
        SampleCount:       stats.SampleCount,
        WindowStart:       from.Unix(),
        WindowEnd:         now.Unix(),
        LastUpdated:       now.Unix(),
        ExpiresAt:         now.Add(7 * 24 * time.Hour).Unix(),
    }

    return s.storeBaseline(ctx, baseline)
}
```

### Anomaly Aggregation Cache
```go
// Source: CONTEXT.md decision: "Caching: aggregated scores cached with TTL, refresh periodically"
import (
    "sync"
    "time"
)

type AggregationCache struct {
    mu      sync.RWMutex
    entries map[string]*CacheEntry
    ttl     time.Duration // Claude's discretion: recommend 5 minutes
}

type CacheEntry struct {
    Value     *AggregatedAnomaly
    ExpiresAt time.Time
}

func NewAggregationCache(ttl time.Duration) *AggregationCache {
    return &AggregationCache{
        entries: make(map[string]*CacheEntry),
        ttl:     ttl,
    }
}

// Get returns cached aggregation or nil if expired/missing
func (c *AggregationCache) Get(key string) *AggregatedAnomaly {
    c.mu.RLock()
    defer c.mu.RUnlock()

    entry, ok := c.entries[key]
    if !ok {
        return nil
    }

    if time.Now().After(entry.ExpiresAt) {
        return nil // Expired
    }

    return entry.Value
}

// Set stores aggregation with TTL jitter to prevent stampede
func (c *AggregationCache) Set(key string, value *AggregatedAnomaly) {
    c.mu.Lock()
    defer c.mu.Unlock()

    // Add jitter to TTL (0-30 seconds)
    jitter := time.Duration(time.Now().UnixNano()%30) * time.Second

    c.entries[key] = &CacheEntry{
        Value:     value,
        ExpiresAt: time.Now().Add(c.ttl + jitter),
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Time-of-day baselines | Single rolling baseline | v1.5 Phase 25 | Simpler, less data, per CONTEXT.md decision |
| Metric-level anomaly detection | Signal-level anomaly detection | v1.5 Phase 25 | Ties to K8s workloads via SignalAnchor |
| Independent anomaly scores | Hierarchical aggregation | v1.5 Phase 25 | Enables workload/namespace/cluster views |
| Statistical-only detection | Alert state integration | v1.5 Phase 25 | Human decisions (alerts) take precedence |
| Manual threshold tuning | Alert-bootstrapped thresholds | v1.5 Phase 25 | Leverages existing Grafana alert rules |

**Deprecated/outdated:**
- Time-of-day matching in anomaly_service.go (matchTimeWindows) is NOT used for Phase 25 per CONTEXT.md. Single rolling baseline per signal.
- The existing `Baseline` type in baseline.go is for alert state distribution, NOT for signal metric baselines. Phase 25 introduces separate `SignalBaseline` type.

## Open Questions

Things that couldn't be fully resolved:

1. **Exact rate limit value for Grafana API protection**
   - What we know: CONTEXT.md says "fixed hardcoded limit" as Claude's discretion
   - What's unclear: Optimal rate depends on Grafana deployment (cloud vs self-hosted)
   - Recommendation: Start with 10 requests/second for forward collection, 2 requests/second for backfill. Make configurable via constants.

2. **Cache TTL duration for aggregated scores**
   - What we know: CONTEXT.md says "cached with TTL, refresh periodically" as Claude's discretion
   - What's unclear: Balance between freshness and graph query load
   - Recommendation: 5 minutes to match forward collection interval. Aggregation should refresh after each collection cycle.

3. **Z-score threshold for anomaly detection**
   - What we know: CONTEXT.md says "Anomaly threshold: 0.5 - above this = anomalous"
   - What's unclear: How to map z-score to 0.0-1.0 score (linear? sigmoid?)
   - Recommendation: Use sigmoid-like mapping where z=2 -> 0.5, z=3 -> 0.75. This makes threshold=0.5 equivalent to ~2 standard deviations.

4. **Percentile thresholds for anomaly flagging**
   - What we know: Current value > P99 should flag anomaly
   - What's unclear: How much above P99 = score 1.0? What about values below P1?
   - Recommendation: Score = 0.5 at P99 boundary, linear scale up to 1.0 at 2x(P99-P50) above P99. Mirror for low values.

5. **Incremental baseline update vs full recompute**
   - What we know: Need to store 7-day rolling statistics
   - What's unclear: Store all samples and recompute, or use streaming algorithms?
   - Recommendation: Store samples in separate cache/storage for computation, store only statistics in graph. For MVP, recompute from samples; optimize later with streaming algorithms if needed.

## Sources

### Primary (HIGH confidence)
- gonum.org/v1/gonum/stat v0.17.0 - already in go.mod, verified stat.Mean, stat.StdDev, stat.Quantile in existing baseline.go and flappiness.go
- github.com/FalkorDB/falkordb-go/v2 v2.0.2 - already in go.mod, MERGE/TTL patterns verified in graph_builder.go
- internal/integration/grafana/baseline.go - verified gonum/stat.StdDev usage for sample variance
- internal/integration/grafana/alert_state_syncer.go - syncer lifecycle pattern (Start/Stop/syncLoop)
- internal/integration/grafana/statistical_detector.go - z-score computation pattern
- Phase 25 CONTEXT.md - User decisions for all major architectural choices

### Secondary (MEDIUM confidence)
- [gonum stat package documentation](https://pkg.go.dev/gonum.org/v1/gonum/stat) - API for Mean, StdDev, Quantile functions
- [Anomaly Detection using Z-Scores](https://medium.com/analytics-vidhya/anomaly-detection-by-modified-z-score-f8ad6be62bac) - Z-score thresholds (2-3 sigma) for anomaly detection
- [The role of baselines in anomaly detection](https://www.eyer.ai/blog/the-role-of-baselines-in-anomaly-detection/) - Rolling window baseline best practices
- [VictoriaMetrics Anomaly Detection Models](https://docs.victoriametrics.com/anomaly-detection/components/models/) - Rolling quantile model patterns

### Tertiary (LOW confidence)
- WebSearch results on streaming quantile algorithms (T-Digest, etc.) - Not needed for MVP per decision to recompute from samples
- WebSearch results on cache stampede prevention - Standard jitter technique confirmed

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - all dependencies already in go.mod, patterns verified in existing code
- Architecture: HIGH - extends Phase 24 patterns (SignalAnchor, MERGE, TTL), syncer pattern proven in AlertStateSyncer
- Pitfalls: MEDIUM - predicted from statistical computing experience and CONTEXT.md constraints, not production-validated

**Research date:** 2026-01-29
**Valid until:** 2026-02-28 (30 days for stable domain - gonum API unlikely to change)
