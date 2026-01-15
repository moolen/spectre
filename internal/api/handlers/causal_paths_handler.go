package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	causalpaths "github.com/moolen/spectre/internal/analysis/causal_paths"
	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// CausalPathsHandler handles /v1/causal-paths requests
type CausalPathsHandler struct {
	discoverer *causalpaths.PathDiscoverer
	logger     *logging.Logger
	validator  *api.Validator
	tracer     trace.Tracer
}

// NewCausalPathsHandler creates a new handler
func NewCausalPathsHandler(graphClient graph.Client, logger *logging.Logger, tracer trace.Tracer) *CausalPathsHandler {
	return &CausalPathsHandler{
		discoverer: causalpaths.NewPathDiscoverer(graphClient),
		logger:     logger,
		validator:  api.NewValidator(),
		tracer:     tracer,
	}
}

// Handle processes causal paths requests
func (h *CausalPathsHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Create tracing span
	var span trace.Span
	if h.tracer != nil {
		ctx, span = h.tracer.Start(ctx, "causal_paths.Handle")
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

	// Add span attributes for observability
	if span != nil {
		span.SetAttributes(
			attribute.String("resource_uid", input.ResourceUID),
			attribute.Int64("failure_timestamp", input.FailureTimestamp),
			attribute.Int("max_depth", input.MaxDepth),
			attribute.Int("max_paths", input.MaxPaths),
		)
	}

	// 2. Validate input
	if err := h.validateInput(input); err != nil {
		if span != nil {
			span.RecordError(err)
		}
		h.respondWithError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// 3. Execute path discovery
	result, err := h.discoverer.DiscoverCausalPaths(ctx, input)
	if err != nil {
		// Check if this is a "no data in range" error - return 200 with hint instead of 500
		var noDataErr *analysis.ErrNoChangeEventInRange
		if errors.As(err, &noDataErr) {
			// Return HTTP 200 with empty paths and a helpful hint
			result = &causalpaths.CausalPathsResponse{
				Paths: []causalpaths.CausalPath{},
				Metadata: causalpaths.ResponseMetadata{
					QueryExecutionMs: 0,
					AlgorithmVersion: causalpaths.AlgorithmVersion,
					ExecutedAt:       time.Now(),
					NodesExplored:    0,
					PathsDiscovered:  0,
					PathsReturned:    0,
				},
				Hint: noDataErr.Hint(),
			}
			if span != nil {
				span.SetAttributes(
					attribute.String("hint", noDataErr.Hint()),
					attribute.Bool("no_data_in_range", true),
				)
			}
			h.logger.Debug("No data in requested time range: %v", err)
		} else {
			if span != nil {
				span.RecordError(err)
			}
			h.logger.Error("Causal paths discovery failed: %v", err)
			h.respondWithError(w, http.StatusInternalServerError, "DISCOVERY_FAILED", err.Error())
			return
		}
	}

	// Add result metrics to span
	if span != nil {
		span.SetAttributes(
			attribute.Int("paths_discovered", result.Metadata.PathsDiscovered),
			attribute.Int("paths_returned", result.Metadata.PathsReturned),
			attribute.Int("nodes_explored", result.Metadata.NodesExplored),
			attribute.Int64("query_execution_ms", result.Metadata.QueryExecutionMs),
		)
	}

	// 4. Return JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = api.WriteJSON(w, result)
}

// parseInput extracts and normalizes query parameters
func (h *CausalPathsHandler) parseInput(r *http.Request) (causalpaths.CausalPathsInput, error) {
	query := r.URL.Query()

	// Required: resourceUID
	resourceUID := query.Get("resourceUID")
	if resourceUID == "" {
		return causalpaths.CausalPathsInput{}, api.NewValidationError("resourceUID is required")
	}

	// Required: failureTimestamp
	failureTimestampStr := query.Get("failureTimestamp")
	if failureTimestampStr == "" {
		return causalpaths.CausalPathsInput{}, api.NewValidationError("failureTimestamp is required")
	}
	failureTimestamp, err := strconv.ParseInt(failureTimestampStr, 10, 64)
	if err != nil {
		return causalpaths.CausalPathsInput{}, api.NewValidationError("invalid failureTimestamp format")
	}

	// Normalize timestamp to nanoseconds
	failureTimestamp = normalizeToNanoseconds(failureTimestamp)

	// Optional: maxDepth (default 5, range 1-10)
	maxDepth := causalpaths.DefaultMaxDepth
	if v := query.Get("maxDepth"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			if parsed >= causalpaths.MinMaxDepth && parsed <= causalpaths.MaxMaxDepth {
				maxDepth = parsed
			}
		}
	}

	// Optional: maxPaths (default 5, range 1-20)
	maxPaths := causalpaths.DefaultMaxPaths
	if v := query.Get("maxPaths"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			if parsed >= causalpaths.MinMaxPaths && parsed <= causalpaths.MaxMaxPaths {
				maxPaths = parsed
			}
		}
	}

	// Optional: lookback (default 10m)
	lookbackNs := causalpaths.DefaultLookbackNs
	if v := query.Get("lookback"); v != "" {
		if dur, err := time.ParseDuration(v); err == nil && dur > 0 {
			lookbackNs = dur.Nanoseconds()
		}
	}

	return causalpaths.CausalPathsInput{
		ResourceUID:      resourceUID,
		FailureTimestamp: failureTimestamp,
		LookbackNs:       lookbackNs,
		MaxDepth:         maxDepth,
		MaxPaths:         maxPaths,
	}, nil
}

// validateInput validates the parsed input
func (h *CausalPathsHandler) validateInput(input causalpaths.CausalPathsInput) error {
	if input.ResourceUID == "" {
		return api.NewValidationError("resourceUID cannot be empty")
	}
	if input.FailureTimestamp <= 0 {
		return api.NewValidationError("failureTimestamp must be positive")
	}
	return nil
}

// respondWithError writes an error response
func (h *CausalPathsHandler) respondWithError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = api.WriteJSON(w, map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

// normalizeToNanoseconds converts timestamp to nanoseconds based on magnitude
func normalizeToNanoseconds(ts int64) int64 {
	// Nanoseconds: 19 digits (e.g., 1704067200000000000)
	// Milliseconds: 13 digits (e.g., 1704067200000)
	// Seconds: 10 digits (e.g., 1704067200)
	switch {
	case ts > 1e18: // Already nanoseconds
		return ts
	case ts > 1e12: // Milliseconds
		return ts * 1_000_000
	default: // Seconds
		return ts * 1_000_000_000
	}
}
