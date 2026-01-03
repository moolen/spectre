package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
)

func TestPipeline_ProcessSingleEvent(t *testing.T) {
	harness, err := NewTestHarness(t)
	if err != nil {
		t.Fatalf("Failed to create test harness: %v", err)
	}
	defer harness.Cleanup(context.Background())

	ctx := context.Background()
	client := harness.GetClient()

	// Create a Pod event
	podUID := uuid.New().String()
	event := CreatePodEvent(
		podUID,
		"test-pod",
		"default",
		time.Now(),
		models.EventTypeCreate,
		"Running",
		nil,
	)

	// Process event
	err = harness.SeedEvent(ctx, event)
	if err != nil {
		t.Fatalf("Failed to process event: %v", err)
	}

	// Verify resource exists
	AssertResourceExists(t, client, podUID)

	// Verify event count
	AssertEventCount(t, client, podUID, 1)
}

func TestPipeline_ProcessBatch(t *testing.T) {
	harness, err := NewTestHarness(t)
	if err != nil {
		t.Fatalf("Failed to create test harness: %v", err)
	}
	defer harness.Cleanup(context.Background())

	ctx := context.Background()
	client := harness.GetClient()

	// Create multiple events
	baseTime := time.Now()
	events := []models.Event{
		CreatePodEvent(uuid.New().String(), "pod-1", "default", baseTime, models.EventTypeCreate, "Running", nil),
		CreatePodEvent(uuid.New().String(), "pod-2", "default", baseTime.Add(1*time.Second), models.EventTypeCreate, "Running", nil),
		CreatePodEvent(uuid.New().String(), "pod-3", "default", baseTime.Add(2*time.Second), models.EventTypeCreate, "Running", nil),
	}

	// Process batch
	err = harness.SeedEvents(ctx, events)
	if err != nil {
		t.Fatalf("Failed to process batch: %v", err)
	}

	// Verify all resources exist
	for _, event := range events {
		AssertResourceExists(t, client, event.Resource.UID)
		AssertEventCount(t, client, event.Resource.UID, 1)
	}
}

func TestPipeline_OwnershipChain(t *testing.T) {
	harness, err := NewTestHarness(t)
	if err != nil {
		t.Fatalf("Failed to create test harness: %v", err)
	}
	defer harness.Cleanup(context.Background())

	ctx := context.Background()
	client := harness.GetClient()

	// Create ownership chain: Deployment -> ReplicaSet -> Pod
	deploymentUID := uuid.New().String()
	replicasetUID := uuid.New().String()
	podUID := uuid.New().String()

	baseTime := time.Now()

	// Create Deployment
	deploymentEvent := CreateDeploymentEvent(
		deploymentUID,
		"test-deployment",
		"default",
		baseTime,
		models.EventTypeCreate,
		1,
	)

	// Create ReplicaSet with owner reference
	replicasetData := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "ReplicaSet",
		"metadata": map[string]interface{}{
			"name":      "test-replicaset",
			"namespace": "default",
			"uid":       replicasetUID,
			"ownerReferences": []map[string]interface{}{
				{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       "test-deployment",
					"uid":        deploymentUID,
					"controller": true,
				},
			},
		},
		"spec": map[string]interface{}{
			"replicas": 1,
		},
	}
	replicasetEvent := CreateReplicaSetEvent(replicasetUID, "test-replicaset", "default", baseTime.Add(1*time.Second), models.EventTypeCreate, 1, deploymentUID)
	replicasetEvent.Data, _ = json.Marshal(replicasetData)
	replicasetEvent.DataSize = int32(len(replicasetEvent.Data))

	// Create Pod with owner reference
	podData := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":      "test-pod",
			"namespace": "default",
			"uid":       podUID,
			"ownerReferences": []map[string]interface{}{
				{
					"apiVersion": "apps/v1",
					"kind":       "ReplicaSet",
					"name":       "test-replicaset",
					"uid":        replicasetUID,
					"controller": true,
				},
			},
		},
		"spec": map[string]interface{}{
			"containers": []map[string]interface{}{
				{"name": "main", "image": "nginx:latest"},
			},
		},
	}
	podEvent := CreatePodEvent(podUID, "test-pod", "default", baseTime.Add(2*time.Second), models.EventTypeCreate, "Running", podData)

	events := []models.Event{deploymentEvent, replicasetEvent, podEvent}

	// Process batch
	err = harness.SeedEvents(ctx, events)
	if err != nil {
		t.Fatalf("Failed to process ownership chain: %v", err)
	}

	// Verify OWNS edges exist
	AssertEdgeExists(t, client, deploymentUID, replicasetUID, graph.EdgeTypeOwns)
	AssertEdgeExists(t, client, replicasetUID, podUID, graph.EdgeTypeOwns)
}

