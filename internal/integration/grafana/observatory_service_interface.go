package grafana

import (
	"context"
	"time"
)

// ObservatoryServiceInterface defines the contract for observatory services
// that provide cluster/namespace/workload anomaly data. Both the Grafana-specific
// ObservatoryService and the multi-provider observatory.Service implement this interface.
//
// This allows MCP tools to work with either implementation, enabling gradual
// migration from Grafana-specific services to the multi-provider registry.
type ObservatoryServiceInterface interface {
	// GetClusterAnomalies returns cluster-wide anomaly summary with top hotspots.
	// Returns anomalies filtered by optional scope options.
	GetClusterAnomalies(ctx context.Context, opts *ScopeOptions) (*ClusterAnomaliesResult, error)

	// GetNamespaceAnomalies returns workload-level anomalies within a namespace.
	// Returns anomalies ranked by severity.
	GetNamespaceAnomalies(ctx context.Context, namespace string) (*NamespaceAnomaliesResult, error)

	// GetWorkloadAnomalyDetail returns signal-level anomaly details for a workload.
	// Returns all anomalous signals for the specified workload.
	GetWorkloadAnomalyDetail(ctx context.Context, namespace, workload string) (*WorkloadAnomalyDetailResult, error)
}

// ObservatoryInvestigateServiceInterface defines the contract for investigation services
// that provide deep signal inspection. Both the Grafana-specific ObservatoryInvestigateService
// and the multi-provider observatory.InvestigateService implement this interface.
type ObservatoryInvestigateServiceInterface interface {
	// GetWorkloadSignals returns all signals for a workload with current anomaly scores.
	// Used for the Narrow stage to enumerate available signals.
	GetWorkloadSignals(ctx context.Context, namespace, workload string) (*WorkloadSignalsResult, error)

	// GetSignalDetail returns detailed baseline and anomaly information for a signal.
	// Used for the Investigate stage for deep signal inspection.
	GetSignalDetail(ctx context.Context, namespace, workload, metricName string) (*SignalDetailResult, error)

	// CompareSignal compares signal values across time periods.
	// Used for the Investigate stage to detect trending changes.
	CompareSignal(ctx context.Context, namespace, workload, metricName string, lookback time.Duration) (*SignalComparisonResult, error)
}

// Verify that existing services implement the interfaces
var _ ObservatoryServiceInterface = (*ObservatoryService)(nil)
var _ ObservatoryInvestigateServiceInterface = (*ObservatoryInvestigateService)(nil)
