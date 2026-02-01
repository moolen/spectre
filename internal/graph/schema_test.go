package graph

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpsertResourceIdentityQuery(t *testing.T) {
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

	query := UpsertResourceIdentityQuery(resource)

	assert.Contains(t, query.Query, "MERGE")
	assert.Contains(t, query.Query, "ResourceIdentity")
	assert.Contains(t, query.Query, "ON CREATE SET")
	assert.Contains(t, query.Query, "ON MATCH SET")

	assert.Equal(t, "pod-123", query.Parameters["uid"])
	assert.Equal(t, "Pod", query.Parameters["kind"])
	assert.Equal(t, "default", query.Parameters["namespace"])
	assert.Equal(t, "frontend-abc", query.Parameters["name"])
	assert.Equal(t, int64(1703001000000000000), query.Parameters["firstSeen"])
}

func TestCreateChangeEventQuery(t *testing.T) {
	event := ChangeEvent{
		ID:              "event-456",
		Timestamp:       1703001000000000000,
		EventType:       "UPDATE",
		Status:          "Error",
		ErrorMessage:    "CrashLoopBackOff",
		ContainerIssues: []string{"CrashLoopBackOff"},
		ConfigChanged:   true,
		StatusChanged:   true,
		ReplicasChanged: false,
		ImpactScore:     0.85,
	}

	query := CreateChangeEventQuery(event)

	assert.Contains(t, query.Query, "MERGE")
	assert.Contains(t, query.Query, "ChangeEvent")
	assert.Contains(t, query.Query, "ON CREATE SET")

	assert.Equal(t, "event-456", query.Parameters["id"])
	assert.Equal(t, int64(1703001000000000000), query.Parameters["timestamp"])
	assert.Equal(t, "UPDATE", query.Parameters["eventType"])
	assert.Equal(t, "Error", query.Parameters["status"])
	assert.Equal(t, 0.85, query.Parameters["impactScore"])
}

func TestCreateK8sEventQuery(t *testing.T) {
	event := K8sEvent{
		ID:        "k8s-event-789",
		Timestamp: 1703001000000000000,
		Reason:    "FailedScheduling",
		Message:   "No nodes available",
		Type:      "Warning",
		Count:     5,
		Source:    "scheduler",
	}

	query := CreateK8sEventQuery(event)

	assert.Contains(t, query.Query, "MERGE")
	assert.Contains(t, query.Query, "K8sEvent")

	assert.Equal(t, "k8s-event-789", query.Parameters["id"])
	assert.Equal(t, "FailedScheduling", query.Parameters["reason"])
	assert.Equal(t, 5, query.Parameters["count"])
}

func TestCreateOwnsEdgeQuery(t *testing.T) {
	props := OwnsEdge{
		Controller:         true,
		BlockOwnerDeletion: false,
	}

	query := CreateOwnsEdgeQuery("owner-uid", "owned-uid", props)

	// Uses MERGE for both nodes (no MATCH) to handle out-of-order event processing
	assert.Contains(t, query.Query, "MERGE (owner:ResourceIdentity")
	assert.Contains(t, query.Query, "MERGE (owned:ResourceIdentity")
	assert.Contains(t, query.Query, "OWNS")
	assert.Contains(t, query.Query, "MERGE (owner)-[r:OWNS]->(owned)")

	assert.Equal(t, "owner-uid", query.Parameters["ownerUID"])
	assert.Equal(t, "owned-uid", query.Parameters["ownedUID"])
	assert.Equal(t, true, query.Parameters["controller"])
	assert.Equal(t, false, query.Parameters["blockOwnerDeletion"])
}

func TestCreateChangedEdgeQuery(t *testing.T) {
	query := CreateChangedEdgeQuery("resource-uid", "event-id", 5)

	assert.Contains(t, query.Query, "MATCH")
	assert.Contains(t, query.Query, "ResourceIdentity")
	assert.Contains(t, query.Query, "ChangeEvent")
	assert.Contains(t, query.Query, "CHANGED")

	assert.Equal(t, "resource-uid", query.Parameters["resourceUID"])
	assert.Equal(t, "event-id", query.Parameters["eventID"])
	assert.Equal(t, 5, query.Parameters["sequenceNumber"])
}

