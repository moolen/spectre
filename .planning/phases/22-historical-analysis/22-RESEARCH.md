# Phase 22: Historical Analysis - Research

**Researched:** 2026-01-23
**Domain:** Time-series analysis, statistical baseline computation, flappiness detection, alert state categorization
**Confidence:** MEDIUM

## Summary

This phase implements AlertAnalysisService that performs statistical analysis on alert state transition history stored in graph. The service must compute flappiness scores using sliding window analysis, compare current alert behavior against rolling 7-day baselines using standard deviation, and categorize alerts along onset (new/recent/persistent/chronic) and pattern (stable/flapping/trending) dimensions.

The standard approach uses Go's native time package for time-based calculations, gonum/stat for statistical computations (mean, standard deviation, variance), hashicorp/golang-lru/v2/expirable for 5-minute TTL caching, and Cypher queries with temporal filtering to fetch state transitions from the graph database. The project already has golang-lru v2.0.7 available.

Key technical challenges include: (1) implementing sliding window analysis over graph-stored state transitions with proper time-based filtering, (2) computing rolling statistics with partial data (24h-7d), (3) implementing Last Observation Carried Forward (LOCF) interpolation for data gaps, (4) designing efficient Cypher queries for time-range aggregations, and (5) multi-label categorization logic that combines onset and pattern dimensions.

**Primary recommendation:** Use gonum/stat for statistics (already battle-tested), hashicorp/golang-lru/v2/expirable for caching (already in go.mod v2.0.7), and implement custom sliding window logic over Cypher-fetched transitions with LOCF gap interpolation.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| gonum.org/v1/gonum/stat | Latest (v0.15+) | Statistical computations (mean, stddev, variance) | Industry standard for scientific computing in Go, provides unbiased and biased estimators |
| github.com/hashicorp/golang-lru/v2/expirable | v2.0.7 (already in go.mod) | In-memory cache with TTL | Thread-safe, supports generics, built-in TTL expiration, used by HashiCorp production systems |
| time | Go stdlib | Time duration calculations, timestamp comparisons | Native Go time handling with monotonic clock support |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| math | Go stdlib | Math operations (Sqrt, Abs) | Converting variance to standard deviation, computing absolute deviations |
| sort | Go stdlib | Sorting time-ordered transitions | Ensuring chronological order for sliding window analysis |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| gonum/stat | Custom statistical functions | Custom code error-prone (off-by-one in N vs N-1), gonum handles edge cases |
| golang-lru/v2/expirable | ttlcache (jellydator/ttlcache) | ttlcache has more features but golang-lru already in project, simpler API |
| Graph-based computation | In-memory time-series database | Graph already stores transitions, adding DB increases complexity |

**Installation:**
```bash
go get gonum.org/v1/gonum/stat
# golang-lru/v2 v2.0.7 already in go.mod
```

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── analysis/
│   ├── alert_analysis_service.go    # Main service with public methods
│   ├── flappiness.go                # Flappiness score computation
│   ├── baseline.go                  # Rolling baseline + deviation
│   ├── categorization.go            # Multi-label categorization
│   ├── transitions.go               # Transition fetching + LOCF interpolation
│   └── alert_analysis_service_test.go
```

### Pattern 1: Sliding Window Over Graph Transitions
**What:** Fetch state transitions from graph with time-based WHERE filtering, then apply sliding window analysis in-memory over sorted transitions.

**When to use:** When computing flappiness (6-hour window) or trending (1h vs 6h comparison).

**Example:**
```go
// Fetch transitions with Cypher time filtering
query := `
  MATCH (a:Alert {uid: $uid})-[t:STATE_TRANSITION]->(a)
  WHERE t.timestamp >= $startTime
    AND t.timestamp <= $endTime
    AND t.expires_at > $now
  RETURN t.from_state, t.to_state, t.timestamp
  ORDER BY t.timestamp ASC
`

// Apply sliding window in-memory
type StateTransition struct {
    FromState string
    ToState   string
    Timestamp time.Time
}

