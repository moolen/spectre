package api

import (
	"encoding/json"
	"fmt"

	"github.com/moolen/spectre/internal/api/pb"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TimelineGRPCService implements the gRPC TimelineService
type TimelineGRPCService struct {
	pb.UnimplementedTimelineServiceServer
	storageExecutor QueryExecutor
	graphExecutor   QueryExecutor
	querySource     TimelineQuerySource
	logger          *logging.Logger
	tracer          trace.Tracer
	validator       *Validator
}

// NewTimelineGRPCService creates a new timeline gRPC service with storage executor only
func NewTimelineGRPCService(queryExecutor QueryExecutor, logger *logging.Logger, tracer trace.Tracer) *TimelineGRPCService {
	return &TimelineGRPCService{
		storageExecutor: queryExecutor,
		querySource:     TimelineQuerySourceStorage,
		logger:          logger,
		validator:       NewValidator(),
		tracer:          tracer,
	}
}

// NewTimelineGRPCServiceWithMode creates a new timeline gRPC service with both executors
func NewTimelineGRPCServiceWithMode(storageExecutor, graphExecutor QueryExecutor, querySource TimelineQuerySource, logger *logging.Logger, tracer trace.Tracer) *TimelineGRPCService {
	return &TimelineGRPCService{
		storageExecutor: storageExecutor,
		graphExecutor:   graphExecutor,
		querySource:     querySource,
		logger:          logger,
		validator:       NewValidator(),
		tracer:          tracer,
	}
}

// GetTimeline implements the gRPC streaming endpoint
func (s *TimelineGRPCService) GetTimeline(req *pb.TimelineRequest, stream pb.TimelineService_GetTimelineServer) error {
	ctx := stream.Context()

	// Start tracing span
	ctx, span := s.tracer.Start(ctx, "grpc.GetTimeline",
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
		s.logger.Warn("Invalid gRPC request: %v (start=%d, end=%d, namespace=%q, kind=%q)",
			err, req.StartTimestamp, req.EndTimestamp, req.Namespace, req.Kind)
		// Return proper gRPC error status
		return fmt.Errorf("invalid request: %w", err)
	}

	// Execute concurrent queries (reuse existing logic from TimelineHandler)
	timelineHandler := &TimelineHandler{
		storageExecutor: s.storageExecutor,
		graphExecutor:   s.graphExecutor,
		querySource:     s.querySource,
		logger:          s.logger,
		validator:       s.validator,
		tracer:          s.tracer,
	}

	resourceResult, eventResult, err := timelineHandler.executeConcurrentQueries(ctx, query)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Query execution failed")
		s.logger.Error("gRPC query execution failed: %v (start=%d, end=%d, namespace=%q, kind=%q)",
			err, query.StartTimestamp, query.EndTimestamp, query.Filters.Namespace, query.Filters.Kind)
		// Return proper gRPC error status
		return fmt.Errorf("query execution failed: %w", err)
	}

	// Log query results for debugging
	s.logger.Debug("gRPC query completed: resources=%d, events=%d", resourceResult.Count, eventResult.Count)

	// Build timeline response
	timelineResponse := timelineHandler.buildTimelineResponse(resourceResult, eventResult)

	span.SetAttributes(
		attribute.Int("result.resource_count", timelineResponse.Count),
		attribute.Int64("result.execution_time_ms", timelineResponse.ExecutionTimeMs),
	)

	// Stream metadata first
	err = s.sendMetadata(stream, resourceResult, timelineResponse.Count)
	if err != nil {
		span.RecordError(err)
		s.logger.Error("Failed to send metadata: %v", err)
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
			s.logger.Error("Failed to send empty batch: %v", err)
			return err
		}
	} else {
		err = s.streamResourceBatches(stream, groupedResources)
		if err != nil {
			span.RecordError(err)
			s.logger.Error("Failed to stream resources: %v", err)
			return err
		}
	}

	span.SetStatus(codes.Ok, "Streaming completed successfully")
	s.logger.Debug("gRPC streaming completed: %d resources in %d groups", timelineResponse.Count, len(groupedResources))

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
			pbResources[i] = s.resourceToProto(&res)
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

	if err := s.validator.ValidateFilters(filters); err != nil {
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
func (s *TimelineGRPCService) resourceToProto(res *models.Resource) *pb.TimelineResource {
	pbResource := &pb.TimelineResource{
		Id:          res.ID,
		Kind:        res.Kind,
		ApiVersion:  fmt.Sprintf("%s/%s", res.Group, res.Version),
		Namespace:   res.Namespace,
		Name:        res.Name,
		PreExisting: res.PreExisting,
		Labels:      make(map[string]string),
	}

	// Convert status segments
	pbResource.StatusSegments = make([]*pb.StatusSegment, len(res.StatusSegments))
	for i, seg := range res.StatusSegments {
		// Extract reason and determine if status is inferred
		reason, inferred := s.extractReasonFromResourceData(seg.ResourceData)
		pbResource.StatusSegments[i] = &pb.StatusSegment{
			Id:           fmt.Sprintf("%s-%d", res.ID, i),
			ResourceId:   res.ID,
			Status:       seg.Status,
			Reason:       reason,
			Message:      seg.Message,
			StartTime:    seg.StartTime,
			EndTime:      seg.EndTime,
			Inferred:     inferred,
			ResourceData: seg.ResourceData, // Full Kubernetes resource JSON
		}
	}

	// Convert K8s events (note: protobuf generated K8SEvent with capital S)
	pbResource.Events = make([]*pb.K8SEvent, len(res.Events))
	for i, evt := range res.Events {
		pbResource.Events[i] = &pb.K8SEvent{
			Uid:               evt.ID,
			Type:              evt.Type,
			Reason:            evt.Reason,
			Message:           evt.Message,
			Timestamp:         evt.Timestamp,
			InvolvedObjectUid: res.ID, // The resource this event belongs to
		}
	}

	return pbResource
}

// extractReasonFromResourceData parses the resource JSON and extracts the reason
// from the status conditions. Returns the reason and whether the status was inferred.
func (s *TimelineGRPCService) extractReasonFromResourceData(data []byte) (string, bool) {
	if len(data) == 0 {
		return "", true // No data means status is inferred
	}

	// Parse the resource data
	var resource map[string]interface{}
	if err := json.Unmarshal(data, &resource); err != nil {
		return "", true // Parse error means status is inferred
	}

	// Extract status.conditions
	status, ok := resource["status"].(map[string]interface{})
	if !ok {
		return "", true // No status means inferred
	}

	conditions, ok := status["conditions"].([]interface{})
	if !ok || len(conditions) == 0 {
		return "", true // No conditions means inferred
	}

	// Look for Ready or Healthy condition
	for _, condInterface := range conditions {
		cond, ok := condInterface.(map[string]interface{})
		if !ok {
			continue
		}

		condType, _ := cond["type"].(string)
		if condType == "Ready" || condType == "Healthy" {
			// Found a condition - status is not inferred
			reason, _ := cond["reason"].(string)
			return reason, false
		}
	}

	// No Ready/Healthy condition found - status is inferred
	return "", true
}
