package embeddedstore

import (
	"context"
	"fmt"
	"testing"
	"time"

	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestProjection_TrimsK8sEventInfosToHorizon(t *testing.T) {
	projection := NewProjection()
	projection.SetK8sEventInfoHorizon(time.Hour)
	hour := int64(time.Hour / time.Second)

	for i := int64(0); i <= 10; i++ {
		require.NoError(t, projection.Apply(plannerParityK8sEvent(
			fmt.Sprintf("evt-%d", i), fmt.Sprintf("event-uid-%d", i), "evt", "ns-a", fmt.Sprintf("pod-%d", i%2),
			i*hour, "Pulled", "pulled", "Normal", 1, "kubelet",
		)))
	}

	projection.mu.RLock()
	defer projection.mu.RUnlock()
	// Summaries are trimmed in steps of an eighth of the horizon, so at most 9/8 of an
	// hour survives: the hour-10 and hour-9 events.
	cutoff := 10*hour*1e9 - int64(time.Hour)
	require.GreaterOrEqual(t, projection.k8sEventInfosTrimmedThrough, cutoff-int64(time.Hour)/8-1)
	for uid, infos := range projection.k8sEventsByInvolvedUID {
		for _, info := range infos {
			require.Greater(t, info.Timestamp.UnixNano(), projection.k8sEventInfosTrimmedThrough, uid)
		}
	}
	require.False(t, projection.k8sEventInfosComplete("pod-0", 0))
	require.True(t, projection.k8sEventInfosComplete("pod-0", projection.k8sEventInfosTrimmedThrough+1))
}

func TestEngine_ReadsK8sEventsOlderThanHorizonFromSegments(t *testing.T) {
	dir := t.TempDir()
	cfg := EngineConfig{
		DataDir:                dir,
		HotMaxEvents:           128,
		HotMaxResourceVersions: 32,
		K8sEventInfoHorizon:    time.Hour,
		DisableAutoCompaction:  true,
	}
	engine, err := OpenEngine(cfg)
	require.NoError(t, err)

	hour := int64(time.Hour / time.Second)
	pod := plannerParityResource("pod-a", "Pod", "ns-a", "pod-a")
	events := []models.Event{plannerParityEvent("pod-a-create", 0, models.EventTypeCreate, pod)}
	for i := int64(1); i <= 10; i++ {
		events = append(events, plannerParityK8sEvent(
			fmt.Sprintf("evt-%d", i), fmt.Sprintf("event-uid-%d", i), "evt", "ns-a", "pod-a",
			i*hour, "BackOff", fmt.Sprintf("restart %d", i), "Warning", 1, "kubelet",
		))
	}
	require.NoError(t, engine.ProcessBatch(context.Background(), events))
	require.NoError(t, engine.Flush(context.Background()))

	assertOldEventsVisible := func(engine *Engine) {
		t.Helper()
		engine.projection.mu.RLock()
		inMemory := len(engine.projection.k8sEventsByInvolvedUID["pod-a"])
		engine.projection.mu.RUnlock()
		require.Less(t, inMemory, 10, "old summaries should have left memory")

		// The timeline attaches the early events from segments.
		result, _, err := engine.QueryExecutor().ExecutePaginated(context.Background(),
			&models.QueryRequest{StartTimestamp: 0, EndTimestamp: 3 * hour, Filters: models.QueryFilters{Kinds: []string{"Pod"}}},
			&models.PaginationRequest{PageSize: 10})
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"evt-1", "evt-2", "evt-3"}, plannerParityK8sEventIDs(result.K8sEventsByResource["pod-a"]))

		// So does the analysis store, newest first.
		analysisEvents, err := engine.analysis.GetK8sEvents(context.Background(), []string{"pod-a"}, analysisstore.ResourceWindow{
			FailureTimestampNs: 3 * hour * 1e9,
			LookbackNs:         int64(3 * time.Hour),
		})
		require.NoError(t, err)
		ids := make([]string, 0, len(analysisEvents["pod-a"]))
		for _, event := range analysisEvents["pod-a"] {
			ids = append(ids, event.EventID)
		}
		require.Equal(t, []string{"evt-3", "evt-2", "evt-1"}, ids)
	}
	assertOldEventsVisible(engine)

	// The trim point survives a checkpoint and restart.
	require.NoError(t, engine.Checkpoint(context.Background()))
	require.NoError(t, engine.Close())
	reopened, err := OpenEngine(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })
	require.Greater(t, reopened.projection.k8sEventInfosTrimmedThrough, int64(0))
	assertOldEventsVisible(reopened)
}
