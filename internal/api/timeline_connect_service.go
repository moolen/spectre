package api

import (
	"context"

	"connectrpc.com/connect"
	"github.com/moolen/spectre/internal/api/pb"
	"github.com/moolen/spectre/internal/api/pb/pbconnect"
	apptimeline "github.com/moolen/spectre/internal/app/timeline"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TimelineConnectService implements the Connect TimelineService interface
// It wraps the unified TimelineService with Connect-compatible streaming
type TimelineConnectService struct {
	pbconnect.UnimplementedTimelineServiceHandler
	service *apptimeline.Service
}

// NewTimelineConnectService creates a new timeline Connect service with storage executor only
func NewTimelineConnectService(queryExecutor QueryExecutor, logger *logging.Logger, tracer trace.Tracer) *TimelineConnectService {
	return &TimelineConnectService{
		service: apptimeline.NewService(queryExecutor, logger, tracer),
	}
}

// NewTimelineConnectServiceWithMode creates a new timeline Connect service with both executors
func NewTimelineConnectServiceWithMode(storageExecutor, graphExecutor QueryExecutor, querySource TimelineQuerySource, logger *logging.Logger, tracer trace.Tracer) *TimelineConnectService {
	return &TimelineConnectService{
		service: apptimeline.NewServiceWithMode(storageExecutor, graphExecutor, toAppQuerySource(querySource), logger, tracer),
	}
}

// GetTimeline implements the Connect streaming endpoint
func (s *TimelineConnectService) GetTimeline(
	ctx context.Context,
	req *connect.Request[pb.TimelineRequest],
	stream *connect.ServerStream[pb.TimelineChunk],
) error {
	// Start tracing span
	ctx, span := s.service.Tracer().Start(ctx, "connect.GetTimeline",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.Int64("query.start_timestamp", req.Msg.StartTimestamp),
			attribute.Int64("query.end_timestamp", req.Msg.EndTimestamp),
			attribute.StringSlice("query.namespaces", req.Msg.Namespaces),
			attribute.StringSlice("query.kinds", req.Msg.Kinds),
			attribute.Int("query.page_size", int(req.Msg.PageSize)),
		),
	)
	defer span.End()

	// Convert proto request to internal query request and pagination
	query, pagination, err := s.protoToQueryRequest(req.Msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid request")
		s.service.Logger().Warn("Invalid Connect request: %v (start=%d, end=%d, namespaces=%v, kinds=%v)",
			err, req.Msg.StartTimestamp, req.Msg.EndTimestamp, req.Msg.Namespaces, req.Msg.Kinds)
		return connect.NewError(connect.CodeInvalidArgument, err)
	}

	result, err := s.service.ExecuteTimeline(ctx, query, pagination)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Query execution failed")
		s.service.Logger().Error("Connect query execution failed: %v", err)
		return connect.NewError(connect.CodeInternal, err)
	}

	s.service.Logger().Debug("Query completed: resources=%d events=%d", result.ResourceResult.Count, len(result.EventResult.Events))

	span.SetAttributes(
		attribute.Int("result.resource_count", result.Index.Count()),
		attribute.Int64("result.execution_time_ms", result.Index.ExecutionTimeMs()),
	)

	s.service.Logger().Debug("Timeline response indexed: %d total resources from %d events",
		result.Index.Count(), result.ResourceResult.Count)

	s.service.Logger().Debug("Final result: %d resources, hasMore=%v, nextCursor=%q",
		len(result.Entries), result.Pagination != nil && result.Pagination.HasMore, paginationCursor(result.Pagination))

	// Stream metadata first (including pagination info)
	if err := s.sendMetadata(stream, result.ResourceResult, len(result.Entries), result.Pagination); err != nil {
		span.RecordError(err)
		s.service.Logger().Error("Failed to send metadata: %v", err)
		return connect.NewError(connect.CodeInternal, err)
	}

	// Stream resources in batches
	// If no resources, send an empty batch to signal completion
	if len(result.Entries) == 0 {
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
			return connect.NewError(connect.CodeInternal, err)
		}
	} else {
		groupedEntries := groupAndSortTimelineEntries(result.Entries)
		if err := s.streamEntryBatches(stream, result.Index, groupedEntries); err != nil {
			span.RecordError(err)
			s.service.Logger().Error("Failed to stream resources: %v", err)
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	span.SetStatus(codes.Ok, "Streaming completed successfully")
	s.service.Logger().Debug("Connect streaming completed: %d paginated resources (hasMore=%v)",
		len(result.Entries), result.Pagination != nil && result.Pagination.HasMore)

	return nil
}

// sendMetadata sends the metadata chunk with count, query stats, and pagination info
func (s *TimelineConnectService) sendMetadata(stream *connect.ServerStream[pb.TimelineChunk], result *models.QueryResult, totalCount int, pagination *models.PaginationResponse) error {
	chunk := &pb.TimelineChunk{
		ChunkType: &pb.TimelineChunk_Metadata{
			Metadata: buildTimelineMetadata(result, totalCount, pagination),
		},
	}

	return stream.Send(chunk)
}

func (s *TimelineConnectService) streamEntryBatches(stream *connect.ServerStream[pb.TimelineChunk], index *apptimeline.TimelineIndex, groups []*GroupedTimelineEntries) error {
	return streamTimelineEntryBatches(
		index,
		groups,
		s.service.BuildTimelineResources,
		func(res *models.Resource) *pb.TimelineResource { return ResourceToProto(s.service, res) },
		stream.Send,
	)
}

// Helper methods (reused from TimelineGRPCService)
func (s *TimelineConnectService) protoToQueryRequest(req *pb.TimelineRequest) (*models.QueryRequest, *models.PaginationRequest, error) {
	queryRequest, pagination, err := parseTimelineProtoRequest(s.service, req)
	if err != nil {
		return nil, nil, err
	}
	if pagination != nil {
		s.service.Logger().Debug("Pagination requested: pageSize=%d, cursor=%q", req.PageSize, req.Cursor)
	} else {
		s.service.Logger().Debug("No pagination requested (pageSize=0, cursor empty), will return all results")
	}
	return queryRequest, pagination, nil
}

func paginationCursor(pagination *models.PaginationResponse) string {
	if pagination == nil {
		return ""
	}
	return pagination.NextCursor
}
