package apiserver

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/server"
	namespacegraph "github.com/moolen/spectre/internal/analysis/namespace_graph"
	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graph/sync"
	"github.com/moolen/spectre/internal/integration"
	"github.com/moolen/spectre/internal/logging"
	"go.opentelemetry.io/otel/trace"
)

// ReadinessChecker is an interface for checking component readiness
type ReadinessChecker interface {
	IsReady() bool
}

// NoOpReadinessChecker is a ReadinessChecker that always returns true.
// Use this when no readiness checking is needed (e.g., when watcher is disabled).
type NoOpReadinessChecker struct{}

// IsReady always returns true for the no-op checker.
func (n *NoOpReadinessChecker) IsReady() bool {
	return true
}

// Server handles HTTP API requests and Connect RPC requests
type Server struct {
	port             int
	server           *http.Server
	logger           *logging.Logger
	queryExecutor    api.QueryExecutor
	graphExecutor    api.QueryExecutor       // Graph-based query executor
	querySource      api.TimelineQuerySource // Which executor to use for timeline queries
	graphClient      graph.Client
	graphPipeline    sync.Pipeline               // Graph sync pipeline for imports
	timelineService  *api.TimelineService        // Shared timeline service for REST handlers and MCP tools
	metadataCache    *api.MetadataCache          // In-memory metadata cache for fast responses
	nsGraphCache     *namespacegraph.Cache       // In-memory namespace graph cache for fast responses
	staticCache      *staticFileCache            // In-memory static file cache for fast UI serving
	router           *http.ServeMux
	readinessChecker ReadinessChecker
	tracingProvider  interface {
		GetTracer(string) trace.Tracer
		IsEnabled() bool
	}
	// Integration config management
	integrationsConfigPath string
	integrationManager     *integration.Manager
	// MCP server
	mcpServer *server.MCPServer
}

// NamespaceGraphCacheConfig holds configuration for the namespace graph cache
type NamespaceGraphCacheConfig struct {
	Enabled     bool
	RefreshTTL  time.Duration
	MaxMemoryMB int64
}

// NewWithStorageGraphAndPipeline creates a new API server with graph query executor and pipeline support
func NewWithStorageGraphAndPipeline(
	port int,
	storageExecutor api.QueryExecutor, // Can be nil - not used in graph-only mode
	graphExecutor api.QueryExecutor,
	querySource api.TimelineQuerySource,
	storage interface{}, // Can be nil - kept for signature compatibility but not used
	graphClient graph.Client,
	graphPipeline sync.Pipeline, // Graph pipeline for imports
	readinessChecker ReadinessChecker,
	tracingProvider interface {
		GetTracer(string) trace.Tracer
		IsEnabled() bool
	},
	metadataRefreshPeriod time.Duration, // How often to refresh the metadata cache
	nsGraphCacheConfig NamespaceGraphCacheConfig, // Namespace graph cache configuration
	integrationsConfigPath string,         // Path to integrations config file (optional)
	integrationManager *integration.Manager, // Integration manager (optional)
	mcpServer *server.MCPServer, // MCP server for /v1/mcp endpoint (optional)
) *Server {
	s := &Server{
		port:                   port,
		logger:                 logging.GetLogger("api"),
		queryExecutor:          storageExecutor,
		graphExecutor:          graphExecutor,
		querySource:            querySource,
		graphClient:            graphClient,
		graphPipeline:          graphPipeline,
		router:                 http.NewServeMux(),
		readinessChecker:       readinessChecker,
		tracingProvider:        tracingProvider,
		integrationsConfigPath: integrationsConfigPath,
		integrationManager:     integrationManager,
		mcpServer:              mcpServer,
	}

	// Create metadata cache if we have a query executor
	// Use graph executor if available (more efficient), otherwise storage executor
	var metadataExecutor api.QueryExecutor
	if graphExecutor != nil {
		metadataExecutor = graphExecutor
	} else {
		metadataExecutor = storageExecutor
	}

	if metadataExecutor != nil {
		// Create cache with configurable refresh period
		s.metadataCache = api.NewMetadataCache(metadataExecutor, s.logger, metadataRefreshPeriod)
		s.logger.Info("Metadata cache created with refresh period %v (will initialize on server start)", metadataRefreshPeriod)
	}

	// Create timeline service with appropriate executor(s)
	// This service is shared by REST handlers and MCP tools
	tracer := s.getTracer("spectre.api.timeline")
	if graphExecutor != nil && querySource == api.TimelineQuerySourceGraph {
		s.logger.Info("Timeline service using GRAPH query executor")
		s.timelineService = api.NewTimelineServiceWithMode(storageExecutor, graphExecutor, querySource, s.logger, tracer)
	} else if graphExecutor != nil {
		s.logger.Info("Timeline service using STORAGE query executor (graph available for comparison)")
		s.timelineService = api.NewTimelineServiceWithMode(storageExecutor, graphExecutor, api.TimelineQuerySourceStorage, s.logger, tracer)
	} else {
		s.logger.Info("Timeline service using STORAGE query executor only")
		s.timelineService = api.NewTimelineService(storageExecutor, s.logger, tracer)
	}

	// Create namespace graph cache if enabled and graph client is available
	if nsGraphCacheConfig.Enabled && graphClient != nil {
		analyzer := namespacegraph.NewAnalyzer(graphClient)
		cacheConfig := namespacegraph.CacheConfig{
			RefreshTTL:  nsGraphCacheConfig.RefreshTTL,
			MaxMemoryMB: nsGraphCacheConfig.MaxMemoryMB,
		}
		// Pass metadata cache for namespace discovery and pre-warming
		s.nsGraphCache = namespacegraph.NewCache(cacheConfig, analyzer, s.metadataCache, s.logger)
		s.logger.Info("Namespace graph cache created with refresh TTL %v, max memory %dMB (will initialize on server start)",
			nsGraphCacheConfig.RefreshTTL, nsGraphCacheConfig.MaxMemoryMB)
	}

	// Register all routes and handlers
	s.registerHandlers()

	// Configure HTTP server with CORS middleware and timeouts
	s.configureHTTPServer(port)

	return s
}

