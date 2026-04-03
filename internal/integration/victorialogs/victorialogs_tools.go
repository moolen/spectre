package victorialogs

import (
	"fmt"

	"github.com/moolen/spectre/internal/integration"
)

// RegisterTools registers MCP tools with the server for this integration instance.
func (v *VictoriaLogsIntegration) RegisterTools(registry integration.ToolRegistry) error {
	v.logger.Info("Registering VictoriaLogs MCP tools for instance: %s", v.name)

	v.registry = registry
	if v.client == nil || v.templateStore == nil {
		v.logger.Warn("Client or template store not initialized, skipping tool registration")
		return nil
	}

	toolCtx := ToolContext{
		Client:   v.client,
		Logger:   v.logger,
		Instance: v.name,
	}

	if err := v.registerOverviewTool(registry, toolCtx); err != nil {
		return err
	}
	if err := v.registerPatternsTool(registry, toolCtx); err != nil {
		return err
	}
	if err := v.registerLogsTool(registry, toolCtx); err != nil {
		return err
	}

	v.logger.Info("VictoriaLogs progressive disclosure tools registered: overview, patterns, logs")
	return nil
}

func (v *VictoriaLogsIntegration) registerOverviewTool(registry integration.ToolRegistry, toolCtx ToolContext) error {
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
	return nil
}

func (v *VictoriaLogsIntegration) registerPatternsTool(registry integration.ToolRegistry, toolCtx ToolContext) error {
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
	return nil
}

func (v *VictoriaLogsIntegration) registerLogsTool(registry integration.ToolRegistry, toolCtx ToolContext) error {
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
	return nil
}
