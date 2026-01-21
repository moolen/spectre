package handlers

import (
	"net/http"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/logging"
	"go.opentelemetry.io/otel/trace"
)

// SearchHandler handles /v1/search requests
type SearchHandler struct {
	searchService *api.SearchService
	logger        *logging.Logger
	tracer        trace.Tracer
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(searchService *api.SearchService, logger *logging.Logger, tracer trace.Tracer) *SearchHandler {
	return &SearchHandler{
		searchService: searchService,
		logger:        logger,
		tracer:        tracer,
	}
}

// Handle handles search requests
func (sh *SearchHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract query parameters
	query := r.URL.Query()
	q := query.Get("q")
	startStr := query.Get("start")
	endStr := query.Get("end")

	// Build filters map
	filters := map[string]string{
		"group":     query.Get("group"),
		"version":   query.Get("version"),
		"kind":      query.Get("kind"),
		"namespace": query.Get("namespace"),
	}

	// Parse query using SearchService
	queryRequest, err := sh.searchService.ParseSearchQuery(q, startStr, endStr, filters)
	if err != nil {
		sh.logger.Warn("Invalid request: %v", err)
		sh.respondWithError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Execute search using SearchService
	result, err := sh.searchService.ExecuteSearch(ctx, queryRequest)
	if err != nil {
		sh.logger.Error("Query execution failed: %v", err)
		sh.respondWithError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to execute query")
		return
	}

	// Build response using SearchService
	searchResponse, err := sh.searchService.BuildSearchResponse(result)
	if err != nil {
		sh.logger.Error("Failed to build response: %v", err)
		sh.respondWithError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to build search response")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = api.WriteJSON(w, searchResponse)

	sh.logger.Debug("Search completed: resources=%d, executionTime=%dms", searchResponse.Count, searchResponse.ExecutionTimeMs)
}

// respondWithError sends an error response
func (sh *SearchHandler) respondWithError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	api.WriteError(w, statusCode, errorCode, message)
}
