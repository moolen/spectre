package observatory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCuratedMetrics_Load(t *testing.T) {
	count := GetCuratedMetricCount()
	assert.Greater(t, count, 0, "should load curated metrics from embedded JSON files")
	t.Logf("Loaded %d curated metrics", count)
}

func TestCuratedMetrics_LookupExact(t *testing.T) {
	testCases := []struct {
		metricName   string
		expectedRole SignalRole
	}{
		{"up", SignalAvailability},
		{"kube_pod_status_phase", SignalAvailability},
		{"container_cpu_usage_seconds_total", SignalSaturation},
		{"container_memory_working_set_bytes", SignalSaturation},
		{"apiserver_request_duration_seconds", SignalLatency},
		{"etcd_server_has_leader", SignalAvailability},
		{"kube_pod_container_status_restarts_total", SignalNovelty}, // churn maps to novelty
	}

	for _, tc := range testCases {
		t.Run(tc.metricName, func(t *testing.T) {
			metric := LookupCuratedMetric(tc.metricName)
			require.NotNil(t, metric, "metric %s should be found in curated data", tc.metricName)
			assert.Equal(t, tc.expectedRole, metric.ToSignalRole(), "metric %s should have role %s", tc.metricName, tc.expectedRole)
			assert.Greater(t, metric.Confidence, 0.0, "confidence should be positive")
			assert.LessOrEqual(t, metric.Confidence, 1.0, "confidence should be <= 1.0")
		})
	}
}

func TestCuratedMetrics_LookupNotFound(t *testing.T) {
	metric := LookupCuratedMetric("nonexistent_metric_foobar_12345")
	assert.Nil(t, metric, "nonexistent metric should return nil")
}

func TestCuratedMetrics_SignalRoleConversion(t *testing.T) {
	testCases := []struct {
		roleStr  string
		expected SignalRole
	}{
		{"availability", SignalAvailability},
		{"Availability", SignalAvailability},
		{"latency", SignalLatency},
		{"errors", SignalErrors},
		{"traffic", SignalTraffic},
		{"saturation", SignalSaturation},
		{"novelty", SignalNovelty},
		{"churn", SignalNovelty}, // churn maps to novelty
		{"unknown", SignalUnknown},
		{"foobar", SignalUnknown},
	}

	for _, tc := range testCases {
		t.Run(tc.roleStr, func(t *testing.T) {
			result := signalRoleFromString(tc.roleStr)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestCuratedMetrics_MetadataPreserved(t *testing.T) {
	metric := LookupCuratedMetric("up")
	require.NotNil(t, metric)

	assert.Equal(t, "up", metric.Name)
	assert.Equal(t, "availability", metric.SignalRole)
	assert.Equal(t, "prometheus/scrape", metric.Source)
	assert.Equal(t, "gauge", metric.MetricType)
	assert.NotEmpty(t, metric.Notes)
	assert.NotEmpty(t, metric.LabelsOfInterest)
	assert.NotEmpty(t, metric.CommonPromQLPatterns)
}

func TestCuratedMetrics_ClassifyKnownMetric(t *testing.T) {
	// Test that the classifier uses curated metrics
	testCases := []struct {
		metricName   string
		expectedRole SignalRole
	}{
		{"up", SignalAvailability},
		{"kube_pod_status_phase", SignalAvailability},
		{"container_cpu_usage_seconds_total", SignalSaturation},
		{"etcd_disk_wal_fsync_duration_seconds", SignalLatency},
	}

	for _, tc := range testCases {
		t.Run(tc.metricName, func(t *testing.T) {
			result := classifyKnownMetric(tc.metricName)
			require.NotNil(t, result, "metric %s should be classified", tc.metricName)
			assert.Equal(t, tc.expectedRole, result.Role)
			assert.Equal(t, 1, result.Layer, "should be layer 1 classification")
		})
	}
}

func TestCuratedMetrics_AllMetrics(t *testing.T) {
	metrics := GetAllCuratedMetrics()
	require.NotEmpty(t, metrics)

	// Verify all metrics have required fields
	for _, m := range metrics {
		// Name should be populated (either directly or from name_pattern)
		assert.NotEmpty(t, m.Name, "metric name should not be empty (metric: %+v)", m)
		assert.NotEmpty(t, m.SignalRole, "signal role should not be empty for metric: %s", m.Name)
		assert.NotEmpty(t, m.Source, "source should not be empty for metric: %s", m.Name)
		assert.NotEmpty(t, m.MetricType, "metric type should not be empty for metric: %s", m.Name)
		assert.Greater(t, m.Confidence, 0.0, "confidence should be positive for metric: %s", m.Name)
		assert.LessOrEqual(t, m.Confidence, 1.0, "confidence should be <= 1.0 for metric: %s", m.Name)
	}
}
