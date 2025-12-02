package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/moolen/spectre/internal/mcp"
)

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
	Name        string                   `json:"name"`
	Description string                   `json:"description,omitempty"`
	Arguments   []PromptArgument         `json:"arguments,omitempty"`
}

// PromptArgument represents an argument for a prompt
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// PromptMessage represents a message in a prompt
type PromptMessage struct {
	Role    string        `json:"role"`    // "user" or "assistant"
	Content PromptContent `json:"content"` // Content with type and text
}

// PromptContent represents the content of a prompt message
type PromptContent struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"` // The actual text content
}

// GetPromptResult represents the result of getting a prompt
type GetPromptResult struct {
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
}

// SessionState tracks MCP session state
type SessionState struct {
	mu                sync.RWMutex
	initialized       bool
	clientInfo        map[string]interface{}
	protocolVersion   string
	lastActivity      time.Time
	loggingLevel      string
}

func main() {
	cfg := LoadConfig()

	// Set up logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("Starting Spectre MCP Server")
	log.Printf("Connecting to Spectre API at %s", cfg.SpectreURL)

	// Create MCP server
	server, err := mcp.NewMCPServer(cfg.SpectreURL)
	if err != nil {
		log.Fatalf("Failed to create MCP server: %v", err)
	}

	log.Println("Successfully connected to Spectre API")

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      newHTTPHandler(server),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		log.Printf("Starting HTTP server on %s", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}()

	// Wait for shutdown signal
	sig := <-sigCh
	log.Printf("Received signal: %v, shutting down gracefully...", sig)

	// Create a context with timeout for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown the HTTP server
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Server stopped")
}

// newHTTPHandler creates an HTTP handler for the MCP server
func newHTTPHandler(mcpServer *mcp.MCPServer) http.Handler {
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
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Root endpoint
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"name":    "Spectre MCP Server",
			"version": "1.0.0",
		})
	})

	return mux
}

// handleMCPRequest processes an MCP JSON-RPC request
func handleMCPRequest(w http.ResponseWriter, r *http.Request, mcpServer *mcp.MCPServer, sessionState *SessionState) {
	w.Header().Set("Content-Type", "application/json")

	var req MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, nil, -32700, "Parse error")
		return
	}

	if req.JSONRPC != "2.0" {
		sendError(w, req.ID, -32600, "Invalid Request: jsonrpc must be 2.0")
		return
	}

	if req.Method == "" {
		sendError(w, req.ID, -32600, "Invalid Request: method is required")
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
		sendError(w, req.ID, respErr.Code, respErr.Message)
	} else {
		sendResponse(w, req.ID, response)
	}
}

// handlePing handles the ping method
func handlePing(params map[string]interface{}) interface{} {
	return map[string]interface{}{}
}

// handleInitialize handles the initialize method
func handleInitialize(params map[string]interface{}, sessionState *SessionState) (interface{}, *MCPError) {
	if params == nil {
		return nil, &MCPError{Code: -32600, Message: "params is required"}
	}

	// Store client info
	sessionState.mu.Lock()
	defer sessionState.mu.Unlock()

	if clientInfo, ok := params["clientInfo"].(map[string]interface{}); ok {
		sessionState.clientInfo = clientInfo
		log.Printf("Client connected: %v", clientInfo)
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
			"version": "1.0.0",
		},
	}, nil
}

// handleToolsList handles the tools/list method
func handleToolsList(mcpServer *mcp.MCPServer, params map[string]interface{}) interface{} {
	return map[string]interface{}{
		"tools": mcpServer.GetTools(),
	}
}

// handlePromptsList handles the prompts/list method
func handlePromptsList(params map[string]interface{}) interface{} {
	return map[string]interface{}{
		"prompts": getPrompts(),
	}
}

// handlePromptsGet handles the prompts/get method
func handlePromptsGet(params map[string]interface{}) (interface{}, *MCPError) {
	if params == nil {
		return nil, &MCPError{Code: -32600, Message: "params is required"}
	}

	promptName, ok := params["name"].(string)
	if !ok {
		return nil, &MCPError{Code: -32600, Message: "params.name is required"}
	}

	// Check if prompt exists
	promptDef := getPromptByName(promptName)
	if promptDef == nil {
		return nil, &MCPError{Code: -32602, Message: fmt.Sprintf("unknown prompt: %s", promptName)}
	}

	// Get arguments if provided
	args := make(map[string]interface{})
	if arguments, ok := params["arguments"].(map[string]interface{}); ok {
		args = arguments
	}

	// Build the prompt messages
	result, err := buildPromptMessages(promptName, args)
	if err != nil {
		return nil, &MCPError{Code: -32603, Message: fmt.Sprintf("failed to build prompt: %v", err)}
	}

	return result, nil
}