func TestPipeline_ChangeEvents(t *testing.T) {
	harness, err := NewTestHarness(t)
	if err != nil {
		t.Fatalf("Failed to create test harness: %v", err)
	}
	defer harness.Cleanup(context.Background())

	ctx := context.Background()
	client := harness.GetClient()

	// Create multiple change events for the same resource
	podUID := uuid.New().String()
	baseTime := time.Now()

	events := []models.Event{
		CreatePodEvent(podUID, "test-pod", "default", baseTime, models.EventTypeCreate, "Pending", nil),
		CreatePodEvent(podUID, "test-pod", "default", baseTime.Add(5*time.Second), models.EventTypeUpdate, "Running", nil),
		CreatePodEvent(podUID, "test-pod", "default", baseTime.Add(10*time.Second), models.EventTypeUpdate, "Ready", nil),
	}

	err = harness.SeedEvents(ctx, events)
	if err != nil {
		t.Fatalf("Failed to process change events: %v", err)
	}

	// Verify resource exists
	AssertResourceExists(t, client, podUID)

	// Verify we have 3 events
	AssertEventCount(t, client, podUID, 3)

	// Verify CHANGED edges exist (one per event)
	// The pipeline should create CHANGED edges from resource to each event
	// We can verify by counting edges
	edgeCount := CountEdges(t, client, graph.EdgeTypeChanged)
	if edgeCount < 3 {
		t.Errorf("Expected at least 3 CHANGED edges, got %d", edgeCount)
	}
}

func TestPipeline_ResourceDeletion(t *testing.T) {
	harness, err := NewTestHarness(t)
	if err != nil {
		t.Fatalf("Failed to create test harness: %v", err)
	}
	defer harness.Cleanup(context.Background())

	ctx := context.Background()
	client := harness.GetClient()

	podUID := uuid.New().String()
	baseTime := time.Now()

	// Create resource
	createEvent := CreatePodEvent(podUID, "test-pod", "default", baseTime, models.EventTypeCreate, "Running", nil)
	err = harness.SeedEvent(ctx, createEvent)
	if err != nil {
		t.Fatalf("Failed to create resource: %v", err)
	}

	// Delete resource
	deleteEvent := CreatePodEvent(podUID, "test-pod", "default", baseTime.Add(10*time.Second), models.EventTypeDelete, "Terminating", nil)
	err = harness.SeedEvent(ctx, deleteEvent)
	if err != nil {
		t.Fatalf("Failed to delete resource: %v", err)
	}

	// Verify resource still exists (but marked as deleted)
	// The graph should still have the resource node, but with deleted=true
	AssertResourceExists(t, client, podUID)

	// Verify we have 2 events (create + delete)
	AssertEventCount(t, client, podUID, 2)
}

func TestPipeline_ConcurrentEvents(t *testing.T) {
	harness, err := NewTestHarness(t)
	if err != nil {
		t.Fatalf("Failed to create test harness: %v", err)
	}
	defer harness.Cleanup(context.Background())

	ctx := context.Background()
	client := harness.GetClient()

	// Create events for different resources at the same time
	baseTime := time.Now()
	events := make([]models.Event, 10)

	for i := 0; i < 10; i++ {
		podUID := uuid.New().String()
		events[i] = CreatePodEvent(
			podUID,
			fmt.Sprintf("pod-%d", i),
			"default",
			baseTime.Add(time.Duration(i)*time.Millisecond),
			models.EventTypeCreate,
			"Running",
			nil,
		)
	}

	// Process all events in a batch
	err = harness.SeedEvents(ctx, events)
	if err != nil {
		t.Fatalf("Failed to process concurrent events: %v", err)
	}

	// Verify all resources exist
	for _, event := range events {
		AssertResourceExists(t, client, event.Resource.UID)
	}
}
