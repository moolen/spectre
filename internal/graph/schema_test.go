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
