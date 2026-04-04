package embeddedstore

import (
	"context"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestQueryParityGolden(t *testing.T) {
	t.Run("hot only history", func(t *testing.T) {
		engine := newQueryParityEngine(t)
		pod := plannerParityResource("uid-hot", "Pod", "ns-a", "pod-hot")

		require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
			plannerParityEvent("hot-10", 10, models.EventTypeCreate, pod),
			plannerParityEvent("hot-20", 20, models.EventTypeUpdate, pod),
		}))

		result, err := engine.QueryExecutor().Execute(context.Background(), &models.QueryRequest{
			StartTimestamp: 0,
			EndTimestamp:   30,
			Filters: models.QueryFilters{
				Namespace: "ns-a",
				Kind:      "Pod",
			},
		})
		require.NoError(t, err)
		require.Equal(t, []string{"hot-10", "hot-20"}, eventIDs(result.Events))

		exported, err := engine.QueryExecutor().ExportTimeRange(context.Background(), &models.QueryRequest{
			StartTimestamp: 0,
			EndTimestamp:   30,
			Filters: models.QueryFilters{
				Namespace: "ns-a",
				Kind:      "Pod",
			},
		})
		require.NoError(t, err)
		require.Equal(t, []string{"hot-10", "hot-20"}, eventIDs(exported))
	})

	t.Run("segment only history", func(t *testing.T) {
		engine := newQueryParityEngine(t)
		pod := plannerParityResource("uid-cold", "Pod", "ns-a", "pod-cold")

		require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
			plannerParityEvent("cold-10", 10, models.EventTypeCreate, pod),
			plannerParityEvent("cold-20", 20, models.EventTypeUpdate, pod),
		}))
		require.NoError(t, engine.Flush(context.Background()))

		result, err := engine.QueryExecutor().Execute(context.Background(), &models.QueryRequest{
			StartTimestamp: 0,
			EndTimestamp:   30,
			Filters: models.QueryFilters{
				Namespace: "ns-a",
				Kind:      "Pod",
			},
		})
		require.NoError(t, err)
		require.Equal(t, []string{"cold-10", "cold-20"}, eventIDs(result.Events))

		exported, err := engine.QueryExecutor().ExportTimeRange(context.Background(), &models.QueryRequest{
			StartTimestamp: 0,
			EndTimestamp:   30,
			Filters: models.QueryFilters{
				Namespace: "ns-a",
				Kind:      "Pod",
			},
		})
		require.NoError(t, err)
		require.Equal(t, []string{"cold-10", "cold-20"}, eventIDs(exported))
	})

	t.Run("mixed history preserves resource pagination and export ordering", func(t *testing.T) {
		engine := newQueryParityEngine(t)
		podA := plannerParityResource("uid-a", "Pod", "ns-a", "pod-a")
		podB := plannerParityResource("uid-b", "Pod", "ns-a", "pod-b")

		require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
			plannerParityEvent("b-20", 20, models.EventTypeCreate, podB),
			plannerParityEvent("z-40", 40, models.EventTypeUpdate, podB),
		}))
		require.NoError(t, engine.Flush(context.Background()))
		require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
			plannerParityEvent("a-20", 20, models.EventTypeCreate, podA),
			plannerParityEvent("b-20", 20, models.EventTypeCreate, podB),
			plannerParityEvent("a-40", 40, models.EventTypeUpdate, podA),
		}))

		result, err := engine.QueryExecutor().Execute(context.Background(), &models.QueryRequest{
			StartTimestamp: 20,
			EndTimestamp:   40,
			Filters: models.QueryFilters{
				Namespace: "ns-a",
				Kind:      "Pod",
			},
		})
		require.NoError(t, err)
		require.Equal(t, []string{"a-20", "a-40", "b-20", "z-40"}, eventIDs(result.Events))

		exported, err := engine.QueryExecutor().ExportTimeRange(context.Background(), &models.QueryRequest{
			StartTimestamp: 20,
			EndTimestamp:   40,
			Filters: models.QueryFilters{
				Namespace: "ns-a",
				Kind:      "Pod",
			},
		})
		require.NoError(t, err)
		require.Equal(t, []string{"a-20", "b-20", "a-40", "z-40"}, eventIDs(exported))
	})

	t.Run("deleted resources still inject latest pre existing event", func(t *testing.T) {
		engine := newQueryParityEngine(t)
		pod := plannerParityResource("uid-delete", "Pod", "ns-a", "pod-delete")

		require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
			plannerParityEvent("pod-create", 10, models.EventTypeCreate, pod),
		}))
		require.NoError(t, engine.Flush(context.Background()))
		require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
			plannerParityEvent("pod-delete", 25, models.EventTypeDelete, pod),
		}))

		result, err := engine.QueryExecutor().Execute(context.Background(), &models.QueryRequest{
			StartTimestamp: 20,
			EndTimestamp:   30,
			Filters: models.QueryFilters{
				Namespace: "ns-a",
				Kind:      "Pod",
			},
		})
		require.NoError(t, err)
		require.Equal(t, []string{"pod-create", "pod-delete"}, eventIDs(result.Events))
		require.Equal(t, []bool{true, false}, plannerParityPreExistingFlags(result.Events))
	})

	t.Run("event kind timelines and attachments remain deterministic", func(t *testing.T) {
		engine := newQueryParityEngine(t)
		pod := plannerParityResource("uid-pod", "Pod", "ns-a", "pod-a")

		require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
			plannerParityEvent("pod-create", 10, models.EventTypeCreate, pod),
			plannerParityK8sEvent("evt-1", "event-uid-1", "evt-1", "ns-a", "uid-pod", 20, "Failed", "first", "Warning", 1, "kubelet"),
		}))
		require.NoError(t, engine.Flush(context.Background()))
		require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
			plannerParityEvent("pod-update", 35, models.EventTypeUpdate, pod),
			plannerParityK8sEvent("evt-2-b", "event-uid-2", "evt-2", "ns-a", "uid-pod", 30, "BackOff", "later-b", "Warning", 2, "scheduler"),
			plannerParityK8sEvent("evt-2-a", "event-uid-2", "evt-2", "ns-a", "uid-pod", 30, "BackOff", "later-a", "Warning", 3, "scheduler"),
			plannerParityK8sEvent("evt-3", "event-uid-3", "evt-3", "ns-a", "uid-pod", 40, "Recovered", "done", "Normal", 4, "controller"),
		}))

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
		require.Equal(t, []string{"pod-create", "pod-update"}, eventIDs(resourceResult.Events))
		require.Equal(t, []string{"evt-1", "evt-2-a", "evt-2-b", "evt-3"}, plannerParityK8sEventIDs(resourceResult.K8sEventsByResource["uid-pod"]))
	})
}

func newQueryParityEngine(t *testing.T) *Engine {
	t.Helper()

	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	return engine
}
