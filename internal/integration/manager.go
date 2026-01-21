package integration

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/moolen/spectre/internal/config"
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
}

// Manager orchestrates the lifecycle of all integration instances.
// It handles:
// - Version validation on startup (PLUG-06)
// - Starting enabled instances from config
// - Health monitoring with auto-recovery
// - Hot-reload on config changes (full restart)
// - Graceful shutdown with timeout
type Manager struct {
	config       ManagerConfig
	registry     *Registry
	watcher      *config.IntegrationWatcher
	healthCancel context.CancelFunc
	stopped      chan struct{}
	mu           sync.RWMutex
	logger       *logging.Logger

	// minVersion is the parsed minimum version constraint
	minVersion *version.Version

	// mcpRegistry is the optional MCP tool registry for integrations
	mcpRegistry ToolRegistry
}

// NewManager creates a new integration lifecycle manager.
// Returns error if ConfigPath is empty or MinIntegrationVersion is invalid.
func NewManager(cfg ManagerConfig) (*Manager, error) {
	if cfg.ConfigPath == "" {
		return nil, fmt.Errorf("ConfigPath cannot be empty")
	}

	// Set defaults
	if cfg.HealthCheckInterval == 0 {
		cfg.HealthCheckInterval = 30 * time.Second
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 10 * time.Second
	}

	m := &Manager{
		config:  cfg,
		registry: NewRegistry(),
		stopped: make(chan struct{}),
		logger:  logging.GetLogger("integration.manager"),
	}

	// Parse minimum version if provided
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
// This is a convenience constructor for servers that want to enable MCP integration.
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

// Start initializes the manager and starts all enabled integration instances.
// Performs version validation (PLUG-06) before starting any instances.
// Returns error if:
// - Initial config load fails
// - Any instance version is below minimum
// - Config watcher fails to start
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Starting integration manager")

	// Load initial config
	integrationsFile, err := config.LoadIntegrationsFile(m.config.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load integrations config: %w", err)
	}

	// Validate versions and start instances
	if err := m.startInstances(ctx, integrationsFile); err != nil {
		return err
	}

	// Create and start config watcher with reload callback
	watcherConfig := config.IntegrationWatcherConfig{
		FilePath:       m.config.ConfigPath,
		DebounceMillis: 500,
	}
	m.watcher, err = config.NewIntegrationWatcher(watcherConfig, m.handleConfigReload)
	if err != nil {
		// Stop any instances we started before returning error
		m.stopAllInstances(ctx)
		return fmt.Errorf("failed to create config watcher: %w", err)
	}

	if err := m.watcher.Start(ctx); err != nil {
		// Stop any instances we started before returning error
		m.stopAllInstances(ctx)
		return fmt.Errorf("failed to start config watcher: %w", err)
	}

	// Start health check loop
	healthCtx, cancel := context.WithCancel(context.Background())
	m.healthCancel = cancel
	go m.runHealthChecks(healthCtx)

	m.logger.Info("Integration manager started successfully with %d instances", len(m.registry.List()))
	return nil
}

// Stop gracefully stops the manager, config watcher, and all integration instances.
func (m *Manager) Stop(ctx context.Context) error {
	m.logger.Info("Stopping integration manager")

	// Stop health checks
	if m.healthCancel != nil {
		m.healthCancel()
	}

	// Stop config watcher
	if m.watcher != nil {
		if err := m.watcher.Stop(); err != nil {
			m.logger.Warn("Error stopping config watcher: %v", err)
		}
	}

	// Stop all instances
	m.stopAllInstances(ctx)

	// Signal that we've stopped
	close(m.stopped)

	m.logger.Info("Integration manager stopped")
	return nil
}

// GetRegistry returns the instance registry for MCP server to query.
func (m *Manager) GetRegistry() *Registry {
	return m.registry
}

// startInstances validates versions and starts all enabled instances from config.
// Returns error if any version validation fails.
// Instance start failures are logged and marked degraded, but don't fail the manager.
func (m *Manager) startInstances(ctx context.Context, integrationsFile *config.IntegrationsFile) error {
	m.logger.Info("Starting %d integration instance(s)", len(integrationsFile.Instances))

	for _, instanceConfig := range integrationsFile.Instances {
		if !instanceConfig.Enabled {
			m.logger.Debug("Skipping disabled instance: %s", instanceConfig.Name)
			continue
		}

		// Get factory for this integration type
		factory, ok := GetFactory(instanceConfig.Type)
		if !ok {
			m.logger.Error("No factory registered for integration type %q (instance: %s)",
				instanceConfig.Type, instanceConfig.Name)
			continue
		}

		// Create instance
		instance, err := factory(instanceConfig.Name, instanceConfig.Config)
		if err != nil {
			m.logger.Error("Failed to create instance %s (type: %s): %v",
				instanceConfig.Name, instanceConfig.Type, err)
			continue
		}

		// Version validation (PLUG-06)
		if err := m.validateInstanceVersion(instance); err != nil {
			return err // Fail fast on version mismatch
		}

		// Register instance
		if err := m.registry.Register(instanceConfig.Name, instance); err != nil {
			m.logger.Error("Failed to register instance %s: %v", instanceConfig.Name, err)
			continue
		}

		// Start instance
		if err := instance.Start(ctx); err != nil {
			m.logger.Error("Failed to start instance %s: %v (marking as degraded)", instanceConfig.Name, err)
			// Instance is registered but degraded - continue with other instances
			// Fall through to register tools even for degraded instances
		} else {
			m.logger.Info("Started instance: %s (type: %s, version: %s)",
				instanceConfig.Name, instanceConfig.Type, instance.Metadata().Version)
		}

		// Register MCP tools if registry provided
		// This happens after Start() regardless of status (Healthy or Degraded)
		// Degraded instances can still expose tools that return service unavailable errors
		if m.mcpRegistry != nil {
			if err := instance.RegisterTools(m.mcpRegistry); err != nil {
				m.logger.Error("Failed to register tools for %s: %v", instanceConfig.Name, err)
				// Don't fail startup - log and continue
			}
		}
	}

	return nil
}

