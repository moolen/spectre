// Package tools provides tool registry and execution for the Spectre agent.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/agent/provider"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/mcp/client"
	mcptools "github.com/moolen/spectre/internal/mcp/tools"
)

const (
	// MaxToolResponseBytes is the maximum size of a tool response in bytes.
	// Responses larger than this will be truncated to prevent context overflow.
	// 50KB is a reasonable limit (~12,500 tokens at 4 chars/token).
	MaxToolResponseBytes = 50 * 1024
)

// truncatedData is used when tool output exceeds MaxToolResponseBytes.
// It preserves structure while indicating data was truncated.
type truncatedData struct {
	Truncated      bool   `json:"_truncated"`
	OriginalBytes  int    `json:"_original_bytes"`
	TruncatedBytes int    `json:"_truncated_bytes"`
	TruncationNote string `json:"_truncation_note"`
	PartialData    string `json:"partial_data"`
}

// truncateResult checks if the result data exceeds MaxToolResponseBytes and
// truncates it if necessary to prevent context overflow.
func truncateResult(result *Result, maxBytes int) *Result {
	if result == nil || result.Data == nil {
		return result
	}

	// Marshal the data to check its size
	dataBytes, err := json.Marshal(result.Data)
	if err != nil {
		// If we can't marshal, return as-is and let the caller handle it
		return result
	}

	if len(dataBytes) <= maxBytes {
		return result
	}

	// Data exceeds limit - create truncated version
	// Keep some of the original data for context (first ~80% of allowed bytes for partial data)
	partialDataBytes := maxBytes * 80 / 100
	partialData := string(dataBytes)
	if len(partialData) > partialDataBytes {
		partialData = partialData[:partialDataBytes]
	}

	truncated := &truncatedData{
		Truncated:      true,
		OriginalBytes:  len(dataBytes),
		TruncatedBytes: maxBytes,
		TruncationNote: fmt.Sprintf("Response truncated from %d to ~%d bytes to prevent context overflow. Consider using more specific filters to reduce result size.", len(dataBytes), maxBytes),
		PartialData:    partialData,
	}

	// Update summary to indicate truncation
	summary := result.Summary
	if summary != "" {
		summary = fmt.Sprintf("%s [TRUNCATED: %d→%d bytes]", summary, len(dataBytes), maxBytes)
	} else {
		summary = fmt.Sprintf("[TRUNCATED: %d→%d bytes]", len(dataBytes), maxBytes)
	}

	return &Result{
		Success:         result.Success,
		Data:            truncated,
		Error:           result.Error,
		Summary:         summary,
		ExecutionTimeMs: result.ExecutionTimeMs,
	}
}

// Tool defines the interface for agent tools.
type Tool interface {
	// Name returns the tool's unique identifier.
	Name() string

	// Description returns a human-readable description for the LLM.
	Description() string

	// InputSchema returns JSON Schema for input validation.
	InputSchema() map[string]interface{}

	// Execute runs the tool with given input.
	Execute(ctx context.Context, input json.RawMessage) (*Result, error)
}

// Result represents the output of a tool execution.
type Result struct {
	// Success indicates if the tool executed successfully
	Success bool `json:"success"`

	// Data contains the tool's output (tool-specific structure)
	Data interface{} `json:"data,omitempty"`

	// Error contains error details if Success is false
	Error string `json:"error,omitempty"`

	// Summary is a brief description of what happened (for display)
	Summary string `json:"summary,omitempty"`

	// ExecutionTimeMs is how long the tool took to run
	ExecutionTimeMs int64 `json:"executionTimeMs"`
}

// Registry manages tool registration and discovery.
type Registry struct {
	tools  map[string]Tool
	mu     sync.RWMutex
	logger *slog.Logger
}

// Dependencies contains the external dependencies needed by tools.
type Dependencies struct {
	SpectreClient *client.SpectreClient
	GraphClient   graph.Client
	Logger        *slog.Logger
}

