package api

import (
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"github.com/moolen/spectre/internal/storage"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TimelineQuerySource specifies which executor to use for queries
type TimelineQuerySource string

const (
	TimelineQuerySourceStorage TimelineQuerySource = "storage"
	TimelineQuerySourceGraph   TimelineQuerySource = "graph"
)

// TimelineHandler handles /v1/timeline requests
// Returns full resource data with statusSegments and events for timeline visualization
type TimelineHandler struct {
	storageExecutor QueryExecutor       // Storage-based query executor
	graphExecutor   QueryExecutor       // Graph-based query executor (optional)
	querySource     TimelineQuerySource // Which executor to use
	logger          *logging.Logger
	validator       *Validator
	tracer          trace.Tracer
}

// NewTimelineHandler creates a new timeline handler with storage executor only
func NewTimelineHandler(queryExecutor QueryExecutor, logger *logging.Logger, tracer trace.Tracer) *TimelineHandler {
	return &TimelineHandler{
		storageExecutor: queryExecutor,
		querySource:     TimelineQuerySourceStorage,
		logger:          logger,
		validator:       NewValidator(),
		tracer:          tracer,
	}
}

// NewTimelineHandlerWithMode creates a timeline handler with dual executors
func NewTimelineHandlerWithMode(storageExecutor, graphExecutor QueryExecutor, source TimelineQuerySource, logger *logging.Logger, tracer trace.Tracer) *TimelineHandler {
	return &TimelineHandler{
		storageExecutor: storageExecutor,
		graphExecutor:   graphExecutor,
		querySource:     source,
		logger:          logger,
		validator:       NewValidator(),
		tracer:          tracer,
	}
}

// Handle handles timeline requests
func (th *TimelineHandler) Handle(w http.ResponseWriter, r *http.Request) {
	requestStart := time.Now()
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
		attribute.StringSlice("query.namespaces", query.Filters.GetNamespaces()),
		attribute.StringSlice("query.kinds", query.Filters.GetKinds()),
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

	// Calculate total request time
	totalDuration := time.Since(requestStart)

	// Add Server-Timing headers
	th.addServerTimingHeaders(w, result, eventResult, totalDuration)

	// Write JSON response with compression if supported
	th.writeJSONResponse(w, r, timelineResponse)

	th.logger.Debug("Timeline completed: resources=%d, executionTime=%dms total=%dms", timelineResponse.Count, timelineResponse.ExecutionTimeMs, totalDuration.Milliseconds())
}

// executeConcurrentQueries executes resource and Event queries concurrently
func (th *TimelineHandler) executeConcurrentQueries(ctx context.Context, query *models.QueryRequest) (*models.QueryResult, *models.QueryResult, error) {
	// Create child span for concurrent execution
	ctx, span := th.tracer.Start(ctx, "timeline.executeConcurrentQueries")
	defer span.End()

	// Select which executor to use
	executor := th.getActiveExecutor()
	if executor == nil {
		return nil, nil, fmt.Errorf("no query executor available")
	}

	span.SetAttributes(attribute.String("query.source", string(th.querySource)))

	var (
		resourceResult *models.QueryResult
		eventResult    *models.QueryResult
		resourceErr    error
		eventErr       error
		wg             sync.WaitGroup
	)

	// Create shared cache for coordinating file reads between concurrent queries
	// This ensures each file is only read once even though both queries may need it
	// Note: Only applies to storage executor
	if th.querySource == TimelineQuerySourceStorage {
		sharedCache := storage.NewSharedFileDataCache()
		executor.SetSharedCache(sharedCache)
		defer func() {
			// Clear shared cache after queries complete
			executor.SetSharedCache(nil)
			th.logger.Debug("Shared cache coordinated %d files across concurrent queries", sharedCache.Size())
		}()
	}

	// Build Event query upfront
	// Use same namespaces filter as the resource query
	eventQuery := &models.QueryRequest{
		StartTimestamp: query.StartTimestamp,
		EndTimestamp:   query.EndTimestamp,
		Filters: models.QueryFilters{
			Kinds:      []string{"Event"},
			Version:    "v1",
			Namespaces: query.Filters.GetNamespaces(),
		},
	}

	wg.Add(2)

	// Execute resource query
	go func() {
		defer wg.Done()
		_, resourceSpan := th.tracer.Start(ctx, "timeline.resourceQuery")
		defer resourceSpan.End()

		resourceResult, resourceErr = executor.Execute(ctx, query)
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

		eventResult, eventErr = executor.Execute(ctx, eventQuery)
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
	resourceMap := resourceBuilder.BuildResourcesFromEventsWithQueryTime(queryResult.Events, queryResult.QueryStartTime)

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

	// Parse multi-value filters
	// Support both ?kind=Pod&kind=Deployment and ?kinds=Pod,Deployment
	kinds := parseMultiValueParam(query, "kind", "kinds")
	namespaces := parseMultiValueParam(query, "namespace", "namespaces")

	filters := models.QueryFilters{
		Group:      query.Get("group"),
		Version:    query.Get("version"),
		Kinds:      kinds,
		Namespaces: namespaces,
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

// parseQueryWithPagination parses query parameters including pagination
func (th *TimelineHandler) parseQueryWithPagination(r *http.Request) (*models.QueryRequest, *models.PaginationRequest, error) {
	queryRequest, err := th.parseQuery(r)
	if err != nil {
		return nil, nil, err
	}

	pagination := th.parsePagination(r)
	return queryRequest, pagination, nil
}

// parsePagination parses pagination query parameters
func (th *TimelineHandler) parsePagination(r *http.Request) *models.PaginationRequest {
	query := r.URL.Query()

	pageSize := parseIntOrDefault(query.Get("page_size"), models.DefaultPageSize)
	cursor := query.Get("cursor")

	return &models.PaginationRequest{
		PageSize: pageSize,
		Cursor:   cursor,
	}
}

// parseMultiValueParam parses a query parameter that can be specified multiple times
// or as a comma-separated list in an alternate parameter name
// e.g., ?kind=Pod&kind=Deployment or ?kinds=Pod,Deployment
func parseMultiValueParam(query map[string][]string, singularName, pluralName string) []string {
	// First, try the repeated singular param (e.g., ?kind=Pod&kind=Deployment)
	values := query[singularName]
	if len(values) > 0 {
		return values
	}

	// Then, try the plural param with comma-separated values (e.g., ?kinds=Pod,Deployment)
	if pluralCSV, ok := query[pluralName]; ok && len(pluralCSV) > 0 && pluralCSV[0] != "" {
		return strings.Split(pluralCSV[0], ",")
	}

	return nil
}

// parseIntOrDefault parses an integer from string, returning default on error
func parseIntOrDefault(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	var val int
	if _, err := fmt.Sscanf(s, "%d", &val); err != nil {
		return defaultVal
	}
	return val
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

// addServerTimingHeaders adds Server-Timing headers to the response
// following the Server Timing API specification: https://w3c.github.io/server-timing/
func (th *TimelineHandler) addServerTimingHeaders(w http.ResponseWriter, resourceResult, eventResult *models.QueryResult, totalDuration time.Duration) {
	// Format: metric-name;dur=duration;desc="description"
	// Multiple metrics are comma-separated in a single header
	var metrics []string

	// Resource query execution time
	if resourceResult != nil {
		metrics = append(metrics, fmt.Sprintf("resource;dur=%.2f;desc=\"Resource query\"", float64(resourceResult.ExecutionTimeMs)))
		if resourceResult.FilesSearched > 0 {
			metrics = append(metrics, fmt.Sprintf("files;desc=\"Files searched: %d\"", resourceResult.FilesSearched))
		}
		if resourceResult.SegmentsScanned > 0 || resourceResult.SegmentsSkipped > 0 {
			metrics = append(metrics, fmt.Sprintf("segments;desc=\"Scanned: %d, skipped: %d\"", resourceResult.SegmentsScanned, resourceResult.SegmentsSkipped))
		}
	}

	// Event query execution time
	if eventResult != nil && eventResult.ExecutionTimeMs > 0 {
		metrics = append(metrics, fmt.Sprintf("events;dur=%.2f;desc=\"K8s Event query\"", float64(eventResult.ExecutionTimeMs)))
	}

	// Total request duration
	totalMs := float64(totalDuration.Nanoseconds()) / 1e6
	metrics = append(metrics, fmt.Sprintf("total;dur=%.2f;desc=\"Total request\"", totalMs))

	// Set Server-Timing header with all metrics comma-separated
	// Per spec: multiple metrics can be in one header separated by commas
	if len(metrics) > 0 {
		headerValue := metrics[0]
		for i := 1; i < len(metrics); i++ {
			headerValue += ", " + metrics[i]
		}
		w.Header().Set("Server-Timing", headerValue)
	}
}

// writeJSONResponse writes a JSON response with gzip compression if the client supports it
func (th *TimelineHandler) writeJSONResponse(w http.ResponseWriter, r *http.Request, data interface{}) {
	w.Header().Set("Content-Type", "application/json")

	// Check if client supports gzip compression
	acceptEncoding := r.Header.Get("Accept-Encoding")
	canCompress := strings.Contains(acceptEncoding, "gzip")

	if canCompress {
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)

		gzWriter := gzip.NewWriter(w)
		defer func() {
			if err := gzWriter.Close(); err != nil {
				th.logger.Warn("Failed to close gzip writer: %v", err)
			}
		}()

		if err := writeJSON(gzWriter, data); err != nil {
			th.logger.Error("Failed to write compressed JSON: %v", err)
		}
	} else {
		w.WriteHeader(http.StatusOK)
		if err := writeJSON(w, data); err != nil {
			th.logger.Error("Failed to write JSON: %v", err)
		}
	}
}

// getActiveExecutor returns the appropriate query executor based on configuration
func (th *TimelineHandler) getActiveExecutor() QueryExecutor {
	switch th.querySource {
	case TimelineQuerySourceGraph:
		if th.graphExecutor != nil {
			return th.graphExecutor
		}
		th.logger.Warn("Graph executor requested but not available, falling back to storage")
		return th.storageExecutor
	case TimelineQuerySourceStorage:
		fallthrough
	default:
		return th.storageExecutor
	}
}
