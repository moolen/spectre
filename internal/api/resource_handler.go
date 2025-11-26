package api

import (
	"math"
	"net/http"
	"strings"

	"github.com/moritz/rpk/internal/logging"
	"github.com/moritz/rpk/internal/models"
	"github.com/moritz/rpk/internal/storage"
)

// ResourceHandler handles /v1/resources/{resourceId} requests
type ResourceHandler struct {
	queryExecutor *storage.QueryExecutor
	logger        *logging.Logger
}

// NewResourceHandler creates a new resource handler
func NewResourceHandler(queryExecutor *storage.QueryExecutor, logger *logging.Logger) *ResourceHandler {
	return &ResourceHandler{
		queryExecutor: queryExecutor,
		logger:        logger,
	}
}

// Handle handles resource detail requests
func (rh *ResourceHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// Extract resource ID from path: /v1/resources/{resourceId}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		rh.respondWithError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid resource ID")
		return
	}

	resourceID := parts[3]
	if resourceID == "" {
		rh.respondWithError(w, http.StatusBadRequest, "INVALID_REQUEST", "Resource ID cannot be empty")
		return
	}

	// Query all events (use very large time range)
	query := &models.QueryRequest{
		StartTimestamp: 0,
		EndTimestamp:   math.MaxInt64,
		Filters:        models.QueryFilters{},
	}

	queryResult, err := rh.queryExecutor.Execute(query)
	if err != nil {
		rh.logger.Error("Failed to query events: %v", err)
		rh.respondWithError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch resource")
		return
	}

	// Find resource with matching ID
	resourceBuilder := storage.NewResourceBuilder()
	resources := resourceBuilder.BuildResourcesFromEvents(queryResult.Events)

	if resource, exists := resources[resourceID]; exists {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeJSON(w, resource)
		return
	}

	rh.respondWithError(w, http.StatusNotFound, "NOT_FOUND", "Resource not found")
}

// respondWithError sends an error response
func (rh *ResourceHandler) respondWithError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]string{
		"error":   errorCode,
		"message": message,
	}

	writeJSON(w, response)
}
