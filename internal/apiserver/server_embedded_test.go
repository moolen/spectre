package apiserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/embedded"
	"github.com/stretchr/testify/require"
)

func newEmbeddedTestServer(t *testing.T) *Server {
	t.Helper()

	executor, err := embedded.NewQueryExecutor(nil)
	require.NoError(t, err)

	return NewWithStorageGraphAndPipeline(
		0,
		executor,
		nil,
		api.TimelineQuerySourceStorage,
		nil,
		nil,
		nil,
		&NoOpReadinessChecker{},
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

func TestServer_EmbeddedMode_RouteSurface(t *testing.T) {
	server := newEmbeddedTestServer(t)

	handler := server.server.Handler
	require.NotNil(t, handler)

	assertRouteStatus(t, handler, http.MethodGet, "/v1/timeline?start=1&end=2", http.StatusOK)
	assertRouteStatus(t, handler, http.MethodGet, "/v1/metadata", http.StatusOK)

	assertRouteStatus(t, handler, http.MethodPost, "/v1/storage/import", http.StatusNotFound)
	assertRouteStatus(t, handler, http.MethodGet, "/v1/causal-graph", http.StatusNotFound)
	assertRouteStatus(t, handler, http.MethodPost, "/v1/mcp", http.StatusNotFound)
}
