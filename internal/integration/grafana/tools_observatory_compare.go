package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

// ObservatoryCompareTool provides time-based signal comparison for the Investigate stage.
// Compares current signal value/score against historical value at a lookback period.
//
// Per TOOL-11: Returns correlation analysis between current and past time
// Per TOOL-12: No categorical labels - just numeric scores
// Per CONTEXT.md: "Compare tool compares across time only (current vs N hours/days ago)"
type ObservatoryCompareTool struct {
	investigateService ObservatoryInvestigateServiceInterface
	logger             *logging.Logger
}

// NewObservatoryCompareTool creates a new compare tool.
// Accepts ObservatoryInvestigateServiceInterface to support both Grafana-specific and
// multi-provider registry-based services.
func NewObservatoryCompareTool(
	investigateService ObservatoryInvestigateServiceInterface,
	logger *logging.Logger,
) *ObservatoryCompareTool {
	return &ObservatoryCompareTool{
		investigateService: investigateService,
		logger:             logger,
	}
}

// ObservatoryCompareParams defines input parameters for the compare tool.
type ObservatoryCompareParams struct {
	Namespace  string `json:"namespace"`           // Required: Kubernetes namespace
	Workload   string `json:"workload"`            // Required: Workload name
	MetricName string `json:"metric_name"`         // Required: PromQL metric name
	Lookback   string `json:"lookback,omitempty"`  // Optional: Duration string (default "24h", max "168h"/7d)
}

// ObservatoryCompareResponse contains time-based signal comparison.
//
// Per TOOL-11, TOOL-12: Correlation analysis with numeric scores only
// ScoreDelta is the "correlation" - positive means worsening, negative means improving.
type ObservatoryCompareResponse struct {
	MetricName    string  `json:"metric_name"`
	CurrentValue  float64 `json:"current_value"`
	CurrentScore  float64 `json:"current_score"`  // Current anomaly score (0.0-1.0)
	PastValue     float64 `json:"past_value"`     // Value at lookback
	PastScore     float64 `json:"past_score"`     // Anomaly score at lookback
	ScoreDelta    float64 `json:"score_delta"`    // Current - Past (positive = worsening)
	LookbackHours int     `json:"lookback_hours"`
	Timestamp     string  `json:"timestamp"`
}

// MaxLookbackDuration is the maximum lookback duration (7 days).
const MaxLookbackDuration = 168 * time.Hour // 7 days

// DefaultLookbackDuration is the default lookback duration (24 hours).
const DefaultLookbackDuration = 24 * time.Hour

// Execute runs the compare tool.
//
// Process:
// 1. Unmarshal and validate parameters
// 2. Parse and validate lookback duration
// 3. Call investigateService.CompareSignal
// 4. Return comparison result with score delta
//
// Lookback parsing:
// - Default: "24h" if not specified
// - Maximum: "168h" (7 days) - caps at max if exceeded
// - Accepts Go duration strings: "1h", "12h", "24h", "48h", etc.
func (t *ObservatoryCompareTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
	var params ObservatoryCompareParams
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

	// Parse lookback duration
	lookback := DefaultLookbackDuration
	if params.Lookback != "" {
		parsed, err := time.ParseDuration(params.Lookback)
		if err != nil {
			return nil, fmt.Errorf("invalid lookback duration %q: %w", params.Lookback, err)
		}
		if parsed <= 0 {
			return nil, fmt.Errorf("lookback must be positive, got %v", parsed)
		}
		lookback = parsed
	}

	// Cap at maximum lookback (7 days)
	if lookback > MaxLookbackDuration {
		t.logger.Debug("Capping lookback from %v to max %v", lookback, MaxLookbackDuration)
		lookback = MaxLookbackDuration
	}

	t.logger.Debug("Comparing signal %s/%s/%s with lookback %v",
		params.Namespace, params.Workload, params.MetricName, lookback)

	// Call service to compare signal
	comparison, err := t.investigateService.CompareSignal(
		ctx,
		params.Namespace,
		params.Workload,
		params.MetricName,
		lookback,
	)
	if err != nil {
		return nil, fmt.Errorf("compare signal: %w", err)
	}

	// Build response
	response := &ObservatoryCompareResponse{
		MetricName:    comparison.MetricName,
		CurrentValue:  comparison.CurrentValue,
		CurrentScore:  comparison.CurrentScore,
		PastValue:     comparison.PastValue,
		PastScore:     comparison.PastScore,
		ScoreDelta:    comparison.ScoreDelta,
		LookbackHours: comparison.LookbackHours,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}

	return response, nil
}
