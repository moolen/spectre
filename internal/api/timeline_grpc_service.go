package api

import (
	"fmt"

	"github.com/moolen/spectre/internal/api/pb"
	apptimeline "github.com/moolen/spectre/internal/app/timeline"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TimelineGRPCService implements the gRPC TimelineService
// It wraps the unified TimelineService with gRPC-compatible streaming
type TimelineGRPCService struct {
	pb.UnimplementedTimelineServiceServer
	service *apptimeline.Service
}

// NewTimelineGRPCService creates a new timeline gRPC service with storage executor only
func NewTimelineGRPCService(queryExecutor QueryExecutor, logger *logging.Logger, tracer trace.Tracer) *TimelineGRPCService {
	return &TimelineGRPCService{
		service: apptimeline.NewService(queryExecutor, logger, tracer),
	}
}

// NewTimelineGRPCServiceWithMode creates a new timeline gRPC service with both executors
func NewTimelineGRPCServiceWithMode(storageExecutor, graphExecutor QueryExecutor, querySource TimelineQuerySource, logger *logging.Logger, tracer trace.Tracer) *TimelineGRPCService {
	return &TimelineGRPCService{
		service: apptimeline.NewServiceWithMode(storageExecutor, graphExecutor, toAppQuerySource(querySource), logger, tracer),
	}
}

// GetTimeline implements the gRPC streaming endpoint
func (s *TimelineGRPCService) GetTimeline(req *pb.TimelineRequest, stream pb.TimelineService_GetTimelineServer) error {
	ctx := stream.Context()

	// Start tracing span
	ctx, span := s.service.Tracer().Start(ctx, "grpc.GetTimeline",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.Int64("query.start_timestamp", req.StartTimestamp),
			attribute.Int64("query.end_timestamp", req.EndTimestamp),
			attribute.String("query.namespace", req.Namespace),
			attribute.String("query.kind", req.Kind),
		),
	)
	defer span.End()

	// Convert proto request to internal query request
	query, pagination, err := s.protoToQueryRequest(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid request")
		s.service.Logger().Warn("Invalid gRPC request: %v (start=%d, end=%d, namespace=%q, kind=%q)",
			err, req.StartTimestamp, req.EndTimestamp, req.Namespace, req.Kind)
		// Return proper gRPC error status
		return fmt.Errorf("invalid request: %w", err)
	}

	result, err := s.service.ExecuteTimeline(ctx, query, pagination)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Query execution failed")
		s.service.Logger().Error("gRPC query execution failed: %v (start=%d, end=%d, namespace=%q, kind=%q)",
			err, query.StartTimestamp, query.EndTimestamp, query.Filters.Namespace, query.Filters.Kind)
		// Return proper gRPC error status
		return fmt.Errorf("query execution failed: %w", err)
	}

	// Log query results for debugging
	s.service.Logger().Debug("gRPC query completed: resources=%d, events=%d", result.ResourceResult.Count, result.EventResult.Count)

	span.SetAttributes(
		attribute.Int("result.resource_count", result.Index.Count()),
		attribute.Int64("result.execution_time_ms", result.Index.ExecutionTimeMs()),
	)

	// Stream metadata first
	err = s.sendMetadata(stream, result.ResourceResult, len(result.Entries), result.Pagination)
	if err != nil {
		span.RecordError(err)
		s.service.Logger().Error("Failed to send metadata: %v", err)
		return err
	}

	groupedEntries := groupAndSortTimelineEntries(result.Entries)

	// Stream resources in batches
	// If no resources, send an empty batch to signal completion
	if len(groupedEntries) == 0 {
		emptyBatch := &pb.TimelineChunk{
			ChunkType: &pb.TimelineChunk_Batch{
				Batch: &pb.ResourceBatch{
					Kind:         "",
					Resources:    []*pb.TimelineResource{},
					IsFinalBatch: true,
				},
			},
		}
		if err := stream.Send(emptyBatch); err != nil {
			span.RecordError(err)
			s.service.Logger().Error("Failed to send empty batch: %v", err)
			return err
		}
	} else {
		err = s.streamEntryBatches(stream, result.Index, groupedEntries)
		if err != nil {
			span.RecordError(err)
			s.service.Logger().Error("Failed to stream resources: %v", err)
			return err
		}
	}

	span.SetStatus(codes.Ok, "Streaming completed successfully")
	s.service.Logger().Debug("gRPC streaming completed: %d resources in %d groups", len(result.Entries), len(groupedEntries))

	return nil
}

// sendMetadata sends the metadata chunk with count, query stats, and pagination info.
func (s *TimelineGRPCService) sendMetadata(stream pb.TimelineService_GetTimelineServer, result *models.QueryResult, totalCount int, pagination *models.PaginationResponse) error {
	chunk := &pb.TimelineChunk{
		ChunkType: &pb.TimelineChunk_Metadata{
			Metadata: buildTimelineMetadata(result, totalCount, pagination),
		},
	}

	return stream.Send(chunk)
}

func (s *TimelineGRPCService) streamEntryBatches(stream pb.TimelineService_GetTimelineServer, index *apptimeline.TimelineIndex, groups []*GroupedTimelineEntries) error {
	return streamTimelineEntryBatches(
		index,
		groups,
		s.service.BuildTimelineResources,
		func(res *models.Resource) *pb.TimelineResource { return ResourceToProto(s.service, res) },
		stream.Send,
	)
}

// protoToQueryRequest converts protobuf request to internal query and pagination requests.
func (s *TimelineGRPCService) protoToQueryRequest(req *pb.TimelineRequest) (*models.QueryRequest, *models.PaginationRequest, error) {
	return parseTimelineProtoRequest(s.service, req)
}

// resourceToProto converts internal Resource model to protobuf TimelineResource
