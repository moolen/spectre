package integration

import (
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/moolen/spectre/internal/config"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// ManagerConfig holds configuration for the integration Manager.
type ManagerConfig struct {
	// ConfigPath is the path to the integrations YAML file
	ConfigPath string

	// HealthCheckInterval is how often to check integration health for auto-recovery
	// Default: 30 seconds
	HealthCheckInterval time.Duration

	// ShutdownTimeout is the maximum time to wait for instances to stop gracefully
	// Default: 10 seconds
	ShutdownTimeout time.Duration

	// MinIntegrationVersion is the minimum required integration version (PLUG-06)
	// If set, integrations with older versions will be rejected during startup
	// Format: semantic version string (e.g., "1.0.0")
	MinIntegrationVersion string

	// GraphClient is the optional graph database client for integrations that need it.
	// If set, integrations implementing GraphClientSetter will receive this client.
	GraphClient graph.Client
}

// Manager orchestrates the lifecycle of all integration instances.
type Manager struct {
	config       ManagerConfig
	registry     *Registry
	watcher      *config.IntegrationWatcher
	healthCancel func()
	stopped      chan struct{}
	mu           sync.RWMutex
	logger       *logging.Logger

	minVersion  *version.Version
	mcpRegistry ToolRegistry
	graphClient graph.Client
}

// NewManager creates a new integration lifecycle manager.
// Returns error if ConfigPath is empty or MinIntegrationVersion is invalid.
func NewManager(cfg ManagerConfig) (*Manager, error) {
	if cfg.ConfigPath == "" {
		return nil, fmt.Errorf("ConfigPath cannot be empty")
	}

	if cfg.HealthCheckInterval == 0 {
		cfg.HealthCheckInterval = 30 * time.Second
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 10 * time.Second
	}

	m := &Manager{
		config:      cfg,
		registry:    NewRegistry(),
		stopped:     make(chan struct{}),
		logger:      logging.GetLogger("integration.manager"),
		graphClient: cfg.GraphClient,
	}

	if cfg.MinIntegrationVersion != "" {
		minVer, err := version.NewVersion(cfg.MinIntegrationVersion)
		if err != nil {
			return nil, fmt.Errorf("invalid MinIntegrationVersion %q: %w", cfg.MinIntegrationVersion, err)
		}
		m.minVersion = minVer
		m.logger.Debug("Minimum integration version: %s", cfg.MinIntegrationVersion)
	}

	return m, nil
}

// NewManagerWithMCPRegistry creates a new integration lifecycle manager with MCP tool registration.
func NewManagerWithMCPRegistry(cfg ManagerConfig, mcpRegistry ToolRegistry) (*Manager, error) {
	m, err := NewManager(cfg)
	if err != nil {
		return nil, err
	}
	m.mcpRegistry = mcpRegistry
	return m, nil
}

// Name returns the component name for lifecycle management.
func (m *Manager) Name() string {
	return "integration-manager"
}

// GetRegistry returns the instance registry for MCP server to query.
func (m *Manager) GetRegistry() *Registry {
	return m.registry
}
