package embeddedstore

import (
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
	require.Equal(t, []string{"a"}, eventIDsForUID(engine.projection.eventsByResourceUID["uid-a"]))
	require.Equal(t, []string{"b"}, eventIDsForUID(engine.projection.eventsByResourceUID["uid-b"]))
	require.Equal(t, []string{"c"}, eventIDsForUID(engine.projection.eventsByResourceUID["uid-c"]))
	require.Equal(t, []string{"d"}, eventIDsForUID(engine.projection.eventsByResourceUID["uid-d"]))
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
