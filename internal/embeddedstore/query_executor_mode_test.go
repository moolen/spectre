package embeddedstore

import (
	"context"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestQueryExecutor_ResourceTimelineStillUsesProjectionWhenFallbackDisabled(t *testing.T) {
	projection := NewProjection()
	require.NoError(t, projection.Apply(models.Event{
		ID:        "event-1",
		Timestamp: 1,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			UID:       "uid-1",
			Namespace: "default",
			Kind:      "Pod",
			Name:      "pod-1",
			Version:   "v1",
		},
		Data: []byte(`{"kind":"Pod","metadata":{"name":"pod-1","namespace":"default","uid":"uid-1"}}`),
	}))

	qe := NewQueryExecutor(projection)
	qe.DisableProjectionHistoryFallback()

	events, _, err := qe.resourceEvents(context.Background(), "uid-1", models.ResourceMetadata{
		UID:       "uid-1",
		Namespace: "default",
		Kind:      "Pod",
		Name:      "pod-1",
		Version:   "v1",
	}, 0, 10)
	require.NoError(t, err)
	require.Equal(t, []string{"event-1"}, eventIDs(events))
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
