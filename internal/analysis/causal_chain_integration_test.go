//go:build integration
// +build integration

package analysis

import (
	"context"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCausalChainIntegration requires FalkorDB to be running
// Run with: make graph-up && go test ./internal/analysis -v -tags=integration -run Integration
func TestCausalChainIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create client
	config := graph.DefaultClientConfig()
	client := graph.NewClient(config)

	ctx := context.Background()

	// Connect to FalkorDB
	err := client.Connect(ctx)
	require.NoError(t, err, "Failed to connect to FalkorDB - make sure it's running (make graph-up)")
	defer client.Close()

	// Clean up any existing graph data
	cleanupGraph(t, client)

	t.Run("getOwnershipChain_simple_pod", func(t *testing.T) {
		cleanupGraph(t, client)
		now := time.Now().UnixNano()

		// Create: Pod <- ReplicaSet <- Deployment
		pod := graph.ResourceIdentity{
			UID:       "pod-uid-001",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "my-pod-abc123",
			FirstSeen: now,
			LastSeen:  now,
		}
		rs := graph.ResourceIdentity{
			UID:       "rs-uid-001",
			Kind:      "ReplicaSet",
			Namespace: "default",
			Name:      "my-deployment-abc123",
			FirstSeen: now,
			LastSeen:  now,
		}
		deploy := graph.ResourceIdentity{
			UID:       "deploy-uid-001",
			Kind:      "Deployment",
			Namespace: "default",
			Name:      "my-deployment",
			FirstSeen: now,
			LastSeen:  now,
		}

		// Create resources
		createResource(t, client, pod)
		createResource(t, client, rs)
		createResource(t, client, deploy)

		// Create OWNS edges: Deployment -> ReplicaSet -> Pod
		createOwnsEdge(t, client, deploy.UID, rs.UID)
		createOwnsEdge(t, client, rs.UID, pod.UID)

		// Create analyzer and test
		analyzer := NewRootCauseAnalyzer(client)

		chain, err := analyzer.getOwnershipChain(ctx, pod.UID)
		require.NoError(t, err)
		require.Len(t, chain, 3)

		// Verify order: Pod (distance 0), ReplicaSet (distance 1), Deployment (distance 2)
		assert.Equal(t, pod.UID, chain[0].Resource.UID)
		assert.Equal(t, 0, chain[0].Distance)

		assert.Equal(t, rs.UID, chain[1].Resource.UID)
		assert.Equal(t, 1, chain[1].Distance)

		assert.Equal(t, deploy.UID, chain[2].Resource.UID)
		assert.Equal(t, 2, chain[2].Distance)
	})

	t.Run("getManagers_helmrelease", func(t *testing.T) {
		cleanupGraph(t, client)
		now := time.Now().UnixNano()

		// Create: Deployment managed by HelmRelease
		deploy := graph.ResourceIdentity{
			UID:       "deploy-uid-002",
			Kind:      "Deployment",
			Namespace: "default",
			Name:      "my-app",
			FirstSeen: now,
			LastSeen:  now,
		}
		hr := graph.ResourceIdentity{
			UID:       "hr-uid-001",
			Kind:      "HelmRelease",
			Namespace: "default",
			Name:      "my-app-release",
			FirstSeen: now,
			LastSeen:  now,
		}

		createResource(t, client, deploy)
		createResource(t, client, hr)
		createManagesEdge(t, client, hr.UID, deploy.UID, 0.85)

		analyzer := NewRootCauseAnalyzer(client)

		managers, err := analyzer.getManagers(ctx, []string{deploy.UID})
		require.NoError(t, err)

		mgrData, ok := managers[deploy.UID]
		require.True(t, ok, "Should have manager for deployment")
		assert.Equal(t, hr.UID, mgrData.Manager.UID)
		assert.Equal(t, "HelmRelease", mgrData.Manager.Kind)
		assert.InDelta(t, 0.85, mgrData.ManagesEdge.Confidence, 0.01)
	})

	t.Run("getRelatedResources_pod_with_node_and_sa", func(t *testing.T) {
		cleanupGraph(t, client)
		now := time.Now().UnixNano()

		// Create Pod, Node, ServiceAccount
		pod := graph.ResourceIdentity{
			UID:       "pod-uid-003",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "my-pod",
			FirstSeen: now,
			LastSeen:  now,
		}
		node := graph.ResourceIdentity{
			UID:       "node-uid-001",
			Kind:      "Node",
			Namespace: "",
			Name:      "worker-1",
			FirstSeen: now,
			LastSeen:  now,
		}
		sa := graph.ResourceIdentity{
			UID:       "sa-uid-001",
			Kind:      "ServiceAccount",
			Namespace: "default",
			Name:      "my-sa",
			FirstSeen: now,
			LastSeen:  now,
		}

		createResource(t, client, pod)
		createResource(t, client, node)
		createResource(t, client, sa)

		// Create relationships
		createScheduledOnEdge(t, client, pod.UID, node.UID)
		createUsesServiceAccountEdge(t, client, pod.UID, sa.UID)

		analyzer := NewRootCauseAnalyzer(client)

		related, err := analyzer.getRelatedResources(ctx, []string{pod.UID})
		require.NoError(t, err)

		relList := related[pod.UID]
		assert.GreaterOrEqual(t, len(relList), 2)

		// Verify we have both relationships
		hasNode := false
		hasSA := false
		for _, rel := range relList {
			if rel.RelationshipType == "SCHEDULED_ON" && rel.Resource.UID == node.UID {
				hasNode = true
			}
			if rel.RelationshipType == "USES_SERVICE_ACCOUNT" && rel.Resource.UID == sa.UID {
				hasSA = true
			}
		}
		assert.True(t, hasNode, "Should have SCHEDULED_ON relationship")
		assert.True(t, hasSA, "Should have USES_SERVICE_ACCOUNT relationship")
	})

	t.Run("getChangeEvents_within_lookback", func(t *testing.T) {
		cleanupGraph(t, client)
		failureTime := time.Now()
		lookback := 10 * time.Minute

		// Create pod
		pod := graph.ResourceIdentity{
			UID:       "pod-uid-004",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "my-pod",
			FirstSeen: failureTime.Add(-1 * time.Hour).UnixNano(),
			LastSeen:  failureTime.UnixNano(),
		}
		createResource(t, client, pod)

		// Create events: 2 within lookback, 1 outside
		event1 := graph.ChangeEvent{
			ID:            "event-001",
			Timestamp:     failureTime.Add(-5 * time.Minute).UnixNano(),
			EventType:     "UPDATE",
			ConfigChanged: true,
		}
		event2 := graph.ChangeEvent{
			ID:            "event-002",
			Timestamp:     failureTime.Add(-2 * time.Minute).UnixNano(),
			EventType:     "UPDATE",
			StatusChanged: true,
		}
		event3 := graph.ChangeEvent{
			ID:            "event-003",
			Timestamp:     failureTime.Add(-30 * time.Minute).UnixNano(), // Outside lookback
			EventType:     "CREATE",
		}

		createChangeEvent(t, client, pod.UID, event1)
		createChangeEvent(t, client, pod.UID, event2)
		createChangeEvent(t, client, pod.UID, event3)

		analyzer := NewRootCauseAnalyzer(client)

		events, err := analyzer.getChangeEvents(ctx, []string{pod.UID}, failureTime.UnixNano(), lookback.Nanoseconds())
		require.NoError(t, err)

		podEvents := events[pod.UID]
		assert.Len(t, podEvents, 2, "Should only get 2 events within lookback window")

		// Verify the right events are returned
		eventIDs := map[string]bool{}
		for _, e := range podEvents {
			eventIDs[e.EventID] = true
		}
		assert.True(t, eventIDs["event-001"], "Should include event-001")
		assert.True(t, eventIDs["event-002"], "Should include event-002")
		assert.False(t, eventIDs["event-003"], "Should NOT include event-003 (outside lookback)")
	})

	t.Run("getK8sEvents_attached_to_pod", func(t *testing.T) {
		cleanupGraph(t, client)
		failureTime := time.Now()
		lookback := 10 * time.Minute

		pod := graph.ResourceIdentity{
			UID:       "pod-uid-005",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "my-failing-pod",
			FirstSeen: failureTime.Add(-1 * time.Hour).UnixNano(),
			LastSeen:  failureTime.UnixNano(),
		}
		createResource(t, client, pod)

		// Create K8s events
		k8sEvent1 := graph.K8sEvent{
			ID:        "k8s-event-001",
			Timestamp: failureTime.Add(-3 * time.Minute).UnixNano(),
			Reason:    "BackOff",
			Message:   "Back-off restarting failed container",
			Type:      "Warning",
			Count:     5,
			Source:    "kubelet",
		}
		k8sEvent2 := graph.K8sEvent{
			ID:        "k8s-event-002",
			Timestamp: failureTime.Add(-1 * time.Minute).UnixNano(),
			Reason:    "FailedMount",
			Message:   "Unable to mount volume",
			Type:      "Warning",
			Count:     3,
			Source:    "kubelet",
		}

		createK8sEvent(t, client, pod.UID, k8sEvent1)
		createK8sEvent(t, client, pod.UID, k8sEvent2)

		analyzer := NewRootCauseAnalyzer(client)

		k8sEvents, err := analyzer.getK8sEvents(ctx, []string{pod.UID}, failureTime.UnixNano(), lookback.Nanoseconds())
		require.NoError(t, err)

		podK8sEvents := k8sEvents[pod.UID]
		assert.Len(t, podK8sEvents, 2)

		// Verify events
		reasons := map[string]bool{}
		for _, e := range podK8sEvents {
			reasons[e.Reason] = true
		}
		assert.True(t, reasons["BackOff"], "Should include BackOff event")
		assert.True(t, reasons["FailedMount"], "Should include FailedMount event")
	})

	t.Run("buildCausalGraph_full_chain", func(t *testing.T) {
		cleanupGraph(t, client)
		failureTime := time.Now()
		lookback := 10 * time.Minute

		// Create full chain: Pod <- ReplicaSet <- Deployment <- HelmRelease
		pod := graph.ResourceIdentity{
			UID:       "pod-full-001",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "my-app-pod",
			FirstSeen: failureTime.Add(-1 * time.Hour).UnixNano(),
			LastSeen:  failureTime.UnixNano(),
		}
		rs := graph.ResourceIdentity{
			UID:       "rs-full-001",
			Kind:      "ReplicaSet",
			Namespace: "default",
			Name:      "my-app-rs",
			FirstSeen: failureTime.Add(-1 * time.Hour).UnixNano(),
			LastSeen:  failureTime.UnixNano(),
		}
		deploy := graph.ResourceIdentity{
			UID:       "deploy-full-001",
			Kind:      "Deployment",
			Namespace: "default",
			Name:      "my-app",
			FirstSeen: failureTime.Add(-1 * time.Hour).UnixNano(),
			LastSeen:  failureTime.UnixNano(),
		}
		hr := graph.ResourceIdentity{
			UID:       "hr-full-001",
			Kind:      "HelmRelease",
			Namespace: "flux-system",
			Name:      "my-app-release",
			FirstSeen: failureTime.Add(-1 * time.Hour).UnixNano(),
			LastSeen:  failureTime.UnixNano(),
		}
		node := graph.ResourceIdentity{
			UID:       "node-full-001",
			Kind:      "Node",
			Namespace: "",
			Name:      "worker-1",
			FirstSeen: failureTime.Add(-24 * time.Hour).UnixNano(),
			LastSeen:  failureTime.UnixNano(),
		}

		createResource(t, client, pod)
		createResource(t, client, rs)
		createResource(t, client, deploy)
		createResource(t, client, hr)
		createResource(t, client, node)

		// Create OWNS edges
		createOwnsEdge(t, client, deploy.UID, rs.UID)
		createOwnsEdge(t, client, rs.UID, pod.UID)

		// Create MANAGES edge
		createManagesEdge(t, client, hr.UID, deploy.UID, 0.9)

		// Create SCHEDULED_ON edge
		createScheduledOnEdge(t, client, pod.UID, node.UID)

		// Create change events
		hrEvent := graph.ChangeEvent{
			ID:            "hr-event-001",
			Timestamp:     failureTime.Add(-8 * time.Minute).UnixNano(),
			EventType:     "UPDATE",
			ConfigChanged: true,
		}
		podEvent := graph.ChangeEvent{
			ID:            "pod-event-001",
			Timestamp:     failureTime.Add(-1 * time.Minute).UnixNano(),
			EventType:     "UPDATE",
			StatusChanged: true,
			Status:        "CrashLoopBackOff",
		}

		createChangeEvent(t, client, hr.UID, hrEvent)
		createChangeEvent(t, client, pod.UID, podEvent)

		// Create K8s event
		k8sEvent := graph.K8sEvent{
			ID:        "k8s-full-001",
			Timestamp: failureTime.Add(-30 * time.Second).UnixNano(),
			Reason:    "BackOff",
			Message:   "Back-off restarting failed container",
			Type:      "Warning",
			Count:     3,
			Source:    "kubelet",
		}
		createK8sEvent(t, client, pod.UID, k8sEvent)

		// Build causal graph
		analyzer := NewRootCauseAnalyzer(client)

		symptom := &ObservedSymptom{
			Resource: SymptomResource{
				UID:       pod.UID,
				Kind:      pod.Kind,
				Namespace: pod.Namespace,
				Name:      pod.Name,
			},
			Status:       "CrashLoopBackOff",
			ErrorMessage: "Back-off restarting failed container",
			ObservedAt:   failureTime,
			SymptomType:  "CrashLoop",
		}

		causalGraph, err := analyzer.buildCausalGraph(ctx, symptom, failureTime.UnixNano(), lookback.Nanoseconds())
		require.NoError(t, err)

		// Verify SPINE nodes
		spineNodes := []GraphNode{}
		relatedNodes := []GraphNode{}
		for _, n := range causalGraph.Nodes {
			if n.NodeType == "SPINE" {
				spineNodes = append(spineNodes, n)
			} else if n.NodeType == "RELATED" {
				relatedNodes = append(relatedNodes, n)
			}
		}

		// Should have at least Pod, ReplicaSet, Deployment in spine
		assert.GreaterOrEqual(t, len(spineNodes), 3, "Should have at least 3 SPINE nodes")

		// Should have Node as RELATED
		nodeFound := false
		for _, n := range relatedNodes {
			if n.Resource.Kind == "Node" {
				nodeFound = true
			}
		}
		assert.True(t, nodeFound, "Should have Node as RELATED node")

		// Verify edges
		ownsEdges := 0
		managesEdges := 0
		scheduledOnEdges := 0
		for _, e := range causalGraph.Edges {
			switch e.RelationshipType {
			case "OWNS":
				ownsEdges++
			case "MANAGES":
				managesEdges++
			case "SCHEDULED_ON":
				scheduledOnEdges++
			}
		}

		assert.GreaterOrEqual(t, ownsEdges, 2, "Should have at least 2 OWNS edges")
		assert.GreaterOrEqual(t, scheduledOnEdges, 1, "Should have at least 1 SCHEDULED_ON edge")

		// Verify Pod has K8s events
		for _, n := range causalGraph.Nodes {
			if n.Resource.UID == pod.UID {
				assert.NotEmpty(t, n.K8sEvents, "Pod should have K8s events attached")
			}
		}
	})

	t.Run("buildCausalGraph_empty_chain", func(t *testing.T) {
		cleanupGraph(t, client)
		failureTime := time.Now()
		lookback := 10 * time.Minute

		// Create a single Pod with no owners
		pod := graph.ResourceIdentity{
			UID:       "orphan-pod-001",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "orphan-pod",
			FirstSeen: failureTime.Add(-1 * time.Hour).UnixNano(),
			LastSeen:  failureTime.UnixNano(),
		}
		createResource(t, client, pod)

		analyzer := NewRootCauseAnalyzer(client)

		symptom := &ObservedSymptom{
			Resource: SymptomResource{
				UID:       pod.UID,
				Kind:      pod.Kind,
				Namespace: pod.Namespace,
				Name:      pod.Name,
			},
			Status:      "Error",
			ObservedAt:  failureTime,
			SymptomType: "Unknown",
		}

		causalGraph, err := analyzer.buildCausalGraph(ctx, symptom, failureTime.UnixNano(), lookback.Nanoseconds())
		require.NoError(t, err)

		// Should have at least the symptom node
		assert.GreaterOrEqual(t, len(causalGraph.Nodes), 1, "Should have at least the symptom node")

		// Find the pod node
		found := false
		for _, n := range causalGraph.Nodes {
			if n.Resource.UID == pod.UID {
				found = true
				assert.Equal(t, "SPINE", n.NodeType)
			}
		}
		assert.True(t, found, "Should have the pod in the graph")
	})

	// Clean up
	cleanupGraph(t, client)
}

