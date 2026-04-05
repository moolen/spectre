package api

import (
	"context"
	"testing"

	"github.com/moolen/spectre/internal/embeddedstore"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedRuntimeFastRestartServesTimelineImmediately(t *testing.T) {
	dir := t.TempDir()
	seedEmbeddedRuntimeForFastRestart(t, dir)

	backend, err := embeddedstore.Open(embeddedstore.Config{DataDir: dir})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = backend.Close()
	})

	require.True(t, backend.IsReady())

	server := newEmbeddedRuntimeServer(t, backend)
	response := queryEmbeddedTimeline(t, server, 0, 1_000_000)
	require.NotEmpty(t, response.Resources)

	resource := findResource(response.Resources, "Pod", "fast-restart-pod")
	require.NotNil(t, resource)
	require.Len(t, resource.StatusSegments, 2)
}

func seedEmbeddedRuntimeForFastRestart(t *testing.T, dir string) {
	t.Helper()

	engine, err := embeddedstore.OpenEngine(embeddedstore.EngineConfig{
		DataDir:                dir,
		HotMaxEvents:           32,
		HotMaxResourceVersions: 8,
		CheckpointOnShutdown:   false,
	})
	require.NoError(t, err)

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "fast-restart-create",
			Timestamp: 100,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Version:   "v1",
				Name:      "fast-restart-pod",
				Namespace: "default",
				UID:       "fast-restart-pod-uid",
			},
			Data: []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"fast-restart-pod","namespace":"default","uid":"fast-restart-pod-uid"}}`),
		},
	}))
	require.NoError(t, engine.Checkpoint(context.Background()))
	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "fast-restart-update",
			Timestamp: 110,
			Type:      models.EventTypeUpdate,
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Version:   "v1",
				Name:      "fast-restart-pod",
				Namespace: "default",
				UID:       "fast-restart-pod-uid",
			},
			Data: []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"fast-restart-pod","namespace":"default","uid":"fast-restart-pod-uid"},"spec":{"containers":[{"name":"app","image":"nginx:1.29"}]}}`),
		},
	}))
	require.NoError(t, engine.Close())
}
