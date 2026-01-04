package graph

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryExecutor_BasicTimelineQuery(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	ctx := context.Background()
	client := harness.GetClient()

	// Seed some events
	baseTime := time.Now().Add(-1 * time.Hour)
	events := []models.Event{
		CreatePodEvent(uuid.New().String(), "pod-1", "default", baseTime.Add(10*time.Minute), models.EventTypeCreate, "Running", nil),
		CreatePodEvent(uuid.New().String(), "pod-2", "default", baseTime.Add(20*time.Minute), models.EventTypeCreate, "Running", nil),
		CreateDeploymentEvent(uuid.New().String(), "deploy-1", "default", baseTime.Add(30*time.Minute), models.EventTypeCreate, 1),
	}

	err = harness.SeedEvents(ctx, events)
	require.NoError(t, err)

	// Query for events in time range
	queryExecutor := graph.NewQueryExecutor(client)
	query := &models.QueryRequest{
		StartTimestamp: baseTime.Unix(),
		EndTimestamp:   baseTime.Add(1 * time.Hour).Unix(),
		Filters:        models.QueryFilters{},
	}

	result, err := queryExecutor.Execute(ctx, query)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(result.Events), len(events), "Should return at least the events we created")
}

func TestQueryExecutor_FilterByKind(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	ctx := context.Background()
	client := harness.GetClient()

	baseTime := time.Now().Add(-1 * time.Hour)
	podUID := uuid.New().String()
	deploymentUID := uuid.New().String()

	events := []models.Event{
		CreatePodEvent(podUID, "pod-1", "default", baseTime.Add(10*time.Minute), models.EventTypeCreate, "Running", nil),
		CreateDeploymentEvent(deploymentUID, "deploy-1", "default", baseTime.Add(20*time.Minute), models.EventTypeCreate, 1),
	}

	err = harness.SeedEvents(ctx, events)
	require.NoError(t, err)

	// Query for Pods only
	queryExecutor := graph.NewQueryExecutor(client)
	query := &models.QueryRequest{
		StartTimestamp: baseTime.Unix(),
		EndTimestamp:   baseTime.Add(1 * time.Hour).Unix(),
		Filters: models.QueryFilters{
			Kind: "Pod",
		},
	}

	result, err := queryExecutor.Execute(ctx, query)
	require.NoError(t, err)

	// Verify all returned events are Pods
	for _, event := range result.Events {
		assert.Equal(t, "Pod", event.Resource.Kind, "All events should be Pods")
		assert.Equal(t, podUID, event.Resource.UID, "Should only return the Pod event")
	}
}

func TestQueryExecutor_FilterByNamespace(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	ctx := context.Background()
	client := harness.GetClient()

	baseTime := time.Now().Add(-1 * time.Hour)
	ns1UID := uuid.New().String()
	ns2UID := uuid.New().String()

	events := []models.Event{
		CreatePodEvent(ns1UID, "pod-1", "namespace1", baseTime.Add(10*time.Minute), models.EventTypeCreate, "Running", nil),
		CreatePodEvent(ns2UID, "pod-2", "namespace2", baseTime.Add(20*time.Minute), models.EventTypeCreate, "Running", nil),
	}

	err = harness.SeedEvents(ctx, events)
	require.NoError(t, err)

	// Query for namespace1 only
	queryExecutor := graph.NewQueryExecutor(client)
	query := &models.QueryRequest{
		StartTimestamp: baseTime.Unix(),
		EndTimestamp:   baseTime.Add(1 * time.Hour).Unix(),
		Filters: models.QueryFilters{
			Namespace: "namespace1",
		},
	}

	result, err := queryExecutor.Execute(ctx, query)
	require.NoError(t, err)

	// Verify all returned events are from namespace1
	for _, event := range result.Events {
		assert.Equal(t, "namespace1", event.Resource.Namespace, "All events should be from namespace1")
		assert.Equal(t, ns1UID, event.Resource.UID, "Should only return the namespace1 Pod event")
	}
}

