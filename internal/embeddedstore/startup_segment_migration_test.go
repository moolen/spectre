package embeddedstore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestStartupMigration_RewritesLegacySegmentsWithAssociatedIndex(t *testing.T) {
	engine := newCompactionTestEngine(t, EngineConfig{
		HotMaxEvents:           128,
		HotMaxResourceVersions: 16,
	})

	require.NoError(t, engine.ProcessBatch(context.Background(), makeStartupMigrationEvents()))
	require.NoError(t, engine.Flush(context.Background()))
	require.Len(t, engine.manifest.ActiveSegments, 1)

	rootDir := embeddedRootDir(engine.config.DataDir)
	legacySegmentID := engine.manifest.ActiveSegments[0].ID
	legacyAssociatedIndexPath := filepath.Join(rootDir, segmentsDirName, legacySegmentID, segmentAssociatedUIDIndexFile)
	require.NoError(t, os.Remove(legacyAssociatedIndexPath))

	manifest, err := loadOrCreateManifest(rootDir)
	require.NoError(t, err)
	manifest.SegmentIndexGeneration = 0
	require.NoError(t, storeManifest(rootDir, manifest))
	require.NoError(t, engine.Close())

	reopened, err := OpenEngine(EngineConfig{
		DataDir:                engine.config.DataDir,
		HotMaxEvents:           128,
		HotMaxResourceVersions: 16,
	})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, reopened.Close())
	}()

	require.True(t, reopened.IsReady())
	require.Eventually(t, func() bool {
		reopened.mu.Lock()
		defer reopened.mu.Unlock()
		return reopened.manifest.SegmentIndexGeneration == associatedIndexGeneration &&
			len(reopened.manifest.ActiveSegments) == 1 &&
			reopened.manifest.ActiveSegments[0].ID != legacySegmentID
	}, 5*time.Second, 20*time.Millisecond)

	reopened.mu.Lock()
	activeReader := reopened.segmentReaders[0]
	reopened.mu.Unlock()

	events, indexed, err := activeReader.ScanAssociatedUIDs(context.Background(), []string{"pod-1"})
	require.NoError(t, err)
	require.True(t, indexed)
	require.Len(t, events, 1)
	require.Equal(t, "event-1", events[0].ID)
}

func TestStartupMigration_SkipsWhenGenerationAlreadyCurrent(t *testing.T) {
	engine := newCompactionTestEngine(t, EngineConfig{
		HotMaxEvents:           128,
		HotMaxResourceVersions: 16,
	})

	require.NoError(t, engine.ProcessBatch(context.Background(), makeStartupMigrationEvents()))
	require.NoError(t, engine.Flush(context.Background()))

	rootDir := embeddedRootDir(engine.config.DataDir)
	manifest, err := loadOrCreateManifest(rootDir)
	require.NoError(t, err)
	manifest.SegmentIndexGeneration = associatedIndexGeneration
	require.NoError(t, storeManifest(rootDir, manifest))
	require.NoError(t, engine.Close())

	reopened, err := OpenEngine(EngineConfig{
		DataDir:                engine.config.DataDir,
		HotMaxEvents:           128,
		HotMaxResourceVersions: 16,
	})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, reopened.Close())
	}()

	require.Len(t, reopened.manifest.ActiveSegments, 1)
	require.Equal(t, manifest.ActiveSegments[0].ID, reopened.manifest.ActiveSegments[0].ID)
	require.Equal(t, associatedIndexGeneration, reopened.manifest.SegmentIndexGeneration)
}

func TestStartupMigration_FailedRewriteLeavesLegacyManifest(t *testing.T) {
	engine := newCompactionTestEngine(t, EngineConfig{
		HotMaxEvents:           128,
		HotMaxResourceVersions: 16,
	})

	require.NoError(t, engine.ProcessBatch(context.Background(), makeStartupMigrationEvents()))
	require.NoError(t, engine.Flush(context.Background()))
	require.Len(t, engine.manifest.ActiveSegments, 1)

	rootDir := embeddedRootDir(engine.config.DataDir)
	legacySegmentID := engine.manifest.ActiveSegments[0].ID
	legacyAssociatedIndexPath := filepath.Join(rootDir, segmentsDirName, legacySegmentID, segmentAssociatedUIDIndexFile)
	require.NoError(t, os.Remove(legacyAssociatedIndexPath))

	manifest, err := loadOrCreateManifest(rootDir)
	require.NoError(t, err)
	manifest.SegmentIndexGeneration = 0
	require.NoError(t, storeManifest(rootDir, manifest))
	require.NoError(t, engine.Close())

	restoreSyncPath := setSyncPathFnForTest(func(path string) error {
		if filepath.Base(filepath.Dir(path)) == segmentTempDirName {
			return os.ErrPermission
		}
		return syncPathDirect(path)
	})
	defer restoreSyncPath()

	reopened, err := OpenEngine(EngineConfig{
		DataDir:                engine.config.DataDir,
		HotMaxEvents:           128,
		HotMaxResourceVersions: 16,
	})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, reopened.Close())
	}()
	require.True(t, reopened.IsReady())

	require.Eventually(t, func() bool {
		reloadedManifest, reloadErr := loadOrCreateManifest(rootDir)
		require.NoError(t, reloadErr)
		return reloadedManifest.SegmentIndexGeneration == 0 &&
			len(reloadedManifest.ActiveSegments) == 1 &&
			reloadedManifest.ActiveSegments[0].ID == legacySegmentID
	}, time.Second, 20*time.Millisecond)
}

