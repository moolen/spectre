package embeddedstore

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestQueryPlanner_MergesHotAndColdTimeRangeResults(t *testing.T) {
	resource := models.ResourceMetadata{
		UID:       "pod-1",
		Namespace: "default",
		Kind:      "Pod",
		Name:      "pod-1",
	}
	engine := newTestEngineWithColdSegment(t,
		[]models.Event{{
			ID:        "cold-1",
			Timestamp: 10,
			Resource:  resource,
		}},
		[]models.Event{{
			ID:        "hot-1",
			Timestamp: 20,
			Resource:  resource,
		}},
	)

	engine.projection.mu.Lock()
	engine.projection.eventsByResourceUID["pod-1"] = nil
	engine.projection.mu.Unlock()

	result, err := engine.QueryExecutor().Execute(context.Background(), &models.QueryRequest{
		StartTimestamp: 0,
		EndTimestamp:   30,
		Filters:        models.QueryFilters{},
	})
	require.NoError(t, err)
	require.Len(t, result.Events, 2)
	require.Equal(t, []string{"cold-1", "hot-1"}, []string{result.Events[0].ID, result.Events[1].ID})
}

func TestQueryPlanner_PrunesColdSegmentsOutsideRequestedTimeRange(t *testing.T) {
	engine := newTestEngineWithColdSegment(t,
		[]models.Event{{
			ID:        "cold-old",
			Timestamp: 10,
			Resource: models.ResourceMetadata{
				UID:       "pod-1",
				Namespace: "default",
				Kind:      "Pod",
				Name:      "pod-1",
			},
		}},
		[]models.Event{{
			ID:        "hot-new",
			Timestamp: 40,
			Resource: models.ResourceMetadata{
				UID:       "pod-1",
				Namespace: "default",
				Kind:      "Pod",
				Name:      "pod-1",
			},
		}},
	)

	planner := engine.QueryExecutor().planner
	require.NotNil(t, planner)

	relevant := planner.relevantSegments("pod-1", models.ResourceMetadata{
		UID:       "pod-1",
		Namespace: "default",
		Kind:      "Pod",
		Name:      "pod-1",
	}, 30*1e9, 50*1e9)
	require.Empty(t, relevant)
}

func TestQueryPlanner_ResourceTimelinePrunesOldSegmentsButKeepsLatestPreWindowSegment(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	resource := models.ResourceMetadata{
		Version:   "v1",
		UID:       "pod-prune",
		Namespace: "default",
		Kind:      "Pod",
		Name:      "pod-prune",
	}

	for _, sample := range []struct {
		id string
		ts int64
	}{
		{id: "cold-05", ts: 5},
		{id: "cold-15", ts: 15},
		{id: "cold-25", ts: 25},
		{id: "cold-35", ts: 35},
	} {
		require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
			plannerParityEvent(sample.id, sample.ts, models.EventTypeUpdate, resource),
		}))
		require.NoError(t, engine.Flush(context.Background()))
	}

	planner := engine.QueryExecutor().planner
	require.NotNil(t, planner)

	events, stats, err := planner.planResourceEvents(context.Background(), resource.UID, resource, 30*1e9, 40*1e9)
	require.NoError(t, err)
	require.Equal(t, []string{"cold-25", "cold-35"}, eventIDs(events))
	require.Equal(t, 2, stats.relevantSegments)
	require.Equal(t, 2, stats.scannedSegments)
	require.Equal(t, 2, stats.uidDiskLookups)
}

func TestQueryPlanner_QueryDistinctMetadataUsesProjectionState(t *testing.T) {
	engine := newTestEngineWithEvents(t, []models.Event{{
		ID:        "1",
		Timestamp: 10,
		Resource: models.ResourceMetadata{
			UID:       "pod-1",
			Namespace: "default",
			Kind:      "Pod",
			Name:      "pod-1",
		},
	}})

	namespaces, kinds, _, _, err := engine.QueryExecutor().QueryDistinctMetadata(context.Background(), 0, 30)
	require.NoError(t, err)
	require.Equal(t, []string{"default"}, namespaces)
	require.Equal(t, []string{"Pod"}, kinds)
}

func TestQueryPlanner_DedupesHotAndColdDuplicatesDeterministically(t *testing.T) {
	resource := models.ResourceMetadata{
		Version:   "v1",
		UID:       "pod-dedupe",
		Namespace: "default",
		Kind:      "Pod",
		Name:      "pod-dedupe",
	}

	coldDuplicate := plannerParityEvent("dup", 20, models.EventTypeUpdate, resource)
	coldDuplicate.Data = []byte(`{"source":"cold"}`)
	hotDuplicate := plannerParityEvent("dup", 20, models.EventTypeUpdate, resource)
	hotDuplicate.Data = []byte(`{"source":"hot"}`)

	engine := newTestEngineWithColdSegment(t, []models.Event{coldDuplicate}, []models.Event{hotDuplicate})
	planner := engine.QueryExecutor().planner
	require.NotNil(t, planner)

	merged, _, err := planner.collectMergedResourceEvents(context.Background(), resource.UID, resource, 0, 30*1e9)
	require.NoError(t, err)
	require.Len(t, merged, 1)

	exported, _, err := planner.exportTimeRange(context.Background(), 0, 30*1e9, models.QueryFilters{})
	require.NoError(t, err)
	require.Len(t, exported, 1)

	require.Equal(t, string(merged[0].Data), string(exported[0].Data))
	require.JSONEq(t, `{"source":"cold"}`, string(merged[0].Data))
}

func TestQueryPlanner_ExportFallsBackToFullScanWhenDimensionIndexIsMissing(t *testing.T) {
	engine := newFlushedTestEngine(t, []models.Event{
		{
			ID:        "cold-1",
			Timestamp: 10,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:   "v1",
				UID:       "pod-1",
				Namespace: "default",
				Kind:      "Pod",
				Name:      "pod-1",
			},
			Data: []byte(`{"kind":"Pod"}`),
		},
	})

	require.Len(t, engine.segmentReaders, 1)
	dimPath := filepath.Join(filepath.Dir(engine.segmentReaders[0].eventsPath), segmentDimIndexFile)
	require.NoError(t, os.Remove(dimPath))

	planner := engine.QueryExecutor().planner
	require.NotNil(t, planner)
	require.Len(t, planner.relevantExportSegments(models.QueryFilters{
		Namespace: "default",
		Kind:      "Pod",
	}, 0, 20*1e9), 1)

	exported, err := engine.QueryExecutor().ExportTimeRange(context.Background(), &models.QueryRequest{
		StartTimestamp: 0,
		EndTimestamp:   20,
		Filters: models.QueryFilters{
			Namespace: "default",
			Kind:      "Pod",
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"cold-1"}, eventIDs(exported))
}

func newTestEngineWithEvents(t *testing.T, events []models.Event) *Engine {
	t.Helper()

	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           100,
		HotMaxResourceVersions: 10,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	require.NoError(t, engine.ProcessBatch(context.Background(), events))
	return engine
}

func newTestEngineWithColdSegment(t *testing.T, coldEvents, hotEvents []models.Event) *Engine {
	t.Helper()

	engine := newTestEngineWithEvents(t, coldEvents)
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.ProcessBatch(context.Background(), hotEvents))
	return engine
}
