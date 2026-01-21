// Package victorialogs provides VictoriaLogs integration for Spectre.
package victorialogs

import (
	"context"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/integration"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/logprocessing"
	"github.com/prometheus/client_golang/prometheus"
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
	url           string
	client        *Client                      // VictoriaLogs HTTP client
	pipeline      *Pipeline                    // Backpressure-aware ingestion pipeline
	metrics       *Metrics                     // Prometheus metrics for observability
	logger        *logging.Logger
	registry      integration.ToolRegistry     // MCP tool registry for dynamic tool registration
	templateStore *logprocessing.TemplateStore // Template store for pattern mining
}

// NewVictoriaLogsIntegration creates a new VictoriaLogs integration instance.
// Note: Client, pipeline, and metrics are initialized in Start() to follow lifecycle pattern.
func NewVictoriaLogsIntegration(name string, config map[string]interface{}) (integration.Integration, error) {
	url, ok := config["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("victorialogs integration requires 'url' in config")
	}

	return &VictoriaLogsIntegration{
		name:          name,
		url:           url,
		client:        nil, // Initialized in Start()
		pipeline:      nil, // Initialized in Start()
		metrics:       nil, // Initialized in Start()
		templateStore: nil, // Initialized in Start()
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
	v.logger.Info("Starting VictoriaLogs integration: %s (url: %s)", v.name, v.url)

	// Create Prometheus metrics (registers with global registry)
	v.metrics = NewMetrics(prometheus.DefaultRegisterer, v.name)

	// Create HTTP client with 30-second query timeout
	v.client = NewClient(v.url, 30*time.Second)

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
	}

	v.logger.Info("VictoriaLogs integration started successfully")
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

	// Clear references
	v.client = nil
	v.pipeline = nil
	v.metrics = nil
	v.templateStore = nil

	v.logger.Info("VictoriaLogs integration stopped")
	return nil
}

// Health returns the current health status.
func (v *VictoriaLogsIntegration) Health(ctx context.Context) integration.HealthStatus {
	// If client is nil, integration hasn't been started or has been stopped
	if v.client == nil {
		return integration.Stopped
	}

	// Test connectivity
	if err := v.testConnection(ctx); err != nil {
		return integration.Degraded
	}

	return integration.Healthy
}

// RegisterTools registers MCP tools with the server for this integration instance.
func (v *VictoriaLogsIntegration) RegisterTools(registry integration.ToolRegistry) error {
	v.logger.Info("Registering VictoriaLogs MCP tools for instance: %s", v.name)

	// Store registry reference for future tool implementations (Plans 3-4)
	v.registry = registry

	// Check if client is initialized (might be nil if integration is stopped or degraded)
	if v.client == nil {
		v.logger.Warn("Client not initialized, skipping tool registration")
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
	if err := registry.RegisterTool(overviewName, overviewTool.Execute); err != nil {
		return fmt.Errorf("failed to register overview tool: %w", err)
	}
	v.logger.Info("Registered tool: %s", overviewName)

	// TODO Phase 5 Plan 3: Register patterns tool (victorialogs_{name}_patterns)
	// TODO Phase 5 Plan 4: Register logs tool (victorialogs_{name}_logs)

	v.logger.Info("VictoriaLogs tools registration complete")
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
