package embeddedstore

import (
	"context"
	"errors"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

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
