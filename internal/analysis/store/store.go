package store

import (
	"context"

	"github.com/moolen/spectre/internal/graph"
)

// AnalysisStore defines backend-neutral domain queries used by analysis pipelines.
type AnalysisStore interface {
	GetResource(ctx context.Context, uid string) (*graph.ResourceIdentity, error)
	GetOwnershipChain(ctx context.Context, uid string, atNs int64, maxDepth int) ([]ResourceWithDistance, error)
	GetManagers(ctx context.Context, resourceUIDs []string, minConfidence float64) (map[string]*ManagerData, error)
	GetRelatedResources(ctx context.Context, resourceUIDs []string, window ResourceWindow) (map[string][]RelatedResourceData, error)
	GetChangeEvents(ctx context.Context, resourceUIDs []string, window ResourceWindow) (map[string][]ChangeEventInfo, error)
	GetK8sEvents(ctx context.Context, resourceUIDs []string, window ResourceWindow) (map[string][]K8sEventInfo, error)
	GetNamespaceGraph(ctx context.Context, input NamespaceGraphQuery) (*NamespaceGraphData, error)
}

// ResourceWindow defines an incident-relative time window.
type ResourceWindow struct {
	FailureTimestamp int64
	LookbackNs       int64
}

// Start returns the inclusive start timestamp for the window.
func (w ResourceWindow) Start() int64 {
	return w.FailureTimestamp - w.LookbackNs
}
