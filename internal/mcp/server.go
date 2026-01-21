package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/moolen/spectre/internal/integration"
	"github.com/moolen/spectre/internal/mcp/client"
	"github.com/moolen/spectre/internal/mcp/tools"
)

// Tool defines the interface for our existing tool implementations
type Tool interface {
	Execute(ctx context.Context, input json.RawMessage) (interface{}, error)
}

// SpectreServer wraps mcp-go server with Spectre-specific logic
type SpectreServer struct {
	mcpServer     *server.MCPServer
	spectreClient *SpectreClient
	tools         map[string]Tool
	version       string
}

// ServerOptions configures the Spectre MCP server
type ServerOptions struct {
	SpectreURL string
	Version    string
	Logger     client.Logger // Optional logger for retry messages
}

// NewSpectreServer creates a new Spectre MCP server
func NewSpectreServer(spectreURL, version string) (*SpectreServer, error) {
	return NewSpectreServerWithOptions(ServerOptions{
		SpectreURL: spectreURL,
		Version:    version,
	})
}

// NewSpectreServerWithOptions creates a new Spectre MCP server with optional graph support
func NewSpectreServerWithOptions(opts ServerOptions) (*SpectreServer, error) {
	// Test connection to Spectre with retry logic for container startup
	spectreClient := NewSpectreClient(opts.SpectreURL)
	if err := spectreClient.PingWithRetry(opts.Logger); err != nil {
		return nil, fmt.Errorf("failed to connect to Spectre API: %w", err)
	}

	// Create mcp-go server with capabilities
	mcpServer := server.NewMCPServer(
		"Spectre MCP Server",
		opts.Version,
		server.WithToolCapabilities(false), // No tool subscription for now
		server.WithLogging(),               // Enable logging capability
	)

	s := &SpectreServer{
		mcpServer:     mcpServer,
		spectreClient: spectreClient,
		tools:         make(map[string]Tool),
		version:       opts.Version,
	}

	// Register tools
	s.registerTools()

	// Register prompts
	s.registerPrompts()

	return s, nil
}

