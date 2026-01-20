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
	"testing"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	"github.com/moolen/spectre/internal/api/handlers"
	"github.com/moolen/spectre/internal/logging"
	"github.com/stretchr/testify/require"
)

// TestCausalGraphHandler_FluxHelmRelease tests the causal graph handler
// for a Flux-managed HelmRelease scenario.
func TestCausalGraphHandler_FluxHelmRelease(t *testing.T) {
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
	handler := handlers.NewCausalGraphHandler(harness.GetClient(), logger, nil)

	// Create HTTP request
	req := httptest.NewRequest(http.MethodGet, "/v1/causal-graph", nil)
	q := req.URL.Query()
	q.Set("resourceUID", podUID)
	q.Set("failureTimestamp", formatTimestamp(timestamp))
	q.Set("lookback", "10m")
	q.Set("maxDepth", "5")
	q.Set("format", "diff")
	req.URL.RawQuery = q.Encode()

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute handler
	handler.Handle(rr, req)

	// Verify response
	require.Equal(t, http.StatusOK, rr.Code, "Expected status OK, got: %s", rr.Body.String())

	// Parse response
	var result analysis.RootCauseAnalysisV2
	err = json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err, "Failed to unmarshal response: %s", rr.Body.String())

	// Verify graph structure
	require.NotNil(t, result.Incident, "Incident should not be nil")
	require.NotNil(t, result.Incident.Graph, "Graph should not be nil")
	require.Greater(t, len(result.Incident.Graph.Nodes), 0, "Graph should have at least one node")
	require.Greater(t, len(result.Incident.Graph.Edges), 0, "Graph should have at least one edge")

	// Verify expected node kinds exist
	nodeKinds := extractNodeKinds(&result)
	t.Logf("Found node kinds: %v", nodeKinds)

	require.Contains(t, nodeKinds, "HelmRelease", "Graph should contain HelmRelease")
	require.Contains(t, nodeKinds, "Deployment", "Graph should contain Deployment")
	require.Contains(t, nodeKinds, "ReplicaSet", "Graph should contain ReplicaSet")
	require.Contains(t, nodeKinds, "Pod", "Graph should contain Pod")

	// Verify expected edges
	assertEdgeBetweenKinds(t, &result, "HelmRelease", "MANAGES", "Deployment")
	assertEdgeBetweenKinds(t, &result, "Deployment", "OWNS", "ReplicaSet")
	assertEdgeBetweenKinds(t, &result, "ReplicaSet", "OWNS", "Pod")

	t.Logf("Graph contains %d nodes and %d edges", len(result.Incident.Graph.Nodes), len(result.Incident.Graph.Edges))
}

// TestCausalGraphHandler_FluxKustomization tests the causal graph handler
// for a Flux Kustomization managing a Deployment.
func TestCausalGraphHandler_FluxKustomization(t *testing.T) {
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
	handler := handlers.NewCausalGraphHandler(harness.GetClient(), logger, nil)

	// Create HTTP request
	req := httptest.NewRequest(http.MethodGet, "/v1/causal-graph", nil)
	q := req.URL.Query()
	q.Set("resourceUID", podUID)
	q.Set("failureTimestamp", formatTimestamp(timestamp))
	q.Set("lookback", "10m")
	q.Set("maxDepth", "5")
	req.URL.RawQuery = q.Encode()

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute handler
	handler.Handle(rr, req)

	// Verify response
	require.Equal(t, http.StatusOK, rr.Code, "Expected status OK, got: %s", rr.Body.String())

	// Parse response
	var result analysis.RootCauseAnalysisV2
	err = json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err, "Failed to unmarshal response")

	// Verify graph structure
	require.NotNil(t, result.Incident, "Incident should not be nil")
	require.NotNil(t, result.Incident.Graph, "Graph should not be nil")
	require.Greater(t, len(result.Incident.Graph.Nodes), 0, "Graph should have at least one node")
	require.Greater(t, len(result.Incident.Graph.Edges), 0, "Graph should have at least one edge")

	// Verify expected node kinds exist
	nodeKinds := extractNodeKinds(&result)
	t.Logf("Found node kinds: %v", nodeKinds)

	require.Contains(t, nodeKinds, "Kustomization", "Graph should contain Kustomization")
	require.Contains(t, nodeKinds, "Deployment", "Graph should contain Deployment")
	require.Contains(t, nodeKinds, "Pod", "Graph should contain Pod")

	// Verify expected edges
	assertEdgeBetweenKinds(t, &result, "Kustomization", "MANAGES", "Deployment")
	assertEdgeBetweenKinds(t, &result, "Deployment", "OWNS", "ReplicaSet")
	assertEdgeBetweenKinds(t, &result, "ReplicaSet", "OWNS", "Pod")

	t.Logf("Graph contains %d nodes and %d edges", len(result.Incident.Graph.Nodes), len(result.Incident.Graph.Edges))
}

