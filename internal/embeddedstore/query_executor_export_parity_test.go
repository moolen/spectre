package embeddedstore

import (
	"context"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestQueryExecutor_ExportPlannerParity(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	podA := plannerParityResource("uid-a", "Pod", "ns-a", "pod-a")
	podB := plannerParityResource("uid-b", "Pod", "ns-a", "pod-b")

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("outside-low", 10, models.EventTypeCreate, podA),
		plannerParityEvent("b-20", 20, models.EventTypeCreate, podB),
		plannerParityEvent("z-40", 40, models.EventTypeUpdate, podB),
	}))
	require.NoError(t, engine.Flush(context.Background()))

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		plannerParityEvent("a-20", 20, models.EventTypeCreate, podA),
		plannerParityEvent("b-20", 20, models.EventTypeCreate, podB),
		plannerParityEvent("a-40", 40, models.EventTypeUpdate, podA),
		plannerParityEvent("outside-high", 41, models.EventTypeUpdate, podA),
	}))

	engine.projection.mu.Lock()
	engine.projection.events = nil
	engine.projection.mu.Unlock()

	exported, err := engine.QueryExecutor().ExportTimeRange(context.Background(), &models.QueryRequest{
		StartTimestamp: 20,
		EndTimestamp:   40,
		Filters: models.QueryFilters{
			Namespace: "ns-a",
			Kind:      "Pod",
		},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"a-20", "b-20", "a-40", "z-40"}, eventIDs(exported))
}
