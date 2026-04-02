package handlers

import (
	"context"
	"testing"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/require"
)

type stubAnalysisStore struct{}

func (stubAnalysisStore) GetResource(context.Context, string) (*graph.ResourceIdentity, error) {
	return nil, nil
}

func (stubAnalysisStore) GetOwnershipChain(
	context.Context,
	string,
	int64,
	int,
) ([]analysisstore.ResourceWithDistance, error) {
	return nil, nil
}

func (stubAnalysisStore) GetManagers(
	context.Context,
	[]string,
	float64,
) (map[string]*analysisstore.ManagerData, error) {
	return nil, nil
}

func (stubAnalysisStore) GetRelatedResources(
	context.Context,
	[]string,
	analysisstore.ResourceWindow,
) (map[string][]analysisstore.RelatedResourceData, error) {
	return nil, nil
}

func (stubAnalysisStore) GetChangeEvents(
	context.Context,
	[]string,
	analysisstore.ResourceWindow,
) (map[string][]analysisstore.ChangeEventInfo, error) {
	return nil, nil
}

func (stubAnalysisStore) GetK8sEvents(
	context.Context,
	[]string,
	analysisstore.ResourceWindow,
) (map[string][]analysisstore.K8sEventInfo, error) {
	return nil, nil
}

func (stubAnalysisStore) GetNamespaceGraph(
	context.Context,
	analysisstore.NamespaceGraphQuery,
) (*analysisstore.NamespaceGraphData, error) {
	return nil, nil
}

func TestNewCausalGraphHandler_UsesStore(t *testing.T) {
	handler := NewCausalGraphHandler(stubAnalysisStore{}, logging.GetLogger("test"), nil)
	require.NotNil(t, handler)
}
