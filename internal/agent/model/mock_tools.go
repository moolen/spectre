// Package model provides LLM adapters for the ADK multi-agent system.
package model

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	spectretools "github.com/moolen/spectre/internal/agent/tools"
)

// MockToolRegistry provides canned responses for tools during mock testing.
// It implements the same interface as spectretools.Registry but returns pre-defined responses.
type MockToolRegistry struct {
	tools    map[string]*MockTool
	mu       sync.RWMutex
	logger   *slog.Logger
	scenario *Scenario // Optional: load responses from scenario
}

// MockTool wraps a tool with a canned response.
type MockTool struct {
	name        string
	description string
	schema      map[string]interface{}
	response    *spectretools.Result
	delay       time.Duration
}

// NewMockToolRegistry creates a new mock tool registry with default responses.
func NewMockToolRegistry() *MockToolRegistry {
	r := &MockToolRegistry{
		tools:  make(map[string]*MockTool),
		logger: slog.Default(),
	}

	// Register default mock tools
	r.registerDefaultTools()

	return r
}

// NewMockToolRegistryFromScenario creates a mock registry with responses from a scenario.
func NewMockToolRegistryFromScenario(scenario *Scenario) *MockToolRegistry {
	r := &MockToolRegistry{
		tools:    make(map[string]*MockTool),
		logger:   slog.Default(),
		scenario: scenario,
	}

	// Register default tools first
	r.registerDefaultTools()

	// Override with scenario-specific responses
	if scenario != nil && scenario.ToolResponses != nil {
		for name, resp := range scenario.ToolResponses {
			r.SetResponse(name, &spectretools.Result{
				Success: resp.Success,
				Summary: resp.Summary,
				Data:    resp.Data,
				Error:   resp.Error,
			}, time.Duration(resp.DelayMs)*time.Millisecond)
		}
	}

	return r
}

// registerDefaultTools registers all tools with default mock responses.
func (r *MockToolRegistry) registerDefaultTools() {
	// cluster_health
	r.register(&MockTool{
		name:        "cluster_health",
		description: "Get cluster health status for a namespace",
		schema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"namespace":  map[string]interface{}{"type": "string"},
				"start_time": map[string]interface{}{"type": "integer"},
				"end_time":   map[string]interface{}{"type": "integer"},
			},
		},
		response: &spectretools.Result{
			Success: true,
			Summary: "Found 2 issues in the cluster",
			Data: map[string]interface{}{
				"healthy": false,
				"issues": []map[string]interface{}{
					{"severity": "high", "resource": "pod/my-app-xyz", "message": "Pod not ready - CrashLoopBackOff"},
					{"severity": "medium", "resource": "deployment/my-app", "message": "Deployment has unavailable replicas"},
				},
				"resources_checked": 15,
			},
		},
		delay: 500 * time.Millisecond,
	})

	// resource_timeline_changes
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
		response: &spectretools.Result{
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
			},
		},
		delay: 500 * time.Millisecond,
	})

	// causal_paths
	r.register(&MockTool{
		name:        "causal_paths",
		description: "Find causal paths between resources",
		schema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"source_id": map[string]interface{}{"type": "string"},
				"target_id": map[string]interface{}{"type": "string"},
			},
		},
		response: &spectretools.Result{
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
						"edges": []map[string]interface{}{
							{"from": "deployment/default/my-app", "to": "replicaset/default/my-app-abc123", "relation": "manages"},
							{"from": "replicaset/default/my-app-abc123", "to": "pod/default/my-app-xyz", "relation": "owns"},
						},
					},
				},
			},
		},
		delay: 500 * time.Millisecond,
	})

	// resource_timeline
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
		response: &spectretools.Result{
			Success: true,
			Summary: "Retrieved timeline for 1 resource",
			Data: map[string]interface{}{
				"timelines": []map[string]interface{}{
					{
						"resource_id":     "abc-123-def",
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
		delay: 500 * time.Millisecond,
	})

	// detect_anomalies
	r.register(&MockTool{
		name:        "detect_anomalies",
		description: "Detect anomalies in the cluster",
		schema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"namespace":  map[string]interface{}{"type": "string"},
				"start_time": map[string]interface{}{"type": "integer"},
				"end_time":   map[string]interface{}{"type": "integer"},
			},
		},
		response: &spectretools.Result{
			Success: true,
			Summary: "Detected 2 anomalies",
			Data: map[string]interface{}{
				"anomalies": []map[string]interface{}{
					{
						"type":       "restart_spike",
						"resource":   "pod/default/my-app-xyz",
						"severity":   "high",
						"message":    "Pod restart count increased from 0 to 15 in 10 minutes",
						"start_time": "2026-01-12T18:30:00Z",
					},
					{
						"type":       "error_rate_increase",
						"resource":   "deployment/default/my-app",
						"severity":   "medium",
						"message":    "Error rate increased by 200%",
						"start_time": "2026-01-12T18:30:00Z",
					},
				},
				"total": 2,
			},
		},
		delay: 500 * time.Millisecond,
	})
}

// register adds a mock tool to the registry.
func (r *MockToolRegistry) register(tool *MockTool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.name] = tool
	if r.logger != nil {
		r.logger.Debug("registered mock tool", "name", tool.name)
	}
}

// SetResponse sets or updates the canned response for a tool.
func (r *MockToolRegistry) SetResponse(toolName string, result *spectretools.Result, delay time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tool, ok := r.tools[toolName]; ok {
		tool.response = result
		tool.delay = delay
	} else {
		// Create a new tool with this response
		r.tools[toolName] = &MockTool{
			name:        toolName,
			description: fmt.Sprintf("Mock tool: %s", toolName),
			schema:      map[string]interface{}{"type": "object"},
			response:    result,
			delay:       delay,
		}
	}
}

// Get returns a tool by name.
func (r *MockToolRegistry) Get(name string) (spectretools.Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tools.
func (r *MockToolRegistry) List() []spectretools.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]spectretools.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// ToDefinitions converts all tools to provider.ToolDefinition format.
func (r *MockToolRegistry) ToDefinitions() []map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]map[string]interface{}, 0, len(r.tools))
	for _, tool := range r.tools {
		defs = append(defs, map[string]interface{}{
			"name":         tool.name,
			"description":  tool.description,
			"input_schema": tool.schema,
		})
	}
	return defs
}

// MockTool implementation of spectretools.Tool interface

// Name returns the tool's unique identifier.
func (t *MockTool) Name() string {
	return t.name
}

// Description returns a human-readable description.
func (t *MockTool) Description() string {
	return t.description
}

// InputSchema returns the JSON Schema for input validation.
func (t *MockTool) InputSchema() map[string]interface{} {
	return t.schema
}

// Execute returns the canned response after the configured delay.
func (t *MockTool) Execute(ctx context.Context, input json.RawMessage) (*spectretools.Result, error) {
	// Simulate execution delay
	if t.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(t.delay):
		}
	}

	if t.response == nil {
		return &spectretools.Result{
			Success: true,
			Summary: fmt.Sprintf("Mock response for %s", t.name),
			Data:    map[string]interface{}{"mock": true},
		}, nil
	}

	// Return a copy to prevent mutation
	return &spectretools.Result{
		Success:         t.response.Success,
		Data:            t.response.Data,
		Error:           t.response.Error,
		Summary:         t.response.Summary,
		ExecutionTimeMs: t.delay.Milliseconds(),
	}, nil
}

// Ensure MockTool implements spectretools.Tool at compile time.
var _ spectretools.Tool = (*MockTool)(nil)
