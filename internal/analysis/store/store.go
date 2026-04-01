package store

import (
	"context"

	"github.com/moolen/spectre/internal/graph"
)

// AnalysisStore defines backend-neutral domain queries used by analysis pipelines.
// graph.ResourceIdentity is intentionally reused at this boundary as the
// canonical domain type for Kubernetes resource identity.
//
// For map-returning methods, implementations may omit keys for requested UIDs
// that have no matching data.
type AnalysisStore interface {
	GetResource(ctx context.Context, uid string) (*graph.ResourceIdentity, error)
	// atTimestampNs is the evaluation point in Unix nanoseconds.
	GetOwnershipChain(ctx context.Context, uid string, atTimestampNs int64, maxDepth int) ([]ResourceWithDistance, error)
	GetManagers(ctx context.Context, resourceUIDs []string, minConfidence float64) (map[string]*ManagerData, error)
	GetRelatedResources(ctx context.Context, resourceUIDs []string, window ResourceWindow) (map[string][]RelatedResourceData, error)
	GetChangeEvents(ctx context.Context, resourceUIDs []string, window ResourceWindow) (map[string][]ChangeEventInfo, error)
	GetK8sEvents(ctx context.Context, resourceUIDs []string, window ResourceWindow) (map[string][]K8sEventInfo, error)
	GetNamespaceGraph(ctx context.Context, input NamespaceGraphQuery) (*NamespaceGraphData, error)
}

// ResourceWindow defines an incident-relative time window.
type ResourceWindow struct {
	// FailureTimestampNs is the incident timestamp in Unix nanoseconds.
	FailureTimestampNs int64
	// LookbackNs is the lookback duration in nanoseconds.
	LookbackNs int64
}

// Start returns the inclusive start timestamp for the window in Unix nanoseconds.
// If FailureTimestampNs-LookbackNs is negative, Start clamps the value to 0.
func (w ResourceWindow) Start() int64 {
	start := w.FailureTimestampNs - w.LookbackNs
	if start < 0 {
		return 0
	}
	return start
}
