package grafana

import (
	"github.com/moolen/spectre/internal/observatory"
)

// SignalRole is an alias for observatory.SignalRole.
// Represents the operational role of a metric in observability.
// Based on Google's Four Golden Signals (Latency, Traffic, Errors, Saturation)
// plus observability-specific extensions (Availability, Novelty).
type SignalRole = observatory.SignalRole

// Signal role constants - aliased from observatory package
const (
	// SignalAvailability indicates uptime/health metrics (up, kube_pod_status_phase)
	SignalAvailability = observatory.SignalAvailability

	// SignalLatency indicates response time/duration metrics (histogram_quantile, *_duration_*)
	SignalLatency = observatory.SignalLatency

	// SignalErrors indicates failure/error rate metrics (*_error_*, *_failed_*)
	SignalErrors = observatory.SignalErrors

	// SignalTraffic indicates throughput/request rate metrics (rate(*_total), *_count)
	SignalTraffic = observatory.SignalTraffic

	// SignalSaturation indicates resource utilization metrics (cpu, memory, disk)
	SignalSaturation = observatory.SignalSaturation

	// SignalChurn indicates workload churn/restarts (pod restarts, deployments)
	// Deprecated: use SignalNovelty instead (v1.5+)
	SignalChurn = observatory.SignalNovelty

	// SignalNovelty indicates change events/deployments (replaces Churn in v1.5)
	SignalNovelty = observatory.SignalNovelty

	// SignalUnknown indicates metrics that could not be classified
	SignalUnknown = observatory.SignalUnknown
)

// SignalAnchor links a Grafana metric to a classified signal role and K8s workload.
// Stored as graph node with TTL expiration via expires_at timestamp.
//
// Graph relationships:
// - (SignalAnchor)-[:EXTRACTED_FROM]->(Query) - links to Query node in dashboard graph
// - (SignalAnchor)-[:MONITORS]->(ResourceIdentity) - links to K8s workload if inferred
//
// Deduplication: Same metric+workload from multiple dashboards → highest quality wins
// Composite key: metric_name + workload_namespace + workload_name
//
// Note: This is a Grafana-specific extension of observatory.SignalAnchor with
// additional fields for dashboard/panel tracking.
type SignalAnchor struct {
	// MetricName is the PromQL metric name (e.g., "container_cpu_usage_seconds_total")
	MetricName string

	// Role is the classified signal role (Availability, Latency, Errors, etc.)
	Role SignalRole

	// Confidence is the classification confidence (0.0-1.0)
	// Layer 1 (hardcoded): 0.95
	// Layer 2 (PromQL structure): 0.85-0.9
	// Layer 3 (metric name patterns): 0.7-0.8
	// Layer 4 (panel title): 0.5
	// Layer 5 (unknown): 0.0
	Confidence float64

	// QualityScore is inherited from source dashboard (0.0-1.0)
	// Computed from: freshness, usage, alerting, ownership, completeness
	QualityScore float64

	// WorkloadNamespace is the K8s namespace (may be empty if unlinked)
	// Inferred from PromQL label selectors (namespace label)
	WorkloadNamespace string

	// WorkloadName is the K8s workload name (may be empty if unlinked)
	// Inferred from PromQL label selectors (deployment/app/service/job labels)
	WorkloadName string

	// DashboardUID is the source Grafana dashboard UID
	DashboardUID string

	// PanelID is the panel ID within the dashboard
	PanelID int

	// QueryID is the Cypher node ID for the Query node
	// Links SignalAnchor to dashboard graph structure
	QueryID string

	// SourceGrafana is the integration name for multi-source support
	// Allows same metric+workload to exist separately per Grafana instance
	SourceGrafana string

	// FirstSeen is the Unix timestamp when signal was first ingested
	FirstSeen int64

	// LastSeen is the Unix timestamp when signal was last refreshed
	// Updated on every dashboard sync
	LastSeen int64

	// ExpiresAt is the Unix timestamp when signal should expire
	// Set to LastSeen + 7 days (follows v1.4 TTL pattern)
	// Query-time filtering: WHERE expires_at > $now
	ExpiresAt int64
}

// ClassificationResult is an alias for observatory.ClassificationResult.
// Represents the output of layered signal classification.
type ClassificationResult = observatory.ClassificationResult

// WorkloadInference is an alias for observatory.WorkloadInference.
// Represents an inferred K8s workload from PromQL labels.
type WorkloadInference = observatory.WorkloadInference
