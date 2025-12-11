package commands

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/mcp"
	httpTransport "github.com/moolen/spectre/internal/mcp/transport/http"
	stdioTransport "github.com/moolen/spectre/internal/mcp/transport/stdio"
	"github.com/spf13/cobra"
)

var (
	spectreURL     string
	httpAddr       string
	transportType  string
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start the MCP server",
	Long: `Start the Model Context Protocol (MCP) server that exposes
Spectre functionality as MCP tools for AI assistants.

Supports two transport modes:
  - http: HTTP server mode (default, suitable for independent deployment)
  - stdio: Standard input/output mode (for subprocess-based MCP clients)`,
	Run: runMCP,
}

func init() {
	mcpCmd.Flags().StringVar(&spectreURL, "spectre-url", getEnv("SPECTRE_URL", "http://localhost:8080"), "URL to Spectre API server")
	mcpCmd.Flags().StringVar(&httpAddr, "http-addr", getEnv("MCP_HTTP_ADDR", ":8081"), "HTTP server address (host:port)")
	mcpCmd.Flags().StringVar(&transportType, "transport", "http", "Transport type: http or stdio")
}

func runMCP(cmd *cobra.Command, args []string) {
	// Set up logging
	if err := setupLog(GetLogLevel()); err != nil {
		HandleError(err, "Failed to setup logging")
	}
	logger := logging.GetLogger("mcp")
	logger.Info("Starting Spectre MCP Server (transport: %s)", transportType)
	logger.Info("Connecting to Spectre API at %s", spectreURL)

	// Create MCP server
	server, err := mcp.NewMCPServer(spectreURL)
	if err != nil {
		logger.Fatal("Failed to create MCP server: %v", err)
	}

	logger.Info("Successfully connected to Spectre API")

	// Create transport based on type
	switch transportType {
	case "http":
		runHTTPTransport(server, logger)
	case "stdio":
		runStdioTransport(server, logger)
	default:
		logger.Fatal("Invalid transport type: %s (must be 'http' or 'stdio')", transportType)
	}
}

func runHTTPTransport(server *mcp.MCPServer, logger *logging.Logger) {
	// Create HTTP transport
	transport := httpTransport.NewTransport(httpAddr, server, Version)

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Create context that cancels on signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server in a goroutine
	go func() {
		if err := transport.Start(ctx); err != nil {
			logger.Fatal("Failed to start HTTP transport: %v", err)
		}
	}()

	// Wait for shutdown signal
	sig := <-sigCh
	logger.Info("Received signal: %v, shutting down gracefully...", sig)

	// Cancel context to trigger shutdown
	cancel()

	// Stop the transport
	if err := transport.Stop(); err != nil {
		logger.Error("Error during shutdown: %v", err)
	}

	logger.Info("Server stopped")
}

func runStdioTransport(server *mcp.MCPServer, logger *logging.Logger) {
	// Create stdio transport
	transport := stdioTransport.NewTransport(server, Version)

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Create context that cancels on signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start transport in a goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := transport.Start(ctx); err != nil && err != context.Canceled {
			errCh <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case sig := <-sigCh:
		logger.Info("Received signal: %v, shutting down gracefully...", sig)
		cancel()
	case err := <-errCh:
		if err != nil {
			logger.Error("Stdio transport error: %v", err)
		}
	}

	// Stop the transport
	if err := transport.Stop(); err != nil {
		logger.Error("Error during shutdown: %v", err)
	}

	logger.Info("Server stopped")
}

// getEnv returns environment variable value or default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
