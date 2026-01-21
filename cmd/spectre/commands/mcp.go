package commands

import (
	"os"

	"github.com/moolen/spectre/internal/logging"
	"github.com/spf13/cobra"
)

var (
	spectreURL      string
	httpAddr        string
	transportType   string
	mcpEndpointPath string
	// integrationsConfigPath and minIntegrationVersion are shared with server.go
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the MCP server",
	Long: `Start the Model Context Protocol (MCP) server that exposes
Spectre functionality as MCP tools for AI assistants.

Supports two transport modes:
  - http: HTTP server mode (default, suitable for independent deployment)
  - stdio: Standard input/output mode (for subprocess-based MCP clients)

HTTP mode includes a /health endpoint for health checks.`,
	Run: runMCP,
}

func init() {
	mcpCmd.Flags().StringVar(&spectreURL, "spectre-url", getEnv("SPECTRE_URL", "http://localhost:8080"), "URL to Spectre API server")
	mcpCmd.Flags().StringVar(&httpAddr, "http-addr", getEnv("MCP_HTTP_ADDR", ":8082"), "HTTP server address (host:port)")
	mcpCmd.Flags().StringVar(&transportType, "transport", "http", "Transport type: http or stdio")
	mcpCmd.Flags().StringVar(&mcpEndpointPath, "mcp-endpoint", getEnv("MCP_ENDPOINT", "/mcp"), "HTTP endpoint path for MCP requests")
	mcpCmd.Flags().StringVar(&integrationsConfigPath, "integrations-config", "integrations.yaml", "Path to integrations configuration YAML file")
	mcpCmd.Flags().StringVar(&minIntegrationVersion, "min-integration-version", "", "Minimum required integration version for validation (optional)")
}

func runMCP(cmd *cobra.Command, args []string) {
	// Set up logging
	if err := setupLog(logLevelFlags); err != nil {
		HandleError(err, "Failed to setup logging")
	}
	logger := logging.GetLogger("mcp")

	// Standalone MCP server is no longer supported - HTTP client was removed in Phase 7
	logger.Fatal("Standalone MCP server is no longer supported. Use 'spectre server' command instead (MCP is integrated on port 8080).")
}

// getEnv returns environment variable value or default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
