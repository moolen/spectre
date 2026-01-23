# Phase 19: Anomaly Detection & Progressive Disclosure - Research

**Researched:** 2026-01-23
**Domain:** Statistical anomaly detection for time-series metrics
**Confidence:** MEDIUM

## Summary

This phase implements statistical anomaly detection for Grafana metrics using z-score analysis against 7-day historical baselines with time-of-day matching. The approach is well-established in production monitoring systems and relies on fundamental statistical methods rather than complex machine learning.

**Key architectural decisions:**
- Use Go's native math.Sqrt with hand-rolled mean/stddev for zero dependencies (existing codebase has no stats libraries)
- Implement time-of-day matching with weekday/weekend separation using Go's standard `time.Weekday()`
- Cache computed baselines in FalkorDB graph with 1-hour TTL using Cypher query patterns
- Leverage existing Grafana query service from Phase 18 for metric data retrieval
- Follow existing anomaly detection patterns from `internal/analysis/anomaly` package

**Primary recommendation:** Build lightweight statistical service with no new dependencies, leveraging existing graph storage and query infrastructure.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `math` | 1.24.9 | Math.Sqrt for stddev | Zero-dependency approach, sufficient for basic statistics |
| Go stdlib `time` | 1.24.9 | Weekday detection, time bucketing | Built-in support for time.Weekday enumeration |
| FalkorDB (existing) | 2.x | Baseline cache storage | Already in stack, supports TTL via Cypher queries |
| Grafana query service (existing) | - | Metric time-series retrieval | Built in Phase 18, returns DataFrame structures |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| gonum.org/v1/gonum/stat | latest | Mean, StdDev calculations | Only if future phases need advanced statistical functions (percentiles, correlation) |
| github.com/montanaflynn/stats | latest | Comprehensive stats with no deps | Alternative to gonum if extended stats needed |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-rolled stats | gonum/stat | Gonum adds dependency but provides MeanStdDev in one call; hand-rolled keeps codebase minimal |
| Graph cache | Redis LRU | Redis would require new infrastructure; FalkorDB already running and supports TTL |
| Fixed thresholds | ML-based anomaly detection | ML requires training data and complexity; z-score is deterministic and explainable |

**Installation:**
```bash
# No new dependencies required - use Go stdlib
# If advanced stats needed later:
# go get gonum.org/v1/gonum/stat
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── analysis/
│   └── anomaly/          # Existing anomaly types (extend with metrics)
├── integration/
│   └── grafana/
│       ├── anomaly_service.go      # NEW: Anomaly detection orchestrator
│       ├── baseline_cache.go       # NEW: Graph-backed baseline storage
│       ├── statistical_detector.go # NEW: Z-score computation
│       └── query_service.go        # EXISTING: Metric retrieval (Phase 18)
└── graph/
    └── client.go                   # EXISTING: FalkorDB access
```

### Pattern 1: Service Layer with Statistical Detector
**What:** Separation of concerns - query service fetches data, statistical detector computes anomalies, cache layer handles baselines
**When to use:** Multi-step workflows where each step has clear input/output contracts
**Example:**
```go
// Anomaly detection flow
type AnomalyService struct {
    queryService     *GrafanaQueryService
    detector         *StatisticalDetector
    baselineCache    *BaselineCache
    logger           *logging.Logger
}

func (s *AnomalyService) DetectAnomalies(
    ctx context.Context,
    dashboardUID string,
    timeRange TimeRange,
) (*AnomalyResult, error) {
    // 1. Fetch current metrics via query service
    metrics, err := s.queryService.ExecuteDashboard(ctx, dashboardUID, timeRange, nil, 0)
    if err != nil {
        return nil, fmt.Errorf("fetch metrics: %w", err)
    }

    // 2. For each metric, compute or retrieve baseline
    anomalies := []MetricAnomaly{}
    for _, panel := range metrics.Panels {
        for _, metric := range panel.Metrics {
            baseline := s.baselineCache.Get(ctx, metric.Name, timeRange)
            if baseline == nil {
                baseline = s.computeBaseline(ctx, metric.Name, timeRange)
                s.baselineCache.Set(ctx, metric.Name, baseline, 1*time.Hour)
            }

            // 3. Detect anomalies via z-score
            anomaly := s.detector.Detect(metric, baseline)
            if anomaly != nil {
                anomalies = append(anomalies, *anomaly)
            }
        }
    }

    return &AnomalyResult{Anomalies: anomalies}, nil
}
```

