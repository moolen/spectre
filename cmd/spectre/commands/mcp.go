package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/mcp"
	"github.com/spf13/cobra"
)

var (
	spectreURL string
	httpAddr   string
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the MCP server",
	Long: `Start the Model Context Protocol (MCP) server that exposes
Spectre functionality as MCP tools for AI assistants.`,
	Run: runMCP,
}

func init() {
	mcpCmd.Flags().StringVar(&spectreURL, "spectre-url", getEnv("SPECTRE_URL", "http://localhost:8080"), "URL to Spectre API server")
	mcpCmd.Flags().StringVar(&httpAddr, "http-addr", getEnv("MCP_HTTP_ADDR", ":8081"), "HTTP server address (host:port)")
}

func runMCP(cmd *cobra.Command, args []string) {
	// Set up logging
	logging.Initialize(logLevel)
	logger := logging.GetLogger("mcp")
	logger.Info("Starting Spectre MCP Server")
	logger.Info("Connecting to Spectre API at %s", spectreURL)

	// Create MCP server
	server, err := mcp.NewMCPServer(spectreURL)
	if err != nil {
		logger.Fatal("Failed to create MCP server: %v", err)
	}

	logger.Info("Successfully connected to Spectre API")

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         httpAddr,
		Handler:      newMCPHTTPHandler(server),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		logger.Info("Starting HTTP server on %s", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start HTTP server: %v", err)
		}
	}()

	// Wait for shutdown signal
	sig := <-sigCh
	logger.Info("Received signal: %v, shutting down gracefully...", sig)

	// Create a context with timeout for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown the HTTP server
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("Error during shutdown: %v", err)
	}

	logger.Info("Server stopped")
}

// getEnv returns environment variable value or default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// MCP protocol types

// MCPRequest represents a JSON-RPC 2.0 request
type MCPRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id,omitempty"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// MCPResponse represents a JSON-RPC 2.0 response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents a JSON-RPC 2.0 error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// PromptDefinition represents an MCP prompt
type PromptDefinition struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument represents an argument for a prompt
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// PromptMessage represents a message in a prompt
type PromptMessage struct {
	Role    string        `json:"role"`
	Content PromptContent `json:"content"`
}

// PromptContent represents the content of a prompt message
type PromptContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// GetPromptResult represents the result of getting a prompt
type GetPromptResult struct {
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
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

// newMCPHTTPHandler creates an HTTP handler for the MCP server
func newMCPHTTPHandler(mcpServer *mcp.MCPServer) http.Handler {
	mux := http.NewServeMux()
	sessionState := &SessionState{
		protocolVersion: "2024-11-05",
		loggingLevel:    "info",
		lastActivity:    time.Now(),
	}

	// Main MCP endpoint that handles all JSON-RPC requests
	mux.HandleFunc("POST /mcp", func(w http.ResponseWriter, r *http.Request) {
		handleMCPRequest(w, r, mcpServer, sessionState)
	})

	// Health check endpoint
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Root endpoint
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"name":    "Spectre MCP Server",
			"version": Version,
		})
	})

	return mux
}

// handleMCPRequest processes an MCP JSON-RPC request
func handleMCPRequest(w http.ResponseWriter, r *http.Request, mcpServer *mcp.MCPServer, sessionState *SessionState) {
	w.Header().Set("Content-Type", "application/json")

	var req MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendMCPError(w, nil, -32700, "Parse error")
		return
	}

	if req.JSONRPC != "2.0" {
		sendMCPError(w, req.ID, -32600, "Invalid Request: jsonrpc must be 2.0")
		return
	}

	if req.Method == "" {
		sendMCPError(w, req.ID, -32600, "Invalid Request: method is required")
		return
	}

	// Update last activity
	sessionState.mu.Lock()
	sessionState.lastActivity = time.Now()
	sessionState.mu.Unlock()

	// Route to appropriate handler
	var response interface{}
	var respErr *MCPError

	switch req.Method {
	case "ping":
		response = handlePing(req.Params)

	case "initialize":
		response, respErr = handleInitialize(req.Params, sessionState)

	case "tools/list":
		if !isInitialized(sessionState) {
			respErr = &MCPError{Code: -32600, Message: "Server not initialized"}
		} else {
			response = handleToolsList(mcpServer, req.Params)
		}

	case "tools/call":
		if !isInitialized(sessionState) {
			respErr = &MCPError{Code: -32600, Message: "Server not initialized"}
		} else {
			response, respErr = handleToolCall(r.Context(), mcpServer, req.Params)
		}

	case "prompts/list":
		if !isInitialized(sessionState) {
			respErr = &MCPError{Code: -32600, Message: "Server not initialized"}
		} else {
			response = handlePromptsList(req.Params)
		}

	case "prompts/get":
		if !isInitialized(sessionState) {
			respErr = &MCPError{Code: -32600, Message: "Server not initialized"}
		} else {
			response, respErr = handlePromptsGet(req.Params)
		}

	case "logging/setLevel":
		response, respErr = handleLoggingSetLevel(req.Params)

	default:
		respErr = &MCPError{Code: -32601, Message: fmt.Sprintf("Method not found: %s", req.Method)}
	}

	// Send response
	if respErr != nil {
		sendMCPError(w, req.ID, respErr.Code, respErr.Message)
	} else {
		sendMCPResponse(w, req.ID, response)
	}
}

