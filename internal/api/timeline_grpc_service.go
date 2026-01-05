package api

import (
	"fmt"

	"github.com/moolen/spectre/internal/api/pb"
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
	service *TimelineService
}

// NewTimelineGRPCService creates a new timeline gRPC service with storage executor only
func NewTimelineGRPCService(queryExecutor QueryExecutor, logger *logging.Logger, tracer trace.Tracer) *TimelineGRPCService {
	return &TimelineGRPCService{
		service: NewTimelineService(queryExecutor, logger, tracer),
	}
}

// NewTimelineGRPCServiceWithMode creates a new timeline gRPC service with both executors
func NewTimelineGRPCServiceWithMode(storageExecutor, graphExecutor QueryExecutor, querySource TimelineQuerySource, logger *logging.Logger, tracer trace.Tracer) *TimelineGRPCService {
	return &TimelineGRPCService{
		service: NewTimelineServiceWithMode(storageExecutor, graphExecutor, querySource, logger, tracer),
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
	query, err := s.protoToQueryRequest(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid request")
		s.service.Logger().Warn("Invalid gRPC request: %v (start=%d, end=%d, namespace=%q, kind=%q)",
			err, req.StartTimestamp, req.EndTimestamp, req.Namespace, req.Kind)
		// Return proper gRPC error status
		return fmt.Errorf("invalid request: %w", err)
	}

	// Execute concurrent queries
	resourceResult, eventResult, err := s.service.ExecuteConcurrentQueries(ctx, query)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Query execution failed")
		s.service.Logger().Error("gRPC query execution failed: %v (start=%d, end=%d, namespace=%q, kind=%q)",
			err, query.StartTimestamp, query.EndTimestamp, query.Filters.Namespace, query.Filters.Kind)
		// Return proper gRPC error status
		return fmt.Errorf("query execution failed: %w", err)
	}

	// Log query results for debugging
	s.service.Logger().Debug("gRPC query completed: resources=%d, events=%d", resourceResult.Count, eventResult.Count)

	// Build timeline response
	timelineResponse := s.service.BuildTimelineResponse(resourceResult, eventResult)

	span.SetAttributes(
		attribute.Int("result.resource_count", timelineResponse.Count),
		attribute.Int64("result.execution_time_ms", timelineResponse.ExecutionTimeMs),
	)

	// Stream metadata first
	err = s.sendMetadata(stream, resourceResult, timelineResponse.Count)
	if err != nil {
		span.RecordError(err)
		s.service.Logger().Error("Failed to send metadata: %v", err)
		return err
	}

	// Group and sort resources
	groupedResources := groupAndSortResources(timelineResponse.Resources)

	// Stream resources in batches
	// If no resources, send an empty batch to signal completion
	if len(groupedResources) == 0 {
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
		err = s.streamResourceBatches(stream, groupedResources)
		if err != nil {
			span.RecordError(err)
			s.service.Logger().Error("Failed to stream resources: %v", err)
			return err
		}
	}

	span.SetStatus(codes.Ok, "Streaming completed successfully")
	s.service.Logger().Debug("gRPC streaming completed: %d resources in %d groups", timelineResponse.Count, len(groupedResources))

	return nil
}

// sendMetadata sends the metadata chunk with count and query stats
func (s *TimelineGRPCService) sendMetadata(stream pb.TimelineService_GetTimelineServer, result *models.QueryResult, totalCount int) error {
	metadata := &pb.TimelineMetadata{
		TotalCount:           int32(totalCount),
		FilesSearched:        int32(result.FilesSearched),
		SegmentsScanned:      int32(result.SegmentsScanned),
		SegmentsSkipped:      int32(result.SegmentsSkipped),
		QueryExecutionTimeMs: int64(result.ExecutionTimeMs),
	}

	chunk := &pb.TimelineChunk{
		ChunkType: &pb.TimelineChunk_Metadata{
			Metadata: metadata,
		},
	}

	return stream.Send(chunk)
}

// streamResourceBatches streams resources in batches, one batch per kind
func (s *TimelineGRPCService) streamResourceBatches(stream pb.TimelineService_GetTimelineServer, groups []*GroupedResources) error {
	for groupIdx, group := range groups {
		isLastGroup := groupIdx == len(groups)-1

		// Convert all models.Resource to pb.TimelineResource for this kind
		pbResources := make([]*pb.TimelineResource, len(group.Resources))
		for i, res := range group.Resources {
			pbResources[i] = s.service.ResourceToProto(&res)
		}

		chunk := &pb.TimelineChunk{
			ChunkType: &pb.TimelineChunk_Batch{
				Batch: &pb.ResourceBatch{
					Kind:         group.Kind,
					Resources:    pbResources,
					IsFinalBatch: isLastGroup,
				},
			},
		}

		if err := stream.Send(chunk); err != nil {
			return fmt.Errorf("failed to send batch: %w", err)
		}
	}

	return nil
}

// protoToQueryRequest converts protobuf request to internal QueryRequest
func (s *TimelineGRPCService) protoToQueryRequest(req *pb.TimelineRequest) (*models.QueryRequest, error) {
	filters := models.QueryFilters{
		Kind:      req.Kind,
		Namespace: req.Namespace,
		// Note: Name and LabelSelector are not currently supported by QueryFilters
		// They would need to be added to the models.QueryFilters struct if needed
	}

	if err := s.service.Validator().ValidateFilters(filters); err != nil {
		return nil, err
	}

	queryRequest := &models.QueryRequest{
		StartTimestamp: req.StartTimestamp,
		EndTimestamp:   req.EndTimestamp,
		Filters:        filters,
	}

	if err := queryRequest.Validate(); err != nil {
		return nil, err
	}

	return queryRequest, nil
}

// resourceToProto converts internal Resource model to protobuf TimelineResource
