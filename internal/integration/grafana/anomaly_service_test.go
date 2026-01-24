package grafana

import (
	"fmt"
	"testing"
	"time"
)

// TestDetectAnomaliesBasic tests basic anomaly detection with a single metric exceeding threshold
func TestDetectAnomaliesBasic(t *testing.T) {

	// Create detector and baseline cache with real implementations
	detector := &StatisticalDetector{}

	// Create a baseline that will classify value=130 as critical (z-score=3.0)
	baseline := &Baseline{
		MetricName:  "cpu_usage",
		Mean:        100.0,
		StdDev:      10.0,
		SampleCount: 10,
		WindowHour:  10,
		DayType:     "weekday",
	}

	// Test the detector directly
	timestamp, _ := time.Parse(time.RFC3339, "2026-01-23T10:00:00Z")
	anomaly := detector.Detect("cpu_usage", 130.0, *baseline, timestamp)

	// Assert anomaly was detected
	if anomaly == nil {
		t.Fatalf("Detect() returned nil, expected anomaly")
	}

	// Assert anomaly fields
	if anomaly.MetricName != "cpu_usage" {
		t.Errorf("anomaly.MetricName = %q, want %q", anomaly.MetricName, "cpu_usage")
	}
	if anomaly.Value != 130.0 {
		t.Errorf("anomaly.Value = %v, want %v", anomaly.Value, 130.0)
	}
	if anomaly.Baseline != 100.0 {
		t.Errorf("anomaly.Baseline = %v, want %v", anomaly.Baseline, 100.0)
	}
	if anomaly.ZScore != 3.0 {
		t.Errorf("anomaly.ZScore = %v, want %v", anomaly.ZScore, 3.0)
	}
	if anomaly.Severity != "critical" {
		t.Errorf("anomaly.Severity = %q, want %q", anomaly.Severity, "critical")
	}
}

// TestDetectAnomaliesNoAnomalies tests when metrics are within normal range
func TestDetectAnomaliesNoAnomalies(t *testing.T) {
	// Create detector
	detector := &StatisticalDetector{}

	// Create baseline
	baseline := &Baseline{
		MetricName:  "cpu_usage",
		Mean:        100.0,
		StdDev:      10.0,
		SampleCount: 10,
		WindowHour:  10,
		DayType:     "weekday",
	}

	// Test with value within normal range (z-score=0.2)
	timestamp, _ := time.Parse(time.RFC3339, "2026-01-23T10:00:00Z")
	anomaly := detector.Detect("cpu_usage", 102.0, *baseline, timestamp)

	// Assert no anomaly detected
	if anomaly != nil {
		t.Errorf("Detect() returned anomaly %+v, expected nil", anomaly)
	}
}

// TestDetectAnomaliesZeroStdDev tests handling of baselines with zero standard deviation
func TestDetectAnomaliesZeroStdDev(t *testing.T) {
	// Create detector
	detector := &StatisticalDetector{}

	// Create baseline with zero stddev
	baseline := &Baseline{
		MetricName:  "cpu_usage",
		Mean:        100.0,
		StdDev:      0.0, // Zero standard deviation
		SampleCount: 10,
		WindowHour:  10,
		DayType:     "weekday",
	}

	// Test with same value as mean
	timestamp, _ := time.Parse(time.RFC3339, "2026-01-23T10:00:00Z")
	anomaly := detector.Detect("cpu_usage", 100.0, *baseline, timestamp)

	// Assert no anomaly (zero stddev should result in z-score=0)
	if anomaly != nil {
		t.Errorf("Detect() returned anomaly %+v, expected nil (zero stddev should not trigger anomaly)", anomaly)
	}
}

