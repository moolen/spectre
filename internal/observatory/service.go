package observatory

import (
	"context"
	"sort"
	"time"
)

// Service configuration constants
const (
	// anomalyThreshold is the minimum anomaly score to consider a signal anomalous.
	anomalyThreshold = 0.5

	// maxClusterHotspots is the maximum number of hotspots returned in cluster-wide queries.
	maxClusterHotspots = 5

	// maxNamespaceWorkloads is the maximum number of workloads returned in namespace queries.
	maxNamespaceWorkloads = 20
)

// Service encapsulates business logic for observatory MCP tools.
// It composes the Registry for signal data and AnomalyAggregator for hierarchical scoring.
type Service struct {
	registry   *Registry
	aggregator *AnomalyAggregator
}

// NewService creates a new Observatory service.
func NewService(registry *Registry) *Service {
	return &Service{
		registry:   registry,
		aggregator: NewAnomalyAggregator(registry),
	}
}

// ScopeOptions provides optional filters for observatory queries.
type ScopeOptions struct {
	Namespace string // Optional: namespace filter
	Workload  string // Optional: workload filter
}

// ClusterAnomaliesResult contains cluster-wide anomaly summary for Orient stage.
type ClusterAnomaliesResult struct {
	TopHotspots           []Hotspot `json:"top_hotspots"`
	TotalAnomalousSignals int       `json:"total_anomalous_signals"`
	Timestamp             string    `json:"timestamp"`
}

// Hotspot represents a namespace or workload with anomalous signals.
type Hotspot struct {
	Namespace   string  `json:"namespace"`
	Workload    string  `json:"workload,omitempty"`
	Score       float64 `json:"score"`
	Confidence  float64 `json:"confidence"`
	SignalCount int     `json:"signal_count"`
}

// NamespaceAnomaliesResult contains namespace-scoped workload anomalies for Narrow stage.
type NamespaceAnomaliesResult struct {
	Workloads []WorkloadAnomaly `json:"workloads"`
	Namespace string            `json:"namespace"`
	Timestamp string            `json:"timestamp"`
}

// WorkloadAnomaly represents anomaly information for a single workload.
type WorkloadAnomaly struct {
	Name        string  `json:"name"`
	Score       float64 `json:"score"`
	Confidence  float64 `json:"confidence"`
	SignalCount int     `json:"signal_count"`
	TopSignal   string  `json:"top_signal"`
}

// WorkloadAnomalyDetailResult contains signal-level anomalies for a specific workload.
type WorkloadAnomalyDetailResult struct {
	Signals   []SignalAnomaly `json:"signals"`
	Namespace string          `json:"namespace"`
	Workload  string          `json:"workload"`
	Timestamp string          `json:"timestamp"`
}

// SignalAnomaly represents anomaly information for a single signal.
type SignalAnomaly struct {
	MetricName string  `json:"metric_name"`
	Role       string  `json:"role"`
	Score      float64 `json:"score"`
	Confidence float64 `json:"confidence"`
}

// GetClusterAnomalies computes cluster-wide anomaly summary.
//
// Process:
// 1. Query all namespaces with active SignalAnchors
// 2. For each namespace, compute aggregated anomaly
// 3. Filter results where Score >= 0.5
// 4. Rank by score descending, limit to top 5
func (s *Service) GetClusterAnomalies(ctx context.Context, opts *ScopeOptions) (*ClusterAnomaliesResult, error) {
	// Get all signals to find namespaces
	signals, err := s.registry.ListAllSignalAnchors(ctx, SignalListOptions{})
	if err != nil {
		return nil, err
	}

	// Extract unique namespaces
	nsSet := make(map[string]bool)
	for _, signal := range signals {
		if signal.WorkloadNamespace != "" {
			// Apply namespace filter if provided
			if opts != nil && opts.Namespace != "" && signal.WorkloadNamespace != opts.Namespace {
				continue
			}
			nsSet[signal.WorkloadNamespace] = true
		}
	}

	hotspots := make([]Hotspot, 0)
	totalAnomalousSignals := 0

	for ns := range nsSet {
		nsResult, err := s.aggregator.AggregateNamespaceAnomaly(ctx, ns)
		if err != nil || nsResult == nil {
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
// 2. For each workload, compute aggregated anomaly
// 3. Filter where Score >= 0.5
// 4. Rank by score descending, limit to top 20
func (s *Service) GetNamespaceAnomalies(ctx context.Context, namespace string) (*NamespaceAnomaliesResult, error) {
	// Get signals in namespace to find workloads
	signals, err := s.registry.ListAllSignalAnchors(ctx, SignalListOptions{
		Namespace: namespace,
	})
	if err != nil {
		return nil, err
	}

	// Extract unique workload names
	workloadSet := make(map[string]bool)
	for _, signal := range signals {
		if signal.WorkloadName != "" {
			workloadSet[signal.WorkloadName] = true
		}
	}

	workloadAnomalies := make([]WorkloadAnomaly, 0)

	for workload := range workloadSet {
		wlResult, err := s.aggregator.AggregateWorkloadAnomaly(ctx, namespace, workload)
		if err != nil || wlResult == nil {
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
// 1. Query all SignalAnchors for the workload
// 2. For each signal with sufficient baseline, compute anomaly score
// 3. Filter where Score >= 0.5
// 4. Rank by score descending
func (s *Service) GetWorkloadAnomalyDetail(ctx context.Context, namespace, workload string) (*WorkloadAnomalyDetailResult, error) {
	// Get signals for this workload
	signals, err := s.registry.ListAllSignalAnchors(ctx, SignalListOptions{
		Namespace:    namespace,
		WorkloadName: workload,
	})
	if err != nil {
		return nil, err
	}

	signalAnomalies := make([]SignalAnomaly, 0)

	for _, signal := range signals {
		// Get baseline
		baseline, err := s.registry.GetSignalBaseline(ctx, signal.MetricName, namespace, workload)
		if err != nil || baseline == nil {
			continue
		}

		// Get current value
		currentValue, found, _ := s.registry.GetSignalCurrentValue(ctx, signal.MetricName, namespace, workload)
		if !found {
			currentValue = baseline.Mean
		}

		// Compute anomaly score
		score, err := ComputeAnomalyScore(currentValue, *baseline, signal.QualityScore)
		if err != nil {
			continue // Skip signals with insufficient samples
		}

		// Check alert state for override
		alertState, _ := s.registry.GetSignalAlertState(ctx, signal.MetricName, namespace, workload)
		if alertState == "firing" {
			score = ApplyAlertOverride(score, alertState)
		}

		// Filter by anomaly threshold
		if score.Score >= anomalyThreshold {
			signalAnomalies = append(signalAnomalies, SignalAnomaly{
				MetricName: signal.MetricName,
				Role:       string(signal.Role),
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

// GetRegistry returns the underlying registry (for direct access if needed).
func (s *Service) GetRegistry() *Registry {
	return s.registry
}

// GetAggregator returns the underlying aggregator (for direct access if needed).
func (s *Service) GetAggregator() *AnomalyAggregator {
	return s.aggregator
}

// ClearCache clears the aggregator cache (useful for testing).
func (s *Service) ClearCache() {
	s.aggregator.ClearCache()
}
