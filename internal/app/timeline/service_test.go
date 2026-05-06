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

type mockPaginatedQueryExecutor struct {
	mockQueryExecutor
	executePaginatedFunc  func(*models.QueryRequest, *models.PaginationRequest) (*models.QueryResult, *models.PaginationResponse, error)
	executePaginatedCalls int32
}

func (m *mockPaginatedQueryExecutor) ExecutePaginated(_ context.Context, q *models.QueryRequest, p *models.PaginationRequest) (*models.QueryResult, *models.PaginationResponse, error) {
	atomic.AddInt32(&m.executePaginatedCalls, 1)
	if m.executePaginatedFunc != nil {
		return m.executePaginatedFunc(q, p)
	}
	return &models.QueryResult{}, &models.PaginationResponse{}, nil
}

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

func TestService_ExecuteTimeline_UsesExecutorPaginationWhenAvailable(t *testing.T) {
	logger := logging.GetLogger("test")
	tracer := noop.NewTracerProvider().Tracer("test")

	executor := &mockPaginatedQueryExecutor{
		mockQueryExecutor: mockQueryExecutor{
			executeFunc: func(q *models.QueryRequest) (*models.QueryResult, error) {
				if kinds := q.Filters.GetKinds(); len(kinds) == 1 && kinds[0] == kindEvent {
					return &models.QueryResult{
						Events: []models.Event{
							{
								ID:        "k8s-1",
								Timestamp: 120,
								Type:      models.EventTypeCreate,
								Resource: models.ResourceMetadata{
									Version:           "v1",
									Kind:              "Event",
									Namespace:         "default",
									Name:              "pod-a.1",
									UID:               "event-1",
									InvolvedObjectUID: "pod-a-uid",
								},
								Data: json.RawMessage(`{"reason":"Scheduled","message":"Assigned","type":"Normal"}`),
							},
						},
						Count: 1,
					}, nil
				}
				return &models.QueryResult{}, nil
			},
		},
		executePaginatedFunc: func(q *models.QueryRequest, p *models.PaginationRequest) (*models.QueryResult, *models.PaginationResponse, error) {
			if p.GetPageSize() != 1 {
				t.Fatalf("expected page size 1, got %d", p.GetPageSize())
			}
			return &models.QueryResult{
					Events: []models.Event{
						{
							ID:        "pod-a-create",
							Timestamp: 100,
							Type:      models.EventTypeCreate,
							Resource: models.ResourceMetadata{
								Version:   "v1",
								Kind:      "Pod",
								Namespace: "default",
								Name:      "pod-a",
								UID:       "pod-a-uid",
							},
							Data: json.RawMessage(`{"status":{"phase":"Running"}}`),
						},
					},
					Count:           1,
					ExecutionTimeMs: 7,
					QueryStartTime:  100,
					QueryEndTime:    200,
				},
				&models.PaginationResponse{
					HasMore:    true,
					NextCursor: "cursor-1",
					PageSize:   1,
				},
				nil
		},
	}

	service := NewService(executor, logger, tracer)
	result, err := service.ExecuteTimeline(context.Background(), &models.QueryRequest{
		StartTimestamp: 100,
		EndTimestamp:   200,
		Filters: models.QueryFilters{
			Kinds:      []string{"Pod"},
			Namespaces: []string{"default"},
		},
	}, &models.PaginationRequest{PageSize: 1})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if got := atomic.LoadInt32(&executor.executePaginatedCalls); got != 1 {
		t.Fatalf("expected ExecutePaginated to be called once, got %d", got)
	}
	if got := atomic.LoadInt32(&executor.executeCalls); got != 1 {
		t.Fatalf("expected Execute to be called once for event query, got %d", got)
	}
	if result.Pagination == nil || !result.Pagination.HasMore || result.Pagination.NextCursor != "cursor-1" {
		t.Fatalf("expected pagination response from executor, got %#v", result.Pagination)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("expected 1 selected entry, got %d", len(result.Entries))
	}
	if result.Entries[0].Name != "pod-a" {
		t.Fatalf("expected selected pod-a entry, got %q", result.Entries[0].Name)
	}
}

