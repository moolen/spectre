package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/moolen/spectre/internal/api/pb"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/storage"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
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

// Server handles HTTP API requests and gRPC requests
type Server struct {
	port             int
	server           *http.Server
	grpcServer       *grpc.Server
	grpcListener     net.Listener
	logger           *logging.Logger
	queryExecutor    QueryExecutor
	storage          *storage.Storage
	router           *http.ServeMux
	readinessChecker ReadinessChecker
	demoMode         bool
	tracingProvider  interface {
		GetTracer(string) trace.Tracer
		IsEnabled() bool
	}
}

// New creates a new API server
func New(port int, queryExecutor QueryExecutor, readinessChecker ReadinessChecker, tracingProvider interface {
	GetTracer(string) trace.Tracer
	IsEnabled() bool
}) *Server {
	return NewWithStorage(port, queryExecutor, nil, readinessChecker, false, tracingProvider)
}

// NewWithStorage creates a new API server with storage export/import capabilities
func NewWithStorage(port int, queryExecutor QueryExecutor, storage *storage.Storage, readinessChecker ReadinessChecker, demoMode bool, tracingProvider interface {
	GetTracer(string) trace.Tracer
	IsEnabled() bool
}) *Server {
	s := &Server{
		port:             port,
		logger:           logging.GetLogger("api"),
		queryExecutor:    queryExecutor,
		storage:          storage,
		router:           http.NewServeMux(),
		readinessChecker: readinessChecker,
		demoMode:         demoMode,
		tracingProvider:  tracingProvider,
	}

	// Create gRPC server first (needed for gRPC-Web wrapper)
	s.grpcServer = grpc.NewServer()

	// Get tracer for gRPC service
	var tracer trace.Tracer
	if tracingProvider != nil && tracingProvider.IsEnabled() {
		tracer = tracingProvider.GetTracer("spectre.api.grpc")
	} else {
		tracer = otel.GetTracerProvider().Tracer("spectre.api.grpc")
	}

	// Register gRPC services
	timelineGRPCService := NewTimelineGRPCService(queryExecutor, s.logger, tracer)
	pb.RegisterTimelineServiceServer(s.grpcServer, timelineGRPCService)

	// Wrap gRPC server with gRPC-Web support
	grpcWebWrapper := grpcweb.WrapServer(s.grpcServer,
		grpcweb.WithCorsForRegisteredEndpointsOnly(false),
		grpcweb.WithOriginFunc(func(origin string) bool {
			// Allow all origins for development
			return true
		}),
	)

	// Register handlers
	s.registerHandlers()

	// Create HTTP handler that routes gRPC-Web requests to gRPC server, others to router
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if this is a gRPC-Web request
		if grpcWebWrapper.IsGrpcWebRequest(r) || grpcWebWrapper.IsAcceptableGrpcCorsRequest(r) {
			grpcWebWrapper.ServeHTTP(w, r)
			return
		}

		// Otherwise, use the regular router with CORS middleware
		s.corsMiddleware(s.router).ServeHTTP(w, r)
	})

	// Create HTTP server
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// registerHandlers registers all HTTP handlers
func (s *Server) registerHandlers() {
	var tracer trace.Tracer
	if s.tracingProvider != nil && s.tracingProvider.IsEnabled() {
		tracer = s.tracingProvider.GetTracer("spectre.api")
	} else {
		tracer = otel.GetTracerProvider().Tracer("spectre.api")
	}

	searchHandler := NewSearchHandler(s.queryExecutor, s.logger, tracer)
	timelineHandler := NewTimelineHandler(s.queryExecutor, s.logger, tracer)
	metadataHandler := NewMetadataHandler(s.queryExecutor, s.logger, tracer)

	s.router.HandleFunc("/v1/search", s.withMethod(http.MethodGet, searchHandler.Handle))
	s.router.HandleFunc("/v1/timeline", s.withMethod(http.MethodGet, timelineHandler.Handle))
	s.router.HandleFunc("/v1/metadata", s.withMethod(http.MethodGet, metadataHandler.Handle))
	s.router.HandleFunc("/health", s.handleHealth)
	s.router.HandleFunc("/ready", s.handleReady)

	// Register export/import handlers if storage is available
	if s.storage != nil {
		exportHandler := NewExportHandler(s.storage, s.logger)
		importHandler := NewImportHandler(s.storage, s.logger)
		s.router.HandleFunc("/v1/storage/export", s.withMethod(http.MethodGet, exportHandler.Handle))
		s.router.HandleFunc("/v1/storage/import", s.withMethod(http.MethodPost, importHandler.Handle))
	}

	// Serve static UI files and handle SPA routing
	// This must be registered last so it acts as a catch-all
	s.router.HandleFunc("/", s.serveStaticUI)
	s.router.HandleFunc("/timeline", s.serveStaticUI)
}

// withMethod wraps a handler to enforce HTTP method
func (s *Server) withMethod(method string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			s.handleMethodNotAllowed(w, r)
			return
		}
		handler(w, r)
	}
}