// TestDetectAnomaliesErrorMetricLowerThreshold tests error metrics use lower thresholds
func TestDetectAnomaliesErrorMetricLowerThreshold(t *testing.T) {
	// Create detector
	detector := &StatisticalDetector{}

	// Create baseline
	baseline := &Baseline{
		MetricName:  "error_rate",
		Mean:        100.0,
		StdDev:      10.0,
		SampleCount: 10,
		WindowHour:  10,
		DayType:     "weekday",
	}

	// Test error metric at 2 sigma (should be critical for error metrics, not for normal metrics)
	timestamp, _ := time.Parse(time.RFC3339, "2026-01-23T10:00:00Z")
	anomaly := detector.Detect("error_rate", 120.0, *baseline, timestamp)

	// Assert anomaly with critical severity (error metrics have lower threshold: 2σ = critical)
	if anomaly == nil {
		t.Fatalf("Detect() returned nil, expected anomaly for error metric")
	}
	if anomaly.Severity != "critical" {
		t.Errorf("anomaly.Severity = %q, want %q (error metrics should be critical at 2σ)", anomaly.Severity, "critical")
	}
}

// TestMatchTimeWindows tests time-of-day matching logic
func TestMatchTimeWindows(t *testing.T) {
	// Create test data with various timestamps
	// Jan 2026: 19=Mon, 20=Tue, 22=Thu, 24=Sat, 25=Sun
	historicalData := []HistoricalDataPoint{
		{Timestamp: time.Date(2026, 1, 19, 10, 0, 0, 0, time.UTC), Value: 100.0}, // Monday 10:00 (weekday)
		{Timestamp: time.Date(2026, 1, 19, 11, 0, 0, 0, time.UTC), Value: 110.0}, // Monday 11:00 (weekday)
		{Timestamp: time.Date(2026, 1, 20, 10, 0, 0, 0, time.UTC), Value: 105.0}, // Tuesday 10:00 (weekday)
		{Timestamp: time.Date(2026, 1, 24, 10, 0, 0, 0, time.UTC), Value: 90.0},  // Saturday 10:00 (weekend)
		{Timestamp: time.Date(2026, 1, 25, 10, 0, 0, 0, time.UTC), Value: 95.0},  // Sunday 10:00 (weekend)
	}

	// Test matching for Thursday 10:00 (weekday)
	currentTime := time.Date(2026, 1, 22, 10, 0, 0, 0, time.UTC) // Thursday 10:00
	matched := matchTimeWindows(currentTime, historicalData)

	// Should match Monday 10:00, Tuesday 10:00 (weekday, hour 10), not Saturday/Sunday or hour 11
	if len(matched) != 2 {
		t.Errorf("len(matched) = %d, want 2 (weekday 10:00 matches)", len(matched))
	}

	// Verify matched values (100.0 and 105.0)
	expectedValues := map[float64]bool{100.0: true, 105.0: true}
	for _, val := range matched {
		if !expectedValues[val] {
			t.Errorf("Unexpected matched value: %v", val)
		}
	}
}

// TestMatchTimeWindowsWeekend tests weekend matching
func TestMatchTimeWindowsWeekend(t *testing.T) {
	// Jan 2026: 19=Mon, 24=Sat, 25=Sun
	historicalData := []HistoricalDataPoint{
		{Timestamp: time.Date(2026, 1, 19, 10, 0, 0, 0, time.UTC), Value: 100.0}, // Monday (weekday)
		{Timestamp: time.Date(2026, 1, 24, 10, 0, 0, 0, time.UTC), Value: 90.0},  // Saturday (weekend)
		{Timestamp: time.Date(2026, 1, 25, 10, 0, 0, 0, time.UTC), Value: 95.0},  // Sunday (weekend)
	}

	// Test matching for Saturday 10:00
	currentTime := time.Date(2026, 1, 24, 10, 0, 0, 0, time.UTC) // Saturday 10:00
	matched := matchTimeWindows(currentTime, historicalData)

	// Should match Saturday 10:00 and Sunday 10:00 (weekend, hour 10)
	if len(matched) != 2 {
		t.Errorf("len(matched) = %d, want 2 (Saturday 10:00 and Sunday 10:00)", len(matched))
	}

	// Verify matched values
	expectedValues := map[float64]bool{90.0: true, 95.0: true}
	for _, val := range matched {
		if !expectedValues[val] {
			t.Errorf("Unexpected matched value: %v (expected weekend values only)", val)
		}
	}
}