// configureHTTPServer creates the HTTP server with CORS middleware and appropriate timeouts
func (s *Server) configureHTTPServer(port int) {
	// Use router with CORS middleware as the main handler
	handler := s.corsMiddleware(s.router)

	// Create HTTP server
	// Use longer timeouts to accommodate long-running imports (can take 5+ minutes)
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handler,
		ReadTimeout:  10 * time.Minute, // Allow time to read large request bodies
		WriteTimeout: 10 * time.Minute, // Allow time for processing + response writing (imports can take 5+ min)
		IdleTimeout:  60 * time.Second,
	}
}

// registerMCPHandler adds MCP endpoint to the router
func (s *Server) registerMCPHandler() {
	if s.mcpServer == nil {
		s.logger.Debug("MCP server not configured, skipping /v1/mcp endpoint")
		return
	}

	endpointPath := "/v1/mcp"
	s.logger.Info("Registering MCP endpoint at %s", endpointPath)

	// Create StreamableHTTP server with stateless mode
	streamableServer := server.NewStreamableHTTPServer(
		s.mcpServer,
		server.WithEndpointPath(endpointPath),
		server.WithStateLess(true), // Stateless mode per requirements
	)

	// Register on router (must be BEFORE static UI catch-all)
	s.router.Handle(endpointPath, streamableServer)
	s.logger.Info("MCP endpoint registered at %s", endpointPath)
}

// Start implements the lifecycle.Component interface
// Starts the HTTP server with Connect RPC support and begins listening for requests
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("Starting API server on port %d (HTTP with Connect RPC)", s.port)

	// Check context isn't already cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Start metadata cache if available
	if s.metadataCache != nil {
		s.logger.Info("Initializing metadata cache...")
		if err := s.metadataCache.Start(ctx); err != nil {
			s.logger.Error("Failed to start metadata cache: %v", err)
			// Don't fail server startup - cache is optional optimization
			// Handlers will fall back to direct queries
		} else {
			s.logger.Info("Metadata cache started successfully")
		}
	}

	// Start namespace graph cache if available
	if s.nsGraphCache != nil {
		s.logger.Info("Initializing namespace graph cache...")
		if err := s.nsGraphCache.Start(ctx); err != nil {
			s.logger.Error("Failed to start namespace graph cache: %v", err)
			// Don't fail server startup - cache is optional optimization
			// Handlers will fall back to direct queries
		} else {
			s.logger.Info("Namespace graph cache started successfully")
		}
	}

	// Start HTTP server in a goroutine
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error: %v", err)
		}
	}()

	s.logger.Info("API server started and listening on port %d (supports HTTP, gRPC, gRPC-Web, and Connect)", s.port)
	return nil
}

// Stop implements the lifecycle.Component interface
// Gracefully stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Stopping API server...")

	// Stop metadata cache if running
	if s.metadataCache != nil {
		s.metadataCache.Stop()
	}

	// Stop namespace graph cache if running
	if s.nsGraphCache != nil {
		s.nsGraphCache.Stop()
	}

	// Stop HTTP server
	done := make(chan error, 1)
	go func() {
		// Gracefully shutdown server
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done <- s.server.Shutdown(shutdownCtx)
	}()

	select {
	case err := <-done:
		if err != nil {
			s.logger.Error("HTTP server shutdown error: %v", err)
			return err
		}
		s.logger.Info("API server stopped")
		return nil
	case <-ctx.Done():
		s.logger.Warn("API server shutdown timeout")
		return ctx.Err()
	}
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status": "healthy",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = api.WriteJSON(w, response)
}

// handleReady handles readiness check requests
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check readiness if checker is available
	ready := s.readinessChecker != nil && s.readinessChecker.IsReady()

	response := map[string]interface{}{
		"ready": ready,
	}

	if ready {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	_ = api.WriteJSON(w, response)
}

// GetPort returns the port the server is listening on
func (s *Server) GetPort() int {
	return s.port
}

// IsRunning checks if the server is running
func (s *Server) IsRunning() bool {
	return s.server != nil
}

// Name implements the lifecycle.Component interface
// Returns the human-readable name of the API server component
func (s *Server) Name() string {
	return "API Server"
}

// GetNamespaceGraphCache returns the namespace graph cache for registration
// with the event-driven invalidation system.
// Returns nil if caching is disabled.
func (s *Server) GetNamespaceGraphCache() *namespacegraph.Cache {
	return s.nsGraphCache
}

// GetTimelineService returns the shared timeline service for use by MCP tools.
// This enables MCP tools to call the service directly instead of making HTTP requests.
func (s *Server) GetTimelineService() *api.TimelineService {
	return s.timelineService
}

// RegisterMCPEndpoint registers the MCP server endpoint after server initialization.
// This allows the MCP server to be created with the TimelineService from this API server.
func (s *Server) RegisterMCPEndpoint(mcpServer *server.MCPServer) error {
	if mcpServer == nil {
		return fmt.Errorf("mcpServer cannot be nil")
	}
	s.mcpServer = mcpServer

	// Register the MCP endpoint using the existing method
	s.registerMCPHandler()
	return nil
}
