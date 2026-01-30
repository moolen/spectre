package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

// ObservatoryScopeTool provides the Narrow stage MCP tool for scoping anomalies
// to a specific namespace or workload. Returns signals and anomalies ranked by severity.
type ObservatoryScopeTool struct {
	service *ObservatoryService
	logger  *logging.Logger
}

// NewObservatoryScopeTool creates a new observatory scope tool.
func NewObservatoryScopeTool(
	service *ObservatoryService,
	logger *logging.Logger,
) *ObservatoryScopeTool {
	return &ObservatoryScopeTool{
		service: service,
		logger:  logger,
	}
}

// ObservatoryScopeParams defines input parameters for the observatory_scope tool.
// Per TOOL-05: namespace required, workload optional for further narrowing.
type ObservatoryScopeParams struct {
	Namespace string `json:"namespace"`          // Required: namespace to scope to
	Workload  string `json:"workload,omitempty"` // Optional: further narrow to specific workload
}

// ObservatoryScopeResponse contains ranked anomalies for the specified scope.
// Per CONTEXT.md: "Narrow tools return ranked flat lists sorted by anomaly score"
type ObservatoryScopeResponse struct {
	Anomalies []ScopedAnomaly `json:"anomalies"`
	Scope     string          `json:"scope"`     // "namespace" or "namespace/workload"
	Timestamp string          `json:"timestamp"` // RFC3339
}

// ScopedAnomaly represents a single anomaly in the scoped view.
type ScopedAnomaly struct {
	Workload   string  `json:"workload,omitempty"` // Omitted if scope is workload-level
	MetricName string  `json:"metric_name"`
	Role       string  `json:"role"`
	Score      float64 `json:"score"`
	Confidence float64 `json:"confidence"`
}

// Execute runs the observatory_scope tool.
//
// Per TOOL-05 and TOOL-06:
//   - If only namespace provided: returns workload-level anomalies via GetNamespaceAnomalies
//   - If workload also provided: returns signal-level anomalies via GetWorkloadAnomalyDetail
//
// Returns flat list sorted by anomaly score descending.
// Returns empty anomalies array when nothing anomalous (per CONTEXT.md).
func (t *ObservatoryScopeTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
	var params ObservatoryScopeParams
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Validate namespace is provided
	if params.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}

	var anomalies []ScopedAnomaly
	var scope string

	if params.Workload != "" {
		// Narrow to workload: return signal-level anomalies
		scope = fmt.Sprintf("%s/%s", params.Namespace, params.Workload)

		result, err := t.service.GetWorkloadAnomalyDetail(ctx, params.Namespace, params.Workload)
		if err != nil {
			return nil, fmt.Errorf("get workload anomaly detail: %w", err)
		}

		// Convert SignalAnomaly to ScopedAnomaly (omit Workload field at this level)
		anomalies = make([]ScopedAnomaly, 0, len(result.Signals))
		for _, sig := range result.Signals {
			anomalies = append(anomalies, ScopedAnomaly{
				MetricName: sig.MetricName,
				Role:       sig.Role,
				Score:      sig.Score,
				Confidence: sig.Confidence,
			})
		}
	} else {
		// Namespace-level: return workload anomalies
		scope = params.Namespace

		result, err := t.service.GetNamespaceAnomalies(ctx, params.Namespace)
		if err != nil {
			return nil, fmt.Errorf("get namespace anomalies: %w", err)
		}

		// Convert WorkloadAnomaly to ScopedAnomaly (include Workload field)
		anomalies = make([]ScopedAnomaly, 0, len(result.Workloads))
		for _, wl := range result.Workloads {
			anomalies = append(anomalies, ScopedAnomaly{
				Workload:   wl.Name,
				MetricName: wl.TopSignal,
				Role:       "", // Role not available at workload aggregation level
				Score:      wl.Score,
				Confidence: wl.Confidence,
			})
		}
	}

	return &ObservatoryScopeResponse{
		Anomalies: anomalies,
		Scope:     scope,
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}
