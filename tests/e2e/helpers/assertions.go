// Package helpers provides assertion utilities for e2e testing.
package helpers

import (
	"context"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	causalpaths "github.com/moolen/spectre/internal/analysis/causal_paths"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// EventuallyOption configures Eventually assertion behavior.
type EventuallyOption struct {
	Timeout  time.Duration
	Interval time.Duration
}

// DefaultEventuallyOption provides sensible defaults for async operations.
var DefaultEventuallyOption = EventuallyOption{
	Timeout:  30 * time.Second,
	Interval: 3 * time.Second,
}

// SlowEventuallyOption for operations that take longer (config reload, etc).
var SlowEventuallyOption = EventuallyOption{
	Timeout:  90 * time.Second,
	Interval: 5 * time.Second,
}

// EventuallyResourceCreated waits for a resource to be created in the API.
func EventuallyResourceCreated(t *testing.T, client *APIClient, namespace, kind, name string, opts EventuallyOption) *Resource {
	if opts.Timeout == 0 {
		opts = DefaultEventuallyOption
	}

	var result *Resource

	assert.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()

		now := time.Now().Unix()
		startTime := now - 90 // Last 90 seconds
		endTime := now + 10   // Slightly into future for clock skew

		resp, err := client.Search(ctx, startTime, endTime, namespace, kind)
		if err != nil {
			t.Logf("Search failed: %v", err)
			return false
		}

		// Find resource by name
		for _, r := range resp.Resources {
			if r.Name == name {
				result = &r
				return true
			}
		}
		return false
	}, opts.Timeout, opts.Interval)

	require.NotNil(t, result, "Resource %s/%s/%s not found in API after %v", namespace, kind, name, opts.Timeout)
	return result
}

// EventuallyEventCreated waits for an event to appear in the API.
func EventuallyEventCreated(t *testing.T, client *APIClient, resourceID, reason string, opts EventuallyOption) *K8sEvent {
	if opts.Timeout == 0 {
		opts = DefaultEventuallyOption
	}

	var result *K8sEvent

	assert.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()

		now := time.Now().Unix()
		startTime := now - 60 // Last 60 seconds
		endTime := now + 10

		resp, err := client.GetEvents(ctx, resourceID, &startTime, &endTime, nil)
		if err != nil {
			t.Logf("GetEvents failed: %v", err)
			return false
		}

		// Find event by reason
		for _, e := range resp.Events {
			if e.Reason == reason {
				result = &e
				return true
			}
		}
		return false
	}, opts.Timeout, opts.Interval)

	require.NotNil(t, result, "Event with reason %s not found after %v", reason, opts.Timeout)
	return result
}

// EventuallyEventCount waits for a specific number of events.
func EventuallyEventCount(t *testing.T, client *APIClient, resourceID string, expectedCount int, opts EventuallyOption) {
	if opts.Timeout == 0 {
		opts = DefaultEventuallyOption
	}

	assert.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()

		now := time.Now().Unix()
		startTime := now - 120
		endTime := now + 10

		resp, err := client.GetEvents(ctx, resourceID, &startTime, &endTime, nil)
		if err != nil {
			t.Logf("GetEvents failed: %v", err)
			return false
		}

		t.Logf("Event count: %d (expected: %d)", len(resp.Events), expectedCount)
		return len(resp.Events) >= expectedCount
	}, opts.Timeout, opts.Interval)
}

// EventuallySegmentsCount waits for a specific number of segments.
func EventuallySegmentsCount(t *testing.T, client *APIClient, resourceID string, expectedCount int, opts EventuallyOption) {
	if opts.Timeout == 0 {
		opts = DefaultEventuallyOption
	}

	assert.Eventually(t, func() bool {
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()

		now := time.Now().Unix()
		startTime := now - 120
		endTime := now + 10

		resp, err := client.GetSegments(ctx, resourceID, &startTime, &endTime)
		if err != nil {
			t.Logf("GetSegments failed: %v", err)
			return false
		}

		t.Logf("Segment count: %d (expected: %d)", len(resp.Segments), expectedCount)
		return len(resp.Segments) >= expectedCount
	}, opts.Timeout, opts.Interval)
}

// EventuallyCondition waits for a custom condition to be true.
func EventuallyCondition(t *testing.T, condition func() bool, opts EventuallyOption) {
	if opts.Timeout == 0 {
		opts = DefaultEventuallyOption
	}

	assert.Eventually(t, condition, opts.Timeout, opts.Interval)
}

// AssertEventExists verifies an event exists with expected properties.
func AssertEventExists(t *testing.T, event *K8sEvent, expectedReason string) {
	require.NotNil(t, event)
	assert.Equal(t, expectedReason, event.Reason, "Event reason mismatch")
	assert.NotZero(t, event.Timestamp, "Event timestamp should not be zero")
}

