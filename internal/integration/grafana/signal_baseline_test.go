package grafana

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeRollingStatistics_BasicValues(t *testing.T) {
	// Input: [1, 2, 3, 4, 5]
	values := []float64{1, 2, 3, 4, 5}

	stats := ComputeRollingStatistics(values)

	assert.NotNil(t, stats)
	assert.Equal(t, 5, stats.SampleCount)
	assert.Equal(t, 3.0, stats.Mean)
	assert.Equal(t, 1.0, stats.Min)
	assert.Equal(t, 5.0, stats.Max)

	// Median and P50 should be equal
	assert.Equal(t, stats.P50, stats.Median)

	// For [1,2,3,4,5], median is 3
	assert.Equal(t, 3.0, stats.Median)
}

func TestComputeRollingStatistics_EmptyInput(t *testing.T) {
	// Input: empty slice
	values := []float64{}

	stats := ComputeRollingStatistics(values)

	assert.NotNil(t, stats)
	assert.Equal(t, 0, stats.SampleCount)
	assert.Equal(t, 0.0, stats.Mean)
	assert.Equal(t, 0.0, stats.StdDev)
	assert.Equal(t, 0.0, stats.Median)
	assert.Equal(t, 0.0, stats.P50)
	assert.Equal(t, 0.0, stats.P90)
	assert.Equal(t, 0.0, stats.P99)
	assert.Equal(t, 0.0, stats.Min)
	assert.Equal(t, 0.0, stats.Max)
}

func TestComputeRollingStatistics_SingleValue(t *testing.T) {
	// Input: single value
	values := []float64{42.5}

	stats := ComputeRollingStatistics(values)

	assert.NotNil(t, stats)
	assert.Equal(t, 1, stats.SampleCount)
	assert.Equal(t, 42.5, stats.Mean)
	assert.Equal(t, 42.5, stats.Min)
	assert.Equal(t, 42.5, stats.Max)
	assert.Equal(t, 42.5, stats.Median)
	assert.Equal(t, 42.5, stats.P50)
	assert.Equal(t, 42.5, stats.P90)
	assert.Equal(t, 42.5, stats.P99)

	// StdDev of single value should be NaN (due to N-1 formula), but gonum returns 0
	// for single value
	assert.True(t, stats.StdDev == 0.0 || stats.StdDev != stats.StdDev) // 0 or NaN
}

func TestComputeRollingStatistics_Percentiles(t *testing.T) {
	// Input: 100 values from 1-100
	values := make([]float64, 100)
	for i := 0; i < 100; i++ {
		values[i] = float64(i + 1)
	}

	stats := ComputeRollingStatistics(values)

	assert.NotNil(t, stats)
	assert.Equal(t, 100, stats.SampleCount)
	assert.Equal(t, 50.5, stats.Mean) // mean of 1..100

	// Percentile assertions with tolerance for empirical method
	// P50 should be around 50-51
	assert.InDelta(t, 50.0, stats.P50, 2.0, "P50 should be approximately 50")

	// P90 should be around 90-91
	assert.InDelta(t, 90.0, stats.P90, 2.0, "P90 should be approximately 90")

	// P99 should be around 99-100
	assert.InDelta(t, 99.0, stats.P99, 2.0, "P99 should be approximately 99")

	// Min and Max
	assert.Equal(t, 1.0, stats.Min)
	assert.Equal(t, 100.0, stats.Max)
}

func TestComputeRollingStatistics_NoMutateInput(t *testing.T) {
	// Input: unsorted slice
	values := []float64{5, 3, 1, 4, 2}
	original := make([]float64, len(values))
	copy(original, values)

	stats := ComputeRollingStatistics(values)

	assert.NotNil(t, stats)
	assert.Equal(t, 5, stats.SampleCount)

	// Original slice should be unchanged
	assert.Equal(t, original, values, "Input slice should not be mutated")
}

func TestComputeRollingStatistics_LargeDataset(t *testing.T) {
	// Input: 1000 values with known distribution
	values := make([]float64, 1000)
	for i := 0; i < 1000; i++ {
		values[i] = float64(i + 1)
	}

	stats := ComputeRollingStatistics(values)

	assert.NotNil(t, stats)
	assert.Equal(t, 1000, stats.SampleCount)
	assert.Equal(t, 500.5, stats.Mean)
	assert.Equal(t, 1.0, stats.Min)
	assert.Equal(t, 1000.0, stats.Max)

	// P99 should be around 990
	assert.InDelta(t, 990.0, stats.P99, 15.0, "P99 should be approximately 990")
}

