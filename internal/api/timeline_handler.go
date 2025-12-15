package api

import (
	"context"
	"net/http"
	"sync"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"github.com/moolen/spectre/internal/storage"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TimelineHandler handles /v1/timeline requests
// Returns full resource data with statusSegments and events for timeline visualization
type TimelineHandler struct {
	queryExecutor QueryExecutor
	logger        *logging.Logger
	validator     *Validator
	tracer        trace.Tracer
}

// NewTimelineHandler creates a new timeline handler
func NewTimelineHandler(queryExecutor QueryExecutor, logger *logging.Logger, tracer trace.Tracer) *TimelineHandler {
	return &TimelineHandler{
		queryExecutor: queryExecutor,
		logger:        logger,
		validator:     NewValidator(),
		tracer:        tracer,
	}
}

// Handle handles timeline requests
func (th *TimelineHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Start HTTP handler span
	ctx, span := th.tracer.Start(ctx, "timeline.Handle",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.route", "/v1/timeline"),
		),
	)
	defer span.End()

	query, err := th.parseQuery(r)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid request")
		th.logger.Warn("Invalid request: %v", err)
		th.respondWithError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Add query parameters as span attributes
	span.SetAttributes(
		attribute.Int64("query.start_timestamp", query.StartTimestamp),
		attribute.Int64("query.end_timestamp", query.EndTimestamp),
		attribute.String("query.namespace", query.Filters.Namespace),
		attribute.String("query.kind", query.Filters.Kind),
	)

	// Execute both queries concurrently
	result, eventResult, err := th.executeConcurrentQueries(ctx, query)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Query execution failed")
		th.logger.Error("Query execution failed: %v", err)
		th.respondWithError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to execute query")
		return
	}

	// Add result metrics to span
	span.SetAttributes(
		attribute.Int("result.event_count", int(result.Count)),
		attribute.Int("result.files_searched", int(result.FilesSearched)),
		attribute.Int("result.segments_scanned", int(result.SegmentsScanned)),
		attribute.Int("result.segments_skipped", int(result.SegmentsSkipped)),
		attribute.Int64("result.execution_time_ms", int64(result.ExecutionTimeMs)),
		attribute.Int("result.k8s_event_count", int(eventResult.Count)),
		attribute.Int64("result.k8s_events_execution_time_ms", int64(eventResult.ExecutionTimeMs)),
	)

	timelineResponse := th.buildTimelineResponse(result, eventResult)

	span.SetAttributes(
		attribute.Int("response.resource_count", timelineResponse.Count),
	)
	span.SetStatus(codes.Ok, "Request completed successfully")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = writeJSON(w, timelineResponse)

	th.logger.Debug("Timeline completed: resources=%d, executionTime=%dms", timelineResponse.Count, timelineResponse.ExecutionTimeMs)
}

// executeConcurrentQueries executes resource and Event queries concurrently
func (th *TimelineHandler) executeConcurrentQueries(ctx context.Context, query *models.QueryRequest) (*models.QueryResult, *models.QueryResult, error) {
	// Create child span for concurrent execution
	ctx, span := th.tracer.Start(ctx, "timeline.executeConcurrentQueries")
	defer span.End()

	var (
		resourceResult *models.QueryResult
		eventResult    *models.QueryResult
		resourceErr    error
		eventErr       error
		wg             sync.WaitGroup
	)

	// Build Event query upfront
	eventQuery := &models.QueryRequest{
		StartTimestamp: query.StartTimestamp,
		EndTimestamp:   query.EndTimestamp,
		Filters: models.QueryFilters{
			Kind:      "Event",
			Version:   "v1",
			Namespace: query.Filters.Namespace,
		},
	}

	wg.Add(2)

	// Execute resource query
	go func() {
		defer wg.Done()
		_, resourceSpan := th.tracer.Start(ctx, "timeline.resourceQuery")
		defer resourceSpan.End()

		resourceResult, resourceErr = th.queryExecutor.Execute(ctx, query)
		if resourceErr != nil {
			resourceSpan.RecordError(resourceErr)
			resourceSpan.SetStatus(codes.Error, "Resource query failed")
		}
	}()

	// Execute Event query
	go func() {
		defer wg.Done()
		_, eventSpan := th.tracer.Start(ctx, "timeline.eventQuery")
		defer eventSpan.End()

		eventResult, eventErr = th.queryExecutor.Execute(ctx, eventQuery)
		if eventErr != nil {
			eventSpan.RecordError(eventErr)
			eventSpan.SetStatus(codes.Error, "Event query failed")
			th.logger.Warn("Failed to fetch Kubernetes events for timeline: %v", eventErr)
			// Non-critical: Event query failure shouldn't fail the entire request
		}
	}()

	wg.Wait()

	// Handle errors with priority on resource query (critical)
	if resourceErr != nil {
		return nil, nil, resourceErr
	}

	// If Event query failed, return empty result instead of nil
	if eventErr != nil {
		eventResult = &models.QueryResult{
			Events: []models.Event{},
		}
	}

	span.SetAttributes(
		attribute.Int("resource_count", int(resourceResult.Count)),
		attribute.Int("event_count", int(eventResult.Count)),
	)

	th.logger.Debug("Concurrent queries completed: resources=%d (%dms), events=%d (%dms)",
		resourceResult.Count, resourceResult.ExecutionTimeMs,
		eventResult.Count, eventResult.ExecutionTimeMs)

	return resourceResult, eventResult, nil
}

// buildTimelineResponse transforms QueryResult into TimelineResponse with full resource data
func (th *TimelineHandler) buildTimelineResponse(queryResult, eventResult *models.QueryResult) *models.SearchResponse {
	resourceBuilder := storage.NewResourceBuilder()
	resourceMap := resourceBuilder.BuildResourcesFromEvents(queryResult.Events)

	// Attach pre-fetched K8s events
	if len(eventResult.Events) > 0 {
		resourceBuilder.AttachK8sEvents(resourceMap, eventResult.Events)
	}

	resources := make([]models.Resource, 0, len(resourceMap))
	for _, resource := range resourceMap {
		resources = append(resources, *resource)
	}

	return &models.SearchResponse{
		Resources:       resources,
		Count:           len(resources),
		ExecutionTimeMs: int64(queryResult.ExecutionTimeMs),
	}
}

// parseQuery parses and validates query parameters (same as SearchHandler)
func (th *TimelineHandler) parseQuery(r *http.Request) (*models.QueryRequest, error) {
	query := r.URL.Query()

	startStr := query.Get("start")
	start, err := ParseTimestamp(startStr, "start")
	if err != nil {
		return nil, err
	}

	endStr := query.Get("end")
	end, err := ParseTimestamp(endStr, "end")
	if err != nil {
		return nil, err
	}

	if start < 0 || end < 0 {
		return nil, NewValidationError("timestamps must be non-negative")
	}
	if start > end {
		return nil, NewValidationError("start timestamp must be less than or equal to end timestamp")
	}

	filters := models.QueryFilters{
		Group:     query.Get("group"),
		Version:   query.Get("version"),
		Kind:      query.Get("kind"),
		Namespace: query.Get("namespace"),
	}

	if err := th.validator.ValidateFilters(filters); err != nil {
		return nil, err
	}

	queryRequest := &models.QueryRequest{
		StartTimestamp: start,
		EndTimestamp:   end,
		Filters:        filters,
	}

	if err := queryRequest.Validate(); err != nil {
		return nil, err
	}

	return queryRequest, nil
}

func (th *TimelineHandler) respondWithError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]string{
		"error":   errorCode,
		"message": message,
	}

	_ = writeJSON(w, response)
}