// NewRegistry creates a new tool registry with the provided dependencies.
func NewRegistry(deps Dependencies) *Registry {
	r := &Registry{
		tools:  make(map[string]Tool),
		logger: deps.Logger,
	}

	if r.logger == nil {
		r.logger = slog.Default()
	}

	// Register Spectre API tools
	if deps.SpectreClient != nil {
		r.register(NewClusterHealthToolWrapper(deps.SpectreClient))
		r.register(NewResourceTimelineChangesToolWrapper(deps.SpectreClient))
		r.register(NewResourceTimelineToolWrapper(deps.SpectreClient))
		r.register(NewDetectAnomaliesToolWrapper(deps.SpectreClient))
		r.register(NewCausalPathsToolWrapper(deps.SpectreClient))
	}

	// Register graph tools (currently none - causal_paths now uses HTTP API)
	if deps.GraphClient != nil {
		// TODO: Re-enable when GraphBlastRadiusTool is implemented
		// r.register(NewBlastRadiusToolWrapper(deps.GraphClient))
	}

	return r
}

// NewMockRegistry creates a tool registry with mock tools that return canned responses.
// This is used for testing the TUI without requiring a real Spectre API server.
func NewMockRegistry() *Registry {
	r := &Registry{
		tools:  make(map[string]Tool),
		logger: slog.Default(),
	}

	// Register mock versions of all tools
	r.register(&MockTool{
		name:        "cluster_health",
		description: "Get cluster health status",
		schema: map[string]interface{}{
			"type":     "object",
			"required": []string{"start_time", "end_time"},
			"properties": map[string]interface{}{
				"start_time":    map[string]interface{}{"type": "integer"},
				"end_time":      map[string]interface{}{"type": "integer"},
				"namespace":     map[string]interface{}{"type": "string"},
				"max_resources": map[string]interface{}{"type": "integer"},
			},
		},
		response: &Result{
			Success: true,
			Summary: "Found 2 issues in the cluster",
			Data: map[string]interface{}{
				"overall_status":         "Warning",
				"total_resources":        15,
				"error_resource_count":   1,
				"warning_resource_count": 1,
				"issue_resource_uids":    []string{"abc-123-pod", "def-456-deploy"},
				"top_issues": []map[string]interface{}{
					{"resource_uid": "abc-123-pod", "kind": "Pod", "namespace": "default", "name": "my-app-xyz", "current_status": "Error", "error_message": "CrashLoopBackOff"},
					{"resource_uid": "def-456-deploy", "kind": "Deployment", "namespace": "default", "name": "my-app", "current_status": "Warning", "error_message": "Unavailable replicas"},
				},
			},
		},
		delay: 300 * time.Millisecond,
	})

	r.register(&MockTool{
		name:        "resource_timeline_changes",
		description: "Get semantic field-level changes for resources by UID",
		schema: map[string]interface{}{
			"type":     "object",
			"required": []string{"resource_uids"},
			"properties": map[string]interface{}{
				"resource_uids":            map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"start_time":               map[string]interface{}{"type": "integer"},
				"end_time":                 map[string]interface{}{"type": "integer"},
				"include_full_snapshot":    map[string]interface{}{"type": "boolean"},
				"max_changes_per_resource": map[string]interface{}{"type": "integer"},
			},
		},
		response: &Result{
			Success: true,
			Summary: "Found 3 semantic changes for 1 resource",
			Data: map[string]interface{}{
				"resources": []map[string]interface{}{
					{
						"uid":       "abc-123-def",
						"kind":      "Deployment",
						"namespace": "default",
						"name":      "my-app",
						"changes": []map[string]interface{}{
							{
								"timestamp":      1736703000,
								"timestamp_text": "2026-01-12T18:30:00Z",
								"path":           "spec.template.spec.containers[0].image",
								"old":            "my-app:v1.0.0",
								"new":            "my-app:v1.1.0",
								"op":             "replace",
								"category":       "Config",
							},
							{
								"timestamp":      1736703035,
								"timestamp_text": "2026-01-12T18:30:35Z",
								"path":           "status.replicas",
								"old":            3,
								"new":            2,
								"op":             "replace",
								"category":       "Status",
							},
						},
						"status_summary": map[string]interface{}{
							"current_status": "Warning",
							"transitions": []map[string]interface{}{
								{
									"from_status":    "Ready",
									"to_status":      "Warning",
									"timestamp":      1736703035,
									"timestamp_text": "2026-01-12T18:30:35Z",
									"reason":         "Unavailable replicas",
								},
							},
						},
						"change_count": 2,
					},
				},
				"summary": map[string]interface{}{
					"total_resources":       1,
					"total_changes":         2,
					"resources_with_errors": 0,
					"resources_not_found":   0,
				},
				"execution_time_ms": 45,
			},
		},
		delay: 300 * time.Millisecond,
	})

	r.register(&MockTool{
		name:        "resource_timeline",
		description: "Get resource timeline with status segments, events, and transitions",
		schema: map[string]interface{}{
			"type":     "object",
			"required": []string{"resource_kind", "start_time", "end_time"},
			"properties": map[string]interface{}{
				"resource_kind": map[string]interface{}{"type": "string"},
				"resource_name": map[string]interface{}{"type": "string"},
				"namespace":     map[string]interface{}{"type": "string"},
				"start_time":    map[string]interface{}{"type": "integer"},
				"end_time":      map[string]interface{}{"type": "integer"},
				"max_results":   map[string]interface{}{"type": "integer"},
			},
		},
		response: &Result{
			Success: true,
			Summary: "Retrieved timeline for 1 resource",
			Data: map[string]interface{}{
				"timelines": []map[string]interface{}{
					{
						"resource_uid":    "abc-123-pod",
						"kind":            "Pod",
						"namespace":       "default",
						"name":            "my-app-xyz",
						"current_status":  "Error",
						"current_message": "CrashLoopBackOff",
						"status_segments": []map[string]interface{}{
							{
								"start_time": 1736703000,
								"end_time":   1736703600,
								"status":     "Error",
								"message":    "CrashLoopBackOff",
								"duration":   600,
							},
						},
						"events": []map[string]interface{}{
							{
								"timestamp": 1736703000,
								"reason":    "BackOff",
								"message":   "Back-off restarting failed container app",
								"type":      "Warning",
								"count":     15,
							},
						},
					},
				},
				"execution_time_ms": 45,
			},
		},
		delay: 300 * time.Millisecond,
	})

	r.register(&MockTool{
		name:        "detect_anomalies",
		description: "Detect anomalies in the cluster",
		schema: map[string]interface{}{
			"type":     "object",
			"required": []string{"start_time", "end_time"},
			"properties": map[string]interface{}{
				"resource_uid": map[string]interface{}{"type": "string"},
				"namespace":    map[string]interface{}{"type": "string"},
				"kind":         map[string]interface{}{"type": "string"},
				"start_time":   map[string]interface{}{"type": "integer"},
				"end_time":     map[string]interface{}{"type": "integer"},
				"max_results":  map[string]interface{}{"type": "integer"},
			},
		},
		response: &Result{
			Success: true,
			Summary: "Detected 2 anomalies across 5 nodes",
			Data: map[string]interface{}{
				"anomaly_count": 2,
				"metadata": map[string]interface{}{
					"nodes_analyzed": 5,
				},
				"anomalies": []map[string]interface{}{
					{
						"type":       "crash_loop",
						"resource":   "pod/default/my-app-xyz",
						"severity":   "high",
						"message":    "Pod restart count increased from 0 to 15 in 10 minutes",
						"start_time": "2026-01-12T18:30:00Z",
					},
					{
						"type":       "error_rate",
						"resource":   "deployment/default/my-app",
						"severity":   "medium",
						"message":    "Error rate increased by 200%",
						"start_time": "2026-01-12T18:30:00Z",
					},
				},
			},
		},
		delay: 300 * time.Millisecond,
	})

	r.register(&MockTool{
		name:        "causal_paths",
		description: "Find causal paths between resources",
		schema: map[string]interface{}{
			"type":     "object",
			"required": []string{"resource_uid", "failure_timestamp"},
			"properties": map[string]interface{}{
				"resource_uid":      map[string]interface{}{"type": "string"},
				"failure_timestamp": map[string]interface{}{"type": "integer"},
				"lookback_minutes":  map[string]interface{}{"type": "integer"},
				"max_depth":         map[string]interface{}{"type": "integer"},
				"max_paths":         map[string]interface{}{"type": "integer"},
			},
		},
		response: &Result{
			Success: true,
			Summary: "Found 1 causal path",
			Data: map[string]interface{}{
				"paths": []map[string]interface{}{
					{
						"nodes": []string{
							"deployment/default/my-app",
							"replicaset/default/my-app-abc123",
							"pod/default/my-app-xyz",
						},
						"confidence": 0.85,
						"summary":    "Deployment rollout caused pod crash",
					},
				},
			},
		},
		delay: 300 * time.Millisecond,
	})

	return r
}

