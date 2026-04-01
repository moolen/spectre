package falkor

import (
	"context"
	"testing"

	"github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/graph"
)

// compile-time contract assertion for the backend-neutral store interface.
var _ store.AnalysisStore = (*contractProbe)(nil)

func TestAnalysisStoreContract(t *testing.T) {
	t.Parallel()
}

func TestResourceWindow(t *testing.T) {
	t.Parallel()

	window := store.ResourceWindow{
		FailureTimestampNs: 1_000,
		LookbackNs:       150,
	}

	if got, want := window.Start(), int64(850); got != want {
		t.Fatalf("unexpected window start: got %d want %d", got, want)
	}
}

func TestResourceWindowStartClampsToZero(t *testing.T) {
	t.Parallel()

	window := store.ResourceWindow{
		FailureTimestampNs: 5,
		LookbackNs:         10,
	}

	if got, want := window.Start(), int64(0); got != want {
		t.Fatalf("expected clamped zero start: got %d want %d", got, want)
	}
}

type contractProbe struct{}

func (c *contractProbe) GetResource(ctx context.Context, uid string) (*graph.ResourceIdentity, error) {
	return nil, nil
}

func (c *contractProbe) GetOwnershipChain(ctx context.Context, uid string, atNs int64, maxDepth int) ([]store.ResourceWithDistance, error) {
	return nil, nil
}

func (c *contractProbe) GetManagers(ctx context.Context, resourceUIDs []string, minConfidence float64) (map[string]*store.ManagerData, error) {
	return nil, nil
}

func (c *contractProbe) GetRelatedResources(ctx context.Context, resourceUIDs []string, window store.ResourceWindow) (map[string][]store.RelatedResourceData, error) {
	return nil, nil
}

func (c *contractProbe) GetChangeEvents(ctx context.Context, resourceUIDs []string, window store.ResourceWindow) (map[string][]store.ChangeEventInfo, error) {
	return nil, nil
}

func (c *contractProbe) GetK8sEvents(ctx context.Context, resourceUIDs []string, window store.ResourceWindow) (map[string][]store.K8sEventInfo, error) {
	return nil, nil
}

func (c *contractProbe) GetNamespaceGraph(ctx context.Context, input store.NamespaceGraphQuery) (*store.NamespaceGraphData, error) {
	return nil, nil
}