func TestQueryExecutor_Pagination(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	ctx := context.Background()
	client := harness.GetClient()

	// Create many events
	baseTime := time.Now().Add(-1 * time.Hour)
	events := make([]models.Event, 20)

	for i := 0; i < 20; i++ {
		podUID := uuid.New().String()
		events[i] = CreatePodEvent(podUID, fmt.Sprintf("pod-%d", i), "default", baseTime.Add(time.Duration(i)*time.Minute), models.EventTypeCreate, "Running", nil)
	}

	err = harness.SeedEvents(ctx, events)
	require.NoError(t, err)

	queryExecutor := graph.NewQueryExecutor(client)
	query := &models.QueryRequest{
		StartTimestamp: baseTime.Unix(),
		EndTimestamp:   baseTime.Add(1 * time.Hour).Unix(),
		Filters:        models.QueryFilters{},
	}

	// First page
	pagination := &models.PaginationRequest{
		PageSize: 10,
	}

	result, paginationResp, err := queryExecutor.ExecutePaginated(ctx, query, pagination)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(result.Events), 10, "First page should have at most 10 resources")
	assert.True(t, paginationResp.HasMore, "Should have more pages")

	// Second page
	if paginationResp.NextCursor != "" {
		nextPagination := &models.PaginationRequest{
			PageSize: 10,
			Cursor:   paginationResp.NextCursor,
		}

		result2, _, err := queryExecutor.ExecutePaginated(ctx, query, nextPagination)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(result2.Events), 10, "Second page should have at most 10 resources")

		// Verify no overlap
		uids1 := make(map[string]bool)
		for _, event := range result.Events {
			uids1[event.Resource.UID] = true
		}

		uids2 := make(map[string]bool)
		for _, event := range result2.Events {
			uids2[event.Resource.UID] = true
		}

		// Check for overlap
		for uid := range uids1 {
			assert.False(t, uids2[uid], "No resource should appear in both pages")
		}
	}
}

func TestQueryExecutor_PreExistingResources(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	ctx := context.Background()
	client := harness.GetClient()

	// Create a resource before the query window
	beforeWindowTime := time.Now().Add(-2 * time.Hour)
	podUID := uuid.New().String()
	createEvent := CreatePodEvent(podUID, "pre-existing-pod", "default", beforeWindowTime, models.EventTypeCreate, "Running", nil)

	err = harness.SeedEvent(ctx, createEvent)
	require.NoError(t, err)

	// Create an update during the query window
	queryStartTime := time.Now().Add(-1 * time.Hour)
	updateEvent := CreatePodEvent(podUID, "pre-existing-pod", "default", queryStartTime.Add(10*time.Minute), models.EventTypeUpdate, "Ready", nil)

	err = harness.SeedEvent(ctx, updateEvent)
	require.NoError(t, err)

	// Query for events in the window
	queryExecutor := graph.NewQueryExecutor(client)
	query := &models.QueryRequest{
		StartTimestamp: queryStartTime.Unix(),
		EndTimestamp:   time.Now().Unix(),
		Filters:        models.QueryFilters{},
	}

	result, err := queryExecutor.Execute(ctx, query)
	require.NoError(t, err)

	// Should find the update event, and possibly the resource if it existed before the window
	foundUpdate := false
	for _, event := range result.Events {
		if event.Resource.UID == podUID {
			if event.Type == models.EventTypeUpdate {
				foundUpdate = true
			}
		}
	}

	assert.True(t, foundUpdate, "Should find the update event")
}

func TestQueryExecutor_EmptyResults(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	ctx := context.Background()
	client := harness.GetClient()

	// Query for events in a time range with no events
	queryExecutor := graph.NewQueryExecutor(client)
	query := &models.QueryRequest{
		StartTimestamp: time.Now().Add(-1 * time.Hour).Unix(),
		EndTimestamp:   time.Now().Unix(),
		Filters:        models.QueryFilters{},
	}

	result, err := queryExecutor.Execute(ctx, query)
	require.NoError(t, err)
	assert.Empty(t, result.Events, "Should return empty results for empty graph")
}

func TestQueryExecutor_MultipleEventsPerResource(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	ctx := context.Background()
	client := harness.GetClient()

	baseTime := time.Now().Add(-1 * time.Hour)
	podUID := uuid.New().String()

	// Create multiple events for the same resource
	events := []models.Event{
		CreatePodEvent(podUID, "test-pod", "default", baseTime.Add(10*time.Minute), models.EventTypeCreate, "Pending", nil),
		CreatePodEvent(podUID, "test-pod", "default", baseTime.Add(20*time.Minute), models.EventTypeUpdate, "Running", nil),
		CreatePodEvent(podUID, "test-pod", "default", baseTime.Add(30*time.Minute), models.EventTypeUpdate, "Ready", nil),
	}

	err = harness.SeedEvents(ctx, events)
	require.NoError(t, err)

	// Query for events
	queryExecutor := graph.NewQueryExecutor(client)
	query := &models.QueryRequest{
		StartTimestamp: baseTime.Unix(),
		EndTimestamp:   baseTime.Add(1 * time.Hour).Unix(),
		Filters:        models.QueryFilters{},
	}

	result, err := queryExecutor.Execute(ctx, query)
	require.NoError(t, err)

	// Count events for this resource
	podEventCount := 0
	for _, event := range result.Events {
		if event.Resource.UID == podUID {
			podEventCount++
		}
	}

	assert.GreaterOrEqual(t, podEventCount, 3, "Should return all events for the resource")
}
