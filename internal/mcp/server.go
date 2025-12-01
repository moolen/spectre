package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/mcp/tools"
)

// MCPServer represents the MCP server instance
type MCPServer struct {
	spectreClient *SpectreClient
	tools         map[string]Tool
}

// Tool defines the interface for MCP tools
type Tool interface {
	Execute(ctx context.Context, input json.RawMessage) (interface{}, error)
}

// ToolDefinition represents the metadata for a tool
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// NewMCPServer creates a new MCP server
func NewMCPServer(spectreURL string) (*MCPServer, error) {
	// Test connection to Spectre
	client := NewSpectreClient(spectreURL)
	if err := client.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to Spectre API: %w", err)
	}

	server := &MCPServer{
		spectreClient: client,
		tools:         make(map[string]Tool),
	}

	// Register tools
	server.registerTools()

	return server, nil
}

func (s *MCPServer) registerTools() {
	s.tools["cluster_health"] = tools.NewClusterHealthTool(s.spectreClient)
	s.tools["resource_changes"] = tools.NewResourceChangesTool(s.spectreClient)
	s.tools["investigate"] = tools.NewInvestigateTool(s.spectreClient)
	s.tools["resource_explorer"] = tools.NewResourceExplorerTool(s.spectreClient)
}

// GetTools returns all registered tools
func (s *MCPServer) GetTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "cluster_health",
			Description: "Get cluster health overview with resource status breakdown and top issues",
			InputSchema: map[string]interface{}{
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
				},
				"required": []string{"start_time", "end_time"},
			},
		},
		{
			Name:        "resource_changes",
			Description: "Get summarized resource changes with categorization and impact scoring for LLM analysis",
			InputSchema: map[string]interface{}{
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
				},
				"required": []string{"start_time", "end_time"},
			},
		},
		{
			Name:        "investigate",
			Description: "Get detailed investigation evidence with status timeline, events, and investigation prompts for RCA",
			InputSchema: map[string]interface{}{
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
				},
				"required": []string{"resource_kind", "start_time", "end_time"},
			},
		},
		{
			Name:        "resource_explorer",
			Description: "Browse and discover resources in the cluster with filtering and status overview",
			InputSchema: map[string]interface{}{
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
				},
				"required": []string{},
			},
		},
	}
}

// ExecuteTool runs a registered tool
func (s *MCPServer) ExecuteTool(ctx context.Context, toolName string, input json.RawMessage) (interface{}, error) {
	tool, exists := s.tools[toolName]
	if !exists {
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}

	return tool.Execute(ctx, input)
}
