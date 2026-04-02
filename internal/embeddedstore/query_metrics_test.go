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

func TestQueryMetrics_DistinctMetadataUsesProjectionStoreMix(t *testing.T) {
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
