package grafana

import (
	"math"
	"testing"
	"time"
)

func TestComputeMean(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		expected float64
	}{
		{
			name:     "simple sequence",
			values:   []float64{1, 2, 3, 4, 5},
			expected: 3.0,
		},
		{
			name:     "two decimals",
			values:   []float64{10.5, 20.5},
			expected: 15.5,
		},
		{
			name:     "empty slice",
			values:   []float64{},
			expected: 0.0,
		},
		{
			name:     "single value",
			values:   []float64{42.0},
			expected: 42.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeMean(tt.values)
			if math.Abs(result-tt.expected) > 0.0001 {
				t.Errorf("computeMean(%v) = %v, want %v", tt.values, result, tt.expected)
			}
		})
	}
}

func TestComputeStdDev(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		mean     float64
		expected float64
	}{
		{
			name:     "normal distribution",
			values:   []float64{2, 4, 6, 8},
			mean:     5.0,
			expected: 2.581989, // sample stddev with n-1
		},
		{
			name:     "all same values",
			values:   []float64{5, 5, 5},
			mean:     5.0,
			expected: 0.0,
		},
		{
			name:     "single value",
			values:   []float64{10},
			mean:     10.0,
			expected: 0.0,
		},
		{
			name:     "empty slice",
			values:   []float64{},
			mean:     0.0,
			expected: 0.0,
		},
		{
			name:     "two values",
			values:   []float64{10, 20},
			mean:     15.0,
			expected: 7.071068, // sqrt(50)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeStdDev(tt.values, tt.mean)
			if math.Abs(result-tt.expected) > 0.0001 {
				t.Errorf("computeStdDev(%v, %v) = %v, want %v", tt.values, tt.mean, result, tt.expected)
			}
		})
	}
}

func TestComputeZScore(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		mean     float64
		stddev   float64
		expected float64
	}{
		{
			name:     "one sigma above",
			value:    110,
			mean:     100,
			stddev:   10,
			expected: 1.0,
		},
		{
			name:     "one sigma below",
			value:    90,
			mean:     100,
			stddev:   10,
			expected: -1.0,
		},
		{
			name:     "three sigma above",
			value:    130,
			mean:     100,
			stddev:   10,
			expected: 3.0,
		},
		{
			name:     "zero stddev",
			value:    100,
			mean:     100,
			stddev:   0,
			expected: 0.0,
		},
		{
			name:     "at mean",
			value:    100,
			mean:     100,
			stddev:   10,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeZScore(tt.value, tt.mean, tt.stddev)
			if math.Abs(result-tt.expected) > 0.0001 {
				t.Errorf("computeZScore(%v, %v, %v) = %v, want %v",
					tt.value, tt.mean, tt.stddev, result, tt.expected)
			}
		})
	}
}

func TestIsErrorRateMetric(t *testing.T) {
	tests := []struct {
		name       string
		metricName string
		expected   bool
	}{
		{
			name:       "5xx metric",
			metricName: "http_requests_5xx_total",
			expected:   true,
		},
		{
			name:       "error rate",
			metricName: "error_rate",
			expected:   true,
		},
		{
			name:       "failed requests",
			metricName: "failed_requests",
			expected:   true,
		},
		{
			name:       "failure count",
			metricName: "failure_count",
			expected:   true,
		},
		{
			name:       "Error uppercase",
			metricName: "REQUEST_ERROR_TOTAL",
			expected:   true,
		},
		{
			name:       "normal metric",
			metricName: "http_requests_total",
			expected:   false,
		},
		{
			name:       "cpu metric",
			metricName: "cpu_usage",
			expected:   false,
		},
		{
			name:       "memory metric",
			metricName: "memory_bytes",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isErrorRateMetric(tt.metricName)
			if result != tt.expected {
				t.Errorf("isErrorRateMetric(%q) = %v, want %v", tt.metricName, result, tt.expected)
			}
		})
	}
}

