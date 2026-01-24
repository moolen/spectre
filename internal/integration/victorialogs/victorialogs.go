// Package victorialogs provides VictoriaLogs integration for Spectre.
package victorialogs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/integration"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/logprocessing"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func init() {
	// Register the VictoriaLogs factory with the global registry
	if err := integration.RegisterFactory("victorialogs", NewVictoriaLogsIntegration); err != nil {
		// Log but don't fail - factory might already be registered in tests
		logger := logging.GetLogger("integration.victorialogs")
		logger.Warn("Failed to register victorialogs factory: %v", err)
	}
}

// VictoriaLogsIntegration implements the Integration interface for VictoriaLogs.
type VictoriaLogsIntegration struct {
	name          string
	config        Config                       // Full configuration (includes URL and SecretRef)
	client        *Client                      // VictoriaLogs HTTP client
	pipeline      *Pipeline                    // Backpressure-aware ingestion pipeline
	metrics       *Metrics                     // Prometheus metrics for observability
	logger        *logging.Logger
	registry      integration.ToolRegistry     // MCP tool registry for dynamic tool registration
	templateStore *logprocessing.TemplateStore // Template store for pattern mining
	secretWatcher *SecretWatcher               // Optional: manages API token from Kubernetes Secret
	healthStatus  integration.HealthStatus     // Cached health status
	mu            sync.RWMutex                 // Protects healthStatus
}

// NewVictoriaLogsIntegration creates a new VictoriaLogs integration instance.
// Note: Client, pipeline, and metrics are initialized in Start() to follow lifecycle pattern.
func NewVictoriaLogsIntegration(name string, configMap map[string]interface{}) (integration.Integration, error) {
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

	return &VictoriaLogsIntegration{
		name:          name,
		config:        config,
		client:        nil,                    // Initialized in Start()
		pipeline:      nil,                    // Initialized in Start()
		metrics:       nil,                    // Initialized in Start()
		templateStore: nil,                    // Initialized in Start()
		secretWatcher: nil,                    // Initialized in Start() if config uses SecretRef
		healthStatus:  integration.Stopped,    // Initial state
		logger:        logging.GetLogger("integration.victorialogs." + name),
	}, nil
}

// Metadata returns the integration's identifying information.
func (v *VictoriaLogsIntegration) Metadata() integration.IntegrationMetadata {
	return integration.IntegrationMetadata{
		Name:        v.name,
		Version:     "0.1.0",
		Description: "VictoriaLogs log aggregation integration",
		Type:        "victorialogs",
	}
}

// Start initializes the integration and validates connectivity.
func (v *VictoriaLogsIntegration) Start(ctx context.Context) error {
	v.logger.Info("Starting VictoriaLogs integration: %s (url: %s)", v.name, v.config.URL)

	// Create Prometheus metrics (registers with global registry)
	v.metrics = NewMetrics(prometheus.DefaultRegisterer, v.name)

	// Create SecretWatcher if config uses secret ref
	if v.config.UsesSecretRef() {
		v.logger.Info("Creating SecretWatcher for secret: %s, key: %s",
			v.config.APITokenRef.SecretName, v.config.APITokenRef.Key)

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
			v.config.APITokenRef.SecretName,
			v.config.APITokenRef.Key,
			v.logger,
		)
		if err != nil {
			return fmt.Errorf("failed to create secret watcher: %w", err)
		}

		// Start SecretWatcher
		if err := secretWatcher.Start(ctx); err != nil {
			return fmt.Errorf("failed to start secret watcher: %w", err)
		}

		v.secretWatcher = secretWatcher
		v.logger.Info("SecretWatcher started successfully")
	}

	// Create HTTP client (pass secretWatcher if exists)
	v.client = NewClient(v.config.URL, 60*time.Second, v.secretWatcher)

	// Create and start pipeline
	v.pipeline = NewPipeline(v.client, v.metrics, v.name)
	if err := v.pipeline.Start(ctx); err != nil {
		return fmt.Errorf("failed to start pipeline: %w", err)
	}

	// Create template store with default Drain config (from Phase 4)
	drainConfig := logprocessing.DrainConfig{
		LogClusterDepth: 4,
		SimTh:           0.4,
		MaxChildren:     100,
	}
	v.templateStore = logprocessing.NewTemplateStore(drainConfig)
	v.logger.Info("Template store initialized with Drain config: depth=%d, simTh=%.2f", drainConfig.LogClusterDepth, drainConfig.SimTh)

	// Test connectivity (warn on failure but continue - degraded state with auto-recovery)
	if err := v.testConnection(ctx); err != nil {
		v.logger.Warn("Failed initial connectivity test (will retry on health checks): %v", err)
		v.setHealthStatus(integration.Degraded)
	} else {
		v.setHealthStatus(integration.Healthy)
	}

	v.logger.Info("VictoriaLogs integration started successfully (health: %s)", v.getHealthStatus().String())
	return nil
}

