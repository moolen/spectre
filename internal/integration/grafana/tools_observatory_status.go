package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

// ObservatoryStatusTool provides cluster-wide anomaly summary for the Orient stage.
// Returns top 5 hotspots with numeric scores - the entry point for AI-driven investigation.
type ObservatoryStatusTool struct {
	service *ObservatoryService
	logger  *logging.Logger
}

// NewObservatoryStatusTool creates a new observatory status tool.
func NewObservatoryStatusTool(
	service *ObservatoryService,
	logger *logging.Logger,
) *ObservatoryStatusTool {
	return &ObservatoryStatusTool{
		service: service,
		logger:  logger,
	}
}

// ObservatoryStatusParams defines input parameters for the observatory_status tool.
type ObservatoryStatusParams struct {
	Cluster   string `json:"cluster,omitempty"`   // Optional: filter to cluster
	Namespace string `json:"namespace,omitempty"` // Optional: filter to namespace
}

// ObservatoryStatusResponse contains cluster-wide anomaly summary.
// Per CONTEXT.md: minimal JSON responses with numeric scores, empty results when nothing anomalous.
type ObservatoryStatusResponse struct {
	TopHotspots           []Hotspot `json:"top_hotspots"`
	TotalAnomalousSignals int       `json:"total_anomalous_signals"`
	Timestamp             string    `json:"timestamp"` // RFC3339
}

// Execute runs the observatory_status tool.
func (t *ObservatoryStatusTool) Execute(ctx context.Context, args []byte) (interface{}, error) {
	var params ObservatoryStatusParams
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid parameters: %w", err)
	}

	// Build scope options from params
	opts := &ScopeOptions{
		Cluster:   params.Cluster,
		Namespace: params.Namespace,
	}

	// Call service to get cluster anomalies
	result, err := t.service.GetClusterAnomalies(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("get cluster anomalies: %w", err)
	}

	// Return result directly - service already returns minimal structure
	// Per CONTEXT.md: empty results when nothing anomalous (empty array not "healthy" message)
	return &ObservatoryStatusResponse{
		TopHotspots:           result.TopHotspots,
		TotalAnomalousSignals: result.TotalAnomalousSignals,
		Timestamp:             time.Now().Format(time.RFC3339),
	}, nil
}
