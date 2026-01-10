//go:build integration
// +build integration

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	causalpaths "github.com/moolen/spectre/internal/analysis/causal_paths"
	"github.com/moolen/spectre/internal/api/handlers"
	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/require"
)

// TestCausalPathsHandler_FluxHelmRelease tests the causal paths handler
// for a Flux-managed HelmRelease scenario.
func TestCausalPathsHandler_FluxHelmRelease(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	ctx := context.Background()

	// Load the JSONL fixture
	_, testFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(testFile)
	fixturePath := filepath.Join(testDir, "..", "fixtures", "testrootcause-fluxhelmrelease-endpoint-e.jsonl")
	err = harness.SeedEventsFromAuditLog(ctx, fixturePath)
	require.NoError(t, err)

	// Extract timestamp and pod UID from the JSONL file
	timestamp, podUID, err := ExtractTimestampAndPodUIDFromFile(fixturePath)
	require.NoError(t, err)
	require.NotEmpty(t, podUID, "Pod UID should be extracted from JSONL")
	require.NotZero(t, timestamp, "Timestamp should be extracted from JSONL")

	t.Logf("Using pod UID: %s, timestamp: %d", podUID, timestamp)

	// Create the handler
	logger := logging.GetLogger("test")
	handler := handlers.NewCausalPathsHandler(harness.GetClient(), logger, nil)

	// Create HTTP request
	req := httptest.NewRequest(http.MethodGet, "/v1/causal-paths", nil)
	q := req.URL.Query()
	q.Set("resourceUID", podUID)
	q.Set("failureTimestamp", strconv.FormatInt(timestamp, 10))
	q.Set("lookback", "10m")
	q.Set("maxDepth", "5")
	q.Set("maxPaths", "5")
	req.URL.RawQuery = q.Encode()

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute handler
	handler.Handle(rr, req)

	// Verify response
	require.Equal(t, http.StatusOK, rr.Code, "Expected status OK, got: %s", rr.Body.String())

	// Parse response
	var result causalpaths.CausalPathsResponse
	err = json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err, "Failed to unmarshal response: %s", rr.Body.String())

	// Verify response structure
	require.NotNil(t, result.Metadata, "Metadata should not be nil")
	require.Greater(t, result.Metadata.NodesExplored, 0, "Should have explored at least one node")

	t.Logf("Causal paths discovery found %d paths, explored %d nodes",
		len(result.Paths), result.Metadata.NodesExplored)

	// Log path details for debugging
	for i, path := range result.Paths {
		t.Logf("  Path %d: confidence=%.2f, root=%s/%s (%s), steps=%d",
			i+1, path.ConfidenceScore,
			path.CandidateRoot.Resource.Namespace, path.CandidateRoot.Resource.Name,
			path.CandidateRoot.Resource.Kind, len(path.Steps))
		for j, step := range path.Steps {
			edgeInfo := ""
			if step.Edge != nil {
				edgeInfo = " via " + step.Edge.RelationshipType
			}
			t.Logf("    Step %d: %s/%s (%s)%s",
				j+1, step.Node.Resource.Namespace, step.Node.Resource.Name,
				step.Node.Resource.Kind, edgeInfo)
		}
	}

	// Verify paths have expected structure
	if len(result.Paths) > 0 {
		path := result.Paths[0]
		require.NotEmpty(t, path.ID, "Path should have an ID")
		require.NotEmpty(t, path.Steps, "Path should have steps")
		require.GreaterOrEqual(t, path.ConfidenceScore, 0.0, "Confidence should be >= 0")
		require.LessOrEqual(t, path.ConfidenceScore, 1.0, "Confidence should be <= 1")
	}
}

