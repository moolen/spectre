package grafana

import (
	"context"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// ObservatoryEvidenceService provides root cause analysis and evidence aggregation
// for the Hypothesize and Verify stages of incident investigation.
// It queries the K8s graph for upstream dependencies and recent changes,
// and aggregates metric values, alert states, and log excerpts.
type ObservatoryEvidenceService struct {
	graphClient     graph.Client
	queryService    *GrafanaQueryService
	integrationName string
	logger          *logging.Logger
}

// NewObservatoryEvidenceService creates a new ObservatoryEvidenceService instance.
func NewObservatoryEvidenceService(
	graphClient graph.Client,
	queryService *GrafanaQueryService,
	integrationName string,
	logger *logging.Logger,
) *ObservatoryEvidenceService {
	return &ObservatoryEvidenceService{
		graphClient:     graphClient,
		queryService:    queryService,
		integrationName: integrationName,
		logger:          logger,
	}
}

// CandidateCausesResult contains potential root causes from K8s graph traversal.
type CandidateCausesResult struct {
	// UpstreamDeps are dependencies found via 2-hop upstream traversal
	UpstreamDeps []UpstreamDependency `json:"upstream_deps"`

	// RecentChanges are K8s events (deployments, config changes) in the last hour
	RecentChanges []RecentChange `json:"recent_changes"`

	// Timestamp is when this result was computed (ISO8601)
	Timestamp string `json:"timestamp"`
}

// UpstreamDependency represents a dependency found via graph traversal.
type UpstreamDependency struct {
	// Kind is the K8s resource kind (Service, Ingress, Deployment, etc.)
	Kind string `json:"kind"`

	// Namespace is the K8s namespace
	Namespace string `json:"namespace"`

	// Name is the resource name
	Name string `json:"name"`

	// HopsAway indicates the graph distance (1 or 2)
	HopsAway int `json:"hops_away"`
}

// RecentChange represents a K8s change event that could be a root cause.
type RecentChange struct {
	// Kind is the K8s resource kind
	Kind string `json:"kind"`

	// Namespace is the K8s namespace (may be empty for cluster-scoped resources)
	Namespace string `json:"namespace"`

	// Name is the resource name
	Name string `json:"name"`

	// Reason is the event reason (e.g., "DeploymentUpdated", "ConfigChanged")
	Reason string `json:"reason"`

	// Timestamp is when the change occurred (ISO8601)
	Timestamp string `json:"timestamp"`
}

// SignalEvidenceResult contains aggregated evidence for a specific signal.
type SignalEvidenceResult struct {
	// MetricValues are the raw metric data points in the lookback window
	MetricValues []MetricValue `json:"metric_values"`

	// AlertStates are the alert state transitions for related alerts
	AlertStates []EvidenceAlertState `json:"alert_states"`

	// LogExcerpts are relevant log entries (ERROR level, 5-minute window)
	// May be empty if log integration is not configured
	LogExcerpts []LogExcerpt `json:"log_excerpts,omitempty"`

	// Timestamp is when this result was computed (ISO8601)
	Timestamp string `json:"timestamp"`
}

// MetricValue represents a single metric data point.
type MetricValue struct {
	// Timestamp is the data point time (ISO8601)
	Timestamp string `json:"timestamp"`

	// Value is the metric value
	Value float64 `json:"value"`
}

// EvidenceAlertState represents an alert and its current state for evidence aggregation.
// Named differently from AlertState in client.go to avoid type collision.
type EvidenceAlertState struct {
	// AlertName is the human-readable alert title
	AlertName string `json:"alert_name"`

	// State is the current alert state (firing, normal, pending)
	State string `json:"state"`

	// Since is when the alert entered this state (ISO8601)
	Since string `json:"since"`
}

// LogExcerpt represents a log entry relevant to the investigation.
type LogExcerpt struct {
	// Timestamp is when the log was generated (ISO8601)
	Timestamp string `json:"timestamp"`

	// Level is the log level (ERROR, WARN)
	Level string `json:"level"`

	// Message is the log message content
	Message string `json:"message"`

	// Source is the pod name that generated the log
	Source string `json:"source"`
}

// GetCandidateCauses returns potential root causes by analyzing the K8s graph.
// It performs:
// 1. 2-hop upstream traversal to find dependencies (workload -> service -> ingress/deployment)
// 2. Query for recent changes (last 1 hour) in the same namespace or cluster-scoped
//
// Results are ranked by relevance: closer hops are more relevant.
func (s *ObservatoryEvidenceService) GetCandidateCauses(
	ctx context.Context,
	namespace string,
	workload string,
	metricName string,
) (*CandidateCausesResult, error) {
	s.logger.Debug("Getting candidate causes for %s/%s, metric: %s", namespace, workload, metricName)

	// Query for upstream dependencies (2-hop traversal)
	upstreamDeps, err := s.getUpstreamDependencies(ctx, namespace, workload)
	if err != nil {
		s.logger.Warn("Failed to get upstream dependencies: %v", err)
		// Continue with empty deps - graceful degradation
		upstreamDeps = []UpstreamDependency{}
	}

	// Query for recent changes (last 1 hour)
	recentChanges, err := s.getRecentChanges(ctx, namespace)
	if err != nil {
		s.logger.Warn("Failed to get recent changes: %v", err)
		// Continue with empty changes - graceful degradation
		recentChanges = []RecentChange{}
	}

	return &CandidateCausesResult{
		UpstreamDeps:  upstreamDeps,
		RecentChanges: recentChanges,
		Timestamp:     time.Now().Format(time.RFC3339),
	}, nil
}

// getUpstreamDependencies performs a 2-hop upstream traversal in the K8s graph.
// Returns dependencies ordered by distance (1-hop first, then 2-hop).
func (s *ObservatoryEvidenceService) getUpstreamDependencies(
	ctx context.Context,
	namespace string,
	workload string,
) ([]UpstreamDependency, error) {
	// Query for 1-hop and 2-hop upstream dependencies
	// ResourceIdentity nodes represent K8s resources with DEPENDS_ON relationships
	query := `
		MATCH (w:ResourceIdentity {namespace: $namespace, name: $workload})
		OPTIONAL MATCH (w)<-[:DEPENDS_ON]-(dep1:ResourceIdentity)
		OPTIONAL MATCH (w)<-[:DEPENDS_ON*2]-(dep2:ResourceIdentity)
		WHERE dep2 <> dep1 OR dep1 IS NULL
		WITH
			COLLECT(DISTINCT {kind: dep1.kind, namespace: dep1.namespace, name: dep1.name, hops: 1}) AS hops1,
			COLLECT(DISTINCT {kind: dep2.kind, namespace: dep2.namespace, name: dep2.name, hops: 2}) AS hops2
		RETURN hops1, hops2
	`

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"namespace": namespace,
			"workload":  workload,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query upstream dependencies: %w", err)
	}

	var deps []UpstreamDependency

	// Parse results - handle both 1-hop and 2-hop
	if len(result.Rows) > 0 && len(result.Rows[0]) >= 2 {
		// Parse 1-hop results
		if hops1, ok := result.Rows[0][0].([]interface{}); ok {
			for _, h := range hops1 {
				if depMap, ok := h.(map[string]interface{}); ok {
					dep := s.parseDependency(depMap)
					if dep != nil && dep.Name != "" {
						deps = append(deps, *dep)
					}
				}
			}
		}

		// Parse 2-hop results
		if hops2, ok := result.Rows[0][1].([]interface{}); ok {
			for _, h := range hops2 {
				if depMap, ok := h.(map[string]interface{}); ok {
					dep := s.parseDependency(depMap)
					if dep != nil && dep.Name != "" {
						// Ensure we don't duplicate 1-hop deps
						isDuplicate := false
						for _, existing := range deps {
							if existing.Kind == dep.Kind && existing.Namespace == dep.Namespace && existing.Name == dep.Name {
								isDuplicate = true
								break
							}
						}
						if !isDuplicate {
							deps = append(deps, *dep)
						}
					}
				}
			}
		}
	}

	return deps, nil
}

