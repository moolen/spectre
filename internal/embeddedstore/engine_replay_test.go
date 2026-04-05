package embeddedstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestEngine_OpenReplaysActiveSegmentsInGlobalOrder(t *testing.T) {
	dir := t.TempDir()
	rootDir := embeddedRootDir(dir)

	segmentOne, err := writeSegment(rootDir, "seg-one", []models.Event{
		testReplayEvent("a", 10),
		testReplayEvent("c", 30),
	})
	require.NoError(t, err)

	segmentTwo, err := writeSegment(rootDir, "seg-two", []models.Event{
		testReplayEvent("b", 20),
		testReplayEvent("d", 30),
	})
	require.NoError(t, err)

	require.NoError(t, storeManifest(rootDir, Manifest{
		FormatVersion: storageFormatVersion,
		ActiveSegments: []SegmentMeta{
			{ID: segmentOne.ID, HighWaterMark: 1},
			{ID: segmentTwo.ID, HighWaterMark: 2},
		},
		Checkpoints: []CheckpointMeta{},
	}))

	appliedIDs := make([]string, 0, 4)
	restoreApplyFn := setApplyProjectionEventFnForTest(func(projection *Projection, event models.Event) error {
		appliedIDs = append(appliedIDs, event.ID)
		return projection.Apply(event)
	})
	t.Cleanup(restoreApplyFn)

	engine, err := OpenEngine(EngineConfig{DataDir: dir, HotMaxEvents: 100, HotMaxResourceVersions: 4})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	require.Equal(t, []string{"a", "b", "c", "d"}, appliedIDs)
}

func TestEngine_OpenBuildsProjectionFromReplaySegmentsWithoutCheckpoint(t *testing.T) {
	dir := t.TempDir()
	rootDir := embeddedRootDir(dir)

	segmentOne, err := writeSegment(rootDir, "seg-one", []models.Event{
		testReplayEvent("a", 10),
		testReplayEvent("c", 30),
	})
	require.NoError(t, err)

	segmentTwo, err := writeSegment(rootDir, "seg-two", []models.Event{
		testReplayEvent("b", 20),
		testReplayEvent("d", 40),
	})
	require.NoError(t, err)

	require.NoError(t, storeManifest(rootDir, Manifest{
		FormatVersion: storageFormatVersion,
		ActiveSegments: []SegmentMeta{
			{ID: segmentOne.ID, HighWaterMark: 1},
			{ID: segmentTwo.ID, HighWaterMark: 2},
		},
		Checkpoints: []CheckpointMeta{},
	}))

	engine, err := OpenEngine(EngineConfig{DataDir: dir, HotMaxEvents: 100, HotMaxResourceVersions: 4})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	snapshot := engine.projection.SnapshotEvents()
	require.Len(t, snapshot, 4)
	require.Equal(t, []string{"a", "b", "c", "d"}, []string{snapshot[0].ID, snapshot[1].ID, snapshot[2].ID, snapshot[3].ID})
	require.Empty(t, engine.projection.eventsByResourceUID)
	require.Len(t, engine.projection.resourcesByUID["uid-a"].versions, 1)
	require.Len(t, engine.projection.resourcesByUID["uid-b"].versions, 1)
	require.Len(t, engine.projection.resourcesByUID["uid-c"].versions, 1)
	require.Len(t, engine.projection.resourcesByUID["uid-d"].versions, 1)
}

func TestEngine_OpenUsesCheckpointAndTailOnNormalRestart(t *testing.T) {
	dir := t.TempDir()
	expectedEvent := seedStoreWithCheckpointAndTail(t, dir)

	restore := setApplyProjectionEventFnForTest(func(*Projection, models.Event) error {
		t.Fatal("normal restart should not apply cold segment replay")
		return nil
	})
	t.Cleanup(restore)

	engine, err := OpenEngine(EngineConfig{DataDir: dir, HotMaxEvents: 100, HotMaxResourceVersions: 4})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	require.True(t, engine.IsReady())
	record := engine.projection.resourcesByUID[expectedEvent.Resource.UID]
	require.NotNil(t, record)
	require.Len(t, record.versions, 1)
	require.Equal(t, expectedEvent.ID, record.versions[0].eventID)
	require.Equal(t, []string{expectedEvent.ID}, replayTestEventIDs(engine.hot.ScanTimeRange(expectedEvent.Timestamp, expectedEvent.Timestamp)))
}

