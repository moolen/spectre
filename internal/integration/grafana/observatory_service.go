package grafana

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// anomalyThreshold is the internal threshold for filtering anomalous signals.
// Scores >= 0.5 are considered anomalous per CONTEXT.md.
const anomalyThreshold = 0.5

// maxClusterHotspots is the maximum number of hotspots returned in cluster-wide queries.
const maxClusterHotspots = 5

// maxNamespaceWorkloads is the maximum number of workloads returned in namespace queries.
const maxNamespaceWorkloads = 20

// maxDashboards is the maximum number of dashboards returned in quality queries.
const maxDashboards = 20

// ObservatoryService encapsulates business logic for observatory MCP tools.
// It composes the AnomalyAggregator for hierarchical anomaly scoring and
// the graph client for topology queries.
type ObservatoryService struct {
	graphClient     graph.Client
	anomalyAgg      *AnomalyAggregator
	integrationName string
	logger          *logging.Logger
}

// NewObservatoryService creates a new ObservatoryService instance.
func NewObservatoryService(
	graphClient graph.Client,
	anomalyAgg *AnomalyAggregator,
	integrationName string,
	logger *logging.Logger,
) *ObservatoryService {
	return &ObservatoryService{
		graphClient:     graphClient,
		anomalyAgg:      anomalyAgg,
		integrationName: integrationName,
		logger:          logger,
	}
}

// ScopeOptions provides optional filters for observatory queries.
type ScopeOptions struct {
	Cluster   string // Optional: cluster name filter
	Namespace string // Optional: namespace filter
	Workload  string // Optional: workload filter
}

// ClusterAnomaliesResult contains cluster-wide anomaly summary for Orient stage.
type ClusterAnomaliesResult struct {
	TopHotspots           []Hotspot `json:"top_hotspots"`
	TotalAnomalousSignals int       `json:"total_anomalous_signals"`
	Timestamp             string    `json:"timestamp"` // RFC3339
}

// Hotspot represents a namespace or workload with anomalous signals.
type Hotspot struct {
	Namespace   string  `json:"namespace"`
	Workload    string  `json:"workload,omitempty"` // May be empty for namespace-level
	Score       float64 `json:"score"`              // 0.0-1.0
	Confidence  float64 `json:"confidence"`         // 0.0-1.0
	SignalCount int     `json:"signal_count"`
}

// NamespaceAnomaliesResult contains namespace-scoped workload anomalies for Narrow stage.
type NamespaceAnomaliesResult struct {
	Workloads []WorkloadAnomaly `json:"workloads"`
	Namespace string            `json:"namespace"`
	Timestamp string            `json:"timestamp"` // RFC3339
}

// WorkloadAnomaly represents anomaly information for a single workload.
type WorkloadAnomaly struct {
	Name        string  `json:"name"`
	Score       float64 `json:"score"`
	Confidence  float64 `json:"confidence"`
	SignalCount int     `json:"signal_count"`
	TopSignal   string  `json:"top_signal"` // Metric name of highest-scoring signal
}

// WorkloadAnomalyDetailResult contains signal-level anomalies for a specific workload.
type WorkloadAnomalyDetailResult struct {
	Signals   []SignalAnomaly `json:"signals"`
	Namespace string          `json:"namespace"`
	Workload  string          `json:"workload"`
	Timestamp string          `json:"timestamp"` // RFC3339
}

// SignalAnomaly represents anomaly information for a single signal.
type SignalAnomaly struct {
	MetricName string  `json:"metric_name"`
	Role       string  `json:"role"`       // Availability, Latency, etc.
	Score      float64 `json:"score"`      // 0.0-1.0
	Confidence float64 `json:"confidence"` // 0.0-1.0
}

// DashboardQualityResult contains dashboard quality rankings.
type DashboardQualityResult struct {
	Dashboards []DashboardQualityEntry `json:"dashboards"`
	Timestamp  string                  `json:"timestamp"` // RFC3339
}

// DashboardQualityEntry represents quality information for a single dashboard.
type DashboardQualityEntry struct {
	UID          string  `json:"uid"`
	Title        string  `json:"title"`
	QualityScore float64 `json:"quality_score"` // 0.0-1.0
	SignalCount  int     `json:"signal_count"`  // Number of classified signals
}