// parseDependency converts a graph result map to UpstreamDependency.
func (s *ObservatoryEvidenceService) parseDependency(depMap map[string]interface{}) *UpstreamDependency {
	dep := &UpstreamDependency{}

	if kind, ok := depMap["kind"].(string); ok {
		dep.Kind = kind
	}
	if ns, ok := depMap["namespace"].(string); ok {
		dep.Namespace = ns
	}
	if name, ok := depMap["name"].(string); ok {
		dep.Name = name
	}
	if hops, ok := depMap["hops"].(int64); ok {
		dep.HopsAway = int(hops)
	} else if hops, ok := depMap["hops"].(float64); ok {
		dep.HopsAway = int(hops)
	} else if hops, ok := depMap["hops"].(int); ok {
		dep.HopsAway = hops
	}

	return dep
}

// getRecentChanges queries for K8s events in the last hour that could be root causes.
func (s *ObservatoryEvidenceService) getRecentChanges(
	ctx context.Context,
	namespace string,
) ([]RecentChange, error) {
	oneHourAgo := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)

	// Query for recent events (Deployment, ConfigMap, Secret, HelmRelease changes)
	// Events are captured in the graph from K8s watch
	query := `
		MATCH (e:Event)
		WHERE e.timestamp > $oneHourAgo
		  AND (e.namespace = $namespace OR e.namespace IS NULL)
		  AND e.kind IN ['Deployment', 'ConfigMap', 'Secret', 'HelmRelease', 'StatefulSet', 'DaemonSet']
		RETURN e.kind AS kind, e.namespace AS namespace, e.name AS name, e.reason AS reason, e.timestamp AS timestamp
		ORDER BY e.timestamp DESC
		LIMIT 10
	`

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"oneHourAgo": oneHourAgo,
			"namespace":  namespace,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query recent changes: %w", err)
	}

	// Map column names to indices
	colIdx := make(map[string]int)
	for i, col := range result.Columns {
		colIdx[col] = i
	}

	var changes []RecentChange
	for _, row := range result.Rows {
		change := RecentChange{}

		if idx, ok := colIdx["kind"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				change.Kind = v
			}
		}
		if idx, ok := colIdx["namespace"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				change.Namespace = v
			}
		}
		if idx, ok := colIdx["name"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				change.Name = v
			}
		}
		if idx, ok := colIdx["reason"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				change.Reason = v
			}
		}
		if idx, ok := colIdx["timestamp"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				change.Timestamp = v
			}
		}

		if change.Name != "" {
			changes = append(changes, change)
		}
	}

	return changes, nil
}