// handleToolCall handles the tools/call method
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
				"text": formatResult(result),
			},
		},
		"isError": false,
	}, nil
}

// isInitialized checks if the session is initialized
func isInitialized(sessionState *SessionState) bool {
	sessionState.mu.RLock()
	defer sessionState.mu.RUnlock()
	return sessionState.initialized
}

// handleLoggingSetLevel handles the logging/setLevel method
func handleLoggingSetLevel(params map[string]interface{}) (interface{}, *MCPError) {
	if params == nil {
		return nil, &MCPError{Code: -32600, Message: "params is required"}
	}

	level, ok := params["level"].(string)
	if !ok {
		return nil, &MCPError{Code: -32600, Message: "params.level is required and must be a string"}
	}

	// Validate logging level
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}

	if !validLevels[level] {
		return nil, &MCPError{Code: -32600, Message: fmt.Sprintf("invalid logging level: %s (must be debug, info, warn, or error)", level)}
	}

	log.Printf("Logging level set to: %s", level)

	return map[string]interface{}{}, nil
}

// sendResponse sends a JSON-RPC 2.0 success response
func sendResponse(w http.ResponseWriter, id interface{}, result interface{}) {
	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	json.NewEncoder(w).Encode(response)
}

// sendError sends a JSON-RPC 2.0 error response
func sendError(w http.ResponseWriter, id interface{}, code int, message string) {
	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
		},
	}
	json.NewEncoder(w).Encode(response)
}

// formatResult formats the tool result as a human-readable string
func formatResult(result interface{}) string {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting result: %v", err)
	}
	return string(data)
}

// getPrompts returns all available prompts
func getPrompts() []PromptDefinition {
	return []PromptDefinition{
		{
			Name:        "post_mortem_incident_analysis",
			Description: "Conduct a comprehensive post-mortem analysis of a past incident by investigating cluster events, resource changes, and status transitions within a specified time window.",
			Arguments: []PromptArgument{
				{
					Name:        "start_time",
					Description: "Start of the incident time window (Unix timestamp in seconds or milliseconds)",
					Required:    true,
				},
				{
					Name:        "end_time",
					Description: "End of the incident time window (Unix timestamp in seconds or milliseconds)",
					Required:    true,
				},
				{
					Name:        "namespace",
					Description: "Optional Kubernetes namespace to narrow the investigation scope",
					Required:    false,
				},
				{
					Name:        "incident_description",
					Description: "Optional brief description of the incident symptoms or context",
					Required:    false,
				},
			},
		},
		{
			Name:        "live_incident_handling",
			Description: "Triage and investigate an ongoing incident by analyzing recent cluster state, identifying error signals, and recommending immediate mitigation steps.",
			Arguments: []PromptArgument{
				{
					Name:        "incident_start_time",
					Description: "When the incident symptoms first appeared (Unix timestamp in seconds or milliseconds)",
					Required:    true,
				},
				{
					Name:        "current_time",
					Description: "Optional current time (defaults to now if omitted)",
					Required:    false,
				},
				{
					Name:        "namespace",
					Description: "Optional Kubernetes namespace to focus the investigation",
					Required:    false,
				},
				{
					Name:        "symptoms",
					Description: "Optional brief description of observed symptoms (e.g., 'API timeouts', 'pod crashes')",
					Required:    false,
				},
			},
		},
	}
}

// getPromptByName retrieves a specific prompt by name
func getPromptByName(name string) *PromptDefinition {
	prompts := getPrompts()
	for _, p := range prompts {
		if p.Name == name {
			return &p
		}
	}
	return nil
}

// buildPromptMessages constructs the prompt messages with system instructions
func buildPromptMessages(promptName string, args map[string]interface{}) (*GetPromptResult, error) {
	switch promptName {
	case "post_mortem_incident_analysis":
		return buildPostMortemPrompt(args), nil
	case "live_incident_handling":
		return buildLiveIncidentPrompt(args), nil
	default:
		return nil, fmt.Errorf("unknown prompt: %s", promptName)
	}
}

