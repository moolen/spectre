package grafana

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// AggregatedAnomaly represents a rolled-up anomaly score for a scope (workload/namespace/cluster).
// Aggregation uses MAX score across child scopes per CONTEXT.md.
type AggregatedAnomaly struct {
	// Scope is the aggregation level: "signal", "workload", "namespace", or "cluster"
	Scope string

	// ScopeKey identifies the entity (e.g., "default/nginx" for workload)
	ScopeKey string

	// Score is the MAX of child anomaly scores (per CONTEXT.md)
	Score float64

	// Confidence is the MIN of child confidences
	Confidence float64

	// SourceCount is the number of contributing signals
	SourceCount int

	// TopSource is the signal with highest score (for debugging/drilldown)
	TopSource string

	// TopSourceQuality is the quality score of TopSource (tiebreaker when scores equal)
	TopSourceQuality float64
}

// AnomalyAggregator computes hierarchical anomaly scores.
// Aggregation follows: signal -> workload -> namespace -> cluster
// Uses MAX aggregation (per CONTEXT.md: "worst signal anomaly").
type AnomalyAggregator struct {
	graphClient     graph.Client
	cache           *AggregationCache
	integrationName string
	logger          *logging.Logger
}

// NewAnomalyAggregator creates a new AnomalyAggregator instance.
func NewAnomalyAggregator(graphClient graph.Client, integrationName string, logger *logging.Logger) *AnomalyAggregator {
	return &AnomalyAggregator{
		graphClient:     graphClient,
		cache:           NewAggregationCache(5*time.Minute, 30*time.Second),
		integrationName: integrationName,
		logger:          logger,
	}
}

