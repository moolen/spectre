package grafana

import (
	"testing"

	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/assert"
)

func TestAlertSignalMatcher_ExtractMetricNames(t *testing.T) {
	logger := logging.GetLogger("test")
	matcher := NewAlertSignalMatcher(nil, "test-integration", logger)

	testCases := []struct {
		name     string
		promQL   string
		expected []string
	}{
		{
			name:     "simple metric",
			promQL:   "http_requests_total",
			expected: []string{"http_requests_total"},
		},
		{
			name:     "metric with labels",
			promQL:   `http_requests_total{code="500"}`,
			expected: []string{"http_requests_total"},
		},
		{
			name:     "rate function",
			promQL:   `rate(http_requests_total[5m])`,
			expected: []string{"http_requests_total"},
		},
		{
			name:     "complex expression",
			promQL:   `rate(http_requests_total{code=~"5.."}[5m]) > 0.1`,
			expected: []string{"http_requests_total"},
		},
		{
			name:     "multiple metrics",
			promQL:   `rate(requests_total[5m]) / rate(requests_success_total[5m])`,
			expected: []string{"requests_total", "requests_success_total"},
		},
		{
			name:     "histogram quantile",
			promQL:   `histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le))`,
			expected: []string{"http_request_duration_seconds_bucket"},
		},
		{
			name:     "sum by with labels",
			promQL:   `sum(container_cpu_usage_seconds_total{namespace="production"}) by (pod)`,
			expected: []string{"container_cpu_usage_seconds_total"},
		},
		{
			name:     "empty expression",
			promQL:   "",
			expected: nil,
		},
		{
			name:     "metric with colon",
			promQL:   `namespace:container_cpu_usage_seconds_total:sum_rate`,
			expected: []string{"namespace:container_cpu_usage_seconds_total:sum_rate"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := matcher.ExtractMetricNames(tc.promQL)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestAlertSignalMatcher_ExtractMetricNames_SkipsFunctions(t *testing.T) {
	logger := logging.GetLogger("test")
	matcher := NewAlertSignalMatcher(nil, "test-integration", logger)

	promQL := `sum(rate(http_requests_total[5m])) by (service)`
	result := matcher.ExtractMetricNames(promQL)

	// Should extract http_requests_total, not "sum", "rate", or "by"
	assert.Equal(t, []string{"http_requests_total"}, result)
}

func TestAlertSignalMatcher_ExtractMetricNames_Deduplicates(t *testing.T) {
	logger := logging.GetLogger("test")
	matcher := NewAlertSignalMatcher(nil, "test-integration", logger)

	// Same metric referenced multiple times
	promQL := `http_requests_total / http_requests_total`
	result := matcher.ExtractMetricNames(promQL)

	// Should only return the metric once
	assert.Equal(t, []string{"http_requests_total"}, result)
}