func TestEngine_OpenFallsBackToRepairReplayWhenTailStateIsStale(t *testing.T) {
	dir := t.TempDir()
	rootDir := embeddedRootDir(dir)
	checkpointEvent, repairEvent := seedStoreWithCheckpointAndStaleTail(t, dir)

	appliedIDs := make([]string, 0, 2)
	restore := setApplyProjectionEventFnForTest(func(projection *Projection, event models.Event) error {
		appliedIDs = append(appliedIDs, event.ID)
		return projection.Apply(event)
	})
	t.Cleanup(restore)

	engine, err := OpenEngine(EngineConfig{DataDir: dir, HotMaxEvents: 100, HotMaxResourceVersions: 4})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	require.True(t, engine.IsReady())
	require.Equal(t, []string{repairEvent.ID}, appliedIDs)

	record := engine.projection.resourcesByUID[checkpointEvent.Resource.UID]
	require.NotNil(t, record)
	require.Len(t, record.versions, 2)
	require.Equal(t, checkpointEvent.ID, record.versions[0].eventID)
	require.Equal(t, repairEvent.ID, record.versions[1].eventID)

	manifest, err := loadOrCreateManifest(rootDir)
	require.NoError(t, err)
	require.Equal(t, uint64(1), manifest.ActiveTail.LastHighWaterMark)
}

func TestEngine_OpenFailsWhenCheckpointIsCorrupt(t *testing.T) {
	dir := t.TempDir()
	rootDir := embeddedRootDir(dir)

	engine, err := OpenEngine(EngineConfig{DataDir: dir, HotMaxEvents: 100, HotMaxResourceVersions: 4})
	require.NoError(t, err)

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{testReplayEvent("checkpoint-corrupt", 1)}))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.Checkpoint(context.Background()))

	checkpointPath := filepath.Join(rootDir, checkpointsDirName, engine.manifest.ActiveCheckpoint.ID, checkpointStateFile)
	require.NoError(t, os.WriteFile(checkpointPath, []byte("{not-json"), 0o600))
	require.NoError(t, engine.tail.Close())

	_, err = OpenEngine(EngineConfig{DataDir: dir, HotMaxEvents: 100, HotMaxResourceVersions: 4})
	require.Error(t, err)
	require.ErrorContains(t, err, "load checkpoint")
	require.ErrorContains(t, err, "decode state file")
}

func TestEngine_OpenStopsReplayAfterApplyFailure(t *testing.T) {
	dir := t.TempDir()
	rootDir := embeddedRootDir(dir)

	failingSegment, err := writeSegment(rootDir, "seg-fail", []models.Event{
		testReplayEvent("fail-apply", 10),
	})
	require.NoError(t, err)

	corruptSegment, err := writeSegment(rootDir, "seg-corrupt", []models.Event{
		testReplayEvent("ok-before-corruption", 20),
		testReplayEvent("corrupt-later", 30),
	})
	require.NoError(t, err)

	require.NoError(t, storeManifest(rootDir, Manifest{
		FormatVersion: storageFormatVersion,
		ActiveSegments: []SegmentMeta{
			{ID: failingSegment.ID, HighWaterMark: 1},
			{ID: corruptSegment.ID, HighWaterMark: 2},
		},
		Checkpoints: []CheckpointMeta{},
	}))

	corruptEventsPath := filepath.Join(rootDir, segmentsDirName, corruptSegment.ID, segmentEventsFile)
	info, err := os.Stat(corruptEventsPath)
	require.NoError(t, err)
	require.NoError(t, os.Truncate(corruptEventsPath, info.Size()-8))

	restoreApplyFn := setApplyProjectionEventFnForTest(func(projection *Projection, event models.Event) error {
		if event.ID == "fail-apply" {
			return errors.New("boom")
		}
		return projection.Apply(event)
	})
	t.Cleanup(restoreApplyFn)

	_, err = OpenEngine(EngineConfig{DataDir: dir, HotMaxEvents: 100, HotMaxResourceVersions: 4})
	require.Error(t, err)
	require.ErrorContains(t, err, "boom")
}

