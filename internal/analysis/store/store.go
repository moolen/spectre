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
	// Implementations may return best-effort topology (without strict point-in-time filtering)
	// to preserve existing analyzer behavior.
	GetOwnershipChain(ctx context.Context, uid string, atTimestampNs int64, maxDepth int) ([]ResourceWithDistance, error)
	GetManagers(ctx context.Context, resourceUIDs []string, minConfidence float64) (map[string]*ManagerData, error)
	GetRelatedResources(ctx context.Context, resourceUIDs []string, window ResourceWindow) (map[string][]RelatedResourceData, error)
	GetChangeEvents(ctx context.Context, resourceUIDs []string, window ResourceWindow) (map[string][]ChangeEventInfo, error)
	GetK8sEvents(ctx context.Context, resourceUIDs []string, window ResourceWindow) (map[string][]K8sEventInfo, error)
	// Namespace graph retrieval is best-effort: implementations may degrade gracefully
	// (for example, partial enrichment) while preserving current analyzer semantics.
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
// It intentionally uses raw subtraction to preserve existing analyzer semantics.
func (w ResourceWindow) Start() int64 {
	return w.FailureTimestampNs - w.LookbackNs
}