// GetClusterAnomalies computes cluster-wide anomaly summary.
//
// Process:
// 1. Query all namespaces with active SignalAnchors
// 2. For each namespace, call anomalyAgg.AggregateNamespaceAnomaly()
// 3. Filter results where Score >= 0.5
// 4. Rank by score descending, limit to top 5
// 5. Return ClusterAnomaliesResult with TopHotspots and TotalAnomalousSignals
func (s *ObservatoryService) GetClusterAnomalies(ctx context.Context, opts *ScopeOptions) (*ClusterAnomaliesResult, error) {
	// Query all namespaces with active signals
	namespaces, err := s.getClusterNamespaces(ctx)
	if err != nil {
		return nil, err
	}

	hotspots := make([]Hotspot, 0)
	totalAnomalousSignals := 0

	for _, ns := range namespaces {
		// Apply namespace filter if provided
		if opts != nil && opts.Namespace != "" && ns != opts.Namespace {
			continue
		}

		nsResult, err := s.anomalyAgg.AggregateNamespaceAnomaly(ctx, ns)
		if err != nil {
			s.logger.Debug("Error aggregating namespace %s: %v", ns, err)
			continue
		}
		if nsResult == nil {
			continue
		}

		// Filter by anomaly threshold
		if nsResult.Score >= anomalyThreshold {
			hotspots = append(hotspots, Hotspot{
				Namespace:   ns,
				Score:       nsResult.Score,
				Confidence:  nsResult.Confidence,
				SignalCount: nsResult.SourceCount,
			})
			totalAnomalousSignals += nsResult.SourceCount
		}
	}

	// Rank by score descending (with confidence as tiebreaker)
	sort.Slice(hotspots, func(i, j int) bool {
		if hotspots[i].Score != hotspots[j].Score {
			return hotspots[i].Score > hotspots[j].Score
		}
		return hotspots[i].Confidence > hotspots[j].Confidence
	})

	// Limit to top 5
	if len(hotspots) > maxClusterHotspots {
		hotspots = hotspots[:maxClusterHotspots]
	}

	return &ClusterAnomaliesResult{
		TopHotspots:           hotspots,
		TotalAnomalousSignals: totalAnomalousSignals,
		Timestamp:             time.Now().Format(time.RFC3339),
	}, nil
}