// TestCausalPathsHandler_FluxKustomization tests the causal paths handler
// for a Flux Kustomization scenario.
func TestCausalPathsHandler_FluxKustomization(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	ctx := context.Background()

	// Load the JSONL fixture
	_, testFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(testFile)
	fixturePath := filepath.Join(testDir, "..", "fixtures", "testrootcause-fluxkustomization-endpoint.jsonl")
	err = harness.SeedEventsFromAuditLog(ctx, fixturePath)
	require.NoError(t, err)

	// Extract timestamp and pod UID from the JSONL file
	timestamp, podUID, err := ExtractTimestampAndPodUIDFromFile(fixturePath)
	require.NoError(t, err)
	require.NotEmpty(t, podUID, "Pod UID should be extracted from JSONL")
	require.NotZero(t, timestamp, "Timestamp should be extracted from JSONL")

	t.Logf("Using pod UID: %s, timestamp: %d", podUID, timestamp)

	// Create the handler
	logger := logging.GetLogger("test")
	handler := handlers.NewCausalPathsHandler(harness.GetClient(), logger, nil)

	// Create HTTP request
	req := httptest.NewRequest(http.MethodGet, "/v1/causal-paths", nil)
	q := req.URL.Query()
	q.Set("resourceUID", podUID)
	q.Set("failureTimestamp", strconv.FormatInt(timestamp, 10))
	q.Set("lookback", "10m")
	q.Set("maxDepth", "5")
	q.Set("maxPaths", "5")
	req.URL.RawQuery = q.Encode()

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute handler
	handler.Handle(rr, req)

	// Verify response
	require.Equal(t, http.StatusOK, rr.Code, "Expected status OK, got: %s", rr.Body.String())

	// Parse response
	var result causalpaths.CausalPathsResponse
	err = json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err, "Failed to unmarshal response")

	// Verify response structure
	require.NotNil(t, result.Metadata, "Metadata should not be nil")
	require.Greater(t, result.Metadata.NodesExplored, 0, "Should have explored at least one node")

	t.Logf("Causal paths discovery found %d paths, explored %d nodes",
		len(result.Paths), result.Metadata.NodesExplored)

	// Log path details for debugging
	for i, path := range result.Paths {
		t.Logf("  Path %d: confidence=%.2f, root=%s/%s (%s), steps=%d",
			i+1, path.ConfidenceScore,
			path.CandidateRoot.Resource.Namespace, path.CandidateRoot.Resource.Name,
			path.CandidateRoot.Resource.Kind, len(path.Steps))
	}
}

// TestCausalPathsHandler_StatefulSet tests the causal paths handler
// for a StatefulSet scenario.
func TestCausalPathsHandler_StatefulSet(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	ctx := context.Background()

	// Load the JSONL fixture
	_, testFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(testFile)
	fixturePath := filepath.Join(testDir, "..", "fixtures", "testrootcause-statefulsetimagechange-end.jsonl")
	err = harness.SeedEventsFromAuditLog(ctx, fixturePath)
	require.NoError(t, err)

	// Extract timestamp and pod UID from the JSONL file
	timestamp, podUID, err := ExtractTimestampAndPodUIDFromFile(fixturePath)
	require.NoError(t, err)
	require.NotEmpty(t, podUID, "Pod UID should be extracted from JSONL")
	require.NotZero(t, timestamp, "Timestamp should be extracted from JSONL")

	t.Logf("Using pod UID: %s, timestamp: %d", podUID, timestamp)

	// Create the handler
	logger := logging.GetLogger("test")
	handler := handlers.NewCausalPathsHandler(harness.GetClient(), logger, nil)

	// Create HTTP request
	req := httptest.NewRequest(http.MethodGet, "/v1/causal-paths", nil)
	q := req.URL.Query()
	q.Set("resourceUID", podUID)
	q.Set("failureTimestamp", strconv.FormatInt(timestamp, 10))
	q.Set("lookback", "10m")
	q.Set("maxDepth", "5")
	q.Set("maxPaths", "5")
	req.URL.RawQuery = q.Encode()

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute handler
	handler.Handle(rr, req)

	// Verify response
	require.Equal(t, http.StatusOK, rr.Code, "Expected status OK, got: %s", rr.Body.String())

	// Parse response
	var result causalpaths.CausalPathsResponse
	err = json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err, "Failed to unmarshal response")

	// Verify response structure
	require.NotNil(t, result.Metadata, "Metadata should not be nil")
	require.Greater(t, result.Metadata.NodesExplored, 0, "Should have explored at least one node")

	t.Logf("Causal paths discovery found %d paths, explored %d nodes",
		len(result.Paths), result.Metadata.NodesExplored)

	// Log path details for debugging
	for i, path := range result.Paths {
		t.Logf("  Path %d: confidence=%.2f, root=%s/%s (%s), steps=%d",
			i+1, path.ConfidenceScore,
			path.CandidateRoot.Resource.Namespace, path.CandidateRoot.Resource.Name,
			path.CandidateRoot.Resource.Kind, len(path.Steps))
	}
}

