package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel/trace/noop"
)

// mockConcurrentQueryExecutor allows controlling behavior for concurrent tests
type mockConcurrentQueryExecutor struct {
	executeFunc        func(*models.QueryRequest) (*models.QueryResult, error)
	executeCalls       int32
	concurrentExecutes int32
	maxConcurrent      int32
	queryDuration      time.Duration
}

func (m *mockConcurrentQueryExecutor) Execute(ctx context.Context, q *models.QueryRequest) (*models.QueryResult, error) {
	// Track concurrent executions
	concurrent := atomic.AddInt32(&m.concurrentExecutes, 1)
	defer atomic.AddInt32(&m.concurrentExecutes, -1)

	// Track max concurrent
	for {
		currentMax := atomic.LoadInt32(&m.maxConcurrent)
		if concurrent <= currentMax {
			break
		}
		if atomic.CompareAndSwapInt32(&m.maxConcurrent, currentMax, concurrent) {
			break
		}
	}

	// Track total calls
	atomic.AddInt32(&m.executeCalls, 1)

	// Simulate query duration
	if m.queryDuration > 0 {
		time.Sleep(m.queryDuration)
	}

	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if m.executeFunc != nil {
		return m.executeFunc(q)
	}

	return &models.QueryResult{
		Events:          []models.Event{},
		Count:           0,
		ExecutionTimeMs: 10,
	}, nil
}

func (m *mockConcurrentQueryExecutor) GetExecuteCalls() int32 {
	return atomic.LoadInt32(&m.executeCalls)
}

func (m *mockConcurrentQueryExecutor) GetMaxConcurrent() int32 {
	return atomic.LoadInt32(&m.maxConcurrent)
}

func (m *mockConcurrentQueryExecutor) SetSharedCache(cache interface{}) {
	// Mock doesn't need to implement caching
}

// createTestEvent is a helper function to create test events
func createTestEvent(id, kind, namespace, name string, timestamp int64) models.Event {
	return models.Event{
		ID:        id,
		Timestamp: timestamp,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      kind,
			Namespace: namespace,
			Name:      name,
			UID:       "uid-" + id,
		},
		Data:     json.RawMessage(`{"kind":"` + kind + `"}`),
		DataSize: 100,
	}
}

// createTestEventWithGroup creates a test event with a resource group
func createTestEventWithGroup(id, group, kind, namespace, name string, timestamp int64) models.Event {
	evt := createTestEvent(id, kind, namespace, name, timestamp)
	evt.Resource.Group = group
	return evt
}

// TestExecuteConcurrentQueries_BothQueriesSucceed tests the happy path
func TestExecuteConcurrentQueries_BothQueriesSucceed(t *testing.T) {
	logger := logging.GetLogger("test")
	tracer := noop.NewTracerProvider().Tracer("test")

	mockExecutor := &mockConcurrentQueryExecutor{
		queryDuration: 50 * time.Millisecond,
		executeFunc: func(q *models.QueryRequest) (*models.QueryResult, error) {
			// Return different results based on query kind
			kinds := q.Filters.GetKinds()
			if len(kinds) == 1 && kinds[0] == "Event" {
				return &models.QueryResult{
					Events: []models.Event{
						createTestEvent("event-1", "Event", "default", "test-event", time.Now().UnixNano()),
					},
					Count:           1,
					ExecutionTimeMs: 50,
				}, nil
			}
			return &models.QueryResult{
				Events: []models.Event{
					createTestEvent("pod-1", "Pod", "default", "test-pod", time.Now().UnixNano()),
				},
				Count:           1,
				ExecutionTimeMs: 50,
			}, nil
		},
	}

	handler := NewTimelineHandler(mockExecutor, logger, tracer)

	query := &models.QueryRequest{
		StartTimestamp: time.Now().Add(-1 * time.Hour).Unix(),
		EndTimestamp:   time.Now().Unix(),
		Filters: models.QueryFilters{
			Kind:      "Pod",
			Namespace: "default",
		},
	}

	start := time.Now()
	resourceResult, eventResult, err := handler.executeConcurrentQueries(context.Background(), query)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if resourceResult == nil {
		t.Fatal("Expected resource result, got nil")
	}

	if eventResult == nil {
		t.Fatal("Expected event result, got nil")
	}

	if resourceResult.Count != 1 {
		t.Errorf("Expected 1 resource, got %d", resourceResult.Count)
	}

	if eventResult.Count != 1 {
		t.Errorf("Expected 1 event, got %d", eventResult.Count)
	}

	// Verify concurrent execution (should take ~50ms, not 100ms)
	if duration > 80*time.Millisecond {
		t.Errorf("Expected concurrent execution (~50ms), took %v", duration)
	}

	// Verify both queries were executed
	if mockExecutor.GetExecuteCalls() != 2 {
		t.Errorf("Expected 2 execute calls, got %d", mockExecutor.GetExecuteCalls())
	}

	// Verify queries ran concurrently
	if mockExecutor.GetMaxConcurrent() != 2 {
		t.Errorf("Expected max concurrent executions of 2, got %d", mockExecutor.GetMaxConcurrent())
	}
}

