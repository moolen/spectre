package observatory

import (
	"fmt"
	"strings"
)

// ClassifyMetric classifies a metric into signal roles using layered heuristics.
// Layers are tried in order with decreasing confidence:
// 1. Hardcoded known metrics (0.95)
// 2. Query structure patterns (0.85-0.9) - requires queryCtx
// 3. Metric name patterns (0.7-0.8)
// 4. Panel title/description (0.5)
// 5. Unknown (0)
//
// Returns first matching classification, or Unknown if no match.
// Metrics containing ":relabel" are filtered out and return SignalUnknown with confidence 0.
func ClassifyMetric(metricName string, queryCtx QueryContext, panelTitle string) ClassificationResult {
	// Filter: Relabeling recording rules should be excluded from signal classification
	// These are intermediate metrics used for label manipulation, not observable signals
	if strings.Contains(metricName, ":relabel") {
		return ClassificationResult{
			Role:       SignalUnknown,
			Confidence: 0.0,
			Layer:      0,
			Reason:     "filtered: relabeling recording rule",
		}
	}

	// Layer 1: Hardcoded known metrics
	if result := classifyKnownMetric(metricName); result != nil {
		return *result
	}

	// Layer 2: Query structure patterns
	if queryCtx != nil {
		if result := classifyQueryStructure(metricName, queryCtx); result != nil {
			return *result
		}
	}

	// Layer 3: Metric name patterns
	if result := classifyMetricName(metricName); result != nil {
		return *result
	}

	// Layer 4: Panel title/description patterns
	if panelTitle != "" {
		if result := classifyPanelTitle(panelTitle); result != nil {
			return *result
		}
	}

	// Layer 5: Unknown
	return ClassificationResult{
		Role:       SignalUnknown,
		Confidence: 0.0,
		Layer:      5,
		Reason:     "no classification heuristic matched",
	}
}

// classifyKnownMetric matches known metrics from embedded curated metric definitions.
// Layer 1: High confidence based on curated metric database with exact name or pattern matching.
// Confidence values come from the curated data (typically 0.8-1.0).
func classifyKnownMetric(metricName string) *ClassificationResult {
	curated := LookupCuratedMetric(metricName)
	if curated == nil {
		return nil
	}

	role := curated.ToSignalRole()
	if role == SignalUnknown {
		return nil
	}

	matchType := "exact name"
	if curated.NamePattern != nil && *curated.NamePattern != "" {
		matchType = "pattern"
	}

	return &ClassificationResult{
		Role:       role,
		Confidence: curated.Confidence,
		Layer:      1,
		Reason:     fmt.Sprintf("matched curated metric (%s): %s", matchType, curated.Name),
	}
}

// classifyQueryStructure analyzes query structure for classification hints.
// Layer 2: High confidence (0.85-0.9) based on aggregation functions and patterns.
func classifyQueryStructure(metricName string, queryCtx QueryContext) *ClassificationResult {
	aggregations := queryCtx.GetAggregations()
	metricNames := queryCtx.GetMetricNames()

	// histogram_quantile(*_bucket) → Latency (0.9)
	if containsFunc(aggregations, "histogram_quantile") {
		return &ClassificationResult{
			Role:       SignalLatency,
			Confidence: 0.9,
			Layer:      2,
			Reason:     "histogram_quantile indicates latency measurement",
		}
	}

	// rate(*_total) or increase(*_total) with "error" in name → Errors (0.85)
	if containsFunc(aggregations, "rate") || containsFunc(aggregations, "increase") {
		for _, metric := range metricNames {
			lowerMetric := strings.ToLower(metric)
			if strings.Contains(lowerMetric, "error") || strings.Contains(lowerMetric, "failed") || strings.Contains(lowerMetric, "failure") {
				return &ClassificationResult{
					Role:       SignalErrors,
					Confidence: 0.85,
					Layer:      2,
					Reason:     "rate/increase on error metric",
				}
			}
		}

		// rate(*_total) with "request/query/call" in name → Traffic (0.85)
		for _, metric := range metricNames {
			lowerMetric := strings.ToLower(metric)
			if strings.Contains(lowerMetric, "request") || strings.Contains(lowerMetric, "query") || strings.Contains(lowerMetric, "call") {
				return &ClassificationResult{
					Role:       SignalTraffic,
					Confidence: 0.85,
					Layer:      2,
					Reason:     "rate/increase on request/query/call metric",
				}
			}
		}
	}

	return nil
}

