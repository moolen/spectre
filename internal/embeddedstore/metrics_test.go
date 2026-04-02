package embeddedstore

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
)

func TestMetrics_NewMetricsRegistersCollectors(t *testing.T) {
	reg := prometheus.NewRegistry()

	metrics := NewMetrics(reg)

	require.NotNil(t, metrics)

	families, err := reg.Gather()
	require.NoError(t, err)
	require.NotEmpty(t, families)
	require.NotNil(t, findMetricFamily(t, families, "spectre_embedded_query_total"))
}

func TestMetrics_RecordsWritePathState(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)

	metrics.RecordIngest(3, 25*time.Millisecond, nil)
	metrics.RecordIngest(0, 10*time.Millisecond, assertiveError("boom"))
	metrics.SetHotEvents(7)
	metrics.RecordHotEvictions("global", 2)
	metrics.RecordHotEvictions("uid", 1)
	metrics.RecordFlush(12*time.Millisecond, 5, 2048, nil)
	metrics.SetActiveSegments(3)
	metrics.RecordCheckpoint(8*time.Millisecond, nil)
	metrics.RecordCompaction(15*time.Millisecond, nil)

	families, err := reg.Gather()
	require.NoError(t, err)

	require.Equal(t, 2.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_ingest_batches_total"), nil))
	require.Equal(t, 3.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_ingest_events_total"), nil))
	require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_ingest_errors_total"), nil))
	require.Equal(t, 7.0, gaugeValue(t, findMetricFamily(t, families, "spectre_embedded_hot_events"), nil))
	require.Equal(t, 2.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_hot_evictions_total"), map[string]string{"scope": "global"}))
	require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_hot_evictions_total"), map[string]string{"scope": "uid"}))
	require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_flush_total"), nil))
	require.Equal(t, 5.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_flush_events_total"), nil))
	require.Equal(t, 2048.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_flush_bytes_total"), nil))
	require.Equal(t, 3.0, gaugeValue(t, findMetricFamily(t, families, "spectre_embedded_active_segments"), nil))
	require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_checkpoint_total"), nil))
	require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_compaction_total"), nil))
	require.Equal(t, uint64(2), histogramCount(t, findMetricFamily(t, families, "spectre_embedded_ingest_duration_seconds"), nil))
	require.Equal(t, uint64(1), histogramCount(t, findMetricFamily(t, families, "spectre_embedded_flush_duration_seconds"), nil))
}

func TestMetrics_RecordsReadPathState(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)

	metrics.RecordQuery("resource_events", "mixed", 18*time.Millisecond, nil)
	metrics.RecordQuery("export_time_range", "cold_only", 31*time.Millisecond, assertiveError("scan failed"))
	metrics.RecordSegmentScans("export_time_range", 4)
	metrics.RecordHotScans("export_time_range", 1)
	metrics.RecordUIDDiskLookups("resource_events", 2)

	families, err := reg.Gather()
	require.NoError(t, err)

	require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_query_total"), map[string]string{
		"query_family": "resource_events",
		"store_mix":    "mixed",
		"result":       "success",
	}))
	require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_query_total"), map[string]string{
		"query_family": "export_time_range",
		"store_mix":    "cold_only",
		"result":       "error",
	}))
	require.Equal(t, uint64(1), histogramCount(t, findMetricFamily(t, families, "spectre_embedded_query_duration_seconds"), map[string]string{
		"query_family": "resource_events",
		"store_mix":    "mixed",
		"result":       "success",
	}))
	require.Equal(t, 4.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_segment_scans_total"), map[string]string{"query_family": "export_time_range"}))
	require.Equal(t, 1.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_hot_scans_total"), map[string]string{"query_family": "export_time_range"}))
	require.Equal(t, 2.0, counterValue(t, findMetricFamily(t, families, "spectre_embedded_uid_disk_lookups_total"), map[string]string{"query_family": "resource_events"}))
}

func TestMetrics_Unregister(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)

	metrics.Unregister()

	families, err := reg.Gather()
	require.NoError(t, err)
	require.Empty(t, families)
}

type assertiveError string

func (e assertiveError) Error() string {
	return string(e)
}

func findMetricFamily(t *testing.T, families []*dto.MetricFamily, name string) *dto.MetricFamily {
	t.Helper()

	for _, family := range families {
		if family.GetName() == name {
			return family
		}
	}
	t.Fatalf("metric family %q not found", name)
	return nil
}

func counterValue(t *testing.T, family *dto.MetricFamily, labels map[string]string) float64 {
	t.Helper()

	metric := findMetric(t, family, labels)
	require.NotNil(t, metric.Counter)
	return metric.Counter.GetValue()
}

func gaugeValue(t *testing.T, family *dto.MetricFamily, labels map[string]string) float64 {
	t.Helper()

	metric := findMetric(t, family, labels)
	require.NotNil(t, metric.Gauge)
	return metric.Gauge.GetValue()
}

func histogramCount(t *testing.T, family *dto.MetricFamily, labels map[string]string) uint64 {
	t.Helper()

	metric := findMetric(t, family, labels)
	require.NotNil(t, metric.Histogram)
	return metric.Histogram.GetSampleCount()
}

func findMetric(t *testing.T, family *dto.MetricFamily, labels map[string]string) *dto.Metric {
	t.Helper()

	for _, metric := range family.Metric {
		if labelsMatch(metric.GetLabel(), labels) {
			return metric
		}
	}
	t.Fatalf("metric %q with labels %v not found", family.GetName(), labels)
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
