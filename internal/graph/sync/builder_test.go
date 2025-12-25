package sync

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGraphBuilder_BuildFromEvent(t *testing.T) {
	builder := NewGraphBuilder()
	ctx := context.Background()

	t.Run("Pod CREATE event", func(t *testing.T) {
		event := models.Event{
			ID:        "event-1",
			Timestamp: time.Now().UnixNano(),
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				UID:       "pod-123",
				Kind:      "Pod",
				Group:     "",
				Version:   "v1",
				Namespace: "default",
				Name:      "test-pod",
			},
			Data: createPodJSON("test-pod", "Running"),
		}

		update, err := builder.BuildFromEvent(ctx, event)
		require.NoError(t, err)
		require.NotNil(t, update)

		// Should have ResourceIdentity node
		assert.Len(t, update.ResourceNodes, 1)
		assert.Equal(t, "pod-123", update.ResourceNodes[0].UID)
		assert.Equal(t, "Pod", update.ResourceNodes[0].Kind)

		// Should have ChangeEvent node
		assert.Len(t, update.EventNodes, 1)
		assert.Equal(t, "event-1", update.EventNodes[0].ID)
		assert.Equal(t, "CREATE", update.EventNodes[0].EventType)

		// Should have CHANGED edge
		assert.Len(t, update.Edges, 1)
		assert.Equal(t, graph.EdgeTypeChanged, update.Edges[0].Type)
	})

	t.Run("Deployment UPDATE event", func(t *testing.T) {
		event := models.Event{
			ID:        "event-2",
			Timestamp: time.Now().UnixNano(),
			Type:      models.EventTypeUpdate,
			Resource: models.ResourceMetadata{
				UID:       "deploy-456",
				Kind:      "Deployment",
				Group:     "apps",
				Version:   "v1",
				Namespace: "default",
				Name:      "frontend",
			},
			Data: createDeploymentJSON("frontend", 3, 3),
		}

		update, err := builder.BuildFromEvent(ctx, event)
		require.NoError(t, err)

		assert.Len(t, update.ResourceNodes, 1)
		assert.Equal(t, "deploy-456", update.ResourceNodes[0].UID)
		assert.Equal(t, "Deployment", update.ResourceNodes[0].Kind)
		assert.Equal(t, "apps", update.ResourceNodes[0].APIGroup)
	})

	t.Run("K8s Event object", func(t *testing.T) {
		event := models.Event{
			ID:        "k8s-event-789",
			Timestamp: time.Now().UnixNano(),
			Type:      models.EventTypeCreate,
			Resource: models.ResourceMetadata{
				UID:               "event-obj-123",
				Kind:              "Event",
				Version:           "v1",
				Namespace:         "default",
				InvolvedObjectUID: "pod-123",
			},
			Data: createK8sEventJSON("FailedScheduling", "No nodes available"),
		}

		update, err := builder.BuildFromEvent(ctx, event)
		require.NoError(t, err)

		// Should have K8sEvent node, not ChangeEvent
		assert.Len(t, update.K8sEventNodes, 1)
		assert.Len(t, update.EventNodes, 0)
		assert.Equal(t, "k8s-event-789", update.K8sEventNodes[0].ID)
		assert.Equal(t, "FailedScheduling", update.K8sEventNodes[0].Reason)

		// Should have EMITTED_EVENT edge
		assert.Len(t, update.Edges, 1)
		assert.Equal(t, graph.EdgeTypeEmittedEvent, update.Edges[0].Type)
		assert.Equal(t, "pod-123", update.Edges[0].FromUID)
	})

	t.Run("DELETE event", func(t *testing.T) {
		event := models.Event{
			ID:        "event-3",
			Timestamp: time.Now().UnixNano(),
			Type:      models.EventTypeDelete,
			Resource: models.ResourceMetadata{
				UID:       "pod-deleted",
				Kind:      "Pod",
				Namespace: "default",
				Name:      "deleted-pod",
			},
		}

		update, err := builder.BuildFromEvent(ctx, event)
		require.NoError(t, err)

		// Resource should be marked as deleted
		assert.True(t, update.ResourceNodes[0].Deleted)
		assert.NotZero(t, update.ResourceNodes[0].DeletedAt)

		// ChangeEvent status should be Terminating
		assert.Equal(t, "Terminating", update.EventNodes[0].Status)
	})
}