// GetNamespaceAnomalies computes workload-level anomalies within a namespace.
//
// Process:
// 1. Query all workloads in namespace with active signals
// 2. For each workload, call anomalyAgg.AggregateWorkloadAnomaly()
// 3. Filter where Score >= 0.5
// 4. Rank by score descending, limit to top 20
// 5. Return NamespaceAnomaliesResult with Workloads
func (s *ObservatoryService) GetNamespaceAnomalies(ctx context.Context, namespace string) (*NamespaceAnomaliesResult, error) {
	// Query all workloads in namespace
	workloads, err := s.getNamespaceWorkloads(ctx, namespace)
	if err != nil {
		return nil, err
	}

	workloadAnomalies := make([]WorkloadAnomaly, 0)

	for _, workload := range workloads {
		wlResult, err := s.anomalyAgg.AggregateWorkloadAnomaly(ctx, namespace, workload)
		if err != nil {
			s.logger.Debug("Error aggregating workload %s/%s: %v", namespace, workload, err)
			continue
		}
		if wlResult == nil {
			continue
		}

		// Filter by anomaly threshold
		if wlResult.Score >= anomalyThreshold {
			workloadAnomalies = append(workloadAnomalies, WorkloadAnomaly{
				Name:        workload,
				Score:       wlResult.Score,
				Confidence:  wlResult.Confidence,
				SignalCount: wlResult.SourceCount,
				TopSignal:   wlResult.TopSource,
			})
		}
	}

	// Rank by score descending (with confidence as tiebreaker)
	sort.Slice(workloadAnomalies, func(i, j int) bool {
		if workloadAnomalies[i].Score != workloadAnomalies[j].Score {
			return workloadAnomalies[i].Score > workloadAnomalies[j].Score
		}
		return workloadAnomalies[i].Confidence > workloadAnomalies[j].Confidence
	})

	// Limit to top 20
	if len(workloadAnomalies) > maxNamespaceWorkloads {
		workloadAnomalies = workloadAnomalies[:maxNamespaceWorkloads]
	}

	return &NamespaceAnomaliesResult{
		Workloads: workloadAnomalies,
		Namespace: namespace,
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}

// GetWorkloadAnomalyDetail returns signal-level anomaly details for a specific workload.
//
// Process:
// 1. Query all SignalAnchors for the workload with their baselines
// 2. For each signal, compute anomaly score
// 3. Filter where Score >= 0.5
// 4. Rank by score descending
// 5. Return WorkloadAnomalyDetailResult with Signals
func (s *ObservatoryService) GetWorkloadAnomalyDetail(ctx context.Context, namespace, workload string) (*WorkloadAnomalyDetailResult, error) {
	// Query signals with baselines for this workload
	signals, err := s.getWorkloadSignalsWithRole(ctx, namespace, workload)
	if err != nil {
		return nil, err
	}

	signalAnomalies := make([]SignalAnomaly, 0)

	for _, signal := range signals {
		// Skip signals without baselines (cold start)
		if signal.Baseline == nil {
			continue
		}

		// Compute anomaly score
		score, err := ComputeAnomalyScore(signal.CurrentValue, *signal.Baseline, signal.QualityScore)
		if err != nil {
			// InsufficientSamplesError - skip this signal
			var insufficientErr *InsufficientSamplesError
			if errors.As(err, &insufficientErr) {
				continue
			}
			s.logger.Debug("Error computing anomaly for %s: %v", signal.MetricName, err)
			continue
		}

		// Apply alert override if firing
		if signal.AlertState == "firing" {
			score = ApplyAlertOverride(score, signal.AlertState)
		}

		// Filter by anomaly threshold
		if score.Score >= anomalyThreshold {
			signalAnomalies = append(signalAnomalies, SignalAnomaly{
				MetricName: signal.MetricName,
				Role:       signal.Role,
				Score:      score.Score,
				Confidence: score.Confidence,
			})
		}
	}

	// Rank by score descending (with confidence as tiebreaker)
	sort.Slice(signalAnomalies, func(i, j int) bool {
		if signalAnomalies[i].Score != signalAnomalies[j].Score {
			return signalAnomalies[i].Score > signalAnomalies[j].Score
		}
		return signalAnomalies[i].Confidence > signalAnomalies[j].Confidence
	})

	return &WorkloadAnomalyDetailResult{
		Signals:   signalAnomalies,
		Namespace: namespace,
		Workload:  workload,
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}

// GetDashboardQuality returns dashboards ranked by quality score.
//
// Process:
// 1. Query graph for all Dashboard nodes with quality_score property
// 2. Count signals per dashboard
// 3. Rank by quality_score descending, limit to top 20
// 4. Return DashboardQualityResult with Dashboards
func (s *ObservatoryService) GetDashboardQuality(ctx context.Context, opts *ScopeOptions) (*DashboardQualityResult, error) {
	query := `
		MATCH (d:Dashboard {integration: $integration})
		WHERE d.quality_score IS NOT NULL
		OPTIONAL MATCH (d)<-[:EXTRACTED_FROM]-(s:SignalAnchor)
		WHERE s.expires_at > $now
		WITH d, count(s) AS signal_count
		RETURN d.uid AS uid, d.title AS title, d.quality_score AS quality_score, signal_count
		ORDER BY d.quality_score DESC
		LIMIT $limit
	`

	now := time.Now().Unix()
	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"integration": s.integrationName,
			"now":         now,
			"limit":       maxDashboards,
		},
	})
	if err != nil {
		return nil, err
	}

	// Map column names to indices
	colIdx := make(map[string]int)
	for i, col := range result.Columns {
		colIdx[col] = i
	}

	dashboards := make([]DashboardQualityEntry, 0)
	for _, row := range result.Rows {
		entry := DashboardQualityEntry{}

		if idx, ok := colIdx["uid"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				entry.UID = v
			}
		}
		if idx, ok := colIdx["title"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				entry.Title = v
			}
		}
		if idx, ok := colIdx["quality_score"]; ok && idx < len(row) {
			entry.QualityScore = parseFloat64(row[idx])
		}
		if idx, ok := colIdx["signal_count"]; ok && idx < len(row) {
			entry.SignalCount = parseInt(row[idx])
		}

		dashboards = append(dashboards, entry)
	}

	return &DashboardQualityResult{
		Dashboards: dashboards,
		Timestamp:  time.Now().Format(time.RFC3339),
	}, nil
}

// signalWithRole holds signal data with role information for workload detail queries.
type signalWithRole struct {
	MetricName   string
	Role         string
	QualityScore float64
	CurrentValue float64
	AlertState   string
	Baseline     *SignalBaseline
}

// getClusterNamespaces retrieves distinct namespaces with active signals.
func (s *ObservatoryService) getClusterNamespaces(ctx context.Context) ([]string, error) {
	query := `
		MATCH (sig:SignalAnchor {integration: $integration})
		WHERE sig.expires_at > $now AND sig.workload_namespace <> ''
		RETURN DISTINCT sig.workload_namespace AS namespace
	`

	now := time.Now().Unix()
	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"integration": s.integrationName,
			"now":         now,
		},
	})
	if err != nil {
		return nil, err
	}

	var namespaces []string
	for _, row := range result.Rows {
		if len(row) > 0 {
			if ns, ok := row[0].(string); ok && ns != "" {
				namespaces = append(namespaces, ns)
			}
		}
	}

	return namespaces, nil
}