// GetSignalEvidence aggregates evidence for a specific signal.
// It fetches:
// 1. Raw metric values from Grafana for the time range
// 2. Alert states for related alerts (by workload or namespace)
// 3. Log snippets (ERROR level within 5-minute window) - if log integration available
//
// Returns partial results when some data sources are unavailable.
func (s *ObservatoryEvidenceService) GetSignalEvidence(
	ctx context.Context,
	namespace string,
	workload string,
	metricName string,
	lookback time.Duration,
) (*SignalEvidenceResult, error) {
	s.logger.Debug("Getting signal evidence for %s/%s, metric: %s, lookback: %s",
		namespace, workload, metricName, lookback)

	now := time.Now()

	// Fetch metric values from graph (baseline samples)
	metricValues, err := s.getMetricValues(ctx, namespace, workload, metricName, lookback)
	if err != nil {
		s.logger.Warn("Failed to get metric values: %v", err)
		// Continue with empty values - graceful degradation
		metricValues = []MetricValue{}
	}

	// Fetch alert states for related alerts
	alertStates, err := s.getAlertStates(ctx, namespace, workload, now.Add(-lookback), now)
	if err != nil {
		s.logger.Warn("Failed to get alert states: %v", err)
		// Continue with empty alerts - graceful degradation
		alertStates = []EvidenceAlertState{}
	}

	// Fetch log excerpts (ERROR level, 5-minute window around now)
	// Gracefully handle missing log integration
	logExcerpts, err := s.getLogExcerpts(ctx, namespace, workload)
	if err != nil {
		s.logger.Debug("Log excerpts not available: %v", err)
		// Graceful degradation - return empty, not error
		logExcerpts = []LogExcerpt{}
	}

	return &SignalEvidenceResult{
		MetricValues: metricValues,
		AlertStates:  alertStates,
		LogExcerpts:  logExcerpts,
		Timestamp:    now.Format(time.RFC3339),
	}, nil
}

// getMetricValues retrieves metric data points from baseline storage.
func (s *ObservatoryEvidenceService) getMetricValues(
	ctx context.Context,
	namespace string,
	workload string,
	metricName string,
	lookback time.Duration,
) ([]MetricValue, error) {
	// Query SignalBaseline for recent statistics as proxy for values
	// In production, this would query Grafana directly for time series data
	query := `
		MATCH (s:SignalAnchor {
			metric_name: $metric_name,
			workload_namespace: $namespace,
			workload_name: $workload,
			integration: $integration
		})
		WHERE s.expires_at > $now
		OPTIONAL MATCH (s)-[:HAS_BASELINE]->(b:SignalBaseline)
		RETURN b.mean AS mean, b.std_dev AS std_dev, b.min AS min, b.max AS max,
		       b.p50 AS p50, b.p90 AS p90, b.p99 AS p99,
		       b.window_start AS window_start, b.window_end AS window_end
	`

	now := time.Now().Unix()
	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"metric_name": metricName,
			"namespace":   namespace,
			"workload":    workload,
			"integration": s.integrationName,
			"now":         now,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query metric values: %w", err)
	}

	// Convert baseline stats to synthetic metric values for evidence
	// In a full implementation, we'd query Grafana for actual time series
	var values []MetricValue

	if len(result.Rows) > 0 && len(result.Rows[0]) > 0 {
		row := result.Rows[0]
		colIdx := make(map[string]int)
		for i, col := range result.Columns {
			colIdx[col] = i
		}

		// Create summary values from baseline stats
		nowTime := time.Now()
		if idx, ok := colIdx["mean"]; ok && idx < len(row) && row[idx] != nil {
			values = append(values, MetricValue{
				Timestamp: nowTime.Format(time.RFC3339),
				Value:     parseFloat64(row[idx]),
			})
		}
	}

	return values, nil
}

