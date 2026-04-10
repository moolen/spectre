package graph

import (
	"context"
	"testing"
	"time"

	namespacegraph "github.com/moolen/spectre/internal/analysis/namespace_graph"
	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	graphmodel "github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/require"
)

type stubAnalysisStore struct {
	namespaceGraph     *analysisstore.NamespaceGraphData
	namespaceGraphCall int
	lastQuery          analysisstore.NamespaceGraphQuery
}

func (s *stubAnalysisStore) GetResource(context.Context, string) (*graphmodel.ResourceIdentity, error) {
	return nil, nil
}

func (s *stubAnalysisStore) GetOwnershipChain(context.Context, string, int64, int) ([]analysisstore.ResourceWithDistance, error) {
	return nil, nil
}

func (s *stubAnalysisStore) GetManagers(context.Context, []string, float64) (map[string]*analysisstore.ManagerData, error) {
	return nil, nil
}

func (s *stubAnalysisStore) GetRelatedResources(context.Context, []string, analysisstore.ResourceWindow) (map[string][]analysisstore.RelatedResourceData, error) {
	return nil, nil
}

func (s *stubAnalysisStore) GetChangeEvents(context.Context, []string, analysisstore.ResourceWindow) (map[string][]analysisstore.ChangeEventInfo, error) {
	return nil, nil
}

func (s *stubAnalysisStore) GetK8sEvents(context.Context, []string, analysisstore.ResourceWindow) (map[string][]analysisstore.K8sEventInfo, error) {
	return nil, nil
}

func (s *stubAnalysisStore) GetNamespaceGraph(_ context.Context, query analysisstore.NamespaceGraphQuery) (*analysisstore.NamespaceGraphData, error) {
	s.namespaceGraphCall++
	s.lastQuery = query
	return s.namespaceGraph, nil
}

func TestService_AnalyzeNamespaceGraph_UsesStore(t *testing.T) {
	timestamp := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC).UnixNano()
	store := &stubAnalysisStore{
		namespaceGraph: &analysisstore.NamespaceGraphData{
			Graph: analysisstore.NamespaceGraph{
				Nodes: []analysisstore.NamespaceGraphNode{
					{
						UID:       "deployment-uid",
						Kind:      "Deployment",
						Namespace: "default",
						Name:      "web",
						Status:    "ready",
					},
				},
			},
			Metadata: analysisstore.NamespaceGraphMetadata{
				Namespace:   "default",
				TimestampNs: timestamp,
				NodeCount:   1,
			},
		},
	}

	service := NewService(store, logging.GetLogger("test"), nil)

	response, err := service.AnalyzeNamespaceGraph(context.Background(), namespacegraph.AnalyzeInput{
		Namespace: "default",
		Timestamp: timestamp,
		Lookback:  5 * time.Minute,
	})
	require.NoError(t, err)
	require.Equal(t, 1, store.namespaceGraphCall)
	require.Equal(t, "default", response.Metadata.Namespace)
	require.Len(t, response.Graph.Nodes, 1)
	require.Equal(t, "Deployment", response.Graph.Nodes[0].Kind)
}
