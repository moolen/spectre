package embeddedstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestQueryExecutor_EventPlannerParity(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	podA := plannerParityResource("uid-a", "Pod", "ns-a", "pod-a")
	podB := plannerParityResource("uid-b", "Pod", "ns-b", "pod-b")

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("pod-a-create", 10, models.EventTypeCreate, podA),
		plannerParityEvent("pod-b-create", 11, models.EventTypeCreate, podB),
		plannerParityK8sEvent("evt-1", "event-uid-1", "evt-1", "ns-a", "uid-a", 20, "Failed", "first", "Warning", 1, "kubelet"),
		plannerParityK8sEvent("evt-2-b", "event-uid-2", "evt-2", "ns-a", "uid-a", 30, "BackOff", "later-b", "Warning", 2, "scheduler"),
	}))
	require.NoError(t, engine.Flush(context.Background()))

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("pod-a-update", 35, models.EventTypeUpdate, podA),
		plannerParityK8sEvent("evt-2-b", "event-uid-2", "evt-2", "ns-a", "uid-a", 30, "BackOff", "later-b", "Warning", 2, "scheduler"),
		plannerParityK8sEvent("evt-2-a", "event-uid-2", "evt-2", "ns-a", "uid-a", 30, "BackOff", "later-a", "Warning", 3, "scheduler"),
		plannerParityK8sEvent("evt-3", "event-uid-3", "evt-3", "ns-a", "uid-a", 40, "Recovered", "done", "Normal", 4, "controller"),
		plannerParityK8sEvent("evt-b", "event-uid-b", "evt-b", "ns-b", "uid-b", 25, "Other", "ignore", "Normal", 1, "controller"),
	}))

	engine.projection.mu.Lock()
	engine.projection.k8sRawEventsByInvolvedUID = map[string][]models.Event{}
	engine.projection.mu.Unlock()

	eventResult, err := engine.QueryExecutor().Execute(context.Background(), &models.QueryRequest{
		StartTimestamp: 20,
		EndTimestamp:   40,
		Filters: models.QueryFilters{
			Kinds:      []string{"Event"},
			Version:    "v1",
			Namespaces: []string{"ns-a"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"evt-1", "evt-2-a", "evt-2-b", "evt-3"}, eventIDs(eventResult.Events))

	resourceResult, pagination, err := engine.QueryExecutor().ExecutePaginated(context.Background(), &models.QueryRequest{
		StartTimestamp: 20,
		EndTimestamp:   40,
		Filters: models.QueryFilters{
			Namespace: "ns-a",
			Kind:      "Pod",
		},
	}, &models.PaginationRequest{PageSize: 1})
	require.NoError(t, err)
	require.NotNil(t, pagination)
	require.False(t, pagination.HasMore)
	require.Equal(t, []string{"pod-a-create", "pod-a-update"}, eventIDs(resourceResult.Events))
	require.Equal(t, []string{"evt-1", "evt-2-a", "evt-2-b", "evt-3"}, plannerParityK8sEventIDs(resourceResult.K8sEventsByResource["uid-a"]))
	require.NotContains(t, resourceResult.K8sEventsByResource, "uid-b")

	attached := resourceResult.K8sEventsByResource["uid-a"]
	require.Len(t, attached, 4)
	require.Equal(t, "Failed", attached[0].Reason)
	require.Equal(t, "first", attached[0].Message)
	require.Equal(t, "Warning", attached[0].Type)
	require.Equal(t, int32(1), attached[0].Count)
	require.Equal(t, "kubelet", attached[0].Source)
	require.Equal(t, "scheduler", attached[1].Source)
	require.Equal(t, int32(3), attached[1].Count)
	require.Equal(t, "controller", attached[3].Source)
}

func TestQueryExecutor_AttachedK8sEventsIncludeClusterScopedResources(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	node := plannerParityResource("uid-node", "Node", "", "node-a")
	pod := plannerParityResource("uid-pod", "Pod", "ns-a", "pod-a")

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("node-create", 20, models.EventTypeCreate, node),
		plannerParityEvent("pod-create", 20, models.EventTypeCreate, pod),
		plannerParityK8sEvent("evt-node", "event-uid-node", "evt-node", "kube-system", "uid-node", 21, "Cluster", "node event", "Warning", 1, "kubelet"),
		plannerParityK8sEvent("evt-pod", "event-uid-pod", "evt-pod", "ns-a", "uid-pod", 21, "Namespaced", "pod event", "Normal", 1, "controller"),
	}))
	require.NoError(t, engine.Flush(context.Background()))

	engine.projection.mu.Lock()
	engine.projection.k8sRawEventsByInvolvedUID = map[string][]models.Event{}
	engine.projection.mu.Unlock()

	result, pagination, err := engine.QueryExecutor().ExecutePaginated(context.Background(), &models.QueryRequest{
		StartTimestamp: 20,
		EndTimestamp:   25,
		Filters:        models.QueryFilters{},
	}, &models.PaginationRequest{PageSize: 2})
	require.NoError(t, err)
	require.NotNil(t, pagination)
	require.False(t, pagination.HasMore)
	require.Equal(t, []string{"node-create", "pod-create"}, eventIDs(result.Events))
	require.Equal(t, []string{"evt-node"}, plannerParityK8sEventIDs(result.K8sEventsByResource["uid-node"]))
	require.Equal(t, []string{"evt-pod"}, plannerParityK8sEventIDs(result.K8sEventsByResource["uid-pod"]))
}