func TestCreatePrecededByEdgeQuery(t *testing.T) {
	query := CreatePrecededByEdgeQuery("current-event", "previous-event", 5000)

	assert.Contains(t, query.Query, "PRECEDED_BY")

	assert.Equal(t, "current-event", query.Parameters["currentEventID"])
	assert.Equal(t, "previous-event", query.Parameters["previousEventID"])
	assert.Equal(t, int64(5000), query.Parameters["durationMs"])
}

func TestCreateTriggeredByEdgeQuery(t *testing.T) {
	props := TriggeredByEdge{
		Confidence: 0.9,
		LagMs:      34000,
		Reason:     "Deployment update triggered rollout",
	}

	query := CreateTriggeredByEdgeQuery("effect-event", "cause-event", props)

	assert.Contains(t, query.Query, "TRIGGERED_BY")

	assert.Equal(t, "effect-event", query.Parameters["effectEventID"])
	assert.Equal(t, "cause-event", query.Parameters["causeEventID"])
	assert.Equal(t, 0.9, query.Parameters["confidence"])
	assert.Equal(t, int64(34000), query.Parameters["lagMs"])
	assert.Equal(t, "Deployment update triggered rollout", query.Parameters["reason"])
}

func TestCreateEmittedEventEdgeQuery(t *testing.T) {
	query := CreateEmittedEventEdgeQuery("resource-uid", "k8s-event-id")

	assert.Contains(t, query.Query, "EMITTED_EVENT")

	assert.Equal(t, "resource-uid", query.Parameters["resourceUID"])
	assert.Equal(t, "k8s-event-id", query.Parameters["k8sEventID"])
}

func TestFindResourceByUIDQuery(t *testing.T) {
	query := FindResourceByUIDQuery("pod-123")

	assert.Contains(t, query.Query, "MATCH")
	assert.Contains(t, query.Query, "ResourceIdentity")
	assert.Contains(t, query.Query, "RETURN")

	assert.Equal(t, "pod-123", query.Parameters["uid"])
}

func TestFindChangeEventsByResourceQuery(t *testing.T) {
	startTime := int64(1703001000000000000)
	endTime := int64(1703002000000000000)

	query := FindChangeEventsByResourceQuery("pod-123", startTime, endTime)

	assert.Contains(t, query.Query, "MATCH")
	assert.Contains(t, query.Query, "CHANGED")
	assert.Contains(t, query.Query, "WHERE")
	assert.Contains(t, query.Query, "ORDER BY")

	assert.Equal(t, "pod-123", query.Parameters["resourceUID"])
	assert.Equal(t, startTime, query.Parameters["startTime"])
	assert.Equal(t, endTime, query.Parameters["endTime"])
}

func TestFindRootCauseQuery(t *testing.T) {
	failureTimestamp := int64(1703001000000000000)
	maxDepth := 5
	minConfidence := 0.6

	query := FindRootCauseQuery("pod-123", failureTimestamp, maxDepth, minConfidence)

	// Check query structure
	assert.Contains(t, query.Query, "MATCH")
	assert.Contains(t, query.Query, "failedResource")
	assert.Contains(t, query.Query, "TRIGGERED_BY")
	assert.Contains(t, query.Query, "WHERE")
	assert.Contains(t, query.Query, "OWNS")

	// Check depth is embedded in query (not parameterized)
	assert.Contains(t, query.Query, "*1..5")

	// Check parameters
	assert.Equal(t, "pod-123", query.Parameters["resourceUID"])
	assert.Equal(t, failureTimestamp, query.Parameters["failureTimestamp"])
	assert.Equal(t, 0.6, query.Parameters["minConfidence"])
	require.NotNil(t, query.Parameters["tolerance"])
}

func TestCalculateBlastRadiusQuery(t *testing.T) {
	changeTimestamp := int64(1703001000000000000)
	timeWindowMs := int64(300000) // 5 minutes
	relationshipTypes := []string{"OWNS", "SELECTS"}

	query := CalculateBlastRadiusQuery("node-123", changeTimestamp, timeWindowMs, relationshipTypes)

	// Check query structure
	assert.Contains(t, query.Query, "MATCH")
	assert.Contains(t, query.Query, "triggerResource")
	assert.Contains(t, query.Query, "impacted")
	assert.Contains(t, query.Query, "WHERE")

	// Check relationship types are in query
	queryLower := strings.ToLower(query.Query)
	assert.True(t, strings.Contains(queryLower, "owns") || strings.Contains(queryLower, "selects"))

	// Check parameters
	assert.Equal(t, "node-123", query.Parameters["resourceUID"])
	assert.Equal(t, changeTimestamp, query.Parameters["changeTimestamp"])
	assert.Equal(t, timeWindowMs*1_000_000, query.Parameters["timeWindowNs"])
}

