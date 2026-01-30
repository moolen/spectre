package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

// ObservatorySignalsTool provides the Narrow stage MCP tool for viewing all
// signal anchors for a workload with their current anomaly state.
type ObservatorySignalsTool struct {
	investigateService *ObservatoryInvestigateService
	logger             *logging.Logger
}

// NewObservatorySignalsTool creates a new observatory signals tool.
func NewObservatorySignalsTool(
	investigateService *ObservatoryInvestigateService,
	logger *logging.Logger,
) *ObservatorySignalsTool {
	return &ObservatorySignalsTool{
		investigateService: investigateService,
		logger:             logger,
	}
}

// ObservatorySignalsParams defines input parameters for the observatory_signals tool.
// Per TOOL-07: both namespace and workload are required.
type ObservatorySignalsParams struct {
	Namespace string `json:"namespace"` // Required: namespace
	Workload  string `json:"workload"`  // Required: workload name
}

// ObservatorySignalsResponse contains all signals for a workload with current state.
// Per CONTEXT.md: "Narrow tools return ranked flat lists sorted by anomaly score"
type ObservatorySignalsResponse struct {
	Signals   []SignalState `json:"signals"`
	Scope     string        `json:"scope"`     // "namespace/workload"
	Timestamp string        `json:"timestamp"` // RFC3339
}

// SignalState represents the current anomaly state of a signal anchor.
type SignalState struct {
	MetricName   string  `json:"metric_name"`
	Role         string  `json:"role"` // Availability, Latency, Errors, etc.
	Score        float64 `json:"score"`
	Confidence   float64 `json:"confidence"`
	QualityScore float64 `json:"quality_score"` // Source dashboard quality
}

// Execute runs the observatory_signals tool.
//
// Per TOOL-07 and TOOL-08:
//   - Returns all signal anchors for the specified workload
//   - Each signal includes metric_name, role, score, confidence, quality_score
//   - Signals sorted by anomaly score descending
//
// Returns empty signals array when no signals for workload (per CONTEXT.md).
func (t *ObservatorySignalsTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
	var params ObservatorySignalsParams
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Validate namespace and workload are provided
	if params.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	if params.Workload == "" {
		return nil, fmt.Errorf("workload is required")
	}

	// Call investigate service to get workload signals
	result, err := t.investigateService.GetWorkloadSignals(ctx, params.Namespace, params.Workload)
	if err != nil {
		return nil, fmt.Errorf("get workload signals: %w", err)
	}

	// Convert SignalSummary to SignalState
	signals := make([]SignalState, 0, len(result.Signals))
	for _, sig := range result.Signals {
		signals = append(signals, SignalState{
			MetricName:   sig.MetricName,
			Role:         sig.Role,
			Score:        sig.Score,
			Confidence:   sig.Confidence,
			QualityScore: sig.QualityScore,
		})
	}

	return &ObservatorySignalsResponse{
		Signals:   signals,
		Scope:     fmt.Sprintf("%s/%s", params.Namespace, params.Workload),
		Timestamp: time.Now().Format(time.RFC3339),
	}, nil
}
