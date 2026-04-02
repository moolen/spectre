package analysis

import (
	"context"
	"testing"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/graph"
	"github.com/stretchr/testify/require"
)

type ownershipSpyStore struct {
	lastUID       string
	lastTimestamp int64
	lastMaxDepth  int
}

func (s *ownershipSpyStore) GetResource(context.Context, string) (*graph.ResourceIdentity, error) {
	return nil, nil
}

func (s *ownershipSpyStore) GetOwnershipChain(
	_ context.Context,
	uid string,
	atTimestampNs int64,
	maxDepth int,
) ([]analysisstore.ResourceWithDistance, error) {
	s.lastUID = uid
	s.lastTimestamp = atTimestampNs
	s.lastMaxDepth = maxDepth
	return nil, nil
}

func (s *ownershipSpyStore) GetManagers(
	context.Context,
	[]string,
	float64,
) (map[string]*analysisstore.ManagerData, error) {
	return nil, nil
}

func (s *ownershipSpyStore) GetRelatedResources(
	context.Context,
	[]string,
	analysisstore.ResourceWindow,
) (map[string][]analysisstore.RelatedResourceData, error) {
	return nil, nil
}

func (s *ownershipSpyStore) GetChangeEvents(
	context.Context,
	[]string,
	analysisstore.ResourceWindow,
) (map[string][]analysisstore.ChangeEventInfo, error) {
	return nil, nil
}

func (s *ownershipSpyStore) GetK8sEvents(
	context.Context,
	[]string,
	analysisstore.ResourceWindow,
) (map[string][]analysisstore.K8sEventInfo, error) {
	return nil, nil
}

func (s *ownershipSpyStore) GetNamespaceGraph(
	context.Context,
	analysisstore.NamespaceGraphQuery,
) (*analysisstore.NamespaceGraphData, error) {
	return nil, nil
}

func TestGetOwnershipChain_UsesRequestedTimestampAndDepth(t *testing.T) {
	store := &ownershipSpyStore{}
	analyzer := NewRootCauseAnalyzer(store)

	_, err := analyzer.getOwnershipChain(context.Background(), "pod-uid", 123456789, 7)
	require.NoError(t, err)
	require.Equal(t, "pod-uid", store.lastUID)
	require.Equal(t, int64(123456789), store.lastTimestamp)
	require.Equal(t, 7, store.lastMaxDepth)
}

func TestGetOwnershipChain_DefaultsMaxDepthWhenUnset(t *testing.T) {
	store := &ownershipSpyStore{}
	analyzer := NewRootCauseAnalyzer(store)

	_, err := analyzer.getOwnershipChain(context.Background(), "pod-uid", 123456789, 0)
	require.NoError(t, err)
	require.Equal(t, MaxOwnershipDepth, store.lastMaxDepth)
}
