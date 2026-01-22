package integration

import (
	"context"
	"time"
)

// Integration defines the lifecycle contract for all integrations.
// Integrations are compiled into Spectre (in-tree) and can run multiple
// instances with different configurations (e.g., victorialogs-prod, victorialogs-staging).
type Integration interface {
	// Metadata returns the integration's identifying information
	Metadata() IntegrationMetadata

	// Start initializes the integration instance with the provided context.
	// Returns error if initialization fails (e.g., invalid config, connection failure).
	// Failed connections should not prevent startup - mark instance as Degraded instead.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the integration instance.
	// Should wait for in-flight operations with timeout, then force stop.
	Stop(ctx context.Context) error

	// Health returns the current health status of the integration instance.
	// Used for monitoring and auto-recovery (periodic health checks).
	Health(ctx context.Context) HealthStatus

	// RegisterTools registers MCP tools with the server for this integration instance.
	// Called during startup after Start() succeeds or marks instance as Degraded.
	RegisterTools(registry ToolRegistry) error
}

// IntegrationMetadata holds identifying information for an integration instance.
type IntegrationMetadata struct {
	// Name is the unique instance name (e.g., "victorialogs-prod")
	Name string

	// Version is the integration implementation version (e.g., "1.0.0")
	Version string

	// Description is a human-readable description of the integration
	Description string

	// Type is the integration type for multiple instances (e.g., "victorialogs")
	// Multiple instances of the same Type can exist with different Names
	Type string
}

// HealthStatus represents the current health state of an integration instance.
type HealthStatus int

const (
	// Healthy indicates the integration is functioning normally
	Healthy HealthStatus = iota

	// Degraded indicates connection failed but instance remains registered
	// MCP tools for this instance will return errors until health recovers
	Degraded

	// Stopped indicates the integration was explicitly stopped
	Stopped
)

// String returns the string representation of HealthStatus
func (h HealthStatus) String() string {
	switch h {
	case Healthy:
		return "healthy"
	case Degraded:
		return "degraded"
	case Stopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// ToolRegistry is the interface that the MCP server implements to register tools.
// Integration instances call RegisterTool to expose their functionality via MCP.
//
// This is a placeholder interface - concrete implementation will be provided in Phase 2
// when integrating with the existing MCP server (internal/mcp/server.go).
type ToolRegistry interface {
	// RegisterTool registers an MCP tool with the given name, handler, and input schema.
	// name: unique tool name (e.g., "victorialogs_query")
	// description: human-readable description of the tool
	// handler: function that executes the tool logic
	// inputSchema: JSON Schema object defining the tool's input parameters
	RegisterTool(name string, description string, handler ToolHandler, inputSchema map[string]interface{}) error
}

// ToolHandler is the function signature for tool execution logic.
// ctx: context for cancellation and timeouts
// args: JSON-encoded tool arguments
// Returns: result (JSON-serializable) and error
type ToolHandler func(ctx context.Context, args []byte) (interface{}, error)

// InstanceConfig is a placeholder type for instance-specific configuration.
// Each integration type provides its own concrete config struct that embeds
// or implements this interface.
type InstanceConfig interface{}

// IntegrationStatus represents the status of an integration instance.
type IntegrationStatus struct {
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	Enabled    bool        `json:"enabled"`
	Health     string      `json:"health"`
	SyncStatus *SyncStatus `json:"syncStatus,omitempty"` // Optional, only for integrations that sync
}

// SyncStatus represents the synchronization status for integrations that perform periodic syncing.
type SyncStatus struct {
	LastSyncTime   *time.Time `json:"lastSyncTime,omitempty"` // Nil if never synced
	DashboardCount int        `json:"dashboardCount"`         // Total dashboards synced
	LastError      string     `json:"lastError,omitempty"`    // Empty if no error
	InProgress     bool       `json:"inProgress"`             // True during active sync
}
