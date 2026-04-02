package namespacegraph

import (
	"context"
	"testing"
	"time"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/graph"
	"github.com/stretchr/testify/require"
)

type stubAnalysisStore struct {
	namespaceGraph     *analysisstore.NamespaceGraphData
	namespaceGraphErr  error
	namespaceGraphCall int
	lastQuery          analysisstore.NamespaceGraphQuery
}

func (s *stubAnalysisStore) GetResource(context.Context, string) (*graph.ResourceIdentity, error) {
	return nil, nil
}

func (s *stubAnalysisStore) GetOwnershipChain(
	context.Context,
	string,
	int64,
	int,
) ([]analysisstore.ResourceWithDistance, error) {
	return nil, nil
}

func (s *stubAnalysisStore) GetManagers(
	context.Context,
	[]string,
	float64,
) (map[string]*analysisstore.ManagerData, error) {
	return nil, nil
}

func (s *stubAnalysisStore) GetRelatedResources(
	context.Context,
	[]string,
	analysisstore.ResourceWindow,
) (map[string][]analysisstore.RelatedResourceData, error) {
	return nil, nil
}

func (s *stubAnalysisStore) GetChangeEvents(
	context.Context,
	[]string,
	analysisstore.ResourceWindow,
) (map[string][]analysisstore.ChangeEventInfo, error) {
	return nil, nil
}

func (s *stubAnalysisStore) GetK8sEvents(
	context.Context,
	[]string,
	analysisstore.ResourceWindow,
) (map[string][]analysisstore.K8sEventInfo, error) {
	return nil, nil
}

func (s *stubAnalysisStore) GetNamespaceGraph(
	_ context.Context,
	query analysisstore.NamespaceGraphQuery,
) (*analysisstore.NamespaceGraphData, error) {
	s.namespaceGraphCall++
	s.lastQuery = query
	return s.namespaceGraph, s.namespaceGraphErr
}

func TestAnalyze_UsesStoreNamespaceGraphData(t *testing.T) {
	timestamp := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC).UnixNano()
	store := &stubAnalysisStore{
		namespaceGraph: &analysisstore.NamespaceGraphData{
			Graph: analysisstore.NamespaceGraph{
				Nodes: []analysisstore.NamespaceGraphNode{
					{
						UID:       "pod-uid",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "pod-a",
						Status:    StatusReady,
						LatestEvent: &analysisstore.NamespaceGraphChangeEvent{
							TimestampNs: timestamp,
							EventType:   "UPDATE",
							Status:      StatusReady,
						},
						Labels: map[string]string{"app": "demo"},
					},
				},
				Edges: []analysisstore.NamespaceGraphEdge{
					{
						ID:               "edge-1",
						Source:           "svc-uid",
						Target:           "pod-uid",
						RelationshipType: "SELECTS",
					},
				},
			},
			Metadata: analysisstore.NamespaceGraphMetadata{
				Namespace:   "default",
				TimestampNs: timestamp,
				NodeCount:   1,
				EdgeCount:   1,
				HasMore:     true,
				NextCursor:  "cursor-1",
			},
		},
	}

	analyzer := NewAnalyzer(store)

	response, err := analyzer.Analyze(context.Background(), AnalyzeInput{
		Namespace: "default",
		Timestamp: timestamp,
		Lookback:  15 * time.Minute,
		MaxDepth:  2,
		Limit:     25,
		Cursor:    "cursor-0",
	})
	require.NoError(t, err)

	require.Equal(t, 1, store.namespaceGraphCall)
	require.Equal(t, "default", store.lastQuery.Namespace)
	require.Equal(t, timestamp, store.lastQuery.TimestampNs)
	require.Equal(t, int64((15 * time.Minute).Nanoseconds()), store.lastQuery.LookbackNs)
	require.Equal(t, 2, store.lastQuery.MaxDepth)
	require.Equal(t, 25, store.lastQuery.Limit)
	require.Equal(t, "cursor-0", store.lastQuery.Cursor)

	require.Len(t, response.Graph.Nodes, 1)
	require.Equal(t, "pod-uid", response.Graph.Nodes[0].UID)
	require.Equal(t, "UPDATE", response.Graph.Nodes[0].LatestEvent.EventType)
	require.Equal(t, "cursor-1", response.Metadata.NextCursor)
	require.True(t, response.Metadata.HasMore)
}
