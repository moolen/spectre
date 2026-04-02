package embeddedstore

import (
	"context"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestEngine_OpenLoadsCheckpointAndReplaysNewerSegments(t *testing.T) {
	dir := t.TempDir()

	engine1, err := OpenEngine(EngineConfig{DataDir: dir, HotMaxEvents: 100, HotMaxResourceVersions: 4})
	require.NoError(t, err)
	require.NoError(t, engine1.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "1",
			Timestamp: 10,
			Resource: models.ResourceMetadata{
				UID:       "pod-1",
				Namespace: "default",
				Kind:      "Pod",
				Name:      "pod-1",
			},
		},
	}))
	require.NoError(t, engine1.Flush(context.Background()))
	require.NoError(t, engine1.Checkpoint(context.Background()))
	require.NoError(t, engine1.Close())

	engine2, err := OpenEngine(EngineConfig{DataDir: dir, HotMaxEvents: 100, HotMaxResourceVersions: 4})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, engine2.Close())
	}()

	result, err := engine2.QueryExecutor().Execute(context.Background(), &models.QueryRequest{
		StartTimestamp: 0,
		EndTimestamp:   100,
	})
	require.NoError(t, err)
	require.Len(t, result.Events, 1)
}
