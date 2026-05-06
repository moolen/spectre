package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	namespacegraph "github.com/moolen/spectre/internal/analysis/namespace_graph"
	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/logging"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type namespaceGraphAnalyzer interface {
	Analyze(context.Context, namespacegraph.AnalyzeInput) (*namespacegraph.NamespaceGraphResponse, error)
}

// normalizeToNanoseconds accepts Unix timestamps in seconds, milliseconds, or nanoseconds.
func normalizeToNanoseconds(ts int64) int64 {
	switch {
	case ts < 100_000_000_000:
		return ts * int64(time.Second)
	case ts < 100_000_000_000_000:
		return ts * int64(time.Millisecond)
	default:
		return ts
	}
}

// NamespaceGraphHandler handles /v1/namespace-graph requests
type NamespaceGraphHandler struct {
	analyzer namespaceGraphAnalyzer
	logger   *logging.Logger
	tracer   trace.Tracer
}

// NewNamespaceGraphHandler creates a new handler.
func NewNamespaceGraphHandler(analyzer namespaceGraphAnalyzer, logger *logging.Logger, tracer trace.Tracer) *NamespaceGraphHandler {
	return &NamespaceGraphHandler{
		analyzer: analyzer,
		logger:   logger,
		tracer:   tracer,
	}
}

// Handle processes namespace graph requests
func (h *NamespaceGraphHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Create tracing span
	var span trace.Span
	if h.tracer != nil {
		ctx, span = h.tracer.Start(ctx, "namespace_graph.Handle")
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

	// 2. Validate input
	input, err = namespacegraph.PrepareAnalyzeInput(input, time.Now())
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
			attribute.String("namespace", input.Namespace),
			attribute.Int64("timestamp", input.Timestamp),
			attribute.Bool("include_anomalies", input.IncludeAnomalies),
			attribute.Bool("include_causal_paths", input.IncludeCausalPaths),
			attribute.Int("limit", input.Limit),
			attribute.Int("max_depth", input.MaxDepth),
		)
	}

	h.logger.Debug("Processing namespace graph request: namespace=%s, timestamp=%d",
		input.Namespace, input.Timestamp)

	// 3. Execute analysis
	result, err := h.analyzer.Analyze(ctx, input)
	if err != nil {
		if span != nil {
			span.RecordError(err)
		}
		h.logger.Error("Namespace graph analysis failed: %v", err)
		h.respondWithError(w, http.StatusInternalServerError, "ANALYSIS_FAILED", err.Error())
		return
	}

	// Add result metrics to span
	if span != nil {
		span.SetAttributes(
			attribute.Int("nodes_returned", result.Metadata.NodeCount),
			attribute.Int("edges_returned", result.Metadata.EdgeCount),
			attribute.Int64("query_execution_ms", result.Metadata.QueryExecutionMs),
			attribute.Bool("has_more", result.Metadata.HasMore),
			attribute.Bool("cache_hit", result.Metadata.Cached),
			attribute.Int64("cache_age_ms", result.Metadata.CacheAge),
		)
	}

	h.logger.Debug("Namespace graph analysis completed: %d nodes, %d edges in %dms",
		result.Metadata.NodeCount, result.Metadata.EdgeCount, result.Metadata.QueryExecutionMs)

	// 4. Return JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = api.WriteJSON(w, result)
}

// parseInput extracts and normalizes query parameters
func (h *NamespaceGraphHandler) parseInput(r *http.Request) (namespacegraph.AnalyzeInput, error) {
	query := r.URL.Query()

	// Required: namespace
	namespace := query.Get("namespace")
	if namespace == "" {
		return namespacegraph.AnalyzeInput{}, api.NewValidationError("namespace is required")
	}

	// Required: timestamp (supports both RFC3339 and Unix nanoseconds/milliseconds/seconds)
	timestampStr := query.Get("timestamp")
	if timestampStr == "" {
		return namespacegraph.AnalyzeInput{}, api.NewValidationError("timestamp is required")
	}
	timestamp, err := parseTimestampForNamespaceGraph(timestampStr)
	if err != nil {
		return namespacegraph.AnalyzeInput{}, api.NewValidationError("invalid timestamp: %v", err)
	}

	// Optional: includeAnomalies (default false)
	includeAnomalies := false
	if v := query.Get("includeAnomalies"); v != "" {
		includeAnomalies, _ = strconv.ParseBool(v)
	}

	// Optional: includeCausalPaths (default false)
	includeCausalPaths := false
	if v := query.Get("includeCausalPaths"); v != "" {
		includeCausalPaths, _ = strconv.ParseBool(v)
	}

	// Optional: lookback
	var lookback time.Duration
	if v := query.Get("lookback"); v != "" {
		if dur, err := time.ParseDuration(v); err == nil && dur > 0 {
			lookback = dur
		}
	}

	// Optional: maxDepth
	maxDepth := 0
	if v := query.Get("maxDepth"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			maxDepth = parsed
		}
	}

	// Optional: limit
	limit := 0
	if v := query.Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			limit = parsed
		}
	}

	// Optional: cursor for pagination
	cursor := query.Get("cursor")

	return namespacegraph.AnalyzeInput{
		Namespace:          namespace,
		Timestamp:          timestamp,
		IncludeAnomalies:   includeAnomalies,
		IncludeCausalPaths: includeCausalPaths,
		Lookback:           lookback,
		MaxDepth:           maxDepth,
		Limit:              limit,
		Cursor:             cursor,
	}, nil
}

// respondWithError writes an error response
func (h *NamespaceGraphHandler) respondWithError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = api.WriteJSON(w, map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

// parseTimestampForNamespaceGraph parses a timestamp string that can be either:
// - Unix nanoseconds (19 digits)
// - Unix milliseconds (13 digits)
// - Unix seconds (10 digits)
// - RFC3339 format string
func parseTimestampForNamespaceGraph(s string) (int64, error) {
	// Try parsing as integer first (Unix ns, ms, or seconds)
	if ts, err := strconv.ParseInt(s, 10, 64); err == nil {
		return normalizeToNanoseconds(ts), nil
	}

	// Try parsing as RFC3339
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		// Try RFC3339Nano as well
		t, err = time.Parse(time.RFC3339Nano, s)
		if err != nil {
			return 0, fmt.Errorf("timestamp must be Unix nanoseconds/milliseconds/seconds or RFC3339 format")
		}
	}
	return t.UnixNano(), nil
}