// validateInstanceVersion checks if instance version meets minimum requirements.
// Returns error if version is below minimum (PLUG-06).
func (m *Manager) validateInstanceVersion(instance Integration) error {
	if m.minVersion == nil {
		// No minimum version configured, skip validation
		return nil
	}

	metadata := instance.Metadata()
	instanceVer, err := version.NewVersion(metadata.Version)
	if err != nil {
		return fmt.Errorf("instance %s has invalid version %q: %w",
			metadata.Name, metadata.Version, err)
	}

	if instanceVer.LessThan(m.minVersion) {
		return fmt.Errorf("instance %s version %s is below minimum required version %s",
			metadata.Name, metadata.Version, m.minVersion.String())
	}

	m.logger.Debug("Instance %s version %s validated (>= %s)",
		metadata.Name, metadata.Version, m.minVersion.String())
	return nil
}

// handleConfigReload is called when the config file changes.
// It performs a full restart: stop all instances, re-validate versions, start new instances.
func (m *Manager) handleConfigReload(newConfig *config.IntegrationsFile) error {
	m.logger.Info("Config reload triggered - restarting all integration instances")

	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop all existing instances
	ctx, cancel := context.WithTimeout(context.Background(), m.config.ShutdownTimeout)
	defer cancel()
	m.stopAllInstancesLocked(ctx)

	// Clear registry
	instanceNames := m.registry.List()
	for _, name := range instanceNames {
		m.registry.Remove(name)
	}

	// Start instances from new config (with version re-validation)
	if err := m.startInstances(context.Background(), newConfig); err != nil {
		// Log error but don't crash - we'll keep running with empty registry
		m.logger.Error("Failed to start instances after config reload: %v", err)
		return err
	}

	m.logger.Info("Config reload complete - %d instances running", len(m.registry.List()))
	return nil
}

// runHealthChecks periodically checks instance health and attempts auto-recovery.
func (m *Manager) runHealthChecks(ctx context.Context) {
	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()

	m.logger.Debug("Health check loop started (interval: %s)", m.config.HealthCheckInterval)

	for {
		select {
		case <-ctx.Done():
			m.logger.Debug("Health check loop stopped")
			return

		case <-ticker.C:
			m.performHealthChecks(ctx)
		}
	}
}

// performHealthChecks checks health of all instances and attempts recovery.
func (m *Manager) performHealthChecks(ctx context.Context) {
	m.mu.RLock()
	instanceNames := m.registry.List()
	m.mu.RUnlock()

	for _, name := range instanceNames {
		m.mu.RLock()
		instance, ok := m.registry.Get(name)
		m.mu.RUnlock()

		if !ok {
			continue
		}

		// Check health
		healthStatus := instance.Health(ctx)

		// Attempt auto-recovery if degraded
		if healthStatus == Degraded {
			m.logger.Debug("Instance %s is degraded, attempting recovery", name)
			if err := instance.Start(ctx); err != nil {
				m.logger.Debug("Recovery failed for instance %s: %v", name, err)
			} else {
				m.logger.Info("Instance %s recovered successfully", name)
			}
		}
	}
}

// stopAllInstances stops all registered instances with timeout.
func (m *Manager) stopAllInstances(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopAllInstancesLocked(ctx)
}

// stopAllInstancesLocked stops all instances - caller must hold write lock.
func (m *Manager) stopAllInstancesLocked(ctx context.Context) {
	instanceNames := m.registry.List()
	m.logger.Debug("Stopping %d instance(s)", len(instanceNames))

	for _, name := range instanceNames {
		instance, ok := m.registry.Get(name)
		if !ok {
			continue
		}

		// Create timeout context for this instance
		stopCtx, cancel := context.WithTimeout(ctx, m.config.ShutdownTimeout)
		if err := instance.Stop(stopCtx); err != nil {
			m.logger.Warn("Error stopping instance %s: %v", name, err)
		} else {
			m.logger.Debug("Stopped instance: %s", name)
		}
		cancel()
	}
}
