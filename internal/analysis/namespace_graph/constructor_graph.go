package namespacegraph

import (
	analysisfalkor "github.com/moolen/spectre/internal/analysis/store/falkor"
	"github.com/moolen/spectre/internal/graph"
)

// NewAnalyzerFromGraphClient preserves graph-backed construction while routing
// namespace graph reads through the Falkor store adapter.
func NewAnalyzerFromGraphClient(graphClient graph.Client) *Analyzer {
	return NewAnalyzer(analysisfalkor.New(graphClient))
}