// Helper functions

func cleanupGraph(t *testing.T, client graph.Client) {
	ctx := context.Background()
	// Use DeleteGraph for reliable cleanup (completely removes the graph)
	err := client.DeleteGraph(ctx)
	require.NoError(t, err)
}

func createResource(t *testing.T, client graph.Client, resource graph.ResourceIdentity) {
	ctx := context.Background()
	query := graph.UpsertResourceIdentityQuery(resource)
	_, err := client.ExecuteQuery(ctx, query)
	require.NoError(t, err)
}

func createOwnsEdge(t *testing.T, client graph.Client, ownerUID, ownedUID string) {
	ctx := context.Background()
	query := graph.CreateOwnsEdgeQuery(ownerUID, ownedUID, graph.OwnsEdge{Controller: true})
	_, err := client.ExecuteQuery(ctx, query)
	require.NoError(t, err)
}

func createManagesEdge(t *testing.T, client graph.Client, managerUID, managedUID string, confidence float64) {
	ctx := context.Background()
	query := graph.GraphQuery{
		Query: `
			MATCH (manager:ResourceIdentity {uid: $managerUID})
			MATCH (managed:ResourceIdentity {uid: $managedUID})
			MERGE (manager)-[r:MANAGES]->(managed)
			SET r.confidence = $confidence,
			    r.firstObserved = timestamp(),
			    r.lastValidated = timestamp(),
			    r.validationState = 'valid'
		`,
		Parameters: map[string]interface{}{
			"managerUID": managerUID,
			"managedUID": managedUID,
			"confidence": confidence,
		},
	}
	_, err := client.ExecuteQuery(ctx, query)
	require.NoError(t, err)
}

