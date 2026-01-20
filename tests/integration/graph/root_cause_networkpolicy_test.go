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

// TestRootCauseEndpoint_NetworkPolicy_SameNamespace tests root cause analysis
// for a NetworkPolicy selecting pods in the same namespace.
func TestRootCauseEndpoint_NetworkPolicy_SameNamespace(t *testing.T) {
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
	assertGraphHasNetworkPolicy(t, rca)
	assertGraphHasNetworkPolicySelectsPod(t, rca)
}

// TestRootCauseEndpoint_NetworkPolicy_CrossNamespace tests root cause analysis
// for a NetworkPolicy that cannot select pods across namespaces.
func TestRootCauseEndpoint_NetworkPolicy_CrossNamespace(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	ctx := context.Background()

	// Load the JSONL fixture
	_, testFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(testFile)
	fixturePath := filepath.Join(testDir, "..", "fixtures", "testrootcause-networkpolicy-crossnamespa.jsonl")
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
	// The NetworkPolicy should NOT appear in the graph because it cannot select pods across namespaces
	assertGraphHasDeploymentOwnsPod(t, rca)
	assertGraphHasNoNetworkPolicy(t, rca)
}

// Helper functions for NetworkPolicy assertions

// assertGraphHasNetworkPolicy verifies that NetworkPolicy node exists
func assertGraphHasNetworkPolicy(t *testing.T, rca *analysis.RootCauseAnalysisV2) {
	require.NotNil(t, rca, "Root cause analysis should not be nil")
	require.NotNil(t, rca.Incident, "Incident should not be nil")
	require.NotNil(t, rca.Incident.Graph, "Graph should not be nil")
	require.Greater(t, len(rca.Incident.Graph.Nodes), 0, "Graph should have at least one node")
	require.Greater(t, len(rca.Incident.Graph.Edges), 0, "Graph should have at least one edge")

	// Check that NetworkPolicy node exists
	netpolNode := findNodeByKind(rca, "NetworkPolicy")
	require.NotNil(t, netpolNode, "Graph should contain NetworkPolicy node")
	t.Logf("✓ Found NetworkPolicy node: %s", netpolNode.Resource.Name)
}

// assertGraphHasNetworkPolicySelectsPod verifies that NetworkPolicy selects Pod
func assertGraphHasNetworkPolicySelectsPod(t *testing.T, rca *analysis.RootCauseAnalysisV2) {
	assertGraphHasEdgeBetweenKinds(t, rca, "NetworkPolicy", "SELECTS", "Pod")
	t.Logf("✓ Found SELECTS edge from NetworkPolicy to Pod")
}

// assertGraphHasNoNetworkPolicy verifies that NetworkPolicy node does NOT exist
func assertGraphHasNoNetworkPolicy(t *testing.T, rca *analysis.RootCauseAnalysisV2) {
	require.NotNil(t, rca, "Root cause analysis should not be nil")
	require.NotNil(t, rca.Incident, "Incident should not be nil")
	require.NotNil(t, rca.Incident.Graph, "Graph should not be nil")
	require.Greater(t, len(rca.Incident.Graph.Nodes), 0, "Graph should have at least one node")
	require.Greater(t, len(rca.Incident.Graph.Edges), 0, "Graph should have at least one edge")

	// Check that NetworkPolicy node does NOT exist
	netpolNode := findNodeByKind(rca, "NetworkPolicy")
	require.Nil(t, netpolNode, "Graph should NOT contain NetworkPolicy node (cross-namespace selection not supported)")
	t.Logf("✓ Confirmed no NetworkPolicy node in graph (expected for cross-namespace test)")
}
