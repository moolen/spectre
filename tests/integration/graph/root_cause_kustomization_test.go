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

// TestRootCauseEndpoint_FluxKustomization tests root cause analysis
// for a Flux Kustomization managing a Deployment.
func TestRootCauseEndpoint_FluxKustomization(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	ctx := context.Background()

	// Load the JSONL fixture
	_, testFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(testFile)
	fixturePath := filepath.Join(testDir, "fixtures", "testrootcause-fluxkustomization-endpoint.jsonl")
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
	assertGraphHasKustomizationManagesDeployment(t, rca)
}

// Helper functions for Flux Kustomization assertions

// assertGraphHasKustomizationManagesDeployment verifies that Kustomization manages Deployment
func assertGraphHasKustomizationManagesDeployment(t *testing.T, rca *analysis.RootCauseAnalysisV2) {
	require.NotNil(t, rca, "Root cause analysis should not be nil")
	require.NotNil(t, rca.Incident, "Incident should not be nil")
	require.NotNil(t, rca.Incident.Graph, "Graph should not be nil")
	require.Greater(t, len(rca.Incident.Graph.Nodes), 0, "Graph should have at least one node")
	require.Greater(t, len(rca.Incident.Graph.Edges), 0, "Graph should have at least one edge")

	// Check that Kustomization node exists
	kustomizationNode := findNodeByKind(rca, "Kustomization")
	require.NotNil(t, kustomizationNode, "Graph should contain Kustomization node")
	t.Logf("✓ Found Kustomization node: %s", kustomizationNode.Resource.Name)

	// Check that MANAGES edge exists from Kustomization to Deployment
	assertGraphHasEdgeBetweenKinds(t, rca, "Kustomization", "MANAGES", "Deployment")
	t.Logf("✓ Found MANAGES edge from Kustomization to Deployment")

	// Also verify the ownership chain
	assertGraphHasEdgeBetweenKinds(t, rca, "Deployment", "OWNS", "ReplicaSet")
	assertGraphHasEdgeBetweenKinds(t, rca, "ReplicaSet", "OWNS", "Pod")
	t.Logf("✓ Found ownership chain: Deployment -> ReplicaSet -> Pod")
}
