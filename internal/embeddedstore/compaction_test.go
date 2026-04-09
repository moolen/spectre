package embeddedstore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestCompaction_MergesOldSegmentsAndPreservesQueryResults(t *testing.T) {
	engine := newCompactionTestEngine(t, EngineConfig{
		HotMaxEvents:           128,
		HotMaxResourceVersions: 16,
		CompactionMinSegments:  2,
	})

	require.NoError(t, engine.ProcessBatch(context.Background(), makeCompactionTestEvents("seg-a", 0, 3)))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.ProcessBatch(context.Background(), makeCompactionTestEvents("seg-b", 100, 3)))
	require.NoError(t, engine.Flush(context.Background()))

	require.Len(t, engine.manifest.ActiveSegments, 2)
	oldSegmentIDs := []string{
		engine.manifest.ActiveSegments[0].ID,
		engine.manifest.ActiveSegments[1].ID,
	}

	before, err := engine.QueryExecutor().Execute(context.Background(), &models.QueryRequest{
		StartTimestamp: 0,
		EndTimestamp:   500,
	})
	require.NoError(t, err)

	require.NoError(t, engine.Compact(context.Background()))

	after, err := engine.QueryExecutor().Execute(context.Background(), &models.QueryRequest{
		StartTimestamp: 0,
		EndTimestamp:   500,
	})
	require.NoError(t, err)
	require.Equal(t, before.Events, after.Events)
	require.Len(t, engine.manifest.ActiveSegments, 1)

	compactedSegmentDir := filepath.Join(engine.rootDir, segmentsDirName, engine.manifest.ActiveSegments[0].ID)
	require.DirExists(t, compactedSegmentDir)
	for _, segmentID := range oldSegmentIDs {
		_, err := os.Stat(filepath.Join(engine.rootDir, segmentsDirName, segmentID))
		require.Truef(t, errors.Is(err, os.ErrNotExist), "segment %q should be removed after compaction", segmentID)
	}
}

func TestEngine_CheckpointAutoCompactsByDefault(t *testing.T) {
	engine := newCompactionTestEngine(t, EngineConfig{
		HotMaxEvents:           128,
		HotMaxResourceVersions: 16,
		CompactionMinSegments:  2,
	})

	require.NoError(t, engine.ProcessBatch(context.Background(), makeCompactionTestEvents("seg-a", 0, 3)))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.ProcessBatch(context.Background(), makeCompactionTestEvents("seg-b", 100, 3)))
	require.NoError(t, engine.Flush(context.Background()))

	require.Len(t, engine.manifest.ActiveSegments, 2)
	require.NoError(t, engine.Checkpoint(context.Background()))
	require.Len(t, engine.manifest.ActiveSegments, 1)
}

func TestEngine_CheckpointSkipsAutoCompactionWhenDisabled(t *testing.T) {
	engine := newCompactionTestEngine(t, EngineConfig{
		HotMaxEvents:           128,
		HotMaxResourceVersions: 16,
		CompactionMinSegments:  2,
		DisableAutoCompaction:  true,
	})

	require.NoError(t, engine.ProcessBatch(context.Background(), makeCompactionTestEvents("seg-a", 0, 3)))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.ProcessBatch(context.Background(), makeCompactionTestEvents("seg-b", 100, 3)))
	require.NoError(t, engine.Flush(context.Background()))

	require.Len(t, engine.manifest.ActiveSegments, 2)
	require.NoError(t, engine.Checkpoint(context.Background()))
	require.Len(t, engine.manifest.ActiveSegments, 2)
}

func TestEngine_CheckpointRetentionRewritesMixedSegmentsAndDropsExpiredSegments(t *testing.T) {
	now := time.Now().UTC()
	oldTimestamp := now.Add(-48 * time.Hour).UnixNano()
	retainedTimestamp := now.Add(-2 * time.Hour).UnixNano()

	engine := newCompactionTestEngine(t, EngineConfig{
		HotMaxEvents:           128,
		HotMaxResourceVersions: 16,
		CompactionMinSegments:  8,
		DisableAutoCompaction:  true,
		EmbeddedRetentionDays:  1,
	})

	require.NoError(t, engine.ProcessBatch(context.Background(), makeCompactionTestEvents("seg-old", oldTimestamp, 1)))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		makeCompactionSingleEvent("seg-mixed-old", oldTimestamp, "pod-mixed"),
		makeCompactionSingleEvent("seg-mixed-new", retainedTimestamp, "pod-mixed"),
	}))
	require.NoError(t, engine.Flush(context.Background()))

	require.Len(t, engine.manifest.ActiveSegments, 2)
	expiredSegmentID := engine.manifest.ActiveSegments[0].ID

	require.NoError(t, engine.Checkpoint(context.Background()))
	require.Len(t, engine.manifest.ActiveSegments, 1)

	exported, err := engine.QueryExecutor().ExportTimeRange(context.Background(), &models.QueryRequest{
		StartTimestamp: time.Unix(0, oldTimestamp).Add(-time.Hour).Unix(),
		EndTimestamp:   time.Unix(0, retainedTimestamp).Add(time.Hour).Unix(),
	})
	require.NoError(t, err)
	require.Equal(t, []string{"seg-mixed-new"}, checkpointEventIDs(exported))

	_, err = os.Stat(filepath.Join(engine.rootDir, segmentsDirName, expiredSegmentID))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestEngine_StartFlushesHotEventsByInterval(t *testing.T) {
	engine := newCompactionTestEngine(t, EngineConfig{
		HotMaxEvents:           128,
		HotMaxResourceVersions: 16,
		FlushInterval:          20 * time.Millisecond,
	})

	require.NoError(t, engine.Start(context.Background()))
	require.NoError(t, engine.ProcessBatch(context.Background(), makeCompactionTestEvents("interval", 0, 1)))

	require.Eventually(t, func() bool {
		engine.mu.Lock()
		segmentCount := len(engine.manifest.ActiveSegments)
		engine.mu.Unlock()
		return segmentCount == 1 && len(engine.hot.ExtractFlushBatch(0).Events) == 0
	}, time.Second, 10*time.Millisecond)
}

