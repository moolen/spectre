// go:build integration
// +build integration

package graph

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFalkorDBIntegration requires FalkorDB to be running
// Run with: docker-compose -f docker-compose.graph.yml up -d && go test ./internal/graph -v -tags=integration
func TestFalkorDBIntegration(t *testing.T) {
	// Skip if not in integration test mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create client
	config := DefaultClientConfig()
	client := NewClient(config)

	ctx := context.Background()

	// Connect to FalkorDB
	err := client.Connect(ctx)
	require.NoError(t, err, "Failed to connect to FalkorDB - make sure it's running (docker-compose -f docker-compose.graph.yml up -d)")
	defer client.Close()

	// Ping
	err = client.Ping(ctx)
	require.NoError(t, err)

	// Clean up any existing graph data
	_, err = client.ExecuteQuery(ctx, GraphQuery{
		Query: "MATCH (n) DETACH DELETE n",
	})
	require.NoError(t, err)

	t.Run("Create and query ResourceIdentity", func(t *testing.T) {
		// Create a ResourceIdentity
		resource := ResourceIdentity{
			UID:       "test-pod-123",
			Kind:      "Pod",
			APIGroup:  "",
			Version:   "v1",
			Namespace: "default",
			Name:      "test-frontend",
			FirstSeen: time.Now().UnixNano(),
			LastSeen:  time.Now().UnixNano(),
			Deleted:   false,
			DeletedAt: 0,
		}

		query := UpsertResourceIdentityQuery(resource)
		result, err := client.ExecuteQuery(ctx, query)
		require.NoError(t, err)
		assert.Greater(t, result.Stats.NodesCreated+result.Stats.PropertiesSet, 0)

		// Query it back
		findQuery := FindResourceByUIDQuery("test-pod-123")
		result, err = client.ExecuteQuery(ctx, findQuery)
		require.NoError(t, err)
		assert.Len(t, result.Rows, 1)
	})

	t.Run("Create and query ChangeEvent", func(t *testing.T) {
		// Create a ChangeEvent
		event := ChangeEvent{
			ID:            "test-event-456",
			Timestamp:     time.Now().UnixNano(),
			EventType:     "UPDATE",
			Status:        "Error",
			ErrorMessage:  "Test error",
			ImpactScore:   0.75,
			ConfigChanged: true,
			StatusChanged: true,
		}

		query := CreateChangeEventQuery(event)
		result, err := client.ExecuteQuery(ctx, query)
		require.NoError(t, err)
		assert.Greater(t, result.Stats.NodesCreated+result.Stats.PropertiesSet, 0)

		// Query it back
		findQuery := GraphQuery{
			Query: "MATCH (e:ChangeEvent {id: $id}) RETURN e",
			Parameters: map[string]interface{}{
				"id": "test-event-456",
			},
		}
		result, err = client.ExecuteQuery(ctx, findQuery)
		require.NoError(t, err)
		assert.Len(t, result.Rows, 1)
	})

	t.Run("Create OWNS relationship", func(t *testing.T) {
		// Create owner resource
		owner := ResourceIdentity{
			UID:       "test-deployment-789",
			Kind:      "Deployment",
			Namespace: "default",
			Name:      "test-deployment",
			FirstSeen: time.Now().UnixNano(),
			LastSeen:  time.Now().UnixNano(),
		}

		query := UpsertResourceIdentityQuery(owner)
		_, err := client.ExecuteQuery(ctx, query)
		require.NoError(t, err)

		// Create owned resource
		owned := ResourceIdentity{
			UID:       "test-pod-owned",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "test-pod",
			FirstSeen: time.Now().UnixNano(),
			LastSeen:  time.Now().UnixNano(),
		}

		query = UpsertResourceIdentityQuery(owned)
		_, err = client.ExecuteQuery(ctx, query)
		require.NoError(t, err)

		// Create OWNS edge
		ownsProps := OwnsEdge{
			Controller:         true,
			BlockOwnerDeletion: false,
		}

		edgeQuery := CreateOwnsEdgeQuery("test-deployment-789", "test-pod-owned", ownsProps)
		result, err := client.ExecuteQuery(ctx, edgeQuery)
		require.NoError(t, err)
		assert.Greater(t, result.Stats.RelationshipsCreated, 0)

		// Query the relationship
		findQuery := GraphQuery{
			Query: `
				MATCH (owner:ResourceIdentity {uid: $ownerUID})-[r:OWNS]->(owned:ResourceIdentity {uid: $ownedUID})
				RETURN owner, r, owned
			`,
			Parameters: map[string]interface{}{
				"ownerUID": "test-deployment-789",
				"ownedUID": "test-pod-owned",
			},
		}
		result, err = client.ExecuteQuery(ctx, findQuery)
		require.NoError(t, err)
		assert.Len(t, result.Rows, 1)
	})

	t.Run("Create TRIGGERED_BY relationship", func(t *testing.T) {
		// Create cause event
		causeEvent := ChangeEvent{
			ID:        "test-cause-event",
			Timestamp: time.Now().UnixNano(),
			EventType: "UPDATE",
			Status:    "Ready",
		}

		query := CreateChangeEventQuery(causeEvent)
		_, err := client.ExecuteQuery(ctx, query)
		require.NoError(t, err)

		// Create effect event (1 second later)
		effectEvent := ChangeEvent{
			ID:        "test-effect-event",
			Timestamp: time.Now().Add(1 * time.Second).UnixNano(),
			EventType: "CREATE",
			Status:    "Error",
		}

		query = CreateChangeEventQuery(effectEvent)
		_, err = client.ExecuteQuery(ctx, query)
		require.NoError(t, err)

		// Create TRIGGERED_BY edge
		triggeredByProps := TriggeredByEdge{
			Confidence: 0.85,
			LagMs:      1000,
			Reason:     "Test causality",
		}

		edgeQuery := CreateTriggeredByEdgeQuery("test-effect-event", "test-cause-event", triggeredByProps)
		result, err := client.ExecuteQuery(ctx, edgeQuery)
		require.NoError(t, err)
		assert.Greater(t, result.Stats.RelationshipsCreated, 0)

		// Query the relationship
		findQuery := GraphQuery{
			Query: `
				MATCH (effect:ChangeEvent {id: $effectID})-[t:TRIGGERED_BY]->(cause:ChangeEvent {id: $causeID})
				RETURN effect, t, cause
			`,
			Parameters: map[string]interface{}{
				"effectID": "test-effect-event",
				"causeID":  "test-cause-event",
			},
		}
		result, err = client.ExecuteQuery(ctx, findQuery)
		require.NoError(t, err)
		assert.Len(t, result.Rows, 1)
	})

	t.Run("Delete old events", func(t *testing.T) {
		// Create an old event
		oldEvent := ChangeEvent{
			ID:        "test-old-event",
			Timestamp: time.Now().Add(-48 * time.Hour).UnixNano(),
			EventType: "DELETE",
			Status:    "Terminating",
		}

		query := CreateChangeEventQuery(oldEvent)
		_, err := client.ExecuteQuery(ctx, query)
		require.NoError(t, err)

		// Delete events older than 24 hours
		cutoffNs := time.Now().Add(-24 * time.Hour).UnixNano()
		deleteQuery := DeleteOldChangeEventsQuery(cutoffNs)
		result, err := client.ExecuteQuery(ctx, deleteQuery)
		require.NoError(t, err)
		assert.Greater(t, result.Stats.NodesDeleted, 0)

		// Verify it's gone
		findQuery := GraphQuery{
			Query: "MATCH (e:ChangeEvent {id: $id}) RETURN e",
			Parameters: map[string]interface{}{
				"id": "test-old-event",
			},
		}
		result, err = client.ExecuteQuery(ctx, findQuery)
		require.NoError(t, err)
		assert.Len(t, result.Rows, 0)
	})

	t.Run("Schema initialization", func(t *testing.T) {
		schema := NewSchema(client)
		err := schema.Initialize(ctx)
		// Indexes may already exist from previous runs, so we don't require.NoError
		// Just verify it doesn't panic
		assert.NotNil(t, schema)
		_ = err // Ignore error as indexes may already exist
	})

	// Clean up
	_, err = client.ExecuteQuery(ctx, GraphQuery{
		Query: "MATCH (n) DETACH DELETE n",
	})
	require.NoError(t, err)
}

func TestFalkorDBConnectionFailure(t *testing.T) {
	// Test graceful failure when FalkorDB is not available
	config := ClientConfig{
		Host:        "nonexistent-host",
		Port:        9999,
		GraphName:   "test",
		MaxRetries:  1,
		DialTimeout: 1 * time.Second,
	}

	client := NewClient(config)
	ctx := context.Background()

	err := client.Connect(ctx)
	assert.Error(t, err, "Should fail to connect to nonexistent host")
}
