// Package observatory provides core types and interfaces for the Observatory
// anomaly detection system. Observatory aggregates signals from multiple
// monitoring providers (Grafana, Datadog, etc.) and computes anomaly scores
// using statistical analysis.
package observatory

import (
	"fmt"
	"math"
	"sort"

	"gonum.org/v1/gonum/stat"
)

// SignalRole represents the operational role of a metric in observability.
// Based on Google's Four Golden Signals (Latency, Traffic, Errors, Saturation)
// plus observability-specific extensions (Availability, Novelty).
type SignalRole string

const (
	// SignalAvailability indicates uptime/health metrics (up, kube_pod_status_phase)
	SignalAvailability SignalRole = "Availability"

	// SignalLatency indicates response time/duration metrics (histogram_quantile, *_duration_*)
	SignalLatency SignalRole = "Latency"

	// SignalErrors indicates failure/error rate metrics (*_error_*, *_failed_*)
	SignalErrors SignalRole = "Errors"

	// SignalTraffic indicates throughput/request rate metrics (rate(*_total), *_count)
	SignalTraffic SignalRole = "Traffic"

	// SignalSaturation indicates resource utilization metrics (cpu, memory, disk)
	SignalSaturation SignalRole = "Saturation"

	// SignalNovelty indicates change events/deployments (pod restarts, deployments)
	SignalNovelty SignalRole = "Novelty"

	// SignalUnknown indicates metrics that could not be classified
	SignalUnknown SignalRole = "Unknown"
)

// SignalAnchor links a metric to a classified signal role and K8s workload.
// This is the core entity that Observatory uses to track what metrics are
// being monitored for which workloads.
//
// Deduplication: Same metric+workload from multiple sources → highest quality wins
// Composite key: metric_name + workload_namespace + workload_name
type SignalAnchor struct {
	// MetricName is the metric name (e.g., "container_cpu_usage_seconds_total")
	MetricName string

	// Role is the classified signal role (Availability, Latency, Errors, etc.)
	Role SignalRole

	// Confidence is the classification confidence (0.0-1.0)
	// Layer 1 (hardcoded): 0.95
	// Layer 2 (query structure): 0.85-0.9
	// Layer 3 (metric name patterns): 0.7-0.8
	// Layer 4 (panel title): 0.5
	// Layer 5 (unknown): 0.0
	Confidence float64

	// QualityScore is inherited from source (0.0-1.0)
	// Computed from: freshness, usage, alerting, ownership, completeness
	QualityScore float64

	// WorkloadNamespace is the K8s namespace (may be empty if unlinked)
	WorkloadNamespace string

	// WorkloadName is the K8s workload name (may be empty if unlinked)
	WorkloadName string

	// SourceProvider is the provider name (e.g., "grafana-prod", "datadog-main")
	SourceProvider string

	// SourceRef is a provider-specific reference (dashboard UID, monitor ID, etc.)
	SourceRef string

	// FirstSeen is the Unix timestamp when signal was first ingested
	FirstSeen int64

	// LastSeen is the Unix timestamp when signal was last refreshed
	LastSeen int64

	// ExpiresAt is the Unix timestamp when signal should expire (7-day TTL)
	ExpiresAt int64
}

// SignalBaseline stores rolling statistics for a signal anchor.
// Used for anomaly detection via z-score and percentile comparison.
//
// Statistics are computed from values collected over the rolling window (7 days).
type SignalBaseline struct {
	// Identity fields (composite key matching SignalAnchor)

	// MetricName is the metric name
	MetricName string

	// WorkloadNamespace is the K8s namespace (may be empty if unlinked)
	WorkloadNamespace string

	// WorkloadName is the K8s workload name (may be empty if unlinked)
	WorkloadName string

	// SourceProvider is the provider name for multi-source support
	SourceProvider string

	// Rolling statistics

	// Mean is the arithmetic mean of sample values
	Mean float64

	// StdDev is the sample standard deviation (N-1 formula)
	StdDev float64

	// Median is the 50th percentile (same as P50)
	Median float64

	// P50 is the 50th percentile
	P50 float64

	// P90 is the 90th percentile
	P90 float64

	// P99 is the 99th percentile
	P99 float64

	// Min is the minimum observed value
	Min float64

	// Max is the maximum observed value
	Max float64

	// SampleCount is the number of samples in the baseline
	SampleCount int

	// Window metadata

	// WindowStart is the Unix timestamp of the oldest sample in the window
	WindowStart int64

	// WindowEnd is the Unix timestamp of the newest sample in the window
	WindowEnd int64

	// TTL fields

	// LastUpdated is the Unix timestamp when baseline was last computed
	LastUpdated int64

	// ExpiresAt is the Unix timestamp when baseline expires (7-day TTL)
	ExpiresAt int64
}

// AnomalyScore represents the result of anomaly detection for a signal value.
// Score ranges from 0.0 (normal) to 1.0 (highly anomalous).
// Threshold for anomaly is 0.5.
type AnomalyScore struct {
	// Score is the normalized anomaly score (0.0-1.0).
	// >= 0.5 indicates anomalous.
	Score float64

	// Confidence represents statistical confidence in the score.
	// Calculated as MIN(sampleConfidence, qualityScore).
	Confidence float64

	// Method indicates which scoring method produced the final score.
	// Values: "z-score", "percentile", or "alert-override"
	Method string

	// ZScore is the raw z-score for debugging and analysis.
	ZScore float64
}

