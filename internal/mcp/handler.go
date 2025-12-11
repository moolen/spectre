package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

// Handler is a transport-agnostic MCP protocol handler
type Handler struct {
	server       *MCPServer
	sessionState *SessionState
	version      string
}

// SessionState tracks MCP session state
type SessionState struct {
	mu              sync.RWMutex
	initialized     bool
	clientInfo      map[string]interface{}
	protocolVersion string
	lastActivity    time.Time
	loggingLevel    string
}

// NewHandler creates a new MCP protocol handler
func NewHandler(server *MCPServer, version string) *Handler {
	return &Handler{
		server: server,
		sessionState: &SessionState{
			protocolVersion: "2024-11-05",
			loggingLevel:    "info",
			lastActivity:    time.Now(),
		},
		version: version,
	}
}

// HandleRequest processes an MCP request and returns a response
func (h *Handler) HandleRequest(ctx context.Context, req *MCPRequest) *MCPResponse {
	if req.JSONRPC != "2.0" {
		return h.errorResponse(req.ID, -32600, "Invalid Request: jsonrpc must be 2.0")
	}

	if req.Method == "" {
		return h.errorResponse(req.ID, -32600, "Invalid Request: method is required")
	}

	// Update last activity
	h.sessionState.mu.Lock()
	h.sessionState.lastActivity = time.Now()
	h.sessionState.mu.Unlock()

	// Route to appropriate handler
	var result interface{}
	var err *MCPError

	switch req.Method {
	case "ping":
		result = h.handlePing(req.Params)

	case "initialize":
		result, err = h.handleInitialize(req.Params)

	case "tools/list":
		if !h.isInitialized() {
			err = &MCPError{Code: -32600, Message: "Server not initialized"}
		} else {
			result = h.handleToolsList(req.Params)
		}

	case "tools/call":
		if !h.isInitialized() {
			err = &MCPError{Code: -32600, Message: "Server not initialized"}
		} else {
			result, err = h.handleToolCall(ctx, req.Params)
		}

	case "prompts/list":
		if !h.isInitialized() {
			err = &MCPError{Code: -32600, Message: "Server not initialized"}
		} else {
			result = h.handlePromptsList(req.Params)
		}

	case "prompts/get":
		if !h.isInitialized() {
			err = &MCPError{Code: -32600, Message: "Server not initialized"}
		} else {
			result, err = h.handlePromptsGet(req.Params)
		}

	case "logging/setLevel":
		result, err = h.handleLoggingSetLevel(req.Params)

	default:
		err = &MCPError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)}
	}

	if err != nil {
		return h.errorResponse(req.ID, err.Code, err.Message)
	}

	return h.successResponse(req.ID, result)
}

func (h *Handler) handlePing(params map[string]interface{}) interface{} {
	return map[string]interface{}{}
}

func (h *Handler) handleInitialize(params map[string]interface{}) (interface{}, *MCPError) {
	if params == nil {
		return nil, &MCPError{Code: -32600, Message: "params is required"}
	}

	h.sessionState.mu.Lock()
	defer h.sessionState.mu.Unlock()

	if clientInfo, ok := params["clientInfo"].(map[string]interface{}); ok {
		h.sessionState.clientInfo = clientInfo
		logger := logging.GetLogger("mcp")
		logger.Info("Client connected: %v", clientInfo)
	}

	if protocolVersion, ok := params["protocolVersion"].(string); ok {
		h.sessionState.protocolVersion = protocolVersion
	}

	h.sessionState.initialized = true

	return map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools":   map[string]interface{}{},
			"prompts": map[string]interface{}{},
			"logging": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "Spectre MCP Server",
			"version": h.version,
		},
	}, nil
}

func (h *Handler) handleToolsList(_ map[string]interface{}) interface{} {
	return map[string]interface{}{
		"tools": h.server.GetTools(),
	}
}

func (h *Handler) handleToolCall(ctx context.Context, params map[string]interface{}) (interface{}, *MCPError) {
	if params == nil {
		return nil, &MCPError{Code: -32600, Message: "params is required"}
	}

	toolName, ok := params["name"].(string)
	if !ok {
		return nil, &MCPError{Code: -32600, Message: "params.name is required"}
	}

	toolArgs, _ := json.Marshal(params["arguments"])

	result, err := h.server.ExecuteTool(ctx, toolName, toolArgs)
	if err != nil {
		return map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": fmt.Sprintf("Error: %v", err),
				},
			},
			"isError": true,
		}, nil
	}

	return map[string]interface{}{
		"content": []map[string]interface{}{
			{
				"type": "text",
				"text": h.formatResult(result),
			},
		},
		"isError": false,
	}, nil
}