// serveStaticUI serves the built React UI and handles SPA routing
func (s *Server) serveStaticUI(w http.ResponseWriter, r *http.Request) {
	// Get the UI directory path
	uiDir := "/app/ui"
	if _, err := os.Stat(uiDir); os.IsNotExist(err) {
		// Fall back to local dev path if running outside Docker
		uiDir = "./ui/dist"
	}

	// Clean the path to prevent directory traversal
	path := filepath.Clean(r.URL.Path)
	s.logger.Info("static serving path: %q", path)
	if path == "/" || path == "/timeline" {
		path = "/index.html"
	}

	// Try to serve the file
	filePath := filepath.Join(uiDir, path)
	s.logger.Info("trying to serve file: %q", filePath)
	if _, err := os.Stat(filePath); err == nil {
		s.logger.Info("serving file: %q", filePath)
		// File exists, serve it
		http.ServeFile(w, r, filePath)
		return
	}

	// For SPA routing, serve index.html for non-existent files that aren't assets
	if !isAssetPath(path) {
		indexPath := filepath.Join(uiDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			// Set Cache-Control for index.html to prevent caching
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			http.ServeFile(w, r, indexPath)
			return
		}
	}

	// File not found
	w.WriteHeader(http.StatusNotFound)
}

// isAssetPath checks if a path looks like an asset (JS, CSS, image, etc.)
func isAssetPath(path string) bool {
	assetExtensions := map[string]bool{
		".js":    true,
		".css":   true,
		".png":   true,
		".jpg":   true,
		".jpeg":  true,
		".gif":   true,
		".svg":   true,
		".woff":  true,
		".woff2": true,
		".ttf":   true,
		".eot":   true,
	}
	ext := filepath.Ext(path)
	return assetExtensions[ext]
}

// corsMiddleware adds CORS headers to allow browser access
// For local development only - allows all origins
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("path: %s", r.URL.Path)
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "3600")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Continue with the next handler
		next.ServeHTTP(w, r)
	})
}

// Start implements the lifecycle.Component interface
// Starts the HTTP server and gRPC server, and begins listening for requests
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("Starting API servers on ports %d (HTTP) and %d (gRPC)", s.port, s.port+1)

	// Check context isn't already cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Start HTTP server in a goroutine
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("HTTP server error: %v", err)
		}
	}()

	s.logger.Info("HTTP API server started and listening on port %d", s.port)

	// Start gRPC server on different port
	grpcPort := s.port + 1
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcPort))
	if err != nil {
		return fmt.Errorf("failed to listen on gRPC port: %w", err)
	}
	s.grpcListener = lis

	// Start gRPC server in a goroutine
	go func() {
		s.logger.Info("Starting gRPC server on port %d", grpcPort)
		if err := s.grpcServer.Serve(lis); err != nil {
			s.logger.Error("gRPC server error: %v", err)
		}
	}()

	s.logger.Info("gRPC API server started and listening on port %d", grpcPort)
	return nil
}

// Stop implements the lifecycle.Component interface
// Gracefully stops the HTTP server and gRPC server
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Stopping API servers...")

	// Stop gRPC server
	if s.grpcServer != nil {
		s.logger.Info("Stopping gRPC server...")
		s.grpcServer.GracefulStop()
		s.logger.Info("gRPC server stopped")
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
		s.logger.Info("HTTP server stopped")
		return nil
	case <-ctx.Done():
		s.logger.Warn("HTTP server shutdown timeout")
		return ctx.Err()
	}
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status": "healthy",
		"demo":   s.demoMode,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = writeJSON(w, response)
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

	_ = writeJSON(w, response)
}

// handleNotFound handles 404 responses
// This function is currently unused but kept for potential future use
// func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "application/json")
// 	w.WriteHeader(http.StatusNotFound)
//
// 	response := map[string]string{
// 		"error":   "NOT_FOUND",
// 		"message": fmt.Sprintf("Endpoint not found: %s", r.URL.Path),
// 	}
//
// 	_ = writeJSON(w, response)
// }

// handleMethodNotAllowed handles 405 responses
func (s *Server) handleMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusMethodNotAllowed)

	response := map[string]string{
		"error":   "METHOD_NOT_ALLOWED",
		"message": fmt.Sprintf("Method %s not allowed for %s", r.Method, r.URL.Path),
	}

	_ = writeJSON(w, response)
}

// handleIncorrectGRPCPath handles requests to gRPC paths over HTTP
// Returns a helpful error message pointing to the correct HTTP endpoint
func (s *Server) handleIncorrectGRPCPath(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)

	response := map[string]interface{}{
		"error":   "INCORRECT_ENDPOINT",
		"message": "This is a gRPC endpoint. Use the HTTP REST API instead.",
		"correct_endpoint": map[string]string{
			"http": "/v1/timeline",
			"grpc": fmt.Sprintf("grpc://localhost:%d/api.TimelineService/GetTimeline", s.port+1),
		},
		"details": "The gRPC service is available on a separate port. For HTTP requests, use /v1/timeline with query parameters.",
	}

	_ = writeJSON(w, response)
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