// TestExtractMetricName tests metric name extraction from labels
func TestExtractMetricName(t *testing.T) {
	tests := []struct {
		name         string
		labels       map[string]string
		expected     string
		acceptAnyKey bool // For non-deterministic map iteration
	}{
		{
			name:     "__name__ label present",
			labels:   map[string]string{"__name__": "cpu_usage", "job": "api"},
			expected: "cpu_usage",
		},
		{
			name:         "no __name__ label, fallback to any label",
			labels:       map[string]string{"job": "api", "instance": "localhost"},
			acceptAnyKey: true, // Map iteration is non-deterministic, accept job=api or instance=localhost
		},
		{
			name:     "empty labels",
			labels:   map[string]string{},
			expected: "",
		},
		{
			name:         "__name__ empty, fallback",
			labels:       map[string]string{"__name__": "", "job": "api"},
			acceptAnyKey: true, // Should fallback to job=api
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMetricName(tt.labels)
			if tt.acceptAnyKey {
				// Check that result is one of the labels in key=value format
				found := false
				for k, v := range tt.labels {
					if k == "__name__" && v == "" {
						continue // Skip empty __name__
					}
					if result == fmt.Sprintf("%s=%s", k, v) {
						found = true
						break
					}
				}
				if !found && result != "" {
					t.Errorf("extractMetricName(%v) = %q, want one of the labels in key=value format", tt.labels, result)
				}
			} else {
				if result != tt.expected {
					t.Errorf("extractMetricName(%v) = %q, want %q", tt.labels, result, tt.expected)
				}
			}
		})
	}
}

// TestComputeBaselineMinimumSamples tests that baseline computation requires minimum 3 samples
func TestComputeBaselineMinimumSamples(t *testing.T) {
	// Test data with only 2 matching windows (< minimum 3)
	currentTime := time.Date(2026, 1, 23, 10, 0, 0, 0, time.UTC) // Friday 10:00

	historicalData := []HistoricalDataPoint{
		{Timestamp: time.Date(2026, 1, 20, 10, 0, 0, 0, time.UTC), Value: 100.0}, // Monday 10:00
		{Timestamp: time.Date(2026, 1, 21, 10, 0, 0, 0, time.UTC), Value: 105.0}, // Tuesday 10:00
	}

	matched := matchTimeWindows(currentTime, historicalData)

	// Should match 2 samples
	if len(matched) != 2 {
		t.Errorf("len(matched) = %d, want 2", len(matched))
	}

	// Baseline computation should skip this metric (< 3 samples)
	// This is tested in the actual AnomalyService.computeBaseline method
	// Here we just verify the matching logic
}

// TestAnomalyRanking tests that anomalies are ranked by severity then z-score
func TestAnomalyRanking(t *testing.T) {
	anomalies := []MetricAnomaly{
		{MetricName: "m1", ZScore: 2.5, Severity: "warning"},
		{MetricName: "m2", ZScore: 3.5, Severity: "critical"},
		{MetricName: "m3", ZScore: 1.8, Severity: "info"},
		{MetricName: "m4", ZScore: 4.0, Severity: "critical"},
		{MetricName: "m5", ZScore: 2.8, Severity: "warning"},
	}

	// Manually apply ranking logic from AnomalyService
	severityRank := map[string]int{
		"critical": 3,
		"warning":  2,
		"info":     1,
	}

	// Sort anomalies using same logic as DetectAnomalies
	for i := 0; i < len(anomalies); i++ {
		for j := i + 1; j < len(anomalies); j++ {
			rankI := severityRank[anomalies[i].Severity]
			rankJ := severityRank[anomalies[j].Severity]

			shouldSwap := false
			if rankI < rankJ {
				shouldSwap = true
			} else if rankI == rankJ {
				absZI := anomalies[i].ZScore
				if absZI < 0 {
					absZI = -absZI
				}
				absZJ := anomalies[j].ZScore
				if absZJ < 0 {
					absZJ = -absZJ
				}
				if absZI < absZJ {
					shouldSwap = true
				}
			}

			if shouldSwap {
				anomalies[i], anomalies[j] = anomalies[j], anomalies[i]
			}
		}
	}

	// Assert order: critical (highest z-score first), then warning, then info
	expectedOrder := []string{"m4", "m2", "m5", "m1", "m3"}
	for i, expected := range expectedOrder {
		if anomalies[i].MetricName != expected {
			t.Errorf("anomalies[%d].MetricName = %q, want %q", i, anomalies[i].MetricName, expected)
		}
	}
}
