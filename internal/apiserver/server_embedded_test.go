package apiserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/embeddedstore"
	"github.com/stretchr/testify/require"
)

func newEmbeddedTestServer(t *testing.T) *Server {
	t.Helper()

	backend, err := embeddedstore.Open(embeddedstore.Config{DataDir: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = backend.Close()
	})

	return NewWithStorageGraphAndPipeline(
		0,
		backend.QueryExecutor(),
		nil,
		api.TimelineQuerySourceStorage,
		nil,
		nil,
		backend.AnalysisStore(),
		backend,
		backend,
		nil,
		time.Minute,
		NamespaceGraphCacheConfig{},
		"",
		nil,
		nil,
	)
}

func newEmbeddedCompareTestServer(t *testing.T) *Server {
	t.Helper()

	backend, err := embeddedstore.Open(embeddedstore.Config{DataDir: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = backend.Close()
	})

	executor := backend.QueryExecutor()

	return NewWithStorageGraphAndPipeline(
		0,
		executor,
		executor,
		api.TimelineQuerySourceStorage,
		nil,
		nil,
		backend.AnalysisStore(),
		backend,
		backend,
		nil,
		time.Minute,
		NamespaceGraphCacheConfig{},
		"",
		nil,
		nil,
	)
}

func assertRouteStatus(t *testing.T, handler http.Handler, method, path string, expected int) {
	t.Helper()

	req := httptest.NewRequest(method, path, http.NoBody)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	require.Equal(t, expected, recorder.Code, "unexpected status for %s %s: %s", method, path, recorder.Body.String())
}

func assertRouteStatusWithBody(t *testing.T, handler http.Handler, method, path, body string, expected int) {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	require.Equal(t, expected, recorder.Code, "unexpected status for %s %s: %s", method, path, recorder.Body.String())
}

func TestServer_EmbeddedMode_RouteSurface(t *testing.T) {
	server := newEmbeddedTestServer(t)

	handler := server.server.Handler
	require.NotNil(t, handler)

	assertRouteStatus(t, handler, http.MethodGet, "/v1/search?start=1&end=2", http.StatusNotFound)
	assertRouteStatus(t, handler, http.MethodGet, "/v1/timeline?start=1&end=2", http.StatusOK)
	assertRouteStatus(t, handler, http.MethodGet, "/v1/metadata", http.StatusOK)
	assertRouteStatusWithBody(t, handler, http.MethodPost, "/v1/storage/import", `{"events":[{"id":"evt-1","timestamp":1,"type":"CREATE","resource":{"kind":"Pod","version":"v1","name":"pod-1","namespace":"default","uid":"pod-uid"}}]}`, http.StatusOK)
	assertRouteStatus(t, handler, http.MethodGet, "/v1/storage/export?from=0&to=1", http.StatusOK)

	assertRouteStatus(t, handler, http.MethodGet, "/v1/causal-graph", http.StatusNotFound)
	assertRouteStatus(t, handler, http.MethodGet, "/v1/anomalies", http.StatusNotFound)
	assertRouteStatus(t, handler, http.MethodGet, "/v1/causal-paths", http.StatusNotFound)
	assertRouteStatus(t, handler, http.MethodGet, "/v1/namespace-graph", http.StatusBadRequest)
	assertRouteStatus(t, handler, http.MethodGet, "/v1/observatory-graph", http.StatusNotFound)
	assertRouteStatus(t, handler, http.MethodPost, "/v1/mcp", http.StatusNotFound)
}

func TestServer_EmbeddedMode_CompareRouteAbsent(t *testing.T) {
	server := newEmbeddedCompareTestServer(t)

	handler := server.server.Handler
	require.NotNil(t, handler)

	assertRouteStatus(t, handler, http.MethodGet, "/v1/timeline/compare?start=1&end=2", http.StatusNotFound)
}