func TestClassifySeverity(t *testing.T) {
	tests := []struct {
		name       string
		metricName string
		zScore     float64
		expected   string
	}{
		// Non-error metrics
		{
			name:       "non-error critical",
			metricName: "cpu_usage",
			zScore:     3.5,
			expected:   "critical",
		},
		{
			name:       "non-error warning",
			metricName: "cpu_usage",
			zScore:     2.5,
			expected:   "warning",
		},
		{
			name:       "non-error info",
			metricName: "cpu_usage",
			zScore:     1.6,
			expected:   "info",
		},
		{
			name:       "non-error not anomalous",
			metricName: "cpu_usage",
			zScore:     1.0,
			expected:   "",
		},
		// Error metrics (lower thresholds)
		{
			name:       "error metric critical",
			metricName: "http_requests_5xx_total",
			zScore:     2.1,
			expected:   "critical",
		},
		{
			name:       "error metric warning",
			metricName: "error_rate",
			zScore:     1.6,
			expected:   "warning",
		},
		{
			name:       "error metric info",
			metricName: "failed_requests",
			zScore:     1.1,
			expected:   "info",
		},
		{
			name:       "error metric not anomalous",
			metricName: "error_rate",
			zScore:     0.9,
			expected:   "",
		},
		// Negative z-scores (below baseline)
		{
			name:       "negative z-score critical",
			metricName: "cpu_usage",
			zScore:     -3.5,
			expected:   "critical",
		},
		{
			name:       "negative z-score warning",
			metricName: "cpu_usage",
			zScore:     -2.5,
			expected:   "warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifySeverity(tt.metricName, tt.zScore)
			if result != tt.expected {
				t.Errorf("classifySeverity(%q, %v) = %q, want %q",
					tt.metricName, tt.zScore, result, tt.expected)
			}
		})
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name             string
		metricName       string
		value            float64
		baseline         Baseline
		expectedAnomaly  bool
		expectedSeverity string
		expectedZScore   float64
	}{
		{
			name:       "no anomaly",
			metricName: "cpu_usage",
			value:      105,
			baseline: Baseline{
				MetricName: "cpu_usage",
				Mean:       100,
				StdDev:     10,
			},
			expectedAnomaly: false,
		},
		{
			name:       "warning level anomaly",
			metricName: "cpu_usage",
			value:      125,
			baseline: Baseline{
				MetricName: "cpu_usage",
				Mean:       100,
				StdDev:     10,
			},
			expectedAnomaly:  true,
			expectedSeverity: "warning",
			expectedZScore:   2.5,
		},
		{
			name:       "critical level anomaly",
			metricName: "cpu_usage",
			value:      135,
			baseline: Baseline{
				MetricName: "cpu_usage",
				Mean:       100,
				StdDev:     10,
			},
			expectedAnomaly:  true,
			expectedSeverity: "critical",
			expectedZScore:   3.5,
		},
		{
			name:       "error metric critical at 2 sigma",
			metricName: "error_rate",
			value:      120,
			baseline: Baseline{
				MetricName: "error_rate",
				Mean:       100,
				StdDev:     10,
			},
			expectedAnomaly:  true,
			expectedSeverity: "critical",
			expectedZScore:   2.0,
		},
		{
			name:       "zero stddev no anomaly",
			metricName: "cpu_usage",
			value:      100,
			baseline: Baseline{
				MetricName: "cpu_usage",
				Mean:       100,
				StdDev:     0,
			},
			expectedAnomaly: false,
		},
	}

	detector := &StatisticalDetector{}
	timestamp := time.Now()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anomaly := detector.Detect(tt.metricName, tt.value, tt.baseline, timestamp)

			if tt.expectedAnomaly {
				if anomaly == nil {
					t.Fatalf("Detect() returned nil, expected anomaly")
				}
				if anomaly.MetricName != tt.metricName {
					t.Errorf("anomaly.MetricName = %q, want %q", anomaly.MetricName, tt.metricName)
				}
				if anomaly.Value != tt.value {
					t.Errorf("anomaly.Value = %v, want %v", anomaly.Value, tt.value)
				}
				if anomaly.Baseline != tt.baseline.Mean {
					t.Errorf("anomaly.Baseline = %v, want %v", anomaly.Baseline, tt.baseline.Mean)
				}
				if anomaly.Severity != tt.expectedSeverity {
					t.Errorf("anomaly.Severity = %q, want %q", anomaly.Severity, tt.expectedSeverity)
				}
				if math.Abs(anomaly.ZScore-tt.expectedZScore) > 0.0001 {
					t.Errorf("anomaly.ZScore = %v, want %v", anomaly.ZScore, tt.expectedZScore)
				}
				if !anomaly.Timestamp.Equal(timestamp) {
					t.Errorf("anomaly.Timestamp = %v, want %v", anomaly.Timestamp, timestamp)
				}
			} else {
				if anomaly != nil {
					t.Errorf("Detect() returned anomaly %+v, expected nil", anomaly)
				}
			}
		})
	}
}