// buildPostMortemPrompt builds the post-mortem incident analysis prompt
func buildPostMortemPrompt(args map[string]interface{}) *GetPromptResult {
	systemMsg := `You are conducting a post-mortem analysis of a Kubernetes cluster incident. Your goal is to build a comprehensive, chronological chain of events that explains what happened during the incident window.

CRITICAL RULES - You MUST follow these strictly:

1. NO HALLUCINATIONS: Only report events, timestamps, resources, and status changes that are explicitly present in tool responses or logs provided by the user. Never invent or guess information.

2. GROUND ALL STATEMENTS: Every claim about what happened must be traceable to specific tool output. When referencing events, include timestamps and resource names exactly as returned by the tools.

3. USE AVAILABLE TOOLS: You have access to these Spectre MCP tools:
   - cluster_health: Get overall cluster health and top issues for the time window
   - resource_changes: Identify resources that changed and their impact scores
   - investigate: Deep-dive into specific resources with full timeline and events
   - resource_explorer: Browse and discover resources by status

4. ACKNOWLEDGE GAPS: If information is missing or ambiguous, explicitly state what is unknown rather than guessing. Describe what additional evidence would help (logs, metrics, etc.).

5. RECOMMEND LOG COLLECTION: When you need more detail, explicitly ask the user to fetch logs using:
   - kubectl logs <pod-name> -n <namespace> --since-time=<timestamp>
   - kubectl describe <resource-kind> <resource-name> -n <namespace>
   - Cloud provider logging queries (e.g., CloudWatch, Stackdriver)
   - Other MCP log tools if available
   Explain how these logs will clarify the analysis.

WORKFLOW:
1. Confirm the time window (start_time, end_time) and optional namespace filter
2. Call cluster_health to get an overview of the incident window
3. Call resource_changes to identify high-impact changes
4. For critical resources, call investigate to get detailed timelines
5. Use resource_explorer if you need context on related resources
6. Build a chronological timeline of events with timestamps
7. Identify contributing factors and likely root causes
8. Document impact and suggest preventive measures
9. List follow-up actions and additional data needed

OUTPUT FORMAT:
- Timeline: Chronological list of key events with exact timestamps
- Root Cause Analysis: Evidence-based explanation of what caused the incident
- Impact: Which resources were affected and for how long
- Contributing Factors: Conditions that enabled or worsened the incident
- Recommendations: Preventive measures and follow-up actions
- Data Gaps: What information is missing and how to obtain it`

	userMsg := `Please conduct a post-mortem analysis for the following incident:

Time Window:
- Start: {{start_time}}
- End: {{end_time}}
{{#if namespace}}- Namespace: {{namespace}}{{/if}}
{{#if incident_description}}- Context: {{incident_description}}{{/if}}

Use the available Spectre MCP tools to investigate what happened. Build a comprehensive timeline and identify the root cause based only on actual tool results.`

	// Simple template replacement
	if startTime, ok := args["start_time"]; ok {
		userMsg = replaceTemplate(userMsg, "start_time", fmt.Sprintf("%v", startTime))
	}
	if endTime, ok := args["end_time"]; ok {
		userMsg = replaceTemplate(userMsg, "end_time", fmt.Sprintf("%v", endTime))
	}
	if namespace, ok := args["namespace"].(string); ok && namespace != "" {
		userMsg = replaceTemplate(userMsg, "#if namespace", "")
		userMsg = replaceTemplate(userMsg, "namespace", namespace)
		userMsg = replaceTemplate(userMsg, "/if", "")
	} else {
		userMsg = removeConditional(userMsg, "namespace")
	}
	if desc, ok := args["incident_description"].(string); ok && desc != "" {
		userMsg = replaceTemplate(userMsg, "#if incident_description", "")
		userMsg = replaceTemplate(userMsg, "incident_description", desc)
		userMsg = replaceTemplate(userMsg, "/if", "")
	} else {
		userMsg = removeConditional(userMsg, "incident_description")
	}

	return &GetPromptResult{
		Description: "Conduct a comprehensive post-mortem analysis of a past incident",
		Messages: []PromptMessage{
			{
				Role: "user",
				Content: PromptContent{
					Type: "text",
					Text: systemMsg,
				},
			},
			{
				Role: "user",
				Content: PromptContent{
					Type: "text",
					Text: userMsg,
				},
			},
		},
	}
}