func TestSegmentReplayCursor_KeepsFileOpenAcrossSequentialReads(t *testing.T) {
	reader := writeReplayTestSegment(t)
	cursor := newSegmentReplayCursor(replaySegmentReader{segmentID: "seg-1", reader: reader})

	first, ok, err := cursor.next(context.Background())
	require.NoError(t, err)
	require.True(t, ok)

	require.NoError(t, os.Remove(reader.eventsPath))

	second, ok, err := cursor.next(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
	require.NotEqual(t, first.ID, second.ID)
}

func testReplayEvent(id string, timestamp int64) models.Event {
	return models.Event{
		ID:        id,
		Timestamp: timestamp,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			UID:       "uid-" + id,
			Namespace: "default",
			Kind:      "Pod",
			Version:   "v1",
			Name:      "pod-" + id,
		},
		Data: []byte(`{"kind":"Pod","metadata":{"namespace":"default"}}`),
	}
}

func eventIDsForUID(events []models.Event) []string {
	ids := make([]string, 0, len(events))
	for i := range events {
		ids = append(ids, events[i].ID)
	}
	return ids
}

func replayTestEventIDs(events []models.Event) []string {
	ids := make([]string, 0, len(events))
	for i := range events {
		ids = append(ids, events[i].ID)
	}
	return ids
}

func seedStoreWithCheckpointAndTail(t *testing.T, dataDir string) models.Event {
	t.Helper()

	engine, err := OpenEngine(EngineConfig{DataDir: dataDir, HotMaxEvents: 100, HotMaxResourceVersions: 4})
	require.NoError(t, err)

	checkpointed := testReplayEvent("checkpointed", 1)
	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{checkpointed}))
	require.NoError(t, engine.Checkpoint(context.Background()))

	tailEvent := testReplayEvent("tail-overlap", 2)
	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{tailEvent}))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.tail.Close())

	return tailEvent
}

func seedStoreWithCheckpointAndStaleTail(t *testing.T, dataDir string) (models.Event, models.Event) {
	t.Helper()

	rootDir := embeddedRootDir(dataDir)
	engine, err := OpenEngine(EngineConfig{DataDir: dataDir, HotMaxEvents: 100, HotMaxResourceVersions: 4})
	require.NoError(t, err)

	checkpointEvent := testReplayEvent("repair-checkpointed", 1)
	checkpointEvent.Resource.UID = "repair-shared"
	checkpointEvent.Resource.Name = "repair-shared"
	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{checkpointEvent}))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.Checkpoint(context.Background()))

	repairEvent := testReplayEvent("repair-segment", 2)
	repairEvent.Resource.UID = checkpointEvent.Resource.UID
	repairEvent.Resource.Name = checkpointEvent.Resource.Name
	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{repairEvent}))
	require.NoError(t, engine.Flush(context.Background()))

	staleManifest := engine.manifest
	staleManifest.ActiveTail.LastHighWaterMark = staleManifest.ActiveCheckpoint.HighWaterMark
	staleManifest.ActiveTail.EventCount = 0
	staleManifest.ActiveTail.SizeBytes = 0
	require.NoError(t, storeManifest(rootDir, staleManifest))
	require.NoError(t, engine.tail.Close())

	return checkpointEvent, repairEvent
}

func writeReplayTestSegment(t *testing.T) *segmentReader {
	t.Helper()

	rootDir := t.TempDir()
	segment, err := writeSegment(rootDir, "seg-replay", []models.Event{
		testReplayEvent("first", 10),
		testReplayEvent("second", 20),
	})
	require.NoError(t, err)

	reader, err := openSegmentReader(rootDir, segment)
	require.NoError(t, err)
	return reader
}
