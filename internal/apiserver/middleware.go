package apiserver

import (
	"net/http"
)

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