// MockTool is a tool that returns canned responses for testing.
type MockTool struct {
	name        string
	description string
	schema      map[string]interface{}
	response    *Result
	delay       time.Duration
}

func (t *MockTool) Name() string                        { return t.name }
func (t *MockTool) Description() string                 { return t.description }
func (t *MockTool) InputSchema() map[string]interface{} { return t.schema }

func (t *MockTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	// Simulate execution delay
	if t.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(t.delay):
		}
	}

	if t.response == nil {
		return &Result{
			Success: true,
			Summary: fmt.Sprintf("Mock response for %s", t.name),
			Data:    map[string]interface{}{"mock": true},
		}, nil
	}

	return &Result{
		Success:         t.response.Success,
		Data:            t.response.Data,
		Error:           t.response.Error,
		Summary:         t.response.Summary,
		ExecutionTimeMs: t.delay.Milliseconds(),
	}, nil
}

// register adds a tool to the registry (internal, no locking).
func (r *Registry) register(tool Tool) {
	r.tools[tool.Name()] = tool
	r.logger.Debug("registered tool", "name", tool.Name())
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.register(tool)
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tools.
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// ToProviderTools converts registry tools to provider tool definitions.
func (r *Registry) ToProviderTools() []provider.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]provider.ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		defs = append(defs, provider.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.InputSchema(),
		})
	}
	return defs
}

