package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/moritz/rpk/internal/logging"
	"github.com/moritz/rpk/internal/models"
	"github.com/moritz/rpk/internal/storage"
)

// SegmentsHandler handles /v1/resources/{resourceId}/segments requests
type SegmentsHandler struct {
	queryExecutor *storage.QueryExecutor
	logger        *logging.Logger
}

// NewSegmentsHandler creates a new segments handler
func NewSegmentsHandler(queryExecutor *storage.QueryExecutor, logger *logging.Logger) *SegmentsHandler {
	return &SegmentsHandler{
		queryExecutor: queryExecutor,
		logger:        logger,
	}
}

// Handle handles segments requests
func (sh *SegmentsHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// Extract resource ID from path: /v1/resources/{resourceId}/segments
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		sh.respondWithError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid resource ID")
		return
	}

	resourceID := parts[3]
	if resourceID == "" {
		sh.respondWithError(w, http.StatusBadRequest, "INVALID_REQUEST", "Resource ID cannot be empty")
		return
	}

	// Parse optional query parameters
	params := r.URL.Query()
	var startTime, endTime int64 = -1, -1

	if startStr := params.Get("start"); startStr != "" {
		if val, err := strconv.ParseInt(startStr, 10, 64); err == nil {
			startTime = val
		}
	}

	if endStr := params.Get("end"); endStr != "" {
		if val, err := strconv.ParseInt(endStr, 10, 64); err == nil {
			endTime = val
		}
	}

	// Query all events (use very large time range)
	query := &models.QueryRequest{
		StartTimestamp: 0,
		EndTimestamp:   9999999999,
		Filters:        models.QueryFilters{},
	}

	queryResult, err := sh.queryExecutor.Execute(query)
	if err != nil {
		sh.logger.Error("Failed to query events: %v", err)
		sh.respondWithError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch segments")
		return
	}

	// Find resource with matching ID
	resourceBuilder := storage.NewResourceBuilder()
	resources := resourceBuilder.BuildResourcesFromEvents(queryResult.Events)

	if resource, exists := resources[resourceID]; exists {
		// Filter segments by time range if provided
		segments := resource.StatusSegments
		if startTime >= 0 || endTime >= 0 {
			segments = sh.filterSegments(segments, startTime, endTime)
		}

		response := models.SegmentsResponse{
			Segments:   segments,
			ResourceID: resourceID,
			Count:      len(segments),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeJSON(w, response)
		return
	}

	sh.respondWithError(w, http.StatusNotFound, "NOT_FOUND", "Resource not found")
}

// filterSegments filters status segments by time range
func (sh *SegmentsHandler) filterSegments(segments []models.StatusSegment, startTime, endTime int64) []models.StatusSegment {
	var filtered []models.StatusSegment
	for _, segment := range segments {
		// Check if segment overlaps with time range
		if startTime >= 0 && segment.EndTime < startTime {
			continue
		}
		if endTime >= 0 && segment.StartTime > endTime {
			continue
		}
		filtered = append(filtered, segment)
	}
	return filtered
}

// respondWithError sends an error response
func (sh *SegmentsHandler) respondWithError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]string{
		"error":   errorCode,
		"message": message,
	}

	writeJSON(w, response)
}
