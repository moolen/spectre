// Package victorialogs provides VictoriaLogs integration for Spectre.
// This is a placeholder implementation for Phase 2 (Config Management & UI).
// Full implementation will be added in Phase 3 (VictoriaLogs Client & Basic Pipeline).
package victorialogs

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/moolen/spectre/internal/integration"
	"github.com/moolen/spectre/internal/logging"
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
	name    string
	url     string
	client  *http.Client
	logger  *logging.Logger
	healthy bool
}

// NewVictoriaLogsIntegration creates a new VictoriaLogs integration instance.
func NewVictoriaLogsIntegration(name string, config map[string]interface{}) (integration.Integration, error) {
	url, ok := config["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("victorialogs integration requires 'url' in config")
	}

	return &VictoriaLogsIntegration{
		name: name,
		url:  url,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger:  logging.GetLogger("integration.victorialogs." + name),
		healthy: false,
	}, nil
}

// Metadata returns the integration's identifying information.
func (v *VictoriaLogsIntegration) Metadata() integration.IntegrationMetadata {
	return integration.IntegrationMetadata{
		Name:        v.name,
		Version:     "0.1.0", // Placeholder version for Phase 2
		Description: "VictoriaLogs log aggregation integration",
		Type:        "victorialogs",
	}
}

// Start initializes the integration and validates connectivity.
func (v *VictoriaLogsIntegration) Start(ctx context.Context) error {
	v.logger.Info("Starting VictoriaLogs integration: %s (url: %s)", v.name, v.url)

	// Test connectivity by checking the health endpoint
	if err := v.checkHealth(ctx); err != nil {
		v.healthy = false
		return fmt.Errorf("failed to connect to VictoriaLogs at %s: %w", v.url, err)
	}

	v.healthy = true
	v.logger.Info("VictoriaLogs integration started successfully")
	return nil
}

// Stop gracefully shuts down the integration.
func (v *VictoriaLogsIntegration) Stop(ctx context.Context) error {
	v.logger.Info("Stopping VictoriaLogs integration: %s", v.name)
	v.healthy = false
	return nil
}

// Health returns the current health status.
func (v *VictoriaLogsIntegration) Health(ctx context.Context) integration.HealthStatus {
	if !v.healthy {
		return integration.Degraded
	}

	// Quick health check
	if err := v.checkHealth(ctx); err != nil {
		v.healthy = false
		return integration.Degraded
	}

	return integration.Healthy
}

// RegisterTools registers MCP tools with the server for this integration instance.
// Phase 3 will implement actual log query tools.
func (v *VictoriaLogsIntegration) RegisterTools(registry integration.ToolRegistry) error {
	// Placeholder - no tools implemented yet
	// Phase 5 will add progressive disclosure tools:
	// - victorialogs_overview: Global overview of log patterns
	// - victorialogs_patterns: Aggregated log templates with counts
	// - victorialogs_logs: Raw log details for specific scope
	v.logger.Info("VictoriaLogs tools registration (placeholder - no tools yet)")
	return nil
}

// checkHealth performs a health check against the VictoriaLogs instance.
func (v *VictoriaLogsIntegration) checkHealth(ctx context.Context) error {
	// VictoriaLogs exposes a health endpoint at /health
	healthURL := v.url + "/health"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create health request: %w", err)
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}