// Execute runs a tool by name with the given input.
func (r *Registry) Execute(ctx context.Context, name string, input json.RawMessage) *Result {
	tool, ok := r.Get(name)
	if !ok {
		return &Result{
			Success: false,
			Error:   fmt.Sprintf("tool %q not found", name),
		}
	}

	start := time.Now()
	result, err := tool.Execute(ctx, input)
	if err != nil {
		return &Result{
			Success:         false,
			Error:           err.Error(),
			ExecutionTimeMs: time.Since(start).Milliseconds(),
		}
	}

	result.ExecutionTimeMs = time.Since(start).Milliseconds()

	// Truncate result if it exceeds the maximum size to prevent context overflow
	result = truncateResult(result, MaxToolResponseBytes)

	return result
}

// =============================================================================
// Tool Wrappers for Existing MCP Tools
// =============================================================================

// ClusterHealthToolWrapper wraps the MCP cluster_health tool.
type ClusterHealthToolWrapper struct {
	inner *mcptools.ClusterHealthTool
}

func NewClusterHealthToolWrapper(client *client.SpectreClient) *ClusterHealthToolWrapper {
	return &ClusterHealthToolWrapper{
		inner: mcptools.NewClusterHealthToolWithClient(client),
	}
}

func (t *ClusterHealthToolWrapper) Name() string { return "cluster_health" }