func TestDeleteOldChangeEventsQuery(t *testing.T) {
	cutoffNs := int64(1703001000000000000)

	query := DeleteOldChangeEventsQuery(cutoffNs)

	assert.Contains(t, query.Query, "MATCH")
	assert.Contains(t, query.Query, "ChangeEvent")
	assert.Contains(t, query.Query, "WHERE")
	assert.Contains(t, query.Query, "DETACH DELETE")

	assert.Equal(t, cutoffNs, query.Parameters["cutoffNs"])
}

func TestDeleteOldK8sEventsQuery(t *testing.T) {
	cutoffNs := int64(1703001000000000000)

	query := DeleteOldK8sEventsQuery(cutoffNs)

	assert.Contains(t, query.Query, "K8sEvent")
	assert.Contains(t, query.Query, "DETACH DELETE")

	assert.Equal(t, cutoffNs, query.Parameters["cutoffNs"])
}

func TestGetGraphStatsQuery(t *testing.T) {
	query := GetGraphStatsQuery()

	assert.Contains(t, query.Query, "MATCH")
	assert.Contains(t, query.Query, "RETURN")
	assert.NotContains(t, query.Query, "$") // Should not have parameters
	assert.Nil(t, query.Parameters)
}

// =============================================================================
// Batch Query Builder Tests
// =============================================================================

func TestBatchUpsertResourceIdentitiesQuery(t *testing.T) {
	resources := []ResourceIdentity{
		{
			UID:       "pod-1",
			Kind:      "Pod",
			APIGroup:  "",
			Version:   "v1",
			Namespace: "default",
			Name:      "frontend-1",
			Labels:    map[string]string{"app": "frontend"},
			FirstSeen: 1703001000000000000,
			LastSeen:  1703002000000000000,
			Deleted:   false,
		},
		{
			UID:       "pod-2",
			Kind:      "Pod",
			APIGroup:  "",
			Version:   "v1",
			Namespace: "default",
			Name:      "frontend-2",
			Labels:    map[string]string{"app": "frontend", "tier": "web"},
			FirstSeen: 1703001000000000000,
			LastSeen:  1703002000000000000,
			Deleted:   false,
		},
	}

	query := BatchUpsertResourceIdentitiesQuery(resources)

	// Check query structure
	assert.Contains(t, query.Query, "UNWIND")
	assert.Contains(t, query.Query, "$resources")
	assert.Contains(t, query.Query, "MERGE")
	assert.Contains(t, query.Query, "ResourceIdentity")
	assert.Contains(t, query.Query, "ON CREATE SET")
	assert.Contains(t, query.Query, "ON MATCH SET")

	// Check parameters
	resourceParams, ok := query.Parameters["resources"].([]map[string]interface{})
	require.True(t, ok)
	assert.Len(t, resourceParams, 2)

	assert.Equal(t, "pod-1", resourceParams[0]["uid"])
	assert.Equal(t, "Pod", resourceParams[0]["kind"])
	assert.Equal(t, "default", resourceParams[0]["namespace"])
	assert.Equal(t, "frontend-1", resourceParams[0]["name"])

	assert.Equal(t, "pod-2", resourceParams[1]["uid"])
	assert.Equal(t, "frontend-2", resourceParams[1]["name"])
}

func TestBatchUpsertResourceIdentitiesQuery_EmptySlice(t *testing.T) {
	resources := []ResourceIdentity{}

	query := BatchUpsertResourceIdentitiesQuery(resources)

	// Should still produce valid query
	assert.Contains(t, query.Query, "UNWIND")
	resourceParams, ok := query.Parameters["resources"].([]map[string]interface{})
	require.True(t, ok)
	assert.Len(t, resourceParams, 0)
}

func TestBatchUpsertResourceIdentitiesQuery_LabelsSerializedAsJSON(t *testing.T) {
	resources := []ResourceIdentity{
		{
			UID:    "pod-1",
			Labels: map[string]string{"app": "test", "env": "prod"},
		},
	}

	query := BatchUpsertResourceIdentitiesQuery(resources)

	resourceParams, ok := query.Parameters["resources"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, resourceParams, 1)

	// Labels should be JSON string, not map
	labelsJSON, ok := resourceParams[0]["labels"].(string)
	require.True(t, ok)
	assert.Contains(t, labelsJSON, "app")
	assert.Contains(t, labelsJSON, "test")
}