func TestComputeRollingStatistics_StdDev(t *testing.T) {
	// Input: [2, 4, 4, 4, 5, 5, 7, 9] - known stddev
	values := []float64{2, 4, 4, 4, 5, 5, 7, 9}

	stats := ComputeRollingStatistics(values)

	assert.NotNil(t, stats)
	assert.Equal(t, 8, stats.SampleCount)
	assert.Equal(t, 5.0, stats.Mean)

	// Sample standard deviation should be ~2.138
	// Population stddev = 2, sample stddev = sqrt(32/7) ~= 2.138
	assert.InDelta(t, 2.138, stats.StdDev, 0.01, "StdDev should be approximately 2.138")
}

func TestComputeRollingStatistics_NegativeValues(t *testing.T) {
	// Input: mix of positive and negative values
	values := []float64{-5, -3, 0, 3, 5}

	stats := ComputeRollingStatistics(values)

	assert.NotNil(t, stats)
	assert.Equal(t, 5, stats.SampleCount)
	assert.Equal(t, 0.0, stats.Mean)
	assert.Equal(t, -5.0, stats.Min)
	assert.Equal(t, 5.0, stats.Max)
	assert.Equal(t, 0.0, stats.Median)
}

func TestInsufficientSamplesError(t *testing.T) {
	// Create error with Available=5, Required=10
	err := &InsufficientSamplesError{
		Available: 5,
		Required:  10,
	}

	// Error message should contain both numbers
	msg := err.Error()
	assert.Contains(t, msg, "5")
	assert.Contains(t, msg, "10")
	assert.Contains(t, msg, "insufficient samples")
}

func TestInsufficientSamplesError_ZeroSamples(t *testing.T) {
	// Edge case: zero samples
	err := &InsufficientSamplesError{
		Available: 0,
		Required:  MinSamplesRequired,
	}

	msg := err.Error()
	assert.Contains(t, msg, "0")
	assert.Contains(t, msg, "10")
}

func TestMinSamplesRequired_Constant(t *testing.T) {
	// Verify constant is set correctly per CONTEXT.md
	assert.Equal(t, 10, MinSamplesRequired)
}

func TestSignalBaseline_Fields(t *testing.T) {
	// Verify SignalBaseline struct has all required fields
	baseline := SignalBaseline{
		// Identity fields
		MetricName:        "container_cpu_usage_seconds_total",
		WorkloadNamespace: "default",
		WorkloadName:      "my-app",
		Integration:       "grafana-prod",

		// Statistics
		Mean:        50.0,
		StdDev:      10.0,
		Median:      49.0,
		P50:         49.0,
		P90:         70.0,
		P99:         95.0,
		Min:         5.0,
		Max:         100.0,
		SampleCount: 1000,

		// Window metadata
		WindowStart: 1706500000,
		WindowEnd:   1707100000,

		// TTL
		LastUpdated: 1707100000,
		ExpiresAt:   1707704800, // 7 days later
	}

	assert.Equal(t, "container_cpu_usage_seconds_total", baseline.MetricName)
	assert.Equal(t, "default", baseline.WorkloadNamespace)
	assert.Equal(t, "my-app", baseline.WorkloadName)
	assert.Equal(t, "grafana-prod", baseline.Integration)
	assert.Equal(t, 50.0, baseline.Mean)
	assert.Equal(t, 10.0, baseline.StdDev)
	assert.Equal(t, 49.0, baseline.Median)
	assert.Equal(t, 49.0, baseline.P50)
	assert.Equal(t, 70.0, baseline.P90)
	assert.Equal(t, 95.0, baseline.P99)
	assert.Equal(t, 5.0, baseline.Min)
	assert.Equal(t, 100.0, baseline.Max)
	assert.Equal(t, 1000, baseline.SampleCount)
	assert.Equal(t, int64(1706500000), baseline.WindowStart)
	assert.Equal(t, int64(1707100000), baseline.WindowEnd)
	assert.Equal(t, int64(1707100000), baseline.LastUpdated)
	assert.Equal(t, int64(1707704800), baseline.ExpiresAt)
}

func TestRollingStats_Fields(t *testing.T) {
	// Verify RollingStats struct has all required fields
	stats := RollingStats{
		Mean:        50.0,
		StdDev:      10.0,
		Median:      49.0,
		P50:         49.0,
		P90:         70.0,
		P99:         95.0,
		Min:         5.0,
		Max:         100.0,
		SampleCount: 1000,
	}

	assert.Equal(t, 50.0, stats.Mean)
	assert.Equal(t, 10.0, stats.StdDev)
	assert.Equal(t, 49.0, stats.Median)
	assert.Equal(t, 49.0, stats.P50)
	assert.Equal(t, 70.0, stats.P90)
	assert.Equal(t, 95.0, stats.P99)
	assert.Equal(t, 5.0, stats.Min)
	assert.Equal(t, 100.0, stats.Max)
	assert.Equal(t, 1000, stats.SampleCount)
}