func (t *ClusterHealthToolWrapper) Description() string {
	return `Get an overview of cluster health status including resource counts by status (Ready, Warning, Error, Terminating), top issues, and error rates.

Use this tool to:
- Get a quick overview of cluster health
- Find resources in error or warning state
- Identify the most problematic resources

Input:
- start_time: Unix timestamp (seconds) for the start of the time range
- end_time: Unix timestamp (seconds) for the end of the time range
- namespace (optional): Filter to a specific namespace
- max_resources (optional): Maximum resources to list per status (default: 100, max: 500)`
}

func (t *ClusterHealthToolWrapper) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":     "object",
		"required": []string{"start_time", "end_time"},
		"properties": map[string]interface{}{
			"start_time": map[string]interface{}{
				"type":        "integer",
				"description": "Unix timestamp (seconds) for start of time range",
			},
			"end_time": map[string]interface{}{
				"type":        "integer",
				"description": "Unix timestamp (seconds) for end of time range",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Filter to a specific namespace (optional)",
			},
			"max_resources": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum resources to list per status (default: 100)",
			},
		},
	}
}

func (t *ClusterHealthToolWrapper) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	data, err := t.inner.Execute(ctx, input)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	// Generate summary from output
	output, ok := data.(*mcptools.ClusterHealthOutput)
	summary := "Retrieved cluster health status"
	if ok {
		summary = fmt.Sprintf("Cluster %s: %d resources (%d errors, %d warnings)",
			output.OverallStatus, output.TotalResources, output.ErrorResourceCount, output.WarningResourceCount)
	}

	return &Result{
		Success: true,
		Data:    data,
		Summary: summary,
	}, nil
}

// ResourceTimelineChangesToolWrapper wraps the MCP resource_timeline_changes tool.
type ResourceTimelineChangesToolWrapper struct {
	inner *mcptools.ResourceTimelineChangesTool
}

func NewResourceTimelineChangesToolWrapper(client *client.SpectreClient) *ResourceTimelineChangesToolWrapper {
	return &ResourceTimelineChangesToolWrapper{
		inner: mcptools.NewResourceTimelineChangesTool(client),
	}
}

func (t *ResourceTimelineChangesToolWrapper) Name() string { return "resource_timeline_changes" }

func (t *ResourceTimelineChangesToolWrapper) Description() string {
	return `Get semantic field-level changes for resources by UID with noise filtering and status condition summarization.

Use this tool to:
- See exactly what fields changed between resource versions
- Get detailed diffs with path, old value, new value, and operation type
- Understand status condition transitions over time
- Batch query multiple resources by their UIDs

Input:
- resource_uids: List of resource UIDs to query (required, max 10)
- start_time (optional): Unix timestamp (seconds) for start of time range (default: 1 hour ago)
- end_time (optional): Unix timestamp (seconds) for end of time range (default: now)
- include_full_snapshot (optional): Include first segment's full resource JSON (default: false)
- max_changes_per_resource (optional): Max changes per resource (default: 50, max: 200)`
}

func (t *ResourceTimelineChangesToolWrapper) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":     "object",
		"required": []string{"resource_uids"},
		"properties": map[string]interface{}{
			"resource_uids": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "List of resource UIDs to query (required, max 10)",
			},
			"start_time": map[string]interface{}{
				"type":        "integer",
				"description": "Unix timestamp (seconds) for start of time range (default: 1 hour ago)",
			},
			"end_time": map[string]interface{}{
				"type":        "integer",
				"description": "Unix timestamp (seconds) for end of time range (default: now)",
			},
			"include_full_snapshot": map[string]interface{}{
				"type":        "boolean",
				"description": "Include first segment's full resource JSON (default: false)",
			},
			"max_changes_per_resource": map[string]interface{}{
				"type":        "integer",
				"description": "Max changes per resource (default: 50, max: 200)",
			},
		},
	}
}

