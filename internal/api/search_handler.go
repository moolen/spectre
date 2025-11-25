package api

import (
	"net/http"
	"strconv"

	dps "github.com/markusmobius/go-dateparser"
	"github.com/moritz/rpk/internal/logging"
	"github.com/moritz/rpk/internal/models"
	"github.com/moritz/rpk/internal/storage"
)

// SearchHandler handles /v1/search requests
type SearchHandler struct {
	queryExecutor *storage.QueryExecutor
	logger        *logging.Logger
	validator     *Validator
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(queryExecutor *storage.QueryExecutor, logger *logging.Logger) *SearchHandler {
	return &SearchHandler{
		queryExecutor: queryExecutor,
		logger:        logger,
		validator:     NewValidator(),
	}
}

// Handle handles search requests
func (sh *SearchHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query, err := sh.parseQuery(r)
	if err != nil {
		sh.logger.Warn("Invalid request: %v", err)
		sh.respondWithError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	// Execute query
	result, err := sh.queryExecutor.Execute(query)
	if err != nil {
		sh.logger.Error("Query execution failed: %v", err)
		sh.respondWithError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to execute query")
		return
	}

	// Respond with results
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	writeJSON(w, result)

	sh.logger.Debug("Search completed: events=%d, executionTime=%dms", result.Count, result.ExecutionTimeMs)
}

// parseQuery parses and validates query parameters
func (sh *SearchHandler) parseQuery(r *http.Request) (*models.QueryRequest, error) {
	// Get query parameters
	query := r.URL.Query()

	// Parse start timestamp (required)
	startStr := query.Get("start")
	if startStr == "" {
		return nil, NewValidationError("start timestamp is required")
	}
	start, err := sh.parseTimestamp(startStr, "start")
	if err != nil {
		return nil, err
	}

	// Parse end timestamp (required)
	endStr := query.Get("end")
	if endStr == "" {
		return nil, NewValidationError("end timestamp is required")
	}
	end, err := sh.parseTimestamp(endStr, "end")
	if err != nil {
		return nil, err
	}

	// Validate timestamps
	if start < 0 || end < 0 {
		return nil, NewValidationError("timestamps must be non-negative")
	}
	if start > end {
		return nil, NewValidationError("start timestamp must be less than or equal to end timestamp")
	}

	// Parse optional filters
	filters := models.QueryFilters{
		Group:     query.Get("group"),
		Version:   query.Get("version"),
		Kind:      query.Get("kind"),
		Namespace: query.Get("namespace"),
	}

	// Validate filters
	if err := sh.validator.ValidateFilters(filters); err != nil {
		return nil, err
	}

	// Create query request
	queryRequest := &models.QueryRequest{
		StartTimestamp: start,
		EndTimestamp:   end,
		Filters:        filters,
	}

	// Validate the full query
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

	writeJSON(w, response)
}

// parseTimestamp parses a timestamp string, supporting both Unix timestamps and human-readable dates
func (sh *SearchHandler) parseTimestamp(timestampStr, fieldName string) (int64, error) {
	// First, try parsing as Unix timestamp (for backward compatibility)
	if unixTimestamp, err := strconv.ParseInt(timestampStr, 10, 64); err == nil {
		if unixTimestamp < 0 {
			return 0, NewValidationError("%s timestamp must be non-negative", fieldName)
		}
		return unixTimestamp, nil
	}

	// If not a valid integer, try parsing as human-readable date
	parser := dps.Parser{}
	cfg := &dps.Configuration{
		// Use CurrentPeriod as default to interpret dates like "March" as current period
		// This is more intuitive for search queries
		PreferredDateSource: dps.CurrentPeriod,
	}

	parsedDate, err := parser.Parse(cfg, timestampStr)
	if err != nil {
		return 0, NewValidationError("%s must be a valid Unix timestamp or human-readable date: %v", fieldName, err)
	}

	if parsedDate.IsZero() {
		return 0, NewValidationError("%s could not be parsed as a valid date: %s", fieldName, timestampStr)
	}

	// Convert to Unix seconds
	return parsedDate.Time.Unix(), nil
}
