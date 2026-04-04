package embeddedstore

import (
	"context"
	"reflect"
	"testing"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestProjection_GetChangeEventsHydratesDataWithoutRetainingDuplicateCopy(t *testing.T) {
	raw := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"pod-1","namespace":"default","uid":"pod-uid","labels":{"app":"demo"}},"spec":{"containers":[{"name":"app","image":"demo:v1"}]}}`)

	projection, err := BuildProjection([]models.Event{
		{
			ID:        "pod-create",
			Timestamp: 10,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Version:   "v1",
				Name:      "pod-1",
				Namespace: "default",
				UID:       "pod-uid",
			},
			Data: raw,
		},
	})
	require.NoError(t, err)

	record := projection.resourcesByUID["pod-uid"]
	require.NotNil(t, record)
	require.Len(t, record.versions, 1)
	require.Nil(t, record.versions[0].changeEvent.Data)

	events, err := NewAnalysisStore(projection).GetChangeEvents(context.Background(), []string{"pod-uid"}, analysisstore.ResourceWindow{
		FailureTimestampNs: 10,
		LookbackNs:         10,
	})
	require.NoError(t, err)
	require.Len(t, events["pod-uid"], 1)
	require.Equal(t, raw, events["pod-uid"][0].Data)
}

func TestProjection_NamespaceGraphHydratesReplicaMetadataWithoutRetainingParsedObject(t *testing.T) {
	raw := []byte(`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"web","namespace":"default","uid":"deploy-uid","labels":{"app":"web"}},"spec":{"replicas":3,"selector":{"matchLabels":{"app":"web"}}}}`)

	projection, err := BuildProjection([]models.Event{
		{
			ID:        "deploy-create",
			Timestamp: 10,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind:      "Deployment",
				Group:     "apps",
				Version:   "v1",
				Name:      "web",
				Namespace: "default",
				UID:       "deploy-uid",
			},
			Data: raw,
		},
	})
	require.NoError(t, err)

	record := projection.resourcesByUID["deploy-uid"]
	require.NotNil(t, record)
	require.Len(t, record.versions, 1)
	_, hasObjectField := reflect.TypeOf(record.versions[0]).FieldByName("object")
	require.False(t, hasObjectField)

	graph, err := NewAnalysisStore(projection).GetNamespaceGraph(context.Background(), analysisstore.NamespaceGraphQuery{
		Namespace:   "default",
		TimestampNs: 10,
		LookbackNs:  10,
		Limit:       10,
		MaxDepth:    1,
	})
	require.NoError(t, err)
	require.Len(t, graph.Graph.Nodes, 1)
	require.NotNil(t, graph.Graph.Nodes[0].LatestEvent)
	require.NotNil(t, graph.Graph.Nodes[0].LatestEvent.SpecReplicas)
	require.Equal(t, 3, *graph.Graph.Nodes[0].LatestEvent.SpecReplicas)
}
