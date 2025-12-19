package api

import (
	"context"
	"encoding/json"
	"fmt"

	"connectrpc.com/connect"
	"github.com/moolen/spectre/internal/api/pb"
	"github.com/moolen/spectre/internal/api/pb/pbconnect"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TimelineConnectService implements the Connect TimelineService interface
// It wraps the existing TimelineGRPCService logic with Connect-compatible streaming
type TimelineConnectService struct {
	pbconnect.UnimplementedTimelineServiceHandler
	storageExecutor QueryExecutor
	graphExecutor   QueryExecutor
	querySource     TimelineQuerySource
	logger          *logging.Logger
	tracer          trace.Tracer
	validator       *Validator
}

// NewTimelineConnectService creates a new timeline Connect service with storage executor only
func NewTimelineConnectService(queryExecutor QueryExecutor, logger *logging.Logger, tracer trace.Tracer) *TimelineConnectService {
	return &TimelineConnectService{
		storageExecutor: queryExecutor,
		querySource:     TimelineQuerySourceStorage,
		logger:          logger,
		validator:       NewValidator(),
		tracer:          tracer,
	}
}

// NewTimelineConnectServiceWithMode creates a new timeline Connect service with both executors
func NewTimelineConnectServiceWithMode(storageExecutor, graphExecutor QueryExecutor, querySource TimelineQuerySource, logger *logging.Logger, tracer trace.Tracer) *TimelineConnectService {
	return &TimelineConnectService{
		storageExecutor: storageExecutor,
		graphExecutor:   graphExecutor,
		querySource:     querySource,
		logger:          logger,
		validator:       NewValidator(),
		tracer:          tracer,
	}
}

// GetTimeline implements the Connect streaming endpoint
func (s *TimelineConnectService) GetTimeline(
	ctx context.Context,
	req *connect.Request[pb.TimelineRequest],
	stream *connect.ServerStream[pb.TimelineChunk],
) error {
	// Start tracing span
	ctx, span := s.tracer.Start(ctx, "connect.GetTimeline",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.Int64("query.start_timestamp", req.Msg.StartTimestamp),
			attribute.Int64("query.end_timestamp", req.Msg.EndTimestamp),
			attribute.String("query.namespace", req.Msg.Namespace),
			attribute.String("query.kind", req.Msg.Kind),
		),
	)
	defer span.End()

	// Convert proto request to internal query request
	query, err := s.protoToQueryRequest(req.Msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid request")
		s.logger.Warn("Invalid Connect request: %v (start=%d, end=%d, namespace=%q, kind=%q)",
			err, req.Msg.StartTimestamp, req.Msg.EndTimestamp, req.Msg.Namespace, req.Msg.Kind)
		return connect.NewError(connect.CodeInvalidArgument, err)
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
		s.logger.Error("Connect query execution failed: %v (start=%d, end=%d, namespace=%q, kind=%q)",
			err, query.StartTimestamp, query.EndTimestamp, query.Filters.Namespace, query.Filters.Kind)
		return connect.NewError(connect.CodeInternal, err)
	}

	// Log query results for debugging
	s.logger.Debug("Connect query completed: resources=%d, events=%d", resourceResult.Count, eventResult.Count)

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
		return connect.NewError(connect.CodeInternal, err)
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
			return connect.NewError(connect.CodeInternal, err)
		}
	} else {
		err = s.streamResourceBatches(stream, groupedResources)
		if err != nil {
			span.RecordError(err)
			s.logger.Error("Failed to stream resources: %v", err)
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	span.SetStatus(codes.Ok, "Streaming completed successfully")
	s.logger.Debug("Connect streaming completed: %d resources in %d groups", timelineResponse.Count, len(groupedResources))

	return nil
}

// sendMetadata sends the metadata chunk with count and query stats
func (s *TimelineConnectService) sendMetadata(stream *connect.ServerStream[pb.TimelineChunk], result *models.QueryResult, totalCount int) error {
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
func (s *TimelineConnectService) streamResourceBatches(stream *connect.ServerStream[pb.TimelineChunk], groups []*GroupedResources) error {
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
			return err
		}
	}

	return nil
}

// Helper methods (reused from TimelineGRPCService)
func (s *TimelineConnectService) protoToQueryRequest(req *pb.TimelineRequest) (*models.QueryRequest, error) {
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

func (s *TimelineConnectService) resourceToProto(res *models.Resource) *pb.TimelineResource {
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
func (s *TimelineConnectService) extractReasonFromResourceData(data []byte) (string, bool) {
	if len(data) == 0 {
		return "", true // No data means status is inferred
	}

	// Parse the JSON to extract the reason from status.conditions
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return "", true
	}

	status, ok := obj["status"].(map[string]interface{})
	if !ok {
		return "", true
	}

	conditions, ok := status["conditions"].([]interface{})
	if !ok || len(conditions) == 0 {
		return "", true
	}

	// Look for the first condition with a reason
	for _, condRaw := range conditions {
		cond, ok := condRaw.(map[string]interface{})
		if !ok {
			continue
		}

		if reason, ok := cond["reason"].(string); ok && reason != "" {
			return reason, false // Found a reason, status is not inferred
		}
	}

	return "", true
}
