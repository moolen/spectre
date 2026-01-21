package victorialogs

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics holds Prometheus metrics for pipeline observability.
type Metrics struct {
	QueueDepth   prometheus.Gauge   // Current number of logs in pipeline buffer
	BatchesTotal prometheus.Counter // Total number of logs sent to VictoriaLogs
	ErrorsTotal  prometheus.Counter // Total number of pipeline errors
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

	// Register all metrics with the provided registerer
	reg.MustRegister(queueDepth)
	reg.MustRegister(batchesTotal)
	reg.MustRegister(errorsTotal)

	return &Metrics{
		QueueDepth:   queueDepth,
		BatchesTotal: batchesTotal,
		ErrorsTotal:  errorsTotal,
	}
}
