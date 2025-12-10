package api

import (
	"net/http"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"github.com/moolen/spectre/internal/storage"
)

// TimelineHandler handles /v1/timeline requests
// Returns full resource data with statusSegments and events for timeline visualization
type TimelineHandler struct {
	queryExecutor QueryExecutor
	logger        *logging.Logger
	validator     *Validator
}

// NewTimelineHandler creates a new timeline handler
func NewTimelineHandler(queryExecutor QueryExecutor, logger *logging.Logger) *TimelineHandler {
	return &TimelineHandler{
		queryExecutor: queryExecutor,
		logger:        logger,
		validator:     NewValidator(),
	}
}

// Handle handles timeline requests
func (th *TimelineHandler) Handle(w http.ResponseWriter, r *http.Request) {
	query, err := th.parseQuery(r)
	if err != nil {
		th.logger.Warn("Invalid request: %v", err)
		th.respondWithError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	result, err := th.queryExecutor.Execute(query)
	if err != nil {
		th.logger.Error("Query execution failed: %v", err)
		th.respondWithError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to execute query")
		return
	}

	timelineResponse := th.buildTimelineResponse(result, query)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = writeJSON(w, timelineResponse)

	th.logger.Debug("Timeline completed: resources=%d, executionTime=%dms", timelineResponse.Count, timelineResponse.ExecutionTimeMs)
}

// buildTimelineResponse transforms QueryResult into TimelineResponse with full resource data
func (th *TimelineHandler) buildTimelineResponse(queryResult *models.QueryResult, query *models.QueryRequest) *models.SearchResponse {
	resourceBuilder := storage.NewResourceBuilder()
	resourceMap := resourceBuilder.BuildResourcesFromEvents(queryResult.Events)

	th.attachK8sEvents(resourceBuilder, resourceMap, query)

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

// Attaches Kubernetes Events to resources for timeline dots and detail panel
func (th *TimelineHandler) attachK8sEvents(resourceBuilder *storage.ResourceBuilder, resourceMap map[string]*models.Resource, query *models.QueryRequest) {
	if len(resourceMap) == 0 {
		return
	}

	eventQuery := &models.QueryRequest{
		StartTimestamp: query.StartTimestamp,
		EndTimestamp:   query.EndTimestamp,
		Filters: models.QueryFilters{
			Kind:      "Event",
			Version:   "v1",
			Namespace: query.Filters.Namespace,
		},
	}

	eventResult, err := th.queryExecutor.Execute(eventQuery)
	if err != nil {
		th.logger.Warn("Failed to fetch Kubernetes events for timeline: %v", err)
		return
	}

	resourceBuilder.AttachK8sEvents(resourceMap, eventResult.Events)
}

// parseQuery parses and validates query parameters (same as SearchHandler)
func (th *TimelineHandler) parseQuery(r *http.Request) (*models.QueryRequest, error) {
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

	if err := th.validator.ValidateFilters(filters); err != nil {
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

func (th *TimelineHandler) respondWithError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]string{
		"error":   errorCode,
		"message": message,
	}

	_ = writeJSON(w, response)
}