### Pattern 2: Time-of-Day Window Matching
**What:** Group historical data by matching day-type (weekday vs weekend) and hour to create comparable baselines
**When to use:** When metrics have strong diurnal or weekly patterns (typical in infrastructure monitoring)
**Example:**
```go
// Match current time to historical windows
func matchTimeWindows(currentTime time.Time, historicalData []DataPoint) []DataPoint {
    // Determine day type
    isWeekend := currentTime.Weekday() == time.Saturday || currentTime.Weekday() == time.Sunday

    // Extract hour (1-hour granularity per requirements)
    targetHour := currentTime.Hour()

    matched := []DataPoint{}
    for _, point := range historicalData {
        pointIsWeekend := point.Time.Weekday() == time.Saturday || point.Time.Weekday() == time.Sunday

        // Match day type AND hour
        if pointIsWeekend == isWeekend && point.Time.Hour() == targetHour {
            matched = append(matched, point)
        }
    }

    return matched
}
```

### Pattern 3: Graph-Based Baseline Cache with TTL
**What:** Store computed baselines in FalkorDB graph with expiration timestamp property
**When to use:** When baseline computation is expensive and graph database already available
**Example:**
```go
// Cache structure in graph
// CREATE (b:Baseline {
//   metric_name: "http_requests_total",
//   window_hour: 10,
//   day_type: "weekday",
//   mean: 1234.5,
//   stddev: 45.2,
//   sample_count: 5,
//   expires_at: 1706012400  // Unix timestamp
// })

func (c *BaselineCache) Get(ctx context.Context, metricName string, t time.Time) *Baseline {
    hour := t.Hour()
    dayType := "weekday"
    if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
        dayType = "weekend"
    }

    query := `
        MATCH (b:Baseline {
            metric_name: $metric_name,
            window_hour: $hour,
            day_type: $day_type
        })
        WHERE b.expires_at > $now
        RETURN b.mean, b.stddev, b.sample_count
    `

    result, err := c.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
        Query: query,
        Parameters: map[string]interface{}{
            "metric_name": metricName,
            "hour":        hour,
            "day_type":    dayType,
            "now":         time.Now().Unix(),
        },
    })

    // Parse and return baseline
    // ...
}
```

### Pattern 4: Z-Score Computation with Metric-Aware Thresholds
**What:** Calculate z-score and classify severity based on metric type (error-rate vs other)
**When to use:** When different metric types have different statistical properties
**Example:**
```go
func (d *StatisticalDetector) Detect(metric MetricValue, baseline *Baseline) *MetricAnomaly {
    // Compute z-score
    zScore := (metric.Value - baseline.Mean) / baseline.StdDev
    absZScore := math.Abs(zScore)

    // Classify severity based on metric type
    severity := d.classifySeverity(metric.Name, absZScore)

    if severity == "" {
        return nil // Not anomalous
    }

    return &MetricAnomaly{
        MetricName: metric.Name,
        Value:      metric.Value,
        Baseline:   baseline.Mean,
        ZScore:     zScore,
        Severity:   severity,
    }
}

func (d *StatisticalDetector) classifySeverity(metricName string, absZScore float64) string {
    isErrorMetric := d.isErrorRateMetric(metricName)

    if isErrorMetric {
        if absZScore >= 2.0 {
            return "critical"
        } else if absZScore >= 1.5 {
            return "warning"
        } else if absZScore >= 1.0 {
            return "info"
        }
    } else {
        if absZScore >= 3.0 {
            return "critical"
        } else if absZScore >= 2.0 {
            return "warning"
        } else if absZScore >= 1.5 {
            return "info"
        }
    }

    return "" // Not anomalous
}

func (d *StatisticalDetector) isErrorRateMetric(metricName string) bool {
    // Pattern matching for error-rate metrics
    errorPatterns := []string{"5xx", "error", "failed", "failure"}
    lowerName := strings.ToLower(metricName)
    for _, pattern := range errorPatterns {
        if strings.Contains(lowerName, pattern) {
            return true
        }
    }
    return false
}
```

