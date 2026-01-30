package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

// ObservatoryExplainTool provides root cause candidates for anomalous signals.
// It queries the K8s graph for upstream dependencies (2-hop traversal) and
// recent changes (1 hour) that could explain the anomaly.
type ObservatoryExplainTool struct {
	evidenceService *ObservatoryEvidenceService
	logger          *logging.Logger
}

// NewObservatoryExplainTool creates a new ObservatoryExplainTool instance.
func NewObservatoryExplainTool(
	evidenceService *ObservatoryEvidenceService,
	logger *logging.Logger,
) *ObservatoryExplainTool {
	return &ObservatoryExplainTool{
		evidenceService: evidenceService,
		logger:          logger,
	}
}

// ObservatoryExplainParams defines input parameters for the observatory_explain tool.
type ObservatoryExplainParams struct {
	// Namespace is the K8s namespace (required)
	Namespace string `json:"namespace"`

	// Workload is the K8s workload name (required)
	Workload string `json:"workload"`

	// MetricName is the anomalous signal metric (required)
	MetricName string `json:"metric_name"`
}

// ObservatoryExplainResponse contains candidate root causes from K8s graph analysis.
type ObservatoryExplainResponse struct {
	// UpstreamDeps are dependencies found via 2-hop upstream traversal
	UpstreamDeps []UpstreamDependency `json:"upstream_deps"`

	// RecentChanges are K8s events (deployments, config changes) in the last hour
	RecentChanges []RecentChange `json:"recent_changes"`

	// Timestamp is when this result was computed (ISO8601)
	Timestamp string `json:"timestamp"`
}

// Execute runs the observatory_explain tool.
// Returns candidate causes from K8s graph for the specified anomalous signal.
func (t *ObservatoryExplainTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
	var params ObservatoryExplainParams
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

	t.logger.Debug("observatory_explain: namespace=%s, workload=%s, metric=%s",
		params.Namespace, params.Workload, params.MetricName)

	// Get candidate causes from evidence service
	result, err := t.evidenceService.GetCandidateCauses(
		ctx,
		params.Namespace,
		params.Workload,
		params.MetricName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get candidate causes: %w", err)
	}

	// Build response with raw data for AI interpretation
	return &ObservatoryExplainResponse{
		UpstreamDeps:  result.UpstreamDeps,
		RecentChanges: result.RecentChanges,
		Timestamp:     time.Now().Format(time.RFC3339),
	}, nil
}
