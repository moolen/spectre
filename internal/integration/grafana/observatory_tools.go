// Package grafana provides Grafana metrics integration for Spectre.
package grafana

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// ObservatoryToolHandler is the common interface for observatory tools.
// Signature matches existing tool patterns: Execute(ctx, args []byte) (interface{}, error)
type ObservatoryToolHandler func(ctx context.Context, args []byte) (interface{}, error)

// wrapToolHandler adapts an ObservatoryToolHandler to the mcp-go ToolHandlerFunc signature.
// This allows our existing tool implementations to work with the mcp-go server.
func wrapToolHandler(handler ObservatoryToolHandler) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Marshal arguments to JSON for our tool interface
		args, err := json.Marshal(request.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid arguments: %v", err)), nil
		}

		// Execute tool with our existing interface
		result, err := handler(ctx, args)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Tool execution failed: %v", err)), nil
		}

		// Format result as JSON text
		resultJSON, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format result: %v", err)), nil
		}

		return mcp.NewToolResultText(string(resultJSON)), nil
	}
}

// RegisterObservatoryTools registers all 8 observatory MCP tools with the server.
//
// Tool categories (per CONTEXT.md progressive disclosure):
//   - Orient: observatory_status, observatory_changes - cluster-wide situation awareness
//   - Narrow: observatory_scope, observatory_signals - workload scoping
//   - Investigate: observatory_signal_detail, observatory_compare - deep signal inspection
//   - Hypothesize: observatory_explain - root cause candidates
//   - Verify: observatory_evidence - raw metrics, alerts, logs
//
// All tools return minimal JSON responses with numeric scores for AI interpretation.
func RegisterObservatoryTools(
	mcpServer *server.MCPServer,
	observatoryService *ObservatoryService,
	investigateService *ObservatoryInvestigateService,
	evidenceService *ObservatoryEvidenceService,
	graphClient graph.Client,
	integrationName string,
	logger *logging.Logger,
) {
	// Create tool instances
	statusTool := NewObservatoryStatusTool(observatoryService, logger)
	changesTool := NewObservatoryChangesTool(graphClient, integrationName, logger)
	scopeTool := NewObservatoryScopeTool(observatoryService, logger)
	signalsTool := NewObservatorySignalsTool(investigateService, logger)
	signalDetailTool := NewObservatorySignalDetailTool(investigateService, logger)
	compareTool := NewObservatoryCompareTool(investigateService, logger)
	explainTool := NewObservatoryExplainTool(evidenceService, logger)
	evidenceTool := NewObservatoryEvidenceTool(evidenceService, logger)

	// ============================================================================
	// Orient Stage Tools - Cluster-wide situation awareness
	// ============================================================================

	// observatory_status: Top 5 anomaly hotspots cluster-wide
	// Per TOOL-01, TOOL-02: Returns numeric scores, empty results when nothing anomalous
	mcpServer.AddTool(
		mcp.NewTool(
			"observatory_status",
			mcp.WithDescription("Get cluster-wide anomaly summary with top 5 hotspots by namespace/workload. Returns numeric scores (0.0-1.0) and empty array when nothing is anomalous."),
			mcp.WithString("cluster", mcp.Description("Optional: filter to specific cluster")),
			mcp.WithString("namespace", mcp.Description("Optional: filter to specific namespace")),
		),
		wrapToolHandler(statusTool.Execute),
	)

	// observatory_changes: Recent K8s deployment and config changes
	// Per TOOL-03, TOOL-04: Returns deployment, config, and reconciliation changes
	mcpServer.AddTool(
		mcp.NewTool(
			"observatory_changes",
			mcp.WithDescription("Get recent K8s changes (deployments, config updates, Flux reconciliations) that could explain anomalies. Returns max 20 changes."),
			mcp.WithString("namespace", mcp.Description("Optional: filter to specific namespace")),
			mcp.WithString("lookback", mcp.Description("Lookback duration (default: 1h, max: 24h). Format: 30m, 1h, 2h, etc.")),
		),
		wrapToolHandler(changesTool.Execute),
	)

	// ============================================================================
	// Narrow Stage Tools - Workload scoping
	// ============================================================================

	// observatory_scope: Namespace or workload anomaly scoping
	// Per TOOL-05, TOOL-06: Returns ranked flat lists sorted by anomaly score
	mcpServer.AddTool(
		mcp.NewTool(
			"observatory_scope",
			mcp.WithDescription("Get anomalies for a namespace or specific workload, ranked by severity. Returns flat list sorted by anomaly score."),
			mcp.WithString("namespace", mcp.Required(), mcp.Description("Kubernetes namespace (required)")),
			mcp.WithString("workload", mcp.Description("Optional: narrow to specific workload within namespace")),
		),
		wrapToolHandler(scopeTool.Execute),
	)

	// observatory_signals: Workload signal enumeration
	// Per TOOL-07, TOOL-08: Returns all signal anchors with current anomaly state
	mcpServer.AddTool(
		mcp.NewTool(
			"observatory_signals",
			mcp.WithDescription("Get all signal anchors for a workload with current anomaly state. Returns metric name, role, score, confidence, and quality."),
			mcp.WithString("namespace", mcp.Required(), mcp.Description("Kubernetes namespace (required)")),
			mcp.WithString("workload", mcp.Required(), mcp.Description("Workload name (required)")),
		),
		wrapToolHandler(signalsTool.Execute),
	)

	// ============================================================================
	// Investigate Stage Tools - Deep signal inspection
	// ============================================================================

	// observatory_signal_detail: Baseline stats and source dashboard
	// Per TOOL-09, TOOL-10: Returns baseline, current value, anomaly score, confidence
	mcpServer.AddTool(
		mcp.NewTool(
			"observatory_signal_detail",
			mcp.WithDescription("Get detailed signal info: baseline stats (mean, std_dev, percentiles), current value, anomaly score, confidence, and source dashboard."),
			mcp.WithString("namespace", mcp.Required(), mcp.Description("Kubernetes namespace (required)")),
			mcp.WithString("workload", mcp.Required(), mcp.Description("Workload name (required)")),
			mcp.WithString("metric_name", mcp.Required(), mcp.Description("Metric name (required)")),
		),
		wrapToolHandler(signalDetailTool.Execute),
	)

	// observatory_compare: Time-based signal comparison
	// Per TOOL-11, TOOL-12: Returns correlation analysis without categorical labels
	mcpServer.AddTool(
		mcp.NewTool(
			"observatory_compare",
			mcp.WithDescription("Compare signal value and anomaly score between current and past time. ScoreDelta positive means worsening."),
			mcp.WithString("namespace", mcp.Required(), mcp.Description("Kubernetes namespace (required)")),
			mcp.WithString("workload", mcp.Required(), mcp.Description("Workload name (required)")),
			mcp.WithString("metric_name", mcp.Required(), mcp.Description("Metric name (required)")),
			mcp.WithString("lookback", mcp.Description("Comparison lookback (default: 24h, max: 7d). Format: 1h, 12h, 24h, etc.")),
		),
		wrapToolHandler(compareTool.Execute),
	)

	// ============================================================================
	// Hypothesize Stage Tools - Root cause analysis
	// ============================================================================

	// observatory_explain: K8s graph candidates
	// Per TOOL-13, TOOL-14: Returns upstream deps (2-hop) and recent changes (1h)
	mcpServer.AddTool(
		mcp.NewTool(
			"observatory_explain",
			mcp.WithDescription("Get candidate root causes: upstream K8s dependencies (2-hop traversal) and recent changes (last 1h) for an anomalous signal."),
			mcp.WithString("namespace", mcp.Required(), mcp.Description("Kubernetes namespace (required)")),
			mcp.WithString("workload", mcp.Required(), mcp.Description("Workload name (required)")),
			mcp.WithString("metric_name", mcp.Required(), mcp.Description("Anomalous metric name (required)")),
		),
		wrapToolHandler(explainTool.Execute),
	)

	// ============================================================================
	// Verify Stage Tools - Evidence gathering
	// ============================================================================

	// observatory_evidence: Raw metric values, alerts, logs
	// Per TOOL-15, TOOL-16: Returns raw evidence for hypothesis verification
	mcpServer.AddTool(
		mcp.NewTool(
			"observatory_evidence",
			mcp.WithDescription("Get raw evidence for hypothesis verification: metric values, alert states, and log excerpts (ERROR level, 5-min window)."),
			mcp.WithString("namespace", mcp.Required(), mcp.Description("Kubernetes namespace (required)")),
			mcp.WithString("workload", mcp.Required(), mcp.Description("Workload name (required)")),
			mcp.WithString("metric_name", mcp.Required(), mcp.Description("Metric name (required)")),
			mcp.WithString("lookback", mcp.Description("Evidence lookback (default: 1h). Format: 30m, 1h, 2h, etc.")),
		),
		wrapToolHandler(evidenceTool.Execute),
	)

	logger.Info("Registered 8 observatory MCP tools (status, changes, scope, signals, signal_detail, compare, explain, evidence)")
}
