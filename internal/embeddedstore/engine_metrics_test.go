package embeddedstore

import (
	"context"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestEngineMetrics_ProcessBatchAndFlush(t *testing.T) {
	reg := prometheus.NewRegistry()
	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           16,
		HotMaxResourceVersions: 8,
		MetricsRegisterer:      reg,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	events := []models.Event{
		testMetricsEvent("pod-a", "pod-a-1", 10),
		testMetricsEvent("pod-a", "pod-a-2", 20),
	}
	require.NoError(t, engine.ProcessBatch(context.Background(), events))

	families, err := reg.Gather()
	require.NoError(t, err)
	require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_ingest_batches_total"), nil))
	require.Equal(t, 2.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_ingest_events_total"), nil))
	require.Equal(t, 2.0, gaugeValue(t, findMetricFamily(t, families, "spectre_embedded_hot_events"), nil))

	require.NoError(t, engine.Flush(context.Background()))

	families, err = reg.Gather()
	require.NoError(t, err)
	require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_flush_total"), nil))
	require.Equal(t, 2.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_flush_events_total"), nil))
	require.Equal(t, 1.0, gaugeValue(t, findMetricFamily(t, families, "spectre_embedded_active_segments"), nil))
	require.Equal(t, 0.0, gaugeValue(t, findMetricFamily(t, families, "spectre_embedded_hot_events"), nil))
}

func TestHotStoreMetrics_RecordsUIDAndGlobalEvictions(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)
	store := newHotStore(HotStoreConfig{
		MaxEvents:           2,
		MaxResourceVersions: 1,
	}, metrics)

	store.Append([]models.Event{
		testMetricsEvent("pod-a", "pod-a-1", 10),
		testMetricsEvent("pod-a", "pod-a-2", 20),
		testMetricsEvent("pod-b", "pod-b-1", 30),
		testMetricsEvent("pod-c", "pod-c-1", 40),
	})

	families, err := reg.Gather()
	require.NoError(t, err)
	require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_hot_evictions_total"), map[string]string{"scope": "uid"}))
	require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_hot_evictions_total"), map[string]string{"scope": "global"}))
	require.Equal(t, 2.0, gaugeValue(t, findMetricFamily(t, families, "spectre_embedded_hot_events"), nil))
}

func TestEngineMetrics_CheckpointAndCompaction(t *testing.T) {
	reg := prometheus.NewRegistry()
	engine, err := OpenEngine(EngineConfig{
		DataDir:                t.TempDir(),
		HotMaxEvents:           16,
		HotMaxResourceVersions: 8,
		CompactionMinSegments:  2,
		MetricsRegisterer:      reg,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{testMetricsEvent("pod-a", "seg-a-1", 10)}))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{testMetricsEvent("pod-b", "seg-b-1", 20)}))
	require.NoError(t, engine.Flush(context.Background()))

	require.NoError(t, engine.Checkpoint(context.Background()))
	require.NoError(t, engine.Compact(context.Background()))

	families, err := reg.Gather()
	require.NoError(t, err)
	require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_checkpoint_total"), nil))
	require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_compaction_total"), nil))
	require.Equal(t, 1.0, gaugeValue(t, findMetricFamily(t, families, "spectre_embedded_active_segments"), nil))
}

func testMetricsEvent(uid, id string, ts int64) models.Event {
	return models.Event{
		ID:        id,
		Timestamp: ts,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Version:   "v1",
			UID:       uid,
			Namespace: "default",
			Kind:      "Pod",
			Name:      uid,
		},
		Data: []byte(`{"kind":"Pod"}`),
	}
}