// TestExecuteConcurrentQueries_ResourceQueryFails tests resource query failure
func TestExecuteConcurrentQueries_ResourceQueryFails(t *testing.T) {
	logger := logging.GetLogger("test")
	tracer := noop.NewTracerProvider().Tracer("test")

	resourceErr := errors.New("resource query failed")
	mockExecutor := &mockConcurrentQueryExecutor{
		executeFunc: func(q *models.QueryRequest) (*models.QueryResult, error) {
			// Check if this is an Event query (uses Kinds slice)
			kinds := q.Filters.GetKinds()
			if len(kinds) == 1 && kinds[0] == "Event" {
				return &models.QueryResult{
					Events:          []models.Event{},
					Count:           0,
					ExecutionTimeMs: 10,
				}, nil
			}
			return nil, resourceErr
		},
	}

	handler := NewTimelineHandler(mockExecutor, logger, tracer)

	query := &models.QueryRequest{
		StartTimestamp: time.Now().Add(-1 * time.Hour).Unix(),
		EndTimestamp:   time.Now().Unix(),
		Filters: models.QueryFilters{
			Kind:      "Pod",
			Namespace: "default",
		},
	}

	resourceResult, eventResult, err := handler.executeConcurrentQueries(context.Background(), query)

	if !errors.Is(err, resourceErr) && err.Error() != resourceErr.Error() {
		t.Fatalf("Expected resource error, got: %v", err)
	}

	if resourceResult != nil {
		t.Error("Expected nil resource result on error")
	}

	if eventResult != nil {
		t.Error("Expected nil event result on error")
	}
}

// TestExecuteConcurrentQueries_EventQueryFails tests graceful degradation
func TestExecuteConcurrentQueries_EventQueryFails(t *testing.T) {
	logger := logging.GetLogger("test")
	tracer := noop.NewTracerProvider().Tracer("test")

	eventErr := errors.New("event query failed")
	mockExecutor := &mockConcurrentQueryExecutor{
		executeFunc: func(q *models.QueryRequest) (*models.QueryResult, error) {
			// Check if this is an Event query (uses Kinds slice)
			kinds := q.Filters.GetKinds()
			if len(kinds) == 1 && kinds[0] == "Event" {
				return nil, eventErr
			}
			return &models.QueryResult{
				Events: []models.Event{
					createTestEvent("pod-1", "Pod", "default", "test-pod", time.Now().UnixNano()),
				},
				Count:           1,
				ExecutionTimeMs: 10,
			}, nil
		},
	}

	handler := NewTimelineHandler(mockExecutor, logger, tracer)

	query := &models.QueryRequest{
		StartTimestamp: time.Now().Add(-1 * time.Hour).Unix(),
		EndTimestamp:   time.Now().Unix(),
		Filters: models.QueryFilters{
			Kind:      "Pod",
			Namespace: "default",
		},
	}

	resourceResult, eventResult, err := handler.executeConcurrentQueries(context.Background(), query)

	// Should succeed with empty event result (graceful degradation)
	if err != nil {
		t.Fatalf("Expected no error on event query failure, got: %v", err)
	}

	if resourceResult == nil {
		t.Fatal("Expected resource result, got nil")
	}

	if resourceResult.Count != 1 {
		t.Errorf("Expected 1 resource, got %d", resourceResult.Count)
	}

	if eventResult == nil {
		t.Fatal("Expected empty event result, got nil")
	}

	if eventResult.Count != 0 {
		t.Errorf("Expected event count 0 on failure, got %d", eventResult.Count)
	}

	if len(eventResult.Events) != 0 {
		t.Errorf("Expected 0 events on failure, got %d", len(eventResult.Events))
	}
}

