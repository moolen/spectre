package anomaly

import (
	analysisfalkor "github.com/moolen/spectre/internal/analysis/store/falkor"
	"github.com/moolen/spectre/internal/graph"
)

// NewDetectorFromGraphClient preserves graph-backed construction while routing
// analysis reads through the Falkor store adapter.
func NewDetectorFromGraphClient(graphClient graph.Client) *AnomalyDetector {
	return NewDetector(analysisfalkor.New(graphClient))
}