func TestEngine_StartStopsPeriodicFlushOnContextCancel(t *testing.T) {
	engine := newCompactionTestEngine(t, EngineConfig{
		HotMaxEvents:           128,
		HotMaxResourceVersions: 16,
		FlushInterval:          20 * time.Millisecond,
	})

	startCtx, cancel := context.WithCancel(context.Background())
	require.NoError(t, engine.Start(startCtx))
	cancel()

	require.NoError(t, engine.ProcessBatch(context.Background(), makeCompactionTestEvents("cancel", 0, 1)))
	time.Sleep(80 * time.Millisecond)

	engine.mu.Lock()
	defer engine.mu.Unlock()
	require.Empty(t, engine.manifest.ActiveSegments)
	require.Len(t, engine.hot.ExtractFlushBatch(0).Events, 1)
}

func TestEngine_ProcessBatchFlushesWhenSegmentTargetBytesExceeded(t *testing.T) {
	engine := newCompactionTestEngine(t, EngineConfig{
		HotMaxEvents:           128,
		HotMaxResourceVersions: 16,
		SegmentTargetBytes:     1,
	})

	require.NoError(t, engine.ProcessBatch(context.Background(), makeCompactionTestEvents("size", 0, 1)))

	engine.mu.Lock()
	defer engine.mu.Unlock()
	require.Len(t, engine.manifest.ActiveSegments, 1)
	require.Empty(t, engine.hot.ExtractFlushBatch(0).Events)
}

func TestEngine_ProcessBatchKeepsAcceptedEventsWhenSizeTriggeredFlushFails(t *testing.T) {
	engine := newCompactionTestEngine(t, EngineConfig{
		HotMaxEvents:           128,
		HotMaxResourceVersions: 16,
		SegmentTargetBytes:     1,
	})

	restoreSyncPath := setSyncPathFnForTest(func(path string) error {
		if strings.Contains(path, filepath.Join("embedded", segmentTempDirName)) {
			return errors.New("sync temp dir: injected failure")
		}
		return syncPathDirect(path)
	})
	defer restoreSyncPath()

	events := makeCompactionTestEvents("size-fail", 0, 1)
	require.NoError(t, engine.ProcessBatch(context.Background(), events))

	engine.mu.Lock()
	require.Empty(t, engine.manifest.ActiveSegments)
	engine.mu.Unlock()

	result, err := engine.QueryExecutor().Execute(context.Background(), &models.QueryRequest{
		StartTimestamp: 0,
		EndTimestamp:   10,
	})
	require.NoError(t, err)
	require.Len(t, result.Events, 1)
	require.Equal(t, events[0].ID, result.Events[0].ID)
}

func TestCompaction_KeepsUpdatedManifestWhenCleanupSyncFails(t *testing.T) {
	engine := newCompactionTestEngine(t, EngineConfig{
		HotMaxEvents:           128,
		HotMaxResourceVersions: 16,
		CompactionMinSegments:  2,
	})

	require.NoError(t, engine.ProcessBatch(context.Background(), makeCompactionTestEvents("seg-a", 0, 3)))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.ProcessBatch(context.Background(), makeCompactionTestEvents("seg-b", 100, 3)))
	require.NoError(t, engine.Flush(context.Background()))

	segmentsRoot := filepath.Join(engine.rootDir, segmentsDirName)
	segmentsSyncCalls := 0
	restoreSyncPath := setSyncPathFnForTest(func(path string) error {
		if path == segmentsRoot {
			segmentsSyncCalls++
			if segmentsSyncCalls == 2 {
				return errors.New("sync segments dir: injected failure")
			}
		}
		return syncPathDirect(path)
	})
	defer restoreSyncPath()

	err := engine.Compact(context.Background())
	require.EqualError(t, err, "compact embedded engine: sync segments dir: sync segments dir: injected failure")
	require.Len(t, engine.manifest.ActiveSegments, 1)

	require.NoError(t, engine.Checkpoint(context.Background()))
	require.NoError(t, engine.Close())

	reopened, openErr := OpenEngine(EngineConfig{DataDir: filepath.Dir(engine.rootDir), HotMaxEvents: 128, HotMaxResourceVersions: 16})
	require.NoError(t, openErr)
	require.NoError(t, reopened.Close())
}

func newCompactionTestEngine(t *testing.T, cfg EngineConfig) *Engine {
	t.Helper()

	cfg.DataDir = t.TempDir()
	engine, err := OpenEngine(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})
	return engine
}

func makeCompactionTestEvents(prefix string, startTimestamp int64, count int) []models.Event {
	events := make([]models.Event, 0, count)
	for i := 0; i < count; i++ {
		events = append(events, makeCompactionSingleEvent(
			prefix+"-"+time.Unix(0, startTimestamp+int64(i)).UTC().Format("150405.000000000"),
			startTimestamp+int64(i),
			"pod-1",
		))
	}
	return events
}

func makeCompactionSingleEvent(id string, timestamp int64, uid string) models.Event {
	return models.Event{
		ID:        id,
		Timestamp: timestamp,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Version:   "v1",
			UID:       uid,
			Namespace: "default",
			Kind:      "Pod",
			Name:      uid,
		},
		Data: []byte(fmt.Sprintf(`{"kind":"Pod","metadata":{"name":"%s","namespace":"default","uid":"%s"}}`, uid, uid)),
	}
}
