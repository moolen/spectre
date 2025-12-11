// Package helpers provides MCP client utilities for e2e testing.
package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

// MCPClient is an HTTP client for testing the MCP server.
type MCPClient struct {
	BaseURL string
	Client  *http.Client
	t       *testing.T
}

// MCPRequest represents a JSON-RPC 2.0 request.
type MCPRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id,omitempty"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// MCPResponse represents a JSON-RPC 2.0 response.
type MCPResponse struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id,omitempty"`
	Result  map[string]interface{} `json:"result,omitempty"`
	Error   *MCPError              `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC 2.0 error.
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ToolDefinition represents an MCP tool definition.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// PromptDefinition represents an MCP prompt definition.
type PromptDefinition struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument represents an argument for a prompt.
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// NewMCPClient creates a new MCP client for testing.
func NewMCPClient(t *testing.T, baseURL string) *MCPClient {
	t.Logf("Creating MCP client for: %s", baseURL)

	return &MCPClient{
		BaseURL: baseURL,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
		t: t,
	}
}

// sendRequest sends a JSON-RPC request to the MCP server.
func (m *MCPClient) sendRequest(ctx context.Context, method string, params map[string]interface{}) (*MCPResponse, error) {
	reqID := time.Now().UnixNano()

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      reqID,
		Method:  method,
		Params:  params,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", m.BaseURL+"/mcp", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := m.Client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	var mcpResp MCPResponse
	if err := json.NewDecoder(resp.Body).Decode(&mcpResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if mcpResp.Error != nil {
		return &mcpResp, fmt.Errorf("MCP error %d: %s", mcpResp.Error.Code, mcpResp.Error.Message)
	}

	return &mcpResp, nil
}

// Ping sends a ping request to the MCP server.
func (m *MCPClient) Ping(ctx context.Context) error {
	resp, err := m.sendRequest(ctx, "ping", nil)
	if err != nil {
		return err
	}

	if resp.Result == nil {
		return fmt.Errorf("expected result in ping response")
	}

	return nil
}

// Initialize sends an initialize request to the MCP server.
func (m *MCPClient) Initialize(ctx context.Context) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"clientInfo": map[string]interface{}{
			"name":    "spectre-test-client",
			"version": "1.0.0",
		},
	}

	resp, err := m.sendRequest(ctx, "initialize", params)
	if err != nil {
		return nil, err
	}

	return resp.Result, nil
}

// ListTools requests the list of available tools.
func (m *MCPClient) ListTools(ctx context.Context) ([]ToolDefinition, error) {
	resp, err := m.sendRequest(ctx, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	toolsData, ok := resp.Result["tools"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected tools format in response")
	}

	// Convert to JSON and back to get proper types
	toolsJSON, err := json.Marshal(toolsData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tools: %w", err)
	}

	var tools []ToolDefinition
	if err := json.Unmarshal(toolsJSON, &tools); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools: %w", err)
	}

	return tools, nil
}

// CallTool calls a tool with the given name and arguments.
func (m *MCPClient) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"name":      toolName,
		"arguments": args,
	}

	resp, err := m.sendRequest(ctx, "tools/call", params)
	if err != nil {
		return nil, err
	}

	return resp.Result, nil
}

// ListPrompts requests the list of available prompts.
func (m *MCPClient) ListPrompts(ctx context.Context) ([]PromptDefinition, error) {
	resp, err := m.sendRequest(ctx, "prompts/list", nil)
	if err != nil {
		return nil, err
	}

	promptsData, ok := resp.Result["prompts"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected prompts format in response")
	}

	// Convert to JSON and back to get proper types
	promptsJSON, err := json.Marshal(promptsData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal prompts: %w", err)
	}

	var prompts []PromptDefinition
	if err := json.Unmarshal(promptsJSON, &prompts); err != nil {
		return nil, fmt.Errorf("failed to unmarshal prompts: %w", err)
	}

	return prompts, nil
}

// GetPrompt gets a prompt by name with the given arguments.
func (m *MCPClient) GetPrompt(ctx context.Context, promptName string, args map[string]interface{}) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"name":      promptName,
		"arguments": args,
	}

	resp, err := m.sendRequest(ctx, "prompts/get", params)
	if err != nil {
		return nil, err
	}

	return resp.Result, nil
}

// SetLoggingLevel sets the logging level.
func (m *MCPClient) SetLoggingLevel(ctx context.Context, level string) error {
	params := map[string]interface{}{
		"level": level,
	}

	_, err := m.sendRequest(ctx, "logging/setLevel", params)
	return err
}

// Health checks if the MCP server is healthy.
func (m *MCPClient) Health(ctx context.Context) error {
	url := m.BaseURL + "/health"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := m.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute health check: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	return nil
}