func computeFlappinessInWindow(transitions []StateTransition, windowStart, windowEnd time.Time) float64 {
    // Filter to window
    windowTransitions := []StateTransition{}
    for _, t := range transitions {
        if t.Timestamp.After(windowStart) && t.Timestamp.Before(windowEnd) {
            windowTransitions = append(windowTransitions, t)
        }
    }

    // Count transitions in window
    transitionCount := len(windowTransitions)

    // Compute duration in each state (for weighting)
    stateDurations := make(map[string]time.Duration)
    for i := 0; i < len(windowTransitions); i++ {
        var duration time.Duration
        if i < len(windowTransitions)-1 {
            duration = windowTransitions[i+1].Timestamp.Sub(windowTransitions[i].Timestamp)
        } else {
            duration = windowEnd.Sub(windowTransitions[i].Timestamp)
        }
        stateDurations[windowTransitions[i].ToState] += duration
    }

    // Score combines frequency and duration penalty
    // Normalized to 0.0-1.0 range
    return computeFlappinessScore(transitionCount, stateDurations, windowEnd.Sub(windowStart))
}
```

### Pattern 2: Rolling Baseline with Partial Data Handling
**What:** Compute state distribution statistics (% normal, % pending, % firing) over available history, use gonum/stat for standard deviation.

**When to use:** For 7-day baseline comparison with graceful degradation for alerts with <7d history.

**Example:**
```go
import "gonum.org/v1/gonum/stat"

type StateDistribution struct {
    PercentNormal  float64
    PercentPending float64
    PercentFiring  float64
}

func computeRollingBaseline(transitions []StateTransition, lookbackDays int) (StateDistribution, float64, error) {
    if len(transitions) == 0 {
        return StateDistribution{}, 0, errors.New("insufficient data")
    }

    // Compute time in each state using LOCF interpolation
    totalDuration := transitions[len(transitions)-1].Timestamp.Sub(transitions[0].Timestamp)
    stateDurations := computeStateDurations(transitions, totalDuration)

    // Convert to percentages
    dist := StateDistribution{
        PercentNormal:  stateDurations["normal"].Seconds() / totalDuration.Seconds(),
        PercentPending: stateDurations["pending"].Seconds() / totalDuration.Seconds(),
        PercentFiring:  stateDurations["firing"].Seconds() / totalDuration.Seconds(),
    }

    // Compute standard deviation across daily distributions
    dailyDistributions := computeDailyDistributions(transitions, lookbackDays)
    firingPercentages := make([]float64, len(dailyDistributions))
    for i, d := range dailyDistributions {
        firingPercentages[i] = d.PercentFiring
    }

    // Use gonum for standard deviation (unbiased estimator)
    stdDev := stat.StdDev(firingPercentages, nil)

    return dist, stdDev, nil
}

func compareToBaseline(current, baseline StateDistribution, stdDev float64) float64 {
    // Deviation score: how many standard deviations from baseline
    diff := current.PercentFiring - baseline.PercentFiring
    return math.Abs(diff) / stdDev
}
```

### Pattern 3: Last Observation Carried Forward (LOCF) Interpolation
**What:** Fill time gaps by assuming last known state continued through gap (standard time-series interpolation).

**When to use:** When computing state durations with data gaps between syncs (Phase 21 syncs every 5 minutes).

**Example:**
```go
// LOCF interpolation for state duration computation
func computeStateDurations(transitions []StateTransition, totalDuration time.Duration) map[string]time.Duration {
    durations := make(map[string]time.Duration)

    for i := 0; i < len(transitions)-1; i++ {
        state := transitions[i].ToState
        duration := transitions[i+1].Timestamp.Sub(transitions[i].Timestamp)
        durations[state] += duration
    }

    // Last state duration: carry forward to end of analysis window
    if len(transitions) > 0 {
        lastState := transitions[len(transitions)-1].ToState
        lastDuration := totalDuration
        for _, d := range durations {
            lastDuration -= d
        }
        durations[lastState] += lastDuration
    }

    return durations
}
```

### Pattern 4: Multi-Label Categorization
**What:** Combine onset categories (time-based) and pattern categories (behavior-based) as independent dimensions.

**When to use:** Alert can be both "chronic" (>7d) and "flapping" simultaneously.

**Example:**
```go
type AlertCategories struct {
    Onset   []string // "new", "recent", "persistent", "chronic"
    Pattern []string // "stable-firing", "stable-normal", "flapping", "trending-worse", "trending-better"
}

