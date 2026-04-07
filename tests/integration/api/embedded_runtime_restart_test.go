package api

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/apiserver"
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
	require.NotEmpty(t, response.Resources)
	resource := findResource(response.Resources, "Pod", "restart-pod")
	require.NotNil(t, resource)
	require.True(t, backend2.IsReady())
}

func TestEmbeddedRuntimeRestartLoadsCheckpointWithColdSegments(t *testing.T) {
	dir := t.TempDir()

	engine, err := embeddedstore.OpenEngine(embeddedstore.EngineConfig{DataDir: dir})
	require.NoError(t, err)

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "checkpoint-pod-created",
			Timestamp: 200,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Version:   "v1",
				Name:      "checkpoint-pod",
				Namespace: "default",
				UID:       "checkpoint-pod-uid",
			},
		},
		{
			ID:        "checkpoint-pod-updated",
			Timestamp: 220,
			Type:      models.EventTypeUpdate,
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Version:   "v1",
				Name:      "checkpoint-pod",
				Namespace: "default",
				UID:       "checkpoint-pod-uid",
			},
		},
	}))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.Checkpoint(context.Background()))
	require.NoError(t, engine.Close())

	reopened, err := embeddedstore.OpenEngine(embeddedstore.EngineConfig{DataDir: dir})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = reopened.Close()
	})

	server := newEmbeddedRuntimeServer(t, reopened)
	requireRuntimeReady(t, server, true)

	response := queryEmbeddedTimeline(t, server, 0, 1000)
	require.NotEmpty(t, response.Resources)
	resource := findResource(response.Resources, "Pod", "checkpoint-pod")
	require.NotNil(t, resource)
	require.Len(t, resource.StatusSegments, 2)
	require.True(t, reopened.IsReady())
}

func TestEmbeddedRuntimeReadinessStaysTrueDuringFlushAndCheckpointCycles(t *testing.T) {
	engine, err := embeddedstore.OpenEngine(embeddedstore.EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           4096,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = engine.Close()
	})

	require.NoError(t, engine.ProcessBatch(context.Background(), makeReadinessRuntimeEvents(300, 2000)))

	server := newEmbeddedRuntimeServer(t, engine)
	requireRuntimeReady(t, server, true)

	assertRuntimeReadyDuringOperation(t, server, func() error {
		return engine.Flush(context.Background())
	})
	assertRuntimeReadyDuringOperation(t, server, func() error {
		return engine.Checkpoint(context.Background())
	})
}

func assertRuntimeReadyDuringOperation(t *testing.T, server *apiserver.Server, operation func() error) {
	t.Helper()

	done := make(chan error, 1)
	go func() {
		done <- operation()
	}()

	probed := 0
	deadline := time.After(5 * time.Second)
	for {
		select {
		case err := <-done:
			require.NoError(t, err)
			require.Greater(t, probed, 0)
			return
		case <-deadline:
			t.Fatal("timed out waiting for storage operation to finish")
		default:
			requireRuntimeReady(t, server, true)
			probed++
			time.Sleep(1 * time.Millisecond)
		}
	}
}

func makeReadinessRuntimeEvents(startTimestamp int64, count int) []models.Event {
	events := make([]models.Event, 0, count)
	payload := `{"kind":"Pod","metadata":{"namespace":"default"},"data":{"blob":"` + strings.Repeat("x", 4096) + `"}}`
	for i := 0; i < count; i++ {
		events = append(events, models.Event{
			ID:        fmt.Sprintf("ready-%d", i),
			Timestamp: startTimestamp + int64(i),
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Version:   "v1",
				Name:      fmt.Sprintf("ready-pod-%d", i),
				Namespace: "default",
				UID:       fmt.Sprintf("ready-pod-uid-%d", i),
			},
			Data: []byte(payload),
		})
	}
	return events
}
