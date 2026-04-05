package embeddedstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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

func TestEngine_StartPeriodicCheckpointPersistsRestartableStateWithoutFlush(t *testing.T) {
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
		return len(manifest.Checkpoints) >= 1
	}, time.Second, 10*time.Millisecond)

	manifest, err := loadOrCreateManifest(rootDir)
	require.NoError(t, err)
	require.Empty(t, manifest.ActiveSegments)
	require.NotEmpty(t, manifest.Checkpoints)
	require.Equal(t, uint64(0), manifest.FlushHighWaterMark)
	require.Equal(t, uint64(1), latestCheckpointMeta(manifest.Checkpoints).HighWaterMark)
	require.Equal(t, latestCheckpointMeta(manifest.Checkpoints).HighWaterMark, manifest.ActiveTail.BaseHighWaterMark)
	require.Zero(t, manifest.ActiveTail.EventCount)

	readers, tail, recoveredHighWaterMark, projection, recoveredHot, mode, replayedTailEvents, err := loadEngineState(rootDir, manifest)
	require.NoError(t, err)
	require.Empty(t, readers)
	require.Equal(t, startupModeFast, mode)
	require.Equal(t, 0, replayedTailEvents)
	require.Equal(t, uint64(1), recoveredHighWaterMark)
	require.NotNil(t, tail)
	require.Zero(t, tail.meta.EventCount)
	require.NotNil(t, projection)
	require.Contains(t, projection.resourcesByUID, "pod-periodic")
	require.NotNil(t, recoveredHot)
	require.Empty(t, recoveredHot.ExtractFlushBatch(0).Events)
	require.NoError(t, tail.Close())

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

func TestEngine_CheckpointDoesNotPersistWhenProjectionStateIsNotReady(t *testing.T) {
	dir := t.TempDir()

	engine, err := OpenEngine(EngineConfig{
		DataDir:                dir,
		HotMaxEvents:           100,
		HotMaxResourceVersions: 4,
		CheckpointOnShutdown:   true,
	})
	require.NoError(t, err)

	restoreApplyFn := setApplyProjectionEventFnForTest(func(projection *Projection, event models.Event) error {
		if event.ID == "fail-apply" {
			return errors.New("boom")
		}
		return projection.Apply(event)
	})
	t.Cleanup(restoreApplyFn)

	require.Error(t, engine.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "fail-apply",
			Timestamp: 10,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:   "v1",
				UID:       "pod-fail",
				Namespace: "default",
				Kind:      "Pod",
				Name:      "pod-fail",
			},
			Data: []byte(`{"kind":"Pod","metadata":{"name":"pod-fail","namespace":"default","uid":"pod-fail"}}`),
		},
	}))
	require.False(t, engine.IsReady())

	restoreApplyFn()
	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "ok-after-fail",
			Timestamp: 20,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:   "v1",
				UID:       "pod-ok",
				Namespace: "default",
				Kind:      "Pod",
				Name:      "pod-ok",
			},
			Data: []byte(`{"kind":"Pod","metadata":{"name":"pod-ok","namespace":"default","uid":"pod-ok"}}`),
		},
	}))
	require.False(t, engine.IsReady())
	require.Equal(t, 2, engine.tail.meta.EventCount)

	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.Checkpoint(context.Background()))

	manifest, err := loadOrCreateManifest(embeddedRootDir(dir))
	require.NoError(t, err)
	require.Empty(t, manifest.ActiveSegments)
	require.Empty(t, manifest.Checkpoints)

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
	require.Len(t, exported, 2)
}

