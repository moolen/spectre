package api

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/moolen/spectre/internal/embeddedstore"
	"github.com/moolen/spectre/internal/models"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedRuntimeFastRestartServesTimelineImmediately(t *testing.T) {
	dir := t.TempDir()
	seedEmbeddedRuntimeForFastRestart(t, dir)

	manifestBeforeReopen := readEmbeddedManifest(t, dir)
	require.Len(t, manifestBeforeReopen.Checkpoints, 1)
	require.Equal(t, uint64(1), manifestBeforeReopen.ActiveCheckpoint.HighWaterMark)
	require.Equal(t, uint64(1), manifestBeforeReopen.ActiveTail.BaseHighWaterMark)
	require.Equal(t, uint64(2), manifestBeforeReopen.ActiveTail.LastHighWaterMark)
	require.Equal(t, 1, manifestBeforeReopen.ActiveTail.EventCount)

	reg := prometheus.NewRegistry()
	engine, err := embeddedstore.OpenEngine(embeddedstore.EngineConfig{
		DataDir:                dir,
		HotMaxEvents:           32,
		HotMaxResourceVersions: 8,
		MetricsRegisterer:      reg,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = engine.Close()
	})

	require.True(t, engine.IsReady())

	families, err := reg.Gather()
	require.NoError(t, err)
	require.Equal(t, 1.0, gaugeValueFromFamilies(t, families, "spectre_embedded_startup_mode", map[string]string{"mode": "fast"}))
	require.Equal(t, 0.0, gaugeValueFromFamilies(t, families, "spectre_embedded_startup_mode", map[string]string{"mode": "repair"}))
	require.Equal(t, 1.0, counterValueFromFamilies(t, families, "spectre_embedded_tail_replayed_events_total", nil))
	require.Equal(t, 1.0, gaugeValueFromFamilies(t, families, "spectre_embedded_active_tail_events", nil))

	manifestAfterReopen := readEmbeddedManifest(t, dir)
	require.Equal(t, manifestBeforeReopen, manifestAfterReopen)

	server := newEmbeddedRuntimeServer(t, engine)
	response := queryEmbeddedTimeline(t, server, 0, 1_000_000)
	require.NotEmpty(t, response.Resources)

	resource := findResource(response.Resources, "Pod", "fast-restart-pod")
	require.NotNil(t, resource)
	require.Len(t, resource.StatusSegments, 2)
}

func seedEmbeddedRuntimeForFastRestart(t *testing.T, dir string) {
	t.Helper()

	engine, err := embeddedstore.OpenEngine(embeddedstore.EngineConfig{
		DataDir:                dir,
		HotMaxEvents:           32,
		HotMaxResourceVersions: 8,
		CheckpointOnShutdown:   false,
	})
	require.NoError(t, err)

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "fast-restart-create",
			Timestamp: 100,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Version:   "v1",
				Name:      "fast-restart-pod",
				Namespace: "default",
				UID:       "fast-restart-pod-uid",
			},
			Data: []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"fast-restart-pod","namespace":"default","uid":"fast-restart-pod-uid"}}`),
		},
	}))
	require.NoError(t, engine.Checkpoint(context.Background()))
	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		{
			ID:        "fast-restart-update",
			Timestamp: 110,
			Type:      models.EventTypeUpdate,
			Resource: models.ResourceMetadata{
				Kind:      "Pod",
				Version:   "v1",
				Name:      "fast-restart-pod",
				Namespace: "default",
				UID:       "fast-restart-pod-uid",
			},
			Data: []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"fast-restart-pod","namespace":"default","uid":"fast-restart-pod-uid"},"spec":{"containers":[{"name":"app","image":"nginx:1.29"}]}}`),
		},
	}))
	require.NoError(t, engine.Close())
}

func readEmbeddedManifest(t *testing.T, dir string) embeddedstore.Manifest {
	t.Helper()

	payload, err := os.ReadFile(filepath.Join(dir, "embedded", "manifest.json"))
	require.NoError(t, err)

	var manifest embeddedstore.Manifest
	require.NoError(t, json.Unmarshal(payload, &manifest))
	return manifest
}

func counterValueFromFamilies(t *testing.T, families []*dto.MetricFamily, name string, labels map[string]string) float64 {
	t.Helper()

	metric := findMetricFromFamilies(t, families, name, labels)
	require.NotNil(t, metric.Counter)
	return metric.Counter.GetValue()
}

func gaugeValueFromFamilies(t *testing.T, families []*dto.MetricFamily, name string, labels map[string]string) float64 {
	t.Helper()

	metric := findMetricFromFamilies(t, families, name, labels)
	require.NotNil(t, metric.Gauge)
	return metric.Gauge.GetValue()
}

func findMetricFromFamilies(t *testing.T, families []*dto.MetricFamily, name string, labels map[string]string) *dto.Metric {
	t.Helper()

	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.Metric {
			if labelsMatch(metric.GetLabel(), labels) {
				return metric
			}
		}
	}

	t.Fatalf("metric %q with labels %v not found", name, labels)
	return nil
}

func labelsMatch(pairs []*dto.LabelPair, want map[string]string) bool {
	if len(want) == 0 {
		return len(pairs) == 0
	}
	if len(pairs) != len(want) {
		return false
	}

	for _, pair := range pairs {
		if want[pair.GetName()] != pair.GetValue() {
			return false
		}
	}
	return true
}
