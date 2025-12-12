package api

import (
	"net/http"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"github.com/moolen/spectre/internal/storage"
	"go.opentelemetry.io/otel/trace"
)

// SearchHandler handles /v1/search requests
type SearchHandler struct {
	queryExecutor QueryExecutor
	logger        *logging.Logger
	validator     *Validator
	tracer        trace.Tracer
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(queryExecutor QueryExecutor, logger *logging.Logger, tracer trace.Tracer) *SearchHandler {
	return &SearchHandler{
		queryExecutor: queryExecutor,
		logger:        logger,
		validator:     NewValidator(),
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
	_ = writeJSON(w, searchResponse)

	sh.logger.Debug("Search completed: resources=%d, executionTime=%dms", searchResponse.Count, searchResponse.ExecutionTimeMs)
}

// buildSearchResponse transforms QueryResult into SearchResponse with ResourceBuilder
func (sh *SearchHandler) buildSearchResponse(queryResult *models.QueryResult) *models.SearchResponse {
	resourceBuilder := storage.NewResourceBuilder()
	resourceMap := resourceBuilder.BuildResourcesFromEvents(queryResult.Events)

	resources := make([]models.Resource, 0, len(resourceMap))
	for _, resource := range resourceMap {
		minimalResource := models.Resource{
			ID:        resource.ID,
			Group:     resource.Group,
			Version:   resource.Version,
			Kind:      resource.Kind,
			Namespace: resource.Namespace,
			Name:      resource.Name,
		}
		resources = append(resources, minimalResource)
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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]string{
		"error":   errorCode,
		"message": message,
	}

	_ = writeJSON(w, response)
}
