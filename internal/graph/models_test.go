package graph

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceIdentity(t *testing.T) {
	resource := ResourceIdentity{
		UID:       "pod-123",
		Kind:      "Pod",
		APIGroup:  "",
		Version:   "v1",
		Namespace: "default",
		Name:      "frontend-abc",
		FirstSeen: 1703001000000000000,
		LastSeen:  1703002000000000000,
		Deleted:   false,
		DeletedAt: 0,
	}

	// Test JSON marshaling
	data, err := json.Marshal(resource)
	require.NoError(t, err)
	assert.Contains(t, string(data), "pod-123")
	assert.Contains(t, string(data), "Pod")

	// Test JSON unmarshaling
	var decoded ResourceIdentity
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, resource.UID, decoded.UID)
	assert.Equal(t, resource.Kind, decoded.Kind)
	assert.Equal(t, resource.Namespace, decoded.Namespace)
}

func TestChangeEvent(t *testing.T) {
	event := ChangeEvent{
		ID:              "event-456",
		Timestamp:       1703001000000000000,
		EventType:       "UPDATE",
		Status:          "Error",
		ErrorMessage:    "CrashLoopBackOff",
		ContainerIssues: []string{"CrashLoopBackOff", "ImagePullBackOff"},
		ConfigChanged:   false,
		StatusChanged:   true,
		ReplicasChanged: false,
		ImpactScore:     0.85,
	}

	// Test JSON marshaling
	data, err := json.Marshal(event)
	require.NoError(t, err)
	assert.Contains(t, string(data), "event-456")
	assert.Contains(t, string(data), "CrashLoopBackOff")

	// Test JSON unmarshaling
	var decoded ChangeEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, event.ID, decoded.ID)
	assert.Equal(t, event.EventType, decoded.EventType)
	assert.Equal(t, event.Status, decoded.Status)
	assert.Equal(t, event.ImpactScore, decoded.ImpactScore)
	assert.Len(t, decoded.ContainerIssues, 2)
}

func TestK8sEvent(t *testing.T) {
	event := K8sEvent{
		ID:        "k8s-event-789",
		Timestamp: 1703001000000000000,
		Reason:    "FailedScheduling",
		Message:   "0/3 nodes are available: insufficient memory",
		Type:      "Warning",
		Count:     5,
		Source:    "scheduler",
	}

	// Test JSON marshaling
	data, err := json.Marshal(event)
	require.NoError(t, err)
	assert.Contains(t, string(data), "FailedScheduling")
	assert.Contains(t, string(data), "scheduler")

	// Test JSON unmarshaling
	var decoded K8sEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, event.ID, decoded.ID)
	assert.Equal(t, event.Reason, decoded.Reason)
	assert.Equal(t, event.Count, decoded.Count)
}

func TestEdgeProperties(t *testing.T) {
	t.Run("OwnsEdge", func(t *testing.T) {
		edge := OwnsEdge{
			Controller:         true,
			BlockOwnerDeletion: false,
		}

		data, err := json.Marshal(edge)
		require.NoError(t, err)

		var decoded OwnsEdge
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, edge.Controller, decoded.Controller)
		assert.Equal(t, edge.BlockOwnerDeletion, decoded.BlockOwnerDeletion)
	})

	t.Run("TriggeredByEdge", func(t *testing.T) {
		edge := TriggeredByEdge{
			Confidence: 0.9,
			LagMs:      34000,
			Reason:     "Deployment update triggered rollout",
		}

		data, err := json.Marshal(edge)
		require.NoError(t, err)

		var decoded TriggeredByEdge
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, edge.Confidence, decoded.Confidence)
		assert.Equal(t, edge.LagMs, decoded.LagMs)
		assert.Equal(t, edge.Reason, decoded.Reason)
	})

	t.Run("SelectsEdge", func(t *testing.T) {
		edge := SelectsEdge{
			SelectorLabels: map[string]string{
				"app":  "frontend",
				"tier": "web",
			},
		}

		data, err := json.Marshal(edge)
		require.NoError(t, err)

		var decoded SelectsEdge
		err = json.Unmarshal(data, &decoded)
		require.NoError(t, err)
		assert.Equal(t, edge.SelectorLabels["app"], decoded.SelectorLabels["app"])
		assert.Equal(t, edge.SelectorLabels["tier"], decoded.SelectorLabels["tier"])
	})
}

func TestGraphQuery(t *testing.T) {
	query := GraphQuery{
		Query: "MATCH (n:Pod {uid: $uid}) RETURN n",
		Parameters: map[string]interface{}{
			"uid": "pod-123",
		},
	}

	data, err := json.Marshal(query)
	require.NoError(t, err)
	assert.Contains(t, string(data), "MATCH")
	assert.Contains(t, string(data), "pod-123")

	var decoded GraphQuery
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, query.Query, decoded.Query)
	assert.Equal(t, query.Parameters["uid"], decoded.Parameters["uid"])
}

func TestNodeAndEdgeTypes(t *testing.T) {
	// Test NodeType constants
	assert.Equal(t, NodeType("ResourceIdentity"), NodeTypeResourceIdentity)
	assert.Equal(t, NodeType("ChangeEvent"), NodeTypeChangeEvent)
	assert.Equal(t, NodeType("K8sEvent"), NodeTypeK8sEvent)

	// Test EdgeType constants
	assert.Equal(t, EdgeType("OWNS"), EdgeTypeOwns)
	assert.Equal(t, EdgeType("CHANGED"), EdgeTypeChanged)
	assert.Equal(t, EdgeType("SELECTS"), EdgeTypeSelects)
	assert.Equal(t, EdgeType("SCHEDULED_ON"), EdgeTypeScheduledOn)
	assert.Equal(t, EdgeType("MOUNTS"), EdgeTypeMounts)
	assert.Equal(t, EdgeType("USES_SERVICE_ACCOUNT"), EdgeTypeUsesServiceAccount)
	assert.Equal(t, EdgeType("EMITTED_EVENT"), EdgeTypeEmittedEvent)
}
