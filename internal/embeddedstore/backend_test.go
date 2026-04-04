package embeddedstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestConfig_EffectiveEngineConfigAppliesDefaults(t *testing.T) {
	cfg := Config{DataDir: t.TempDir()}

	engineCfg, err := cfg.EffectiveEngineConfig()
	require.NoError(t, err)
	require.Equal(t, cfg.DataDir, engineCfg.DataDir)
	require.Equal(t, defaultHotMaxEvents, engineCfg.HotMaxEvents)
	require.Equal(t, defaultHotMaxResourceVersions, engineCfg.HotMaxResourceVersions)
	require.Equal(t, defaultFlushInterval, engineCfg.FlushInterval)
	require.Equal(t, defaultCheckpointInterval, engineCfg.CheckpointInterval)
	require.Equal(t, defaultSegmentTargetBytes, engineCfg.SegmentTargetBytes)
	require.Equal(t, defaultCompactionMinSegments, engineCfg.CompactionMinSegments)
}

func TestConfig_EffectiveEngineConfigRejectsInvalidValues(t *testing.T) {
	testCases := []struct {
		name   string
		cfg    Config
		substr string
	}{
		{
			name:   "empty data dir",
			cfg:    Config{},
			substr: "data dir is empty",
		},
		{
			name: "negative hot max events",
			cfg: Config{
				DataDir:      t.TempDir(),
				HotMaxEvents: -1,
			},
			substr: "hot max events must be positive",
		},
		{
			name: "negative flush interval",
			cfg: Config{
				DataDir:       t.TempDir(),
				FlushInterval: -1 * time.Second,
			},
			substr: "flush interval must be positive",
		},
		{
			name: "negative segment target bytes",
			cfg: Config{
				DataDir:            t.TempDir(),
				SegmentTargetBytes: -1,
			},
			substr: "segment target bytes must be positive",
		},
		{
			name: "compaction min segments below two",
			cfg: Config{
				DataDir:               t.TempDir(),
				CompactionMinSegments: 1,
			},
			substr: "compaction min segments must be at least 2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.cfg.EffectiveEngineConfig()
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.substr)
		})
	}
}

func TestBackend_ProcessBatchPersistsAcrossRestart(t *testing.T) {
	dir := t.TempDir()

	b1, err := Open(Config{DataDir: dir})
	require.NoError(t, err)

	require.NoError(t, b1.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "1",
			Timestamp: 1,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Version:   "v1",
				Name:      "pod-1",
				Namespace: "default",
				UID:       "pod-uid",
			},
		},
	}))
	require.NoError(t, b1.Close())

	b2, err := Open(Config{DataDir: dir})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = b2.Close()
	})

	result, err := b2.QueryExecutor().Execute(context.Background(), &models.QueryRequest{
		StartTimestamp: 0,
		EndTimestamp:   10,
		Filters:        models.QueryFilters{Kinds: []string{"Pod"}},
	})
	require.NoError(t, err)
	require.Len(t, result.Events, 1)
	require.True(t, b2.IsReady())
}

func TestBackend_ProcessEventUpdatesProjectionImmediately(t *testing.T) {
	dir := t.TempDir()

	backend, err := Open(Config{DataDir: dir})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = backend.Close()
	})

	require.NoError(t, backend.ProcessEvent(context.Background(), models.Event{
		ID:        "pod-create",
		Timestamp: 5,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Kind:      "Pod",
			Version:   "v1",
			Name:      "pod-live",
			Namespace: "default",
			UID:       "pod-live-uid",
		},
	}))

	result, err := backend.QueryExecutor().Execute(context.Background(), &models.QueryRequest{
		StartTimestamp: 0,
		EndTimestamp:   10,
		Filters:        models.QueryFilters{Kinds: []string{"Pod"}},
	})
	require.NoError(t, err)
	require.Len(t, result.Events, 1)

	resource, err := backend.AnalysisStore().GetResource(context.Background(), "pod-live-uid")
	require.NoError(t, err)
	require.NotNil(t, resource)
	require.Equal(t, "pod-live-uid", resource.UID)
}

func TestBackend_ReadinessStaysFalseAfterProjectionApplyFailure(t *testing.T) {
	dir := t.TempDir()

	backend, err := Open(Config{DataDir: dir})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = backend.Close()
	})

	restoreApplyFn := setApplyProjectionEventFnForTest(func(projection *Projection, event models.Event) error {
		if event.ID == "fail-apply" {
			return errors.New("boom")
		}
		return projection.Apply(event)
	})
	t.Cleanup(restoreApplyFn)

	err = backend.ProcessEvent(context.Background(), models.Event{
		ID:        "fail-apply",
		Timestamp: 5,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Kind:      "Pod",
			Version:   "v1",
			Name:      "pod-fail",
			Namespace: "default",
			UID:       "pod-fail-uid",
		},
	})
	require.Error(t, err)
	require.False(t, backend.IsReady())

	err = backend.ProcessEvent(context.Background(), models.Event{
		ID:        "ok-after-fail",
		Timestamp: 6,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Kind:      "Pod",
			Version:   "v1",
			Name:      "pod-ok",
			Namespace: "default",
			UID:       "pod-ok-uid",
		},
	})
	require.NoError(t, err)
	require.False(t, backend.IsReady())

	require.NoError(t, backend.Close())

	reopened, err := Open(Config{DataDir: dir})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = reopened.Close()
	})
	require.True(t, reopened.IsReady())
}

func TestBackend_HasUsableResourceState(t *testing.T) {
	t.Run("empty backend is not usable", func(t *testing.T) {
		backend, err := Open(Config{DataDir: t.TempDir()})
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = backend.Close()
		})

		require.False(t, backend.HasUsableResourceState())
	})

	t.Run("valid projected resource state is usable", func(t *testing.T) {
		backend, err := Open(Config{DataDir: t.TempDir()})
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = backend.Close()
		})

		require.NoError(t, backend.ProcessEvent(context.Background(), models.Event{
			ID:        "pod-create",
			Timestamp: 5,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Version:   "v1",
				Name:      "pod-live",
				Namespace: "default",
				UID:       "pod-live-uid",
			},
		}))

		require.True(t, backend.HasUsableResourceState())
	})

	t.Run("invalid events without projected resources stay unusable", func(t *testing.T) {
		backend, err := Open(Config{DataDir: t.TempDir()})
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = backend.Close()
		})

		require.NoError(t, backend.ProcessEvent(context.Background(), models.Event{
			ID:        "missing-uid",
			Timestamp: 5,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Version:   "v1",
				Name:      "pod-invalid",
				Namespace: "default",
			},
		}))

		require.False(t, backend.HasUsableResourceState())
	})
}
