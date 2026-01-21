package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/api"
	causalpaths "github.com/moolen/spectre/internal/analysis/causal_paths"
)

// CausalPathsTool implements causal path discovery using GraphService
type CausalPathsTool struct {
	graphService *api.GraphService
}

// NewCausalPathsTool creates a new causal paths tool with GraphService
func NewCausalPathsTool(graphService *api.GraphService) *CausalPathsTool {
	return &CausalPathsTool{
		graphService: graphService,
	}
}

// CausalPathsInput defines the input parameters for MCP
type CausalPathsInput struct {
	ResourceUID      string `json:"resourceUID"`
	FailureTimestamp int64  `json:"failureTimestamp"`   // Unix seconds or nanoseconds
	LookbackMinutes  int    `json:"lookbackMinutes"`    // Optional: lookback window in minutes (default: 10)
	MaxDepth         int    `json:"maxDepth,omitempty"` // Optional: max traversal depth (default: 5)
	MaxPaths         int    `json:"maxPaths,omitempty"` // Optional: max paths to return (default: 5)
}

// Execute runs the causal path discovery (implements Tool interface)
func (t *CausalPathsTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params CausalPathsInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	// Validate required fields
	if params.ResourceUID == "" {
		return nil, fmt.Errorf("resourceUID is required")
	}
	if params.FailureTimestamp == 0 {
		return nil, fmt.Errorf("failureTimestamp is required")
	}

	// Set defaults
	if params.LookbackMinutes == 0 {
		params.LookbackMinutes = 10
	}
	if params.MaxDepth == 0 {
		params.MaxDepth = causalpaths.DefaultMaxDepth
	}
	if params.MaxPaths == 0 {
		params.MaxPaths = causalpaths.DefaultMaxPaths
	}

	// Apply limits
	if params.MaxDepth < causalpaths.MinMaxDepth {
		params.MaxDepth = causalpaths.MinMaxDepth
	}
	if params.MaxDepth > causalpaths.MaxMaxDepth {
		params.MaxDepth = causalpaths.MaxMaxDepth
	}
	if params.MaxPaths < causalpaths.MinMaxPaths {
		params.MaxPaths = causalpaths.MinMaxPaths
	}
	if params.MaxPaths > causalpaths.MaxMaxPaths {
		params.MaxPaths = causalpaths.MaxMaxPaths
	}

	// Normalize timestamp (convert seconds to nanoseconds if needed)
	failureTimestamp := normalizeTimestamp(params.FailureTimestamp)

	// Convert lookback minutes to nanoseconds
	lookbackNs := int64(params.LookbackMinutes) * 60 * 1_000_000_000

	// Call GraphService directly
	serviceInput := causalpaths.CausalPathsInput{
		ResourceUID:      params.ResourceUID,
		FailureTimestamp: failureTimestamp,
		LookbackNs:       lookbackNs,
		MaxDepth:         params.MaxDepth,
		MaxPaths:         params.MaxPaths,
	}
	response, err := t.graphService.DiscoverCausalPaths(ctx, serviceInput)
	if err != nil {
		return nil, fmt.Errorf("failed to discover causal paths: %w", err)
	}
	return response, nil
}
