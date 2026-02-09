package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

// ObservatoryEvidenceTool provides raw evidence data for hypothesis verification.
// It aggregates metric values, alert states, and log excerpts for a specific signal.
type ObservatoryEvidenceTool struct {
	evidenceService *ObservatoryEvidenceService
	logger          *logging.Logger
}

// NewObservatoryEvidenceTool creates a new ObservatoryEvidenceTool instance.
func NewObservatoryEvidenceTool(
	evidenceService *ObservatoryEvidenceService,
	logger *logging.Logger,
) *ObservatoryEvidenceTool {
	return &ObservatoryEvidenceTool{
		evidenceService: evidenceService,
		logger:          logger,
	}
}

// ObservatoryEvidenceParams defines input parameters for the observatory_evidence tool.
type ObservatoryEvidenceParams struct {
	// Namespace is the K8s namespace (required)
	Namespace string `json:"namespace"`

	// Workload is the K8s workload name (required)
	Workload string `json:"workload"`

	// MetricName is the signal metric to get evidence for (required)
	MetricName string `json:"metric_name"`

	// Lookback is the time window for evidence (default "1h")
	// Supported formats: "30m", "1h", "2h", "24h"
	Lookback string `json:"lookback,omitempty"`
}

// ObservatoryEvidenceResponse contains raw evidence data for verification.
type ObservatoryEvidenceResponse struct {
	// MetricValues are the raw metric data points in the lookback window
	MetricValues []MetricValue `json:"metric_values"`

	// AlertStates are the alert state transitions for related alerts
	AlertStates []EvidenceAlertState `json:"alert_states"`

	// LogExcerpts are relevant log entries (ERROR level, 5-minute window)
	// May be empty if log integration is not configured
	LogExcerpts []LogExcerpt `json:"log_excerpts"`

	// Lookback is the time window used for evidence gathering
	Lookback string `json:"lookback"`

	// Timestamp is when this result was computed (ISO8601)
	Timestamp string `json:"timestamp"`
}

// Execute runs the observatory_evidence tool.
// Returns raw metric values, alert states, and log excerpts for verification.
func (t *ObservatoryEvidenceTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
	var params ObservatoryEvidenceParams
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

	// Parse lookback duration (default 1h)
	lookback := 1 * time.Hour
	lookbackStr := "1h"
	if params.Lookback != "" {
		parsed, err := time.ParseDuration(params.Lookback)
		if err != nil {
			return nil, fmt.Errorf("invalid lookback format: %w (use format like \"30m\", \"1h\", \"2h\")", err)
		}
		lookback = parsed
		lookbackStr = params.Lookback
	}

	t.logger.Debug("observatory_evidence: namespace=%s, workload=%s, metric=%s, lookback=%s",
		params.Namespace, params.Workload, params.MetricName, lookbackStr)

	// Get signal evidence from evidence service
	result, err := t.evidenceService.GetSignalEvidence(
		ctx,
		params.Namespace,
		params.Workload,
		params.MetricName,
		lookback,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get signal evidence: %w", err)
	}

	// Build response with raw data for AI interpretation
	// Note: LogExcerpts may be empty if log integration is not configured (graceful degradation)
	return &ObservatoryEvidenceResponse{
		MetricValues: result.MetricValues,
		AlertStates:  result.AlertStates,
		LogExcerpts:  result.LogExcerpts,
		Lookback:     lookbackStr,
		Timestamp:    time.Now().Format(time.RFC3339),
	}, nil
}