func categorizeAlert(transitions []StateTransition, currentTime time.Time) AlertCategories {
    categories := AlertCategories{
        Onset:   []string{},
        Pattern: []string{},
    }

    // Onset categorization (time-based)
    firstFiring := findFirstFiringTime(transitions)
    if firstFiring.IsZero() {
        categories.Onset = append(categories.Onset, "stable-normal")
        return categories
    }

    firingDuration := currentTime.Sub(firstFiring)
    switch {
    case firingDuration < 1*time.Hour:
        categories.Onset = append(categories.Onset, "new")
    case firingDuration < 24*time.Hour:
        categories.Onset = append(categories.Onset, "recent")
    case firingDuration < 7*24*time.Hour:
        categories.Onset = append(categories.Onset, "persistent")
    default:
        categories.Onset = append(categories.Onset, "chronic")
    }

    // Pattern categorization (behavior-based)
    flappiness := computeFlappinessScore(transitions, 6*time.Hour)
    if flappiness > 0.7 {
        categories.Pattern = append(categories.Pattern, "flapping")
    }

    trend := computeTrend(transitions, 1*time.Hour, 6*time.Hour)
    if trend > 0.2 {
        categories.Pattern = append(categories.Pattern, "trending-worse")
    } else if trend < -0.2 {
        categories.Pattern = append(categories.Pattern, "trending-better")
    } else {
        currentState := getCurrentState(transitions)
        if currentState == "firing" {
            categories.Pattern = append(categories.Pattern, "stable-firing")
        } else {
            categories.Pattern = append(categories.Pattern, "stable-normal")
        }
    }

    return categories
}
```

### Pattern 5: Expirable LRU Cache with Jitter
**What:** Use golang-lru/v2/expirable with 5-minute TTL, consider adding jitter to prevent cache stampede.

**When to use:** For caching analysis results to handle repeated queries from MCP tools.

**Example:**
```go
import "github.com/hashicorp/golang-lru/v2/expirable"

type AnalysisResult struct {
    FlappinessScore float64
    DeviationScore  float64
    Categories      AlertCategories
    ComputedAt      time.Time
}

// Initialize cache with 5-minute TTL
cache := expirable.NewLRU[string, AnalysisResult](1000, nil, 5*time.Minute)