// TestCausalGraphHandler_StatefulSet tests the causal graph handler
// for a StatefulSet scenario.
func TestCausalGraphHandler_StatefulSet(t *testing.T) {
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
	handler := handlers.NewCausalGraphHandler(harness.GetClient(), logger, nil)

	// Create HTTP request
	req := httptest.NewRequest(http.MethodGet, "/v1/causal-graph", nil)
	q := req.URL.Query()
	q.Set("resourceUID", podUID)
	q.Set("failureTimestamp", formatTimestamp(timestamp))
	q.Set("lookback", "10m")
	q.Set("maxDepth", "5")
	req.URL.RawQuery = q.Encode()

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute handler
	handler.Handle(rr, req)

	// Verify response
	require.Equal(t, http.StatusOK, rr.Code, "Expected status OK, got: %s", rr.Body.String())

	// Parse response
	var result analysis.RootCauseAnalysisV2
	err = json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err, "Failed to unmarshal response")

	// Verify graph structure
	require.NotNil(t, result.Incident, "Incident should not be nil")
	require.NotNil(t, result.Incident.Graph, "Graph should not be nil")
	require.Greater(t, len(result.Incident.Graph.Nodes), 0, "Graph should have at least one node")

	// Verify expected node kinds exist
	nodeKinds := extractNodeKinds(&result)
	t.Logf("Found node kinds: %v", nodeKinds)

	require.Contains(t, nodeKinds, "StatefulSet", "Graph should contain StatefulSet")
	require.Contains(t, nodeKinds, "Pod", "Graph should contain Pod")

	// Verify StatefulSet owns Pod
	assertEdgeBetweenKinds(t, &result, "StatefulSet", "OWNS", "Pod")

	t.Logf("Graph contains %d nodes and %d edges", len(result.Incident.Graph.Nodes), len(result.Incident.Graph.Edges))
}

// TestCausalGraphHandler_NetworkPolicy tests the causal graph handler
// for a NetworkPolicy scenario.
func TestCausalGraphHandler_NetworkPolicy(t *testing.T) {
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
	handler := handlers.NewCausalGraphHandler(harness.GetClient(), logger, nil)

	// Create HTTP request
	req := httptest.NewRequest(http.MethodGet, "/v1/causal-graph", nil)
	q := req.URL.Query()
	q.Set("resourceUID", podUID)
	q.Set("failureTimestamp", formatTimestamp(timestamp))
	q.Set("lookback", "10m")
	q.Set("maxDepth", "5")
	req.URL.RawQuery = q.Encode()

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute handler
	handler.Handle(rr, req)

	// Verify response
	require.Equal(t, http.StatusOK, rr.Code, "Expected status OK, got: %s", rr.Body.String())

	// Parse response
	var result analysis.RootCauseAnalysisV2
	err = json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err, "Failed to unmarshal response")

	// Verify graph structure
	require.NotNil(t, result.Incident, "Incident should not be nil")
	require.NotNil(t, result.Incident.Graph, "Graph should not be nil")
	require.Greater(t, len(result.Incident.Graph.Nodes), 0, "Graph should have at least one node")
	require.Greater(t, len(result.Incident.Graph.Edges), 0, "Graph should have at least one edge")

	// Verify expected node kinds exist
	nodeKinds := extractNodeKinds(&result)
	t.Logf("Found node kinds: %v", nodeKinds)

	require.Contains(t, nodeKinds, "NetworkPolicy", "Graph should contain NetworkPolicy")
	require.Contains(t, nodeKinds, "Pod", "Graph should contain Pod")

	// Verify NetworkPolicy selects Pod
	assertEdgeBetweenKinds(t, &result, "NetworkPolicy", "SELECTS", "Pod")

	t.Logf("Graph contains %d nodes and %d edges", len(result.Incident.Graph.Nodes), len(result.Incident.Graph.Edges))
}

