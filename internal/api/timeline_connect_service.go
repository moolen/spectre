package api

import (
	"context"
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
// It wraps the unified TimelineService with Connect-compatible streaming
type TimelineConnectService struct {
	pbconnect.UnimplementedTimelineServiceHandler
	service *TimelineService
}

// NewTimelineConnectService creates a new timeline Connect service with storage executor only
func NewTimelineConnectService(queryExecutor QueryExecutor, logger *logging.Logger, tracer trace.Tracer) *TimelineConnectService {
	return &TimelineConnectService{
		service: NewTimelineService(queryExecutor, logger, tracer),
	}
}

// NewTimelineConnectServiceWithMode creates a new timeline Connect service with both executors
func NewTimelineConnectServiceWithMode(storageExecutor, graphExecutor QueryExecutor, querySource TimelineQuerySource, logger *logging.Logger, tracer trace.Tracer) *TimelineConnectService {
	return &TimelineConnectService{
		service: NewTimelineServiceWithMode(storageExecutor, graphExecutor, querySource, logger, tracer),
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

	// Execute queries with pagination support
	// Check if the executor supports pagination (graph executor does, storage doesn't yet)
	executor := s.service.GetActiveExecutor()
	if executor == nil {
		span.RecordError(fmt.Errorf("no query executor available"))
		span.SetStatus(codes.Error, "No executor available")
		return connect.NewError(connect.CodeInternal, fmt.Errorf("no query executor available"))
	}

	var resourceResult *models.QueryResult
	var eventResult *models.QueryResult
	var paginationResp *models.PaginationResponse

	// Try to use ExecutePaginated if available (graph executor)
	type PaginatedExecutor interface {
		ExecutePaginated(context.Context, *models.QueryRequest, *models.PaginationRequest) (*models.QueryResult, *models.PaginationResponse, error)
	}

	if paginatedExec, ok := executor.(PaginatedExecutor); ok {
		// Graph executor - supports pagination natively
		if pagination != nil {
			s.service.Logger().Debug("Using paginated executor with pageSize=%d, cursor=%q",
				pagination.GetPageSize(), pagination.Cursor)

			var execErr error
			resourceResult, paginationResp, execErr = paginatedExec.ExecutePaginated(ctx, query, pagination)
			if execErr != nil {
				span.RecordError(execErr)
				span.SetStatus(codes.Error, "Query execution failed")
				s.service.Logger().Error("Connect paginated query execution failed: %v", execErr)
				return connect.NewError(connect.CodeInternal, execErr)
			}
		} else {
			// No pagination requested - use regular Execute to get all results
			s.service.Logger().Debug("Using non-paginated executor (no pagination requested)")
			var execErr error
			resourceResult, execErr = executor.Execute(ctx, query)
			if execErr != nil {
				span.RecordError(execErr)
				span.SetStatus(codes.Error, "Query execution failed")
				s.service.Logger().Error("Connect query execution failed: %v", execErr)
				return connect.NewError(connect.CodeInternal, execErr)
			}
			// No pagination response when pagination wasn't requested
			paginationResp = nil
		}

		// Execute Event query separately (always non-paginated)
		eventQuery := &models.QueryRequest{
			StartTimestamp: query.StartTimestamp,
			EndTimestamp:   query.EndTimestamp,
			Filters: models.QueryFilters{
				Kinds:      []string{"Event"},
				Version:    "v1",
				Namespaces: query.Filters.GetNamespaces(),
			},
		}
		var eventErr error
		eventResult, eventErr = executor.Execute(ctx, eventQuery)
		if eventErr != nil {
			s.service.Logger().Warn("Failed to fetch Kubernetes events: %v", eventErr)
			// Non-critical: Event query failure shouldn't fail the entire request
			eventResult = &models.QueryResult{Events: []models.Event{}}
		}
	} else {
		// Storage executor - doesn't support pagination yet, fall back to client-side pagination
		s.service.Logger().Debug("Using non-paginated executor, will apply pagination client-side if requested")

		var execErr error
		resourceResult, eventResult, execErr = s.service.ExecuteConcurrentQueries(ctx, query)
		if execErr != nil {
			span.RecordError(execErr)
			span.SetStatus(codes.Error, "Query execution failed")
			s.service.Logger().Error("Connect query execution failed: %v", execErr)
			return connect.NewError(connect.CodeInternal, execErr)
		}
	}

	s.service.Logger().Debug("Query completed: resources=%d events=%d", resourceResult.Count, len(eventResult.Events))

	// Build timeline response
	timelineResponse := s.service.BuildTimelineResponse(resourceResult, eventResult)

	span.SetAttributes(
		attribute.Int("result.resource_count", timelineResponse.Count),
		attribute.Int64("result.execution_time_ms", timelineResponse.ExecutionTimeMs),
	)

	s.service.Logger().Debug("Timeline response built: %d total resources from %d events",
		timelineResponse.Count, resourceResult.Count)

	// Group and sort resources
	groupedResources := groupAndSortResources(timelineResponse.Resources)

	// Check if executor already did resource-based pagination
	// If so, use its pagination response directly (it has the correct cursor)
	var executorHasMore bool
	var executorNextCursor string
	var executorPaginationResp *models.PaginationResponse

	if paginationResp != nil {
		executorHasMore = paginationResp.HasMore
		executorNextCursor = paginationResp.NextCursor
		executorPaginationResp = paginationResp
		s.service.Logger().Debug("Executor pagination: hasMore=%v, nextCursor=%q (resource-based pagination already applied)",
			executorHasMore, executorNextCursor)
	}

	// If executor already did resource-based pagination and returned a cursor, use it directly
	// Otherwise, apply client-side pagination (for storage executor or when no cursor)
	var paginatedResources []*GroupedResources
	var paginatedCount int

	if executorPaginationResp != nil && executorNextCursor != "" {
		// Executor already did resource-based pagination, use its results directly
		s.service.Logger().Debug("Using executor's resource-based pagination (cursor=%q)", executorNextCursor)
		paginatedResources = groupedResources
		paginatedCount = timelineResponse.Count
		// Keep executor's pagination response (hasMore and nextCursor are already correct)
		paginationResp = executorPaginationResp
	} else if pagination != nil {
		// Pagination was requested - apply client-side pagination
		s.service.Logger().Debug("Applying client-side resource pagination: %d resources available, pageSize=%d, cursor=%q",
			timelineResponse.Count, pagination.GetPageSize(), pagination.Cursor)

		var paginationErr error
		paginatedResources, paginationResp, paginationErr = s.applyPagination(groupedResources, pagination)
		if paginationErr != nil {
			span.RecordError(paginationErr)
			s.service.Logger().Error("Failed to apply pagination: %v", paginationErr)
			return connect.NewError(connect.CodeInternal, paginationErr)
		}

		// Count paginated resources
		for _, group := range paginatedResources {
			paginatedCount += len(group.Resources)
		}

		// Adjust hasMore: The executor's hasMore is authoritative about whether there's more data in the database
		// applyPagination's hasMore only checks the current result set, which may be incomplete
		if executorHasMore {
			// Executor found more data in database, so there are definitely more resources
			// Override applyPagination's hasMore and preserve executor's cursor if available
			paginationResp.HasMore = true
			if executorNextCursor != "" {
				paginationResp.NextCursor = executorNextCursor
			}
			s.service.Logger().Debug("Executor indicated more data available in DB, overriding hasMore=true, nextCursor=%q (got %d resources from %d total)",
				executorNextCursor, paginatedCount, timelineResponse.Count)
		} else {
			// Executor says no more data in database
			// Use applyPagination's hasMore (which checks if there are more resources in current result set)
			// If we got fewer than pageSize, we're definitely done
			if paginatedCount < pagination.GetPageSize() {
				paginationResp.HasMore = false
				s.service.Logger().Debug("Got %d resources (less than pageSize %d) and executor says no more data, hasMore=false",
					paginatedCount, pagination.GetPageSize())
			} else {
				s.service.Logger().Debug("Got %d resources, hasMore=%v (from resource pagination, executor says no more in DB)",
					paginatedCount, paginationResp.HasMore)
			}
		}
	} else {
		// No pagination requested - return all resources
		s.service.Logger().Debug("No pagination requested, returning all %d resources", timelineResponse.Count)
		paginatedResources = groupedResources
		paginatedCount = timelineResponse.Count
		// Create a pagination response indicating no pagination
		paginationResp = &models.PaginationResponse{
			HasMore:    false,
			NextCursor: "",
			PageSize:   0,
		}
	}

	s.service.Logger().Debug("Final result: %d resources, hasMore=%v, nextCursor=%q",
		paginatedCount, paginationResp.HasMore, paginationResp.NextCursor)

	// Stream metadata first (including pagination info)
	if err := s.sendMetadata(stream, resourceResult, paginatedCount, paginationResp); err != nil {
		span.RecordError(err)
		s.service.Logger().Error("Failed to send metadata: %v", err)
		return connect.NewError(connect.CodeInternal, err)
	}

	// Stream resources in batches
	// If no resources, send an empty batch to signal completion
	if len(paginatedResources) == 0 {
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
		if err := s.streamResourceBatches(stream, paginatedResources); err != nil {
			span.RecordError(err)
			s.service.Logger().Error("Failed to stream resources: %v", err)
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	span.SetStatus(codes.Ok, "Streaming completed successfully")
	s.service.Logger().Debug("Connect streaming completed: %d paginated resources in %d groups (hasMore=%v)", paginatedCount, len(paginatedResources), paginationResp.HasMore)

	return nil
}

// sendMetadata sends the metadata chunk with count, query stats, and pagination info
func (s *TimelineConnectService) sendMetadata(stream *connect.ServerStream[pb.TimelineChunk], result *models.QueryResult, totalCount int, pagination *models.PaginationResponse) error {
	metadata := &pb.TimelineMetadata{
		TotalCount:           int32(totalCount),
		FilesSearched:        int32(result.FilesSearched),
		SegmentsScanned:      int32(result.SegmentsScanned),
		SegmentsSkipped:      int32(result.SegmentsSkipped),
		QueryExecutionTimeMs: int64(result.ExecutionTimeMs),
		// Pagination fields
		NextCursor: "",
		HasMore:    false,
		PageSize:   0,
	}
	// Only set pagination fields if pagination was used
	if pagination != nil {
		metadata.NextCursor = pagination.NextCursor
		metadata.HasMore = pagination.HasMore
		metadata.PageSize = int32(pagination.PageSize)
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
			return err
		}
	}

	return nil
}

// Helper methods (reused from TimelineGRPCService)
func (s *TimelineConnectService) protoToQueryRequest(req *pb.TimelineRequest) (*models.QueryRequest, *models.PaginationRequest, error) {
	// Build filters - prefer multi-value fields, fallback to single-value for backward compatibility
	var kinds []string
	if len(req.Kinds) > 0 {
		kinds = req.Kinds
	} else if req.Kind != "" {
		kinds = []string{req.Kind}
	}

	var namespaces []string
	if len(req.Namespaces) > 0 {
		namespaces = req.Namespaces
	} else if req.Namespace != "" {
		namespaces = []string{req.Namespace}
	}

	filters := models.QueryFilters{
		Kinds:      kinds,
		Namespaces: namespaces,
		// Note: Name and LabelSelector are not currently supported by QueryFilters
		// They would need to be added to the models.QueryFilters struct if needed
	}

	if err := s.service.Validator().ValidateFilters(filters); err != nil {
		return nil, nil, err
	}

	queryRequest := &models.QueryRequest{
		StartTimestamp: req.StartTimestamp,
		EndTimestamp:   req.EndTimestamp,
		Filters:        filters,
	}

	if err := queryRequest.Validate(); err != nil {
		return nil, nil, err
	}

	// Build pagination request
	// Only create pagination if explicitly requested (pageSize > 0 or cursor provided)
	// This maintains backward compatibility for clients that don't request pagination
	var pagination *models.PaginationRequest
	if req.PageSize > 0 || req.Cursor != "" {
		s.service.Logger().Debug("Pagination requested: pageSize=%d, cursor=%q", req.PageSize, req.Cursor)
		pagination = &models.PaginationRequest{
			PageSize: int(req.PageSize),
			Cursor:   req.Cursor,
		}
	} else {
		s.service.Logger().Debug("No pagination requested (pageSize=0, cursor empty), will return all results")
		pagination = nil
	}

	return queryRequest, pagination, nil
}


// applyPagination applies cursor-based pagination to grouped resources
// Returns paginated resources, pagination response, and error
func (s *TimelineConnectService) applyPagination(groups []*GroupedResources, pagination *models.PaginationRequest) ([]*GroupedResources, *models.PaginationResponse, error) {
	pageSize := pagination.GetPageSize()

	// Decode cursor if provided
	cursor, err := models.DecodeCursor(pagination.Cursor)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid cursor: %w", err)
	}

	// Flatten resources from all groups into a single sorted list
	allResources := make([]models.Resource, 0)
	for _, group := range groups {
		allResources = append(allResources, group.Resources...)
	}

	s.service.Logger().Debug("applyPagination: total resources=%d, pageSize=%d, cursor=%v",
		len(allResources), pageSize, cursor)

	// Apply cursor filtering if cursor exists
	startIdx := 0
	if cursor != nil {
		s.service.Logger().Debug("applyPagination: looking for cursor position (kind=%s, ns=%s, name=%s)",
			cursor.Kind, cursor.Namespace, cursor.Name)
		// Find the first resource after the cursor
		// Resources are sorted by Kind -> Namespace -> Name
		for i, res := range allResources {
			// Skip resources until we find one that comes after the cursor
			if res.Kind > cursor.Kind {
				startIdx = i
				break
			}
			if res.Kind == cursor.Kind {
				if res.Namespace > cursor.Namespace {
					startIdx = i
					break
				}
				if res.Namespace == cursor.Namespace && res.Name > cursor.Name {
					startIdx = i
					break
				}
			}
			// If this is the last resource and we haven't found a match, start at the end
			if i == len(allResources)-1 {
				startIdx = len(allResources)
			}
		}
		s.service.Logger().Debug("applyPagination: cursor positioned at index %d", startIdx)
	}

	// Extract page of resources
	endIdx := startIdx + pageSize
	hasMore := endIdx < len(allResources)
	if endIdx > len(allResources) {
		endIdx = len(allResources)
	}

	s.service.Logger().Debug("applyPagination: slice [%d:%d], hasMore=%v", startIdx, endIdx, hasMore)

	pageResources := allResources[startIdx:endIdx]

	// Generate next cursor if there are more resources
	var nextCursor string
	if hasMore && len(pageResources) > 0 {
		lastResource := pageResources[len(pageResources)-1]
		cursorObj := models.NewResourceCursor(lastResource.Kind, lastResource.Namespace, lastResource.Name)
		nextCursor = cursorObj.Encode()
		s.service.Logger().Debug("applyPagination: nextCursor generated from (kind=%s, ns=%s, name=%s)",
			lastResource.Kind, lastResource.Namespace, lastResource.Name)
	}

	// Re-group the paginated resources by kind
	paginatedGroups := groupAndSortResources(pageResources)

	paginationResp := &models.PaginationResponse{
		HasMore:    hasMore,
		NextCursor: nextCursor,
		PageSize:   pageSize,
	}

	return paginatedGroups, paginationResp, nil
}