func (t *ResourceTimelineChangesToolWrapper) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	data, err := t.inner.Execute(ctx, input)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	output, ok := data.(*mcptools.ResourceTimelineChangesOutput)
	summary := "Retrieved resource timeline changes"
	if ok {
		summary = fmt.Sprintf("Found %d changes across %d resources", output.Summary.TotalChanges, output.Summary.TotalResources)
	}

	return &Result{
		Success: true,
		Data:    data,
		Summary: summary,
	}, nil
}

// ResourceTimelineToolWrapper wraps the MCP resource_timeline tool.
type ResourceTimelineToolWrapper struct {
	inner *mcptools.ResourceTimelineTool
}

func NewResourceTimelineToolWrapper(client *client.SpectreClient) *ResourceTimelineToolWrapper {
	return &ResourceTimelineToolWrapper{
		inner: mcptools.NewResourceTimelineToolWithClient(client),
	}
}

func (t *ResourceTimelineToolWrapper) Name() string { return "resource_timeline" }

func (t *ResourceTimelineToolWrapper) Description() string {
	return `Get resource timeline with status segments, events, and transitions for root cause analysis.

Use this tool to:
- Get status history for a resource kind
- See status transitions over time
- View related Kubernetes events
- Filter by name or namespace

Input:
- resource_kind: Resource kind to get timeline for (e.g., 'Pod', 'Deployment')
- resource_name (optional): Specific resource name, or '*' for all
- namespace (optional): Kubernetes namespace to filter by
- start_time: Unix timestamp (seconds) for start of time range
- end_time: Unix timestamp (seconds) for end of time range
- max_results (optional): Max resources to return when using '*' (default 20, max 100)`
}

func (t *ResourceTimelineToolWrapper) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":     "object",
		"required": []string{"resource_kind", "start_time", "end_time"},
		"properties": map[string]interface{}{
			"resource_kind": map[string]interface{}{
				"type":        "string",
				"description": "Resource kind to get timeline for (e.g., 'Pod', 'Deployment')",
			},
			"resource_name": map[string]interface{}{
				"type":        "string",
				"description": "Specific resource name, or '*' for all",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace to filter by",
			},
			"start_time": map[string]interface{}{
				"type":        "integer",
				"description": "Unix timestamp (seconds) for start of time range",
			},
			"end_time": map[string]interface{}{
				"type":        "integer",
				"description": "Unix timestamp (seconds) for end of time range",
			},
			"max_results": map[string]interface{}{
				"type":        "integer",
				"description": "Max resources to return when using '*' (default 20, max 100)",
			},
		},
	}
}

func (t *ResourceTimelineToolWrapper) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	data, err := t.inner.Execute(ctx, input)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	// Generate summary from output
	output, ok := data.(*mcptools.ResourceTimelineOutput)
	summary := "Retrieved resource timeline"
	if ok {
		summary = fmt.Sprintf("Retrieved timeline for %d resources", len(output.Timelines))
	}

	return &Result{
		Success: true,
		Data:    data,
		Summary: summary,
	}, nil
}

// CausalPathsToolWrapper wraps the MCP causal_paths graph tool.
type CausalPathsToolWrapper struct {
	inner *mcptools.CausalPathsTool
}

func NewCausalPathsToolWrapper(spectreClient *client.SpectreClient) *CausalPathsToolWrapper {
	return &CausalPathsToolWrapper{
		inner: mcptools.NewCausalPathsToolWithClient(spectreClient),
	}
}

func (t *CausalPathsToolWrapper) Name() string { return "causal_paths" }