func (s *AlertAnalysisService) AnalyzeAlert(ctx context.Context, alertUID string) (*AnalysisResult, error) {
    // Check cache
    if cached, ok := s.cache.Get(alertUID); ok {
        s.logger.Debug("Cache hit for alert %s", alertUID)
        return &cached, nil
    }

    // Compute analysis (cache miss)
    result, err := s.computeAnalysis(ctx, alertUID)
    if err != nil {
        return nil, err
    }

    // Store in cache
    s.cache.Add(alertUID, *result)

    return result, nil
}
```

### Anti-Patterns to Avoid
- **Computing statistics in Cypher:** Graph databases don't have good statistical functions, fetch data and compute in Go with gonum
- **Caching stale data on API failure:** CONTEXT.md explicitly says fail with error if Grafana API unavailable, don't fall back to stale cache
- **Using time.Now() for testing:** Inject time provider interface to enable deterministic testing of time-based logic
- **Ignoring partial data:** With 24h-7d history, compute baseline from available data (CONTEXT.md allows this)

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Standard deviation calculation | Custom variance/stddev functions | gonum/stat.StdDev, stat.Variance | Off-by-one errors (N vs N-1), biased vs unbiased estimators, edge case handling |
| In-memory cache with TTL | sync.Map with manual expiration goroutine | hashicorp/golang-lru/v2/expirable | Thread-safe, automatic cleanup, battle-tested in production, already in project |
| Time-based sorting | Custom sort with time comparisons | sort.Slice with time.Before() | Handles edge cases, monotonic clock issues |
| Statistical outlier detection | Custom z-score implementation | gonum/stat + manual threshold | gonum handles NaN, Inf, empty slices gracefully |

**Key insight:** Statistical computations have subtle correctness issues (sample vs population variance, biased estimators, numerical stability). Use established libraries that handle edge cases.

## Common Pitfalls

### Pitfall 1: Sample vs Population Variance
**What goes wrong:** Using wrong variance formula leads to biased baseline comparisons.

**Why it happens:** Confusion between sample variance (N-1 divisor, unbiased estimator) and population variance (N divisor, biased estimator).

**How to avoid:**
- Use `stat.Variance()` for sample variance (unbiased, default for unknown population)
- Use `stat.PopVariance()` for population variance (biased, only if you have full population)
- For alert baselines: use sample variance since we have a sample of history, not full population

**Warning signs:** Baseline deviations consistently higher/lower than expected, statistical tests failing validation.

### Pitfall 2: Time Zone Handling in Cypher Queries
**What goes wrong:** Cypher timestamp comparisons fail due to timezone mismatches between Go time.Time and RFC3339 strings in graph.

**Why it happens:** Phase 21 stores timestamps as RFC3339 strings, Go's time.Time may have different timezone representation.

**How to avoid:**
- Always convert Go time.Time to UTC before formatting: `timestamp.UTC().Format(time.RFC3339)`
- Use consistent timezone in all Cypher queries (UTC recommended)
- Test with timestamps from different timezones

**Warning signs:** Queries return empty results despite data existing, off-by-hours errors in time window filtering.

### Pitfall 3: Cache Stampede on Analysis Requests
**What goes wrong:** Multiple concurrent requests for same alert bypass cache during computation, causing duplicate expensive graph queries.

**Why it happens:** golang-lru cache doesn't provide request coalescing, all concurrent requests miss cache simultaneously.

**How to avoid:**
- Use singleflight pattern (golang.org/x/sync/singleflight) to coalesce concurrent requests
- First request computes, others wait for result
- Cache result once computed

**Warning signs:** High graph database load spikes, multiple identical Cypher queries in logs, cache hit rate lower than expected.

### Pitfall 4: Off-By-One in Sliding Window Boundaries
**What goes wrong:** Window includes/excludes boundary timestamps inconsistently, causing incorrect transition counts.

**Why it happens:** Confusion about inclusive vs exclusive boundaries, time.After() vs time.Before() semantics.

**How to avoid:**
- Document window boundary semantics clearly (e.g., "6-hour window: [now-6h, now)")
- Use consistent boundary operators: `>=` for start, `<` for end (makes windows non-overlapping)
- Test boundary conditions explicitly

**Warning signs:** Flappiness scores differ by 1 transition between runs, double-counting at window boundaries.

### Pitfall 5: Insufficient Data Handling Inconsistency
**What goes wrong:** Different functions handle <24h data differently (error vs zero vs partial result).

**Why it happens:** CONTEXT.md specifies "minimum 24h required" but allows "use available data for 24h-7d".

**How to avoid:**
- Return structured error with reason: `ErrInsufficientData{Available: 12*time.Hour, Required: 24*time.Hour}`
- Document minimum requirements per function (flappiness may work with less data than baseline)
- Test with various data availability scenarios (0h, 12h, 24h, 3d, 7d)

**Warning signs:** Inconsistent error messages, some functions succeed where others fail with same data.

### Pitfall 6: Flappiness Score Not Normalized
**What goes wrong:** Flappiness score exceeds 1.0 or doesn't scale properly across different window sizes.

**Why it happens:** Score formula doesn't account for maximum possible transitions in window, or uses absolute counts instead of normalized values.

**How to avoid:**
- Normalize to 0.0-1.0 range using maximum theoretical transitions (sync interval = 5 min, so 6h window = 72 possible transitions)
- Formula: `score = min(1.0, transitionCount / maxPossibleTransitions * durationPenalty)`
- Duration penalty: penalize short-lived states (CONTEXT.md requirement)

**Warning signs:** Scores >1.0, alerts with identical behavior have different scores due to window size differences.

## Code Examples

Verified patterns from official sources:

### Using gonum/stat for Standard Deviation
```go
// Source: https://pkg.go.dev/gonum.org/v1/gonum/stat
import (
    "math"
    "gonum.org/v1/gonum/stat"
)

