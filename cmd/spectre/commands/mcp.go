package commands

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/mcp"
	"github.com/spf13/cobra"
)

var (
	spectreURL      string
	httpAddr        string
	transportType   string
	mcpEndpointPath string
	// Graph configuration
	mcpGraphEnabled bool
	mcpGraphHost    string
	mcpGraphPort    int
	mcpGraphName    string
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

	// Graph reasoning layer configuration
	mcpCmd.Flags().BoolVar(&mcpGraphEnabled, "graph-enabled", getEnvBool("GRAPH_ENABLED", false), "Enable graph-based reasoning tools")
	mcpCmd.Flags().StringVar(&mcpGraphHost, "graph-host", getEnv("GRAPH_HOST", "localhost"), "FalkorDB host")
	mcpCmd.Flags().IntVar(&mcpGraphPort, "graph-port", getEnvInt("GRAPH_PORT", 6379), "FalkorDB port")
	mcpCmd.Flags().StringVar(&mcpGraphName, "graph-name", getEnv("GRAPH_NAME", "spectre"), "FalkorDB graph name")
}

func runMCP(cmd *cobra.Command, args []string) {
	// Set up logging
	if err := setupLog(GetLogLevel()); err != nil {
		HandleError(err, "Failed to setup logging")
	}
	logger := logging.GetLogger("mcp")
	logger.Info("Starting Spectre MCP Server (transport: %s)", transportType)
	logger.Info("Connecting to Spectre API at %s", spectreURL)

	// Create Spectre MCP server with optional graph support
	var spectreServer *mcp.SpectreServer
	var err error

	if mcpGraphEnabled {
		logger.Info("Graph reasoning layer enabled - configuring graph tools")
		graphConfig := &graph.ClientConfig{
			Host:         mcpGraphHost,
			Port:         mcpGraphPort,
			GraphName:    mcpGraphName,
			MaxRetries:   3,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
			PoolSize:     10,
		}

		spectreServer, err = mcp.NewSpectreServerWithOptions(mcp.ServerOptions{
			SpectreURL:  spectreURL,
			Version:     Version,
			GraphConfig: graphConfig,
			Logger:      logger,
		})
	} else {
		spectreServer, err = mcp.NewSpectreServerWithOptions(mcp.ServerOptions{
			SpectreURL: spectreURL,
			Version:    Version,
			Logger:     logger,
		})
	}

	if err != nil {
		logger.Fatal("Failed to create MCP server: %v", err)
	}

	logger.Info("Successfully connected to Spectre API")

	// Get the underlying mcp-go server
	mcpServer := spectreServer.GetMCPServer()

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
				os.Exit(1)
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

// getEnvBool returns environment variable value as bool or default
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}

// getEnvInt returns environment variable value as int or default
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