func (t *CausalPathsToolWrapper) Description() string {
	return `Discover causal paths from root causes to a failing resource.
This tool queries Spectre's graph database to find ownership chains, configuration changes,
and other relationships that may have caused the current failure state.

Returns ranked causal paths with confidence scores based on temporal proximity,
causal distance, and detected anomalies. Each path shows the full chain from
root cause to symptom.

Use this tool when:
- You need to understand why a resource is failing
- You want to find the root cause of an incident
- You need to trace the ownership/dependency chain

Input:
- resource_uid: UID of the failing resource (symptom)
- failure_timestamp: Unix timestamp (seconds or nanoseconds) when failure was observed
- lookback_minutes (optional): How far back to search for causes (default: 10)
- max_depth (optional): Maximum traversal depth (default: 5, max: 10)
- max_paths (optional): Maximum causal paths to return (default: 5, max: 20)`
}

func (t *CausalPathsToolWrapper) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":     "object",
		"required": []string{"resource_uid", "failure_timestamp"},
		"properties": map[string]interface{}{
			"resource_uid": map[string]interface{}{
				"type":        "string",
				"description": "UID of the failing resource (symptom)",
			},
			"failure_timestamp": map[string]interface{}{
				"type":        "integer",
				"description": "Unix timestamp (seconds or nanoseconds) when failure was observed",
			},
			"lookback_minutes": map[string]interface{}{
				"type":        "integer",
				"description": "How far back to search for causes in minutes (default: 10)",
			},
			"max_depth": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum traversal depth (default: 5, max: 10)",
			},
			"max_paths": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum causal paths to return (default: 5, max: 20)",
			},
		},
	}
}

func (t *CausalPathsToolWrapper) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	// Transform input field names from snake_case to camelCase for the inner tool
	var rawInput map[string]interface{}
	if err := json.Unmarshal(input, &rawInput); err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	// Map field names
	transformedInput := make(map[string]interface{})
	if v, ok := rawInput["resource_uid"]; ok {
		transformedInput["resourceUID"] = v
	}
	if v, ok := rawInput["failure_timestamp"]; ok {
		transformedInput["failureTimestamp"] = v
	}
	if v, ok := rawInput["lookback_minutes"]; ok {
		transformedInput["lookbackMinutes"] = v
	}
	if v, ok := rawInput["max_depth"]; ok {
		transformedInput["maxDepth"] = v
	}
	if v, ok := rawInput["max_paths"]; ok {
		transformedInput["maxPaths"] = v
	}

	transformedJSON, err := json.Marshal(transformedInput)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	data, err := t.inner.Execute(ctx, transformedJSON)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	// Generate summary based on response
	summary := "Discovered causal paths"

	return &Result{
		Success: true,
		Data:    data,
		Summary: summary,
	}, nil
}

// TODO: Re-enable BlastRadiusToolWrapper when GraphBlastRadiusTool is implemented
/*
// BlastRadiusToolWrapper wraps the MCP calculate_blast_radius graph tool.
type BlastRadiusToolWrapper struct {
	inner *mcptools.GraphBlastRadiusTool
}

func NewBlastRadiusToolWrapper(graphClient graph.Client) *BlastRadiusToolWrapper {
	return &BlastRadiusToolWrapper{
		inner: mcptools.NewGraphBlastRadiusTool(graphClient),
	}
}

func (t *BlastRadiusToolWrapper) Name() string { return "calculate_blast_radius" }

func (t *BlastRadiusToolWrapper) Description() string {
	return `Calculate the blast radius of a change - what resources could be affected if a given resource changes or fails.

Use this tool to:
- Understand the impact of a potential change
- See what resources depend on a given resource
- Assess risk before making changes

Input:
- resource_uid: UID of the resource to analyze
- max_depth (optional): Maximum depth to traverse (default: 3)
- include_types (optional): List of relationship types to include`
}

func (t *BlastRadiusToolWrapper) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":     "object",
		"required": []string{"resource_uid"},
		"properties": map[string]interface{}{
			"resource_uid": map[string]interface{}{
				"type":        "string",
				"description": "UID of the resource to analyze",
			},
			"max_depth": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum depth to traverse (default: 3)",
			},
			"include_types": map[string]interface{}{
				"type":        "array",
				"description": "List of relationship types to include",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}
}

func (t *BlastRadiusToolWrapper) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	data, err := t.inner.Execute(ctx, input)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	output, ok := data.(*mcptools.BlastRadiusOutput)
	summary := "Calculated blast radius"
	if ok {
		summary = fmt.Sprintf("Blast radius: %d affected resources", output.TotalImpacted)
	}

	return &Result{
		Success: true,
		Data:    data,
		Summary: summary,
	}, nil
}
*/

