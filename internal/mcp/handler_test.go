package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

func TestHandler_JSONRPCCompliance(t *testing.T) {
	server := &MCPServer{}
	handler := NewHandler(server, "test-version")

	tests := []struct {
		name         string
		request      *MCPRequest
		expectedCode int
		expectedMsg  string
	}{
		{
			name: "invalid_jsonrpc_version",
			request: &MCPRequest{
				JSONRPC: "1.0",
				Method:  "ping",
				ID:      1,
			},
			expectedCode: -32600,
			expectedMsg:  "Invalid Request: jsonrpc must be 2.0",
		},
		{
			name: "missing_method",
			request: &MCPRequest{
				JSONRPC: "2.0",
				Method:  "",
				ID:      1,
			},
			expectedCode: -32600,
			expectedMsg:  "Invalid Request: method is required",
		},
		{
			name: "method_not_found",
			request: &MCPRequest{
				JSONRPC: "2.0",
				Method:  "nonexistent/method",
				ID:      1,
			},
			expectedCode: -32601,
			expectedMsg:  "Method not found: nonexistent/method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := handler.HandleRequest(context.Background(), tt.request)

			if resp.Error == nil {
				t.Fatal("Expected error response, got success")
			}

			if resp.Error.Code != tt.expectedCode {
				t.Errorf("Expected error code %d, got %d", tt.expectedCode, resp.Error.Code)
			}

			if resp.Error.Message != tt.expectedMsg {
				t.Errorf("Expected error message '%s', got '%s'", tt.expectedMsg, resp.Error.Message)
			}

			// Error response should have null result
			if resp.Result != nil {
				t.Error("Expected null result for error response")
			}
		})
	}
}

func TestHandler_Ping(t *testing.T) {
	server := &MCPServer{}
	handler := NewHandler(server, "test-version")

	req := &MCPRequest{
		JSONRPC: "2.0",
		Method:  "ping",
		ID:      1,
	}

	resp := handler.HandleRequest(context.Background(), req)

	if resp.Error != nil {
		t.Fatalf("Expected success, got error: %v", resp.Error)
	}

	if resp.Result == nil {
		t.Fatal("Expected non-nil result")
	}
}

