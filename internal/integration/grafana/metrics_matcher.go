package grafana

import (
	"strings"

	"github.com/moolen/spectre/internal/observatory"
)

// MatchResult holds the details of a metric match between Grafana and curated metrics.
type MatchResult struct {
	// GrafanaMetric is the actual metric name in Grafana (may include prefix)
	GrafanaMetric string

	// CuratedMetric is the matched curated metric definition
	CuratedMetric *observatory.CuratedMetric

	// MatchType indicates how the match was found: "exact" or "suffix"
	MatchType string
}

// MatchMetricsToCurated finds all Grafana metrics that match curated definitions.
// Returns one result per matched Grafana metric.
//
// Matching strategy:
// 1. Exact match: Grafana metric name equals curated metric name
// 2. Suffix match: Grafana metric ends with "_" + curated name or ":" + curated name
//    (handles prefixed metrics like "mycompany_container_cpu_usage_seconds_total")
//
// When multiple curated metrics could suffix-match, the longest match wins
// to avoid false positives from shorter, more generic metric names.
func MatchMetricsToCurated(grafanaMetrics []string) []MatchResult {
	curatedMetrics := observatory.GetAllCuratedMetrics() // returns []*CuratedMetric
	if curatedMetrics == nil {
		return nil
	}

	// Build lookup map for O(1) exact matches
	curatedByName := make(map[string]*observatory.CuratedMetric)
	for _, cm := range curatedMetrics {
		if cm.Name != "" {
			curatedByName[cm.Name] = cm
		}
	}

	var results []MatchResult

	for _, gm := range grafanaMetrics {
		// 1. Exact match (highest priority)
		if curated, ok := curatedByName[gm]; ok {
			results = append(results, MatchResult{
				GrafanaMetric: gm,
				CuratedMetric: curated,
				MatchType:     "exact",
			})
			continue
		}

		// 2. Suffix match - prefer longest matching curated name
		var bestMatch *observatory.CuratedMetric
		var bestMatchLen int

		for _, curated := range curatedMetrics {
			if curated.Name == "" {
				continue
			}

			// Check for prefix separator: "_" or ":"
			// Examples:
			// - "mycompany_container_cpu_usage_seconds_total" matches "container_cpu_usage_seconds_total"
			// - "prefix:http_requests_total" matches "http_requests_total"
			if strings.HasSuffix(gm, "_"+curated.Name) ||
				strings.HasSuffix(gm, ":"+curated.Name) {
				if len(curated.Name) > bestMatchLen {
					bestMatch = curated
					bestMatchLen = len(curated.Name)
				}
			}
		}

		if bestMatch != nil {
			results = append(results, MatchResult{
				GrafanaMetric: gm,
				CuratedMetric: bestMatch,
				MatchType:     "suffix",
			})
		}
	}

	return results
}

// MatchStats provides statistics about the matching process.
type MatchStats struct {
	TotalGrafanaMetrics int
	TotalMatched        int
	ExactMatches        int
	SuffixMatches       int
}

// ComputeMatchStats calculates statistics from match results.
func ComputeMatchStats(grafanaMetrics []string, results []MatchResult) MatchStats {
	stats := MatchStats{
		TotalGrafanaMetrics: len(grafanaMetrics),
		TotalMatched:        len(results),
	}

	for _, r := range results {
		switch r.MatchType {
		case "exact":
			stats.ExactMatches++
		case "suffix":
			stats.SuffixMatches++
		}
	}

	return stats
}