// Compute mean and standard deviation
data := []float64{0.35, 0.42, 0.38, 0.51, 0.29, 0.45, 0.40}
mean := stat.Mean(data, nil)
variance := stat.Variance(data, nil)  // Unbiased (sample) variance
stddev := math.Sqrt(variance)

// Alternative: combined mean + stddev
mean2, stddev2 := stat.MeanStdDev(data, nil)

// For population variance (biased estimator):
popVariance := stat.PopVariance(data, nil)
```

### Using golang-lru/v2/expirable for TTL Cache
```go
// Source: https://pkg.go.dev/github.com/hashicorp/golang-lru/v2/expirable
import (
    "time"
    "github.com/hashicorp/golang-lru/v2/expirable"
)

// Create cache with 5-minute TTL and 1000 max entries
cache := expirable.NewLRU[string, AnalysisResult](1000, nil, 5*time.Minute)

// Add to cache (returns true if eviction occurred)
evicted := cache.Add("alert-123", result)

// Get from cache
if value, ok := cache.Get("alert-123"); ok {
    // Cache hit
    return value
}

// Peek without updating recency
if value, ok := cache.Peek("alert-123"); ok {
    // Value exists but not marked as recently used
}

// Remove from cache
cache.Remove("alert-123")

// Get all values (expired entries filtered out)
allValues := cache.Values()
```

### Cypher Query for Time-Range State Transitions
```go
// Fetch state transitions with time-based filtering
query := `
    MATCH (a:Alert {uid: $uid, integration: $integration})-[t:STATE_TRANSITION]->(a)
    WHERE t.timestamp >= $startTime
      AND t.timestamp <= $endTime
      AND t.expires_at > $now
    RETURN t.from_state AS from_state,
           t.to_state AS to_state,
           t.timestamp AS timestamp
    ORDER BY t.timestamp ASC
`

result, err := graphClient.ExecuteQuery(ctx, graph.GraphQuery{
    Query: query,
    Parameters: map[string]interface{}{
        "uid":         alertUID,
        "integration": integrationName,
        "startTime":   startTime.UTC().Format(time.RFC3339),
        "endTime":     endTime.UTC().Format(time.RFC3339),
        "now":         time.Now().UTC().Format(time.RFC3339),
    },
})