func TestHandler_Initialize(t *testing.T) {
	server := &MCPServer{}
	handler := NewHandler(server, "1.0.0")

	req := &MCPRequest{
		JSONRPC: "2.0",
		Method:  "initialize",
		ID:      1,
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	resp := handler.HandleRequest(context.Background(), req)

	if resp.Error != nil {
		t.Fatalf("Expected success, got error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	// Verify protocol version
	if protocolVersion, ok := result["protocolVersion"].(string); !ok || protocolVersion != "2024-11-05" {
		t.Errorf("Expected protocolVersion '2024-11-05', got %v", result["protocolVersion"])
	}

	// Verify capabilities
	capabilities, ok := result["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected capabilities to be a map")
	}

	if _, ok := capabilities["tools"]; !ok {
		t.Error("Expected tools capability")
	}

	if _, ok := capabilities["prompts"]; !ok {
		t.Error("Expected prompts capability")
	}

	// Verify server info
	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected serverInfo to be a map")
	}

	if name, ok := serverInfo["name"].(string); !ok || name != "Spectre MCP Server" {
		t.Errorf("Expected serverInfo.name 'Spectre MCP Server', got %v", serverInfo["name"])
	}

	if version, ok := serverInfo["version"].(string); !ok || version != "1.0.0" {
		t.Errorf("Expected serverInfo.version '1.0.0', got %v", serverInfo["version"])
	}

	// Verify session is initialized
	if !handler.isInitialized() {
		t.Error("Expected session to be initialized")
	}
}

func TestHandler_InitializeWithoutParams(t *testing.T) {
	server := &MCPServer{}
	handler := NewHandler(server, "test-version")

	req := &MCPRequest{
		JSONRPC: "2.0",
		Method:  "initialize",
		ID:      1,
		Params:  nil,
	}

	resp := handler.HandleRequest(context.Background(), req)

	if resp.Error == nil {
		t.Fatal("Expected error for initialize without params")
	}

	if resp.Error.Code != -32600 {
		t.Errorf("Expected error code -32600, got %d", resp.Error.Code)
	}
}

func TestHandler_ToolsListBeforeInitialize(t *testing.T) {
	server := &MCPServer{}
	handler := NewHandler(server, "test-version")

	req := &MCPRequest{
		JSONRPC: "2.0",
		Method:  "tools/list",
		ID:      1,
	}

	resp := handler.HandleRequest(context.Background(), req)

	if resp.Error == nil {
		t.Fatal("Expected error when calling tools/list before initialize")
	}

	if resp.Error.Message != "Server not initialized" {
		t.Errorf("Expected 'Server not initialized' error, got: %s", resp.Error.Message)
	}
}

func TestHandler_ToolsListAfterInitialize(t *testing.T) {
	server := &MCPServer{}
	handler := NewHandler(server, "test-version")

	// Initialize first
	initReq := &MCPRequest{
		JSONRPC: "2.0",
		Method:  "initialize",
		ID:      1,
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
		},
	}
	handler.HandleRequest(context.Background(), initReq)

	// Now call tools/list
	req := &MCPRequest{
		JSONRPC: "2.0",
		Method:  "tools/list",
		ID:      2,
	}

	resp := handler.HandleRequest(context.Background(), req)

	if resp.Error != nil {
		t.Fatalf("Expected success, got error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	// tools can be either []ToolDefinition or []interface{}
	toolsRaw, ok := result["tools"]
	if !ok {
		t.Fatal("Expected 'tools' field in result")
	}

	// Check if we got 4 tools
	var toolCount int
	switch tools := toolsRaw.(type) {
	case []ToolDefinition:
		toolCount = len(tools)
	case []interface{}:
		toolCount = len(tools)
	default:
		t.Fatalf("Expected tools to be an array, got %T", toolsRaw)
	}

	// Should have 4 tools: cluster_health, resource_changes, investigate, resource_explorer
	if toolCount != 4 {
		t.Errorf("Expected 4 tools, got %d", toolCount)
	}
}

func TestHandler_LoggingSetLevel(t *testing.T) {
	server := &MCPServer{}
	handler := NewHandler(server, "test-version")

	tests := []struct {
		name        string
		level       string
		expectError bool
	}{
		{"debug_level", "debug", false},
		{"info_level", "info", false},
		{"warn_level", "warn", false},
		{"error_level", "error", false},
		{"invalid_level", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &MCPRequest{
				JSONRPC: "2.0",
				Method:  "logging/setLevel",
				ID:      1,
				Params: map[string]interface{}{
					"level": tt.level,
				},
			}

			resp := handler.HandleRequest(context.Background(), req)

			if tt.expectError {
				if resp.Error == nil {
					t.Error("Expected error for invalid log level")
				}
			} else {
				if resp.Error != nil {
					t.Errorf("Expected success, got error: %v", resp.Error)
				}
			}
		})
	}
}

func TestHandler_SessionState(t *testing.T) {
	server := &MCPServer{}
	handler := NewHandler(server, "test-version")

	// Initially not initialized
	if handler.isInitialized() {
		t.Error("Expected session to not be initialized initially")
	}

	// Initialize
	req := &MCPRequest{
		JSONRPC: "2.0",
		Method:  "initialize",
		ID:      1,
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"clientInfo": map[string]interface{}{
				"name": "test-client",
			},
		},
	}

	handler.HandleRequest(context.Background(), req)

	// Now should be initialized
	if !handler.isInitialized() {
		t.Error("Expected session to be initialized after initialize call")
	}

	// Verify client info is stored
	handler.sessionState.mu.RLock()
	clientInfo := handler.sessionState.clientInfo
	handler.sessionState.mu.RUnlock()

	if clientInfo == nil {
		t.Fatal("Expected client info to be stored")
	}

	if name, ok := clientInfo["name"].(string); !ok || name != "test-client" {
		t.Errorf("Expected client name 'test-client', got %v", clientInfo["name"])
	}
}

