package falkor

import (
	"context"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/analysis/store"
)

func (s *Store) GetNamespaceGraph(ctx context.Context, input store.NamespaceGraphQuery) (*store.NamespaceGraphData, error) {
	startTime := time.Now()

	normalized := input
	if normalized.Limit <= 0 {
		normalized.Limit = defaultLimit
	}
	if normalized.Limit > maxLimit {
		normalized.Limit = maxLimit
	}
	if normalized.MaxDepth <= 0 {
		normalized.MaxDepth = defaultMaxDepth
	}
	if normalized.MaxDepth > maxMaxDepth {
		normalized.MaxDepth = maxMaxDepth
	}
	if normalized.LookbackNs <= 0 {
		normalized.LookbackNs = defaultLookbackNs
	}
	if normalized.LookbackNs > maxLookbackNs {
		normalized.LookbackNs = maxLookbackNs
	}

	namespacedResources, hasMore, nextCursor, err := s.fetchNamespacedResources(
		ctx, normalized.Namespace, normalized.TimestampNs, normalized.Limit, normalized.Cursor,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch namespaced resources: %w", err)
	}

	namespacedUIDs := make([]string, len(namespacedResources))
	for i, resource := range namespacedResources {
		namespacedUIDs[i] = resource.UID
	}

	clusterScopedResources, err := s.fetchClusterScopedResources(
		ctx, namespacedUIDs, normalized.TimestampNs, normalized.MaxDepth,
	)
	if err != nil {
		clusterScopedResources = nil
	}

	allResources := append(namespacedResources, clusterScopedResources...)
	allUIDs := make([]string, len(allResources))
	for i, resource := range allResources {
		allUIDs[i] = resource.UID
	}

	latestEvents, err := s.fetchLatestEvents(ctx, allUIDs, normalized.TimestampNs)
	if err != nil {
		latestEvents = make(map[string]*store.NamespaceGraphChangeEvent)
	}

	specChanges, err := s.fetchSpecChanges(ctx, allUIDs, normalized.TimestampNs, normalized.LookbackNs)
	if err == nil {
		for uid, specChange := range specChanges {
			event, ok := latestEvents[uid]
			if !ok {
				continue
			}
			diffs, diffErr := computeJSONDiff(specChange.EarliestData, specChange.LatestData)
			if diffErr != nil {
				continue
			}
			diffs = filterSpecOnly(diffs)
			if len(diffs) > 0 {
				event.SpecChanges = formatUnifiedDiff(diffs)
			}
		}
	}

	edgeResults, err := s.fetchRelationships(ctx, allUIDs)
	if err != nil {
		edgeResults = nil
	}

	nodes := s.buildNamespaceGraphNodes(allResources, latestEvents)
	edges := s.buildNamespaceGraphEdges(edgeResults)

	return &store.NamespaceGraphData{
		Graph: store.NamespaceGraph{
			Nodes: nodes,
			Edges: edges,
		},
		Metadata: store.NamespaceGraphMetadata{
			Namespace:        normalized.Namespace,
			TimestampNs:      normalized.TimestampNs,
			NodeCount:        len(nodes),
			EdgeCount:        len(edges),
			QueryExecutionMs: time.Since(startTime).Milliseconds(),
			HasMore:          hasMore,
			NextCursor:       nextCursor,
			Cached:           false,
			CacheAgeMs:       0,
		},
	}, nil
}
