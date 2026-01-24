package grafana

import (
	"math"
	"strings"
	"time"
)

// StatisticalDetector performs z-score based anomaly detection
type StatisticalDetector struct{}

// computeMean calculates the arithmetic mean of values
func computeMean(values []float64) float64 {
	if len(values) == 0 {
		return 0.0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}

	return sum / float64(len(values))
}

// computeStdDev calculates the sample standard deviation
func computeStdDev(values []float64, mean float64) float64 {
	n := len(values)
	if n < 2 {
		return 0.0
	}

	// Compute variance using sample formula (n-1)
	variance := 0.0
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(n - 1)

	return math.Sqrt(variance)
}

// computeZScore calculates the z-score for a value
func computeZScore(value, mean, stddev float64) float64 {
	if stddev == 0 {
		return 0.0
	}

	return (value - mean) / stddev
}

// isErrorRateMetric checks if a metric represents error/failure rates
func isErrorRateMetric(metricName string) bool {
	lowerName := strings.ToLower(metricName)

	patterns := []string{"5xx", "error", "failed", "failure"}
	for _, pattern := range patterns {
		if strings.Contains(lowerName, pattern) {
			return true
		}
	}

	return false
}

// classifySeverity determines anomaly severity based on z-score
func classifySeverity(metricName string, zScore float64) string {
	// Use absolute value for threshold comparison
	absZ := math.Abs(zScore)

	// Error metrics have lower thresholds
	if isErrorRateMetric(metricName) {
		if absZ >= 2.0 {
			return "critical"
		}
		if absZ >= 1.5 {
			return "warning"
		}
		if absZ >= 1.0 {
			return "info"
		}
		return ""
	}

	// Non-error metrics use standard thresholds
	if absZ >= 3.0 {
		return "critical"
	}
	if absZ >= 2.0 {
		return "warning"
	}
	if absZ >= 1.5 {
		return "info"
	}

	return ""
}

// Detect performs anomaly detection on a metric value
func (d *StatisticalDetector) Detect(metricName string, value float64, baseline Baseline, timestamp time.Time) *MetricAnomaly {
	// Compute z-score
	zScore := computeZScore(value, baseline.Mean, baseline.StdDev)

	// Classify severity
	severity := classifySeverity(metricName, zScore)

	// Return nil if not anomalous
	if severity == "" {
		return nil
	}

	// Return anomaly with all fields populated
	return &MetricAnomaly{
		MetricName: metricName,
		Value:      value,
		Baseline:   baseline.Mean,
		ZScore:     zScore,
		Severity:   severity,
		Timestamp:  timestamp,
	}
}