// getNamespaceWorkloads retrieves distinct workload names in a namespace.
func (s *ObservatoryService) getNamespaceWorkloads(ctx context.Context, namespace string) ([]string, error) {
	query := `
		MATCH (sig:SignalAnchor {
			workload_namespace: $namespace,
			integration: $integration
		})
		WHERE sig.expires_at > $now AND sig.workload_name <> ''
		RETURN DISTINCT sig.workload_name AS workload_name
	`

	now := time.Now().Unix()
	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"namespace":   namespace,
			"integration": s.integrationName,
			"now":         now,
		},
	})
	if err != nil {
		return nil, err
	}

	var workloads []string
	for _, row := range result.Rows {
		if len(row) > 0 {
			if workload, ok := row[0].(string); ok && workload != "" {
				workloads = append(workloads, workload)
			}
		}
	}

	return workloads, nil
}

// getWorkloadSignalsWithRole retrieves signals for a workload with their baselines and roles.
func (s *ObservatoryService) getWorkloadSignalsWithRole(ctx context.Context, namespace, workloadName string) ([]signalWithRole, error) {
	query := `
		MATCH (sig:SignalAnchor {
			workload_namespace: $namespace,
			workload_name: $workload_name,
			integration: $integration
		})
		WHERE sig.expires_at > $now
		OPTIONAL MATCH (sig)-[:HAS_BASELINE]->(b:SignalBaseline)
		RETURN sig.metric_name AS metric_name,
		       sig.role AS role,
		       sig.quality_score AS quality_score,
		       b.mean AS mean,
		       b.std_dev AS std_dev,
		       b.min AS min,
		       b.max AS max,
		       b.p50 AS p50,
		       b.p90 AS p90,
		       b.p99 AS p99,
		       b.sample_count AS sample_count
	`

	now := time.Now().Unix()
	result, err := s.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"namespace":     namespace,
			"workload_name": workloadName,
			"integration":   s.integrationName,
			"now":           now,
		},
	})
	if err != nil {
		return nil, err
	}

	// Map column names to indices
	colIdx := make(map[string]int)
	for i, col := range result.Columns {
		colIdx[col] = i
	}

	var signals []signalWithRole
	for _, row := range result.Rows {
		signal := signalWithRole{}

		// Extract metric_name
		if idx, ok := colIdx["metric_name"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				signal.MetricName = v
			}
		}

		// Extract role
		if idx, ok := colIdx["role"]; ok && idx < len(row) {
			if v, ok := row[idx].(string); ok {
				signal.Role = v
			}
		}

		// Extract quality_score
		if idx, ok := colIdx["quality_score"]; ok && idx < len(row) {
			signal.QualityScore = parseFloat64(row[idx])
		}

		// Extract baseline if present
		if idx, ok := colIdx["sample_count"]; ok && idx < len(row) && row[idx] != nil {
			signal.Baseline = &SignalBaseline{
				SampleCount: parseInt(row[colIdx["sample_count"]]),
			}
			if idx, ok := colIdx["mean"]; ok && idx < len(row) {
				signal.Baseline.Mean = parseFloat64(row[idx])
			}
			if idx, ok := colIdx["std_dev"]; ok && idx < len(row) {
				signal.Baseline.StdDev = parseFloat64(row[idx])
			}
			if idx, ok := colIdx["min"]; ok && idx < len(row) {
				signal.Baseline.Min = parseFloat64(row[idx])
			}
			if idx, ok := colIdx["max"]; ok && idx < len(row) {
				signal.Baseline.Max = parseFloat64(row[idx])
			}
			if idx, ok := colIdx["p50"]; ok && idx < len(row) {
				signal.Baseline.P50 = parseFloat64(row[idx])
			}
			if idx, ok := colIdx["p90"]; ok && idx < len(row) {
				signal.Baseline.P90 = parseFloat64(row[idx])
			}
			if idx, ok := colIdx["p99"]; ok && idx < len(row) {
				signal.Baseline.P99 = parseFloat64(row[idx])
			}
		}

		// For now, use baseline mean as current value proxy
		// In production, this would come from recent Grafana query
		if signal.Baseline != nil {
			signal.CurrentValue = signal.Baseline.Mean
		}

		signals = append(signals, signal)
	}

	return signals, nil
}