// TestCausalPathsHandler_NetworkPolicy tests the causal paths handler
// for a NetworkPolicy scenario.
func TestCausalPathsHandler_NetworkPolicy(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	ctx := context.Background()

	// Load the JSONL fixture
	_, testFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(testFile)
	fixturePath := filepath.Join(testDir, "..", "fixtures", "testrootcause-networkpolicy-samenamespac.jsonl")
	err = harness.SeedEventsFromAuditLog(ctx, fixturePath)
	require.NoError(t, err)

	// Extract timestamp and pod UID from the JSONL file
	timestamp, podUID, err := ExtractTimestampAndPodUIDFromFile(fixturePath)
	require.NoError(t, err)
	require.NotEmpty(t, podUID, "Pod UID should be extracted from JSONL")
	require.NotZero(t, timestamp, "Timestamp should be extracted from JSONL")

	t.Logf("Using pod UID: %s, timestamp: %d", podUID, timestamp)

	// Create the handler
	logger := logging.GetLogger("test")
	handler := handlers.NewCausalPathsHandler(harness.GetClient(), logger, nil)

	// Create HTTP request
	req := httptest.NewRequest(http.MethodGet, "/v1/causal-paths", nil)
	q := req.URL.Query()
	q.Set("resourceUID", podUID)
	q.Set("failureTimestamp", strconv.FormatInt(timestamp, 10))
	q.Set("lookback", "10m")
	q.Set("maxDepth", "5")
	q.Set("maxPaths", "5")
	req.URL.RawQuery = q.Encode()

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute handler
	handler.Handle(rr, req)

	// Verify response
	require.Equal(t, http.StatusOK, rr.Code, "Expected status OK, got: %s", rr.Body.String())

	// Parse response
	var result causalpaths.CausalPathsResponse
	err = json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err, "Failed to unmarshal response")

	// Verify response structure
	require.NotNil(t, result.Metadata, "Metadata should not be nil")
	require.Greater(t, result.Metadata.NodesExplored, 0, "Should have explored at least one node")

	t.Logf("Causal paths discovery found %d paths, explored %d nodes",
		len(result.Paths), result.Metadata.NodesExplored)

	// Log path details for debugging
	for i, path := range result.Paths {
		t.Logf("  Path %d: confidence=%.2f, root=%s/%s (%s), steps=%d",
			i+1, path.ConfidenceScore,
			path.CandidateRoot.Resource.Namespace, path.CandidateRoot.Resource.Name,
			path.CandidateRoot.Resource.Kind, len(path.Steps))
	}
}

// TestCausalPathsHandler_Ingress tests the causal paths handler
// for an Ingress scenario.
func TestCausalPathsHandler_Ingress(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	ctx := context.Background()

	// Load the JSONL fixture
	_, testFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(testFile)
	fixturePath := filepath.Join(testDir, "..", "fixtures", "testrootcause-ingress-samenamespace-endp.jsonl")
	err = harness.SeedEventsFromAuditLog(ctx, fixturePath)
	require.NoError(t, err)

	// Extract timestamp and pod UID from the JSONL file
	timestamp, podUID, err := ExtractTimestampAndPodUIDFromFile(fixturePath)
	require.NoError(t, err)
	require.NotEmpty(t, podUID, "Pod UID should be extracted from JSONL")
	require.NotZero(t, timestamp, "Timestamp should be extracted from JSONL")

	t.Logf("Using pod UID: %s, timestamp: %d", podUID, timestamp)

	// Create the handler
	logger := logging.GetLogger("test")
	handler := handlers.NewCausalPathsHandler(harness.GetClient(), logger, nil)

	// Create HTTP request
	req := httptest.NewRequest(http.MethodGet, "/v1/causal-paths", nil)
	q := req.URL.Query()
	q.Set("resourceUID", podUID)
	q.Set("failureTimestamp", strconv.FormatInt(timestamp, 10))
	q.Set("lookback", "10m")
	q.Set("maxDepth", "5")
	q.Set("maxPaths", "5")
	req.URL.RawQuery = q.Encode()

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute handler
	handler.Handle(rr, req)

	// Verify response
	require.Equal(t, http.StatusOK, rr.Code, "Expected status OK, got: %s", rr.Body.String())

	// Parse response
	var result causalpaths.CausalPathsResponse
	err = json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err, "Failed to unmarshal response")

	// Verify response structure
	require.NotNil(t, result.Metadata, "Metadata should not be nil")
	require.Greater(t, result.Metadata.NodesExplored, 0, "Should have explored at least one node")

	t.Logf("Causal paths discovery found %d paths, explored %d nodes",
		len(result.Paths), result.Metadata.NodesExplored)

	// Log path details for debugging
	for i, path := range result.Paths {
		t.Logf("  Path %d: confidence=%.2f, root=%s/%s (%s), steps=%d",
			i+1, path.ConfidenceScore,
			path.CandidateRoot.Resource.Namespace, path.CandidateRoot.Resource.Name,
			path.CandidateRoot.Resource.Kind, len(path.Steps))
	}
}