// MCP protocol handlers

func handlePing(params map[string]interface{}) interface{} {
	return map[string]interface{}{}
}

func handleInitialize(params map[string]interface{}, sessionState *SessionState) (interface{}, *MCPError) {
	if params == nil {
		return nil, &MCPError{Code: -32600, Message: "params is required"}
	}

	sessionState.mu.Lock()
	defer sessionState.mu.Unlock()

	if clientInfo, ok := params["clientInfo"].(map[string]interface{}); ok {
		sessionState.clientInfo = clientInfo
		logger := logging.GetLogger("mcp")
		logger.Info("Client connected: %v", clientInfo)
	}

	if protocolVersion, ok := params["protocolVersion"].(string); ok {
		sessionState.protocolVersion = protocolVersion
	}

	sessionState.initialized = true

	return map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools":   map[string]interface{}{},
			"prompts": map[string]interface{}{},
			"logging": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "Spectre MCP Server",
			"version": Version,
		},
	}, nil
}

func handleToolsList(mcpServer *mcp.MCPServer, _ map[string]interface{}) interface{} {
	return map[string]interface{}{
		"tools": mcpServer.GetTools(),
	}
}

func handleToolCall(ctx context.Context, mcpServer *mcp.MCPServer, params map[string]interface{}) (interface{}, *MCPError) {
	if params == nil {
		return nil, &MCPError{Code: -32600, Message: "params is required"}
	}

	toolName, ok := params["name"].(string)
	if !ok {
		return nil, &MCPError{Code: -32600, Message: "params.name is required"}
	}

	toolArgs, _ := json.Marshal(params["arguments"])

	result, err := mcpServer.ExecuteTool(ctx, toolName, toolArgs)
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
				"text": formatMCPResult(result),
			},
		},
		"isError": false,
	}, nil
}

func handlePromptsList(_ map[string]interface{}) interface{} {
	return map[string]interface{}{
		"prompts": getPrompts(),
	}
}

func handlePromptsGet(params map[string]interface{}) (interface{}, *MCPError) {
	if params == nil {
		return nil, &MCPError{Code: -32600, Message: "params is required"}
	}

	promptName, ok := params["name"].(string)
	if !ok {
		return nil, &MCPError{Code: -32600, Message: "params.name is required"}
	}

	promptDef := getPromptByName(promptName)
	if promptDef == nil {
		return nil, &MCPError{Code: -32602, Message: fmt.Sprintf("unknown prompt: %s", promptName)}
	}

	args := make(map[string]interface{})
	if arguments, ok := params["arguments"].(map[string]interface{}); ok {
		args = arguments
	}

	result := buildPromptMessages(promptName, args)
	return result, nil
}

func handleLoggingSetLevel(params map[string]interface{}) (interface{}, *MCPError) {
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

func isInitialized(sessionState *SessionState) bool {
	sessionState.mu.RLock()
	defer sessionState.mu.RUnlock()
	return sessionState.initialized
}

func sendMCPResponse(w http.ResponseWriter, id, result interface{}) {
	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	_ = json.NewEncoder(w).Encode(response)
}

func sendMCPError(w http.ResponseWriter, id interface{}, code int, message string) {
	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
		},
	}
	_ = json.NewEncoder(w).Encode(response)
}

func formatMCPResult(result interface{}) string {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting result: %v", err)
	}
	return string(data)
}

// Note: getPrompts, getPromptByName, and buildPromptMessages functions
// are imported from the original mcp-server/main.go
// For brevity, these large functions should be moved to internal/mcp package
// Here we reference them from the original implementation

func getPrompts() []PromptDefinition {
	// TODO: Move to internal/mcp package
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

func getPromptByName(name string) *PromptDefinition {
	prompts := getPrompts()
	for _, p := range prompts {
		if p.Name == name {
			return &p
		}
	}
	return nil
}

func buildPromptMessages(promptName string, args map[string]interface{}) *GetPromptResult {
	// Simplified version - full implementation should be in internal/mcp
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