func (h *Handler) handlePromptsList(_ map[string]interface{}) interface{} {
	return map[string]interface{}{
		"prompts": GetPrompts(),
	}
}

func (h *Handler) handlePromptsGet(params map[string]interface{}) (interface{}, *MCPError) {
	if params == nil {
		return nil, &MCPError{Code: -32600, Message: "params is required"}
	}

	promptName, ok := params["name"].(string)
	if !ok {
		return nil, &MCPError{Code: -32600, Message: "params.name is required"}
	}

	promptDef := GetPromptByName(promptName)
	if promptDef == nil {
		return nil, &MCPError{Code: -32602, Message: fmt.Sprintf("unknown prompt: %s", promptName)}
	}

	args := make(map[string]interface{})
	if arguments, ok := params["arguments"].(map[string]interface{}); ok {
		args = arguments
	}

	result := BuildPromptMessages(promptName, args)
	return result, nil
}

func (h *Handler) handleLoggingSetLevel(params map[string]interface{}) (interface{}, *MCPError) {
	if params == nil {
		return nil, &MCPError{Code: -32600, Message: "params is required"}
	}

	level, ok := params["level"].(string)
	if !ok {
		return nil, &MCPError{Code: -32600, Message: "params.level is required and must be a string"}
	}

	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}

	if !validLevels[level] {
		return nil, &MCPError{Code: -32600, Message: fmt.Sprintf("invalid logging level: %s (must be debug, info, warn, or error)", level)}
	}

	return map[string]interface{}{}, nil
}

func (h *Handler) isInitialized() bool {
	h.sessionState.mu.RLock()
	defer h.sessionState.mu.RUnlock()
	return h.sessionState.initialized
}

func (h *Handler) successResponse(id, result interface{}) *MCPResponse {
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

func (h *Handler) errorResponse(id interface{}, code int, message string) *MCPResponse {
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
		},
	}
}

func (h *Handler) formatResult(result interface{}) string {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting result: %v", err)
	}
	return string(data)
}

// GetPrompts returns all available prompts
func GetPrompts() []PromptDefinition {
	return []PromptDefinition{
		{
			Name:        "post_mortem_incident_analysis",
			Description: "Conduct a comprehensive post-mortem analysis of a past incident",
			Arguments: []PromptArgument{
				{Name: "start_time", Description: "Start of the incident time window (Unix timestamp)", Required: true},
				{Name: "end_time", Description: "End of the incident time window (Unix timestamp)", Required: true},
				{Name: "namespace", Description: "Optional Kubernetes namespace", Required: false},
				{Name: "incident_description", Description: "Optional brief description", Required: false},
			},
		},
		{
			Name:        "live_incident_handling",
			Description: "Triage and investigate an ongoing incident",
			Arguments: []PromptArgument{
				{Name: "incident_start_time", Description: "When symptoms first appeared (Unix timestamp)", Required: true},
				{Name: "current_time", Description: "Optional current time", Required: false},
				{Name: "namespace", Description: "Optional Kubernetes namespace", Required: false},
				{Name: "symptoms", Description: "Optional brief description of symptoms", Required: false},
			},
		},
	}
}

// GetPromptByName retrieves a prompt definition by name
func GetPromptByName(name string) *PromptDefinition {
	prompts := GetPrompts()
	for _, p := range prompts {
		if p.Name == name {
			return &p
		}
	}
	return nil
}

// BuildPromptMessages builds the message structure for a prompt
func BuildPromptMessages(promptName string, args map[string]interface{}) *GetPromptResult {
	// Simplified version - full implementation should be expanded
	return &GetPromptResult{
		Description: fmt.Sprintf("Prompt: %s", promptName),
		Messages: []PromptMessage{
			{
				Role: "user",
				Content: PromptContent{
					Type: "text",
					Text: fmt.Sprintf("Execute prompt %s with args: %v", promptName, args),
				},
			},
		},
	}
}
