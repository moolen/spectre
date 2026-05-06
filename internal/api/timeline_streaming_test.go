package api

import (
	"context"
	"testing"

	"github.com/moolen/spectre/internal/api/pb"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel/trace/noop"
)

type testStreamingQueryExecutor struct{}

func (testStreamingQueryExecutor) Execute(context.Context, *models.QueryRequest) (*models.QueryResult, error) {
	return &models.QueryResult{}, nil
}

func (testStreamingQueryExecutor) SetSharedCache(interface{}) {}

func TestProtoToQueryRequest_BuildsPaginationOnlyWhenRequested(t *testing.T) {
	logger := logging.GetLogger("test")
	tracer := noop.NewTracerProvider().Tracer("test")
	service := NewTimelineConnectService(testStreamingQueryExecutor{}, logger, tracer)

	query, pagination, err := service.protoToQueryRequest(&pb.TimelineRequest{
		StartTimestamp: 100,
		EndTimestamp:   200,
		Kinds:          []string{"Pod"},
		Namespaces:     []string{"default"},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if query.StartTimestamp != 100 || query.EndTimestamp != 200 {
		t.Fatalf("unexpected query timestamps: %#v", query)
	}
	if pagination != nil {
		t.Fatalf("expected nil pagination when page size and cursor are empty, got %#v", pagination)
	}
}

func TestProtoToQueryRequest_PreservesExplicitPagination(t *testing.T) {
	logger := logging.GetLogger("test")
	tracer := noop.NewTracerProvider().Tracer("test")
	service := NewTimelineConnectService(testStreamingQueryExecutor{}, logger, tracer)

	_, pagination, err := service.protoToQueryRequest(&pb.TimelineRequest{
		StartTimestamp: 100,
		EndTimestamp:   200,
		Kinds:          []string{"Pod"},
		PageSize:       2,
		Cursor:         "cursor-1",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if pagination == nil {
		t.Fatal("expected pagination request to be created")
	}
	if pagination.PageSize != 2 || pagination.Cursor != "cursor-1" {
		t.Fatalf("unexpected pagination request: %#v", pagination)
	}
}
