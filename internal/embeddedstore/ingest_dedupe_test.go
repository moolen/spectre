package embeddedstore

import (
	"context"
	"fmt"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func dedupeTestEvent(id string, ts int64, eventType models.EventType, uid, rv string) models.Event {
	return models.Event{
		ID:        id,
		Timestamp: ts,
		Type:      eventType,
		Resource: models.ResourceMetadata{
			Kind: "Pod", Version: "v1", Namespace: "default", Name: "pod-" + uid, UID: uid,
		},
		Data: []byte(fmt.Sprintf(`{"kind":"Pod","metadata":{"name":"pod-%s","uid":%q,"resourceVersion":%q},"status":{"phase":"Running"}}`, uid, uid, rv)),
	}
}

func dedupeTestK8sEvent(id string, ts int64, uid, rv string) models.Event {
	return models.Event{
		ID:        id,
		Timestamp: ts,
		Type:      models.EventTypeUpdate,
		Resource: models.ResourceMetadata{
			Kind: "Event", Version: "v1", Namespace: "default", Name: "ev-" + uid, UID: uid,
			InvolvedObjectUID: "pod-a",
		},
		Data: []byte(fmt.Sprintf(`{"kind":"Event","metadata":{"uid":%q,"resourceVersion":%q},"reason":"Pulled","message":"pulled","count":1}`, uid, rv)),
	}
}

func TestBackend_DropsEventsRepeatingLatestState(t *testing.T) {
	reg := prometheus.NewRegistry()
	backend, err := Open(Config{DataDir: t.TempDir(), MetricsRegisterer: reg})
	require.NoError(t, err)
	t.Cleanup(func() { _ = backend.Close() })
	ctx := context.Background()

	require.NoError(t, backend.ProcessBatch(ctx, []models.Event{
		dedupeTestEvent("a-1", 10, models.EventTypeCreate, "pod-a", "1"),
		dedupeTestEvent("b-1", 11, models.EventTypeCreate, "pod-b", "5"),
		dedupeTestK8sEvent("e-1", 12, "event-1", "7"),
	}))
	// A re-list repeats every object verbatim: nothing new is stored.
	require.NoError(t, backend.ProcessBatch(ctx, []models.Event{
		dedupeTestEvent("a-relist", 20, models.EventTypeCreate, "pod-a", "1"),
		dedupeTestEvent("b-relist", 21, models.EventTypeCreate, "pod-b", "5"),
		dedupeTestK8sEvent("e-relist", 22, "event-1", "7"),
	}))
	// Real changes, and a recreate after a delete, are kept.
	require.NoError(t, backend.ProcessBatch(ctx, []models.Event{
		dedupeTestEvent("a-2", 30, models.EventTypeUpdate, "pod-a", "2"),
		dedupeTestEvent("b-del", 31, models.EventTypeDelete, "pod-b", "5"),
		dedupeTestEvent("b-recreate", 32, models.EventTypeCreate, "pod-b", "5"),
		dedupeTestK8sEvent("e-2", 33, "event-1", "8"),
	}))
	require.Equal(t, 3.0, counterTotal(t, backend.engine.metrics.IngestUnchanged))
	require.Equal(t, 7.0, counterTotal(t, backend.engine.metrics.IngestEvents))

	queryIDs := func(kinds ...string) []string {
		result, err := backend.QueryExecutor().Execute(ctx, &models.QueryRequest{
			StartTimestamp: 0,
			EndTimestamp:   100,
			Filters:        models.QueryFilters{Kinds: kinds},
		})
		require.NoError(t, err)
		ids := make([]string, 0, len(result.Events))
		for _, event := range result.Events {
			ids = append(ids, event.ID)
		}
		return ids
	}
	require.ElementsMatch(t, []string{"a-1", "b-1", "a-2", "b-del", "b-recreate"}, queryIDs("Pod"))
	require.ElementsMatch(t, []string{"e-1", "e-2"}, queryIDs("Event"))
}

func TestEngine_KeepsUnchangedEventsUnlessEnabled(t *testing.T) {
	engine, err := OpenEngine(EngineConfig{DataDir: t.TempDir(), HotMaxEvents: 100, HotMaxResourceVersions: 8})
	require.NoError(t, err)
	t.Cleanup(func() { _ = engine.Close() })

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		dedupeTestEvent("a-1", 10, models.EventTypeCreate, "pod-a", "1"),
		dedupeTestEvent("a-2", 20, models.EventTypeUpdate, "pod-a", "1"),
	}))
	result, err := engine.QueryExecutor().Execute(context.Background(), &models.QueryRequest{StartTimestamp: 0, EndTimestamp: 100})
	require.NoError(t, err)
	require.Len(t, result.Events, 2)
}

func TestProjection_ForgetsOldK8sEventHashes(t *testing.T) {
	projection := NewProjection()
	projection.k8sEventHashPruneAt = 4
	hour := int64(k8sEventHashHorizon) / 2
	for i := 0; i < 4; i++ {
		require.NoError(t, projection.Apply(dedupeTestK8sEvent(fmt.Sprintf("e-%d", i), int64(i)*hour, fmt.Sprintf("event-%d", i), "1")))
	}
	// The fourth insert reached the prune threshold; entries older than the horizon
	// relative to it (event-0) are gone.
	require.Len(t, projection.k8sEventHashes, 3)
	require.NotContains(t, projection.k8sEventHashes, "event-0")
	require.GreaterOrEqual(t, projection.k8sEventHashPruneAt, 1024)
}

func counterTotal(t *testing.T, counter prometheus.Counter) float64 {
	t.Helper()
	var metric dto.Metric
	require.NoError(t, counter.Write(&metric))
	return metric.GetCounter().GetValue()
}
