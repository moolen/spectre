package victorialogs

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds Prometheus metrics for pipeline observability.
type Metrics struct {
	QueueDepth   prometheus.Gauge   // Current number of logs in pipeline buffer
	BatchesTotal prometheus.Counter // Total number of logs sent to VictoriaLogs
	ErrorsTotal  prometheus.Counter // Total number of pipeline errors

	// collectors holds references to all registered collectors for cleanup
	collectors []prometheus.Collector
	// registerer is the registry used for registration (needed for unregistration)
	registerer prometheus.Registerer
}

// NewMetrics creates Prometheus metrics for a VictoriaLogs pipeline instance.
// The registerer parameter allows flexible registration (e.g., global registry, test registry).
// The instanceName parameter enables multi-instance metric tracking via ConstLabels.
func NewMetrics(reg prometheus.Registerer, instanceName string) *Metrics {
	// Create QueueDepth gauge to track current buffer occupancy
	queueDepth := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "victorialogs_pipeline_queue_depth",
		Help:        "Current number of logs in pipeline buffer",
		ConstLabels: prometheus.Labels{"instance": instanceName},
	})

	// Create BatchesTotal counter to track total logs sent (not batch count!)
	batchesTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "victorialogs_pipeline_logs_total",
		Help:        "Total number of logs sent to VictoriaLogs",
		ConstLabels: prometheus.Labels{"instance": instanceName},
	})

	// Create ErrorsTotal counter to track pipeline failures
	errorsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "victorialogs_pipeline_errors_total",
		Help:        "Total number of pipeline errors",
		ConstLabels: prometheus.Labels{"instance": instanceName},
	})

	// Collect all metrics for registration and later cleanup
	collectors := []prometheus.Collector{queueDepth, batchesTotal, errorsTotal}

	// Register all metrics with the provided registerer
	reg.MustRegister(collectors...)

	return &Metrics{
		QueueDepth:   queueDepth,
		BatchesTotal: batchesTotal,
		ErrorsTotal:  errorsTotal,
		collectors:   collectors,
		registerer:   reg,
	}
}

// Unregister removes all metrics from the registry.
// This must be called before the integration is restarted to avoid duplicate registration panics.
func (m *Metrics) Unregister() {
	if m.registerer == nil {
		return
	}
	for _, c := range m.collectors {
		m.registerer.Unregister(c)
	}
}
