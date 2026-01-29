package grafana

// SignalRole represents the operational role of a metric in observability.
// Based on Google's Four Golden Signals (Latency, Traffic, Errors, Saturation)
// plus observability-specific extensions (Availability, Churn, Novelty).
type SignalRole string

const (
	// SignalAvailability indicates uptime/health metrics (up, kube_pod_status_phase)
	SignalAvailability SignalRole = "Availability"

	// SignalLatency indicates response time/duration metrics (histogram_quantile, *_duration_*)
	SignalLatency SignalRole = "Latency"

	// SignalErrors indicates failure/error rate metrics (*_error_*, *_failed_*)
	SignalErrors SignalRole = "Errors"

	// SignalTraffic indicates throughput/request rate metrics (rate(*_total), *_count)
	SignalTraffic SignalRole = "Traffic"

	// SignalSaturation indicates resource utilization metrics (cpu, memory, disk)
	SignalSaturation SignalRole = "Saturation"

	// SignalChurn indicates workload churn/restarts (pod restarts, deployments)
	// Deprecated: use SignalNovelty instead (v1.5+)
	SignalChurn SignalRole = "Novelty"

	// SignalNovelty indicates change events/deployments (replaces Churn in v1.5)
	SignalNovelty SignalRole = "Novelty"

	// SignalUnknown indicates metrics that could not be classified
	SignalUnknown SignalRole = "Unknown"
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

// ClassificationResult represents the output of layered classification.
// Used internally by classifier to track confidence and reasoning.
type ClassificationResult struct {
	// Role is the classified signal role
	Role SignalRole

	// Confidence is the classification confidence (0.0-1.0)
	Confidence float64

	// Layer indicates which classification layer matched (1-5)
	// 1: Hardcoded known metrics (confidence ~0.95)
	// 2: PromQL structure patterns (confidence ~0.85-0.9)
	// 3: Metric name patterns (confidence ~0.7-0.8)
	// 4: Panel title/description (confidence ~0.5)
	// 5: Unknown/unclassified (confidence 0)
	Layer int

	// Reason is a human-readable explanation of why this classification was chosen
	// Examples: "matched hardcoded metric: up", "histogram_quantile indicates latency"
	Reason string
}

// WorkloadInference represents an inferred K8s workload from PromQL labels.
// Used to link SignalAnchors to ResourceIdentity nodes in the K8s graph.
type WorkloadInference struct {
	// Namespace is the K8s namespace (from namespace label)
	Namespace string

	// WorkloadName is the inferred workload name
	// Extracted from deployment/app/service/job labels in priority order
	WorkloadName string

	// InferredFrom is the label key used for inference
	// Examples: "deployment", "app.kubernetes.io/name", "app", "service", "job"
	InferredFrom string

	// Confidence is the inference confidence (0.7-0.9)
	// Higher confidence for explicit labels (deployment=0.9)
	// Lower confidence for generic labels (app=0.7)
	Confidence float64
}
