package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

// ObservatorySignalDetailTool provides deep signal inspection for the Investigate stage.
// Returns baseline stats, current value, anomaly score, source dashboard, and confidence.
//
// Per TOOL-09: Returns baseline, current value, anomaly score, and source dashboard
// Per TOOL-10: Returns confidence for statistical reliability
type ObservatorySignalDetailTool struct {
	investigateService *ObservatoryInvestigateService
	logger             *logging.Logger
}

// NewObservatorySignalDetailTool creates a new signal detail tool.
func NewObservatorySignalDetailTool(
	investigateService *ObservatoryInvestigateService,
	logger *logging.Logger,
) *ObservatorySignalDetailTool {
	return &ObservatorySignalDetailTool{
		investigateService: investigateService,
		logger:             logger,
	}
}

// ObservatorySignalDetailParams defines input parameters for the signal detail tool.
type ObservatorySignalDetailParams struct {
	Namespace  string `json:"namespace"`    // Required: Kubernetes namespace
	Workload   string `json:"workload"`     // Required: Workload name
	MetricName string `json:"metric_name"`  // Required: PromQL metric name
}

// ObservatorySignalDetailResponse contains detailed signal information for deep inspection.
//
// Per TOOL-09: baseline, current value, anomaly score, source dashboard
// Per TOOL-10: confidence for statistical reliability
type ObservatorySignalDetailResponse struct {
	MetricName      string                       `json:"metric_name"`
	Role            string                       `json:"role"`
	CurrentValue    float64                      `json:"current_value"`
	Baseline        ObservatoryBaselineStats     `json:"baseline"`
	AnomalyScore    float64                      `json:"anomaly_score"`
	Confidence      float64                      `json:"confidence"`
	SourceDashboard string                       `json:"source_dashboard"` // Dashboard UID
	QualityScore    float64                      `json:"quality_score"`
	Timestamp       string                       `json:"timestamp"`
}

// ObservatoryBaselineStats contains baseline statistical information.
// Separate type from service's BaselineStats to allow tool-specific customization.
type ObservatoryBaselineStats struct {
	Mean        float64 `json:"mean"`
	StdDev      float64 `json:"std_dev"`
	P50         float64 `json:"p50"`
	P90         float64 `json:"p90"`
	P99         float64 `json:"p99"`
	SampleCount int     `json:"sample_count"`
}

// Execute runs the signal detail tool.
//
// Process:
// 1. Unmarshal and validate parameters
// 2. Call investigateService.GetSignalDetail
// 3. Return detailed signal response
//
// Errors:
// - Missing required parameters: returns validation error
// - Signal not found: returns clear error message
// - Insufficient baseline samples: returns partial data with confidence = 0
func (t *ObservatorySignalDetailTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
	var params ObservatorySignalDetailParams
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Validate required parameters
	if params.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	if params.Workload == "" {
		return nil, fmt.Errorf("workload is required")
	}
	if params.MetricName == "" {
		return nil, fmt.Errorf("metric_name is required")
	}

	t.logger.Debug("Getting signal detail for %s/%s/%s", params.Namespace, params.Workload, params.MetricName)

	// Call service to get signal detail
	detail, err := t.investigateService.GetSignalDetail(ctx, params.Namespace, params.Workload, params.MetricName)
	if err != nil {
		// Check if it's a cold start / insufficient baseline case
		if containsInsufficientBaseline(err) {
			t.logger.Debug("Signal %s has insufficient baseline: %v", params.MetricName, err)
			// Return partial data with confidence = 0
			return &ObservatorySignalDetailResponse{
				MetricName:      params.MetricName,
				Role:            "",
				CurrentValue:    0,
				Baseline:        ObservatoryBaselineStats{},
				AnomalyScore:    0,
				Confidence:      0, // Indicate insufficient data
				SourceDashboard: "",
				QualityScore:    0,
				Timestamp:       time.Now().UTC().Format(time.RFC3339),
			}, nil
		}
		return nil, fmt.Errorf("get signal detail: %w", err)
	}

	// Build response
	response := &ObservatorySignalDetailResponse{
		MetricName:   detail.MetricName,
		Role:         detail.Role,
		CurrentValue: detail.CurrentValue,
		Baseline: ObservatoryBaselineStats{
			Mean:        detail.Baseline.Mean,
			StdDev:      detail.Baseline.StdDev,
			P50:         detail.Baseline.P50,
			P90:         detail.Baseline.P90,
			P99:         detail.Baseline.P99,
			SampleCount: detail.Baseline.SampleCount,
		},
		AnomalyScore:    detail.AnomalyScore,
		Confidence:      detail.Confidence,
		SourceDashboard: detail.SourceDashboard,
		QualityScore:    detail.QualityScore,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	}

	return response, nil
}

// containsInsufficientBaseline checks if error indicates insufficient baseline samples.
func containsInsufficientBaseline(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "no baseline") ||
		strings.Contains(errStr, "cold start") ||
		strings.Contains(errStr, "insufficient")
}
