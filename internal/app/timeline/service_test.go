package timeline

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel/trace/noop"
)

const kindEvent = "Event"

type mockQueryExecutor struct {
	executeFunc        func(*models.QueryRequest) (*models.QueryResult, error)
	executeCalls       int32
	concurrentExecutes int32
	maxConcurrent      int32
	queryDuration      time.Duration
}

func (m *mockQueryExecutor) Execute(ctx context.Context, q *models.QueryRequest) (*models.QueryResult, error) {
	concurrent := atomic.AddInt32(&m.concurrentExecutes, 1)
	defer atomic.AddInt32(&m.concurrentExecutes, -1)

	for {
		currentMax := atomic.LoadInt32(&m.maxConcurrent)
		if concurrent <= currentMax {
			break
		}
		if atomic.CompareAndSwapInt32(&m.maxConcurrent, currentMax, concurrent) {
			break
		}
	}

	atomic.AddInt32(&m.executeCalls, 1)

	if m.queryDuration > 0 {
		time.Sleep(m.queryDuration)
	}

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

func (m *mockQueryExecutor) SetSharedCache(cache interface{}) {}

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

func TestService_ExecuteConcurrentQueries_UsesBothQueriesConcurrently(t *testing.T) {
	logger := logging.GetLogger("test")
	tracer := noop.NewTracerProvider().Tracer("test")

	mockExecutor := &mockQueryExecutor{
		queryDuration: 50 * time.Millisecond,
		executeFunc: func(q *models.QueryRequest) (*models.QueryResult, error) {
			kinds := q.Filters.GetKinds()
			if len(kinds) == 1 && kinds[0] == kindEvent {
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

	service := NewService(mockExecutor, logger, tracer)
	query := &models.QueryRequest{
		StartTimestamp: time.Now().Add(-1 * time.Hour).Unix(),
		EndTimestamp:   time.Now().Unix(),
		Filters: models.QueryFilters{
			Kind:      "Pod",
			Namespace: "default",
		},
	}

	start := time.Now()
	resourceResult, eventResult, err := service.ExecuteConcurrentQueries(context.Background(), query)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resourceResult == nil || eventResult == nil {
		t.Fatalf("expected both query results, got resource=%v event=%v", resourceResult, eventResult)
	}
	if resourceResult.Count != 1 || eventResult.Count != 1 {
		t.Fatalf("expected one resource and one event, got resource=%d event=%d", resourceResult.Count, eventResult.Count)
	}
	if duration > 80*time.Millisecond {
		t.Fatalf("expected concurrent execution near 50ms, took %v", duration)
	}
}

func TestService_ParseQueryParameters_ParsesTimeRangeAndFilters(t *testing.T) {
	logger := logging.GetLogger("test")
	tracer := noop.NewTracerProvider().Tracer("test")
	service := NewService(&mockQueryExecutor{}, logger, tracer)

	query, err := service.ParseQueryParameters(context.Background(), "100", "200", map[string][]string{
		"kind":      {"Pod", "Deployment"},
		"namespace": {"default"},
		"group":     {"apps"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if query.StartTimestamp != 100 || query.EndTimestamp != 200 {
		t.Fatalf("unexpected timestamps: %#v", query)
	}
	if len(query.Filters.GetKinds()) != 2 {
		t.Fatalf("expected two kinds, got %#v", query.Filters.GetKinds())
	}
	if query.Filters.Group != "apps" {
		t.Fatalf("expected group apps, got %q", query.Filters.Group)
	}
}

func TestService_BuildTimelineResponse_AssociatesK8sEventsToResources(t *testing.T) {
	logger := logging.GetLogger("test")
	tracer := noop.NewTracerProvider().Tracer("test")
	service := NewService(&mockQueryExecutor{}, logger, tracer)

	resourceEvent := models.Event{
		ID:        "pod-change",
		Timestamp: 100,
		Type:      models.EventTypeUpdate,
		Resource: models.ResourceMetadata{
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "demo",
			UID:       "pod-uid",
		},
		Data: json.RawMessage(`{"status":{"conditions":[{"reason":"Ready"}]}}`),
	}
	k8sEvent := models.Event{
		ID:        "k8s-1",
		Timestamp: 150,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Version:           "v1",
			Kind:              "Event",
			Namespace:         "default",
			Name:              "demo.1",
			UID:               "event-uid",
			InvolvedObjectUID: "pod-uid",
		},
		Data: json.RawMessage(`{"reason":"Scheduled","message":"Assigned","type":"Normal"}`),
	}

	response := service.BuildTimelineResponse(&models.QueryResult{
		Events:          []models.Event{resourceEvent},
		Count:           1,
		ExecutionTimeMs: 5,
		QueryStartTime:  100,
		QueryEndTime:    200,
	}, &models.QueryResult{
		Events: []models.Event{k8sEvent},
		Count:  1,
	})

	if len(response.Resources) != 1 {
		t.Fatalf("expected one resource, got %d", len(response.Resources))
	}
	if len(response.Resources[0].Events) != 1 {
		t.Fatalf("expected one attached K8s event, got %d", len(response.Resources[0].Events))
	}
	if response.Resources[0].Events[0].Reason != "Scheduled" {
		t.Fatalf("expected reason Scheduled, got %q", response.Resources[0].Events[0].Reason)
	}
}

func TestService_ExecuteConcurrentQueries_ResourceQueryFailureWins(t *testing.T) {
	logger := logging.GetLogger("test")
	tracer := noop.NewTracerProvider().Tracer("test")
	resourceErr := errors.New("resource query failed")

	mockExecutor := &mockQueryExecutor{
		executeFunc: func(q *models.QueryRequest) (*models.QueryResult, error) {
			kinds := q.Filters.GetKinds()
			if len(kinds) == 1 && kinds[0] == kindEvent {
				return &models.QueryResult{Events: []models.Event{}}, nil
			}
			return nil, resourceErr
		},
	}

	service := NewService(mockExecutor, logger, tracer)
	_, _, err := service.ExecuteConcurrentQueries(context.Background(), &models.QueryRequest{
		StartTimestamp: 1,
		EndTimestamp:   2,
		Filters: models.QueryFilters{
			Kind: "Pod",
		},
	})
	if !errors.Is(err, resourceErr) {
		t.Fatalf("expected resource error, got %v", err)
	}
}