// TestExecuteConcurrentQueries_ContextCancellation tests context cancellation
func TestExecuteConcurrentQueries_ContextCancellation(t *testing.T) {
	logger := logging.GetLogger("test")
	tracer := noop.NewTracerProvider().Tracer("test")

	mockExecutor := &mockConcurrentQueryExecutor{
		queryDuration: 200 * time.Millisecond, // Long duration to allow cancellation
	}

	handler := NewTimelineHandler(mockExecutor, logger, tracer)

	query := &models.QueryRequest{
		StartTimestamp: time.Now().Add(-1 * time.Hour).Unix(),
		EndTimestamp:   time.Now().Unix(),
		Filters: models.QueryFilters{
			Kind:      "Pod",
			Namespace: "default",
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	
	// Cancel after 50ms
	time.AfterFunc(50*time.Millisecond, cancel)

	resourceResult, eventResult, err := handler.executeConcurrentQueries(ctx, query)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}

	if resourceResult != nil {
		t.Error("Expected nil resource result on cancellation")
	}

	if eventResult != nil {
		t.Error("Expected nil event result on cancellation")
	}
}

// TestExecuteConcurrentQueries_EmptyResults tests empty result handling
func TestExecuteConcurrentQueries_EmptyResults(t *testing.T) {
	logger := logging.GetLogger("test")
	tracer := noop.NewTracerProvider().Tracer("test")

	mockExecutor := &mockConcurrentQueryExecutor{
		executeFunc: func(q *models.QueryRequest) (*models.QueryResult, error) {
			return &models.QueryResult{
				Events:          []models.Event{},
				Count:           0,
				ExecutionTimeMs: 10,
			}, nil
		},
	}

	handler := NewTimelineHandler(mockExecutor, logger, tracer)

	query := &models.QueryRequest{
		StartTimestamp: time.Now().Add(-1 * time.Hour).Unix(),
		EndTimestamp:   time.Now().Unix(),
		Filters: models.QueryFilters{
			Kind:      "Pod",
			Namespace: "default",
		},
	}

	resourceResult, eventResult, err := handler.executeConcurrentQueries(context.Background(), query)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if resourceResult == nil {
		t.Fatal("Expected resource result, got nil")
	}

	if eventResult == nil {
		t.Fatal("Expected event result, got nil")
	}

	if resourceResult.Count != 0 {
		t.Errorf("Expected 0 resources, got %d", resourceResult.Count)
	}

	if eventResult.Count != 0 {
		t.Errorf("Expected 0 events, got %d", eventResult.Count)
	}
}

// TestExecuteConcurrentQueries_ConcurrentSafety tests multiple concurrent calls
func TestExecuteConcurrentQueries_ConcurrentSafety(t *testing.T) {
	logger := logging.GetLogger("test")
	tracer := noop.NewTracerProvider().Tracer("test")

	mockExecutor := &mockConcurrentQueryExecutor{
		queryDuration: 10 * time.Millisecond,
		executeFunc: func(q *models.QueryRequest) (*models.QueryResult, error) {
			return &models.QueryResult{
				Events:          []models.Event{},
				Count:           0,
				ExecutionTimeMs: 10,
			}, nil
		},
	}

	handler := NewTimelineHandler(mockExecutor, logger, tracer)

	query := &models.QueryRequest{
		StartTimestamp: time.Now().Add(-1 * time.Hour).Unix(),
		EndTimestamp:   time.Now().Unix(),
		Filters: models.QueryFilters{
			Kind:      "Pod",
			Namespace: "default",
		},
	}

	// Run multiple concurrent timeline requests
	var wg sync.WaitGroup
	concurrentRequests := 10
	errors := make([]error, concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _, err := handler.executeConcurrentQueries(context.Background(), query)
			errors[idx] = err
		}(i)
	}

	wg.Wait()

	// Verify all requests succeeded
	for i, err := range errors {
		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
		}
	}

	// Verify total execute calls (10 timeline requests * 2 queries each = 20)
	expectedCalls := int32(concurrentRequests * 2)
	if mockExecutor.GetExecuteCalls() != expectedCalls {
		t.Errorf("Expected %d execute calls, got %d", expectedCalls, mockExecutor.GetExecuteCalls())
	}
}