func (s *SpectreServer) registerTools() {
	// Register cluster_health tool
	s.registerTool(
		"cluster_health",
		"Get cluster health overview with resource status breakdown and top issues",
		tools.NewClusterHealthTool(s.spectreClient),
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"start_time": map[string]interface{}{
					"type":        "integer",
					"description": "Start timestamp (Unix seconds or milliseconds)",
				},
				"end_time": map[string]interface{}{
					"type":        "integer",
					"description": "End timestamp (Unix seconds or milliseconds)",
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Optional: filter by Kubernetes namespace",
				},
				"max_resources": map[string]interface{}{
					"type":        "integer",
					"description": "Optional: max resources to list per status (default 100, max 500)",
				},
			},
			"required": []string{"start_time", "end_time"},
		},
	)

	// Register resource_timeline_changes tool
	s.registerTool(
		"resource_timeline_changes",
		"Get semantic field-level changes for resources by UID with noise filtering and status condition summarization",
		tools.NewResourceTimelineChangesTool(s.spectreClient),
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"resource_uids": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "List of resource UIDs to query (required, max 10)",
				},
				"start_time": map[string]interface{}{
					"type":        "integer",
					"description": "Optional: start timestamp (Unix seconds or milliseconds). Default: 1 hour ago",
				},
				"end_time": map[string]interface{}{
					"type":        "integer",
					"description": "Optional: end timestamp (Unix seconds or milliseconds). Default: now",
				},
				"include_full_snapshot": map[string]interface{}{
					"type":        "boolean",
					"description": "Optional: include first segment's full resource JSON (default: false)",
				},
				"max_changes_per_resource": map[string]interface{}{
					"type":        "integer",
					"description": "Optional: max changes per resource (default 50, max 200)",
				},
			},
			"required": []string{"resource_uids"},
		},
	)

	// Register resource_timeline tool
	s.registerTool(
		"resource_timeline",
		"Get resource timeline with status segments, events, and transitions for root cause analysis",
		tools.NewResourceTimelineTool(s.spectreClient),
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"resource_kind": map[string]interface{}{
					"type":        "string",
					"description": "Resource kind to get timeline for (e.g., 'Pod', 'Deployment')",
				},
				"resource_name": map[string]interface{}{
					"type":        "string",
					"description": "Optional: specific resource name, or '*' for all",
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Optional: Kubernetes namespace to filter by",
				},
				"start_time": map[string]interface{}{
					"type":        "integer",
					"description": "Start timestamp (Unix seconds or milliseconds)",
				},
				"end_time": map[string]interface{}{
					"type":        "integer",
					"description": "End timestamp (Unix seconds or milliseconds)",
				},
				"max_results": map[string]interface{}{
					"type":        "integer",
					"description": "Optional: max resources to return when using '*' (default 20, max 100)",
				},
			},
			"required": []string{"resource_kind", "start_time", "end_time"},
		},
	)

	// Register detect_anomalies tool
	s.registerTool(
		"detect_anomalies",
		"Detect anomalies in a resource's causal subgraph including crash loops, config errors, state transitions, and networking issues",
		tools.NewDetectAnomaliesTool(s.spectreClient),
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"resource_uid": map[string]interface{}{
					"type":        "string",
					"description": "The UID of the resource to analyze for anomalies",
				},
				"start_time": map[string]interface{}{
					"type":        "integer",
					"description": "Start timestamp (Unix seconds or milliseconds)",
				},
				"end_time": map[string]interface{}{
					"type":        "integer",
					"description": "End timestamp (Unix seconds or milliseconds)",
				},
			},
			"required": []string{"resource_uid", "start_time", "end_time"},
		},
	)

	// Register causal_paths tool
	s.registerTool(
		"causal_paths",
		"Discover causal paths from root causes to a failing resource using graph-based causality analysis. Returns ranked paths with confidence scores.",
		tools.NewCausalPathsTool(s.spectreClient),
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"resourceUID": map[string]interface{}{
					"type":        "string",
					"description": "The UID of the resource that failed (symptom)",
				},
				"failureTimestamp": map[string]interface{}{
					"type":        "integer",
					"description": "Unix timestamp (seconds or nanoseconds) when the failure occurred",
				},
				"lookbackMinutes": map[string]interface{}{
					"type":        "integer",
					"description": "Optional: how far back to search for causes in minutes (default: 10)",
				},
				"maxDepth": map[string]interface{}{
					"type":        "integer",
					"description": "Optional: maximum depth to traverse causality chain (default: 5, max: 10)",
				},
				"maxPaths": map[string]interface{}{
					"type":        "integer",
					"description": "Optional: maximum number of causal paths to return (default: 5, max: 20)",
				},
			},
			"required": []string{"resourceUID", "failureTimestamp"},
		},
	)
}

func (s *SpectreServer) registerTool(name, description string, tool Tool, inputSchema map[string]interface{}) {
	// Store tool reference
	s.tools[name] = tool

	// Marshal schema to JSON
	schemaJSON, err := json.Marshal(inputSchema)
	if err != nil {
		// This should never happen with well-formed schemas
		panic(fmt.Sprintf("Failed to marshal schema for tool %s: %v", name, err))
	}

	// Create mcp.Tool definition with raw schema
	mcpTool := mcp.NewToolWithRawSchema(name, description, schemaJSON)

	// Register with mcp-go server using adapter
	s.mcpServer.AddTool(mcpTool, s.createToolHandler(tool))
}

func (s *SpectreServer) createToolHandler(tool Tool) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Marshal arguments to JSON for our existing tool interface
		args, err := json.Marshal(request.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid arguments: %v", err)), nil
		}

		// Execute tool with our existing interface
		result, err := tool.Execute(ctx, args)
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