func TestStartupMigration_OpenEngineReturnsBeforeBackgroundMigrationCompletes(t *testing.T) {
	engine := newCompactionTestEngine(t, EngineConfig{
		HotMaxEvents:           128,
		HotMaxResourceVersions: 16,
	})

	require.NoError(t, engine.ProcessBatch(context.Background(), makeStartupMigrationEvents()))
	require.NoError(t, engine.Flush(context.Background()))
	require.Len(t, engine.manifest.ActiveSegments, 1)

	rootDir := embeddedRootDir(engine.config.DataDir)
	legacySegmentID := engine.manifest.ActiveSegments[0].ID
	require.NoError(t, os.Remove(filepath.Join(rootDir, segmentsDirName, legacySegmentID, segmentAssociatedUIDIndexFile)))

	manifest, err := loadOrCreateManifest(rootDir)
	require.NoError(t, err)
	manifest.SegmentIndexGeneration = 0
	require.NoError(t, storeManifest(rootDir, manifest))
	require.NoError(t, engine.Close())

	started := make(chan struct{}, 1)
	unblock := make(chan struct{})
	restoreSyncPath := setSyncPathFnForTest(func(path string) error {
		if strings.Contains(path, filepath.Join("embedded", segmentTempDirName)) {
			select {
			case started <- struct{}{}:
			default:
			}
			<-unblock
		}
		return syncPathDirect(path)
	})
	defer restoreSyncPath()

	reopened, err := OpenEngine(EngineConfig{
		DataDir:                engine.config.DataDir,
		HotMaxEvents:           128,
		HotMaxResourceVersions: 16,
	})
	require.NoError(t, err)
	released := false
	defer func() {
		if !released {
			close(unblock)
		}
		require.NoError(t, reopened.Close())
	}()

	require.True(t, reopened.IsReady())
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background migration did not start")
	}

	result, queryErr := reopened.QueryExecutor().Execute(context.Background(), &models.QueryRequest{
		StartTimestamp: 0,
		EndTimestamp:   30,
	})
	require.NoError(t, queryErr)
	require.Len(t, result.Events, 1)

	close(unblock)
	released = true
	require.Eventually(t, func() bool {
		reopened.mu.Lock()
		defer reopened.mu.Unlock()
		return reopened.manifest.SegmentIndexGeneration == associatedIndexGeneration
	}, 5*time.Second, 20*time.Millisecond)
}

func TestStartupMigration_StartDoesNotBlockWhileBackgroundMigrationIsRunning(t *testing.T) {
	engine := newCompactionTestEngine(t, EngineConfig{
		HotMaxEvents:           128,
		HotMaxResourceVersions: 16,
	})

	require.NoError(t, engine.ProcessBatch(context.Background(), makeStartupMigrationEvents()))
	require.NoError(t, engine.Flush(context.Background()))
	require.Len(t, engine.manifest.ActiveSegments, 1)

	rootDir := embeddedRootDir(engine.config.DataDir)
	legacySegmentID := engine.manifest.ActiveSegments[0].ID
	require.NoError(t, os.Remove(filepath.Join(rootDir, segmentsDirName, legacySegmentID, segmentAssociatedUIDIndexFile)))

	manifest, err := loadOrCreateManifest(rootDir)
	require.NoError(t, err)
	manifest.SegmentIndexGeneration = 0
	require.NoError(t, storeManifest(rootDir, manifest))
	require.NoError(t, engine.Close())

	started := make(chan struct{}, 1)
	unblock := make(chan struct{})
	restoreSyncPath := setSyncPathFnForTest(func(path string) error {
		if strings.Contains(path, filepath.Join("embedded", segmentTempDirName)) {
			select {
			case started <- struct{}{}:
			default:
			}
			<-unblock
		}
		return syncPathDirect(path)
	})
	defer restoreSyncPath()

	reopened, err := OpenEngine(EngineConfig{
		DataDir:                engine.config.DataDir,
		HotMaxEvents:           128,
		HotMaxResourceVersions: 16,
	})
	require.NoError(t, err)
	released := false
	defer func() {
		if !released {
			close(unblock)
		}
		require.NoError(t, reopened.Close())
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background migration did not start")
	}

	startDone := make(chan error, 1)
	go func() {
		startDone <- reopened.Start(context.Background())
	}()

	select {
	case err := <-startDone:
		require.NoError(t, err)
	case <-time.After(200 * time.Millisecond):
		t.Fatal("engine start blocked on background migration")
	}

	close(unblock)
	released = true
	require.Eventually(t, func() bool {
		reopened.mu.Lock()
		defer reopened.mu.Unlock()
		return reopened.manifest.SegmentIndexGeneration == associatedIndexGeneration
	}, 5*time.Second, 20*time.Millisecond)
}

func makeStartupMigrationEvents() []models.Event {
	return []models.Event{
		{
			ID:        "pod-1-create",
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
			ID:        "event-1",
			Timestamp: 12,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:           "v1",
				Kind:              "Event",
				Namespace:         "default",
				Name:              "pod-1.17d49f7f4f",
				UID:               "event-1",
				InvolvedObjectUID: "pod-1",
			},
			Data: []byte(`{"reason":"BackOff","message":"restarting","type":"Warning","count":3,"source":{"component":"kubelet"}}`),
		},
	}
}
