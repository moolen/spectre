package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moolen/spectre/internal/api/pb"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc/metadata"
)

type grpcTestPaginatedExecutor struct {
	executeCalls          int
	executePaginatedCalls int
}

func (e *grpcTestPaginatedExecutor) Execute(_ context.Context, q *models.QueryRequest) (*models.QueryResult, error) {
	e.executeCalls++
	if kinds := q.Filters.GetKinds(); len(kinds) == 1 && kinds[0] == "Event" {
		return &models.QueryResult{Events: []models.Event{}}, nil
	}
	return &models.QueryResult{}, nil
}

func (e *grpcTestPaginatedExecutor) ExecutePaginated(_ context.Context, _ *models.QueryRequest, p *models.PaginationRequest) (*models.QueryResult, *models.PaginationResponse, error) {
	e.executePaginatedCalls++
	if p.GetPageSize() != 1 {
		return nil, nil, nil
	}
	return &models.QueryResult{
			Events: []models.Event{
				{
					ID:        "deploy-a-create",
					Timestamp: 101,
					Type:      models.EventTypeCreate,
					Resource: models.ResourceMetadata{
						Version:   "v1",
						Kind:      "Deployment",
						Namespace: "alpha",
						Name:      "deploy-a",
						UID:       "deploy-a-uid",
					},
					Data: json.RawMessage(`{"status":{"conditions":[{"reason":"Ready"}]}}`),
				},
			},
			Count:           1,
			ExecutionTimeMs: 9,
			QueryStartTime:  100,
			QueryEndTime:    200,
		},
		&models.PaginationResponse{
			HasMore:    true,
			NextCursor: "cursor-1",
			PageSize:   1,
		},
		nil
}

func (e *grpcTestPaginatedExecutor) SetSharedCache(interface{}) {}

type grpcTestStream struct {
	ctx    context.Context
	chunks []*pb.TimelineChunk
}

func (s *grpcTestStream) Send(chunk *pb.TimelineChunk) error {
	s.chunks = append(s.chunks, chunk)
	return nil
}

func (s *grpcTestStream) SetHeader(metadata.MD) error  { return nil }
func (s *grpcTestStream) SendHeader(metadata.MD) error { return nil }
func (s *grpcTestStream) SetTrailer(metadata.MD)       {}
func (s *grpcTestStream) Context() context.Context     { return s.ctx }
func (s *grpcTestStream) SendMsg(any) error            { return nil }
func (s *grpcTestStream) RecvMsg(any) error            { return nil }

func TestTimelineGRPCService_GetTimeline_UsesAppPaginationSeam(t *testing.T) {
	logger := logging.GetLogger("test")
	tracer := noop.NewTracerProvider().Tracer("test")
	executor := &grpcTestPaginatedExecutor{}
	service := NewTimelineGRPCService(executor, logger, tracer)
	stream := &grpcTestStream{ctx: context.Background()}

	err := service.GetTimeline(&pb.TimelineRequest{
		StartTimestamp: 100,
		EndTimestamp:   200,
		Kinds:          []string{"Pod", "Deployment"},
		Namespaces:     []string{"alpha", "zeta"},
		PageSize:       1,
	}, stream)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if executor.executePaginatedCalls != 1 {
		t.Fatalf("expected ExecutePaginated to be called once, got %d", executor.executePaginatedCalls)
	}
	if executor.executeCalls != 1 {
		t.Fatalf("expected Execute to be called once for event query, got %d", executor.executeCalls)
	}
	if len(stream.chunks) != 2 {
		t.Fatalf("expected metadata plus final batch, got %d chunks", len(stream.chunks))
	}

	metadataChunk := stream.chunks[0].GetMetadata()
	if metadataChunk == nil {
		t.Fatal("expected first chunk to be metadata")
	}
	if metadataChunk.TotalCount != 1 {
		t.Fatalf("expected total count 1, got %d", metadataChunk.TotalCount)
	}
	if !metadataChunk.HasMore || metadataChunk.NextCursor != "cursor-1" || metadataChunk.PageSize != 1 {
		t.Fatalf("unexpected pagination metadata: %#v", metadataChunk)
	}

	batch := stream.chunks[1].GetBatch()
	if batch == nil {
		t.Fatal("expected second chunk to be a batch")
	}
	if len(batch.Resources) != 1 || batch.Resources[0].Name != "deploy-a" {
		t.Fatalf("unexpected streamed resources: %#v", batch.Resources)
	}
	if !batch.IsFinalBatch {
		t.Fatal("expected streamed batch to be final")
	}
}
