package falkor

import (
	"context"
	"testing"

	"github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/graph"
)

func TestAnalysisStoreContract(t *testing.T) {
	t.Parallel()

	var candidate store.AnalysisStore = (*contractProbe)(nil)
	if candidate == nil {
		t.Fatal("expected contract probe to satisfy analysis store interface")
	}
}

func TestResourceWindow(t *testing.T) {
	t.Parallel()

	window := store.ResourceWindow{
		FailureTimestamp: 1_000,
		LookbackNs:       150,
	}

	if got, want := window.Start(), int64(850); got != want {
		t.Fatalf("unexpected window start: got %d want %d", got, want)
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
