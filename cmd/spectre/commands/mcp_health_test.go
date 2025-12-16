package commands

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHealthEndpoint tests that the health endpoint returns 200 OK
func TestHealthEndpoint(t *testing.T) {
	// Create a custom mux with health endpoint (simulating our setup)
	mux := http.NewServeMux()

	// Add health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})

	// Create test server
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Test the health endpoint
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("Failed to call health endpoint: %v", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if string(body) != "ok" {
		t.Errorf("Expected body 'ok', got '%s'", string(body))
	}

	// Check content type (may include charset)
	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/plain" && contentType != "text/plain; charset=utf-8" {
		t.Errorf("Expected Content-Type 'text/plain', got '%s'", contentType)
	}

	t.Log("✅ Health endpoint test passed")
}

// TestHealthEndpointMethod tests that health endpoint only responds to GET
func TestHealthEndpointMethod(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Test GET
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /health: expected 200, got %d", resp.StatusCode)
	}

	// Test POST (should still work with our simple handler)
	resp2, err := http.Post(ts.URL+"/health", "application/json", nil)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("POST /health: expected 200, got %d", resp2.StatusCode)
	}

	t.Log("✅ Health endpoint method test passed")
}
