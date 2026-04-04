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

	engine.projection.mu.Lock()
	deletedHistory := append([]models.Event(nil), engine.projection.eventsByResourceUID[deletedPod.UID]...)
	require.Len(t, deletedHistory, 2)
	// Mixed-state parity case: projection keeps only in-window delete, while planner
	// still has cold history containing the pre-window create event.
	engine.projection.eventsByResourceUID[deletedPod.UID] = deletedHistory[1:]
	engine.projection.mu.Unlock()

	namespaces, kinds, minTime, maxTime, err := engine.QueryExecutor().QueryDistinctMetadata(context.Background(), 20*1e9, 40*1e9)
	require.NoError(t, err)
	require.Equal(t, []string{"ns-a", "ns-b"}, namespaces)
	require.Equal(t, []string{"Deployment", "Pod", "Service"}, kinds)
	require.Equal(t, int64(10*1e9), minTime)
	require.Equal(t, int64(40*1e9), maxTime)
}