// AggregateWorkloadAnomaly computes the aggregated anomaly score for a workload.
//
// Process:
// 1. Check cache first (5-minute TTL per CONTEXT.md)
// 2. Query graph for SignalAnchors in workload with their baselines
// 3. For each signal: compute anomaly score (skip if InsufficientSamplesError)
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

	// Query graph for signals in this workload with baselines and alert states
	signals, err := a.getWorkloadSignals(ctx, namespace, workloadName)
	if err != nil {
		return nil, err
	}

	if len(signals) == 0 {
		return nil, nil // No signals for workload
	}

	// Aggregate anomaly scores
	result := a.aggregateSignals(signals, "workload", namespace+"/"+workloadName)

	// Cache result with jitter TTL
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

	// Query for all workloads in namespace
	workloads, err := a.getNamespaceWorkloads(ctx, namespace)
	if err != nil {
		return nil, err
	}

	if len(workloads) == 0 {
		return nil, nil // No workloads in namespace
	}

	// Aggregate across workloads
	var aggregatedResult *AggregatedAnomaly
	var topScore float64
	var minConfidence float64 = 1.0
	var totalSources int
	var topSource string
	var topQuality float64

	for _, workload := range workloads {
		workloadResult, err := a.AggregateWorkloadAnomaly(ctx, namespace, workload)
		if err != nil {
			a.logger.Debug("Error aggregating workload %s/%s: %v", namespace, workload, err)
			continue
		}
		if workloadResult == nil {
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

	aggregatedResult = &AggregatedAnomaly{
		Scope:            "namespace",
		ScopeKey:         namespace,
		Score:            topScore,
		Confidence:       minConfidence,
		SourceCount:      totalSources,
		TopSource:        topSource,
		TopSourceQuality: topQuality,
	}

	// Cache result
	a.cache.Set(cacheKey, aggregatedResult)

	return aggregatedResult, nil
}

// AggregateClusterAnomaly computes the aggregated anomaly score for the entire cluster.
//
// Process:
// 1. Query all namespaces
// 2. For each namespace: call AggregateNamespaceAnomaly
// 3. Aggregate: MAX score across namespaces
func (a *AnomalyAggregator) AggregateClusterAnomaly(ctx context.Context) (*AggregatedAnomaly, error) {
	cacheKey := "cluster:" + a.integrationName

	// Check cache first
	if cached := a.cache.Get(cacheKey); cached != nil {
		return cached, nil
	}

	// Query for all namespaces with signals
	namespaces, err := a.getClusterNamespaces(ctx)
	if err != nil {
		return nil, err
	}

	if len(namespaces) == 0 {
		return nil, nil // No namespaces with signals
	}

	// Aggregate across namespaces
	var topScore float64
	var minConfidence float64 = 1.0
	var totalSources int
	var topSource string
	var topQuality float64

	for _, ns := range namespaces {
		nsResult, err := a.AggregateNamespaceAnomaly(ctx, ns)
		if err != nil {
			a.logger.Debug("Error aggregating namespace %s: %v", ns, err)
			continue
		}
		if nsResult == nil {
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
		ScopeKey:         a.integrationName,
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

// signalWithBaseline holds signal data plus baseline and alert state for scoring.
type signalWithBaseline struct {
	MetricName   string
	QualityScore float64
	CurrentValue float64
	AlertState   string
	Baseline     *SignalBaseline
}

// getWorkloadSignals retrieves signals for a workload with their baselines and current values.
func (a *AnomalyAggregator) getWorkloadSignals(ctx context.Context, namespace, workloadName string) ([]signalWithBaseline, error) {
	query := `
		MATCH (s:SignalAnchor {
			workload_namespace: $namespace,
			workload_name: $workload_name,
			integration: $integration
		})
		WHERE s.expires_at > $now
		OPTIONAL MATCH (s)-[:HAS_BASELINE]->(b:SignalBaseline)
		RETURN s.metric_name AS metric_name,
		       s.quality_score AS quality_score,
		       b.mean AS mean,
		       b.std_dev AS std_dev,
		       b.min AS min,
		       b.max AS max,
		       b.p50 AS p50,
		       b.p90 AS p90,
		       b.p99 AS p99,
		       b.sample_count AS sample_count
	`

	now := time.Now().Unix()
	result, err := a.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"namespace":     namespace,
			"workload_name": workloadName,
			"integration":   a.integrationName,
			"now":           now,
		},
	})
	if err != nil {
		return nil, err
	}

	// Map column names to indices
	colIdx := make(map[string]int)
	for i, col := range result.Columns {
		colIdx[col] = i
	}

	var signals []signalWithBaseline
	for _, row := range result.Rows {
		signal := signalWithBaseline{}

		// Extract metric_name
		if idx, ok := colIdx["metric_name"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				signal.MetricName = v
			}
		}

		// Extract quality_score
		if idx, ok := colIdx["quality_score"]; ok && idx < len(row) {
			signal.QualityScore = parseFloat64(row[idx])
		}

		// Extract baseline if present
		if idx, ok := colIdx["sample_count"]; ok && idx < len(row) && row[idx] != nil {
			signal.Baseline = &SignalBaseline{
				SampleCount: parseInt(row[colIdx["sample_count"]]),
			}
			if idx, ok := colIdx["mean"]; ok && idx < len(row) {
				signal.Baseline.Mean = parseFloat64(row[idx])
			}
			if idx, ok := colIdx["std_dev"]; ok && idx < len(row) {
				signal.Baseline.StdDev = parseFloat64(row[idx])
			}
			if idx, ok := colIdx["min"]; ok && idx < len(row) {
				signal.Baseline.Min = parseFloat64(row[idx])
			}
			if idx, ok := colIdx["max"]; ok && idx < len(row) {
				signal.Baseline.Max = parseFloat64(row[idx])
			}
			if idx, ok := colIdx["p50"]; ok && idx < len(row) {
				signal.Baseline.P50 = parseFloat64(row[idx])
			}
			if idx, ok := colIdx["p90"]; ok && idx < len(row) {
				signal.Baseline.P90 = parseFloat64(row[idx])
			}
			if idx, ok := colIdx["p99"]; ok && idx < len(row) {
				signal.Baseline.P99 = parseFloat64(row[idx])
			}
		}

		// For now, use baseline mean as current value proxy
		// In production, this would come from recent Grafana query
		if signal.Baseline != nil {
			signal.CurrentValue = signal.Baseline.Mean
		}

		signals = append(signals, signal)
	}

	return signals, nil
}