func TestHandler_RequestIDPreserved(t *testing.T) {
	server := &MCPServer{}
	handler := NewHandler(server, "test-version")

	tests := []interface{}{
		1,
		"string-id",
		json.Number("123"),
	}

	for _, id := range tests {
		req := &MCPRequest{
			JSONRPC: "2.0",
			Method:  "ping",
			ID:      id,
		}

		resp := handler.HandleRequest(context.Background(), req)

		if resp.ID != id {
			t.Errorf("Expected response ID to match request ID %v, got %v", id, resp.ID)
		}
	}
}

func TestHandler_PromptsListBeforeInitialize(t *testing.T) {
	server := &MCPServer{}
	handler := NewHandler(server, "test-version")

	req := &MCPRequest{
		JSONRPC: "2.0",
		Method:  "prompts/list",
		ID:      1,
	}

	resp := handler.HandleRequest(context.Background(), req)

	if resp.Error == nil {
		t.Fatal("Expected error when calling prompts/list before initialize")
	}

	if resp.Error.Message != "Server not initialized" {
		t.Errorf("Expected 'Server not initialized' error, got: %s", resp.Error.Message)
	}
}

func TestHandler_PromptsListAfterInitialize(t *testing.T) {
	server := &MCPServer{}
	handler := NewHandler(server, "test-version")

	// Initialize first
	initReq := &MCPRequest{
		JSONRPC: "2.0",
		Method:  "initialize",
		ID:      1,
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
		},
	}
	handler.HandleRequest(context.Background(), initReq)

	// Now call prompts/list
	req := &MCPRequest{
		JSONRPC: "2.0",
		Method:  "prompts/list",
		ID:      2,
	}

	resp := handler.HandleRequest(context.Background(), req)

	if resp.Error != nil {
		t.Fatalf("Expected success, got error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}

	// prompts can be either []PromptDefinition or []interface{}
	promptsRaw, ok := result["prompts"]
	if !ok {
		t.Fatal("Expected 'prompts' field in result")
	}

	// Check if we got any prompts
	var promptCount int
	switch p := promptsRaw.(type) {
	case []PromptDefinition:
		promptCount = len(p)
		if promptCount == 0 {
			t.Error("Expected at least one prompt")
		}
	case []interface{}:
		promptCount = len(p)
		if promptCount == 0 {
			t.Error("Expected at least one prompt")
		}
	default:
		t.Fatalf("Expected prompts to be an array, got %T", promptsRaw)
	}
}

func TestHandler_ConcurrentRequests(t *testing.T) {
	server := &MCPServer{}
	handler := NewHandler(server, "test-version")

	// Initialize first
	initReq := &MCPRequest{
		JSONRPC: "2.0",
		Method:  "initialize",
		ID:      0,
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
		},
	}
	handler.HandleRequest(context.Background(), initReq)

	// Send multiple concurrent ping requests
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			req := &MCPRequest{
				JSONRPC: "2.0",
				Method:  "ping",
				ID:      id,
			}
			resp := handler.HandleRequest(context.Background(), req)
			if resp.Error != nil {
				t.Errorf("Request %d failed: %v", id, resp.Error)
			}
			done <- true
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestHandler_LastActivityTracking(t *testing.T) {
	server := &MCPServer{}
	handler := NewHandler(server, "test-version")

	// Get initial last activity time
	handler.sessionState.mu.RLock()
	initialTime := handler.sessionState.lastActivity
	handler.sessionState.mu.RUnlock()

	// Make a request
	req := &MCPRequest{
		JSONRPC: "2.0",
		Method:  "ping",
		ID:      1,
	}
	handler.HandleRequest(context.Background(), req)

	// Verify last activity was updated
	handler.sessionState.mu.RLock()
	updatedTime := handler.sessionState.lastActivity
	handler.sessionState.mu.RUnlock()

	if !updatedTime.After(initialTime) {
		t.Error("Expected last activity to be updated after request")
	}
}