func TestQueryExecutor_AttachedK8sEventsRecoverFromColdSegmentsWhenProjectionCacheIsMissing(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	pod := plannerParityResource("uid-a", "Pod", "ns-a", "pod-a")

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("pod-a-create", 10, models.EventTypeCreate, pod),
		plannerParityK8sEvent("evt-1", "event-uid-1", "evt-1", "ns-a", "uid-a", 20, "Failed", "first", "Warning", 1, "kubelet"),
	}))
	require.NoError(t, engine.Flush(context.Background()))

	engine.projection.mu.Lock()
	engine.projection.k8sRawEventsByInvolvedUID = map[string][]models.Event{}
	engine.projection.k8sEventsByInvolvedUID = map[string][]analysisstore.K8sEventInfo{}
	engine.projection.mu.Unlock()

	resourceResult, pagination, err := engine.QueryExecutor().ExecutePaginated(context.Background(), &models.QueryRequest{
		StartTimestamp: 10,
		EndTimestamp:   25,
		Filters: models.QueryFilters{
			Namespace: "ns-a",
			Kind:      "Pod",
		},
	}, &models.PaginationRequest{PageSize: 1})
	require.NoError(t, err)
	require.NotNil(t, pagination)
	require.False(t, pagination.HasMore)
	require.Equal(t, []string{"pod-a-create"}, eventIDs(resourceResult.Events))
	require.Equal(t, []string{"evt-1"}, plannerParityK8sEventIDs(resourceResult.K8sEventsByResource["uid-a"]))
}

func TestQueryExecutor_AttachedK8sEventsFallbackToColdSegmentScanWhenAssociatedIndexIsMissing(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	pod := plannerParityResource("uid-a", "Pod", "ns-a", "pod-a")

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("pod-a-create", 10, models.EventTypeCreate, pod),
		plannerParityK8sEvent("evt-1", "event-uid-1", "evt-1", "ns-a", "uid-a", 20, "Failed", "first", "Warning", 1, "kubelet"),
	}))
	require.NoError(t, engine.Flush(context.Background()))

	require.Len(t, engine.segmentReaders, 1)
	associatedPath := filepath.Join(filepath.Dir(engine.segmentReaders[0].eventsPath), segmentAssociatedUIDIndexFile)
	require.NoError(t, os.Remove(associatedPath))

	engine.projection.mu.Lock()
	engine.projection.k8sRawEventsByInvolvedUID = map[string][]models.Event{}
	engine.projection.k8sEventsByInvolvedUID = map[string][]analysisstore.K8sEventInfo{}
	engine.projection.mu.Unlock()

	resourceResult, pagination, err := engine.QueryExecutor().ExecutePaginated(context.Background(), &models.QueryRequest{
		StartTimestamp: 10,
		EndTimestamp:   25,
		Filters: models.QueryFilters{
			Namespace: "ns-a",
			Kind:      "Pod",
		},
	}, &models.PaginationRequest{PageSize: 1})
	require.NoError(t, err)
	require.NotNil(t, pagination)
	require.False(t, pagination.HasMore)
	require.Equal(t, []string{"pod-a-create"}, eventIDs(resourceResult.Events))
	require.Equal(t, []string{"evt-1"}, plannerParityK8sEventIDs(resourceResult.K8sEventsByResource["uid-a"]))
}