func TestEngine_CloseSkipsCheckpointWhenProjectionStateIsNotReady(t *testing.T) {
	dir := t.TempDir()

	engine, err := OpenEngine(EngineConfig{
		DataDir:                dir,
		HotMaxEvents:           100,
		HotMaxResourceVersions: 4,
		CheckpointOnShutdown:   true,
	})
	require.NoError(t, err)

	restoreApplyFn := setApplyProjectionEventFnForTest(func(projection *Projection, event models.Event) error {
		if event.ID == "fail-apply" {
			return errors.New("boom")
		}
		return projection.Apply(event)
	})
	t.Cleanup(restoreApplyFn)

	require.Error(t, engine.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "fail-apply",
			Timestamp: 10,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:   "v1",
				UID:       "pod-fail",
				Namespace: "default",
				Kind:      "Pod",
				Name:      "pod-fail",
			},
			Data: []byte(`{"kind":"Pod","metadata":{"name":"pod-fail","namespace":"default","uid":"pod-fail"}}`),
		},
	}))
	require.False(t, engine.IsReady())

	restoreApplyFn()
	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "ok-after-fail",
			Timestamp: 20,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:   "v1",
				UID:       "pod-ok",
				Namespace: "default",
				Kind:      "Pod",
				Name:      "pod-ok",
			},
			Data: []byte(`{"kind":"Pod","metadata":{"name":"pod-ok","namespace":"default","uid":"pod-ok"}}`),
		},
	}))
	require.False(t, engine.IsReady())

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
	require.Len(t, exported, 2)
}

func TestEngine_AutoCheckpointWhenTailExceedsByteBudget(t *testing.T) {
	dir := t.TempDir()
	engine, err := OpenEngine(EngineConfig{
		DataDir:                 dir,
		HotMaxEvents:            100,
		HotMaxResourceVersions:  4,
		CheckpointMaxTailEvents: 1024,
		CheckpointMaxTailBytes:  1,
		CheckpointOnShutdown:    true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "tail-byte-budget",
			Timestamp: 10,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:   "v1",
				UID:       "pod-byte-budget",
				Namespace: "default",
				Kind:      "Pod",
				Name:      "pod-byte-budget",
			},
			Data: []byte(`{"kind":"Pod","metadata":{"name":"pod-byte-budget","namespace":"default","uid":"pod-byte-budget"},"spec":{"containers":[{"name":"app","image":"demo:v1"}]}}`),
		},
	}))

	require.NotEmpty(t, engine.manifest.Checkpoints)
	require.Equal(t, uint64(1), engine.manifest.ActiveCheckpoint.HighWaterMark)
	require.Equal(t, uint64(1), engine.manifest.ActiveTail.BaseHighWaterMark)
	require.Equal(t, uint64(1), engine.manifest.ActiveTail.LastHighWaterMark)
	require.Zero(t, engine.manifest.ActiveTail.EventCount)
	require.Zero(t, engine.tail.meta.EventCount)
}

func TestEngine_OpenTailReplayPreservesCheckpointedVersionMetadata(t *testing.T) {
	dir := t.TempDir()

	engine, err := OpenEngine(EngineConfig{DataDir: dir, HotMaxEvents: 100, HotMaxResourceVersions: 4})
	require.NoError(t, err)
	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "v1",
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
		{
			ID:        "v2",
			Timestamp: 20,
			Type:      models.EventTypeUpdate,
			Resource: models.ResourceMetadata{
				Version:   "v1",
				UID:       "pod-1",
				Namespace: "default",
				Kind:      "Pod",
				Name:      "pod-1",
			},
			Data: []byte(`{"kind":"Pod","metadata":{"name":"pod-1","namespace":"default","uid":"pod-1"},"spec":{"replicas":1}}`),
		},
	}))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.Checkpoint(context.Background()))
	require.NoError(t, engine.Close())

	rootDir := embeddedRootDir(dir)
	manifest, err := loadOrCreateManifest(rootDir)
	require.NoError(t, err)
	checkpointMeta := latestCheckpointMeta(manifest.Checkpoints)

	resourcesPath := filepath.Join(rootDir, checkpointsDirName, checkpointMeta.ID, checkpointResourcesFile)
	payload, err := os.ReadFile(resourcesPath)
	require.NoError(t, err)

	lines := bytes.Split(bytes.TrimSpace(payload), []byte{'\n'})
	require.Len(t, lines, 1)

	var snapshot ProjectionResourceSnapshot
	require.NoError(t, json.Unmarshal(lines[0], &snapshot))
	require.Len(t, snapshot.Versions, 2)
	snapshot.Versions[0].ChangeEvent.Description = "checkpoint-sentinel"

	updatedPayload, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(resourcesPath, append(updatedPayload, '\n'), 0o600))

	tailSegment, err := writeSegment(rootDir, "seg-tail", []models.Event{
		{
			ID:        "v3",
			Timestamp: 30,
			Type:      models.EventTypeUpdate,
			Resource: models.ResourceMetadata{
				Version:   "v1",
				UID:       "pod-1",
				Namespace: "default",
				Kind:      "Pod",
				Name:      "pod-1",
			},
			Data: []byte(`{"kind":"Pod","metadata":{"name":"pod-1","namespace":"default","uid":"pod-1"},"spec":{"replicas":2}}`),
		},
	})
	require.NoError(t, err)

	manifest.ActiveSegments = append(manifest.ActiveSegments, SegmentMeta{
		ID:            tailSegment.ID,
		HighWaterMark: 3,
	})
	manifest.FlushHighWaterMark = 3
	require.NoError(t, storeManifest(rootDir, manifest))

	reopened, err := OpenEngine(EngineConfig{DataDir: dir, HotMaxEvents: 100, HotMaxResourceVersions: 4})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, reopened.Close())
	})

	record := reopened.projection.resourcesByUID["pod-1"]
	require.NotNil(t, record)
	require.Len(t, record.versions, 3)
	require.Equal(t, "checkpoint-sentinel", record.versions[0].changeEvent.Description)
}