func TestBatchCreateChangeEventsQuery(t *testing.T) {
	events := []ChangeEvent{
		{
			ID:              "event-1",
			Timestamp:       1703001000000000000,
			EventType:       "CREATE",
			Status:          "Ready",
			ConfigChanged:   true,
			StatusChanged:   false,
			ReplicasChanged: false,
			ImpactScore:     0.1,
		},
		{
			ID:              "event-2",
			Timestamp:       1703002000000000000,
			EventType:       "UPDATE",
			Status:          "Error",
			ErrorMessage:    "CrashLoopBackOff",
			ContainerIssues: []string{"CrashLoopBackOff"},
			ConfigChanged:   false,
			StatusChanged:   true,
			ReplicasChanged: false,
			ImpactScore:     0.9,
		},
	}

	query := BatchCreateChangeEventsQuery(events)

	// Check query structure
	assert.Contains(t, query.Query, "UNWIND")
	assert.Contains(t, query.Query, "$events")
	assert.Contains(t, query.Query, "MERGE")
	assert.Contains(t, query.Query, "ChangeEvent")
	assert.Contains(t, query.Query, "ON CREATE SET")

	// Check parameters
	eventParams, ok := query.Parameters["events"].([]map[string]interface{})
	require.True(t, ok)
	assert.Len(t, eventParams, 2)

	assert.Equal(t, "event-1", eventParams[0]["id"])
	assert.Equal(t, "CREATE", eventParams[0]["eventType"])
	assert.Equal(t, "Ready", eventParams[0]["status"])
	assert.Equal(t, 0.1, eventParams[0]["impactScore"])

	assert.Equal(t, "event-2", eventParams[1]["id"])
	assert.Equal(t, "UPDATE", eventParams[1]["eventType"])
	assert.Equal(t, "Error", eventParams[1]["status"])
	assert.Equal(t, "CrashLoopBackOff", eventParams[1]["errorMessage"])
}

func TestBatchCreateK8sEventsQuery(t *testing.T) {
	events := []K8sEvent{
		{
			ID:        "k8s-event-1",
			Timestamp: 1703001000000000000,
			Reason:    "Scheduled",
			Message:   "Successfully assigned pod to node",
			Type:      "Normal",
			Count:     1,
			Source:    "scheduler",
		},
		{
			ID:        "k8s-event-2",
			Timestamp: 1703002000000000000,
			Reason:    "FailedMount",
			Message:   "Unable to mount volume",
			Type:      "Warning",
			Count:     3,
			Source:    "kubelet",
		},
	}

	query := BatchCreateK8sEventsQuery(events)

	// Check query structure
	assert.Contains(t, query.Query, "UNWIND")
	assert.Contains(t, query.Query, "$events")
	assert.Contains(t, query.Query, "MERGE")
	assert.Contains(t, query.Query, "K8sEvent")

	// Check parameters
	eventParams, ok := query.Parameters["events"].([]map[string]interface{})
	require.True(t, ok)
	assert.Len(t, eventParams, 2)

	assert.Equal(t, "k8s-event-1", eventParams[0]["id"])
	assert.Equal(t, "Scheduled", eventParams[0]["reason"])
	assert.Equal(t, "Normal", eventParams[0]["type"])
	assert.Equal(t, 1, eventParams[0]["count"])

	assert.Equal(t, "k8s-event-2", eventParams[1]["id"])
	assert.Equal(t, "Warning", eventParams[1]["type"])
	assert.Equal(t, 3, eventParams[1]["count"])
}

func TestBatchCreateOwnsEdgesQuery(t *testing.T) {
	edges := []BatchEdgeParams{
		{
			FromUID: "deployment-1",
			ToUID:   "replicaset-1",
			Properties: map[string]interface{}{
				"controller":         true,
				"blockOwnerDeletion": true,
			},
		},
		{
			FromUID: "replicaset-1",
			ToUID:   "pod-1",
			Properties: map[string]interface{}{
				"controller":         true,
				"blockOwnerDeletion": false,
			},
		},
	}

	query := BatchCreateOwnsEdgesQuery(edges)

	// Check query structure
	assert.Contains(t, query.Query, "UNWIND")
	assert.Contains(t, query.Query, "$edges")
	assert.Contains(t, query.Query, "MATCH")
	assert.Contains(t, query.Query, "MERGE")
	assert.Contains(t, query.Query, "OWNS")

	// Check parameters
	edgeParams, ok := query.Parameters["edges"].([]map[string]interface{})
	require.True(t, ok)
	assert.Len(t, edgeParams, 2)

	assert.Equal(t, "deployment-1", edgeParams[0]["fromUID"])
	assert.Equal(t, "replicaset-1", edgeParams[0]["toUID"])
	assert.Equal(t, true, edgeParams[0]["controller"])

	assert.Equal(t, "replicaset-1", edgeParams[1]["fromUID"])
	assert.Equal(t, "pod-1", edgeParams[1]["toUID"])
}