// classifyMetricName matches patterns in metric names.
// Layer 3: Medium confidence (0.7-0.8) based on naming conventions.
func classifyMetricName(metricName string) *ClassificationResult {
	lowerName := strings.ToLower(metricName)

	// Latency patterns (0.8)
	latencyPatterns := []string{"_latency", "_duration", "_time", "response_time"}
	for _, pattern := range latencyPatterns {
		if strings.Contains(lowerName, pattern) {
			return &ClassificationResult{
				Role:       SignalLatency,
				Confidence: 0.8,
				Layer:      3,
				Reason:     fmt.Sprintf("metric name contains latency indicator: %s", pattern),
			}
		}
	}

	// Error patterns (0.75)
	errorPatterns := []string{"_error", "_failed", "_failure", "_fault"}
	for _, pattern := range errorPatterns {
		if strings.Contains(lowerName, pattern) {
			return &ClassificationResult{
				Role:       SignalErrors,
				Confidence: 0.75,
				Layer:      3,
				Reason:     fmt.Sprintf("metric name contains error indicator: %s", pattern),
			}
		}
	}

	// Traffic patterns (0.7) - only if not error and not resource-related
	trafficPatterns := []string{"_total", "_count"}
	for _, pattern := range trafficPatterns {
		if strings.Contains(lowerName, pattern) {
			// Make sure it's not an error metric
			if !strings.Contains(lowerName, "error") && !strings.Contains(lowerName, "failed") {
				return &ClassificationResult{
					Role:       SignalTraffic,
					Confidence: 0.7,
					Layer:      3,
					Reason:     fmt.Sprintf("metric name contains traffic indicator: %s", pattern),
				}
			}
		}
	}

	// Specific traffic pattern: _requests (but not resource_requests which is Saturation)
	if strings.Contains(lowerName, "_requests") && !strings.Contains(lowerName, "resource_requests") {
		if !strings.Contains(lowerName, "error") && !strings.Contains(lowerName, "failed") {
			return &ClassificationResult{
				Role:       SignalTraffic,
				Confidence: 0.7,
				Layer:      3,
				Reason:     "metric name contains traffic indicator: _requests",
			}
		}
	}

	// Size bytes patterns (0.7) - throughput/bandwidth indicators
	if strings.Contains(lowerName, "_size_bytes") || strings.Contains(lowerName, "_bytes_total") {
		return &ClassificationResult{
			Role:       SignalTraffic,
			Confidence: 0.7,
			Layer:      3,
			Reason:     "metric name contains size/bytes indicator for throughput",
		}
	}

	// Saturation patterns (0.75)
	saturationPatterns := []string{"_usage", "_utilization", "_used", "_capacity"}
	for _, pattern := range saturationPatterns {
		if strings.Contains(lowerName, pattern) {
			return &ClassificationResult{
				Role:       SignalSaturation,
				Confidence: 0.75,
				Layer:      3,
				Reason:     fmt.Sprintf("metric name contains saturation indicator: %s", pattern),
			}
		}
	}

	return nil
}

// classifyPanelTitle matches patterns in panel titles for fallback classification.
// Layer 4: Low confidence (0.5) based on human-written panel descriptions.
func classifyPanelTitle(panelTitle string) *ClassificationResult {
	lowerTitle := strings.ToLower(panelTitle)

	// Error patterns
	errorPhrases := []string{"error rate", "failures", "failed", "errors"}
	for _, phrase := range errorPhrases {
		if strings.Contains(lowerTitle, phrase) {
			return &ClassificationResult{
				Role:       SignalErrors,
				Confidence: 0.5,
				Layer:      4,
				Reason:     fmt.Sprintf("panel title contains error phrase: %s", phrase),
			}
		}
	}

	// Latency patterns
	latencyPhrases := []string{"latency", "response time", "duration", "p95", "p99"}
	for _, phrase := range latencyPhrases {
		if strings.Contains(lowerTitle, phrase) {
			return &ClassificationResult{
				Role:       SignalLatency,
				Confidence: 0.5,
				Layer:      4,
				Reason:     fmt.Sprintf("panel title contains latency phrase: %s", phrase),
			}
		}
	}

	// Traffic patterns
	trafficPhrases := []string{"qps", "throughput", "requests", "rps", "traffic"}
	for _, phrase := range trafficPhrases {
		if strings.Contains(lowerTitle, phrase) {
			return &ClassificationResult{
				Role:       SignalTraffic,
				Confidence: 0.5,
				Layer:      4,
				Reason:     fmt.Sprintf("panel title contains traffic phrase: %s", phrase),
			}
		}
	}

	// Saturation patterns
	saturationPhrases := []string{"cpu", "memory", "disk", "saturation", "utilization"}
	for _, phrase := range saturationPhrases {
		if strings.Contains(lowerTitle, phrase) {
			return &ClassificationResult{
				Role:       SignalSaturation,
				Confidence: 0.5,
				Layer:      4,
				Reason:     fmt.Sprintf("panel title contains saturation phrase: %s", phrase),
			}
		}
	}

	// Availability patterns
	availabilityPhrases := []string{"uptime", "availability", "health", "status"}
	for _, phrase := range availabilityPhrases {
		if strings.Contains(lowerTitle, phrase) {
			return &ClassificationResult{
				Role:       SignalAvailability,
				Confidence: 0.5,
				Layer:      4,
				Reason:     fmt.Sprintf("panel title contains availability phrase: %s", phrase),
			}
		}
	}

	return nil
}

// containsFunc checks if a slice contains a specific string (case-sensitive).
func containsFunc(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