// Stop gracefully shuts down the integration.
func (v *VictoriaLogsIntegration) Stop(ctx context.Context) error {
	v.logger.Info("Stopping VictoriaLogs integration: %s", v.name)

	// Stop pipeline if it exists
	if v.pipeline != nil {
		if err := v.pipeline.Stop(ctx); err != nil {
			v.logger.Error("Error stopping pipeline: %v", err)
			// Continue with shutdown even if pipeline stop fails
		}
	}

	// Stop secret watcher if it exists
	if v.secretWatcher != nil {
		if err := v.secretWatcher.Stop(); err != nil {
			v.logger.Error("Error stopping secret watcher: %v", err)
		}
	}

	// Unregister metrics before clearing reference to avoid duplicate registration on restart
	if v.metrics != nil {
		v.metrics.Unregister()
	}

	// Clear references
	v.client = nil
	v.pipeline = nil
	v.metrics = nil
	v.templateStore = nil
	v.secretWatcher = nil
	v.setHealthStatus(integration.Stopped)

	v.logger.Info("VictoriaLogs integration stopped")
	return nil
}

// Health returns the current cached health status.
// This method is called frequently (e.g., SSE polling every 2s) so it returns
// cached status rather than testing connectivity. Actual connectivity tests
// happen during Start() and periodic health checks by the integration manager.
func (v *VictoriaLogsIntegration) Health(ctx context.Context) integration.HealthStatus {
	// If client is nil, integration hasn't been started or has been stopped
	if v.client == nil {
		return integration.Stopped
	}

	// If using secret ref, check if token is available
	if v.secretWatcher != nil && !v.secretWatcher.IsHealthy() {
		v.setHealthStatus(integration.Degraded)
		return integration.Degraded
	}

	// Return cached health status - connectivity is tested by manager's periodic health checks
	return v.getHealthStatus()
}

// CheckConnectivity implements integration.ConnectivityChecker.
// Called by the manager during periodic health checks (every 30s) to verify actual connectivity.
func (v *VictoriaLogsIntegration) CheckConnectivity(ctx context.Context) error {
	if v.client == nil {
		v.setHealthStatus(integration.Stopped)
		return fmt.Errorf("client not initialized")
	}

	if err := v.testConnection(ctx); err != nil {
		v.setHealthStatus(integration.Degraded)
		return err
	}

	v.setHealthStatus(integration.Healthy)
	return nil
}

// setHealthStatus updates the health status in a thread-safe manner.
func (v *VictoriaLogsIntegration) setHealthStatus(status integration.HealthStatus) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.healthStatus = status
}

// getHealthStatus retrieves the health status in a thread-safe manner.
func (v *VictoriaLogsIntegration) getHealthStatus() integration.HealthStatus {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.healthStatus
}

