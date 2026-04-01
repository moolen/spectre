package grafana

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStatisticalAnalyzer_Analyze(t *testing.T) {
	analyzer := NewStatisticalAnalyzer(0.05, 0.8, 2.0)

	testCases := []struct {
		name           string
		beforeValues   []float64
		afterValues    []float64
		expectSignificant bool
	}{
		{
			name:           "significant change - mean doubled",
			beforeValues:   []float64{10, 11, 9, 10, 11, 10, 9, 10, 11, 10},
			afterValues:    []float64{20, 21, 19, 20, 21, 20, 19, 20, 21, 20},
			expectSignificant: true,
		},
		{
			name:           "no significant change - similar means",
			beforeValues:   []float64{10, 11, 9, 10, 11, 10, 9, 10, 11, 10},
			afterValues:    []float64{10.5, 11.5, 9.5, 10.5, 11.5, 10.5, 9.5, 10.5, 11.5, 10.5},
			expectSignificant: false,
		},
		{
			name:           "large effect size detected",
			beforeValues:   []float64{100, 102, 98, 101, 99, 100, 101, 99, 100, 101},
			afterValues:    []float64{150, 152, 148, 151, 149, 150, 151, 149, 150, 151},
			expectSignificant: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			before := &MetricWindow{
				Start:  now.Add(-30 * time.Minute),
				End:    now.Add(-15 * time.Minute),
				Values: tc.beforeValues,
			}
			after := &MetricWindow{
				Start:  now,
				End:    now.Add(15 * time.Minute),
				Values: tc.afterValues,
			}

			result := analyzer.Analyze(before, after)

			assert.Equal(t, tc.expectSignificant, result.IsSignificant)
			assert.Equal(t, len(tc.beforeValues), result.SamplesBefore)
			assert.Equal(t, len(tc.afterValues), result.SamplesAfter)
		})
	}
}

func TestStatisticalAnalyzer_TTest(t *testing.T) {
	analyzer := NewStatisticalAnalyzer(0.05, 0.8, 2.0)

	// Test with clearly different distributions (need some variance)
	before := []float64{10, 11, 9, 10, 11, 9, 10, 11, 9, 10}
	after := []float64{20, 21, 19, 20, 21, 19, 20, 21, 19, 20}

	result := analyzer.welchTTest(before, after)

	assert.True(t, result.Significant)
	assert.Less(t, result.PValue, 0.05)
}

func TestStatisticalAnalyzer_CohensD(t *testing.T) {
	analyzer := NewStatisticalAnalyzer(0.05, 0.8, 2.0)

	testCases := []struct {
		name        string
		before      []float64
		after       []float64
		expectLarge bool
	}{
		{
			name:        "large effect - means far apart",
			before:      []float64{10, 11, 9, 10, 11, 9, 10, 11},   // mean=10, stddev≈0.9
			after:       []float64{20, 21, 19, 20, 21, 19, 20, 21}, // mean=20
			expectLarge: true, // d = (20-10)/~1 = 10, definitely > 0.8
		},
		{
			name:        "small effect - means close",
			before:      []float64{10, 11, 9, 10, 11, 9, 10, 11}, // mean=10, stddev≈0.9
			after:       []float64{10.2, 11.2, 9.2, 10.2, 11.2, 9.2, 10.2, 11.2}, // mean=10.2
			expectLarge: false, // d = (10.2-10)/~0.9 ≈ 0.2, definitely < 0.8
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzer.cohensD(tc.before, tc.after)
			assert.Equal(t, tc.expectLarge, result.Significant, "CohensD: %.2f", result.CohensD)
		})
	}
}

func TestStatisticalAnalyzer_ThresholdCheck(t *testing.T) {
	analyzer := NewStatisticalAnalyzer(0.05, 0.8, 2.0)

	testCases := []struct {
		name           string
		before         []float64
		after          []float64
		expectExceeded bool
	}{
		{
			name:           "exceeds 2 sigma",
			before:         []float64{100, 101, 99, 100, 101},  // mean=100, stddev≈0.7
			after:          []float64{110, 111, 109, 110, 111}, // mean shifted by ~10
			expectExceeded: true,
		},
		{
			name:           "within 2 sigma",
			before:         []float64{100, 101, 99, 100, 101},
			after:          []float64{101, 102, 100, 101, 102}, // mean shifted by ~1
			expectExceeded: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzer.thresholdCheck(tc.before, tc.after)
			assert.Equal(t, tc.expectExceeded, result.Exceeded)
		})
	}
}

func TestCorrelationResult_ToGraphStats(t *testing.T) {
	result := &CorrelationResult{
		TTest:         TTestResult{TStatistic: 5.5, PValue: 0.001, Significant: true},
		EffectSize:    EffectSizeResult{CohensD: 1.2, Significant: true},
		Threshold:     ThresholdResult{SigmaChange: 3.5, Exceeded: true},
		MeanBefore:    100.0,
		MeanAfter:     150.0,
		StddevBefore:  10.0,
		StddevAfter:   12.0,
		SamplesBefore: 30,
		SamplesAfter:  30,
	}

	stats := result.ToGraphStats()

	assert.Equal(t, 5.5, stats.TStatistic)
	assert.Equal(t, 0.001, stats.PValue)
	assert.Equal(t, 1.2, stats.CohensD)
	assert.True(t, stats.ThresholdExceeded)
	assert.Equal(t, 100.0, stats.MeanBefore)
	assert.Equal(t, 150.0, stats.MeanAfter)
	assert.Equal(t, 10.0, stats.StddevBefore)
	assert.Equal(t, 12.0, stats.StddevAfter)
	assert.Equal(t, 30, stats.SamplesBefore)
	assert.Equal(t, 30, stats.SamplesAfter)
}
