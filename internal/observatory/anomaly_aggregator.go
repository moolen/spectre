package observatory

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

// AnomalyAggregator computes hierarchical anomaly scores using data from the Registry.
// Aggregation follows: signal -> workload -> namespace -> cluster
// Uses MAX aggregation for scores and MIN for confidence.
type AnomalyAggregator struct {
	registry *Registry
	cache    *AggregationCache
}

// NewAnomalyAggregator creates a new AnomalyAggregator instance.
func NewAnomalyAggregator(registry *Registry) *AnomalyAggregator {
	return &AnomalyAggregator{
		registry: registry,
		cache:    NewAggregationCache(5*time.Minute, 30*time.Second),
	}
}

// AggregateWorkloadAnomaly computes the aggregated anomaly score for a workload.
//
// Process:
// 1. Check cache first (5-minute TTL)
// 2. Query registry for SignalAnchors in workload
// 3. For each signal: fetch baseline and current value, compute anomaly score
// 4. Check alert state for firing override
// 5. Aggregate: Score = MAX, Confidence = MIN, TopSource = signal with MAX score
// 6. Cache result with jitter TTL
//
// Returns nil if no valid signals for workload.
func (a *AnomalyAggregator) AggregateWorkloadAnomaly(ctx context.Context, namespace, workloadName string) (*AggregatedAnomaly, error) {
	cacheKey := "workload:" + namespace + "/" + workloadName

	// Check cache first
	if cached := a.cache.Get(cacheKey); cached != nil {
		return cached, nil
	}

	// Query registry for signals in this workload
	signals, err := a.registry.ListAllSignalAnchors(ctx, SignalListOptions{
		Namespace:    namespace,
		WorkloadName: workloadName,
	})
	if err != nil {
		return nil, err
	}

	if len(signals) == 0 {
		return nil, nil // No signals for workload
	}

	// Compute anomaly scores for each signal
	var scoredSignals []scoredSignal
	for _, signal := range signals {
		// Get baseline
		baseline, err := a.registry.GetSignalBaseline(ctx, signal.MetricName, namespace, workloadName)
		if err != nil || baseline == nil {
			continue // Skip signals without baselines
		}

		// Get current value
		currentValue, found, err := a.registry.GetSignalCurrentValue(ctx, signal.MetricName, namespace, workloadName)
		if err != nil {
			continue
		}
		if !found {
			currentValue = baseline.Mean // Fallback to mean
		}

		// Compute anomaly score
		score, err := ComputeAnomalyScore(currentValue, *baseline, signal.QualityScore)
		if err != nil {
			continue // Skip signals with insufficient samples
		}

		// Check alert state for override
		alertState, _ := a.registry.GetSignalAlertState(ctx, signal.MetricName, namespace, workloadName)
		if alertState == "firing" {
			score = ApplyAlertOverride(score, alertState)
		}

		scoredSignals = append(scoredSignals, scoredSignal{
			metricName:   signal.MetricName,
			qualityScore: signal.QualityScore,
			score:        score,
		})
	}

	if len(scoredSignals) == 0 {
		return nil, nil // No valid signals
	}

	// Aggregate scores
	result := a.aggregateScores(scoredSignals, "workload", namespace+"/"+workloadName)

	// Cache result
	a.cache.Set(cacheKey, result)

	return result, nil
}

// AggregateNamespaceAnomaly computes the aggregated anomaly score for a namespace.
//
// Process:
// 1. Query all workloads in namespace
// 2. For each workload: call AggregateWorkloadAnomaly
// 3. Aggregate: MAX score across workloads, MIN confidence
func (a *AnomalyAggregator) AggregateNamespaceAnomaly(ctx context.Context, namespace string) (*AggregatedAnomaly, error) {
	cacheKey := "namespace:" + namespace

	// Check cache first
	if cached := a.cache.Get(cacheKey); cached != nil {
		return cached, nil
	}

	// Get all signals in namespace to find workloads
	signals, err := a.registry.ListAllSignalAnchors(ctx, SignalListOptions{
		Namespace: namespace,
	})
	if err != nil {
		return nil, err
	}

	// Extract unique workload names
	workloadSet := make(map[string]bool)
	for _, signal := range signals {
		if signal.WorkloadName != "" {
			workloadSet[signal.WorkloadName] = true
		}
	}

	if len(workloadSet) == 0 {
		return nil, nil // No workloads in namespace
	}

	// Aggregate across workloads
	var topScore float64
	var minConfidence float64 = 1.0
	var totalSources int
	var topSource string
	var topQuality float64

	for workload := range workloadSet {
		workloadResult, err := a.AggregateWorkloadAnomaly(ctx, namespace, workload)
		if err != nil || workloadResult == nil {
			continue
		}

		totalSources += workloadResult.SourceCount

		// MAX score aggregation
		if workloadResult.Score > topScore || (workloadResult.Score == topScore && workloadResult.TopSourceQuality > topQuality) {
			topScore = workloadResult.Score
			topSource = workloadResult.TopSource
			topQuality = workloadResult.TopSourceQuality
		}

		// MIN confidence
		if workloadResult.Confidence < minConfidence {
			minConfidence = workloadResult.Confidence
		}
	}

	if totalSources == 0 {
		return nil, nil // No signals found
	}

	result := &AggregatedAnomaly{
		Scope:            "namespace",
		ScopeKey:         namespace,
		Score:            topScore,
		Confidence:       minConfidence,
		SourceCount:      totalSources,
		TopSource:        topSource,
		TopSourceQuality: topQuality,
	}

	// Cache result
	a.cache.Set(cacheKey, result)

	return result, nil
}