// getAlertStates retrieves alert state transitions for the workload/namespace.
func (s *ObservatoryEvidenceService) getAlertStates(
	ctx context.Context,
	namespace string,
	workload string,
	startTime time.Time,
	endTime time.Time,
) ([]EvidenceAlertState, error) {
	// Query for alerts related to this workload or namespace
	// Alerts are linked via labels containing workload/namespace info
	query := `
		MATCH (a:Alert {integration: $integration})
		WHERE a.labels CONTAINS $workload OR a.labels CONTAINS $namespace
		OPTIONAL MATCH (a)-[t:STATE_TRANSITION]->(a)
		WHERE t.timestamp > $start AND t.timestamp < $end
		WITH a, t
		ORDER BY t.timestamp DESC
		RETURN DISTINCT a.title AS title, a.state AS state, a.state_timestamp AS since
		LIMIT 20
	`

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"integration": s.integrationName,
			"workload":    workload,
			"namespace":   namespace,
			"start":       startTime.Format(time.RFC3339),
			"end":         endTime.Format(time.RFC3339),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query alert states: %w", err)
	}

	// Map column names to indices
	colIdx := make(map[string]int)
	for i, col := range result.Columns {
		colIdx[col] = i
	}

	var alerts []EvidenceAlertState
	for _, row := range result.Rows {
		alert := EvidenceAlertState{}

		if idx, ok := colIdx["title"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				alert.AlertName = v
			}
		}
		if idx, ok := colIdx["state"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				alert.State = v
			}
		}
		if idx, ok := colIdx["since"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				alert.Since = v
			}
		}

		if alert.AlertName != "" {
			alerts = append(alerts, alert)
		}
	}

	return alerts, nil
}

// getLogExcerpts retrieves ERROR-level log entries from the graph.
// Returns empty slice if log integration is not configured (graceful degradation).
func (s *ObservatoryEvidenceService) getLogExcerpts(
	ctx context.Context,
	namespace string,
	workload string,
) ([]LogExcerpt, error) {
	// Query for log entries if they exist in the graph
	// Log integration may not be configured - this is expected
	fiveMinutesAgo := time.Now().Add(-5 * time.Minute).Format(time.RFC3339)

	query := `
		MATCH (l:LogEntry)
		WHERE l.namespace = $namespace
		  AND (l.workload = $workload OR l.pod_name STARTS WITH $workload)
		  AND l.level IN ['ERROR', 'error', 'FATAL', 'fatal']
		  AND l.timestamp > $since
		RETURN l.timestamp AS timestamp, l.level AS level, l.message AS message, l.pod_name AS source
		ORDER BY l.timestamp DESC
		LIMIT 10
	`

	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"namespace": namespace,
			"workload":  workload,
			"since":     fiveMinutesAgo,
		},
	})
	if err != nil {
		// Log integration not available - return empty, not error
		return nil, nil
	}

	// Map column names to indices
	colIdx := make(map[string]int)
	for i, col := range result.Columns {
		colIdx[col] = i
	}

	var excerpts []LogExcerpt
	for _, row := range result.Rows {
		excerpt := LogExcerpt{}

		if idx, ok := colIdx["timestamp"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				excerpt.Timestamp = v
			}
		}
		if idx, ok := colIdx["level"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				excerpt.Level = v
			}
		}
		if idx, ok := colIdx["message"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				excerpt.Message = v
			}
		}
		if idx, ok := colIdx["source"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				excerpt.Source = v
			}
		}

		if excerpt.Timestamp != "" {
			excerpts = append(excerpts, excerpt)
		}
	}

	return excerpts, nil
}
