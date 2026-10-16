package namespacegraph

import (
	"context"
	"testing"
	"time"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/require"
)

// echoAnalysisStore returns a graph describing the query it was asked.
type echoAnalysisStore struct {
	stubAnalysisStore
}

func (s *echoAnalysisStore) GetNamespaceGraph(_ context.Context, query analysisstore.NamespaceGraphQuery) (*analysisstore.NamespaceGraphData, error) {
	s.namespaceGraphCall++
	return &analysisstore.NamespaceGraphData{
		Graph: analysisstore.NamespaceGraph{Nodes: []analysisstore.NamespaceGraphNode{{UID: query.Cursor + "-node", Kind: "ConfigMap"}}},
		Metadata: analysisstore.NamespaceGraphMetadata{
			Namespace: query.Namespace, TimestampNs: query.TimestampNs, NodeCount: 1,
			HasMore: true, NextCursor: query.Cursor + "+",
		},
	}, nil
}

func newTestCache(store analysisstore.AnalysisStore) *Cache {
	return NewCache(CacheConfig{RefreshTTL: time.Minute}, NewAnalyzer(store), logging.GetLogger("test"))
}

func TestCacheServesOnlyMatchingRequests(t *testing.T) {
	store := &echoAnalysisStore{}
	cache := newTestCache(store)
	ctx := context.Background()
	// Requests reach the cache normalized: "now" is rounded to its timestamp bucket.
	now := normalizeTimestamp(time.Now().UnixNano(), time.Now().UnixNano())
	base := AnalyzeInput{Namespace: "prod", Timestamp: now, Limit: 50, MaxDepth: 1, Lookback: 10 * time.Minute}

	first, err := cache.Analyze(ctx, base)
	require.NoError(t, err)
	require.False(t, first.Metadata.Cached)

	again, err := cache.Analyze(ctx, base)
	require.NoError(t, err)
	require.True(t, again.Metadata.Cached, "an identical request is served from the cache")
	require.Equal(t, 1, store.namespaceGraphCall)

	// Each of these asks for a different page, point in time or shape and must not get
	// the first response back.
	next := base
	next.Cursor = first.Metadata.NextCursor
	historical := base
	historical.Timestamp = now - int64(6*time.Hour)
	bigger := base
	bigger.Limit = 500
	for name, input := range map[string]AnalyzeInput{"next page": next, "historical": historical, "limit": bigger} {
		result, err := cache.Analyze(ctx, input)
		require.NoError(t, err, name)
		require.False(t, result.Metadata.Cached, name)
		require.Equal(t, input.Timestamp, result.Metadata.Timestamp, name)
		require.Equal(t, input.Cursor+"+", result.Metadata.NextCursor, name)
	}
}

func TestCacheInvalidationRecomputes(t *testing.T) {
	store := &echoAnalysisStore{}
	cache := newTestCache(store)
	ctx := context.Background()
	input := AnalyzeInput{Namespace: "prod", Timestamp: time.Now().UnixNano(), Limit: 50, MaxDepth: 1}

	_, err := cache.Analyze(ctx, input)
	require.NoError(t, err)
	other := input
	other.Namespace = "staging"
	_, err = cache.Analyze(ctx, other)
	require.NoError(t, err)
	cache.InvalidateNamespaces([]string{"prod"})
	result, err := cache.Analyze(ctx, input)
	require.NoError(t, err)
	require.False(t, result.Metadata.Cached)
	result, err = cache.Analyze(ctx, input)
	require.NoError(t, err)
	require.True(t, result.Metadata.Cached)
	result, err = cache.Analyze(ctx, other)
	require.NoError(t, err)
	require.True(t, result.Metadata.Cached, "other namespaces stay cached")
	require.Equal(t, 3, store.namespaceGraphCall)
}