// AggregateClusterAnomaly computes the aggregated anomaly score for the entire cluster.
func (a *AnomalyAggregator) AggregateClusterAnomaly(ctx context.Context) (*AggregatedAnomaly, error) {
	cacheKey := "cluster:all"

	// Check cache first
	if cached := a.cache.Get(cacheKey); cached != nil {
		return cached, nil
	}

	// Get all signals to find namespaces
	signals, err := a.registry.ListAllSignalAnchors(ctx, SignalListOptions{})
	if err != nil {
		return nil, err
	}

	// Extract unique namespaces
	nsSet := make(map[string]bool)
	for _, signal := range signals {
		if signal.WorkloadNamespace != "" {
			nsSet[signal.WorkloadNamespace] = true
		}
	}

	if len(nsSet) == 0 {
		return nil, nil // No namespaces with signals
	}

	// Aggregate across namespaces
	var topScore float64
	var minConfidence float64 = 1.0
	var totalSources int
	var topSource string
	var topQuality float64

	for ns := range nsSet {
		nsResult, err := a.AggregateNamespaceAnomaly(ctx, ns)
		if err != nil || nsResult == nil {
			continue
		}

		totalSources += nsResult.SourceCount

		// MAX score aggregation
		if nsResult.Score > topScore || (nsResult.Score == topScore && nsResult.TopSourceQuality > topQuality) {
			topScore = nsResult.Score
			topSource = nsResult.TopSource
			topQuality = nsResult.TopSourceQuality
		}

		// MIN confidence
		if nsResult.Confidence < minConfidence {
			minConfidence = nsResult.Confidence
		}
	}

	if totalSources == 0 {
		return nil, nil // No signals found
	}

	result := &AggregatedAnomaly{
		Scope:            "cluster",
		ScopeKey:         "all",
		Score:            topScore,
		Confidence:       minConfidence,
		SourceCount:      totalSources,
		TopSource:        topSource,
		TopSourceQuality: topQuality,
	}

	// Cache result
	a.cache.Set(cacheKey, result)

	return result, nil
}

// ClearCache clears all cached aggregations.
func (a *AnomalyAggregator) ClearCache() {
	a.cache.Clear()
}

// scoredSignal holds a signal with its computed anomaly score.
type scoredSignal struct {
	metricName   string
	qualityScore float64
	score        *AnomalyScore
}

// aggregateScores computes aggregated anomaly from a list of scored signals.
func (a *AnomalyAggregator) aggregateScores(signals []scoredSignal, scope, scopeKey string) *AggregatedAnomaly {
	var topScore float64
	var minConfidence float64 = 1.0
	var topSource string
	var topQuality float64

	for _, signal := range signals {
		// MAX score aggregation with quality tiebreaker
		if signal.score.Score > topScore || (signal.score.Score == topScore && signal.qualityScore > topQuality) {
			topScore = signal.score.Score
			topSource = signal.metricName
			topQuality = signal.qualityScore
		}

		// MIN confidence
		if signal.score.Confidence < minConfidence {
			minConfidence = signal.score.Confidence
		}
	}

	return &AggregatedAnomaly{
		Scope:            scope,
		ScopeKey:         scopeKey,
		Score:            topScore,
		Confidence:       minConfidence,
		SourceCount:      len(signals),
		TopSource:        topSource,
		TopSourceQuality: topQuality,
	}
}

// AggregationCache provides TTL-based caching with jitter for anomaly aggregations.
type AggregationCache struct {
	data      sync.Map
	ttl       time.Duration
	jitterMax time.Duration
}

type cacheEntry struct {
	result    *AggregatedAnomaly
	expiresAt time.Time
}

// NewAggregationCache creates a new cache with TTL and jitter.
func NewAggregationCache(ttl, jitterMax time.Duration) *AggregationCache {
	return &AggregationCache{
		ttl:       ttl,
		jitterMax: jitterMax,
	}
}

// Get retrieves a cached result if not expired.
func (c *AggregationCache) Get(key string) *AggregatedAnomaly {
	if value, ok := c.data.Load(key); ok {
		entry := value.(*cacheEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.result
		}
		c.data.Delete(key)
	}
	return nil
}

// Set stores a result with TTL + random jitter.
func (c *AggregationCache) Set(key string, result *AggregatedAnomaly) {
	var jitter time.Duration
	if c.jitterMax > 0 {
		jitter = time.Duration(rand.Int63n(int64(c.jitterMax)))
	}
	expiresAt := time.Now().Add(c.ttl + jitter)

	c.data.Store(key, &cacheEntry{
		result:    result,
		expiresAt: expiresAt,
	})
}

// Clear removes all entries from the cache.
func (c *AggregationCache) Clear() {
	c.data.Range(func(key, value any) bool {
		c.data.Delete(key)
		return true
	})
}