// AggregatedAnomaly represents a rolled-up anomaly score for a scope.
// Aggregation uses MAX score across child scopes.
type AggregatedAnomaly struct {
	// Scope is the aggregation level: "signal", "workload", "namespace", or "cluster"
	Scope string

	// ScopeKey identifies the entity (e.g., "default/nginx" for workload)
	ScopeKey string

	// Score is the MAX of child anomaly scores
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

// ClassificationResult represents the output of layered signal classification.
type ClassificationResult struct {
	// Role is the classified signal role
	Role SignalRole

	// Confidence is the classification confidence (0.0-1.0)
	Confidence float64

	// Layer indicates which classification layer matched (1-5)
	Layer int

	// Reason is a human-readable explanation of why this classification was chosen
	Reason string
}

// QueryContext provides context about a metric's query for classification.
// Different metric sources (PromQL, SQL, etc.) can implement this interface.
type QueryContext interface {
	// MetricNames returns all metric names in the query.
	GetMetricNames() []string

	// Aggregations returns all aggregation functions in the query.
	// Examples: "sum", "rate", "histogram_quantile"
	GetAggregations() []string
}

// WorkloadInference represents an inferred K8s workload from metric labels.
type WorkloadInference struct {
	// Namespace is the K8s namespace
	Namespace string

	// WorkloadName is the inferred workload name
	WorkloadName string

	// InferredFrom is the label key used for inference
	InferredFrom string

	// Confidence is the inference confidence (0.7-0.9)
	Confidence float64
}

// RollingStats is the intermediate result of statistical computation.
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

// MinSamplesRequired is the minimum number of samples before baseline is valid.
// Below this threshold, ComputeAnomalyScore returns InsufficientSamplesError.
const MinSamplesRequired = 10

// InsufficientSamplesError indicates baseline cannot be computed due to cold start.
type InsufficientSamplesError struct {
	Available int
	Required  int
}

func (e *InsufficientSamplesError) Error() string {
	return fmt.Sprintf("insufficient samples for baseline: %d available, %d required", e.Available, e.Required)
}

// ComputeRollingStatistics computes rolling statistics from sample values.
// Uses gonum/stat for accurate statistical computation.
func ComputeRollingStatistics(values []float64) *RollingStats {
	n := len(values)

	if n == 0 {
		return &RollingStats{SampleCount: 0}
	}

	mean := stat.Mean(values, nil)
	stdDev := stat.StdDev(values, nil)

	// Copy values for sorting (don't mutate input)
	sorted := make([]float64, n)
	copy(sorted, values)
	sort.Float64s(sorted)

	p50 := stat.Quantile(0.50, stat.Empirical, sorted, nil)
	p90 := stat.Quantile(0.90, stat.Empirical, sorted, nil)
	p99 := stat.Quantile(0.99, stat.Empirical, sorted, nil)

	return &RollingStats{
		Mean:        mean,
		StdDev:      stdDev,
		Median:      p50,
		P50:         p50,
		P90:         p90,
		P99:         p99,
		Min:         sorted[0],
		Max:         sorted[n-1],
		SampleCount: n,
	}
}

// ComputeAnomalyScore computes an anomaly score using hybrid z-score + percentile comparison.
// The final score is MAX of both methods.
//
// Z-Score Method:
//   - zScore = (currentValue - mean) / stddev
//   - Normalized: zScoreNormalized = 1.0 - exp(-|zScore|/2.0)
//
// Percentile Method:
//   - If currentValue > P99: score starts at 0.5, scales up based on distance
//   - If currentValue < Min: score starts at 0.5, scales up based on distance
//
// Returns InsufficientSamplesError if baseline has < MinSamplesRequired samples.
func ComputeAnomalyScore(currentValue float64, baseline SignalBaseline, qualityScore float64) (*AnomalyScore, error) {
	if baseline.SampleCount < MinSamplesRequired {
		return nil, &InsufficientSamplesError{
			Available: baseline.SampleCount,
			Required:  MinSamplesRequired,
		}
	}

	// Compute z-score
	var zScore float64
	if baseline.StdDev > 0 {
		zScore = (currentValue - baseline.Mean) / baseline.StdDev
	}

	// Normalize z-score to 0-1 range
	zScoreNormalized := 1.0 - math.Exp(-math.Abs(zScore)/2.0)

	// Compute percentile score
	var percentileScore float64

	if currentValue > baseline.P99 && baseline.P99 > baseline.P50 {
		excess := currentValue - baseline.P99
		range99 := baseline.P99 - baseline.P50
		percentileScore = math.Min(1.0, 0.5+(excess/range99)*0.5)
	} else if currentValue < baseline.Min {
		deficit := baseline.Min - currentValue
		rangeLow := baseline.P50 - baseline.Min
		if rangeLow > 0 {
			percentileScore = math.Min(1.0, 0.5+(deficit/rangeLow)*0.5)
		} else {
			percentileScore = 0.5
		}
	}

	// Hybrid score = MAX of both methods
	score := math.Max(zScoreNormalized, percentileScore)

	method := "z-score"
	if percentileScore > zScoreNormalized {
		method = "percentile"
	}

	// Compute confidence
	sampleConfidence := math.Min(1.0, 0.5+float64(baseline.SampleCount-MinSamplesRequired)/180.0)
	confidence := math.Min(sampleConfidence, qualityScore)

	return &AnomalyScore{
		Score:      score,
		Confidence: confidence,
		Method:     method,
		ZScore:     zScore,
	}, nil
}

// ApplyAlertOverride modifies an anomaly score based on alert state.
// If alert is firing, the score is overridden to 1.0 with confidence 1.0.
func ApplyAlertOverride(score *AnomalyScore, alertState string) *AnomalyScore {
	if alertState == "firing" {
		return &AnomalyScore{
			Score:      1.0,
			Confidence: 1.0,
			Method:     "alert-override",
			ZScore:     score.ZScore,
		}
	}
	return score
}
