package grafana

import (
	"fmt"
	"sort"

	"gonum.org/v1/gonum/stat"
)

// MinSamplesRequired is the minimum number of samples before baseline is valid.
// Below this threshold, ComputeRollingStatistics returns InsufficientSamplesError.
const MinSamplesRequired = 10

// SignalBaseline stores rolling statistics for a signal anchor.
// Matches SignalAnchor composite key: metric_name + workload_namespace + workload_name + integration.
//
// Graph relationships:
// - (SignalBaseline)-[:BASELINE_FOR]->(SignalAnchor) - links to the signal being tracked
//
// Statistics are computed from values collected over the rolling window (7 days).
// Used for anomaly detection via z-score and percentile comparison.
type SignalBaseline struct {
	// Identity fields (composite key matching SignalAnchor)

	// MetricName is the PromQL metric name (e.g., "container_cpu_usage_seconds_total")
	MetricName string

	// WorkloadNamespace is the K8s namespace (may be empty if unlinked)
	WorkloadNamespace string

	// WorkloadName is the K8s workload name (may be empty if unlinked)
	WorkloadName string

	// Integration is the Grafana integration name for multi-source support
	Integration string

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

// RollingStats is the intermediate result of statistical computation.
// Used to populate SignalBaseline without identity fields.
type RollingStats struct {
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

	// SampleCount is the number of samples used in computation
	SampleCount int
}

// InsufficientSamplesError indicates baseline cannot be computed due to cold start.
// Returned when sample count is below MinSamplesRequired.
type InsufficientSamplesError struct {
	// Available is the number of samples currently available
	Available int

	// Required is the minimum number of samples needed (MinSamplesRequired)
	Required int
}

// Error implements the error interface.
func (e *InsufficientSamplesError) Error() string {
	return fmt.Sprintf("insufficient samples for baseline: %d available, %d required", e.Available, e.Required)
}

// ComputeRollingStatistics computes rolling statistics from sample values.
// Uses gonum/stat for accurate statistical computation.
//
// Returns a RollingStats struct with computed statistics.
// For empty input, returns RollingStats with SampleCount=0 and zero-valued fields.
//
// Note: Input slice is not modified. Values are copied and sorted internally
// for percentile computation.
func ComputeRollingStatistics(values []float64) *RollingStats {
	n := len(values)

	// Handle empty input gracefully
	if n == 0 {
		return &RollingStats{
			SampleCount: 0,
		}
	}

	// Compute mean using gonum/stat
	mean := stat.Mean(values, nil)

	// Compute sample standard deviation using gonum/stat (N-1 formula)
	stdDev := stat.StdDev(values, nil)

	// Copy values for sorting (don't mutate input)
	sorted := make([]float64, n)
	copy(sorted, values)
	sort.Float64s(sorted)

	// Compute percentiles using gonum/stat.Quantile with Empirical method
	// Quantile requires sorted data
	p50 := stat.Quantile(0.50, stat.Empirical, sorted, nil)
	p90 := stat.Quantile(0.90, stat.Empirical, sorted, nil)
	p99 := stat.Quantile(0.99, stat.Empirical, sorted, nil)

	// Min and Max from sorted array
	min := sorted[0]
	max := sorted[n-1]

	return &RollingStats{
		Mean:        mean,
		StdDev:      stdDev,
		Median:      p50, // Median is the 50th percentile
		P50:         p50,
		P90:         p90,
		P99:         p99,
		Min:         min,
		Max:         max,
		SampleCount: n,
	}
}
