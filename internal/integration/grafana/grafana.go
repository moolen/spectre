// Package grafana provides Grafana metrics integration for Spectre.
package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/moolen/spectre/internal/integration"
	"github.com/moolen/spectre/internal/logging"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func init() {
	// Register the Grafana factory with the global registry
	if err := integration.RegisterFactory("grafana", NewGrafanaIntegration); err != nil {
		// Log but don't fail - factory might already be registered in tests
		logger := logging.GetLogger("integration.grafana")
		logger.Warn("Failed to register grafana factory: %v", err)
	}
}

// GrafanaIntegration implements the Integration interface for Grafana.
type GrafanaIntegration struct {
	name          string
	config        *Config              // Full configuration (includes URL and SecretRef)
	client        *GrafanaClient       // Grafana HTTP client
	secretWatcher *SecretWatcher       // Optional: manages API token from Kubernetes Secret
	logger        *logging.Logger
	ctx           context.Context
	cancel        context.CancelFunc

	// Thread-safe health status
	mu           sync.RWMutex
	healthStatus integration.HealthStatus
}

// NewGrafanaIntegration creates a new Grafana integration instance.
// Note: Client is initialized in Start() to follow lifecycle pattern.
func NewGrafanaIntegration(name string, configMap map[string]interface{}) (integration.Integration, error) {
	// Parse config map into Config struct
	// First marshal to JSON, then unmarshal to Config (handles nested structures)
	configJSON, err := json.Marshal(configMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	var config Config
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate config
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &GrafanaIntegration{
		name:         name,
		config:       &config,
		client:       nil, // Initialized in Start()
		secretWatcher: nil, // Initialized in Start() if config uses SecretRef
		logger:       logging.GetLogger("integration.grafana." + name),
		healthStatus: integration.Stopped,
	}, nil
}

// Metadata returns the integration's identifying information.
func (g *GrafanaIntegration) Metadata() integration.IntegrationMetadata {
	return integration.IntegrationMetadata{
		Name:        g.name,
		Version:     "1.0.0",
		Description: "Grafana metrics integration",
		Type:        "grafana",
	}
}

// Start initializes the integration and validates connectivity.
func (g *GrafanaIntegration) Start(ctx context.Context) error {
	g.logger.Info("Starting Grafana integration: %s (url: %s)", g.name, g.config.URL)

	// Store context for lifecycle management
	g.ctx, g.cancel = context.WithCancel(ctx)

	// Create SecretWatcher if config uses secret ref
	if g.config.UsesSecretRef() {
		g.logger.Info("Creating SecretWatcher for secret: %s, key: %s",
			g.config.APITokenRef.SecretName, g.config.APITokenRef.Key)

		// Create in-cluster Kubernetes client
		k8sConfig, err := rest.InClusterConfig()
		if err != nil {
			return fmt.Errorf("failed to get in-cluster config: %w", err)
		}
		clientset, err := kubernetes.NewForConfig(k8sConfig)
		if err != nil {
			return fmt.Errorf("failed to create Kubernetes clientset: %w", err)
		}

		// Get current namespace (read from ServiceAccount mount)
		namespace, err := getCurrentNamespace()
		if err != nil {
			return fmt.Errorf("failed to determine namespace: %w", err)
		}

		// Create SecretWatcher
		secretWatcher, err := NewSecretWatcher(
			clientset,
			namespace,
			g.config.APITokenRef.SecretName,
			g.config.APITokenRef.Key,
			g.logger,
		)
		if err != nil {
			return fmt.Errorf("failed to create secret watcher: %w", err)
		}

		// Start SecretWatcher
		if err := secretWatcher.Start(g.ctx); err != nil {
			return fmt.Errorf("failed to start secret watcher: %w", err)
		}

		g.secretWatcher = secretWatcher
		g.logger.Info("SecretWatcher started successfully")
	}

	// Create HTTP client (pass secretWatcher if exists)
	g.client = NewGrafanaClient(g.config, g.secretWatcher, g.logger)

	// Test connectivity (warn on failure but continue - degraded state with auto-recovery)
	if err := g.testConnection(g.ctx); err != nil {
		g.logger.Warn("Failed initial connectivity test (will retry on health checks): %v", err)
		g.setHealthStatus(integration.Degraded)
	} else {
		g.setHealthStatus(integration.Healthy)
	}

	g.logger.Info("Grafana integration started successfully (health: %s)", g.getHealthStatus().String())
	return nil
}

// Stop gracefully shuts down the integration.
func (g *GrafanaIntegration) Stop(ctx context.Context) error {
	g.logger.Info("Stopping Grafana integration: %s", g.name)

	// Cancel context
	if g.cancel != nil {
		g.cancel()
	}

	// Stop secret watcher if it exists
	if g.secretWatcher != nil {
		if err := g.secretWatcher.Stop(); err != nil {
			g.logger.Error("Error stopping secret watcher: %v", err)
		}
	}

	// Clear references
	g.client = nil
	g.secretWatcher = nil

	// Update health status
	g.setHealthStatus(integration.Stopped)

	g.logger.Info("Grafana integration stopped")
	return nil
}

// Health returns the current health status.
func (g *GrafanaIntegration) Health(ctx context.Context) integration.HealthStatus {
	// If client is nil, integration hasn't been started or has been stopped
	if g.client == nil {
		return integration.Stopped
	}

	// If using secret ref, check if token is available
	if g.secretWatcher != nil && !g.secretWatcher.IsHealthy() {
		g.logger.Warn("Integration degraded: SecretWatcher has no valid token")
		g.setHealthStatus(integration.Degraded)
		return integration.Degraded
	}

	// Test connectivity
	if err := g.testConnection(ctx); err != nil {
		g.setHealthStatus(integration.Degraded)
		return integration.Degraded
	}

	g.setHealthStatus(integration.Healthy)
	return integration.Healthy
}

// RegisterTools registers MCP tools with the server for this integration instance.
// Placeholder - tools will be registered in Phase 18.
func (g *GrafanaIntegration) RegisterTools(registry integration.ToolRegistry) error {
	g.logger.Info("Grafana MCP tools registration placeholder (tools will be added in Phase 18)")
	// Phase 18 will implement:
	// - grafana_{name}_metrics_overview
	// - grafana_{name}_dashboard_list
	// - grafana_{name}_panel_query
	return nil
}

// testConnection tests connectivity to Grafana by executing minimal queries.
// Tests both dashboard access (required) and datasource access (optional, warns on failure).
func (g *GrafanaIntegration) testConnection(ctx context.Context) error {
	// Test 1: Dashboard read access (REQUIRED)
	dashboards, err := g.client.ListDashboards(ctx)
	if err != nil {
		return fmt.Errorf("dashboard access test failed: %w", err)
	}
	g.logger.Debug("Dashboard access test passed: found %d dashboards", len(dashboards))

	// Test 2: Datasource access (OPTIONAL - warn on failure, don't block)
	datasources, err := g.client.ListDatasources(ctx)
	if err != nil {
		g.logger.Warn("Datasource access test failed (non-blocking): %v", err)
		// Continue - datasource access is not critical for initial connectivity
	} else {
		g.logger.Debug("Datasource access test passed: found %d datasources", len(datasources))
	}

	return nil
}

// setHealthStatus updates the health status in a thread-safe manner.
func (g *GrafanaIntegration) setHealthStatus(status integration.HealthStatus) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.healthStatus = status
}

// getHealthStatus retrieves the health status in a thread-safe manner.
func (g *GrafanaIntegration) getHealthStatus() integration.HealthStatus {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.healthStatus
}

// getCurrentNamespace reads the namespace from the ServiceAccount mount.
// This file is automatically mounted by Kubernetes in all pods at a well-known path.
func getCurrentNamespace() (string, error) {
	const namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	data, err := os.ReadFile(namespaceFile)
	if err != nil {
		return "", fmt.Errorf("failed to read namespace file: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}