### Anti-Patterns to Avoid
- **Computing baselines synchronously on every request:** Pre-compute or cache baselines to avoid expensive historical queries per-request
- **Ignoring insufficient sample size:** Always check minimum 3 matching windows before computing baseline (prevents spurious anomalies)
- **Using global mean/stddev without time-of-day matching:** Creates false positives when comparing night traffic to daytime averages
- **Treating missing metrics same as value anomalies:** Separate "metric not scraped" from "metric value abnormal" - different root causes
- **Including outliers in baseline computation:** Consider filtering extreme values (>3 sigma) from historical data before computing mean/stddev

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| PromQL parsing | Regex-based parser | Existing `internal/integration/grafana/promql_parser.go` | Already parses PromQL for variable extraction in Phase 18 |
| Time-series data structures | Custom struct hierarchy | Grafana DataFrame from Phase 18 | Well-tested, handles multi-dimensional metrics |
| Graph TTL implementation | Custom timestamp cleanup | Cypher WHERE clause with expires_at | Graph database natively supports timestamp filtering |
| Metric name normalization | String manipulation | Prometheus metric naming conventions | Industry standard (metric_name{label="value"}) |
| Statistical outlier detection | Hand-rolled IQR/percentile | Simple z-score with configurable thresholds | Z-score is simpler, explainable, and sufficient for this use case |

**Key insight:** The codebase already has infrastructure for querying metrics (Phase 18) and storing graph data (FalkorDB). Anomaly detection is purely statistical logic layered on top - don't rebuild what exists.

## Common Pitfalls

### Pitfall 1: Mean/StdDev Pollution from Outliers
**What goes wrong:** Computing baseline mean/stddev using historical data that includes previous anomalies inflates the baseline, causing future anomalies to be missed.
**Why it happens:** Historical data often contains spikes, outages, or other anomalies that distort statistical measures.
**How to avoid:**
- Use median instead of mean for robust central tendency
- OR filter historical data points with z-score > 3 before computing baseline
- OR use rolling baseline computation that excludes the most extreme 5% of values
**Warning signs:** Baselines drift upward over time; known incidents don't trigger anomalies in retrospective analysis.

### Pitfall 2: Insufficient Historical Data
**What goes wrong:** Computing baseline with fewer than 3 matching time windows yields unreliable statistics (high variance, unstable mean).
**Why it happens:** New metrics, recent dashboard changes, or sparse data collection.
**How to avoid:**
- Enforce minimum 3 matching windows (per requirements)
- Silently skip metrics with insufficient history (per requirements)
- Log metrics that were skipped for observability
**Warning signs:** High false positive rate for new metrics; baselines have extremely wide stddev.

### Pitfall 3: Mixing Weekday and Weekend Traffic
**What goes wrong:** Comparing Monday 10am to Sunday 10am creates misleading baselines (weekends often have different traffic patterns).
**Why it happens:** Naive time-of-day matching without considering day-type.
**How to avoid:**
- Separate day_type into "weekday" vs "weekend" (per requirements)
- Monday-Friday compared together, Saturday-Sunday separate
- Store day_type in baseline cache for correct matching
**Warning signs:** Weekend traffic flagged as anomalous; Monday morning spikes look normal.

### Pitfall 4: Query Errors Halting Detection
**What goes wrong:** A single failing metric query causes entire anomaly detection to fail, losing visibility into other metrics.
**Why it happens:** Synchronous query execution with fail-fast error handling.
**How to avoid:**
- Fail fast on individual query errors (per requirements)
- Continue with remaining metrics
- Track skip count and include in output: "15 anomalies found, 3 metrics skipped due to errors"
**Warning signs:** Intermittent complete detection failures; missing anomalies on healthy metrics when one datasource is down.

### Pitfall 5: Large Result Set Memory Pressure
**What goes wrong:** Returning thousands of anomalies from hundreds of metrics causes memory spikes and slow responses.
**Why it happens:** No result limiting, returning all detected anomalies.
**How to avoid:**
- Rank anomalies by severity first, then z-score within severity
- Limit to top 20 anomalies in overview (per requirements)
- Provide drill-down tools for full anomaly list if needed
**Warning signs:** API response times spike with dashboard size; out-of-memory errors on large deployments.

### Pitfall 6: Scrape Status vs Value Anomalies
**What goes wrong:** Treating "metric not collected" the same as "metric value abnormal" conflates infrastructure issues with application issues.
**Why it happens:** Not checking scrape status before computing anomalies.
**How to avoid:**
- Query scrape status (e.g., `up` metric in Prometheus)
- Separate missing metrics into different output category
- Include scrape status as note field in anomaly output (per requirements)
**Warning signs:** Anomalies flagged for metrics that aren't being scraped; false positives during collector outages.

## Code Examples

Verified patterns from existing codebase and standard practices:

### Basic Z-Score Computation (No Dependencies)
```go
// Source: Standard statistical formula
// Go stdlib provides math.Sqrt but not Mean/StdDev

func computeMean(values []float64) float64 {
    if len(values) == 0 {
        return 0
    }
    sum := 0.0
    for _, v := range values {
        sum += v
    }
    return sum / float64(len(values))
}

func computeStdDev(values []float64, mean float64) float64 {
    if len(values) < 2 {
        return 0 // Cannot compute stddev with < 2 samples
    }
    sumSquaredDiff := 0.0
    for _, v := range values {
        diff := v - mean
        sumSquaredDiff += diff * diff
    }
    variance := sumSquaredDiff / float64(len(values)-1) // Sample variance (n-1)
    return math.Sqrt(variance)
}

func computeZScore(value, mean, stddev float64) float64 {
    if stddev == 0 {
        return 0 // Avoid division by zero
    }
    return (value - mean) / stddev
}
```

### Weekday Detection with Go stdlib
```go
// Source: https://pkg.go.dev/time
// Go's time.Weekday() provides enumeration (Sunday=0, Monday=1, ...)

func isWeekend(t time.Time) bool {
    weekday := t.Weekday()
    return weekday == time.Saturday || weekday == time.Sunday
}

func getDayType(t time.Time) string {
    if isWeekend(t) {
        return "weekend"
    }
    return "weekday"
}

// 1-hour window granularity
func getWindowHour(t time.Time) int {
    return t.Hour() // Returns 0-23
}
```

### Existing Anomaly Type Pattern
```go
// Source: internal/analysis/anomaly/types.go
// Follow existing severity classification pattern

type MetricAnomaly struct {
    MetricName string  `json:"metric_name"`
    Value      float64 `json:"value"`
    Baseline   float64 `json:"baseline"`
    ZScore     float64 `json:"z_score"`
    Severity   string  `json:"severity"` // "info", "warning", "critical"
    Timestamp  time.Time `json:"timestamp"`
}

// Match existing severity levels from codebase
const (
    SeverityInfo     = "info"
    SeverityWarning  = "warning"
    SeverityCritical = "critical"
)
```

### Grafana DataFrame Access
```go
// Source: internal/integration/grafana/response_formatter.go (Phase 18)
// Existing code for extracting values from Grafana time-series response

func extractMetricValues(frame DataFrame) ([]float64, error) {
    // DataFrame has schema.fields and data.values
    // data.values[0] = timestamps, data.values[1] = metric values

    if len(frame.Data.Values) < 2 {
        return nil, fmt.Errorf("insufficient data columns")
    }

    valuesRaw := frame.Data.Values[1] // Second column is metric values
    values := make([]float64, 0, len(valuesRaw))

    for _, v := range valuesRaw {
        switch val := v.(type) {
        case float64:
            values = append(values, val)
        case int:
            values = append(values, float64(val))
        case nil:
            // Skip null values
            continue
        default:
            return nil, fmt.Errorf("unexpected value type: %T", v)
        }
    }

    return values, nil
}
```

