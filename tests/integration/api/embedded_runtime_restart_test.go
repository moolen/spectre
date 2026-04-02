package api

import (
	"context"
	"testing"

	"github.com/moolen/spectre/internal/embeddedstore"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedRuntimeRestartPersistsDataInDataDir(t *testing.T) {
	dir := t.TempDir()

	backend1, err := embeddedstore.Open(embeddedstore.Config{DataDir: dir})
	require.NoError(t, err)

	require.NoError(t, backend1.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "restart-pod",
			Timestamp: 100,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Version:   "v1",
				Name:      "restart-pod",
				Namespace: "default",
				UID:       "restart-pod-uid",
			},
		},
	}))
	require.NoError(t, backend1.Close())

	backend2, err := embeddedstore.Open(embeddedstore.Config{DataDir: dir})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = backend2.Close()
	})

	server := newEmbeddedRuntimeServer(t, backend2)
	response := queryEmbeddedTimeline(t, server, 0, 1000)
	resource := findResource(response.Resources, "Pod", "restart-pod")
	require.NotNil(t, resource)
	require.True(t, backend2.IsReady())
}
