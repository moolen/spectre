package embeddedstore

import (
	"context"
	"testing"

	"github.com/moolen/spectre/internal/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestQueryMetrics_ResourceEventStoreMixes(t *testing.T) {
	t.Run("hot only", func(t *testing.T) {
		reg := prometheus.NewRegistry()
		engine := newMetricsTestEngine(t, reg)
		require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
			testMetricsEvent("pod-hot", "hot-1", 10),
		}))

		_, err := engine.QueryExecutor().Execute(context.Background(), testMetricsQuery("default", "Pod", 0, 20))
		require.NoError(t, err)

		families, gatherErr := reg.Gather()
		require.NoError(t, gatherErr)
		require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_query_total"), map[string]string{
			"query_family": "resource_events",
			"store_mix":    "hot_only",
			"result":       "success",
		}))
	})

	t.Run("cold only", func(t *testing.T) {
		reg := prometheus.NewRegistry()
		engine := newMetricsTestEngine(t, reg)
		require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
			testMetricsEvent("pod-cold", "cold-1", 10),
		}))
		require.NoError(t, engine.Flush(context.Background()))

		_, err := engine.QueryExecutor().Execute(context.Background(), testMetricsQuery("default", "Pod", 0, 20))
		require.NoError(t, err)

		families, gatherErr := reg.Gather()
		require.NoError(t, gatherErr)
		require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_query_total"), map[string]string{
			"query_family": "resource_events",
			"store_mix":    "cold_only",
			"result":       "success",
		}))
		require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_uid_disk_lookups_total"), map[string]string{
			"query_family": "resource_events",
		}))
	})

	t.Run("mixed", func(t *testing.T) {
		reg := prometheus.NewRegistry()
		engine := newMetricsTestEngine(t, reg)
		require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
			testMetricsEvent("pod-mixed", "cold-1", 10),
		}))
		require.NoError(t, engine.Flush(context.Background()))
		require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
			testMetricsEvent("pod-mixed", "hot-1", 20),
		}))

		_, err := engine.QueryExecutor().Execute(context.Background(), testMetricsQuery("default", "Pod", 0, 30))
		require.NoError(t, err)

		families, gatherErr := reg.Gather()
		require.NoError(t, gatherErr)
		require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_query_total"), map[string]string{
			"query_family": "resource_events",
			"store_mix":    "mixed",
			"result":       "success",
		}))
	})
}

func TestQueryMetrics_ResourcePaginationStopsAfterPageWindow(t *testing.T) {
	reg := prometheus.NewRegistry()
	engine := newMetricsTestEngine(t, reg)

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		testMetricsEvent("pod-a", "cold-a", 10),
		testMetricsEvent("pod-b", "cold-b", 11),
		testMetricsEvent("pod-c", "cold-c", 12),
	}))
	require.NoError(t, engine.Flush(context.Background()))

	result, pagination, err := engine.QueryExecutor().ExecutePaginated(
		context.Background(),
		testMetricsQuery("default", "Pod", 0, 20),
		&models.PaginationRequest{PageSize: 1},
	)
	require.NoError(t, err)
	require.Equal(t, []string{"cold-a"}, eventIDs(result.Events))
	require.NotNil(t, pagination)
	require.True(t, pagination.HasMore)
	require.NotEmpty(t, pagination.NextCursor)

	families, gatherErr := reg.Gather()
	require.NoError(t, gatherErr)
	require.Equal(t, 2.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_uid_disk_lookups_total"), map[string]string{
		"query_family": "resource_events",
	}))
	require.Equal(t, 2.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_segment_scans_total"), map[string]string{
		"query_family": "resource_events",
	}))
}

func TestQueryMetrics_ResourceAttachmentsUseProjectionSummaries(t *testing.T) {
	reg := prometheus.NewRegistry()
	engine := newMetricsTestEngine(t, reg)

	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		testMetricsEvent("pod-attach", "cold-pod", 10),
		{
			ID:        "cold-k8s-event",
			Timestamp: 12 * 1e9,
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				Version:           "v1",
				Kind:              "Event",
				Namespace:         "default",
				Name:              "pod-attach.17d49f7f4f",
				UID:               "event-1",
				InvolvedObjectUID: "pod-attach",
			},
			Data: []byte(`{"reason":"BackOff","message":"restarting","type":"Warning","count":3,"source":{"component":"kubelet"}}`),
		},
	}))
	require.NoError(t, engine.Flush(context.Background()))

	result, _, err := engine.QueryExecutor().ExecutePaginated(
		context.Background(),
		testMetricsQuery("default", "Pod", 0, 20),
		&models.PaginationRequest{PageSize: 1},
	)
	require.NoError(t, err)
	require.Contains(t, result.K8sEventsByResource, "pod-attach")
	require.Len(t, result.K8sEventsByResource["pod-attach"], 1)
	require.Equal(t, "BackOff", result.K8sEventsByResource["pod-attach"][0].Reason)
	require.Equal(t, int32(3), result.K8sEventsByResource["pod-attach"][0].Count)
	require.Equal(t, "kubelet", result.K8sEventsByResource["pod-attach"][0].Source)

	families, gatherErr := reg.Gather()
	require.NoError(t, gatherErr)
	require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_uid_disk_lookups_total"), map[string]string{
		"query_family": "resource_events",
	}))
	require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_segment_scans_total"), map[string]string{
		"query_family": "resource_events",
	}))
}

