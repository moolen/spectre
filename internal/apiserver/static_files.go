package apiserver

import (
	"net/http"
	"os"
	"path/filepath"
)

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