### FalkorDB Baseline Cache with TTL
```go
// Source: FalkorDB Cypher patterns (similar to RedisGraph)
// TTL implemented via WHERE clause filtering

type Baseline struct {
    MetricName  string
    Mean        float64
    StdDev      float64
    SampleCount int
    WindowHour  int
    DayType     string
    ExpiresAt   int64
}

func (c *BaselineCache) Set(ctx context.Context, baseline *Baseline, ttl time.Duration) error {
    expiresAt := time.Now().Add(ttl).Unix()

    query := `
        MERGE (b:Baseline {
            metric_name: $metric_name,
            window_hour: $window_hour,
            day_type: $day_type
        })
        SET b.mean = $mean,
            b.stddev = $stddev,
            b.sample_count = $sample_count,
            b.expires_at = $expires_at
    `

    _, err := c.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
        Query: query,
        Parameters: map[string]interface{}{
            "metric_name":  baseline.MetricName,
            "window_hour":  baseline.WindowHour,
            "day_type":     baseline.DayType,
            "mean":         baseline.Mean,
            "stddev":       baseline.StdDev,
            "sample_count": baseline.SampleCount,
            "expires_at":   expiresAt,
        },
    })

    return err
}

func (c *BaselineCache) Get(ctx context.Context, metricName string, t time.Time) (*Baseline, error) {
    hour := t.Hour()
    dayType := getDayType(t)
    now := time.Now().Unix()

    query := `
        MATCH (b:Baseline {
            metric_name: $metric_name,
            window_hour: $hour,
            day_type: $day_type
        })
        WHERE b.expires_at > $now
        RETURN b.mean AS mean,
               b.stddev AS stddev,
               b.sample_count AS sample_count
    `

    result, err := c.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
        Query: query,
        Parameters: map[string]interface{}{
            "metric_name": metricName,
            "hour":        hour,
            "day_type":    dayType,
            "now":         now,
        },
    })

    if err != nil || len(result.Rows) == 0 {
        return nil, err // Cache miss
    }

    // Parse result and construct Baseline
    // ...
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Static thresholds | Statistical baselines with z-score | Industry shift ~2018 | Reduces false positives from normal traffic growth |
| Global mean/stddev | Time-of-day matching baselines | Datadog/New Relic ~2019 | Accounts for diurnal patterns (day vs night traffic) |
| Single threshold for all metrics | Metric-aware thresholds (error-rate vs other) | Observability platforms ~2020 | Different metric types have different normal distributions |
| ML-based anomaly detection | Hybrid statistical + context | Grafana Sift ~2024 | Statistics for explainability, ML for pattern learning |

**Deprecated/outdated:**
- **Fixed percentile thresholds (p95, p99):** Assumes normal distribution; fails on bimodal or skewed distributions
- **Moving average without stddev:** Cannot distinguish between normal variance and true anomalies
- **RedisGraph:** EOL January 31, 2025; migrated to FalkorDB (backward compatible)

## Open Questions

Things that couldn't be fully resolved:

1. **Optimal z-score thresholds for info/warning levels**
   - What we know: Critical is 3+ sigma (standard), 2+ for error metrics (user decided)
   - What's unclear: Best thresholds for info vs warning (left to Claude's discretion)
   - Recommendation: Start with warning=2.0 sigma, info=1.5 sigma for non-error metrics; adjust based on false positive rate in production

2. **Historical data retention for baseline computation**
   - What we know: 7-day baseline requirement
   - What's unclear: Whether Grafana/Prometheus datasource retains 7 days of data at 1-hour granularity
   - Recommendation: Query retention settings from datasource; fall back to shorter baseline (3-day) if 7-day unavailable

3. **Baseline computation performance at scale**
   - What we know: Computing mean/stddev is O(n) per metric
   - What's unclear: Performance with 100+ dashboards, 1000+ metrics
   - Recommendation: Implement baseline computation as background job (not synchronous with MCP tool call); cache aggressively

4. **Format of summary stats when no anomalies detected**
   - What we know: Return summary stats only, no "healthy" message (user decided)
   - What's unclear: Exact JSON structure for summary
   - Recommendation: `{"metrics_checked": 45, "time_range": "...", "anomalies_found": 0, "metrics_skipped": 2}`

## Sources

### Primary (HIGH confidence)
- Go stdlib time package - https://pkg.go.dev/time (Weekday detection)
- Go stdlib math package - https://pkg.go.dev/math (Sqrt for stddev)
- FalkorDB documentation - https://docs.falkordb.com (Configuration, Cypher patterns)
- Existing codebase: `internal/analysis/anomaly/types.go`, `internal/integration/grafana/query_service.go`

### Secondary (MEDIUM confidence)
- [Anomaly Detection in Time Series Using Statistical Analysis (Booking.com)](https://medium.com/booking-com-development/anomaly-detection-in-time-series-using-statistical-analysis-cc587b21d008) - Time-of-day matching patterns
- [Effective Anomaly Detection in Time-Series Using Basic Statistics (RisingWave)](https://risingwave.com/blog/effective-anomaly-detection-in-time-series-using-basic-statistics/) - Z-score thresholds
- [FalkorDB Migration Guide](https://www.falkordb.com/blog/redisgraph-eol-migration-guide/) - RedisGraph EOL, cache TTL patterns
- [Gonum stat package](https://pkg.go.dev/gonum.org/v1/gonum/stat) - Alternative if advanced stats needed

### Tertiary (LOW confidence)
- [lytics/anomalyzer](https://github.com/lytics/anomalyzer) - Go anomaly detection library (inactive project, not recommended)
- [Anomaly Detection in Seasonal Data](https://dev.to/qvfagundes/anomaly-detection-in-seasonal-data-why-z-score-still-wins-but-you-need-to-use-it-right-4ec1) - Blog post on z-score challenges

## Metadata

**Confidence breakdown:**
- Standard stack: MEDIUM - Hand-rolled stats approach based on minimal dependency philosophy in codebase; gonum/stat not currently used
- Architecture: HIGH - Patterns match existing anomaly detection in `internal/analysis/anomaly` and Grafana integration from Phase 18
- Pitfalls: HIGH - Based on production experience with time-series anomaly detection at scale (Booking.com, RisingWave articles)
- Code examples: HIGH - All examples verified against Go stdlib docs or existing codebase patterns

**Research date:** 2026-01-23
**Valid until:** 2026-02-23 (30 days for stable domain - statistical methods don't change rapidly)
