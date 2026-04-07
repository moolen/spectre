package embeddedstore

import (
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestProjection_DoesNotRetainHistoricalEventArrays(t *testing.T) {
	projection, err := BuildProjection([]models.Event{
		{
			ID:        "pod-create",
			Timestamp: 10,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:   "v1",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "pod-1",
				UID:       "pod-1",
			},
			Data: []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-1","namespace":"default","uid":"pod-1","labels":{"app":"demo"}}}`),
		},
		{
			ID:        "pod-update",
			Timestamp: 20,
			Type:      models.EventTypeUpdate,
			Resource: models.ResourceMetadata{
				Version:   "v1",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "pod-1",
				UID:       "pod-1",
			},
			Data: []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-1","namespace":"default","uid":"pod-1","labels":{"app":"demo","tier":"api"}}}`),
		},
		{
			ID:        "k8s-event",
			Timestamp: 30,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:           "v1",
				Kind:              "Event",
				Namespace:         "default",
				Name:              "pod-1.17d49f7f4f",
				UID:               "event-1",
				InvolvedObjectUID: "pod-1",
			},
			Data: []byte(`{"reason":"BackOff","message":"Back-off restarting failed container","type":"Warning","count":3,"source":{"component":"kubelet"}}`),
		},
	})
	require.NoError(t, err)

	require.Empty(t, projection.events)
	require.Empty(t, projection.eventsByResourceUID)
	require.Empty(t, projection.k8sRawEventsByInvolvedUID)

	require.Len(t, projection.resourcesByUID, 1)
	require.Len(t, projection.resourceMetaByUID, 1)
	require.Len(t, projection.orderedResources, 1)
	require.Len(t, projection.k8sEventsByInvolvedUID, 1)
	require.Len(t, projection.k8sEventsByInvolvedUID["pod-1"], 1)

	record := projection.resourcesByUID["pod-1"]
	require.NotNil(t, record)
	require.Len(t, record.versions, 2)

	meta := projection.resourceMetaByUID["pod-1"]
	require.Equal(t, "pod-1", meta.UID)
	require.Equal(t, "Pod", meta.Kind)
	require.Equal(t, "default", meta.Namespace)
}

func TestEngine_ProcessBatchDoesNotRetainHistoricalEventArraysInPlannerMode(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	require.NoError(t, engine.ProcessBatch(t.Context(), []models.Event{
		{
			ID:        "pod-create",
			Timestamp: 10,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:   "v1",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "pod-1",
				UID:       "pod-1",
			},
			Data: []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-1","namespace":"default","uid":"pod-1"}}`),
		},
		{
			ID:        "pod-warning",
			Timestamp: 12,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:           "v1",
				Kind:              "Event",
				Namespace:         "default",
				Name:              "pod-1.17d49f7f4f",
				UID:               "event-1",
				InvolvedObjectUID: "pod-1",
			},
			Data: []byte(`{"reason":"BackOff","message":"restarting","type":"Warning","count":3,"source":{"component":"kubelet"}}`),
		},
	}))

	require.Empty(t, engine.projection.events)
	require.Empty(t, engine.projection.eventsByResourceUID)
	require.Empty(t, engine.projection.k8sRawEventsByInvolvedUID)

	result, pagination, err := engine.QueryExecutor().ExecutePaginated(
		t.Context(),
		&models.QueryRequest{
			StartTimestamp: 0,
			EndTimestamp:   20,
			Filters: models.QueryFilters{
				Namespace: "default",
				Kind:      "Pod",
			},
		},
		&models.PaginationRequest{PageSize: 1},
	)
	require.NoError(t, err)
	require.NotNil(t, pagination)
	require.False(t, pagination.HasMore)
	require.Equal(t, []string{"pod-create"}, eventIDs(result.Events))
	require.Equal(t, []string{"pod-warning"}, plannerParityK8sEventIDs(result.K8sEventsByResource["pod-1"]))
}