// buildLiveIncidentPrompt builds the live incident handling prompt
func buildLiveIncidentPrompt(args map[string]interface{}) *GetPromptResult {
	systemMsg := `You are responding to a LIVE Kubernetes cluster incident. Your goal is to quickly triage the situation, identify the root cause, and recommend immediate mitigation steps.

CRITICAL RULES - You MUST follow these strictly:

1. NO HALLUCINATIONS: Only report events, timestamps, resources, and status changes that are explicitly present in tool responses or logs provided by the user. Never invent or guess information.

2. GROUND ALL STATEMENTS: Every claim must be traceable to specific tool output. When referencing events, include exact timestamps and resource names as returned by the tools.

3. USE AVAILABLE TOOLS: You have access to these Spectre MCP tools:
   - cluster_health: Get current cluster health and identify critical/warning resources
   - resource_changes: See what changed recently that may have triggered the incident
   - investigate: Deep-dive into suspect resources with full event history
   - resource_explorer: Browse resources by status to find related issues

4. ACKNOWLEDGE UNCERTAINTY: If data is incomplete or ambiguous, state what is unknown. Don't guess—describe what additional evidence is needed.

5. RECOMMEND LOG COLLECTION: When you need more detail, explicitly ask the user to fetch logs:
   - kubectl logs <pod-name> -n <namespace> --tail=100 --timestamps
   - kubectl describe <resource-kind> <resource-name> -n <namespace>
   - kubectl get events -n <namespace> --sort-by='.lastTimestamp'
   - Cloud provider logs (CloudWatch, Stackdriver, etc.)
   - Application-specific logs via other MCP tools
   Explain what you're looking for in these logs.

6. FOCUS ON RECENT DATA: Look at the time window from slightly before incident_start_time to now. Check for precursor events that may have triggered the issue.

WORKFLOW:
1. Confirm incident_start_time and optional namespace/symptoms
2. Call cluster_health for the incident window to identify critical resources
3. Call resource_changes to see what changed around incident_start_time
4. For top suspect resources, call investigate to get detailed timelines
5. Correlate events to identify the most likely root cause
6. Recommend immediate mitigation steps (restart, rollback, scale, etc.)
7. Suggest monitoring and follow-up actions
8. List additional logs/metrics needed for deeper analysis

OUTPUT FORMAT:
- Current Status: What is the current state of affected resources
- Timeline: Key events leading up to and during the incident
- Suspected Root Cause: Evidence-based hypothesis (clearly mark as hypothesis if not certain)
- Immediate Actions: Concrete mitigation steps to try now
- Monitoring: What to watch to confirm mitigation worked
- Follow-Up: Additional investigation needed and logs to collect
- Data Gaps: What information is missing`

	userMsg := `LIVE INCIDENT - Please help triage and investigate:

Incident Details:
- Started at: {{incident_start_time}}
{{#if current_time}}- Current time: {{current_time}}{{/if}}
{{#if namespace}}- Namespace: {{namespace}}{{/if}}
{{#if symptoms}}- Symptoms: {{symptoms}}{{/if}}

Use the Spectre MCP tools to investigate the current state and recent changes. Identify the root cause and recommend immediate mitigation steps based only on actual tool results.`

	// Simple template replacement
	if startTime, ok := args["incident_start_time"]; ok {
		userMsg = replaceTemplate(userMsg, "incident_start_time", fmt.Sprintf("%v", startTime))
	}
	if currentTime, ok := args["current_time"]; ok {
		userMsg = replaceTemplate(userMsg, "#if current_time", "")
		userMsg = replaceTemplate(userMsg, "current_time", fmt.Sprintf("%v", currentTime))
		userMsg = replaceTemplate(userMsg, "/if", "")
	} else {
		userMsg = removeConditional(userMsg, "current_time")
	}
	if namespace, ok := args["namespace"].(string); ok && namespace != "" {
		userMsg = replaceTemplate(userMsg, "#if namespace", "")
		userMsg = replaceTemplate(userMsg, "namespace", namespace)
		userMsg = replaceTemplate(userMsg, "/if", "")
	} else {
		userMsg = removeConditional(userMsg, "namespace")
	}
	if symptoms, ok := args["symptoms"].(string); ok && symptoms != "" {
		userMsg = replaceTemplate(userMsg, "#if symptoms", "")
		userMsg = replaceTemplate(userMsg, "symptoms", symptoms)
		userMsg = replaceTemplate(userMsg, "/if", "")
	} else {
		userMsg = removeConditional(userMsg, "symptoms")
	}

	return &GetPromptResult{
		Description: "Triage and investigate an ongoing incident",
		Messages: []PromptMessage{
			{
				Role: "user",
				Content: PromptContent{
					Type: "text",
					Text: systemMsg,
				},
			},
			{
				Role: "user",
				Content: PromptContent{
					Type: "text",
					Text: userMsg,
				},
			},
		},
	}
}

// replaceTemplate replaces {{key}} with value in the template
func replaceTemplate(template, key, value string) string {
	return strings.ReplaceAll(template, "{{"+key+"}}", value)
}

// removeConditional removes conditional blocks like {{#if key}}...{{/if}}
func removeConditional(template, key string) string {
	startTag := "{{#if " + key + "}}"
	endTag := "{{/if}}"

	startIdx := strings.Index(template, startTag)
	if startIdx == -1 {
		return template
	}

	endIdx := strings.Index(template[startIdx:], endTag)
	if endIdx == -1 {
		return template
	}

	// Remove the entire conditional block including the line
	endIdx += startIdx + len(endTag)

	// Also remove the newline before if it exists
	if startIdx > 0 && template[startIdx-1] == '\n' {
		startIdx--
	}

	return template[:startIdx] + template[endIdx:]
}
