package sync

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds Prometheus metrics for graph sync pipeline observability.
type Metrics struct {
	// State cache metrics
	StateCacheHits   prometheus.Counter // State cache hit count
	StateCacheMisses prometheus.Counter // State cache miss count
	StateCacheSize   prometheus.Gauge   // Current number of entries in state cache

	// Label index metrics
	LabelIndexHits      prometheus.Counter // Label index hit count
	LabelIndexMisses    prometheus.Counter // Label index miss count
	LabelIndexSize      prometheus.Gauge   // Current number of resources in label index
	LabelIndexNamespaces prometheus.Gauge   // Number of namespaces in label index

	// Pipeline metrics
	EventsProcessed prometheus.Counter // Total events processed
	EventsSkipped   prometheus.Counter // Events skipped (no changes detected)
	NodesCreated    prometheus.Counter // Graph nodes created
	EdgesCreated    prometheus.Counter // Graph edges created
	ProcessingTime  prometheus.Histogram // Event processing duration

	// Batch metrics
	BatchSize     prometheus.Histogram // Batch size distribution
	BatchDuration prometheus.Histogram // Batch processing duration

	// Error metrics
	ProcessingErrors prometheus.Counter // Total processing errors

	// collectors holds references to all registered collectors for cleanup
	collectors []prometheus.Collector
	// registerer is the registry used for registration (needed for unregistration)
	registerer prometheus.Registerer
}

// NewMetrics creates Prometheus metrics for the graph sync pipeline.
// The registerer parameter allows flexible registration (e.g., global registry, test registry).
func NewMetrics(reg prometheus.Registerer) *Metrics {
	// State cache metrics
	stateCacheHits := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "spectre_graph_sync_state_cache_hits_total",
		Help: "Total number of state cache hits during change detection",
	})

	stateCacheMisses := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "spectre_graph_sync_state_cache_misses_total",
		Help: "Total number of state cache misses during change detection",
	})

	stateCacheSize := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "spectre_graph_sync_state_cache_size",
		Help: "Current number of resource states in the LRU cache",
	})

	// Label index metrics
	labelIndexHits := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "spectre_graph_sync_label_index_hits_total",
		Help: "Total number of label index hits during selector lookups",
	})

	labelIndexMisses := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "spectre_graph_sync_label_index_misses_total",
		Help: "Total number of label index misses during selector lookups",
	})

	labelIndexSize := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "spectre_graph_sync_label_index_size",
		Help: "Current number of resources indexed by labels",
	})

	labelIndexNamespaces := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "spectre_graph_sync_label_index_namespaces",
		Help: "Number of namespaces in the label index",
	})

	// Pipeline metrics
	eventsProcessed := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "spectre_graph_sync_events_processed_total",
		Help: "Total number of events processed by the sync pipeline",
	})

	eventsSkipped := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "spectre_graph_sync_events_skipped_total",
		Help: "Total number of events skipped (no changes detected)",
	})

	nodesCreated := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "spectre_graph_sync_nodes_created_total",
		Help: "Total number of graph nodes created",
	})

	edgesCreated := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "spectre_graph_sync_edges_created_total",
		Help: "Total number of graph edges created",
	})

	processingTime := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "spectre_graph_sync_event_processing_seconds",
		Help:    "Time spent processing individual events",
		Buckets: prometheus.ExponentialBuckets(0.0001, 2, 15), // 0.1ms to ~1.6s
	})

	// Batch metrics
	batchSize := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "spectre_graph_sync_batch_size",
		Help:    "Distribution of batch sizes",
		Buckets: prometheus.ExponentialBuckets(1, 2, 10), // 1 to 512
	})

	batchDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "spectre_graph_sync_batch_duration_seconds",
		Help:    "Time spent processing entire batches",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 15), // 1ms to ~16s
	})

	// Error metrics
	processingErrors := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "spectre_graph_sync_errors_total",
		Help: "Total number of errors during event processing",
	})

	// Collect all metrics
	collectors := []prometheus.Collector{
		stateCacheHits,
		stateCacheMisses,
		stateCacheSize,
		labelIndexHits,
		labelIndexMisses,
		labelIndexSize,
		labelIndexNamespaces,
		eventsProcessed,
		eventsSkipped,
		nodesCreated,
		edgesCreated,
		processingTime,
		batchSize,
		batchDuration,
		processingErrors,
	}

	// Register all metrics
	reg.MustRegister(collectors...)

	return &Metrics{
		StateCacheHits:      stateCacheHits,
		StateCacheMisses:    stateCacheMisses,
		StateCacheSize:      stateCacheSize,
		LabelIndexHits:      labelIndexHits,
		LabelIndexMisses:    labelIndexMisses,
		LabelIndexSize:      labelIndexSize,
		LabelIndexNamespaces: labelIndexNamespaces,
		EventsProcessed:     eventsProcessed,
		EventsSkipped:       eventsSkipped,
		NodesCreated:        nodesCreated,
		EdgesCreated:        edgesCreated,
		ProcessingTime:      processingTime,
		BatchSize:           batchSize,
		BatchDuration:       batchDuration,
		ProcessingErrors:    processingErrors,
		collectors:          collectors,
		registerer:          reg,
	}
}

