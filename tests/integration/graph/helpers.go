package graph

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AssertResourceExists checks that a resource with the given UID exists in the graph
func AssertResourceExists(t *testing.T, client graph.Client, uid string) {
	ctx := context.Background()
	query := graph.FindResourceByUIDQuery(uid)
	result, err := client.ExecuteQuery(ctx, query)
	require.NoError(t, err, "Failed to query for resource %s", uid)
	require.NotEmpty(t, result.Rows, "Resource %s not found in graph", uid)
}

// AssertResourceNotExists checks that a resource with the given UID does not exist in the graph
func AssertResourceNotExists(t *testing.T, client graph.Client, uid string) {
	ctx := context.Background()
	query := graph.FindResourceByUIDQuery(uid)
	result, err := client.ExecuteQuery(ctx, query)
	require.NoError(t, err, "Failed to query for resource %s", uid)
	assert.Empty(t, result.Rows, "Resource %s should not exist in graph", uid)
}

// AssertEventCount checks that a resource has the expected number of ChangeEvents
func AssertEventCount(t *testing.T, client graph.Client, resourceUID string, expectedCount int) {
	ctx := context.Background()

	// Query for change events for this resource
	query := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity {uid: $resourceUID})-[:CHANGED]->(e:ChangeEvent)
			RETURN count(e) as count
		`,
		Parameters: map[string]interface{}{
			"resourceUID": resourceUID,
		},
	}

	result, err := client.ExecuteQuery(ctx, query)
	require.NoError(t, err, "Failed to query event count for resource %s", resourceUID)
	require.NotEmpty(t, result.Rows, "No result returned for event count query")

	count, ok := result.Rows[0][0].(int64)
	require.True(t, ok, "Failed to parse event count")
	assert.Equal(t, int64(expectedCount), count, "Expected %d events for resource %s, got %d", expectedCount, resourceUID, count)
}

// AssertEdgeExists checks that an edge of the specified type exists between two resources
func AssertEdgeExists(t *testing.T, client graph.Client, fromUID, toUID string, edgeType graph.EdgeType) {
	ctx := context.Background()

	// Map EdgeType to Cypher relationship type
	edgeTypeName := string(edgeType)

	query := graph.GraphQuery{
		Query: fmt.Sprintf(`
			MATCH (from:ResourceIdentity {uid: $fromUID})-[r:%s]->(to:ResourceIdentity {uid: $toUID})
			RETURN r
			LIMIT 1
		`, edgeTypeName),
		Parameters: map[string]interface{}{
			"fromUID": fromUID,
			"toUID":   toUID,
		},
	}

	result, err := client.ExecuteQuery(ctx, query)
	require.NoError(t, err, "Failed to query for edge %s -> %s", fromUID, toUID)
	assert.NotEmpty(t, result.Rows, "Edge %s[%s]->%s does not exist", fromUID, edgeTypeName, toUID)
}

// AssertEdgeNotExists checks that an edge of the specified type does not exist between two resources
func AssertEdgeNotExists(t *testing.T, client graph.Client, fromUID, toUID string, edgeType graph.EdgeType) {
	ctx := context.Background()

	edgeTypeName := string(edgeType)

	query := graph.GraphQuery{
		Query: fmt.Sprintf(`
			MATCH (from:ResourceIdentity {uid: $fromUID})-[r:%s]->(to:ResourceIdentity {uid: $toUID})
			RETURN r
			LIMIT 1
		`, edgeTypeName),
		Parameters: map[string]interface{}{
			"fromUID": fromUID,
			"toUID":   toUID,
		},
	}

	result, err := client.ExecuteQuery(ctx, query)
	require.NoError(t, err, "Failed to query for edge %s -> %s", fromUID, toUID)
	assert.Empty(t, result.Rows, "Edge %s[%s]->%s should not exist", fromUID, edgeTypeName, toUID)
}

// CreateTimeSequence creates a sequence of timestamps
func CreateTimeSequence(start time.Time, count int, interval time.Duration) []time.Time {
	times := make([]time.Time, count)
	for i := 0; i < count; i++ {
		times[i] = start.Add(time.Duration(i) * interval)
	}
	return times
}

// CreateResourceMetadata creates a ResourceMetadata for testing
func CreateResourceMetadata(uid, kind, name, namespace, group, version string) models.ResourceMetadata {
	if version == "" {
		version = "v1"
	}
	if group == "" && kind != "Pod" && kind != "Service" && kind != "ConfigMap" {
		// Default to apps for common resources
		if kind == "Deployment" || kind == "ReplicaSet" {
			group = "apps"
		}
	}

	return models.ResourceMetadata{
		UID:       uid,
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
		Group:     group,
		Version:   version,
	}
}

// CreateBasicPodMetadata creates ResourceMetadata for a Pod
func CreateBasicPodMetadata(uid, name, namespace string) models.ResourceMetadata {
	return CreateResourceMetadata(uid, "Pod", name, namespace, "", "v1")
}

// CreateBasicDeploymentMetadata creates ResourceMetadata for a Deployment
func CreateBasicDeploymentMetadata(uid, name, namespace string) models.ResourceMetadata {
	return CreateResourceMetadata(uid, "Deployment", name, namespace, "apps", "v1")
}

// CreateBasicServiceMetadata creates ResourceMetadata for a Service
func CreateBasicServiceMetadata(uid, name, namespace string) models.ResourceMetadata {
	return CreateResourceMetadata(uid, "Service", name, namespace, "", "v1")
}

// CountResources counts the total number of ResourceIdentity nodes in the graph
func CountResources(t *testing.T, client graph.Client) int {
	ctx := context.Background()
	query := graph.GraphQuery{
		Query: `MATCH (r:ResourceIdentity) RETURN count(r) as count`,
	}

	result, err := client.ExecuteQuery(ctx, query)
	require.NoError(t, err)
	require.NotEmpty(t, result.Rows)

	count, ok := result.Rows[0][0].(int64)
	require.True(t, ok)
	return int(count)
}

// CountEdges counts the total number of edges of a specific type in the graph
func CountEdges(t *testing.T, client graph.Client, edgeType graph.EdgeType) int {
	ctx := context.Background()
	edgeTypeName := string(edgeType)

	query := graph.GraphQuery{
		Query: fmt.Sprintf(`MATCH ()-[r:%s]->() RETURN count(r) as count`, edgeTypeName),
	}

	result, err := client.ExecuteQuery(ctx, query)
	require.NoError(t, err)
	require.NotEmpty(t, result.Rows)

	count, ok := result.Rows[0][0].(int64)
	require.True(t, ok)
	return int(count)
}
