package helpers

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPortForwarderWaitForReadyPrefersReadyEndpoint(t *testing.T) {
	readyAfter := 1200 * time.Millisecond
	serverStart := time.Now()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/ready":
			if time.Since(serverStart) >= readyAfter {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	pf := &PortForwarder{
		LocalPort: serverPort(t, server),
		t:         t,
	}

	start := time.Now()
	err := pf.WaitForReady(5 * time.Second)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.GreaterOrEqual(t, elapsed, 1*time.Second, "WaitForReady must wait for /ready, not return early on /health")
}

func TestPortForwarderWaitForReadyFallsBackToHealthWhenReadyEndpointMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	pf := &PortForwarder{
		LocalPort: serverPort(t, server),
		t:         t,
	}

	require.NoError(t, pf.WaitForReady(3*time.Second))
}

func serverPort(t *testing.T, server *httptest.Server) uint16 {
	t.Helper()

	addr, ok := server.Listener.Addr().(*net.TCPAddr)
	require.True(t, ok, "test server must expose a TCP address")
	return uint16(addr.Port)
}
