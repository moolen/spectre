package grafana

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchMetricsToCurated_ExactMatch(t *testing.T) {
	// Use a metric we know is in the curated list
	grafanaMetrics := []string{
		"container_cpu_usage_seconds_total",
		"unknown_metric_name",
	}

	results := MatchMetricsToCurated(grafanaMetrics)

	// Should match the known curated metric
	var foundExact bool
	for _, r := range results {
		if r.GrafanaMetric == "container_cpu_usage_seconds_total" {
			foundExact = true
			assert.Equal(t, "exact", r.MatchType)
			assert.NotNil(t, r.CuratedMetric)
		}
	}
	assert.True(t, foundExact, "should find exact match for container_cpu_usage_seconds_total")

	// Should not match unknown metric
	for _, r := range results {
		assert.NotEqual(t, "unknown_metric_name", r.GrafanaMetric)
	}
}

func TestMatchMetricsToCurated_SuffixMatch(t *testing.T) {
	// Test suffix matching with prefixed metrics
	grafanaMetrics := []string{
		"mycompany_container_cpu_usage_seconds_total",
		"prefix:http_requests_total",
	}

	results := MatchMetricsToCurated(grafanaMetrics)

	// Find the prefixed container metric
	var foundSuffix bool
	for _, r := range results {
		if r.GrafanaMetric == "mycompany_container_cpu_usage_seconds_total" {
			foundSuffix = true
			assert.Equal(t, "suffix", r.MatchType)
			assert.NotNil(t, r.CuratedMetric)
			assert.Equal(t, "container_cpu_usage_seconds_total", r.CuratedMetric.Name)
		}
	}
	assert.True(t, foundSuffix, "should find suffix match for prefixed metric")
}

func TestMatchMetricsToCurated_LongestMatchWins(t *testing.T) {
	// Test that when multiple curated metrics could match,
	// the longest one wins
	grafanaMetrics := []string{
		"mycompany_node_cpu_seconds_total",
	}

	results := MatchMetricsToCurated(grafanaMetrics)

	// Should prefer longer match
	for _, r := range results {
		if r.GrafanaMetric == "mycompany_node_cpu_seconds_total" {
			// Should match "node_cpu_seconds_total" not just "cpu_seconds_total"
			assert.Equal(t, "node_cpu_seconds_total", r.CuratedMetric.Name)
		}
	}
}

func TestMatchMetricsToCurated_EmptyInput(t *testing.T) {
	results := MatchMetricsToCurated(nil)
	assert.Nil(t, results)

	results = MatchMetricsToCurated([]string{})
	assert.Empty(t, results)
}

func TestMatchMetricsToCurated_NoMatches(t *testing.T) {
	grafanaMetrics := []string{
		"completely_unknown_metric_xyz",
		"another_random_metric_abc",
	}

	results := MatchMetricsToCurated(grafanaMetrics)
	assert.Empty(t, results)
}

func TestComputeMatchStats(t *testing.T) {
	grafanaMetrics := []string{
		"container_cpu_usage_seconds_total",
		"mycompany_node_memory_MemTotal_bytes",
		"unknown_metric",
	}

	results := MatchMetricsToCurated(grafanaMetrics)
	stats := ComputeMatchStats(grafanaMetrics, results)

	assert.Equal(t, 3, stats.TotalGrafanaMetrics)
	require.GreaterOrEqual(t, stats.TotalMatched, 1, "should match at least one metric")
	assert.Equal(t, stats.ExactMatches+stats.SuffixMatches, stats.TotalMatched)
}

func TestMatchMetricsToCurated_KnownCuratedMetrics(t *testing.T) {
	// Test against known curated metrics to ensure they're loaded
	knownMetrics := []string{
		"up",
		"kube_pod_status_phase",
		"container_memory_usage_bytes",
		"node_cpu_seconds_total",
		"go_goroutines",
	}

	results := MatchMetricsToCurated(knownMetrics)

	// Should match most of these
	assert.GreaterOrEqual(t, len(results), 3, "should match at least 3 known metrics")

	for _, r := range results {
		assert.Equal(t, "exact", r.MatchType, "known metrics should be exact matches")
		assert.NotNil(t, r.CuratedMetric)
		assert.NotEmpty(t, r.CuratedMetric.SignalRole)
	}
}
