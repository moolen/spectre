package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/analysis"
	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// CausalGraphHandler handles /v1/causal-graph requests
type CausalGraphHandler struct {
	analyzer  *analysis.RootCauseAnalyzer
	logger    *logging.Logger
	validator *api.Validator
	tracer    trace.Tracer
}

// NewCausalGraphHandler creates a new handler
func NewCausalGraphHandler(graphClient graph.Client, logger *logging.Logger, tracer trace.Tracer) *CausalGraphHandler {
	return &CausalGraphHandler{
		analyzer:  analysis.NewRootCauseAnalyzer(graphClient),
		logger:    logger,
		validator: api.NewValidator(),
		tracer:    tracer,
	}
}

// Handle processes causal graph requests
func (h *CausalGraphHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Create tracing span
	var span trace.Span
	if h.tracer != nil {
		ctx, span = h.tracer.Start(ctx, "causal_graph.Handle")
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
			attribute.String("format", string(input.Format)),
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

	// 3. Execute analysis
	result, err := h.analyzer.Analyze(ctx, input)
	if err != nil {
		if span != nil {
			span.RecordError(err)
		}
		h.logger.Error("Causal graph analysis failed: %v", err)
		h.respondWithError(w, http.StatusInternalServerError, "ANALYSIS_FAILED", err.Error())
		return
	}

	// 4. Return JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = api.WriteJSON(w, result)
}

// parseInput extracts and normalizes query parameters
func (h *CausalGraphHandler) parseInput(r *http.Request) (analysis.AnalyzeInput, error) {
	query := r.URL.Query()

	// Required: resourceUID
	resourceUID := query.Get("resourceUID")
	if resourceUID == "" {
		return analysis.AnalyzeInput{}, api.NewValidationError("resourceUID is required")
	}

	// Extract UID from resource ID if it's in format: /group/version/kind/uid or group/version/kind/uid
	// The graph database stores only the UUID portion, not the full GVK path
	parts := strings.Split(resourceUID, "/")
	if len(parts) >= 4 {
		// Format: /v1/Pod/uuid or apps/v1/Deployment/uuid
		resourceUID = parts[len(parts)-1] // Take the last part (the UUID)
	}

	// Required: failureTimestamp
	failureTimestampStr := query.Get("failureTimestamp")
	if failureTimestampStr == "" {
		return analysis.AnalyzeInput{}, api.NewValidationError("failureTimestamp is required")
	}

	var failureTimestamp int64

	// Try parsing as RFC3339/RFC3339Nano format first (e.g., "2026-01-01T20:51:36.708229799+01:00")
	var parseErr error
	var parsedTime time.Time
	
	parsedTime, parseErr = time.Parse(time.RFC3339Nano, failureTimestampStr)
	if parseErr == nil {
		failureTimestamp = parsedTime.UnixNano()
	} else {
		// Try RFC3339 format (without nanoseconds)
		parsedTime, parseErr = time.Parse(time.RFC3339, failureTimestampStr)
		if parseErr == nil {
			failureTimestamp = parsedTime.UnixNano()
		} else {
			// Fall back to parsing as integer (backward compatibility)
			var err error
			failureTimestamp, err = strconv.ParseInt(failureTimestampStr, 10, 64)
			if err != nil {
				return analysis.AnalyzeInput{}, api.NewValidationError("invalid failureTimestamp: %v", err)
			}

			// Normalize timestamp to nanoseconds
			// If value is less than year 2000 in seconds (946684800), assume it's in seconds
			// If value is less than year 2000 in nanoseconds (946684800000000000), assume it's in milliseconds
			// Otherwise, assume it's already in nanoseconds
			if failureTimestamp < 946684800 {
				// Seconds to nanoseconds
				failureTimestamp *= 1_000_000_000
			} else if failureTimestamp < 946684800000000000 {
				// Milliseconds to nanoseconds
				failureTimestamp *= 1_000_000
			}
			// else: already in nanoseconds, use as-is
		}
	}

	// Optional: maxDepth (default 5)
	maxDepth := 5
	if maxDepthStr := query.Get("maxDepth"); maxDepthStr != "" {
		parsed, err := strconv.Atoi(maxDepthStr)
		if err != nil {
			return analysis.AnalyzeInput{}, api.NewValidationError("invalid maxDepth: %v", err)
		}
		maxDepth = parsed
	}

	// Optional: minConfidence (default 0.6)
	minConfidence := 0.6
	if minConfidenceStr := query.Get("minConfidence"); minConfidenceStr != "" {
		parsed, err := strconv.ParseFloat(minConfidenceStr, 64)
		if err != nil {
			return analysis.AnalyzeInput{}, api.NewValidationError("invalid minConfidence: %v", err)
		}
		minConfidence = parsed
	}

	// Optional: lookback duration (default 10 minutes)
	lookbackNs := int64(600_000_000_000) // 10 minutes in nanoseconds
	if lookbackStr := query.Get("lookback"); lookbackStr != "" {
		// Parse as duration string (e.g., "10m", "1h", "30s")
		duration, err := time.ParseDuration(lookbackStr)
		if err != nil {
			return analysis.AnalyzeInput{}, api.NewValidationError("invalid lookback duration: %v", err)
		}
		lookbackNs = duration.Nanoseconds()
	} else if lookbackMsStr := query.Get("lookbackMs"); lookbackMsStr != "" {
		// Alternative: milliseconds
		lookbackMs, err := strconv.ParseInt(lookbackMsStr, 10, 64)
		if err != nil {
			return analysis.AnalyzeInput{}, api.NewValidationError("invalid lookbackMs: %v", err)
		}
		lookbackNs = lookbackMs * 1_000_000
	}

	// Optional: format (default "diff" for new format with significance)
	format := analysis.FormatDiff
	if formatStr := query.Get("format"); formatStr != "" {
		switch formatStr {
		case "legacy":
			format = analysis.FormatLegacy
		case "diff":
			format = analysis.FormatDiff
		default:
			return analysis.AnalyzeInput{}, api.NewValidationError("invalid format: must be 'legacy' or 'diff'")
		}
	}

	return analysis.AnalyzeInput{
		ResourceUID:      resourceUID,
		FailureTimestamp: failureTimestamp,
		LookbackNs:       lookbackNs,
		MaxDepth:         maxDepth,
		MinConfidence:    minConfidence,
		Format:           format,
	}, nil
}

// validateInput validates the parsed input parameters
func (h *CausalGraphHandler) validateInput(input analysis.AnalyzeInput) error {
	// Validate resourceUID
	if input.ResourceUID == "" {
		return api.NewValidationError("resourceUID cannot be empty")
	}

	// Validate timestamp is positive
	if input.FailureTimestamp <= 0 {
		return api.NewValidationError("failureTimestamp must be positive")
	}

	// Validate maxDepth is reasonable
	if input.MaxDepth < 1 || input.MaxDepth > 20 {
		return api.NewValidationError("maxDepth must be between 1 and 20")
	}

	// Validate minConfidence is in valid range
	if input.MinConfidence < 0.0 || input.MinConfidence > 1.0 {
		return api.NewValidationError("minConfidence must be between 0.0 and 1.0")
	}

	return nil
}

// respondWithError writes an error response
func (h *CausalGraphHandler) respondWithError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	api.WriteError(w, statusCode, errorCode, message)
}