// Unregister removes all metrics from the registry.
// This must be called before the pipeline is restarted to avoid duplicate registration panics.
func (m *Metrics) Unregister() {
	if m.registerer == nil {
		return
	}
	for _, c := range m.collectors {
		m.registerer.Unregister(c)
	}
}

// UpdateStateCacheStats updates state cache metrics from cache statistics.
func (m *Metrics) UpdateStateCacheStats(hits, misses int64, size int) {
	// We track deltas, so these should be called periodically
	// For now, we set the current values directly
	m.StateCacheSize.Set(float64(size))
}

// UpdateLabelIndexStats updates label index metrics from index statistics.
func (m *Metrics) UpdateLabelIndexStats(hits, misses int64, namespaces, resources int) {
	m.LabelIndexSize.Set(float64(resources))
	m.LabelIndexNamespaces.Set(float64(namespaces))
}

// RecordEventProcessed records a successfully processed event.
func (m *Metrics) RecordEventProcessed() {
	m.EventsProcessed.Inc()
}

// RecordEventSkipped records a skipped event.
func (m *Metrics) RecordEventSkipped() {
	m.EventsSkipped.Inc()
}

// RecordNodesCreated records the number of nodes created.
func (m *Metrics) RecordNodesCreated(count int) {
	m.NodesCreated.Add(float64(count))
}

// RecordEdgesCreated records the number of edges created.
func (m *Metrics) RecordEdgesCreated(count int) {
	m.EdgesCreated.Add(float64(count))
}

// RecordProcessingTime records the time taken to process an event.
func (m *Metrics) RecordProcessingTime(seconds float64) {
	m.ProcessingTime.Observe(seconds)
}

// RecordBatchProcessed records batch processing metrics.
func (m *Metrics) RecordBatchProcessed(batchSize int, durationSeconds float64) {
	m.BatchSize.Observe(float64(batchSize))
	m.BatchDuration.Observe(durationSeconds)
}

// RecordError records a processing error.
func (m *Metrics) RecordError() {
	m.ProcessingErrors.Inc()
}

// RecordStateCacheHit records a state cache hit.
func (m *Metrics) RecordStateCacheHit() {
	m.StateCacheHits.Inc()
}

// RecordStateCacheMiss records a state cache miss.
func (m *Metrics) RecordStateCacheMiss() {
	m.StateCacheMisses.Inc()
}

// RecordLabelIndexHit records a label index hit.
func (m *Metrics) RecordLabelIndexHit() {
	m.LabelIndexHits.Inc()
}

// RecordLabelIndexMiss records a label index miss.
func (m *Metrics) RecordLabelIndexMiss() {
	m.LabelIndexMisses.Inc()
}
