package observatory

import (
	"context"
	"time"
)

// Provider is the interface that integrations must implement to feed data to Observatory.
// Each integration (Grafana, Datadog, CloudWatch, etc.) implements this interface.
//
// Provider implementations are responsible for:
//   - Discovering signals from their data source (dashboards, monitors, etc.)
//   - Fetching current metric values for anomaly scoring
//   - Managing baseline statistics (storage and retrieval)
//   - Reporting alert states for score overrides
type Provider interface {
	// Name returns the unique identifier for this provider (e.g., "grafana-prod")
	Name() string

	// --- Signal Discovery ---

	// ListSignalAnchors returns all active SignalAnchors from this provider.
	// Called during aggregation to enumerate available signals.
	//
	// The returned signals should have:
	//   - MetricName, Role, Confidence, QualityScore populated
	//   - WorkloadNamespace/WorkloadName if the signal is linked to a K8s workload
	//   - SourceProvider set to this provider's Name()
	//   - ExpiresAt set appropriately (signals past expiry should not be returned)
	ListSignalAnchors(ctx context.Context, opts SignalListOptions) ([]SignalAnchor, error)

	// --- Current Values ---

	// GetCurrentValue fetches the current value of a metric for anomaly scoring.
	// Returns (value, found, error).
	//
	// If found=false, the caller should use baseline.Mean as a fallback.
	// This allows graceful handling of metrics that are temporarily unavailable.
	GetCurrentValue(ctx context.Context, metricName, namespace, workload string) (float64, bool, error)

	// --- Baselines ---

	// GetBaseline retrieves the baseline statistics for a signal.
	// Returns nil if no baseline exists (cold start condition).
	//
	// Baselines should have at least MinSamplesRequired samples to be useful
	// for anomaly detection. The caller handles InsufficientSamplesError.
	GetBaseline(ctx context.Context, metricName, namespace, workload string) (*SignalBaseline, error)

	// --- Alert State ---

	// GetAlertState returns the current alert state for a signal.
	// Returns empty string if no alert is associated with this signal.
	//
	// Valid states: "firing", "pending", "normal", ""
	// A "firing" state triggers score override to 1.0 in anomaly aggregation.
	GetAlertState(ctx context.Context, metricName, namespace, workload string) (string, error)
}

// SignalListOptions provides filtering for ListSignalAnchors.
type SignalListOptions struct {
	// Namespace filters signals to a specific K8s namespace.
	// Empty string means all namespaces.
	Namespace string

	// WorkloadName filters signals to a specific workload within the namespace.
	// Empty string means all workloads (requires Namespace to be set for meaningful results).
	WorkloadName string

	// Role filters signals to a specific role.
	// Empty string means all roles.
	Role SignalRole
}

// EvidenceProvider is an optional interface for integrations that can provide
// detailed evidence for root cause analysis.
//
// Not all providers need to implement this. The Registry checks for this
// interface at runtime when evidence is requested.
type EvidenceProvider interface {
	Provider

	// GetMetricValues returns raw metric values for evidence gathering.
	// Used to show recent metric history in investigation tools.
	//
	// The lookback duration specifies how far back to query (e.g., 1h, 6h).
	// Returns empty slice if no data is available (graceful degradation).
	GetMetricValues(ctx context.Context, metricName, namespace, workload string, lookback time.Duration) ([]MetricValue, error)

	// GetRelatedAlerts returns alerts related to a workload.
	// Used to show alert context in investigation tools.
	//
	// The lookback duration specifies the time window for alert transitions.
	// Returns empty slice if no alerts are found.
	GetRelatedAlerts(ctx context.Context, namespace, workload string, lookback time.Duration) ([]AlertState, error)
}

// BackfillProvider is an optional interface for integrations that support
// historical baseline backfill.
//
// This is useful for bootstrapping baselines when Observatory is first deployed,
// rather than waiting for the normal collection interval to build up samples.
type BackfillProvider interface {
	Provider

	// BackfillBaseline queries historical data to populate baseline statistics.
	// Returns a fully populated SignalBaseline with statistics from historical data.
	//
	// windowDays specifies how many days of history to query (typically 7).
	// Returns nil if insufficient historical data is available.
	BackfillBaseline(ctx context.Context, metricName, namespace, workload string, windowDays int) (*SignalBaseline, error)
}

// MetricValue represents a single metric data point for evidence gathering.
type MetricValue struct {
	// Timestamp is the data point time (RFC3339 format)
	Timestamp string

	// Value is the metric value at this timestamp
	Value float64
}

// AlertState represents an alert and its current state for evidence gathering.
type AlertState struct {
	// AlertName is the human-readable alert title
	AlertName string

	// State is the current alert state (firing, normal, pending)
	State string

	// Since is when the alert entered this state (RFC3339 format)
	Since string
}
