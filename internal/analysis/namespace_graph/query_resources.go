package namespacegraph

import (
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// ResourceFetcher handles querying resources from the graph database
type ResourceFetcher struct {
	graphClient graph.Client
	logger      *logging.Logger
}

// NewResourceFetcher creates a new ResourceFetcher
func NewResourceFetcher(graphClient graph.Client) *ResourceFetcher {
	return &ResourceFetcher{
		graphClient: graphClient,
		logger:      logging.GetLogger("namespacegraph.fetcher"),
	}
}