func createScheduledOnEdge(t *testing.T, client graph.Client, podUID, nodeUID string) {
	ctx := context.Background()
	query := graph.GraphQuery{
		Query: `
			MATCH (pod:ResourceIdentity {uid: $podUID})
			MATCH (node:ResourceIdentity {uid: $nodeUID})
			MERGE (pod)-[r:SCHEDULED_ON]->(node)
			SET r.scheduledAt = timestamp()
		`,
		Parameters: map[string]interface{}{
			"podUID":  podUID,
			"nodeUID": nodeUID,
		},
	}
	_, err := client.ExecuteQuery(ctx, query)
	require.NoError(t, err)
}

func createUsesServiceAccountEdge(t *testing.T, client graph.Client, podUID, saUID string) {
	ctx := context.Background()
	query := graph.GraphQuery{
		Query: `
			MATCH (pod:ResourceIdentity {uid: $podUID})
			MATCH (sa:ResourceIdentity {uid: $saUID})
			MERGE (pod)-[r:USES_SERVICE_ACCOUNT]->(sa)
		`,
		Parameters: map[string]interface{}{
			"podUID": podUID,
			"saUID":  saUID,
		},
	}
	_, err := client.ExecuteQuery(ctx, query)
	require.NoError(t, err)
}

func createChangeEvent(t *testing.T, client graph.Client, resourceUID string, event graph.ChangeEvent) {
	ctx := context.Background()

	// Create the event node
	eventQuery := graph.CreateChangeEventQuery(event)
	_, err := client.ExecuteQuery(ctx, eventQuery)
	require.NoError(t, err)

	// Link resource to event
	linkQuery := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity {uid: $resourceUID})
			MATCH (e:ChangeEvent {id: $eventID})
			MERGE (r)-[:CHANGED]->(e)
		`,
		Parameters: map[string]interface{}{
			"resourceUID": resourceUID,
			"eventID":     event.ID,
		},
	}
	_, err = client.ExecuteQuery(ctx, linkQuery)
	require.NoError(t, err)
}

func createK8sEvent(t *testing.T, client graph.Client, resourceUID string, event graph.K8sEvent) {
	ctx := context.Background()

	// Create the K8s event node
	query := graph.GraphQuery{
		Query: `
			MERGE (e:K8sEvent {id: $id})
			SET e.timestamp = $timestamp,
			    e.reason = $reason,
			    e.message = $message,
			    e.type = $type,
			    e.count = $count,
			    e.source = $source
		`,
		Parameters: map[string]interface{}{
			"id":        event.ID,
			"timestamp": event.Timestamp,
			"reason":    event.Reason,
			"message":   event.Message,
			"type":      event.Type,
			"count":     event.Count,
			"source":    event.Source,
		},
	}
	_, err := client.ExecuteQuery(ctx, query)
	require.NoError(t, err)

	// Link resource to K8s event
	linkQuery := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity {uid: $resourceUID})
			MATCH (e:K8sEvent {id: $eventID})
			MERGE (r)-[:EMITTED_EVENT]->(e)
		`,
		Parameters: map[string]interface{}{
			"resourceUID": resourceUID,
			"eventID":     event.ID,
		},
	}
	_, err = client.ExecuteQuery(ctx, linkQuery)
	require.NoError(t, err)
}
