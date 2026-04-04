package embeddedstore

import (
	"context"
	"fmt"
	"testing"

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