// TestCausalGraphHandler_Ingress tests the causal graph handler
// for an Ingress scenario.
func TestCausalGraphHandler_Ingress(t *testing.T) {
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
	handler := handlers.NewCausalGraphHandler(harness.GetClient(), logger, nil)

	// Create HTTP request
	req := httptest.NewRequest(http.MethodGet, "/v1/causal-graph", nil)
	q := req.URL.Query()
	q.Set("resourceUID", podUID)
	q.Set("failureTimestamp", formatTimestamp(timestamp))
	q.Set("lookback", "10m")
	q.Set("maxDepth", "5")
	req.URL.RawQuery = q.Encode()

	// Create response recorder
	rr := httptest.NewRecorder()

	// Execute handler
	handler.Handle(rr, req)

	// Verify response
	require.Equal(t, http.StatusOK, rr.Code, "Expected status OK, got: %s", rr.Body.String())

	// Parse response
	var result analysis.RootCauseAnalysisV2
	err = json.Unmarshal(rr.Body.Bytes(), &result)
	require.NoError(t, err, "Failed to unmarshal response")

	// Verify graph structure
	require.NotNil(t, result.Incident, "Incident should not be nil")
	require.NotNil(t, result.Incident.Graph, "Graph should not be nil")
	require.Greater(t, len(result.Incident.Graph.Nodes), 0, "Graph should have at least one node")
	require.Greater(t, len(result.Incident.Graph.Edges), 0, "Graph should have at least one edge")

	// Verify expected node kinds exist
	nodeKinds := extractNodeKinds(&result)
	t.Logf("Found node kinds: %v", nodeKinds)

	require.Contains(t, nodeKinds, "Ingress", "Graph should contain Ingress")
	require.Contains(t, nodeKinds, "Service", "Graph should contain Service")
	require.Contains(t, nodeKinds, "Pod", "Graph should contain Pod")

	// Verify expected edges
	assertEdgeBetweenKinds(t, &result, "Ingress", "REFERENCES_SPEC", "Service")
	assertEdgeBetweenKinds(t, &result, "Service", "SELECTS", "Pod")

	t.Logf("Graph contains %d nodes and %d edges", len(result.Incident.Graph.Nodes), len(result.Incident.Graph.Edges))
}

// Helper functions

func formatTimestamp(ts int64) string {
	return time.Unix(0, ts).Format(time.RFC3339Nano)
}

func extractNodeKinds(result *analysis.RootCauseAnalysisV2) map[string]bool {
	kinds := make(map[string]bool)
	for _, node := range result.Incident.Graph.Nodes {
		kinds[node.Resource.Kind] = true
	}
	return kinds
}

func findNodeByKind(result *analysis.RootCauseAnalysisV2, kind string) *analysis.GraphNode {
	for i := range result.Incident.Graph.Nodes {
		if result.Incident.Graph.Nodes[i].Resource.Kind == kind {
			return &result.Incident.Graph.Nodes[i]
		}
	}
	return nil
}

func assertEdgeBetweenKinds(t *testing.T, result *analysis.RootCauseAnalysisV2, fromKind, relType, toKind string) {
	fromNode := findNodeByKind(result, fromKind)
	require.NotNil(t, fromNode, "Graph should contain node of kind %s", fromKind)

	toNode := findNodeByKind(result, toKind)
	require.NotNil(t, toNode, "Graph should contain node of kind %s", toKind)

	found := false
	for _, edge := range result.Incident.Graph.Edges {
		if edge.From == fromNode.ID && edge.To == toNode.ID && edge.RelationshipType == relType {
			found = true
			break
		}
	}

	require.True(t, found, "Graph should contain edge %s -[%s]-> %s", fromKind, relType, toKind)
	t.Logf("Found edge %s -[%s]-> %s", fromKind, relType, toKind)
}
