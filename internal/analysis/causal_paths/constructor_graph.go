package causalpaths

import (
	analysisfalkor "github.com/moolen/spectre/internal/analysis/store/falkor"
	"github.com/moolen/spectre/internal/graph"
)

// NewPathDiscovererFromGraphClient preserves graph-backed construction while
// routing analysis reads through the Falkor store adapter.
func NewPathDiscovererFromGraphClient(graphClient graph.Client) *PathDiscoverer {
	return NewPathDiscoverer(analysisfalkor.New(graphClient))
}
