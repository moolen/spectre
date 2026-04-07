package embeddedstore

import (
	"context"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestQueryExecutor_MetadataPlannerParity(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	deletedPod := plannerParityResource("uid-deleted", "Pod", "ns-a", "pod-deleted")
	hotService := plannerParityResource("uid-service", "Service", "ns-b", "svc-hot")
	boundaryDeploy := plannerParityResource("uid-deploy", "Deployment", "ns-a", "deploy-boundary")

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("deleted-pre", 10, models.EventTypeCreate, deletedPod),
		plannerParityEvent("deleted-in-window", 20, models.EventTypeDelete, deletedPod),
	}))
	require.NoError(t, engine.Flush(context.Background()))

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("service-35", 35, models.EventTypeCreate, hotService),
		plannerParityEvent("deploy-40", 40, models.EventTypeCreate, boundaryDeploy),
	}))

	engine.projection.mu.RLock()
	require.Empty(t, engine.projection.eventsByResourceUID)
	engine.projection.mu.RUnlock()

	namespaces, kinds, minTime, maxTime, err := engine.QueryExecutor().QueryDistinctMetadata(context.Background(), 20*1e9, 40*1e9)
	require.NoError(t, err)
	require.Equal(t, []string{"ns-a", "ns-b"}, namespaces)
	require.Equal(t, []string{"Deployment", "Pod", "Service"}, kinds)
	require.Equal(t, int64(10*1e9), minTime)
	require.Equal(t, int64(40*1e9), maxTime)
}

func TestQueryExecutor_MetadataFullRangeUsesProjectionStateWithoutHistoryFallback(t *testing.T) {
	projection, err := BuildProjection([]models.Event{
		{
			ID:        "pod-create",
			Timestamp: 10,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				UID:       "pod-1",
				Namespace: "default",
				Kind:      "Pod",
				Name:      "pod-1",
			},
		},
		{
			ID:        "deploy-create",
			Timestamp: 20,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				UID:       "deploy-1",
				Namespace: "apps",
				Kind:      "Deployment",
				Name:      "deploy-1",
			},
		},
	})
	require.NoError(t, err)

	executor := NewQueryExecutor(projection)
	executor.DisableProjectionHistoryFallback()

	namespaces, kinds, minTime, maxTime, err := executor.QueryDistinctMetadata(context.Background(), 0, 30)
	require.NoError(t, err)
	require.Equal(t, []string{"apps", "default"}, namespaces)
	require.Equal(t, []string{"Deployment", "Pod"}, kinds)
	require.Equal(t, int64(10), minTime)
	require.Equal(t, int64(20), maxTime)
}