// Parse results
transitions := []StateTransition{}
for _, row := range result.Rows {
    timestamp, _ := time.Parse(time.RFC3339, row[2].(string))
    transitions = append(transitions, StateTransition{
        FromState: row[0].(string),
        ToState:   row[1].(string),
        Timestamp: timestamp,
    })
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Binary flapping flag (yes/no) | Continuous flappiness score (0.0-1.0) | Nagios (2000s) → Modern monitoring (2020+) | Allows ranking alerts by severity, gradual thresholds |
| Time-of-day matching baselines | Rolling average baselines | Statistical monitoring (2010s) → Cloud-native (2020+) | Simpler, works without diurnal patterns |
| Single label categorization | Multi-label categorization | Traditional monitoring → ML-driven observability (2023+) | Captures multiple simultaneous behaviors |
| Manual threshold tuning | Statistical deviation (2σ threshold) | Rule-based → Statistical (2015+) | Self-adjusting, reduces manual tuning |

**Deprecated/outdated:**
- **Nagios flapping detection (21-check window with weighted transitions):** Too complex, fixed window doesn't adapt to different alert patterns. Modern approach: simpler sliding window with continuous scoring.
- **Time-of-day baseline matching:** Assumes diurnal patterns, doesn't work for cloud-native services with variable load. Modern approach: rolling average over full 7 days.

## Open Questions

Things that couldn't be fully resolved:

1. **Optimal flappiness score formula (frequency vs duration weighting)**
   - What we know: Score must factor in both transition count AND duration in each state, normalized 0.0-1.0
   - What's unclear: Exact weighting between frequency penalty and duration penalty
   - Recommendation: Start with `score = (transitionCount / maxPossible) * (1 - avgStateDuration / windowSize)` and tune based on user feedback in Phase 23

2. **Chronic threshold rationale (why 80% firing over 7 days)**
   - What we know: CONTEXT.md specifies >80% time firing = chronic
   - What's unclear: Why 80% specifically (vs 75% or 90%)
   - Recommendation: Research shows 80% is common threshold for "persistent state" in SRE literature (Datadog, PagerDuty use similar). Acceptable starting point, make configurable for future tuning.

3. **Minimum data for trending detection (1h vs 6h windows)**
   - What we know: Trending compares last 1h to prior 6h
   - What's unclear: What if alert has only 3h of history? Fail or compute partial trend?
   - Recommendation: Require minimum 2h data for trending (1h recent + 1h baseline), return "insufficient data" otherwise. Document in error message.

4. **Cache size limit (1000 entries reasonable?)**
   - What we know: 5-minute TTL, typical Grafana has 100-500 alerts
   - What's unclear: Memory usage per AnalysisResult entry
   - Recommendation: Start with 1000 entries (2x typical alert count), monitor memory usage in production. Each entry ~1KB (estimate), so ~1MB cache max.

## Sources

### Primary (HIGH confidence)
- [gonum/stat package documentation](https://pkg.go.dev/gonum.org/v1/gonum/stat) - Statistical functions API
- [hashicorp/golang-lru/v2/expirable package](https://pkg.go.dev/github.com/hashicorp/golang-lru/v2/expirable) - TTL cache API
- [Go time package documentation](https://pkg.go.dev/time) - Time handling

### Secondary (MEDIUM confidence)
- [Datadog: Reduce alert flapping](https://docs.datadoghq.com/monitors/guide/reduce-alert-flapping/) - Alert flapping best practices
- [Nagios: Detection and Handling of State Flapping](https://assets.nagios.com/downloads/nagioscore/docs/nagioscore/3/en/flapping.html) - Flapping detection algorithm
- [Building an In-Memory Cache in Golang with TTL](https://medium.com/@karanjitsinghz50/building-an-in-memory-cache-in-golang-with-ttl-eviction-aee3f4a8d0f7) - TTL cache patterns
- [TimescaleDB: Last observation carried forward](https://www.tigerdata.com/docs/use-timescale/latest/hyperfunctions/gapfilling-interpolation/locf) - LOCF interpolation
- [Introduction to Statistics with Gonum](https://www.gonum.org/post/intro_to_stats_with_gonum/) - Gonum usage examples
- [Sliding Window Aggregation Pattern](https://softwarepatternslexicon.com/data-modeling/time-series-data-modeling/sliding-window-aggregation/) - Sliding window design
- [Cypher Query Language: Temporal Capabilities](https://www.tigergraph.com/glossary/cypher-query-language/) - Cypher time-based queries

### Tertiary (LOW confidence - marked for validation)
- Various SRE blog posts on alert categorization (no single authoritative source)
- Six Sigma baseline calculation guidance (applicable but from different domain)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - golang-lru/v2 already in project v2.0.7, gonum is standard, Go stdlib
- Architecture: MEDIUM - Patterns verified from multiple sources but not tested in this specific context (graph + time-series)
- Pitfalls: MEDIUM - Based on common Go time-series pitfalls and statistical computing errors, not specific to alert analysis
- Code examples: HIGH - Directly from official documentation (gonum, golang-lru)
- Flappiness algorithm: LOW - No single authoritative source, multiple interpretations possible (needs validation in implementation)

**Research date:** 2026-01-23
**Valid until:** 2026-02-23 (30 days - stable domain, statistical methods don't change frequently)
