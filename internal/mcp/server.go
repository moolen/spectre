package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	appgraph "github.com/moolen/spectre/internal/app/graph"
	apptimeline "github.com/moolen/spectre/internal/app/timeline"
	"github.com/moolen/spectre/internal/integration"
)

// Tool defines the interface for our existing tool implementations
type Tool interface {
	Execute(ctx context.Context, input json.RawMessage) (interface{}, error)
}

// SpectreServer wraps mcp-go server with Spectre-specific logic
type SpectreServer struct {
	mcpServer       *server.MCPServer
	timelineService *apptimeline.Service
	graphService    *appgraph.Service
	tools           map[string]Tool
	version         string
}

// ServerOptions configures the Spectre MCP server
type ServerOptions struct {
	Version         string
	TimelineService *apptimeline.Service // Required: Direct service for tools
	GraphService    *appgraph.Service    // Required: Direct graph service for tools
}

// NewSpectreServerWithOptions creates a new Spectre MCP server with services
func NewSpectreServerWithOptions(opts ServerOptions) (*SpectreServer, error) {
	// Validate required services
	if opts.TimelineService == nil {
		return nil, fmt.Errorf("TimelineService is required")
	}
	if opts.GraphService == nil {
		return nil, fmt.Errorf("GraphService is required")
	}

	// Create mcp-go server with capabilities
	mcpServer := server.NewMCPServer(
		"Spectre MCP Server",
		opts.Version,
		server.WithToolCapabilities(false), // No tool subscription for now
		server.WithLogging(),               // Enable logging capability
	)

	s := &SpectreServer{
		mcpServer:       mcpServer,
		timelineService: opts.TimelineService,
		graphService:    opts.GraphService,
		tools:           make(map[string]Tool),
		version:         opts.Version,
	}

	// Register tools
	s.registerTools()

	// Register prompts
	s.registerPrompts()

	return s, nil
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
func (r *MCPToolRegistry) RegisterTool(name string, description string, handler integration.ToolHandler, inputSchema map[string]interface{}) error {
	// Validation
	if name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}

	// Use provided schema or fall back to empty object schema
	if inputSchema == nil {
		inputSchema = map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}
	schemaJSON, err := json.Marshal(inputSchema)
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	// Create MCP tool with provided schema
	mcpTool := mcp.NewToolWithRawSchema(name, description, schemaJSON)

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
