package handlers

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// TimelineHandler handles /v1/timeline requests
// Returns full resource data with statusSegments and events for timeline visualization
type TimelineHandler struct {
	timelineService *api.TimelineService
	logger          *logging.Logger
	tracer          trace.Tracer
}

// NewTimelineHandler creates a new timeline handler using the provided TimelineService
func NewTimelineHandler(timelineService *api.TimelineService, logger *logging.Logger, tracer trace.Tracer) *TimelineHandler {
	return &TimelineHandler{
		timelineService: timelineService,
		logger:          logger,
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

	// Parse query parameters using service
	queryParams := r.URL.Query()
	query, err := th.timelineService.ParseQueryParameters(
		ctx,
		queryParams.Get("start"),
		queryParams.Get("end"),
		queryParams,
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Invalid request")
		th.logger.Warn("Invalid request: %v", err)
		th.respondWithError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Parse pagination using service
	const maxPageSize = 1000 // Maximum page size for timeline queries
	pagination := th.timelineService.ParsePagination(
		queryParams.Get("page_size"),
		queryParams.Get("cursor"),
		maxPageSize,
	)

	// Attach pagination to query so executor can use it
	query.Pagination = pagination

	// Add query parameters as span attributes
	span.SetAttributes(
		attribute.Int64("query.start_timestamp", query.StartTimestamp),
		attribute.Int64("query.end_timestamp", query.EndTimestamp),
		attribute.StringSlice("query.namespaces", query.Filters.GetNamespaces()),
		attribute.StringSlice("query.kinds", query.Filters.GetKinds()),
	)

	// Execute both queries concurrently using service
	result, eventResult, err := th.timelineService.ExecuteConcurrentQueries(ctx, query)
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

	// Build timeline response using service
	timelineResponse := th.timelineService.BuildTimelineResponse(result, eventResult)

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

func (th *TimelineHandler) respondWithError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	api.WriteError(w, statusCode, errorCode, message)
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

		if err := api.WriteJSON(gzWriter, data); err != nil {
			th.logger.Error("Failed to write compressed JSON: %v", err)
		}
	} else {
		w.WriteHeader(http.StatusOK)
		if err := api.WriteJSON(w, data); err != nil {
			th.logger.Error("Failed to write JSON: %v", err)
		}
	}
}