// AssertNamespaceInMetadata verifies a namespace appears in metadata.
func AssertNamespaceInMetadata(t *testing.T, metadata *MetadataResponse, namespace string) {
	require.NotNil(t, metadata)
	assert.Contains(t, metadata.Namespaces, namespace, "Namespace %s not found in metadata", namespace)
}

// AssertKindInMetadata verifies a resource kind appears in metadata.
func AssertKindInMetadata(t *testing.T, metadata *MetadataResponse, kind string) {
	require.NotNil(t, metadata)
	assert.Contains(t, metadata.Kinds, kind, "Kind %s not found in metadata", kind)
}

// ==================== Root Cause Analysis Assertions ====================

// RequireGraphHasKinds verifies that the root cause graph contains nodes of all specified kinds.
func RequireGraphHasKinds(t *testing.T, rca *analysis.RootCauseAnalysisV2, expectedKinds []string) {
	require.NotNil(t, rca, "Root cause analysis should not be nil")
	require.NotNil(t, rca.Incident, "Incident should not be nil")
	require.NotNil(t, rca.Incident.Graph, "Graph should not be nil")

	kindSet := make(map[string]bool)
	for _, node := range rca.Incident.Graph.Nodes {
		kindSet[node.Resource.Kind] = true
	}

	for _, expectedKind := range expectedKinds {
		require.True(t, kindSet[expectedKind], "Graph should contain node of kind %s. Found kinds: %v", expectedKind, getKeys(kindSet))
	}
}

// FindNodeByKind finds a node in the graph by its resource kind.
// Returns nil if not found.
func FindNodeByKind(rca *analysis.RootCauseAnalysisV2, kind string) *analysis.GraphNode {
	if rca == nil || rca.Incident.Graph.Nodes == nil {
		return nil
	}

	for i := range rca.Incident.Graph.Nodes {
		if rca.Incident.Graph.Nodes[i].Resource.Kind == kind {
			return &rca.Incident.Graph.Nodes[i]
		}
	}
	return nil
}

// FindAllNodesByKind finds all nodes in the graph matching the given resource kind.
// Returns an empty slice if none found.
func FindAllNodesByKind(rca *analysis.RootCauseAnalysisV2, kind string) []*analysis.GraphNode {
	var nodes []*analysis.GraphNode
	if rca == nil || rca.Incident.Graph.Nodes == nil {
		return nodes
	}

	for i := range rca.Incident.Graph.Nodes {
		if rca.Incident.Graph.Nodes[i].Resource.Kind == kind {
			nodes = append(nodes, &rca.Incident.Graph.Nodes[i])
		}
	}
	return nodes
}

// RequireGraphHasEdgeBetweenKinds verifies that the graph contains an edge of the specified
// relationship type between nodes of the given kinds.
func RequireGraphHasEdgeBetweenKinds(t *testing.T, rca *analysis.RootCauseAnalysisV2, fromKind, relType, toKind string) {
	require.NotNil(t, rca, "Root cause analysis should not be nil")
	require.NotNil(t, rca.Incident, "Incident should not be nil")
	require.NotNil(t, rca.Incident.Graph, "Graph should not be nil")

	fromNodes := FindAllNodesByKind(rca, fromKind)
	require.NotEmpty(t, fromNodes, "Graph should contain node of kind %s", fromKind)

	toNodes := FindAllNodesByKind(rca, toKind)
	require.NotEmpty(t, toNodes, "Graph should contain node of kind %s", toKind)

	// Build sets of node IDs for efficient lookup
	fromIDs := make(map[string]bool)
	for _, n := range fromNodes {
		fromIDs[n.ID] = true
	}
	toIDs := make(map[string]bool)
	for _, n := range toNodes {
		toIDs[n.ID] = true
	}

	// Check if any edge exists between any fromKind node and any toKind node
	found := false
	for _, edge := range rca.Incident.Graph.Edges {
		if fromIDs[edge.From] && toIDs[edge.To] && edge.RelationshipType == relType {
			found = true
			break
		}
	}

	require.True(t, found, "Graph should contain edge %s -[%s]-> %s", fromKind, relType, toKind)
}

// RequireNodeHasEventTypes verifies that a node has events of the specified types.
func RequireNodeHasEventTypes(t *testing.T, node *analysis.GraphNode, expectedTypes []string) {
	require.NotNil(t, node, "Node should not be nil")

	typeSet := make(map[string]bool)
	for _, event := range node.AllEvents {
		typeSet[event.EventType] = true
	}

	for _, expectedType := range expectedTypes {
		require.True(t, typeSet[expectedType], "Node should have event of type %s. Found types: %v", expectedType, getKeys(typeSet))
	}
}

// RequireUpdateConfigChanged verifies that a node has at least one UPDATE event with configChanged=true.
func RequireUpdateConfigChanged(t *testing.T, node *analysis.GraphNode) {
	require.NotNil(t, node, "Node should not be nil")

	found := false
	for _, event := range node.AllEvents {
		if event.EventType == "UPDATE" && event.ConfigChanged {
			found = true
			break
		}
	}

	require.True(t, found, "Node should have at least one UPDATE event with configChanged=true")
}