func TestQueryExecutor_WarmTimelineCachesLoadsColdIndexes(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	pod := plannerParityResource("uid-a", "Pod", "ns-a", "pod-a")

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("pod-a-create", 10, models.EventTypeCreate, pod),
		plannerParityK8sEvent("evt-1", "event-uid-1", "evt-1", "ns-a", "uid-a", 20, "Failed", "first", "Warning", 1, "kubelet"),
	}))
	require.NoError(t, engine.Flush(context.Background()))
	engine.QueryExecutor().ConfigureRecentEventTimelineCache(0)

	require.Len(t, engine.segmentReaders, 1)
	engine.segmentReaders[0].associatedIndex = nil
	engine.segmentReaders[0].associatedIndexLoaded = false
	engine.segmentReaders[0].associatedIndexPresent = false
	engine.segmentReaders[0].timeIndex = nil
	require.Nil(t, engine.segmentReaders[0].resourceIndex)
	require.Nil(t, engine.segmentReaders[0].timeIndex)
	require.False(t, engine.segmentReaders[0].associatedIndexLoaded)

	err = engine.QueryExecutor().warmTimelineCaches(
		context.Background(),
		25*1e9,
		[]time.Duration{time.Hour},
		1,
	)
	require.NoError(t, err)
	require.NotNil(t, engine.segmentReaders[0].timeIndex)
	require.Nil(t, engine.segmentReaders[0].resourceIndex)
	require.False(t, engine.segmentReaders[0].associatedIndexLoaded)
}

func TestQueryExecutor_WarmRecentAssociatedEventIndexesLoadsRecentEventSegmentsOnly(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	oldBase := int64((2 * time.Hour).Seconds())
	recentBase := int64((8 * time.Hour).Seconds())
	endTimeNs := int64((10 * time.Hour).Nanoseconds())
	pod := plannerParityResource("uid-a", "Pod", "ns-a", "pod-a")

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("pod-old", oldBase, models.EventTypeCreate, pod),
		plannerParityK8sEvent("evt-old", "event-uid-old", "evt-old", "ns-a", "uid-a", oldBase+int64(time.Minute.Seconds()), "Old", "old", "Warning", 1, "kubelet"),
	}))
	require.NoError(t, engine.Flush(context.Background()))

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("pod-recent", recentBase, models.EventTypeUpdate, pod),
		plannerParityK8sEvent("evt-recent", "event-uid-recent", "evt-recent", "ns-a", "uid-a", recentBase+int64(time.Minute.Seconds()), "Recent", "recent", "Warning", 1, "kubelet"),
	}))
	require.NoError(t, engine.Flush(context.Background()))

	require.Len(t, engine.segmentReaders, 2)
	for i := range engine.segmentReaders {
		engine.segmentReaders[i].associatedIndex = nil
		engine.segmentReaders[i].associatedIndexLoaded = false
		engine.segmentReaders[i].associatedIndexPresent = false
	}
	require.False(t, engine.segmentReaders[0].associatedIndexLoaded)
	require.False(t, engine.segmentReaders[1].associatedIndexLoaded)

	loadedSegments, err := engine.QueryExecutor().warmRecentAssociatedEventIndexes(
		context.Background(),
		endTimeNs,
		3*time.Hour,
		8,
		64<<20,
	)
	require.NoError(t, err)
	require.Equal(t, 1, loadedSegments)
	require.False(t, engine.segmentReaders[0].associatedIndexLoaded)
	require.True(t, engine.segmentReaders[1].associatedIndexLoaded)
}

