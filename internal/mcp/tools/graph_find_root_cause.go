package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/analysis"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// GraphFindRootCauseTool implements root cause analysis using the graph
type GraphFindRootCauseTool struct {
	analyzer *analysis.RootCauseAnalyzer
	logger   *logging.Logger
}

// NewGraphFindRootCauseTool creates a new root cause analysis tool
func NewGraphFindRootCauseTool(graphClient graph.Client) *GraphFindRootCauseTool {
	return &GraphFindRootCauseTool{
		analyzer: analysis.NewRootCauseAnalyzer(graphClient),
		logger:   logging.GetLogger("mcp.tools.root_cause"),
	}
}

// FindRootCauseInput defines the input parameters for MCP
type FindRootCauseInput struct {
	ResourceUID      string  `json:"resourceUID"`
	FailureTimestamp int64   `json:"failureTimestamp"` // Unix seconds or nanoseconds
	MaxDepth         int     `json:"maxDepth,omitempty"`
	MinConfidence    float64 `json:"minConfidence,omitempty"`
}

// Execute runs the root cause analysis using V2 causality-first approach
func (t *GraphFindRootCauseTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	// Use V2 implementation by default
	return t.ExecuteV2(ctx, input)
}

// ExecuteV2 runs the new causality-first root cause analysis
func (t *GraphFindRootCauseTool) ExecuteV2(ctx context.Context, input json.RawMessage) (*analysis.RootCauseAnalysisV2, error) {
	var params FindRootCauseInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	// Set defaults
	if params.MaxDepth == 0 {
		params.MaxDepth = 5
	}
	if params.MinConfidence == 0 {
		params.MinConfidence = 0.6
	}

	// Normalize timestamp (convert seconds to nanoseconds if needed)
	failureTimestamp := normalizeTimestamp(params.FailureTimestamp)

	// Delegate to shared analyzer
	return t.analyzer.Analyze(ctx, analysis.AnalyzeInput{
		ResourceUID:      params.ResourceUID,
		FailureTimestamp: failureTimestamp,
		MaxDepth:         params.MaxDepth,
		MinConfidence:    params.MinConfidence,
	})
}
