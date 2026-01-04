package graph

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPerformance_LargeEventBatch tests processing a large batch of events
func TestPerformance_LargeEventBatch(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	ctx := context.Background()
	client := harness.GetClient()

	// Create 100 events
	baseTime := time.Now().Add(-1 * time.Hour)
	events := make([]models.Event, 100)

	for i := 0; i < 100; i++ {
		podUID := uuid.New().String()
		events[i] = CreatePodEvent(
			podUID,
			"perf-pod-"+uuid.New().String()[:8],
			"default",
			baseTime.Add(time.Duration(i)*time.Minute),
			models.EventTypeCreate,
			"Running",
			nil,
		)
	}

	// Process batch and measure time
	start := time.Now()
	err = harness.SeedEvents(ctx, events)
	duration := time.Since(start)

	require.NoError(t, err)
	assert.Less(t, duration, 10*time.Second, "Processing 100 events should take less than 10 seconds")

	// Verify all resources exist
	resourceCount := CountResources(t, client)
	assert.GreaterOrEqual(t, resourceCount, 100, "Should have at least 100 resources")
}

// TestPerformance_DeepOwnershipChain tests a deep ownership chain
func TestPerformance_DeepOwnershipChain(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	ctx := context.Background()

	// Create a chain: Deployment -> ReplicaSet -> Pod
	baseTime := time.Now().Add(-1 * time.Hour)
	deploymentUID := uuid.New().String()
	replicasetUID := uuid.New().String()
	podUID := uuid.New().String()

	// Note: This is a simplified version. Full implementation should create actual ownership chains
	events := []models.Event{
		CreateDeploymentEvent(deploymentUID, "test-deploy", "default", baseTime, models.EventTypeCreate, 1),
		CreateReplicaSetEvent(replicasetUID, "test-rs", "default", baseTime.Add(1*time.Second), models.EventTypeCreate, 1, deploymentUID),
		CreatePodEvent(podUID, "test-pod", "default", baseTime.Add(2*time.Second), models.EventTypeCreate, "Running", nil),
	}

	err = harness.SeedEvents(ctx, events)
	require.NoError(t, err)

	// Verify ownership chain
	client := harness.GetClient()
	AssertEdgeExists(t, client, deploymentUID, replicasetUID, graph.EdgeTypeOwns)
	// Note: Pod ownership would need proper owner references in the event data
}

// TODO: Add more performance tests:
// - TestPerformance_ComplexQuery
// - TestPerformance_Pagination