func TestBatchCreateChangedEdgesQuery(t *testing.T) {
	edges := []BatchEdgeParams{
		{
			FromUID:    "pod-1",
			ToUID:      "event-1",
			Properties: map[string]interface{}{"sequenceNumber": 1},
		},
		{
			FromUID:    "pod-1",
			ToUID:      "event-2",
			Properties: map[string]interface{}{"sequenceNumber": 2},
		},
	}

	query := BatchCreateChangedEdgesQuery(edges)

	assert.Contains(t, query.Query, "UNWIND")
	assert.Contains(t, query.Query, "CHANGED")
	assert.Contains(t, query.Query, "sequenceNumber")

	edgeParams, ok := query.Parameters["edges"].([]map[string]interface{})
	require.True(t, ok)
	assert.Len(t, edgeParams, 2)
}

func TestBatchCreateSelectsEdgesQuery(t *testing.T) {
	edges := []BatchEdgeParams{
		{
			FromUID: "service-1",
			ToUID:   "pod-1",
			Properties: map[string]interface{}{
				"selector":  `{"app":"frontend"}`,
				"matchType": "labels",
			},
		},
		{
			FromUID: "service-1",
			ToUID:   "pod-2",
			Properties: map[string]interface{}{
				"selector":  `{"app":"frontend"}`,
				"matchType": "labels",
			},
		},
	}

	query := BatchCreateSelectsEdgesQuery(edges)

	assert.Contains(t, query.Query, "UNWIND")
	assert.Contains(t, query.Query, "SELECTS")
	assert.Contains(t, query.Query, "selector")
	assert.Contains(t, query.Query, "matchType")

	edgeParams, ok := query.Parameters["edges"].([]map[string]interface{})
	require.True(t, ok)
	assert.Len(t, edgeParams, 2)
}

func TestBatchCreateScheduledOnEdgesQuery(t *testing.T) {
	edges := []BatchEdgeParams{
		{
			FromUID: "pod-1",
			ToUID:   "node-1",
			Properties: map[string]interface{}{
				"scheduledAt": int64(1703001000000000000),
				"hostIP":      "10.0.0.1",
			},
		},
	}

	query := BatchCreateScheduledOnEdgesQuery(edges)

	assert.Contains(t, query.Query, "UNWIND")
	assert.Contains(t, query.Query, "SCHEDULED_ON")
	assert.Contains(t, query.Query, "scheduledAt")
	assert.Contains(t, query.Query, "hostIP")
}

func TestBatchCreateMountsEdgesQuery(t *testing.T) {
	edges := []BatchEdgeParams{
		{
			FromUID: "pod-1",
			ToUID:   "configmap-1",
			Properties: map[string]interface{}{
				"mountPath": "/etc/config",
				"readOnly":  true,
				"subPath":   "",
			},
		},
	}

	query := BatchCreateMountsEdgesQuery(edges)

	assert.Contains(t, query.Query, "UNWIND")
	assert.Contains(t, query.Query, "MOUNTS")
	assert.Contains(t, query.Query, "mountPath")
	assert.Contains(t, query.Query, "readOnly")
}

func TestBatchCreateTriggeredByEdgesQuery(t *testing.T) {
	edges := []BatchEdgeParams{
		{
			FromUID: "effect-event-1",
			ToUID:   "cause-event-1",
			Properties: map[string]interface{}{
				"confidence": 0.9,
				"lagMs":      int64(5000),
				"reason":     "Deployment rollout",
			},
		},
	}

	query := BatchCreateTriggeredByEdgesQuery(edges)

	assert.Contains(t, query.Query, "UNWIND")
	assert.Contains(t, query.Query, "TRIGGERED_BY")
	assert.Contains(t, query.Query, "confidence")
	assert.Contains(t, query.Query, "lagMs")
	assert.Contains(t, query.Query, "reason")
}
