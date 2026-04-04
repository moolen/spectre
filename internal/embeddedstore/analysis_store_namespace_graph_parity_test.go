package embeddedstore

import (
	"context"
	"testing"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestAnalysisStore_NamespaceGraphParityAfterCompactCheckpointReload(t *testing.T) {
	dataDir := t.TempDir()
	engine, err := OpenEngine(EngineConfig{
		DataDir:                dataDir,
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	deployment := models.ResourceMetadata{
		Group:     "apps",
		Version:   "v1",
		Kind:      "Deployment",
		Namespace: "default",
		Name:      "web",
		UID:       "deploy-uid",
	}
	service := models.ResourceMetadata{
		Version:   "v1",
		Kind:      "Service",
		Namespace: "default",
		Name:      "web",
		UID:       "svc-uid",
	}
	pod := models.ResourceMetadata{
		Version:   "v1",
		Kind:      "Pod",
		Namespace: "default",
		Name:      "web-abc",
		UID:       "pod-uid",
	}

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "deploy-create",
			Timestamp: 10,
			Type:      models.EventTypeCreate,
			Resource:  deployment,
			Data:      []byte(`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"web","namespace":"default","uid":"deploy-uid","labels":{"app":"web"}},"spec":{"replicas":3,"selector":{"matchLabels":{"app":"web"}}}}`),
		},
		{
			ID:        "svc-create",
			Timestamp: 12,
			Type:      models.EventTypeCreate,
			Resource:  service,
			Data:      []byte(`{"apiVersion":"v1","kind":"Service","metadata":{"name":"web","namespace":"default","uid":"svc-uid"},"spec":{"selector":{"app":"web"}}}`),
		},
		{
			ID:        "pod-create",
			Timestamp: 14,
			Type:      models.EventTypeCreate,
			Resource:  pod,
			Data:      []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"web-abc","namespace":"default","uid":"pod-uid","labels":{"app":"web"}}}`),
		},
	}))
	require.NoError(t, engine.Flush(context.Background()))

	query := analysisstore.NamespaceGraphQuery{
		Namespace:   "default",
		TimestampNs: 20,
		LookbackNs:  20,
		Limit:       10,
		MaxDepth:    2,
	}

	expectedGraph, err := engine.AnalysisStore().GetNamespaceGraph(context.Background(), query)
	require.NoError(t, err)
	require.NoError(t, engine.Checkpoint(context.Background()))
	require.NoError(t, engine.Close())

	reopened, err := OpenEngine(EngineConfig{
		DataDir:                dataDir,
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, reopened.Close())
	}()

	actualGraph, err := reopened.AnalysisStore().GetNamespaceGraph(context.Background(), query)
	require.NoError(t, err)

	require.Equal(t, namespaceGraphNodeKeys(expectedGraph.Graph.Nodes), namespaceGraphNodeKeys(actualGraph.Graph.Nodes))
	require.Equal(t, namespaceGraphEdgeKeys(expectedGraph.Graph.Edges), namespaceGraphEdgeKeys(actualGraph.Graph.Edges))
	require.Contains(t, namespaceGraphEdgeKeys(actualGraph.Graph.Edges), "svc-uid/SELECTS/pod-uid")

	expectedDeployment := namespaceGraphNodeByUID(expectedGraph.Graph.Nodes, "deploy-uid")
	actualDeployment := namespaceGraphNodeByUID(actualGraph.Graph.Nodes, "deploy-uid")
	require.NotNil(t, expectedDeployment)
	require.NotNil(t, actualDeployment)
	require.NotNil(t, expectedDeployment.LatestEvent)
	require.NotNil(t, actualDeployment.LatestEvent)
	require.NotNil(t, expectedDeployment.LatestEvent.SpecReplicas)
	require.NotNil(t, actualDeployment.LatestEvent.SpecReplicas)
	require.Equal(t, *expectedDeployment.LatestEvent.SpecReplicas, *actualDeployment.LatestEvent.SpecReplicas)
}

func namespaceGraphNodeByUID(nodes []analysisstore.NamespaceGraphNode, uid string) *analysisstore.NamespaceGraphNode {
	for i := range nodes {
		if nodes[i].UID == uid {
			return &nodes[i]
		}
	}
	return nil
}
