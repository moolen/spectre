package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
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

// NewSpectreServer creates a new Spectre MCP server
func NewSpectreServer(spectreURL, version string) (*SpectreServer, error) {
	// Test connection to Spectre
	client := NewSpectreClient(spectreURL)
	if err := client.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to Spectre API: %w", err)
	}

	// Create mcp-go server with capabilities
	mcpServer := server.NewMCPServer(
		"Spectre MCP Server",
		version,
		server.WithToolCapabilities(false), // No tool subscription for now
		server.WithLogging(),               // Enable logging capability
	)

	s := &SpectreServer{
		mcpServer:     mcpServer,
		spectreClient: client,
		tools:         make(map[string]Tool),
		version:       version,
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

	// Register resource_changes tool
	s.registerTool(
		"resource_changes",
		"Get summarized resource changes with categorization and impact scoring for LLM analysis",
		tools.NewResourceChangesTool(s.spectreClient),
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
				"kinds": map[string]interface{}{
					"type":        "string",
					"description": "Optional: comma-separated resource kinds to filter (e.g., 'Pod,Deployment')",
				},
				"impact_threshold": map[string]interface{}{
					"type":        "number",
					"description": "Optional: minimum impact score 0-1.0 to include in results",
				},
				"max_resources": map[string]interface{}{
					"type":        "integer",
					"description": "Optional: max resources to return (default 50, max 500)",
				},
			},
			"required": []string{"start_time", "end_time"},
		},
	)

	// Register investigate tool
	s.registerTool(
		"investigate",
		"Get detailed investigation evidence with status timeline, events, and investigation prompts for RCA",
		tools.NewInvestigateTool(s.spectreClient),
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"resource_kind": map[string]interface{}{
					"type":        "string",
					"description": "Resource kind to investigate (e.g., 'Pod', 'Deployment')",
				},
				"resource_name": map[string]interface{}{
					"type":        "string",
					"description": "Optional: specific resource name to investigate, or '*' for all",
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
				"investigation_type": map[string]interface{}{
					"type":        "string",
					"description": "Optional: 'incident' for live response, 'post-mortem' for historical analysis, or 'auto' to detect",
				},
				"max_investigations": map[string]interface{}{
					"type":        "integer",
					"description": "Optional: max resources to investigate when using '*' (default 20, max 100)",
				},
			},
			"required": []string{"resource_kind", "start_time", "end_time"},
		},
	)

	// Register resource_explorer tool
	s.registerTool(
		"resource_explorer",
		"Browse and discover resources in the cluster with filtering and status overview",
		tools.NewResourceExplorerTool(s.spectreClient),
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"kind": map[string]interface{}{
					"type":        "string",
					"description": "Optional: filter by resource kind (e.g., 'Pod', 'Deployment')",
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Optional: filter by Kubernetes namespace",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"description": "Optional: filter by status (Ready, Warning, Error, Terminating)",
				},
				"time": map[string]interface{}{
					"type":        "integer",
					"description": "Optional: snapshot at specific time (Unix seconds), 0 or omit for latest",
				},
				"max_resources": map[string]interface{}{
					"type":        "integer",
					"description": "Optional: max resources to return (default 200, max 1000)",
				},
			},
			"required": []string{},
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
