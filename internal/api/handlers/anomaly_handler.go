package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/moolen/spectre/internal/analysis/anomaly"
	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// AnomalyHandler handles /v1/anomalies requests
type AnomalyHandler struct {
	detector  *anomaly.AnomalyDetector
	logger    *logging.Logger
	validator *api.Validator
	tracer    trace.Tracer
}

// NewAnomalyHandler creates a new handler
func NewAnomalyHandler(graphClient graph.Client, logger *logging.Logger, tracer trace.Tracer) *AnomalyHandler {
	return &AnomalyHandler{
		detector:  anomaly.NewDetector(graphClient),
		logger:    logger,
		validator: api.NewValidator(),
		tracer:    tracer,
	}
}

// Handle processes anomaly detection requests
func (h *AnomalyHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	startTime := time.Now()

	var span trace.Span
	if h.tracer != nil {
		ctx, span = h.tracer.Start(ctx, "anomaly.Handle")
		defer span.End()
	}

	// 1. Parse query parameters
	input, err := h.parseInput(r)
	if err != nil {
		if span != nil {
			span.RecordError(err)
		}
		h.respondWithError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if span != nil {
		span.SetAttributes(
			attribute.String("resource_uid", input.ResourceUID),
			attribute.Int64("start", input.Start),
			attribute.Int64("end", input.End),
		)
	}

	h.logger.Debug("Processing anomaly request for resource %s, time range: %d to %d",
		input.ResourceUID, input.Start, input.End)

	// 2. Validate input
	if err := h.validateInput(input); err != nil {
		if span != nil {
			span.RecordError(err)
		}
		h.respondWithError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// 3. Execute anomaly detection
	result, err := h.detector.Detect(ctx, input)
	if err != nil {
		if span != nil {
			span.RecordError(err)
		}
		h.logger.Error("Anomaly detection failed: %v", err)
		h.respondWithError(w, http.StatusInternalServerError, "DETECTION_FAILED", err.Error())
		return
	}

	// 4. Add execution time to metadata
	result.Metadata.ExecutionTimeMs = time.Since(startTime).Milliseconds()

	h.logger.Debug("Anomaly detection completed: %d anomalies found in %dms",
		len(result.Anomalies), result.Metadata.ExecutionTimeMs)

	// 5. Return JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = api.WriteJSON(w, result)
}

func (h *AnomalyHandler) parseInput(r *http.Request) (anomaly.DetectInput, error) {
	query := r.URL.Query()

	// Required: resourceUID
	resourceUID := query.Get("resourceUID")
	if resourceUID == "" {
		return anomaly.DetectInput{}, api.NewValidationError("resourceUID is required")
	}

	// Required: start
	startStr := query.Get("start")
	if startStr == "" {
		return anomaly.DetectInput{}, api.NewValidationError("start is required")
	}
	start, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil {
		return anomaly.DetectInput{}, api.NewValidationError("invalid start: %v", err)
	}

	// Required: end
	endStr := query.Get("end")
	if endStr == "" {
		return anomaly.DetectInput{}, api.NewValidationError("end is required")
	}
	end, err := strconv.ParseInt(endStr, 10, 64)
	if err != nil {
		return anomaly.DetectInput{}, api.NewValidationError("invalid end: %v", err)
	}

	return anomaly.DetectInput{
		ResourceUID: resourceUID,
		Start:       start,
		End:         end,
	}, nil
}

func (h *AnomalyHandler) validateInput(input anomaly.DetectInput) error {
	// Validate resourceUID
	if input.ResourceUID == "" {
		return api.NewValidationError("resourceUID cannot be empty")
	}

	// Validate time range
	if input.End <= input.Start {
		return api.NewValidationError("end must be greater than start")
	}

	// Validate timestamps are positive
	if input.Start <= 0 {
		return api.NewValidationError("start must be positive")
	}
	if input.End <= 0 {
		return api.NewValidationError("end must be positive")
	}

	// Validate time range is reasonable (not more than 24 hours)
	const maxRangeSeconds = 24 * 60 * 60
	if input.End-input.Start > maxRangeSeconds {
		return api.NewValidationError("time range cannot exceed 24 hours")
	}

	return nil
}

func (h *AnomalyHandler) respondWithError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	api.WriteError(w, statusCode, errorCode, message)
}
