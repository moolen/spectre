package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// TimelineCompareHandler handles /v1/timeline/compare requests for A/B testing
type TimelineCompareHandler struct {
	storageExecutor QueryExecutor
	graphExecutor   QueryExecutor
	logger          *logging.Logger
}

// NewTimelineCompareHandler creates a new comparison handler
func NewTimelineCompareHandler(storageExecutor, graphExecutor QueryExecutor, logger *logging.Logger) *TimelineCompareHandler {
	return &TimelineCompareHandler{
		storageExecutor: storageExecutor,
		graphExecutor:   graphExecutor,
		logger:          logger,
	}
}

// ComparisonResult contains results from both executors
type ComparisonResult struct {
	StorageResult ComparisonMetrics `json:"storageResult"`
	GraphResult   ComparisonMetrics `json:"graphResult"`
	Comparison    Comparison        `json:"comparison"`
}

// ComparisonMetrics captures key metrics from a query
type ComparisonMetrics struct {
	ExecutionTimeMs int64            `json:"executionTimeMs"`
	EventCount      int              `json:"eventCount"`
	ResourceCount   int              `json:"resourceCount"`
	Error           string           `json:"error,omitempty"`
	Resources       []models.Resource `json:"resources,omitempty"`
}

// Comparison contains the delta between storage and graph results
type Comparison struct {
	TimeDeltaMs        int64   `json:"timeDeltaMs"`         // Difference in execution time
	EventCountDelta    int     `json:"eventCountDelta"`     // Difference in event count
	ResourceCountDelta int     `json:"resourceCountDelta"`  // Difference in resource count
	Accuracy           float64 `json:"accuracy"`            // Percentage match (0.0 to 1.0)
	Match              bool    `json:"match"`               // True if results are equivalent
}

// Handle executes the same query against both storage and graph, comparing results
func (h *TimelineCompareHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query (reusing timeline handler logic would be better)
	query, err := h.parseQuery(r)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Execute against both backends
	storageMetrics := h.executeQuery(ctx, h.storageExecutor, query)
	graphMetrics := h.executeQuery(ctx, h.graphExecutor, query)

	// Compare results
	comparison := h.compareResults(storageMetrics, graphMetrics)

	result := ComparisonResult{
		StorageResult: storageMetrics,
		GraphResult:   graphMetrics,
		Comparison:    comparison,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		h.logger.Error("Failed to encode comparison result: %v", err)
	}
}

// executeQuery runs a query and captures metrics
func (h *TimelineCompareHandler) executeQuery(ctx context.Context, executor QueryExecutor, query *models.QueryRequest) ComparisonMetrics {
	start := time.Now()

	result, err := executor.Execute(ctx, query)
	elapsed := time.Since(start)

	if err != nil {
		return ComparisonMetrics{
			ExecutionTimeMs: elapsed.Milliseconds(),
			Error:           err.Error(),
		}
	}

	// Build resources from events (simplified - would need full resource builder)
	resourceMap := make(map[string]bool)
	for _, event := range result.Events {
		resourceMap[event.Resource.UID] = true
	}

	return ComparisonMetrics{
		ExecutionTimeMs: elapsed.Milliseconds(),
		EventCount:      int(result.Count),
		ResourceCount:   len(resourceMap),
	}
}

// compareResults computes the delta between storage and graph results
func (h *TimelineCompareHandler) compareResults(storage, graph ComparisonMetrics) Comparison {
	eventDelta := graph.EventCount - storage.EventCount
	resourceDelta := graph.ResourceCount - storage.ResourceCount
	timeDelta := graph.ExecutionTimeMs - storage.ExecutionTimeMs

	// Calculate accuracy: percentage of events that match
	accuracy := 0.0
	if storage.EventCount > 0 {
		matchingEvents := storage.EventCount - abs(eventDelta)
		accuracy = float64(matchingEvents) / float64(storage.EventCount)
	}

	match := eventDelta == 0 && resourceDelta == 0 && storage.Error == "" && graph.Error == ""

	return Comparison{
		TimeDeltaMs:        timeDelta,
		EventCountDelta:    eventDelta,
		ResourceCountDelta: resourceDelta,
		Accuracy:           accuracy,
		Match:              match,
	}
}

// parseQuery extracts query parameters from request
func (h *TimelineCompareHandler) parseQuery(r *http.Request) (*models.QueryRequest, error) {
	query := r.URL.Query()

	// Parse start/end timestamps
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

	// Parse filters
	filters := models.QueryFilters{
		Kind:      query.Get("kind"),
		Namespace: query.Get("namespace"),
		Group:     query.Get("group"),
		Version:   query.Get("version"),
	}

	return &models.QueryRequest{
		StartTimestamp: start,
		EndTimestamp:   end,
		Filters:        filters,
	}, nil
}

// respondWithError writes an error response
func (h *TimelineCompareHandler) respondWithError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	})
}

// abs returns absolute value
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
