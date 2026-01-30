package grafana

import (
	"github.com/moolen/spectre/internal/observatory"
)

// ClassifyMetric classifies a metric into signal roles using layered heuristics.
// This is a wrapper around observatory.ClassifyMetric that provides backward compatibility.
//
// Layers are tried in order with decreasing confidence:
// 1. Hardcoded known metrics (0.95)
// 2. PromQL structure patterns (0.85-0.9)
// 3. Metric name patterns (0.7-0.8)
// 4. Panel title/description (0.5)
// 5. Unknown (0)
//
// Returns first matching classification, or Unknown if no match.
// Metrics containing ":relabel" are filtered out and return SignalUnknown with confidence 0.
func ClassifyMetric(metricName string, extraction *QueryExtraction, panelTitle string) ClassificationResult {
	// QueryExtraction implements observatory.QueryContext
	var queryCtx observatory.QueryContext
	if extraction != nil {
		queryCtx = extraction
	}

	// Call observatory's classifier
	result := observatory.ClassifyMetric(metricName, queryCtx, panelTitle)

	// Convert observatory.ClassificationResult to grafana.ClassificationResult
	return ClassificationResult{
		Role:       SignalRole(result.Role),
		Confidence: result.Confidence,
		Layer:      result.Layer,
		Reason:     result.Reason,
	}
}
