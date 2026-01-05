package apiserver

import (
	"fmt"
	"net/http"

	"github.com/moolen/spectre/internal/api"
)

// handleMethodNotAllowed handles 405 responses
func (s *Server) handleMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusMethodNotAllowed)

	response := map[string]string{
		"error":   "METHOD_NOT_ALLOWED",
		"message": fmt.Sprintf("Method %s not allowed for %s", r.Method, r.URL.Path),
	}

	_ = api.WriteJSON(w, response)
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

	_ = api.WriteJSON(w, response)
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
// 	_ = api.WriteJSON(w, response)
// }
