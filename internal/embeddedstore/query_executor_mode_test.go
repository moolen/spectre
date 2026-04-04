package embeddedstore

import (
	"context"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestQueryExecutor_RejectsProjectionHistoryFallbackWhenDisabled(t *testing.T) {
	projection := NewProjection()
	qe := NewQueryExecutor(projection)
	qe.DisableProjectionHistoryFallback()

	_, _, err := qe.resourceEvents(context.Background(), "uid-1", models.ResourceMetadata{}, 0, 10)
	require.ErrorContains(t, err, "projection history fallback disabled")
}

func TestEngine_ProjectionHistoryFallbackModeKeepsPlannerDisabledAcrossRefreshes(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{
		DataDir:                   t.TempDir(),
		CompactionMinSegments:     2,
		ProjectionHistoryFallback: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	require.Nil(t, engine.QueryExecutor().planner)

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{{
		ID:        "event-1",
		Timestamp: 1,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			UID:       "pod-1",
			Namespace: "default",
			Kind:      "Pod",
			Name:      "pod-1",
			Version:   "v1",
		},
	}}))
	require.NoError(t, engine.Flush(context.Background()))
	require.Nil(t, engine.QueryExecutor().planner)

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{{
		ID:        "event-2",
		Timestamp: 2,
		Type:      models.EventTypeUpdate,
		Resource: models.ResourceMetadata{
			UID:       "pod-1",
			Namespace: "default",
			Kind:      "Pod",
			Name:      "pod-1",
			Version:   "v1",
		},
	}}))
	require.NoError(t, engine.Flush(context.Background()))
	require.Nil(t, engine.QueryExecutor().planner)

	require.NoError(t, engine.Compact(context.Background()))
	require.Nil(t, engine.QueryExecutor().planner)
}
