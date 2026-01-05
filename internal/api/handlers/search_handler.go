package handlers

import (
	"fmt"
	"net/http"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel/trace"
)

// SearchHandler handles /v1/search requests
type SearchHandler struct {
	queryExecutor api.QueryExecutor
	logger        *logging.Logger
	validator     *api.Validator
	tracer        trace.Tracer
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(queryExecutor api.QueryExecutor, logger *logging.Logger, tracer trace.Tracer) *SearchHandler {
	return &SearchHandler{
		queryExecutor: queryExecutor,
		logger:        logger,
		validator:     api.NewValidator(),
		tracer:        tracer,
	}
}

// Handle handles search requests
func (sh *SearchHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	query, err := sh.parseQuery(r)
	if err != nil {
		sh.logger.Warn("Invalid request: %v", err)
		sh.respondWithError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	result, err := sh.queryExecutor.Execute(ctx, query)
	if err != nil {
		sh.logger.Error("Query execution failed: %v", err)
		sh.respondWithError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to execute query")
		return
	}

	searchResponse := sh.buildSearchResponse(result)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = api.WriteJSON(w, searchResponse)

	sh.logger.Debug("Search completed: resources=%d, executionTime=%dms", searchResponse.Count, searchResponse.ExecutionTimeMs)
}

// buildSearchResponse transforms QueryResult into SearchResponse
// TODO: Reimplement ResourceBuilder functionality for graph-based queries
func (sh *SearchHandler) buildSearchResponse(queryResult *models.QueryResult) *models.SearchResponse {
	// Build resources directly from events (simplified version)
	resourceMap := make(map[string]*models.Resource)
	for _, event := range queryResult.Events {
		resourceID := fmt.Sprintf("%s/%s/%s/%s", event.Resource.Group, event.Resource.Version, event.Resource.Kind, event.Resource.UID)
		if _, exists := resourceMap[resourceID]; !exists {
			resourceMap[resourceID] = &models.Resource{
				ID:        resourceID,
				Group:     event.Resource.Group,
				Version:   event.Resource.Version,
				Kind:      event.Resource.Kind,
				Namespace: event.Resource.Namespace,
				Name:      event.Resource.Name,
			}
		}
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

// parseQuery parses and validates query parameters
func (sh *SearchHandler) parseQuery(r *http.Request) (*models.QueryRequest, error) {
	query := r.URL.Query()

	startStr := query.Get("start")
	start, err := api.ParseTimestamp(startStr, "start")
	if err != nil {
		return nil, err
	}

	endStr := query.Get("end")
	end, err := api.ParseTimestamp(endStr, "end")
	if err != nil {
		return nil, err
	}

	if start < 0 || end < 0 {
		return nil, api.NewValidationError("timestamps must be non-negative")
	}
	if start > end {
		return nil, api.NewValidationError("start timestamp must be less than or equal to end timestamp")
	}

	filters := models.QueryFilters{
		Group:     query.Get("group"),
		Version:   query.Get("version"),
		Kind:      query.Get("kind"),
		Namespace: query.Get("namespace"),
	}

	if err := sh.validator.ValidateFilters(filters); err != nil {
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

// respondWithError sends an error response
func (sh *SearchHandler) respondWithError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	api.WriteError(w, statusCode, errorCode, message)
}
