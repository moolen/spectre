package apiserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServer_MetricsEndpoint(t *testing.T) {
	server := newEmbeddedTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/metrics", http.NoBody)
	recorder := httptest.NewRecorder()

	server.server.Handler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Header().Get("Content-Type"), "text/plain")
	require.NotContains(t, strings.ToLower(recorder.Body.String()), "<html")
	require.Contains(t, recorder.Body.String(), "go_gc_duration_seconds")
}
