package apiserver

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graph/sync"
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
	graphPipeline    sync.Pipeline      // Graph sync pipeline for imports
	metadataCache    *api.MetadataCache // In-memory metadata cache for fast responses
	staticCache      *staticFileCache   // In-memory static file cache for fast UI serving
	router           *http.ServeMux
	readinessChecker ReadinessChecker
	tracingProvider  interface {
		GetTracer(string) trace.Tracer
		IsEnabled() bool
	}
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
	demoMode bool, // Not used but kept for signature compatibility
	tracingProvider interface {
		GetTracer(string) trace.Tracer
		IsEnabled() bool
	},
	metadataRefreshPeriod time.Duration, // How often to refresh the metadata cache
) *Server {
	s := &Server{
		port:             port,
		logger:           logging.GetLogger("api"),
		queryExecutor:    storageExecutor,
		graphExecutor:    graphExecutor,
		querySource:      querySource,
		graphClient:      graphClient,
		graphPipeline:    graphPipeline,
		router:           http.NewServeMux(),
		readinessChecker: readinessChecker,
		tracingProvider:  tracingProvider,
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