func TestService_ExecuteTimeline_FallsBackToClientPagination(t *testing.T) {
	logger := logging.GetLogger("test")
	tracer := noop.NewTracerProvider().Tracer("test")

	executor := &mockQueryExecutor{
		executeFunc: func(q *models.QueryRequest) (*models.QueryResult, error) {
			if kinds := q.Filters.GetKinds(); len(kinds) == 1 && kinds[0] == kindEvent {
				return &models.QueryResult{Events: []models.Event{}}, nil
			}

			return &models.QueryResult{
				Events: []models.Event{
					createTestEvent("pod-c", "Pod", "zeta", "pod-c", 103),
					createTestEvent("deploy-a", "Deployment", "alpha", "deploy-a", 101),
					createTestEvent("pod-a", "Pod", "alpha", "pod-a", 102),
				},
				Count:           3,
				ExecutionTimeMs: 11,
				QueryStartTime:  100,
				QueryEndTime:    200,
			}, nil
		},
	}

	service := NewService(executor, logger, tracer)
	result, err := service.ExecuteTimeline(context.Background(), &models.QueryRequest{
		StartTimestamp: 100,
		EndTimestamp:   200,
		Filters:        models.QueryFilters{Kinds: []string{"Pod"}},
	}, &models.PaginationRequest{PageSize: 2})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.Pagination == nil || !result.Pagination.HasMore || result.Pagination.NextCursor == "" {
		t.Fatalf("expected client pagination response with next cursor, got %#v", result.Pagination)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 selected entries, got %d", len(result.Entries))
	}
	if result.Entries[0].Kind != "Deployment" || result.Entries[0].Namespace != "alpha" || result.Entries[0].Name != "deploy-a" {
		t.Fatalf("unexpected first paginated entry: %#v", result.Entries[0])
	}
	if result.Entries[1].Kind != "Pod" || result.Entries[1].Namespace != "alpha" || result.Entries[1].Name != "pod-a" {
		t.Fatalf("unexpected second paginated entry: %#v", result.Entries[1])
	}
}

func TestService_PaginateEntries_SortsAndSlicesByCursor(t *testing.T) {
	logger := logging.GetLogger("test")
	tracer := noop.NewTracerProvider().Tracer("test")
	service := NewService(&mockQueryExecutor{}, logger, tracer)

	entries := []*TimelineResourceEntry{
		{Kind: "Pod", Namespace: "zeta", Name: "pod-c"},
		{Kind: "Deployment", Namespace: "alpha", Name: "deploy-a"},
		{Kind: "Pod", Namespace: "alpha", Name: "pod-a"},
	}

	page1, page1Resp, err := service.paginateEntries(entries, &models.PaginationRequest{PageSize: 2})
	if err != nil {
		t.Fatalf("expected no error paginating first page, got %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("expected 2 entries on first page, got %d", len(page1))
	}
	if page1[0].Kind != "Deployment" || page1[0].Namespace != "alpha" || page1[0].Name != "deploy-a" {
		t.Fatalf("unexpected first entry on page 1: %#v", page1[0])
	}
	if page1[1].Kind != "Pod" || page1[1].Namespace != "alpha" || page1[1].Name != "pod-a" {
		t.Fatalf("unexpected second entry on page 1: %#v", page1[1])
	}
	if !page1Resp.HasMore {
		t.Fatal("expected first page to report more results")
	}

	page2, page2Resp, err := service.paginateEntries(entries, &models.PaginationRequest{
		PageSize: 2,
		Cursor:   page1Resp.NextCursor,
	})
	if err != nil {
		t.Fatalf("expected no error paginating second page, got %v", err)
	}
	if len(page2) != 1 {
		t.Fatalf("expected 1 entry on second page, got %d", len(page2))
	}
	if page2[0].Kind != "Pod" || page2[0].Namespace != "zeta" || page2[0].Name != "pod-c" {
		t.Fatalf("unexpected entry on page 2: %#v", page2[0])
	}
	if page2Resp.HasMore {
		t.Fatal("expected second page to be terminal")
	}
}
