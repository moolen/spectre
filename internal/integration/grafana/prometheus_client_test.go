package grafana

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrometheusClient_GetTargets(t *testing.T) {
	logger := logging.GetLogger("test")

	testCases := []struct {
		name           string
		serverResponse interface{}
		statusCode     int
		expectedCount  int
		expectError    bool
	}{
		{
			name: "returns healthy targets only",
			serverResponse: map[string]interface{}{
				"status": "success",
				"data": map[string]interface{}{
					"activeTargets": []map[string]interface{}{
						{
							"labels": map[string]string{
								"namespace": "default",
								"pod":       "nginx-abc123",
								"app":       "nginx",
							},
							"scrapePool":         "kubernetes-pods",
							"scrapeUrl":          "http://10.0.0.1:9090/metrics",
							"health":             "up",
							"lastScrape":         "2026-01-23T10:00:00Z",
							"lastScrapeDuration": 0.05,
						},
						{
							"labels": map[string]string{
								"namespace": "monitoring",
								"pod":       "prometheus-0",
							},
							"scrapePool":         "kubernetes-pods",
							"scrapeUrl":          "http://10.0.0.2:9090/metrics",
							"health":             "down",
							"lastScrape":         "2026-01-23T10:00:00Z",
							"lastScrapeDuration": 0.1,
						},
					},
				},
			},
			statusCode:    http.StatusOK,
			expectedCount: 1, // Only healthy target
			expectError:   false,
		},
		{
			name: "returns all healthy targets",
			serverResponse: map[string]interface{}{
				"status": "success",
				"data": map[string]interface{}{
					"activeTargets": []map[string]interface{}{
						{
							"labels": map[string]string{
								"namespace": "default",
								"pod":       "nginx-1",
							},
							"scrapePool": "kubernetes-pods",
							"health":     "up",
						},
						{
							"labels": map[string]string{
								"namespace": "default",
								"pod":       "nginx-2",
							},
							"scrapePool": "kubernetes-pods",
							"health":     "up",
						},
					},
				},
			},
			statusCode:    http.StatusOK,
			expectedCount: 2,
			expectError:   false,
		},
		{
			name: "empty targets",
			serverResponse: map[string]interface{}{
				"status": "success",
				"data": map[string]interface{}{
					"activeTargets": []map[string]interface{}{},
				},
			},
			statusCode:    http.StatusOK,
			expectedCount: 0,
			expectError:   false,
		},
		{
			name: "error status in response",
			serverResponse: map[string]interface{}{
				"status": "error",
				"error":  "something went wrong",
			},
			statusCode:    http.StatusOK,
			expectedCount: 0,
			expectError:   true,
		},
		{
			name:           "HTTP error",
			serverResponse: "Internal Server Error",
			statusCode:     http.StatusInternalServerError,
			expectedCount:  0,
			expectError:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/targets", r.URL.Path)
				assert.Equal(t, "active", r.URL.Query().Get("state"))

				w.WriteHeader(tc.statusCode)
				if tc.statusCode == http.StatusOK {
					json.NewEncoder(w).Encode(tc.serverResponse)
				} else {
					w.Write([]byte(tc.serverResponse.(string)))
				}
			}))
			defer server.Close()

			client := NewPrometheusClient(server.URL, nil, nil, logger)
			targets, err := client.GetTargets(context.Background())

			if tc.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, targets, tc.expectedCount)
			}
		})
	}
}

func TestPrometheusClient_TargetParsing(t *testing.T) {
	logger := logging.GetLogger("test")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status": "success",
			"data": map[string]interface{}{
				"activeTargets": []map[string]interface{}{
					{
						"labels": map[string]string{
							"namespace":              "production",
							"pod":                    "api-server-abc123",
							"app_kubernetes_io_name": "api-server",
							"job":                    "kubernetes-pods",
						},
						"scrapePool":         "kubernetes-pods",
						"scrapeUrl":          "http://10.0.0.1:8080/metrics",
						"health":             "up",
						"lastScrape":         "2026-01-23T10:00:00.123456789Z",
						"lastScrapeDuration": 0.042,
					},
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := NewPrometheusClient(server.URL, nil, nil, logger)
	targets, err := client.GetTargets(context.Background())

	require.NoError(t, err)
	require.Len(t, targets, 1)

	target := targets[0]
	assert.Equal(t, "production", target.Labels["namespace"])
	assert.Equal(t, "api-server-abc123", target.Labels["pod"])
	assert.Equal(t, "api-server", target.Labels["app_kubernetes_io_name"])
	assert.Equal(t, "kubernetes-pods", target.ScrapePool)
	assert.Equal(t, "up", target.Health)
	assert.Equal(t, 42*time.Millisecond, target.LastScrapeDuration)
	assert.False(t, target.LastScrape.IsZero())
}

func TestPrometheusClient_TestConnection(t *testing.T) {
	logger := logging.GetLogger("test")

	t.Run("successful connection", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := map[string]interface{}{
				"status": "success",
				"data":   map[string]interface{}{"activeTargets": []interface{}{}},
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewPrometheusClient(server.URL, nil, nil, logger)
		err := client.TestConnection(context.Background())
		assert.NoError(t, err)
	})

	t.Run("failed connection", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("unauthorized"))
		}))
		defer server.Close()

		client := NewPrometheusClient(server.URL, nil, nil, logger)
		err := client.TestConnection(context.Background())
		assert.Error(t, err)
	})
}