// TestBuildTimelineResponse_WithEvents tests event attachment
func TestBuildTimelineResponse_WithEvents(t *testing.T) {
	logger := logging.GetLogger("test")
	tracer := noop.NewTracerProvider().Tracer("test")

	handler := NewTimelineHandler(nil, logger, tracer)

	now := time.Now()
	podUID := "pod-uid-123"

	resourceResult := &models.QueryResult{
		Events: []models.Event{
			{
				ID:        "1",
				Timestamp: now.UnixNano(),
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					Kind:      "Pod",
					Namespace: "default",
					Name:      "test-pod",
					UID:       podUID,
				},
			},
		},
		Count:           1,
		ExecutionTimeMs: 10,
	}

	eventResult := &models.QueryResult{
		Events: []models.Event{
			{
				ID:        "event-1",
				Timestamp: now.UnixNano(),
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					Kind:              "Event",
					Namespace:         "default",
					Name:              "test-event",
					InvolvedObjectUID: podUID,
				},
				Data:     []byte(`{"reason":"Created","message":"Pod created","type":"Normal"}`),
				DataSize: 100,
			},
		},
		Count:           1,
		ExecutionTimeMs: 5,
	}

	response := handler.buildTimelineResponse(resourceResult, eventResult)

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	if response.Count != 1 {
		t.Errorf("Expected 1 resource, got %d", response.Count)
	}

	if len(response.Resources) != 1 {
		t.Fatalf("Expected 1 resource in array, got %d", len(response.Resources))
	}

	// Verify event was attached (checking if Events array exists and has items)
	resource := response.Resources[0]
	if len(resource.Events) == 0 {
		t.Error("Expected K8s events to be attached to resource")
	}
}

// TestBuildTimelineResponse_WithoutEvents tests response without events
func TestBuildTimelineResponse_WithoutEvents(t *testing.T) {
	logger := logging.GetLogger("test")
	tracer := noop.NewTracerProvider().Tracer("test")

	handler := NewTimelineHandler(nil, logger, tracer)

	now := time.Now()

	resourceResult := &models.QueryResult{
		Events: []models.Event{
			{
				ID:        "1",
				Timestamp: now.UnixNano(),
				Type:      models.EventTypeCreate,
				Resource: models.ResourceMetadata{
					Kind:      "Pod",
					Namespace: "default",
					Name:      "test-pod",
					UID:       "pod-uid-123",
				},
			},
		},
		Count:           1,
		ExecutionTimeMs: 10,
	}

	eventResult := &models.QueryResult{
		Events:          []models.Event{},
		Count:           0,
		ExecutionTimeMs: 5,
	}

	response := handler.buildTimelineResponse(resourceResult, eventResult)

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	if response.Count != 1 {
		t.Errorf("Expected 1 resource, got %d", response.Count)
	}

	if len(response.Resources) != 1 {
		t.Fatalf("Expected 1 resource in array, got %d", len(response.Resources))
	}
}
