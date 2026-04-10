package apiserver

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/server"
	namespacegraph "github.com/moolen/spectre/internal/analysis/namespace_graph"
	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/api"
	apptimeline "github.com/moolen/spectre/internal/app/timeline"
	"github.com/moolen/spectre/internal/logging"
	"go.opentelemetry.io/otel/trace"
)

// ReadinessChecker is an interface for checking component readiness.
type ReadinessChecker interface {
	IsReady() bool
}

// NoOpReadinessChecker is a ReadinessChecker that always returns true.
type NoOpReadinessChecker struct{}

// IsReady always returns true for the no-op checker.
func (n *NoOpReadinessChecker) IsReady() bool {
	return true
}

// Server handles HTTP API requests and Connect RPC requests.
type Server struct {
	port             int
	server           *http.Server
	logger           *logging.Logger
	queryExecutor    api.QueryExecutor
	analysisStore    analysisstore.AnalysisStore
	importIngestor   api.BatchIngestor
	timelineService  *apptimeline.Service
	metadataCache    *api.MetadataCache
	nsGraphCache     *namespacegraph.Cache
	staticCache      *staticFileCache
	router           *http.ServeMux
	readinessChecker ReadinessChecker
	tracingProvider  interface {
		GetTracer(string) trace.Tracer
		IsEnabled() bool
	}
	mcpServer *server.MCPServer
}

// NamespaceGraphCacheConfig holds configuration for the namespace graph cache.
type NamespaceGraphCacheConfig struct {
	Enabled     bool
	RefreshTTL  time.Duration
	MaxMemoryMB int64
}

// NewWithStorageGraphAndPipeline creates a new storage-backed API server.
func NewWithStorageGraphAndPipeline(
	port int,
	storageExecutor api.QueryExecutor,
	analysisStore analysisstore.AnalysisStore,
	importIngestor api.BatchIngestor,
	readinessChecker ReadinessChecker,
	tracingProvider interface {
		GetTracer(string) trace.Tracer
		IsEnabled() bool
	},
	metadataRefreshPeriod time.Duration,
	nsGraphCacheConfig NamespaceGraphCacheConfig,
) *Server {
	s := &Server{
		port:             port,
		logger:           logging.GetLogger("api"),
		queryExecutor:    storageExecutor,
		analysisStore:    analysisStore,
		importIngestor:   importIngestor,
		router:           http.NewServeMux(),
		readinessChecker: readinessChecker,
		tracingProvider:  tracingProvider,
	}

	if storageExecutor != nil {
		s.metadataCache = api.NewMetadataCache(storageExecutor, s.logger, metadataRefreshPeriod)
		s.logger.Info("Metadata cache created with refresh period %v (will initialize on server start)", metadataRefreshPeriod)
	}

	tracer := s.getTracer("spectre.api.timeline")
	s.logger.Info("Timeline service using STORAGE query executor")
	s.timelineService = apptimeline.NewService(storageExecutor, s.logger, tracer)

	if nsGraphCacheConfig.Enabled && analysisStore != nil {
		analyzer := namespacegraph.NewAnalyzer(analysisStore)
		cacheConfig := namespacegraph.CacheConfig{
			RefreshTTL:  nsGraphCacheConfig.RefreshTTL,
			MaxMemoryMB: nsGraphCacheConfig.MaxMemoryMB,
		}
		s.nsGraphCache = namespacegraph.NewCache(cacheConfig, analyzer, s.metadataCache, s.logger)
		s.logger.Info(
			"Namespace graph cache created with refresh TTL %v, max memory %dMB (will initialize on server start)",
			nsGraphCacheConfig.RefreshTTL,
			nsGraphCacheConfig.MaxMemoryMB,
		)
	}

	s.registerHandlers()
	s.configureHTTPServer(port)

	return s
}

func (s *Server) configureHTTPServer(port int) {
	handler := s.corsMiddleware(s.router)
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handler,
		ReadTimeout:  10 * time.Minute,
		WriteTimeout: 10 * time.Minute,
		IdleTimeout:  60 * time.Second,
	}
}

func (s *Server) registerMCPHandler() {
	if s.mcpServer == nil {
		s.logger.Debug("MCP server not configured, skipping /v1/mcp endpoint")
		return
	}

	endpointPath := "/v1/mcp"
	s.logger.Info("Registering MCP endpoint at %s", endpointPath)

	streamableServer := server.NewStreamableHTTPServer(
		s.mcpServer,
		server.WithEndpointPath(endpointPath),
		server.WithStateLess(true),
	)

	s.router.Handle(endpointPath, streamableServer)
	s.logger.Info("MCP endpoint registered at %s", endpointPath)
}

// Start implements the lifecycle.Component interface.
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("Starting API server on port %d (HTTP with Connect RPC)", s.port)

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if s.metadataCache != nil {
		s.logger.Info("Initializing metadata cache...")
		if err := s.metadataCache.Start(ctx); err != nil {
			s.logger.Error("Failed to start metadata cache: %v", err)
		} else {
			s.logger.Info("Metadata cache started successfully")
		}
	}

	if s.nsGraphCache != nil {
		s.logger.Info("Initializing namespace graph cache...")
		if err := s.nsGraphCache.Start(ctx); err != nil {
			s.logger.Error("Failed to start namespace graph cache: %v", err)
		} else {
			s.logger.Info("Namespace graph cache started successfully")
		}
	}

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error: %v", err)
		}
	}()

	s.logger.Info("API server started and listening on port %d (supports HTTP, gRPC, gRPC-Web, and Connect)", s.port)
	return nil
}

// Stop implements the lifecycle.Component interface.
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Stopping API server...")

	if s.metadataCache != nil {
		s.metadataCache.Stop()
	}
	if s.nsGraphCache != nil {
		s.nsGraphCache.Stop()
	}

	done := make(chan error, 1)
	go func() {
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

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	response := map[string]interface{}{"status": "healthy"}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = api.WriteJSON(w, response)
}

func (s *Server) handleReady(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ready := s.readinessChecker != nil && s.readinessChecker.IsReady()
	response := map[string]interface{}{"ready": ready}

	if ready {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	_ = api.WriteJSON(w, response)
}

func (s *Server) GetPort() int {
	return s.port
}

func (s *Server) Handler() http.Handler {
	if s.server != nil && s.server.Handler != nil {
		return s.server.Handler
	}
	return s.router
}

func (s *Server) IsRunning() bool {
	return s.server != nil
}

func (s *Server) Name() string {
	return "API Server"
}

func (s *Server) GetNamespaceGraphCache() *namespacegraph.Cache {
	return s.nsGraphCache
}

func (s *Server) GetTimelineService() *apptimeline.Service {
	return s.timelineService
}

// RegisterMCPEndpoint registers the MCP server endpoint after server initialization.
func (s *Server) RegisterMCPEndpoint(mcpServer *server.MCPServer) error {
	if mcpServer == nil {
		return fmt.Errorf("mcpServer cannot be nil")
	}
	s.mcpServer = mcpServer
	s.registerMCPHandler()
	return nil
}
