package apiserver

import (
	"crypto/md5" // #nosec G501 -- MD5 used only for ETag generation, not cryptography
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const pathIndexHTML = "/index.html"

// cachedFile represents a single cached static file
type cachedFile struct {
	content     []byte
	contentType string
	modTime     time.Time
	etag        string
}

// staticFileCache provides thread-safe in-memory caching for static files
type staticFileCache struct {
	mu         sync.RWMutex
	files      map[string]*cachedFile
	uiDir      string
	reloadLock sync.Mutex // Prevents concurrent reloads
	reloading  map[string]bool // Tracks files currently being reloaded
}

// newStaticFileCache creates a new static file cache
func newStaticFileCache(uiDir string) *staticFileCache {
	return &staticFileCache{
		files:     make(map[string]*cachedFile),
		uiDir:     uiDir,
		reloading: make(map[string]bool),
	}
}

// get retrieves a cached file, loading it from disk if not cached
func (c *staticFileCache) get(path string) (*cachedFile, error) {
	// Fast path: check if file is in cache with read lock
	c.mu.RLock()
	cached, exists := c.files[path]
	c.mu.RUnlock()

	if exists {
		// Check if file on disk is newer than cached version
		// Trigger async reload if needed (but serve cached version immediately)
		go c.maybeReload(path, cached.modTime)
		return cached, nil
	}

	// Slow path: file not in cache, load it with write lock
	return c.loadAndCache(path)
}

// loadAndCache loads a file from disk and caches it (with write lock)
func (c *staticFileCache) loadAndCache(path string) (*cachedFile, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check: another goroutine might have loaded it while we waited for lock
	if cached, exists := c.files[path]; exists {
		return cached, nil
	}

	// Load file from disk
	// filePath is constructed from sanitized uiDir (hardcoded base) + cleaned path
	filePath := filepath.Join(c.uiDir, path)
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	// #nosec G304 -- Path is sanitized via filepath.Clean and constrained to uiDir
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	// Generate ETag from content hash
	// #nosec G401 -- MD5 used only for ETag generation (cache validation), not cryptography
	hash := md5.Sum(content)
	etag := fmt.Sprintf(`"%x"`, hash)

	// Detect content type
	contentType := detectContentType(path, content)

	cached := &cachedFile{
		content:     content,
		contentType: contentType,
		modTime:     fileInfo.ModTime(),
		etag:        etag,
	}

	// Store in cache
	c.files[path] = cached

	return cached, nil
}

// maybeReload checks if a file needs reloading and reloads it if necessary
func (c *staticFileCache) maybeReload(path string, cachedModTime time.Time) {
	// Check if file on disk is newer
	filePath := filepath.Join(c.uiDir, path)
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		// File might have been deleted, ignore
		return
	}

	// If disk file is not newer, no need to reload
	if !fileInfo.ModTime().After(cachedModTime) {
		return
	}

	// Acquire reload lock to prevent concurrent reloads of the same file
	c.reloadLock.Lock()
	if c.reloading[path] {
		// Already reloading this file, skip
		c.reloadLock.Unlock()
		return
	}
	c.reloading[path] = true
	c.reloadLock.Unlock()

	// Reload the file
	defer func() {
		c.reloadLock.Lock()
		delete(c.reloading, path)
		c.reloadLock.Unlock()
	}()

	// Load file from disk
	// #nosec G304 -- Path is sanitized via filepath.Clean and constrained to uiDir
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer func() {
		_ = file.Close()
	}()

	content, err := io.ReadAll(file)
	if err != nil {
		return
	}

	// Generate new ETag
	// #nosec G401 -- MD5 used only for ETag generation (cache validation), not cryptography
	hash := md5.Sum(content)
	etag := fmt.Sprintf(`"%x"`, hash)

	// Detect content type
	contentType := detectContentType(path, content)

	// Update cache with write lock
	c.mu.Lock()
	c.files[path] = &cachedFile{
		content:     content,
		contentType: contentType,
		modTime:     fileInfo.ModTime(),
		etag:        etag,
	}
	c.mu.Unlock()
}

// detectContentType detects the MIME type of a file
func detectContentType(path string, content []byte) string {
	// Use extension-based detection first for better accuracy
	ext := filepath.Ext(path)
	switch ext {
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".eot":
		return "application/vnd.ms-fontobject"
	case ".ico":
		return "image/x-icon"
	}

	// Fall back to content sniffing
	return http.DetectContentType(content)
}

// serveStaticUI serves the built React UI and handles SPA routing
func (s *Server) serveStaticUI(w http.ResponseWriter, r *http.Request) {
	// Initialize cache if not already done
	if s.staticCache == nil {
		// Get the UI directory path
		uiDir := "/app/ui"
		if _, err := os.Stat(uiDir); os.IsNotExist(err) {
			// Fall back to local dev path if running outside Docker
			uiDir = "./ui/dist"
		}
		s.staticCache = newStaticFileCache(uiDir)
	}

	// Clean the path to prevent directory traversal
	path := filepath.Clean(r.URL.Path)

	// Handle root and SPA routes
	originalPath := path
	if path == "/" || path == "/timeline" {
		path = pathIndexHTML
	}

	// Try to serve the file from cache
	cached, err := s.staticCache.get(path)
	if err == nil {
		// File found in cache, serve it
		s.serveCachedFile(w, r, cached, path)
		return
	}

	// For SPA routing, serve index.html for non-existent files that aren't assets
	if !isAssetPath(originalPath) {
		cached, err := s.staticCache.get(pathIndexHTML)
		if err == nil {
			// Serve index.html with no-cache headers
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			s.serveCachedFile(w, r, cached, "pathIndexHTML")
			return
		}
	}

	// File not found
	http.Error(w, "Not Found", http.StatusNotFound)
}

// serveCachedFile serves a cached file with proper headers
func (s *Server) serveCachedFile(w http.ResponseWriter, r *http.Request, cached *cachedFile, path string) {
	// Set content type
	w.Header().Set("Content-Type", cached.contentType)

	// Set ETag
	w.Header().Set("ETag", cached.etag)

	// Set Last-Modified
	w.Header().Set("Last-Modified", cached.modTime.UTC().Format(http.TimeFormat))

	// Set cache headers for assets (but not for index.html)
	if path != pathIndexHTML && isAssetPath(path) {
		// Cache assets for 1 year (immutable if using hash-based filenames)
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else if path == pathIndexHTML {
		// Never cache index.html
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	}

	// Handle If-None-Match (ETag validation)
	if match := r.Header.Get("If-None-Match"); match != "" {
		if match == cached.etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	// Handle If-Modified-Since
	if modifiedSince := r.Header.Get("If-Modified-Since"); modifiedSince != "" {
		if t, err := time.Parse(http.TimeFormat, modifiedSince); err == nil {
			if cached.modTime.Before(t.Add(1 * time.Second)) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
	}

	// Write content
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(cached.content)
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
		".ico":   true,
		".json":  true,
	}
	ext := filepath.Ext(path)
	return assetExtensions[ext]
}