func TestEngine_ReopenRestoresTailEventsWithoutColdReplay(t *testing.T) {
	dir := t.TempDir()
	engine, err := OpenEngine(EngineConfig{DataDir: dir, CheckpointMaxTailEvents: 32})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	require.NoError(t, engine.ProcessBatch(context.Background(), makeReplayHeavyEvents(20)))
	require.NoError(t, engine.Checkpoint(context.Background()))
	require.NoError(t, engine.ProcessBatch(context.Background(), makeReplayHeavyEventsFrom(21, 5)))

	restore := setApplyProjectionEventFnForTest(func(*Projection, models.Event) error {
		t.Fatal("normal restart must not rebuild head state from cold replay")
		return nil
	})
	t.Cleanup(restore)

	reopened, err := OpenEngine(EngineConfig{DataDir: dir})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, reopened.Close())
	})

	require.Equal(t, uint64(25), reopened.nextHighWaterMark)
}

func TestEngine_ProcessBatchCancellationAfterTailAppendDoesNotWedgeIngest(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{DataDir: t.TempDir(), HotMaxEvents: 100, HotMaxResourceVersions: 4})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	event := makeReplayHeavyEventsFrom(1, 1)[0]
	ctx := &cancelAfterNErrChecksContext{
		Context:   context.Background(),
		remaining: 3,
		cancelErr: context.Canceled,
	}

	err = engine.ProcessBatch(ctx, []models.Event{event})
	require.NoError(t, err)
	require.Equal(t, uint64(1), engine.nextHighWaterMark)
	require.Equal(t, uint64(1), engine.tail.meta.LastHighWaterMark)

	record := engine.projection.resourcesByUID[event.Resource.UID]
	require.NotNil(t, record)
	require.Len(t, record.versions, 1)
	require.Len(t, engine.hot.ScanTimeRange(event.Timestamp, event.Timestamp), 1)

	nextEvent := makeReplayHeavyEventsFrom(2, 1)[0]
	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{nextEvent}))
	require.Equal(t, uint64(2), engine.nextHighWaterMark)
	require.Equal(t, uint64(2), engine.tail.meta.LastHighWaterMark)
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

func makeReplayHeavyEventsFrom(start, count int) []models.Event {
	events := make([]models.Event, 0, count)
	for i := 0; i < count; i++ {
		index := start + i - 1
		uid := fmt.Sprintf("pod-%04d", index)
		events = append(events, models.Event{
			ID:        fmt.Sprintf("evt-%04d", index),
			Timestamp: int64(index + 1),
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:   "v1",
				UID:       uid,
				Namespace: "default",
				Kind:      "Pod",
				Name:      uid,
			},
			Data: []byte(fmt.Sprintf(
				`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"%s","namespace":"default","uid":"%s"}}`,
				uid,
				uid,
			)),
		})
	}
	return events
}

type cancelAfterNErrChecksContext struct {
	context.Context

	mu        sync.Mutex
	remaining int
	cancelErr error
}

func (c *cancelAfterNErrChecksContext) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.remaining > 0 {
		c.remaining--
		return nil
	}
	return c.cancelErr
}