// RegisterTools registers MCP tools with the server for this integration instance.
func (v *VictoriaLogsIntegration) RegisterTools(registry integration.ToolRegistry) error {
	v.logger.Info("Registering VictoriaLogs MCP tools for instance: %s", v.name)

	// Store registry reference
	v.registry = registry

	// Check if client and template store are initialized
	if v.client == nil || v.templateStore == nil {
		v.logger.Warn("Client or template store not initialized, skipping tool registration")
		return nil
	}

	// Create tool context shared across all tools
	toolCtx := ToolContext{
		Client:   v.client,
		Logger:   v.logger,
		Instance: v.name,
	}

	// Register overview tool: victorialogs_{name}_overview
	overviewTool := &OverviewTool{ctx: toolCtx}
	overviewName := fmt.Sprintf("victorialogs_%s_overview", v.name)
	overviewSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"start_time": map[string]interface{}{
				"type":        "integer",
				"description": "Start timestamp (Unix seconds or milliseconds). Default: 1 hour ago",
			},
			"end_time": map[string]interface{}{
				"type":        "integer",
				"description": "End timestamp (Unix seconds or milliseconds). Default: now",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Optional: filter to specific Kubernetes namespace",
			},
		},
	}
	if err := registry.RegisterTool(overviewName, "Get global overview of log volume and severity counts by namespace", overviewTool.Execute, overviewSchema); err != nil {
		return fmt.Errorf("failed to register overview tool: %w", err)
	}
	v.logger.Info("Registered tool: %s", overviewName)

	// Register patterns tool: victorialogs_{name}_patterns
	patternsTool := &PatternsTool{
		ctx:           toolCtx,
		templateStore: v.templateStore,
	}
	patternsName := fmt.Sprintf("victorialogs_%s_patterns", v.name)
	patternsSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace to query (required)",
			},
			"severity": map[string]interface{}{
				"type":        "string",
				"description": "Optional: filter by severity level (error, warn). Only logs matching the severity pattern will be processed.",
				"enum":        []string{"error", "warn"},
			},
			"start_time": map[string]interface{}{
				"type":        "integer",
				"description": "Start timestamp (Unix seconds or milliseconds). Default: 1 hour ago",
			},
			"end_time": map[string]interface{}{
				"type":        "integer",
				"description": "End timestamp (Unix seconds or milliseconds). Default: now",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Max templates to return (default 50)",
			},
		},
		"required": []string{"namespace"},
	}
	if err := registry.RegisterTool(patternsName, "Get aggregated log patterns with novelty detection for a namespace", patternsTool.Execute, patternsSchema); err != nil {
		return fmt.Errorf("failed to register patterns tool: %w", err)
	}
	v.logger.Info("Registered tool: %s", patternsName)

	// Register logs tool: victorialogs_{name}_logs
	logsTool := &LogsTool{ctx: toolCtx}
	logsName := fmt.Sprintf("victorialogs_%s_logs", v.name)
	logsSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Kubernetes namespace to query (required)",
			},
			"start_time": map[string]interface{}{
				"type":        "integer",
				"description": "Start timestamp (Unix seconds or milliseconds). Default: 1 hour ago",
			},
			"end_time": map[string]interface{}{
				"type":        "integer",
				"description": "End timestamp (Unix seconds or milliseconds). Default: now",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Max logs to return (default 100, max 500)",
			},
			"level": map[string]interface{}{
				"type":        "string",
				"description": "Optional: filter by log level (error, warn, info, debug)",
			},
			"pod": map[string]interface{}{
				"type":        "string",
				"description": "Optional: filter by pod name",
			},
			"container": map[string]interface{}{
				"type":        "string",
				"description": "Optional: filter by container name",
			},
		},
		"required": []string{"namespace"},
	}
	if err := registry.RegisterTool(logsName, "Get raw logs from a namespace with optional filters", logsTool.Execute, logsSchema); err != nil {
		return fmt.Errorf("failed to register logs tool: %w", err)
	}
	v.logger.Info("Registered tool: %s", logsName)

	v.logger.Info("VictoriaLogs progressive disclosure tools registered: overview, patterns, logs")
	return nil
}

// testConnection tests connectivity to VictoriaLogs by executing a minimal query.
func (v *VictoriaLogsIntegration) testConnection(ctx context.Context) error {
	// Create test query params with default time range and minimal limit
	params := QueryParams{
		TimeRange: DefaultTimeRange(),
		Limit:     1,
	}

	// Execute test query
	_, err := v.client.QueryLogs(ctx, params)
	if err != nil {
		return fmt.Errorf("connectivity test failed: %w", err)
	}

	return nil
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