func TestQueryExecutor_RecentEventCacheServesTimelineWithoutPlanner(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	pod := plannerParityResource("uid-a", "Pod", "ns-a", "pod-a")

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("pod-a-create", 10, models.EventTypeCreate, pod),
		plannerParityK8sEvent("evt-1", "event-uid-1", "evt-1", "ns-a", "uid-a", 20, "Failed", "first", "Warning", 1, "kubelet"),
		plannerParityK8sEvent("evt-2", "event-uid-2", "evt-2", "ns-a", "uid-a", 30, "BackOff", "second", "Warning", 2, "scheduler"),
	}))
	require.NoError(t, engine.Flush(context.Background()))

	engine.QueryExecutor().ConfigureRecentEventTimelineCache(2 * time.Hour)
	require.NoError(t, engine.QueryExecutor().SeedRecentEventTimelineCache(context.Background(), 40*1e9, 2*time.Hour))

	engine.QueryExecutor().SetSharedCache((*QueryPlanner)(nil))

	result, err := engine.QueryExecutor().Execute(context.Background(), &models.QueryRequest{
		StartTimestamp: 15,
		EndTimestamp:   40,
		Filters: models.QueryFilters{
			Kinds:      []string{"Event"},
			Version:    "v1",
			Namespaces: []string{"ns-a"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"evt-1", "evt-2"}, eventIDs(result.Events))
}

func TestQueryExecutor_RecentEventCacheTracksNewIngestAfterSeed(t *testing.T) {
	nowSeconds := time.Now().Unix()
	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	pod := plannerParityResource("uid-a", "Pod", "ns-a", "pod-a")

	engine.QueryExecutor().ConfigureRecentEventTimelineCache(2 * time.Hour)
	require.NoError(t, engine.QueryExecutor().SeedRecentEventTimelineCache(context.Background(), (nowSeconds-120)*1e9, 2*time.Hour))

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("pod-a-create", nowSeconds-90, models.EventTypeCreate, pod),
		plannerParityK8sEvent("evt-3", "event-uid-3", "evt-3", "ns-a", "uid-a", nowSeconds-60, "Recovered", "later", "Normal", 1, "controller"),
	}))

	engine.QueryExecutor().SetSharedCache((*QueryPlanner)(nil))

	result, err := engine.QueryExecutor().Execute(context.Background(), &models.QueryRequest{
		StartTimestamp: nowSeconds - 120,
		EndTimestamp:   nowSeconds,
		Filters: models.QueryFilters{
			Kinds:      []string{"Event"},
			Version:    "v1",
			Namespaces: []string{"ns-a"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"evt-3"}, eventIDs(result.Events))
}

func TestQueryExecutor_RecentEventCacheFallsBackOutsideConfiguredHorizon(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	pod := plannerParityResource("uid-a", "Pod", "ns-a", "pod-a")

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("pod-a-create", 10, models.EventTypeCreate, pod),
		plannerParityK8sEvent("evt-1", "event-uid-1", "evt-1", "ns-a", "uid-a", 20, "Failed", "first", "Warning", 1, "kubelet"),
	}))
	require.NoError(t, engine.Flush(context.Background()))

	engine.QueryExecutor().ConfigureRecentEventTimelineCache(30 * time.Minute)
	require.NoError(t, engine.QueryExecutor().SeedRecentEventTimelineCache(context.Background(), 40*1e9, 30*time.Minute))

	engine.QueryExecutor().SetSharedCache((*QueryPlanner)(nil))

	_, err = engine.QueryExecutor().Execute(context.Background(), &models.QueryRequest{
		StartTimestamp: 0,
		EndTimestamp:   3600,
		Filters: models.QueryFilters{
			Kinds:   []string{"Event"},
			Version: "v1",
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "projection history fallback disabled")
}

func TestEngine_OpenSeedsRecentEventTimelineCache(t *testing.T) {
	dataDir := t.TempDir()
	nowSeconds := time.Now().Unix()
	engine, err := OpenEngine(EngineConfig{
		DataDir:                dataDir,
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)

	pod := plannerParityResource("uid-a", "Pod", "ns-a", "pod-a")

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("pod-a-create", nowSeconds-600, models.EventTypeCreate, pod),
		plannerParityK8sEvent("evt-1", "event-uid-1", "evt-1", "ns-a", "uid-a", nowSeconds-300, "Failed", "first", "Warning", 1, "kubelet"),
	}))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.Close())

	reopened, err := OpenEngine(EngineConfig{
		DataDir:                dataDir,
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, reopened.Close())
	})

	reopened.QueryExecutor().SetSharedCache((*QueryPlanner)(nil))

	require.Eventually(t, func() bool {
		result, err := reopened.QueryExecutor().Execute(context.Background(), &models.QueryRequest{
			StartTimestamp: nowSeconds - 900,
			EndTimestamp:   nowSeconds,
			Filters: models.QueryFilters{
				Kinds:      []string{"Event"},
				Version:    "v1",
				Namespaces: []string{"ns-a"},
			},
		})
		return err == nil && len(result.Events) == 1 && result.Events[0].ID == "evt-1"
	}, time.Second, 10*time.Millisecond)
}

func TestEngine_FlushWarmsAssociatedIndexForNewEventSegment(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	pod := plannerParityResource("uid-a", "Pod", "ns-a", "pod-a")

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("pod-a-create", 10, models.EventTypeCreate, pod),
		plannerParityK8sEvent("evt-1", "event-uid-1", "evt-1", "ns-a", "uid-a", 20, "Failed", "first", "Warning", 1, "kubelet"),
	}))
	require.NoError(t, engine.Flush(context.Background()))

	require.Len(t, engine.segmentReaders, 1)
	require.True(t, engine.segmentReaders[0].associatedIndexLoaded)
}

func plannerParityK8sEvent(
	id, uid, name, namespace, involvedUID string,
	tsSeconds int64,
	reason, message, eventType string,
	count int,
	source string,
) models.Event {
	data := []byte(fmt.Sprintf(
		`{"reason":%q,"message":%q,"type":%q,"count":%d,"source":{"component":%q}}`,
		reason,
		message,
		eventType,
		count,
		source,
	))

	return models.Event{
		ID:        id,
		Timestamp: tsSeconds * 1e9,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Version:           "v1",
			UID:               uid,
			Namespace:         namespace,
			Kind:              "Event",
			Name:              name,
			InvolvedObjectUID: involvedUID,
		},
		Data: data,
	}
}

func plannerParityK8sEventIDs(events []models.K8sEvent) []string {
	ids := make([]string, 0, len(events))
	for i := range events {
		ids = append(ids, events[i].ID)
	}
	return ids
}
