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
