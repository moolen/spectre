package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	namespacegraph "github.com/moolen/spectre/internal/analysis/namespace_graph"
	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// TimestampBucketSize is the bucket size for timestamp normalization.
// All timestamps are rounded down to the nearest 30-second boundary.
// This ensures all requests within a 30s window get identical responses.
const TimestampBucketSize = 30 * time.Second

// bucketTimestamp rounds a timestamp (in nanoseconds) down to the nearest bucket boundary.
// This normalizes "now" requests so multiple clients get the same cached snapshot.
func bucketTimestamp(ts int64) int64 {
	bucketNs := int64(TimestampBucketSize)
	return (ts / bucketNs) * bucketNs
}

// NamespaceGraphHandler handles /v1/namespace-graph requests
type NamespaceGraphHandler struct {
	analyzer  *namespacegraph.Analyzer
	cache     *namespacegraph.Cache
	logger    *logging.Logger
	validator *api.Validator
	tracer    trace.Tracer
}

// NewNamespaceGraphHandler creates a new handler without caching
func NewNamespaceGraphHandler(graphClient graph.Client, logger *logging.Logger, tracer trace.Tracer) *NamespaceGraphHandler {
	return &NamespaceGraphHandler{
		analyzer:  namespacegraph.NewAnalyzer(graphClient),
		logger:    logger,
		validator: api.NewValidator(),
		tracer:    tracer,
	}
}

// NewNamespaceGraphHandlerWithCache creates a new handler with caching enabled
func NewNamespaceGraphHandlerWithCache(graphClient graph.Client, cache *namespacegraph.Cache, logger *logging.Logger, tracer trace.Tracer) *NamespaceGraphHandler {
	return &NamespaceGraphHandler{
		analyzer:  namespacegraph.NewAnalyzer(graphClient),
		cache:     cache,
		logger:    logger,
		validator: api.NewValidator(),
		tracer:    tracer,
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

	// 2. Validate input
	if err := h.validateInput(input); err != nil {
		if span != nil {
			span.RecordError(err)
		}
		h.respondWithError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	h.logger.Debug("Processing namespace graph request: namespace=%s, timestamp=%d",
		input.Namespace, input.Timestamp)

	// 3. Execute analysis (use cache if available)
	var result *namespacegraph.NamespaceGraphResponse

	if h.cache != nil {
		result, err = h.cache.Analyze(ctx, input)
	} else {
		result, err = h.analyzer.Analyze(ctx, input)
	}

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

	// Bucket timestamp to 30s intervals for cache efficiency
	// All requests within a 30s window get the same cached snapshot
	timestamp = bucketTimestamp(timestamp)

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

	// Optional: lookback (default 10m)
	lookback := namespacegraph.DefaultLookback
	if v := query.Get("lookback"); v != "" {
		if dur, err := time.ParseDuration(v); err == nil && dur > 0 {
			lookback = dur
		}
	}

	// Optional: maxDepth (default 3, range 1-10)
	maxDepth := namespacegraph.DefaultMaxDepth
	if v := query.Get("maxDepth"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			if parsed >= namespacegraph.MinMaxDepth && parsed <= namespacegraph.MaxMaxDepth {
				maxDepth = parsed
			}
		}
	}

	// Optional: limit (default 100, max 500)
	limit := namespacegraph.DefaultLimit
	if v := query.Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			if parsed > 0 && parsed <= namespacegraph.MaxLimit {
				limit = parsed
			}
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

// validateInput validates the parsed input
func (h *NamespaceGraphHandler) validateInput(input namespacegraph.AnalyzeInput) error {
	if input.Namespace == "" {
		return api.NewValidationError("namespace cannot be empty")
	}

	// Validate namespace length (Kubernetes limit is 63 characters)
	if len(input.Namespace) > 63 {
		return api.NewValidationError("namespace must be 63 characters or less")
	}

	if input.Timestamp <= 0 {
		return api.NewValidationError("timestamp must be positive")
	}

	// Validate timestamp is not in the future (with some tolerance)
	now := time.Now().UnixNano()
	if input.Timestamp > now+int64(time.Hour) {
		return api.NewValidationError("timestamp cannot be more than 1 hour in the future")
	}

	return nil
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