func (s *SpectreServer) registerPrompts() {
	// Register post-mortem incident analysis prompt
	postMortemPrompt := mcp.Prompt{
		Name:        "post_mortem_incident_analysis",
		Description: "Conduct a comprehensive post-mortem analysis of a past incident",
		Arguments: []mcp.PromptArgument{
			{Name: "start_time", Description: "Start of the incident time window (Unix timestamp)", Required: true},
			{Name: "end_time", Description: "End of the incident time window (Unix timestamp)", Required: true},
			{Name: "namespace", Description: "Optional Kubernetes namespace", Required: false},
			{Name: "incident_description", Description: "Optional brief description", Required: false},
		},
	}

	s.mcpServer.AddPrompt(postMortemPrompt, func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		// Get arguments (mcp-go provides them as map[string]string)
		startTime := request.Params.Arguments["start_time"]
		endTime := request.Params.Arguments["end_time"]
		namespace := request.Params.Arguments["namespace"]

		// Build prompt message
		text := fmt.Sprintf("Analyze the incident from %s to %s. Use the investigate and cluster_health tools to gather evidence.", startTime, endTime)
		if namespace != "" {
			text += fmt.Sprintf(" Focus on namespace: %s", namespace)
		}

		// Build prompt messages
		messages := []mcp.PromptMessage{
			{
				Role: mcp.RoleUser,
				Content: mcp.TextContent{
					Type: "text",
					Text: text,
				},
			},
		}

		return &mcp.GetPromptResult{
			Description: "Post-mortem incident analysis workflow",
			Messages:    messages,
		}, nil
	})

	// Register live incident handling prompt
	liveIncidentPrompt := mcp.Prompt{
		Name:        "live_incident_handling",
		Description: "Triage and investigate an ongoing incident",
		Arguments: []mcp.PromptArgument{
			{Name: "incident_start_time", Description: "When symptoms first appeared (Unix timestamp)", Required: true},
			{Name: "current_time", Description: "Optional current time", Required: false},
			{Name: "namespace", Description: "Optional Kubernetes namespace", Required: false},
			{Name: "symptoms", Description: "Optional brief description of symptoms", Required: false},
		},
	}

	s.mcpServer.AddPrompt(liveIncidentPrompt, func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		// Get arguments (mcp-go provides them as map[string]string)
		incidentStartTime := request.Params.Arguments["incident_start_time"]
		namespace := request.Params.Arguments["namespace"]
		symptoms := request.Params.Arguments["symptoms"]

		// Build prompt message
		text := fmt.Sprintf("Investigate the ongoing incident starting at %s. Use cluster_health and investigate tools for triage.", incidentStartTime)
		if namespace != "" {
			text += fmt.Sprintf(" Focus on namespace: %s", namespace)
		}
		if symptoms != "" {
			text += fmt.Sprintf(" Reported symptoms: %s", symptoms)
		}

		// Build prompt messages
		messages := []mcp.PromptMessage{
			{
				Role: mcp.RoleUser,
				Content: mcp.TextContent{
					Type: "text",
					Text: text,
				},
			},
		}

		return &mcp.GetPromptResult{
			Description: "Live incident handling workflow",
			Messages:    messages,
		}, nil
	})
}

// GetMCPServer returns the underlying mcp-go server for transport setup
func (s *SpectreServer) GetMCPServer() *server.MCPServer {
	return s.mcpServer
}

// MCPToolRegistry adapts the integration.ToolRegistry interface to the mcp-go server.
// It allows integrations to register tools dynamically during startup.
type MCPToolRegistry struct {
	mcpServer *server.MCPServer
}

// NewMCPToolRegistry creates a new tool registry adapter.
func NewMCPToolRegistry(mcpServer *server.MCPServer) *MCPToolRegistry {
	return &MCPToolRegistry{
		mcpServer: mcpServer,
	}
}

// RegisterTool registers an MCP tool with the mcp-go server.
// It adapts the integration.ToolHandler to the mcp-go handler format.
func (r *MCPToolRegistry) RegisterTool(name string, handler integration.ToolHandler) error {
	// Validation
	if name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	// Generic schema (tools provide args via JSON)
	// Integration handlers will validate their own arguments
	inputSchema := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
	schemaJSON, err := json.Marshal(inputSchema)
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	// Create MCP tool with generic schema
	mcpTool := mcp.NewToolWithRawSchema(name, "", schemaJSON)

	// Adapter: integration.ToolHandler -> server.ToolHandlerFunc
	adaptedHandler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Marshal mcp arguments to []byte for integration handler
		args, err := json.Marshal(request.Params.Arguments)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid arguments: %v", err)), nil
		}

		// Call integration handler
		result, err := handler(ctx, args)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Tool execution failed: %v", err)), nil
		}

		// Format result as JSON
		resultJSON, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format result: %v", err)), nil
		}

		return mcp.NewToolResultText(string(resultJSON)), nil
	}

	r.mcpServer.AddTool(mcpTool, adaptedHandler)
	return nil
}
