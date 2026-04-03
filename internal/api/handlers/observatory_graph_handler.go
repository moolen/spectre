package handlers

import (
	"net/http"
	"strconv"

	observatorygraph "github.com/moolen/spectre/internal/analysis/observatory_graph"
	"github.com/moolen/spectre/internal/api"
	appgraph "github.com/moolen/spectre/internal/app/graph"
	"github.com/moolen/spectre/internal/logging"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ObservatoryGraphHandler handles /v1/observatory-graph requests
type ObservatoryGraphHandler struct {
	graphService *appgraph.Service
	logger       *logging.Logger
	tracer       trace.Tracer
}

// NewObservatoryGraphHandler creates a new handler
func NewObservatoryGraphHandler(graphService *appgraph.Service, logger *logging.Logger, tracer trace.Tracer) *ObservatoryGraphHandler {
	return &ObservatoryGraphHandler{
		graphService: graphService,
		logger:       logger,
		tracer:       tracer,
	}
}

// Handle processes observatory graph requests
func (h *ObservatoryGraphHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Create tracing span
	var span trace.Span
	if h.tracer != nil {
		ctx, span = h.tracer.Start(ctx, "observatory_graph.Handle")
		defer span.End()
	}

	// Parse query parameters
	input := h.parseInput(r)

	// Add span attributes for observability
	if span != nil {
		span.SetAttributes(
			attribute.String("integration", input.Integration),
			attribute.String("namespace", input.Namespace),
			attribute.String("workload", input.WorkloadName),
			attribute.Bool("include_baselines", input.IncludeBaselines),
			attribute.Int("limit", input.Limit),
		)
	}

	h.logger.Debug("Processing observatory graph request: integration=%s, namespace=%s, workload=%s",
		input.Integration, input.Namespace, input.WorkloadName)

	// Execute analysis
	result, err := h.graphService.AnalyzeObservatoryGraph(ctx, input)
	if err != nil {
		if span != nil {
			span.RecordError(err)
		}
		h.logger.Error("Observatory graph analysis failed: %v", err)
		h.respondWithError(w, http.StatusInternalServerError, "ANALYSIS_FAILED", err.Error())
		return
	}

	// Add result metrics to span
	if span != nil {
		span.SetAttributes(
			attribute.Int("nodes_returned", result.Metadata.NodeCount),
			attribute.Int("edges_returned", result.Metadata.EdgeCount),
			attribute.Int64("query_execution_ms", result.Metadata.QueryExecutionMs),
		)
	}

	h.logger.Debug("Observatory graph analysis completed: %d nodes, %d edges in %dms",
		result.Metadata.NodeCount, result.Metadata.EdgeCount, result.Metadata.QueryExecutionMs)

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = api.WriteJSON(w, result)
}

// parseInput extracts query parameters
func (h *ObservatoryGraphHandler) parseInput(r *http.Request) observatorygraph.AnalyzeInput {
	query := r.URL.Query()

	input := observatorygraph.AnalyzeInput{
		Integration:      query.Get("integration"),
		Namespace:        query.Get("namespace"),
		WorkloadName:     query.Get("workload"),
		IncludeBaselines: false,
		Limit:            observatorygraph.DefaultLimit,
	}

	// Parse includeBaselines
	if v := query.Get("includeBaselines"); v != "" {
		input.IncludeBaselines, _ = strconv.ParseBool(v)
	}

	// Parse limit
	if v := query.Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= observatorygraph.MaxLimit {
			input.Limit = parsed
		}
	}

	return input
}

// respondWithError writes an error response
func (h *ObservatoryGraphHandler) respondWithError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = api.WriteJSON(w, map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
