package embeddedstore

import (
	"context"
	"errors"
	"testing"
	"time"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
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

func TestEngine_OpenRecoversCheckpointDiscoveredOnDiskWhenManifestLags(t *testing.T) {
	dir := t.TempDir()

	engine, err := OpenEngine(EngineConfig{DataDir: dir, HotMaxEvents: 100, HotMaxResourceVersions: 4})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "checkpoint-one",
			Timestamp: 10,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:   "v1",
				UID:       "pod-1",
				Namespace: "default",
				Kind:      "Pod",
				Name:      "pod-1",
			},
			Data: []byte(`{"kind":"Pod","metadata":{"name":"pod-1","namespace":"default","uid":"pod-1"}}`),
		},
	}))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.Checkpoint(context.Background()))

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "checkpoint-two",
			Timestamp: 20,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:   "v1",
				UID:       "pod-2",
				Namespace: "default",
				Kind:      "Pod",
				Name:      "pod-2",
			},
			Data: []byte(`{"kind":"Pod","metadata":{"name":"pod-2","namespace":"default","uid":"pod-2"}}`),
		},
	}))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.Checkpoint(context.Background()))

	rootDir := embeddedRootDir(dir)
	manifest, err := loadOrCreateManifest(rootDir)
	require.NoError(t, err)
	require.Len(t, manifest.Checkpoints, 2)

	olderCheckpoint := manifest.Checkpoints[0]
	newerCheckpoint := latestCheckpointMeta(manifest.Checkpoints)
	require.Greater(t, newerCheckpoint.HighWaterMark, olderCheckpoint.HighWaterMark)

	manifest.Checkpoints = []CheckpointMeta{olderCheckpoint}
	require.NoError(t, storeManifest(rootDir, manifest))

	restoreApplyFn := setApplyProjectionEventFnForTest(func(projection *Projection, event models.Event) error {
		if event.ID == "checkpoint-two" {
			return errors.New("forced replay failure")
		}
		return projection.Apply(event)
	})
	t.Cleanup(restoreApplyFn)

	reopened, err := OpenEngine(EngineConfig{DataDir: dir, HotMaxEvents: 100, HotMaxResourceVersions: 4})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, reopened.Close())
	})

	healedManifest, err := loadOrCreateManifest(rootDir)
	require.NoError(t, err)
	require.Contains(t, healedManifest.Checkpoints, newerCheckpoint)
}

func TestEngine_ExportReadsFlushedColdSegment(t *testing.T) {
	engine := newFlushedTestEngine(t, []models.Event{
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
	})

	result, err := engine.QueryExecutor().ExportTimeRange(context.Background(), &models.QueryRequest{
		StartTimestamp: 0,
		EndTimestamp:   20,
		Filters:        models.QueryFilters{},
	})
	require.NoError(t, err)
	require.NotEmpty(t, result)
}

func TestEngine_ExportDoesNotWaitOnProjectionLockWhenPlannerIsAvailable(t *testing.T) {
	engine := newFlushedTestEngine(t, []models.Event{
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
	})

	engine.projection.mu.Lock()
	defer engine.projection.mu.Unlock()
	done := make(chan error, 1)
	go func() {
		_, err := engine.QueryExecutor().ExportTimeRange(context.Background(), &models.QueryRequest{
			StartTimestamp: 0,
			EndTimestamp:   20,
		})
		done <- err
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("export blocked on projection lock")
	}
}

func TestEngine_AnalysisReadsSurviveFlushCheckpointRestart(t *testing.T) {
	dir := t.TempDir()
	engine1, err := OpenEngine(EngineConfig{DataDir: dir, HotMaxEvents: 100, HotMaxResourceVersions: 4})
	require.NoError(t, err)
	require.NoError(t, engine1.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "1",
			Timestamp: 10,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:   "v1",
				UID:       "pod-1",
				Namespace: "default",
				Kind:      "Pod",
				Name:      "pod-1",
			},
			Data: []byte(`{"kind":"Pod","metadata":{"name":"pod-1","namespace":"default","uid":"pod-1"}}`),
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

	resource, err := engine2.AnalysisStore().GetResource(context.Background(), "pod-1")
	require.NoError(t, err)
	require.NotNil(t, resource)
	require.Equal(t, "pod-1", resource.UID)

	window := analysisstore.ResourceWindow{
		FailureTimestampNs: 20,
		LookbackNs:         20,
	}
	changeEvents, err := engine2.AnalysisStore().GetChangeEvents(context.Background(), []string{"pod-1"}, window)
	require.NoError(t, err)
	require.NotEmpty(t, changeEvents["pod-1"])
}

func TestEngine_StartPeriodicCheckpointFlushesBeforePersisting(t *testing.T) {
	dir := t.TempDir()

	engine, err := OpenEngine(EngineConfig{
		DataDir:                dir,
		HotMaxEvents:           100,
		HotMaxResourceVersions: 4,
		FlushInterval:          time.Hour,
		CheckpointInterval:     20 * time.Millisecond,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	require.NoError(t, engine.Start(context.Background()))
	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "periodic-checkpoint",
			Timestamp: 10,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:   "v1",
				UID:       "pod-periodic",
				Namespace: "default",
				Kind:      "Pod",
				Name:      "pod-periodic",
			},
			Data: []byte(`{"kind":"Pod","metadata":{"name":"pod-periodic","namespace":"default","uid":"pod-periodic"}}`),
		},
	}))

	rootDir := embeddedRootDir(dir)
	require.Eventually(t, func() bool {
		manifest, err := loadOrCreateManifest(rootDir)
		if err != nil {
			return false
		}
		return len(manifest.ActiveSegments) == 1 && len(manifest.Checkpoints) >= 1
	}, time.Second, 10*time.Millisecond)

	manifest, err := loadOrCreateManifest(rootDir)
	require.NoError(t, err)
	require.Len(t, manifest.ActiveSegments, 1)
	require.NotEmpty(t, manifest.Checkpoints)
	require.Equal(t, manifest.FlushHighWaterMark, latestCheckpointMeta(manifest.Checkpoints).HighWaterMark)

	require.NoError(t, engine.Close())

	reopened, err := OpenEngine(EngineConfig{DataDir: dir, HotMaxEvents: 100, HotMaxResourceVersions: 4})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, reopened.Close())
	})

	exported, err := reopened.QueryExecutor().ExportTimeRange(context.Background(), &models.QueryRequest{
		StartTimestamp: 0,
		EndTimestamp:   100,
	})
	require.NoError(t, err)
	require.Len(t, exported, 1)
}

func newFlushedTestEngine(t *testing.T, events []models.Event) *Engine {
	t.Helper()

	engine, err := OpenEngine(EngineConfig{DataDir: t.TempDir(), HotMaxEvents: 100, HotMaxResourceVersions: 4})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	require.NoError(t, engine.ProcessBatch(context.Background(), events))
	require.NoError(t, engine.Flush(context.Background()))
	return engine
}
