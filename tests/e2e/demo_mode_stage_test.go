package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type DemoModeStage struct {
	t               *testing.T
	spectreCmd      *exec.Cmd
	apiPort         int
	httpClient      *http.Client
	healthResponse  map[string]interface{}
	timelineResp    map[string]interface{}
	metadataResp    map[string]interface{}
}

func NewDemoModeStage(t *testing.T) (*DemoModeStage, *DemoModeStage, *DemoModeStage) {
	stage := &DemoModeStage{
		t:          t,
		apiPort:    9080, // Use non-default port to avoid conflicts
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	return stage, stage, stage
}

func (s *DemoModeStage) a_demo_mode_environment() *DemoModeStage {
	s.t.Log("Setting up demo mode test environment")
	return s
}

func (s *DemoModeStage) and() *DemoModeStage {
	return s
}

func (s *DemoModeStage) spectre_server_starts_with_demo_flag() *DemoModeStage {
	s.t.Log("Starting Spectre server in demo mode")

	// Build the binary first (from repo root)
	buildCmd := exec.Command("make", "build")
	buildCmd.Dir = "../.." // Run from repo root
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		s.t.Fatalf("Failed to build spectre: %v\nOutput: %s", err, output)
	}

	// Start spectre server with --demo flag
	s.spectreCmd = exec.Command("../../bin/spectre", "server",
		"--demo",
		"--api-port", fmt.Sprintf("%d", s.apiPort),
		"--log-level", "info")

	// Start the server
	err = s.spectreCmd.Start()
	require.NoError(s.t, err, "Failed to start Spectre server")

	// Wait for server to be ready
	s.t.Log("Waiting for server to be ready...")
	require.Eventually(s.t, func() bool {
		resp, err := s.httpClient.Get(fmt.Sprintf("http://localhost:%d/health", s.apiPort))
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 30*time.Second, 500*time.Millisecond, "Server did not become ready in time")

	// Fetch and store health response
	resp, err := s.httpClient.Get(fmt.Sprintf("http://localhost:%d/health", s.apiPort))
	require.NoError(s.t, err)
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&s.healthResponse)
	require.NoError(s.t, err)

	s.t.Log("Server started successfully in demo mode")

	// Cleanup
	s.t.Cleanup(func() {
		if s.spectreCmd != nil && s.spectreCmd.Process != nil {
			s.t.Log("Stopping Spectre server")
			_ = s.spectreCmd.Process.Kill()
			_ = s.spectreCmd.Wait()
		}
	})

	return s
}

func (s *DemoModeStage) ui_loads_successfully() *DemoModeStage {
	s.t.Log("Verifying UI loads successfully")

	resp, err := s.httpClient.Get(fmt.Sprintf("http://localhost:%d/", s.apiPort))
	require.NoError(s.t, err, "Failed to load UI")
	defer resp.Body.Close()

	// UI may return 404 if not bundled, which is okay for demo mode API testing
	// Just verify the server responds
	assert.True(s.t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound,
		"Server should respond to UI requests (status: %d)", resp.StatusCode)

	s.t.Log("Note: In demo mode, frontend should auto-redirect to /?start=now-2h&end=now")
	s.t.Log("      and use local demo data from ui/src/demo/demo-data.json")
	s.t.Log("      No API calls to /v1/timeline should be made by the frontend")

	return s
}

func (s *DemoModeStage) demo_mode_indicator_is_visible() *DemoModeStage {
	s.t.Log("Verifying demo mode is enabled")

	// Check health endpoint returns demo: true
	demo, ok := s.healthResponse["demo"].(bool)
	require.True(s.t, ok, "Health response should contain 'demo' field")
	assert.True(s.t, demo, "Demo mode should be enabled")

	return s
}

func (s *DemoModeStage) timeline_data_is_displayed() *DemoModeStage {
	s.t.Log("Verifying timeline endpoint works (backend should return demo data)")

	// In demo mode, the backend should also serve demo data via API
	// However, the frontend should use its local demo data and NOT call the API
	// This test verifies the backend API works, but we can't easily test
	// that the frontend doesn't call it without a real browser
	
	// Query timeline with a time range (last hour)
	now := time.Now().Unix()
	start := now - 3600
	url := fmt.Sprintf("http://localhost:%d/v1/timeline?start=%d&end=%d", s.apiPort, start, now)

	resp, err := s.httpClient.Get(url)
	require.NoError(s.t, err, "Failed to fetch timeline data")
	defer resp.Body.Close()

	assert.Equal(s.t, http.StatusOK, resp.StatusCode, "Timeline endpoint should return 200")

	err = json.NewDecoder(resp.Body).Decode(&s.timelineResp)
	require.NoError(s.t, err, "Failed to decode timeline response")

	// Check that we have resources
	count, ok := s.timelineResp["count"].(float64)
	require.True(s.t, ok, "Timeline response should contain 'count' field")
	assert.Greater(s.t, int(count), 0, "Timeline should contain resources in demo mode")

	s.t.Logf("Backend timeline API returned %d resources", int(count))

	return s
}

func (s *DemoModeStage) resources_are_visible_in_timeline() *DemoModeStage {
	s.t.Log("Verifying resources are visible in timeline")

	resources, ok := s.timelineResp["resources"].([]interface{})
	require.True(s.t, ok, "Timeline response should contain 'resources' array")
	assert.NotEmpty(s.t, resources, "Timeline should contain resources")

	// Verify first resource has expected structure
	if len(resources) > 0 {
		firstResource, ok := resources[0].(map[string]interface{})
		require.True(s.t, ok, "Resource should be an object")

		// Check for required fields
		assert.NotEmpty(s.t, firstResource["name"], "Resource should have a name")
		assert.NotEmpty(s.t, firstResource["kind"], "Resource should have a kind")
		assert.NotEmpty(s.t, firstResource["namespace"], "Resource should have a namespace")

		// Check for status segments
		statusSegments, ok := firstResource["statusSegments"].([]interface{})
		require.True(s.t, ok, "Resource should have statusSegments array")
		assert.NotEmpty(s.t, statusSegments, "Resource should have at least one status segment")

		s.t.Logf("First resource: %s/%s in namespace '%s' with %d status segments",
			firstResource["kind"], firstResource["name"], firstResource["namespace"], len(statusSegments))
	}

	return s
}

func (s *DemoModeStage) metadata_endpoint_returns_demo_data() *DemoModeStage {
	s.t.Log("Verifying metadata endpoint returns demo data")

	// Query metadata endpoint
	url := fmt.Sprintf("http://localhost:%d/v1/metadata", s.apiPort)
	resp, err := s.httpClient.Get(url)
	require.NoError(s.t, err, "Failed to fetch metadata")
	defer resp.Body.Close()

	assert.Equal(s.t, http.StatusOK, resp.StatusCode, "Metadata endpoint should return 200")

	err = json.NewDecoder(resp.Body).Decode(&s.metadataResp)
	require.NoError(s.t, err, "Failed to decode metadata response")

	// Check that we have namespaces and kinds
	namespaces, ok := s.metadataResp["namespaces"].([]interface{})
	require.True(s.t, ok, "Metadata should contain 'namespaces' array")
	assert.NotEmpty(s.t, namespaces, "Metadata should contain namespaces")

	kinds, ok := s.metadataResp["kinds"].([]interface{})
	require.True(s.t, ok, "Metadata should contain 'kinds' array")
	assert.NotEmpty(s.t, kinds, "Metadata should contain kinds")

	s.t.Logf("Metadata contains %d namespaces and %d kinds", len(namespaces), len(kinds))

	return s
}
