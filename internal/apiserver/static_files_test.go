package apiserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestServeStaticUI_ServesIndexWhenBundleIsComplete(t *testing.T) {
	t.Parallel()

	uiDir := t.TempDir()
	writeTestFile(t, filepath.Join(uiDir, "index.html"), `<!doctype html><html><head><script type="module" src="/assets/app.js"></script></head><body><div id="root"></div></body></html>`)
	writeTestFile(t, filepath.Join(uiDir, "assets", "app.js"), `console.log("ok");`)

	server := &Server{
		staticCache: newStaticFileCache(uiDir),
	}

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	recorder := httptest.NewRecorder()

	server.serveStaticUI(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
}

func TestServeStaticUI_ReturnsErrorWhenIndexReferencesMissingBundleAsset(t *testing.T) {
	t.Parallel()

	uiDir := t.TempDir()
	writeTestFile(t, filepath.Join(uiDir, "index.html"), `<!doctype html><html><head><script type="module" src="/assets/app.js"></script></head><body><div id="root"></div></body></html>`)

	server := &Server{
		staticCache: newStaticFileCache(uiDir),
	}

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	recorder := httptest.NewRecorder()

	server.serveStaticUI(recorder, req)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d: %s", http.StatusInternalServerError, recorder.Code, recorder.Body.String())
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