// aggregateSignals computes aggregated anomaly from a list of signals.
func (a *AnomalyAggregator) aggregateSignals(signals []signalWithBaseline, scope, scopeKey string) *AggregatedAnomaly {
	var topScore float64
	var minConfidence float64 = 1.0
	var validCount int
	var topSource string
	var topQuality float64

	for _, signal := range signals {
		// Skip signals without baselines (cold start)
		if signal.Baseline == nil {
			continue
		}

		// Compute anomaly score
		score, err := ComputeAnomalyScore(signal.CurrentValue, *signal.Baseline, signal.QualityScore)
		if err != nil {
			// InsufficientSamplesError - skip this signal
			a.logger.Debug("Skipping signal %s: %v", signal.MetricName, err)
			continue
		}

		// Apply alert override if firing
		if signal.AlertState == "firing" {
			score = ApplyAlertOverride(score, signal.AlertState)
		}

		validCount++

		// MAX score aggregation with quality tiebreaker
		if score.Score > topScore || (score.Score == topScore && signal.QualityScore > topQuality) {
			topScore = score.Score
			topSource = signal.MetricName
			topQuality = signal.QualityScore
		}

		// MIN confidence
		if score.Confidence < minConfidence {
			minConfidence = score.Confidence
		}
	}

	if validCount == 0 {
		return nil // No valid signals
	}

	return &AggregatedAnomaly{
		Scope:            scope,
		ScopeKey:         scopeKey,
		Score:            topScore,
		Confidence:       minConfidence,
		SourceCount:      validCount,
		TopSource:        topSource,
		TopSourceQuality: topQuality,
	}
}

// getNamespaceWorkloads retrieves distinct workload names in a namespace.
func (a *AnomalyAggregator) getNamespaceWorkloads(ctx context.Context, namespace string) ([]string, error) {
	query := `
		MATCH (s:SignalAnchor {
			workload_namespace: $namespace,
			integration: $integration
		})
		WHERE s.expires_at > $now AND s.workload_name <> ''
		RETURN DISTINCT s.workload_name AS workload_name
	`

	now := time.Now().Unix()
	result, err := a.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"namespace":   namespace,
			"integration": a.integrationName,
			"now":         now,
		},
	})
	if err != nil {
		return nil, err
	}

	var workloads []string
	for _, row := range result.Rows {
		if len(row) > 0 {
			if workload, ok := row[0].(string); ok && workload != "" {
				workloads = append(workloads, workload)
			}
		}
	}

	return workloads, nil
}

// getClusterNamespaces retrieves distinct namespaces with signals.
func (a *AnomalyAggregator) getClusterNamespaces(ctx context.Context) ([]string, error) {
	query := `
		MATCH (s:SignalAnchor {integration: $integration})
		WHERE s.expires_at > $now AND s.workload_namespace <> ''
		RETURN DISTINCT s.workload_namespace AS namespace
	`

	now := time.Now().Unix()
	result, err := a.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"integration": a.integrationName,
			"now":         now,
		},
	})
	if err != nil {
		return nil, err
	}

	var namespaces []string
	for _, row := range result.Rows {
		if len(row) > 0 {
			if ns, ok := row[0].(string); ok && ns != "" {
				namespaces = append(namespaces, ns)
			}
		}
	}

	return namespaces, nil
}

// AggregationCache provides TTL-based caching with jitter for anomaly aggregations.
// Uses sync.Map for thread safety.
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
// Jitter prevents thundering herd on cache expiration.
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
		// Expired - delete and return nil
		c.data.Delete(key)
	}
	return nil
}

// Set stores a result with TTL + random jitter.
func (c *AggregationCache) Set(key string, result *AggregatedAnomaly) {
	// Add random jitter to prevent stampede
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
	c.data.Range(func(key, value interface{}) bool {
		c.data.Delete(key)
		return true
	})
}