// DetectAnomaliesToolWrapper wraps the MCP detect_anomalies tool.
type DetectAnomaliesToolWrapper struct {
	inner *mcptools.DetectAnomaliesTool
}

func NewDetectAnomaliesToolWrapper(client *client.SpectreClient) *DetectAnomaliesToolWrapper {
	return &DetectAnomaliesToolWrapper{
		inner: mcptools.NewDetectAnomaliesToolWithClient(client),
	}
}

func (t *DetectAnomaliesToolWrapper) Name() string { return "detect_anomalies" }

func (t *DetectAnomaliesToolWrapper) Description() string {
	return `Detect anomalies in resources. Analyzes resources for issues like crash loops, image pull errors, OOM kills, config errors, state transitions, and networking problems.

Use this tool when:
- You need to find what's wrong with a specific resource (use resource_uid)
- You want to scan all resources of a certain type in a namespace (use namespace + kind)
- You're investigating why resources are unhealthy

Input (two modes):
Mode 1 - Single resource by UID:
- resource_uid: The UID of the resource to analyze (from cluster_health or resource_timeline)
- start_time: Unix timestamp (seconds) for start of time range
- end_time: Unix timestamp (seconds) for end of time range

Mode 2 - Multiple resources by namespace/kind:
- namespace: Kubernetes namespace to filter by
- kind: Resource kind to filter by (e.g., 'Pod', 'Deployment')
- start_time: Unix timestamp (seconds) for start of time range
- end_time: Unix timestamp (seconds) for end of time range
- max_results (optional): Max resources to analyze (default: 10, max: 50)`
}

func (t *DetectAnomaliesToolWrapper) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":     "object",
		"required": []string{"start_time", "end_time"},
		"properties": map[string]interface{}{
			"resource_uid": map[string]interface{}{
				"type":        "string",
				"description": "The UID of the resource to analyze for anomalies (alternative to namespace+kind)",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace to filter by (use with kind as alternative to resource_uid)",
			},
			"kind": map[string]interface{}{
				"type":        "string",
				"description": "Resource kind to filter by, e.g., 'Pod', 'Deployment' (use with namespace as alternative to resource_uid)",
			},
			"start_time": map[string]interface{}{
				"type":        "integer",
				"description": "Unix timestamp (seconds) for start of time range",
			},
			"end_time": map[string]interface{}{
				"type":        "integer",
				"description": "Unix timestamp (seconds) for end of time range",
			},
			"max_results": map[string]interface{}{
				"type":        "integer",
				"description": "Max resources to analyze when using namespace/kind filter (default: 10, max: 50)",
			},
		},
	}
}

func (t *DetectAnomaliesToolWrapper) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	data, err := t.inner.Execute(ctx, input)
	if err != nil {
		return &Result{Success: false, Error: err.Error()}, nil
	}

	output, ok := data.(*mcptools.DetectAnomaliesOutput)
	summary := "Detected anomalies in resource"
	if ok {
		if output.AnomalyCount == 0 {
			summary = fmt.Sprintf("No anomalies detected (%d nodes analyzed)", output.Metadata.NodesAnalyzed)
		} else {
			summary = fmt.Sprintf("Detected %d anomalies across %d nodes", output.AnomalyCount, output.Metadata.NodesAnalyzed)
		}
	}

	return &Result{
		Success: true,
		Data:    data,
		Summary: summary,
	}, nil
}
