package sync

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetrics_NewMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)

	require.NotNil(t, metrics)
	assert.NotNil(t, metrics.StateCacheHits)
	assert.NotNil(t, metrics.StateCacheMisses)
	assert.NotNil(t, metrics.StateCacheSize)
	assert.NotNil(t, metrics.LabelIndexHits)
	assert.NotNil(t, metrics.LabelIndexMisses)
	assert.NotNil(t, metrics.LabelIndexSize)
	assert.NotNil(t, metrics.EventsProcessed)
	assert.NotNil(t, metrics.ProcessingErrors)
}

func TestMetrics_StateCacheMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)

	// Record cache operations
	metrics.RecordStateCacheHit()
	metrics.RecordStateCacheHit()
	metrics.RecordStateCacheMiss()
	metrics.UpdateStateCacheStats(2, 1, 100)

	// Gather metrics to verify values
	families, err := reg.Gather()
	require.NoError(t, err)

	var foundHits, foundMisses, foundSize bool
	for _, family := range families {
		switch *family.Name {
		case "spectre_graph_sync_state_cache_hits_total":
			foundHits = true
			assert.Equal(t, 2.0, *family.Metric[0].Counter.Value)
		case "spectre_graph_sync_state_cache_misses_total":
			foundMisses = true
			assert.Equal(t, 1.0, *family.Metric[0].Counter.Value)
		case "spectre_graph_sync_state_cache_size":
			foundSize = true
			assert.Equal(t, 100.0, *family.Metric[0].Gauge.Value)
		}
	}

	assert.True(t, foundHits, "Should have state_cache_hits metric")
	assert.True(t, foundMisses, "Should have state_cache_misses metric")
	assert.True(t, foundSize, "Should have state_cache_size metric")
}

func TestMetrics_LabelIndexMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)

	// Record label index operations
	metrics.RecordLabelIndexHit()
	metrics.RecordLabelIndexHit()
	metrics.RecordLabelIndexHit()
	metrics.RecordLabelIndexMiss()
	metrics.UpdateLabelIndexStats(3, 1, 10, 1000)

	// Gather metrics
	families, err := reg.Gather()
	require.NoError(t, err)

	var foundHits, foundMisses, foundSize, foundNs bool
	for _, family := range families {
		switch *family.Name {
		case "spectre_graph_sync_label_index_hits_total":
			foundHits = true
			assert.Equal(t, 3.0, *family.Metric[0].Counter.Value)
		case "spectre_graph_sync_label_index_misses_total":
			foundMisses = true
			assert.Equal(t, 1.0, *family.Metric[0].Counter.Value)
		case "spectre_graph_sync_label_index_size":
			foundSize = true
			assert.Equal(t, 1000.0, *family.Metric[0].Gauge.Value)
		case "spectre_graph_sync_label_index_namespaces":
			foundNs = true
			assert.Equal(t, 10.0, *family.Metric[0].Gauge.Value)
		}
	}

	assert.True(t, foundHits, "Should have label_index_hits metric")
	assert.True(t, foundMisses, "Should have label_index_misses metric")
	assert.True(t, foundSize, "Should have label_index_size metric")
	assert.True(t, foundNs, "Should have label_index_namespaces metric")
}

func TestMetrics_PipelineMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)

	// Record pipeline operations
	metrics.RecordEventProcessed()
	metrics.RecordEventProcessed()
	metrics.RecordEventSkipped()
	metrics.RecordNodesCreated(5)
	metrics.RecordEdgesCreated(3)
	metrics.RecordProcessingTime(0.001)
	metrics.RecordBatchProcessed(100, 0.5)
	metrics.RecordError()

	// Gather metrics
	families, err := reg.Gather()
	require.NoError(t, err)

	var foundProcessed, foundSkipped, foundNodes, foundEdges, foundErrors bool
	for _, family := range families {
		switch *family.Name {
		case "spectre_graph_sync_events_processed_total":
			foundProcessed = true
			assert.Equal(t, 2.0, *family.Metric[0].Counter.Value)
		case "spectre_graph_sync_events_skipped_total":
			foundSkipped = true
			assert.Equal(t, 1.0, *family.Metric[0].Counter.Value)
		case "spectre_graph_sync_nodes_created_total":
			foundNodes = true
			assert.Equal(t, 5.0, *family.Metric[0].Counter.Value)
		case "spectre_graph_sync_edges_created_total":
			foundEdges = true
			assert.Equal(t, 3.0, *family.Metric[0].Counter.Value)
		case "spectre_graph_sync_errors_total":
			foundErrors = true
			assert.Equal(t, 1.0, *family.Metric[0].Counter.Value)
		}
	}

	assert.True(t, foundProcessed, "Should have events_processed metric")
	assert.True(t, foundSkipped, "Should have events_skipped metric")
	assert.True(t, foundNodes, "Should have nodes_created metric")
	assert.True(t, foundEdges, "Should have edges_created metric")
	assert.True(t, foundErrors, "Should have errors metric")
}

func TestMetrics_Unregister(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)

	// Verify metrics are registered
	families, err := reg.Gather()
	require.NoError(t, err)
	assert.NotEmpty(t, families)

	// Unregister
	metrics.Unregister()

	// Verify metrics are unregistered (should be empty or have fewer metrics)
	families, err = reg.Gather()
	require.NoError(t, err)
	assert.Empty(t, families)
}

func TestMetrics_BatchMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)

	// Record multiple batches
	metrics.RecordBatchProcessed(50, 0.1)
	metrics.RecordBatchProcessed(100, 0.2)
	metrics.RecordBatchProcessed(200, 0.4)

	// Gather metrics
	families, err := reg.Gather()
	require.NoError(t, err)

	var foundBatchSize, foundBatchDuration bool
	for _, family := range families {
		switch *family.Name {
		case "spectre_graph_sync_batch_size":
			foundBatchSize = true
			// Histogram should have 3 samples
			assert.Equal(t, uint64(3), *family.Metric[0].Histogram.SampleCount)
		case "spectre_graph_sync_batch_duration_seconds":
			foundBatchDuration = true
			assert.Equal(t, uint64(3), *family.Metric[0].Histogram.SampleCount)
		}
	}

	assert.True(t, foundBatchSize, "Should have batch_size metric")
	assert.True(t, foundBatchDuration, "Should have batch_duration metric")
}
