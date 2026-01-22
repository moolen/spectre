// Package logzio provides Logz.io integration for Spectre.
package logzio

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/integration"
	"github.com/moolen/spectre/internal/integration/victorialogs"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/logprocessing"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func init() {
	// Register the Logz.io factory with the global registry
	if err := integration.RegisterFactory("logzio", NewLogzioIntegration); err != nil {
		// Log but don't fail - factory might already be registered in tests
		logger := logging.GetLogger("integration.logzio")
		logger.Warn("Failed to register logzio factory: %v", err)
	}
}

// LogzioIntegration implements the Integration interface for Logz.io.
type LogzioIntegration struct {
	name          string
	config        Config                              // Full configuration (includes Region and SecretRef)
	client        *Client                             // Logz.io HTTP client
	logger        *logging.Logger
	registry      integration.ToolRegistry            // MCP tool registry for dynamic tool registration
	secretWatcher *victorialogs.SecretWatcher         // Optional: manages API token from Kubernetes Secret
	templateStore *logprocessing.TemplateStore        // Template store for pattern mining
}

// NewLogzioIntegration creates a new Logz.io integration instance.
// Note: Client is initialized in Start() to follow lifecycle pattern.
func NewLogzioIntegration(name string, configMap map[string]interface{}) (integration.Integration, error) {
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

	return &LogzioIntegration{
		name:          name,
		config:        config,
		client:        nil,        // Initialized in Start()
		secretWatcher: nil,        // Initialized in Start() if config uses SecretRef
		logger:        logging.GetLogger("integration.logzio." + name),
	}, nil
}

// Metadata returns the integration's identifying information.
func (l *LogzioIntegration) Metadata() integration.IntegrationMetadata {
	return integration.IntegrationMetadata{
		Name:        l.name,
		Version:     "0.1.0",
		Description: "Logz.io log aggregation integration",
		Type:        "logzio",
	}
}

// Start initializes the integration and validates connectivity.
func (l *LogzioIntegration) Start(ctx context.Context) error {
	l.logger.Info("Starting Logz.io integration: %s (region: %s, baseURL: %s)",
		l.name, l.config.Region, l.config.GetBaseURL())

	// Create SecretWatcher if config uses secret ref
	if l.config.UsesSecretRef() {
		l.logger.Info("Creating SecretWatcher for secret: %s, key: %s",
			l.config.APITokenRef.SecretName, l.config.APITokenRef.Key)

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
		secretWatcher, err := victorialogs.NewSecretWatcher(
			clientset,
			namespace,
			l.config.APITokenRef.SecretName,
			l.config.APITokenRef.Key,
			l.logger,
		)
		if err != nil {
			return fmt.Errorf("failed to create secret watcher: %w", err)
		}

		// Start SecretWatcher
		if err := secretWatcher.Start(ctx); err != nil {
			return fmt.Errorf("failed to start secret watcher: %w", err)
		}

		l.secretWatcher = secretWatcher
		l.logger.Info("SecretWatcher started successfully")
	}

	// Create HTTP client with 30s timeout
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create Logz.io client wrapper
	l.client = NewClient(l.config.GetBaseURL(), httpClient, l.secretWatcher, l.logger)

	// Initialize template store for pattern mining
	l.templateStore = logprocessing.NewTemplateStore(logprocessing.DefaultDrainConfig())
	l.logger.Info("Template store initialized for pattern mining")

	l.logger.Info("Logz.io integration started successfully")
	return nil
}

// Stop gracefully shuts down the integration.
func (l *LogzioIntegration) Stop(ctx context.Context) error {
	l.logger.Info("Stopping Logz.io integration: %s", l.name)

	// Stop secret watcher if it exists
	if l.secretWatcher != nil {
		if err := l.secretWatcher.Stop(); err != nil {
			l.logger.Error("Error stopping secret watcher: %v", err)
		}
	}

	// Clear references
	l.client = nil
	l.secretWatcher = nil

	l.logger.Info("Logz.io integration stopped")
	return nil
}

// Health returns the current health status.
func (l *LogzioIntegration) Health(ctx context.Context) integration.HealthStatus {
	// If client is nil, integration hasn't been started or has been stopped
	if l.client == nil {
		return integration.Stopped
	}

	// If using secret ref, check if token is available
	if l.secretWatcher != nil && !l.secretWatcher.IsHealthy() {
		l.logger.Warn("Integration degraded: SecretWatcher has no valid token")
		return integration.Degraded
	}

	// Token is available (or not using secret ref), integration is healthy
	return integration.Healthy
}

// RegisterTools registers MCP tools with the server for this integration instance.
func (l *LogzioIntegration) RegisterTools(registry integration.ToolRegistry) error {
	l.logger.Info("Registering MCP tools for Logz.io integration: %s", l.name)

	// Store registry reference
	l.registry = registry

	// Create tool context for dependency injection
	toolCtx := ToolContext{
		Client:   l.client,
		Logger:   l.logger,
		Instance: l.name,
	}

	// Instantiate tools
	overviewTool := &OverviewTool{ctx: toolCtx}
	logsTool := &LogsTool{ctx: toolCtx}
	patternsTool := &PatternsTool{
		ctx:           toolCtx,
		templateStore: l.templateStore,
	}

	// Register overview tool
	overviewName := fmt.Sprintf("logzio_%s_overview", l.name)
	overviewDesc := fmt.Sprintf("Get overview of log volume and severity by namespace for Logz.io %s. Returns namespace-level error, warning, and total log counts. Use this first to identify namespaces with high error rates before drilling into specific logs.", l.name)
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
				"description": "Optional: filter to specific namespace",
			},
		},
	}

	if err := registry.RegisterTool(overviewName, overviewDesc, overviewTool.Execute, overviewSchema); err != nil {
		return fmt.Errorf("failed to register overview tool: %w", err)
	}
	l.logger.Info("Registered tool: %s", overviewName)

	// Register logs tool
	logsName := fmt.Sprintf("logzio_%s_logs", l.name)
	logsDesc := fmt.Sprintf("Retrieve raw logs from Logz.io %s with filters. Namespace is required. Returns up to 100 log entries. Use after overview to investigate specific namespaces or errors.", l.name)
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
				"description": "Maximum logs to return (default: 100, max: 100)",
			},
			"level": map[string]interface{}{
				"type":        "string",
				"description": "Filter by log level (e.g., error, warn, info)",
			},
			"pod": map[string]interface{}{
				"type":        "string",
				"description": "Filter by pod name",
			},
			"container": map[string]interface{}{
				"type":        "string",
				"description": "Filter by container name",
			},
		},
		"required": []interface{}{"namespace"},
	}

	if err := registry.RegisterTool(logsName, logsDesc, logsTool.Execute, logsSchema); err != nil {
		return fmt.Errorf("failed to register logs tool: %w", err)
	}
	l.logger.Info("Registered tool: %s", logsName)

	// Register patterns tool
	patternsName := fmt.Sprintf("logzio_%s_patterns", l.name)
	patternsDesc := fmt.Sprintf("Get aggregated log patterns with novelty detection for Logz.io %s. Returns log templates with occurrence counts. Use after overview to understand error patterns.", l.name)
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

	if err := registry.RegisterTool(patternsName, patternsDesc, patternsTool.Execute, patternsSchema); err != nil {
		return fmt.Errorf("failed to register patterns tool: %w", err)
	}
	l.logger.Info("Registered tool: %s", patternsName)

	l.logger.Info("Successfully registered 3 MCP tools for Logz.io integration: %s", l.name)
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
