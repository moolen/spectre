package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/mcp"
)

// Transport implements HTTP-based MCP transport
type Transport struct {
	handler      *mcp.Handler
	server       *http.Server
	addr         string
	version      string
	endpointPath string
}

// NewTransport creates a new HTTP transport
func NewTransport(addr string, mcpServer *mcp.MCPServer, version, endpointPath string) *Transport {
	handler := mcp.NewHandler(mcpServer, version)

	t := &Transport{
		handler:      handler,
		addr:         addr,
		version:      version,
		endpointPath: endpointPath,
	}

	t.server = &http.Server{
		Addr:         addr,
		Handler:      t.createHTTPHandler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return t
}

// Start starts the HTTP server
func (t *Transport) Start(ctx context.Context) error {
	logger := logging.GetLogger("mcp")
	logger.Info("Starting HTTP server on %s", t.addr)

	errCh := make(chan error, 1)

	go func() {
		if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return t.Stop()
	}
}

// Stop gracefully shuts down the HTTP server
func (t *Transport) Stop() error {
	logger := logging.GetLogger("mcp")
	logger.Info("Shutting down HTTP server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := t.server.Shutdown(shutdownCtx); err != nil {
		logger.Error("Error during shutdown: %v", err)
		return err
	}

	logger.Info("HTTP server stopped")
	return nil
}

// createHTTPHandler creates the HTTP mux with all endpoints
func (t *Transport) createHTTPHandler() http.Handler {
	mux := http.NewServeMux()

	// Main MCP endpoint
	mux.HandleFunc("POST "+t.endpointPath, func(w http.ResponseWriter, r *http.Request) {
		t.handleMCPRequest(w, r)
	})

	// Health check endpoint
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Root endpoint
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"name":    "Spectre MCP Server",
			"version": t.version,
		})
	})

	return mux
}

// handleMCPRequest processes an HTTP MCP request
func (t *Transport) handleMCPRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req mcp.MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		t.sendError(w, nil, -32700, "Parse error")
		return
	}

	resp := t.handler.HandleRequest(r.Context(), &req)
	_ = json.NewEncoder(w).Encode(resp)
}

// sendError sends an error response
func (t *Transport) sendError(w http.ResponseWriter, id interface{}, code int, message string) {
	resp := &mcp.MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &mcp.MCPError{
			Code:    code,
			Message: message,
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}
