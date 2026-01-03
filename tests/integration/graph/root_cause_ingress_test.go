//go:build integration
// +build integration

package graph

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	"github.com/stretchr/testify/require"
)

// TestRootCauseEndpoint_Ingress_SameNamespace tests root cause analysis
// for an Ingress routing to a Service that selects pods.
func TestRootCauseEndpoint_Ingress_SameNamespace(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	ctx := context.Background()

	// Load the JSONL fixture
	_, testFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(testFile)
	fixturePath := filepath.Join(testDir, "fixtures", "testrootcause-ingress-samenamespace-endp.jsonl")
	err = harness.SeedEventsFromAuditLog(ctx, fixturePath)
	require.NoError(t, err)

	// Extract timestamp and pod UID from the JSONL file
	timestamp, podUID, err := extractTimestampAndPodUID(fixturePath)
	require.NoError(t, err)
	require.NotEmpty(t, podUID, "Pod UID should be extracted from JSONL")
	require.NotZero(t, timestamp, "Timestamp should be extracted from JSONL")

	t.Logf("Using pod UID: %s, timestamp: %d", podUID, timestamp)

	// Create RootCauseAnalyzer
	analyzer := analysis.NewRootCauseAnalyzer(harness.GetClient())

	// Perform root cause analysis
	lookback := 10 * time.Minute
	rca, err := analyzer.Analyze(ctx, analysis.AnalyzeInput{
		ResourceUID:      podUID,
		FailureTimestamp: timestamp,
		LookbackNs:       lookback.Nanoseconds(),
		MaxDepth:         5,
		MinConfidence:    0.6,
	})
	require.NoError(t, err)
	require.NotNil(t, rca)

	// Assert graph structure (same as e2e test)
	assertGraphHasDeploymentOwnsPod(t, rca)
	assertGraphHasService(t, rca)
	assertGraphHasServiceSelectsPod(t, rca)
	assertGraphHasIngress(t, rca)
	assertGraphHasIngressReferencesService(t, rca)
}

// Helper functions for Ingress assertions

// assertGraphHasService verifies that Service node exists
func assertGraphHasService(t *testing.T, rca *analysis.RootCauseAnalysisV2) {
	require.NotNil(t, rca, "Root cause analysis should not be nil")
	require.NotNil(t, rca.Incident, "Incident should not be nil")
	require.NotNil(t, rca.Incident.Graph, "Graph should not be nil")

	serviceNode := findNodeByKind(rca, "Service")
	require.NotNil(t, serviceNode, "Graph should contain Service node")
	t.Logf("✓ Found Service node: %s", serviceNode.Resource.Name)
}

// assertGraphHasServiceSelectsPod verifies that Service selects Pod
func assertGraphHasServiceSelectsPod(t *testing.T, rca *analysis.RootCauseAnalysisV2) {
	assertGraphHasEdgeBetweenKinds(t, rca, "Service", "SELECTS", "Pod")
	t.Logf("✓ Found SELECTS edge from Service to Pod")
}

// assertGraphHasIngress verifies that Ingress node exists
func assertGraphHasIngress(t *testing.T, rca *analysis.RootCauseAnalysisV2) {
	require.NotNil(t, rca, "Root cause analysis should not be nil")
	require.NotNil(t, rca.Incident, "Incident should not be nil")
	require.NotNil(t, rca.Incident.Graph, "Graph should not be nil")
	require.Greater(t, len(rca.Incident.Graph.Nodes), 0, "Graph should have at least one node")
	require.Greater(t, len(rca.Incident.Graph.Edges), 0, "Graph should have at least one edge")

	// Check that Ingress node exists
	ingressNode := findNodeByKind(rca, "Ingress")
	require.NotNil(t, ingressNode, "Graph should contain Ingress node")
	t.Logf("✓ Found Ingress node: %s", ingressNode.Resource.Name)
}

// assertGraphHasIngressReferencesService verifies that Ingress references Service
func assertGraphHasIngressReferencesService(t *testing.T, rca *analysis.RootCauseAnalysisV2) {
	assertGraphHasEdgeBetweenKinds(t, rca, "Ingress", "REFERENCES_SPEC", "Service")
	t.Logf("✓ Found REFERENCES_SPEC edge from Ingress to Service")
}