func TestGraphBuilder_ExtractOwnership(t *testing.T) {
	builder := NewGraphBuilder()
	ctx := context.Background()

	podData := map[string]interface{}{
		"metadata": map[string]interface{}{
			"ownerReferences": []interface{}{
				map[string]interface{}{
					"uid":                "rs-123",
					"kind":               "ReplicaSet",
					"name":               "frontend-rs",
					"controller":         true,
					"blockOwnerDeletion": true,
				},
			},
		},
	}

	podJSON, _ := json.Marshal(podData)

	event := models.Event{
		ID:       "event-1",
		Resource: models.ResourceMetadata{UID: "pod-123"},
		Data:     podJSON,
	}

	edges, err := builder.ExtractRelationships(ctx, event)
	require.NoError(t, err)

	// Should extract OWNS edge
	assert.Len(t, edges, 1)
	assert.Equal(t, graph.EdgeTypeOwns, edges[0].Type)
	assert.Equal(t, "rs-123", edges[0].FromUID)
	assert.Equal(t, "pod-123", edges[0].ToUID)

	// Check properties
	var props graph.OwnsEdge
	err = json.Unmarshal(edges[0].Properties, &props)
	require.NoError(t, err)
	assert.True(t, props.Controller)
	assert.True(t, props.BlockOwnerDeletion)
}

func TestGraphBuilder_CalculateImpactScore(t *testing.T) {
	builder := NewGraphBuilder().(*graphBuilder)

	tests := []struct {
		name            string
		status          string
		containerIssues []string
		expectedMin     float64
		expectedMax     float64
	}{
		{
			name:            "Error status",
			status:          "Error",
			containerIssues: []string{},
			expectedMin:     0.8,
			expectedMax:     0.8,
		},
		{
			name:            "Error with container issues",
			status:          "Error",
			containerIssues: []string{"CrashLoopBackOff"},
			expectedMin:     1.0,
			expectedMax:     1.0,
		},
		{
			name:            "Warning status",
			status:          "Warning",
			containerIssues: []string{},
			expectedMin:     0.5,
			expectedMax:     0.5,
		},
		{
			name:            "Ready status",
			status:          "Ready",
			containerIssues: []string{},
			expectedMin:     0.1,
			expectedMax:     0.1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := builder.calculateImpactScore(tt.status, tt.containerIssues)
			assert.GreaterOrEqual(t, score, tt.expectedMin)
			assert.LessOrEqual(t, score, tt.expectedMax)
		})
	}
}

// Helper functions

func createPodJSON(name, phase string) json.RawMessage {
	data := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": name,
		},
		"status": map[string]interface{}{
			"phase": phase,
		},
	}
	jsonData, _ := json.Marshal(data)
	return jsonData
}

func createDeploymentJSON(name string, replicas, readyReplicas int) json.RawMessage {
	data := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": name,
		},
		"spec": map[string]interface{}{
			"replicas": replicas,
		},
		"status": map[string]interface{}{
			"replicas":      replicas,
			"readyReplicas": readyReplicas,
		},
	}
	jsonData, _ := json.Marshal(data)
	return jsonData
}

func createK8sEventJSON(reason, message string) json.RawMessage {
	data := map[string]interface{}{
		"reason":  reason,
		"message": message,
		"type":    "Warning",
		"count":   1,
		"source": map[string]interface{}{
			"component": "scheduler",
		},
	}
	jsonData, _ := json.Marshal(data)
	return jsonData
}
