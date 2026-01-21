package commands

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/moolen/spectre/internal/config"
	"github.com/moolen/spectre/internal/integration"
	// Import integration implementations to register their factories
	_ "github.com/moolen/spectre/internal/integration/victorialogs"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/mcp"
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
	logger.Info("Starting Spectre MCP Server (transport: %s)", transportType)
	logger.Info("Connecting to Spectre API at %s", spectreURL)

	// Create Spectre MCP server
	spectreServer, err := mcp.NewSpectreServerWithOptions(mcp.ServerOptions{
		SpectreURL: spectreURL,
		Version:    Version,
		Logger:     logger,
	})

	if err != nil {
		logger.Fatal("Failed to create MCP server: %v", err)
	}

	logger.Info("Successfully connected to Spectre API")

	// Get the underlying mcp-go server
	mcpServer := spectreServer.GetMCPServer()

	// Initialize integration manager with MCP tool registry
	var integrationMgr *integration.Manager
	if integrationsConfigPath != "" {
		// Create default config file if it doesn't exist
		if _, err := os.Stat(integrationsConfigPath); os.IsNotExist(err) {
			logger.Info("Creating default integrations config file: %s", integrationsConfigPath)
			defaultConfig := &config.IntegrationsFile{
				SchemaVersion: "v1",
				Instances:     []config.IntegrationConfig{},
			}
			if err := config.WriteIntegrationsFile(integrationsConfigPath, defaultConfig); err != nil {
				logger.Error("Failed to create default integrations config: %v", err)
				HandleError(err, "Integration config creation error")
			}
		}

		logger.Info("Initializing integration manager from: %s", integrationsConfigPath)

		// Create MCPToolRegistry adapter
		mcpRegistry := mcp.NewMCPToolRegistry(mcpServer)

		// Create integration manager with MCP registry
		var err error
		integrationMgr, err = integration.NewManagerWithMCPRegistry(integration.ManagerConfig{
			ConfigPath:            integrationsConfigPath,
			MinIntegrationVersion: minIntegrationVersion,
		}, mcpRegistry)
		if err != nil {
			logger.Error("Failed to create integration manager: %v", err)
			HandleError(err, "Integration manager initialization error")
		}

		logger.Info("Integration manager created with MCP tool registry")
	}

	// Set up signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Info("Received signal: %v, shutting down gracefully...", sig)
		cancel()
	}()

	// Start integration manager (this calls RegisterTools for each integration)
	if integrationMgr != nil {
		if err := integrationMgr.Start(ctx); err != nil {
			logger.Error("Failed to start integration manager: %v", err)
			HandleError(err, "Integration manager startup error")
		}
		logger.Info("Integration manager started, tools registered")
	}

	// Start appropriate transport
	switch transportType {
	case "http":
		// Ensure endpoint path starts with /
		endpointPath := mcpEndpointPath
		if endpointPath == "" {
			endpointPath = "/mcp"
		} else if endpointPath[0] != '/' {
			endpointPath = "/" + endpointPath
		}

		logger.Info("Starting HTTP server on %s (endpoint: %s)", httpAddr, endpointPath)

		// Create custom mux with health endpoint
		mux := http.NewServeMux()

		// Add health endpoint
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("ok"))
		})

		// Create StreamableHTTP server with stateless session management
		// This is important for compatibility with clients that don't manage sessions
		streamableServer := server.NewStreamableHTTPServer(
			mcpServer,
			server.WithEndpointPath(endpointPath),
			server.WithStateLess(true), // Enable stateless mode for backward compatibility
		)

		// Register MCP handler at the endpoint path
		mux.Handle(endpointPath, streamableServer)

		// Create HTTP server with our custom mux
		httpSrv := &http.Server{
			Addr:              httpAddr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second, // Prevent Slowloris attacks
		}

		// Provide custom HTTP server to streamable server
		// (we need to recreate it with the custom server option)
		streamableServer = server.NewStreamableHTTPServer(
			mcpServer,
			server.WithEndpointPath(endpointPath),
			server.WithStateLess(true), // Enable stateless mode
			server.WithStreamableHTTPServer(httpSrv),
		)

		// Start server in goroutine
		errCh := make(chan error, 1)
		go func() {
			if err := streamableServer.Start(httpAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
		}()

		// Wait for shutdown signal or error
		select {
		case <-ctx.Done():
			logger.Info("Shutting down HTTP server...")
			// Use a timeout context for shutdown (don't hang forever)
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()

			if err := streamableServer.Shutdown(shutdownCtx); err != nil {
				logger.Error("Error during shutdown: %v", err)
				// Force exit if graceful shutdown fails
				shutdownCancel() // Call explicitly before exit
				os.Exit(1) //nolint:gocritic // shutdownCancel() is explicitly called on line 153
			}

			// Stop integration manager
			if integrationMgr != nil {
				logger.Info("Stopping integration manager...")
				if err := integrationMgr.Stop(shutdownCtx); err != nil {
					logger.Error("Error stopping integration manager: %v", err)
				}
			}
		case err := <-errCh:
			logger.Error("Server error: %v", err)
			os.Exit(1)
		}

	case "stdio":
		logger.Info("Starting stdio transport")
		if err := server.ServeStdio(mcpServer); err != nil {
			logger.Error("Stdio transport error: %v", err)
		}

		// Stop integration manager after stdio transport ends
		if integrationMgr != nil {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			logger.Info("Stopping integration manager...")
			if err := integrationMgr.Stop(shutdownCtx); err != nil {
				logger.Error("Error stopping integration manager: %v", err)
			}
		}

	default:
		logger.Fatal("Invalid transport type: %s (must be 'http' or 'stdio')", transportType)
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