// RequireGraphNonEmpty verifies that the graph has nodes and edges.
func RequireGraphNonEmpty(t *testing.T, rca *analysis.RootCauseAnalysisV2) {
	require.NotNil(t, rca, "Root cause analysis should not be nil")
	require.NotNil(t, rca.Incident, "Incident should not be nil")
	require.NotNil(t, rca.Incident.Graph, "Graph should not be nil")
	require.Greater(t, len(rca.Incident.Graph.Nodes), 0, "Graph should have at least one node")
	require.Greater(t, len(rca.Incident.Graph.Edges), 0, "Graph should have at least one edge")
}

// Helper function to get keys from a map
func getKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ==================== Causal Paths Assertions ====================

// RequireCausalPathsNonEmpty verifies that the causal paths response has at least one path.
func RequireCausalPathsNonEmpty(t *testing.T, resp *causalpaths.CausalPathsResponse) {
	require.NotNil(t, resp, "Causal paths response should not be nil")
	require.NotEmpty(t, resp.Paths, "Causal paths should not be empty")
	require.Greater(t, resp.Metadata.NodesExplored, 0, "Should have explored at least one node")
}

// RequireCausalPathsHasKinds verifies that the causal paths contain nodes of all specified kinds.
func RequireCausalPathsHasKinds(t *testing.T, resp *causalpaths.CausalPathsResponse, expectedKinds []string) {
	require.NotNil(t, resp, "Causal paths response should not be nil")

	kindSet := make(map[string]bool)
	for _, path := range resp.Paths {
		for _, step := range path.Steps {
			kindSet[step.Node.Resource.Kind] = true
		}
	}

	for _, expectedKind := range expectedKinds {
		require.True(t, kindSet[expectedKind], "Paths should contain node of kind %s. Found kinds: %v", expectedKind, getKeys(kindSet))
	}
}

// FindCausalPathNodeByKind finds a node in the causal paths by its resource kind.
// Returns nil if not found.
func FindCausalPathNodeByKind(resp *causalpaths.CausalPathsResponse, kind string) *causalpaths.PathNode {
	if resp == nil {
		return nil
	}

	for _, path := range resp.Paths {
		for i := range path.Steps {
			if path.Steps[i].Node.Resource.Kind == kind {
				return &path.Steps[i].Node
			}
		}
	}
	return nil
}

// RequireCausalPathsHasEdgeBetweenKinds verifies that paths contain an edge of the specified
// relationship type between nodes of the given kinds.
func RequireCausalPathsHasEdgeBetweenKinds(t *testing.T, resp *causalpaths.CausalPathsResponse, fromKind, relType, toKind string) {
	require.NotNil(t, resp, "Causal paths response should not be nil")

	found := false
	for _, path := range resp.Paths {
		for i := 1; i < len(path.Steps); i++ {
			prevStep := path.Steps[i-1]
			step := path.Steps[i]

			if prevStep.Node.Resource.Kind == fromKind &&
				step.Node.Resource.Kind == toKind &&
				step.Edge != nil &&
				step.Edge.RelationshipType == relType {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	require.True(t, found, "Paths should contain edge %s -[%s]-> %s", fromKind, relType, toKind)
}

// GetCausalPathRootCause returns the root cause (first node in first path) from causal paths.
func GetCausalPathRootCause(resp *causalpaths.CausalPathsResponse) *causalpaths.PathNode {
	if resp == nil || len(resp.Paths) == 0 {
		return nil
	}
	return &resp.Paths[0].CandidateRoot
}

// RequireCausalPathRootCauseKind verifies that the root cause is of the expected kind.
func RequireCausalPathRootCauseKind(t *testing.T, resp *causalpaths.CausalPathsResponse, expectedKind string) {
	require.NotNil(t, resp, "Causal paths response should not be nil")
	require.NotEmpty(t, resp.Paths, "Should have at least one path")

	rootCause := &resp.Paths[0].CandidateRoot
	require.Equal(t, expectedKind, rootCause.Resource.Kind,
		"Root cause should be %s, got %s", expectedKind, rootCause.Resource.Kind)
}

// RequireCausalPathConfidenceAbove verifies the top path's confidence score is above the threshold.
func RequireCausalPathConfidenceAbove(t *testing.T, resp *causalpaths.CausalPathsResponse, threshold float64) {
	require.NotNil(t, resp, "Causal paths response should not be nil")
	require.NotEmpty(t, resp.Paths, "Should have at least one path")

	confidence := resp.Paths[0].ConfidenceScore
	require.GreaterOrEqual(t, confidence, threshold,
		"Confidence score should be >= %.2f (got %.2f)", threshold, confidence)
}
