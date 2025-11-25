package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/moritz/rpk/internal/logging"
	"github.com/moritz/rpk/internal/storage"
)

// Server handles HTTP API requests
type Server struct {
	port            int
	server          *http.Server
	logger          *logging.Logger
	queryExecutor   *storage.QueryExecutor
	router          *http.ServeMux
}

// New creates a new API server
func New(port int, queryExecutor *storage.QueryExecutor) *Server {
	s := &Server{
		port:          port,
		logger:        logging.GetLogger("api"),
		queryExecutor: queryExecutor,
		router:        http.NewServeMux(),
	}

	// Register handlers
	s.registerHandlers()

	// Create HTTP server
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// registerHandlers registers all HTTP handlers
func (s *Server) registerHandlers() {
	// Search endpoint
	s.router.HandleFunc("/v1/search", s.handleSearch)

	// Health check endpoints
	s.router.HandleFunc("/health", s.handleHealth)
	s.router.HandleFunc("/healthz", s.handleHealth)

	// Root endpoint
	s.router.HandleFunc("/", s.handleRoot)
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("Starting API server on port %d", s.port)

	// Start server in a goroutine
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Server error: %v", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()
	s.logger.Info("Received shutdown signal")

	return s.Stop()
}

// Stop stops the HTTP server
func (s *Server) Stop() error {
	s.logger.Info("Stopping API server...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Gracefully shutdown server
	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.Error("Server shutdown error: %v", err)
		return err
	}

	s.logger.Info("API server stopped")
	return nil
}

// handleRoot handles requests to /
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		s.handleNotFound(w, r)
		return
	}

	response := map[string]string{
		"service":     "Kubernetes Event Monitor",
		"version":     "0.1.0",
		"status":      "operational",
		"description": "Monitor and query Kubernetes resource events",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	writeJSON(w, response)
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status": "healthy",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	writeJSON(w, response)
}

// handleSearch handles /v1/search requests
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	// Only allow GET
	if r.Method != http.MethodGet {
		s.handleMethodNotAllowed(w, r)
		return
	}

	// Parse and validate request
	searchHandler := NewSearchHandler(s.queryExecutor, s.logger)
	searchHandler.Handle(w, r)
}

// handleNotFound handles 404 responses
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)

	response := map[string]string{
		"error":   "NOT_FOUND",
		"message": fmt.Sprintf("Endpoint not found: %s", r.URL.Path),
	}

	writeJSON(w, response)
}

// handleMethodNotAllowed handles 405 responses
func (s *Server) handleMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusMethodNotAllowed)

	response := map[string]string{
		"error":   "METHOD_NOT_ALLOWED",
		"message": fmt.Sprintf("Method %s not allowed for %s", r.Method, r.URL.Path),
	}

	writeJSON(w, response)
}

// GetPort returns the port the server is listening on
func (s *Server) GetPort() int {
	return s.port
}

// IsRunning checks if the server is running
func (s *Server) IsRunning() bool {
	return s.server != nil
}
