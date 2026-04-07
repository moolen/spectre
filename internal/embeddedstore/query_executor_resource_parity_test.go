package embeddedstore

import (
	"context"
	"fmt"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestQueryExecutor_ResourceTimelinePlannerParity(t *testing.T) {
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
	podB := plannerParityResource("uid-b", "Pod", "ns-a", "pod-b")

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("a-pre", 10, models.EventTypeCreate, podA),
		plannerParityEvent("b-pre", 12, models.EventTypeCreate, podB),
	}))
	require.NoError(t, engine.Flush(context.Background()))

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("a-20-dup", 20, models.EventTypeUpdate, podA),
		plannerParityEvent("a-40-b", 40, models.EventTypeUpdate, podA),
	}))
	require.NoError(t, engine.Flush(context.Background()))

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("a-20-dup", 20, models.EventTypeUpdate, podA),
		plannerParityEvent("a-40-a", 40, models.EventTypeUpdate, podA),
		plannerParityEvent("b-25", 25, models.EventTypeUpdate, podB),
	}))

	engine.projection.mu.Lock()
	engine.projection.eventsByResourceUID[podA.UID] = nil
	engine.projection.eventsByResourceUID[podB.UID] = nil
	engine.projection.mu.Unlock()

	query := &models.QueryRequest{
		StartTimestamp: 20,
		EndTimestamp:   40,
		Filters: models.QueryFilters{
			Namespace: "ns-a",
			Kind:      "Pod",
		},
	}

	page1, page1Pagination, err := engine.QueryExecutor().ExecutePaginated(context.Background(), query, &models.PaginationRequest{PageSize: 1})
	require.NoError(t, err)
	require.Equal(t, []string{"a-pre", "a-20-dup", "a-40-a", "a-40-b"}, eventIDs(page1.Events))
	require.Equal(t, []bool{true, false, false, false}, plannerParityPreExistingFlags(page1.Events))
	require.NotNil(t, page1Pagination)
	require.True(t, page1Pagination.HasMore)
	require.NotEmpty(t, page1Pagination.NextCursor)

	page2, page2Pagination, err := engine.QueryExecutor().ExecutePaginated(context.Background(), query, &models.PaginationRequest{
		PageSize: 1,
		Cursor:   page1Pagination.NextCursor,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"b-pre", "b-25"}, eventIDs(page2.Events))
	require.Equal(t, []bool{true, false}, plannerParityPreExistingFlags(page2.Events))
	require.NotNil(t, page2Pagination)
	require.False(t, page2Pagination.HasMore)
	require.Empty(t, page2Pagination.NextCursor)
}

func TestQueryExecutor_ResourceTimelineRecentWindowParity(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	stable := plannerParityResource("uid-stable", "Pod", "ns-a", "pod-a-stable")
	deletedInWindow := plannerParityResource("uid-deleted-window", "Pod", "ns-a", "pod-b-deleted")
	createdInWindow := plannerParityResource("uid-created-window", "Pod", "ns-a", "pod-c-created")
	updatedInWindow := plannerParityResource("uid-updated-window", "Pod", "ns-a", "pod-d-updated")
	deletedBeforeWindow := plannerParityResource("uid-deleted-before", "Pod", "ns-a", "pod-e-old")

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("a-create", 10, models.EventTypeCreate, stable),
		plannerParityEvent("b-create", 10, models.EventTypeCreate, deletedInWindow),
		plannerParityEvent("d-create", 10, models.EventTypeCreate, updatedInWindow),
		plannerParityEvent("e-create", 10, models.EventTypeCreate, deletedBeforeWindow),
	}))
	require.NoError(t, engine.Flush(context.Background()))

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("e-delete", 30, models.EventTypeDelete, deletedBeforeWindow),
	}))
	require.NoError(t, engine.Flush(context.Background()))

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("c-create", 60, models.EventTypeCreate, createdInWindow),
		plannerParityEvent("d-update", 65, models.EventTypeUpdate, updatedInWindow),
		plannerParityEvent("b-delete", 70, models.EventTypeDelete, deletedInWindow),
	}))

	resources, pagination, stats, ok, err := engine.QueryExecutor().collectPaginatedRecentWindowResources(
		context.Background(),
		50*1e9,
		80*1e9,
		models.QueryFilters{
			Namespace: "ns-a",
			Kind:      "Pod",
		},
		nil,
		10,
	)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, stats.projectionUsed)
	require.NotNil(t, pagination)
	require.False(t, pagination.HasMore)
	require.Empty(t, pagination.NextCursor)

	var resultEvents []models.Event
	for i := range resources {
		resultEvents = append(resultEvents, resources[i].events...)
	}

	require.Equal(t, []string{
		"a-create",
		"b-create",
		"b-delete",
		"c-create",
		"d-create",
		"d-update",
	}, eventIDs(resultEvents))
	require.Equal(t, []bool{
		true,
		true,
		false,
		false,
		true,
		false,
	}, plannerParityPreExistingFlags(resultEvents))
}

func plannerParityResource(uid, kind, namespace, name string) models.ResourceMetadata {
	return models.ResourceMetadata{
		Version:   "v1",
		UID:       uid,
		Namespace: namespace,
		Kind:      kind,
		Name:      name,
	}
}

func plannerParityEvent(id string, tsSeconds int64, eventType models.EventType, resource models.ResourceMetadata) models.Event {
	data := []byte(fmt.Sprintf(
		`{"kind":%q,"metadata":{"name":%q,"namespace":%q,"uid":%q}}`,
		resource.Kind,
		resource.Name,
		resource.Namespace,
		resource.UID,
	))
	if eventType == models.EventTypeDelete {
		data = nil
	}

	return models.Event{
		ID:        id,
		Timestamp: tsSeconds * 1e9,
		Type:      eventType,
		Resource:  resource,
		Data:      data,
	}
}

func plannerParityPreExistingFlags(events []models.Event) []bool {
	flags := make([]bool, 0, len(events))
	for i := range events {
		flags = append(flags, events[i].PreExisting)
	}
	return flags
}
