package grafana

import (
	"context"
	"time"

	"github.com/moolen/spectre/internal/observatory"
)

// ObservatoryServiceAdapter wraps observatory.Service to implement ObservatoryServiceInterface.
// This adapter converts between observatory package types and grafana package types,
// enabling MCP tools to work with the multi-provider registry-based service.
type ObservatoryServiceAdapter struct {
	service *observatory.Service
}

// NewObservatoryServiceAdapter creates a new adapter wrapping an observatory.Service.
func NewObservatoryServiceAdapter(service *observatory.Service) *ObservatoryServiceAdapter {
	return &ObservatoryServiceAdapter{service: service}
}

// GetClusterAnomalies implements ObservatoryServiceInterface.
func (a *ObservatoryServiceAdapter) GetClusterAnomalies(ctx context.Context, opts *ScopeOptions) (*ClusterAnomaliesResult, error) {
	// Convert grafana.ScopeOptions to observatory.ScopeOptions
	var obsOpts *observatory.ScopeOptions
	if opts != nil {
		obsOpts = &observatory.ScopeOptions{
			Namespace: opts.Namespace,
			Workload:  opts.Workload,
		}
	}

	result, err := a.service.GetClusterAnomalies(ctx, obsOpts)
	if err != nil {
		return nil, err
	}

	// Convert observatory types to grafana types
	hotspots := make([]Hotspot, len(result.TopHotspots))
	for i, h := range result.TopHotspots {
		hotspots[i] = Hotspot{
			Namespace:   h.Namespace,
			Workload:    h.Workload,
			Score:       h.Score,
			Confidence:  h.Confidence,
			SignalCount: h.SignalCount,
		}
	}

	return &ClusterAnomaliesResult{
		TopHotspots:           hotspots,
		TotalAnomalousSignals: result.TotalAnomalousSignals,
		Timestamp:             result.Timestamp,
	}, nil
}

// GetNamespaceAnomalies implements ObservatoryServiceInterface.
func (a *ObservatoryServiceAdapter) GetNamespaceAnomalies(ctx context.Context, namespace string) (*NamespaceAnomaliesResult, error) {
	result, err := a.service.GetNamespaceAnomalies(ctx, namespace)
	if err != nil {
		return nil, err
	}

	// Convert observatory types to grafana types
	workloads := make([]WorkloadAnomaly, len(result.Workloads))
	for i, w := range result.Workloads {
		workloads[i] = WorkloadAnomaly{
			Name:        w.Name,
			Score:       w.Score,
			Confidence:  w.Confidence,
			SignalCount: w.SignalCount,
			TopSignal:   w.TopSignal,
		}
	}

	return &NamespaceAnomaliesResult{
		Workloads: workloads,
		Namespace: result.Namespace,
		Timestamp: result.Timestamp,
	}, nil
}

// GetWorkloadAnomalyDetail implements ObservatoryServiceInterface.
func (a *ObservatoryServiceAdapter) GetWorkloadAnomalyDetail(ctx context.Context, namespace, workload string) (*WorkloadAnomalyDetailResult, error) {
	result, err := a.service.GetWorkloadAnomalyDetail(ctx, namespace, workload)
	if err != nil {
		return nil, err
	}

	// Convert observatory types to grafana types
	signals := make([]SignalAnomaly, len(result.Signals))
	for i, s := range result.Signals {
		signals[i] = SignalAnomaly{
			MetricName: s.MetricName,
			Role:       s.Role,
			Score:      s.Score,
			Confidence: s.Confidence,
		}
	}

	return &WorkloadAnomalyDetailResult{
		Signals:   signals,
		Namespace: result.Namespace,
		Workload:  result.Workload,
		Timestamp: result.Timestamp,
	}, nil
}

// Verify adapter implements interface
var _ ObservatoryServiceInterface = (*ObservatoryServiceAdapter)(nil)

// ObservatoryInvestigateServiceAdapter wraps observatory.InvestigateService to implement
// ObservatoryInvestigateServiceInterface. This adapter converts between observatory package
// types and grafana package types.
type ObservatoryInvestigateServiceAdapter struct {
	service *observatory.InvestigateService
}

// NewObservatoryInvestigateServiceAdapter creates a new adapter wrapping an observatory.InvestigateService.
func NewObservatoryInvestigateServiceAdapter(service *observatory.InvestigateService) *ObservatoryInvestigateServiceAdapter {
	return &ObservatoryInvestigateServiceAdapter{service: service}
}

// GetWorkloadSignals implements ObservatoryInvestigateServiceInterface.
func (a *ObservatoryInvestigateServiceAdapter) GetWorkloadSignals(ctx context.Context, namespace, workload string) (*WorkloadSignalsResult, error) {
	result, err := a.service.GetWorkloadSignals(ctx, namespace, workload)
	if err != nil {
		return nil, err
	}

	// Convert observatory types to grafana types
	signals := make([]SignalSummary, len(result.Signals))
	for i, s := range result.Signals {
		signals[i] = SignalSummary{
			MetricName:   s.MetricName,
			Role:         s.Role,
			Score:        s.Score,
			Confidence:   s.Confidence,
			QualityScore: s.QualityScore,
		}
	}

	return &WorkloadSignalsResult{
		Signals: signals,
		Scope:   result.Scope,
	}, nil
}

// GetSignalDetail implements ObservatoryInvestigateServiceInterface.
func (a *ObservatoryInvestigateServiceAdapter) GetSignalDetail(ctx context.Context, namespace, workload, metricName string) (*SignalDetailResult, error) {
	result, err := a.service.GetSignalDetail(ctx, namespace, workload, metricName)
	if err != nil {
		return nil, err
	}

	return &SignalDetailResult{
		MetricName:      result.MetricName,
		Role:            result.Role,
		CurrentValue:    result.CurrentValue,
		Baseline: BaselineStats{
			Mean:        result.Baseline.Mean,
			StdDev:      result.Baseline.StdDev,
			P50:         result.Baseline.P50,
			P90:         result.Baseline.P90,
			P99:         result.Baseline.P99,
			SampleCount: result.Baseline.SampleCount,
		},
		AnomalyScore:    result.AnomalyScore,
		Confidence:      result.Confidence,
		SourceDashboard: result.SourceProvider, // Map SourceProvider to SourceDashboard
		QualityScore:    result.QualityScore,
	}, nil
}

// CompareSignal implements ObservatoryInvestigateServiceInterface.
func (a *ObservatoryInvestigateServiceAdapter) CompareSignal(ctx context.Context, namespace, workload, metricName string, lookback time.Duration) (*SignalComparisonResult, error) {
	result, err := a.service.CompareSignal(ctx, namespace, workload, metricName, lookback)
	if err != nil {
		return nil, err
	}

	return &SignalComparisonResult{
		MetricName:    result.MetricName,
		CurrentValue:  result.CurrentValue,
		CurrentScore:  result.CurrentScore,
		PastValue:     result.PastValue,
		PastScore:     result.PastScore,
		LookbackHours: result.LookbackHours,
		ScoreDelta:    result.ScoreDelta,
	}, nil
}

// Verify adapter implements interface
var _ ObservatoryInvestigateServiceInterface = (*ObservatoryInvestigateServiceAdapter)(nil)
