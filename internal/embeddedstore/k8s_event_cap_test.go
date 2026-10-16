package embeddedstore

import (
	"context"
	"fmt"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/require"
)

func TestK8sEventInfos_CappedPerObjectWithSegmentFallback(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{DataDir: t.TempDir(), HotMaxEvents: 1000, HotMaxResourceVersions: 32})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	pod := plannerParityResource("pod-noisy", "Pod", "default", "noisy")
	events := []models.Event{plannerParityEvent("pod-create", 1, models.EventTypeCreate, pod)}
	const total = maxK8sEventInfosPerUID + 36
	for i := 0; i < total; i++ {
		events = append(events, plannerParityK8sEvent(
			fmt.Sprintf("evt-%03d", i), fmt.Sprintf("event-%03d", i), fmt.Sprintf("noisy.%03d", i), "default", "pod-noisy",
			int64(10+i), "BackOff", "restarting", "Warning", 1, "kubelet",
		))
	}
	require.NoError(t, engine.ProcessBatch(context.Background(), events))
	require.NoError(t, engine.Flush(context.Background()))

	engine.projection.mu.RLock()
	require.Len(t, engine.projection.k8sEventsByInvolvedUID["pod-noisy"], maxK8sEventInfosPerUID)
	require.Equal(t, int64(10+total-maxK8sEventInfosPerUID-1)*1e9, engine.projection.k8sEventsTruncatedThrough["pod-noisy"])
	engine.projection.mu.RUnlock()

	full, _, err := engine.QueryExecutor().ExecutePaginated(context.Background(), &models.QueryRequest{StartTimestamp: 0, EndTimestamp: 1000}, nil)
	require.NoError(t, err)
	require.Len(t, full.K8sEventsByResource["pod-noisy"], total, "windows older than the cap read the segment index")

	recent, _, err := engine.QueryExecutor().ExecutePaginated(context.Background(), &models.QueryRequest{StartTimestamp: int64(10 + total - 10), EndTimestamp: 1000}, nil)
	require.NoError(t, err)
	require.Len(t, recent.K8sEventsByResource["pod-noisy"], 10, "recent windows are served from memory")
}