func TestQueryMetrics_ExportTimeRangeRecordsStoreScans(t *testing.T) {
	reg := prometheus.NewRegistry()
	engine := newMetricsTestEngine(t, reg)
	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		testMetricsEvent("pod-export", "cold-1", 10),
	}))
	require.NoError(t, engine.Flush(context.Background()))
	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		testMetricsEvent("pod-export", "hot-1", 20),
	}))

	exported, err := engine.QueryExecutor().ExportTimeRange(context.Background(), testMetricsQuery("default", "Pod", 0, 30))
	require.NoError(t, err)
	require.Len(t, exported, 2)

	families, gatherErr := reg.Gather()
	require.NoError(t, gatherErr)
	require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_query_total"), map[string]string{
		"query_family": "export_time_range",
		"store_mix":    "mixed",
		"result":       "success",
	}))
	require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_segment_scans_total"), map[string]string{
		"query_family": "export_time_range",
	}))
	require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_hot_scans_total"), map[string]string{
		"query_family": "export_time_range",
	}))
}

func TestQueryMetrics_DistinctMetadataUsesProjectionFastPathForFullRange(t *testing.T) {
	reg := prometheus.NewRegistry()
	engine := newMetricsTestEngine(t, reg)
	require.NoError(t, engine.ProcessBatch(context.Background(), []models.Event{
		testMetricsEvent("pod-meta", "meta-1", 10),
	}))

	namespaces, kinds, _, _, err := engine.QueryExecutor().QueryDistinctMetadata(context.Background(), 0, 20)
	require.NoError(t, err)
	require.Equal(t, []string{"default"}, namespaces)
	require.Equal(t, []string{"Pod"}, kinds)

	families, gatherErr := reg.Gather()
	require.NoError(t, gatherErr)
	require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_query_total"), map[string]string{
		"query_family": "distinct_metadata",
		"store_mix":    "projection_only",
		"result":       "success",
	}))
}

func TestQueryMetrics_CompactProjectionFallbackRecordsProjectionOnly(t *testing.T) {
	dataDir := t.TempDir()

	bootstrapEngine, err := OpenEngine(EngineConfig{
		DataDir:                   dataDir,
		HotMaxEvents:              16,
		HotMaxResourceVersions:    8,
		ProjectionHistoryFallback: true,
	})
	require.NoError(t, err)
	require.NoError(t, bootstrapEngine.ProcessBatch(context.Background(), []models.Event{
		testMetricsEvent("pod-fallback", "cold-1", 10),
	}))
	require.NoError(t, bootstrapEngine.Flush(context.Background()))
	require.NoError(t, bootstrapEngine.Close())

	reg := prometheus.NewRegistry()
	engine, err := OpenEngine(EngineConfig{
		DataDir:                   dataDir,
		HotMaxEvents:              16,
		HotMaxResourceVersions:    8,
		MetricsRegisterer:         reg,
		ProjectionHistoryFallback: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
	})

	exported, err := engine.QueryExecutor().ExportTimeRange(context.Background(), testMetricsQuery("default", "Pod", 0, 20))
	require.NoError(t, err)
	require.Len(t, exported, 1)

	namespaces, kinds, _, _, err := engine.QueryExecutor().QueryDistinctMetadata(context.Background(), 0, 20*1e9)
	require.NoError(t, err)
	require.Equal(t, []string{"default"}, namespaces)
	require.Equal(t, []string{"Pod"}, kinds)

	families, gatherErr := reg.Gather()
	require.NoError(t, gatherErr)
	require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_query_total"), map[string]string{
		"query_family": "export_time_range",
		"store_mix":    "projection_only",
		"result":       "success",
	}))
	require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_query_total"), map[string]string{
		"query_family": "distinct_metadata",
		"store_mix":    "projection_only",
		"result":       "success",
	}))
}

func newMetricsTestEngine(t *testing.T, reg prometheus.Registerer) *Engine {
	t.Helper()

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
	return engine
}

func testMetricsQuery(namespace, kind string, start, end int64) *models.QueryRequest {
	return &models.QueryRequest{
		StartTimestamp: start,
		EndTimestamp:   end,
		Filters: models.QueryFilters{
			Namespace: namespace,
			Kind:      kind,
		},
	}
}
