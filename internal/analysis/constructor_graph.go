package analysis

import (
	analysisfalkor "github.com/moolen/spectre/internal/analysis/store/falkor"
	"github.com/moolen/spectre/internal/graph"
)

// NewRootCauseAnalyzerFromGraphClient preserves the existing graph-client entrypoint
// while routing reads through the Falkor-backed analysis store adapter.
func NewRootCauseAnalyzerFromGraphClient(graphClient graph.Client) *RootCauseAnalyzer {
	return NewRootCauseAnalyzer(analysisfalkor.New(graphClient))
}
