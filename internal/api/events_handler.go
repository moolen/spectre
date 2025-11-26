package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/moritz/rpk/internal/logging"
	"github.com/moritz/rpk/internal/models"
	"github.com/moritz/rpk/internal/storage"
)

// EventsHandler handles /v1/resources/{resourceId}/events requests
type EventsHandler struct {
	queryExecutor *storage.QueryExecutor
	logger        *logging.Logger
}

// NewEventsHandler creates a new events handler
func NewEventsHandler(queryExecutor *storage.QueryExecutor, logger *logging.Logger) *EventsHandler {
	return &EventsHandler{
		queryExecutor: queryExecutor,
		logger:        logger,
	}
}

// Handle handles events requests
func (eh *EventsHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// Extract resource ID from path: /v1/resources/{resourceId}/events
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		eh.respondWithError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid resource ID")
		return
	}

	resourceID := parts[3]
	if resourceID == "" {
		eh.respondWithError(w, http.StatusBadRequest, "INVALID_REQUEST", "Resource ID cannot be empty")
		return
	}

	// Parse optional query parameters
	query := r.URL.Query()
	var startTime, endTime int64 = -1, -1
	limit := 100

	if startStr := query.Get("start"); startStr != "" {
		if val, err := strconv.ParseInt(startStr, 10, 64); err == nil {
			startTime = val
		}
	}

	if endStr := query.Get("end"); endStr != "" {
		if val, err := strconv.ParseInt(endStr, 10, 64); err == nil {
			endTime = val
		}
	}

	if limitStr := query.Get("limit"); limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil && val > 0 {
			limit = val
		}
	}

	// Query all events (use very large time range)
	queryReq := &models.QueryRequest{
		StartTimestamp: 0,
		EndTimestamp:   9999999999,
		Filters:        models.QueryFilters{},
	}

	queryResult, err := eh.queryExecutor.Execute(queryReq)
	if err != nil {
		eh.logger.Error("Failed to query events: %v", err)
		eh.respondWithError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch events")
		return
	}

	// Find resource with matching ID
	resourceBuilder := storage.NewResourceBuilder()
	resources := resourceBuilder.BuildResourcesFromEvents(queryResult.Events)

	if resource, exists := resources[resourceID]; exists {
		// Filter events by time range if provided
		events := resource.Events
		if startTime >= 0 || endTime >= 0 {
			events = eh.filterEvents(events, startTime, endTime)
		}

		// Apply limit
		if len(events) > limit {
			events = events[:limit]
		}

		response := models.EventsResponse{
			Events:     events,
			Count:      len(events),
			ResourceID: resourceID,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		writeJSON(w, response)
		return
	}

	eh.respondWithError(w, http.StatusNotFound, "NOT_FOUND", "Resource not found")
}

// filterEvents filters audit events by time range
func (eh *EventsHandler) filterEvents(events []models.AuditEvent, startTime, endTime int64) []models.AuditEvent {
	var filtered []models.AuditEvent
	for _, event := range events {
		// Check if event timestamp is within range
		if startTime >= 0 && event.Timestamp < startTime {
			continue
		}
		if endTime >= 0 && event.Timestamp > endTime {
			continue
		}
		filtered = append(filtered, event)
	}
	return filtered
}

// respondWithError sends an error response
func (eh *EventsHandler) respondWithError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]string{
		"error":   errorCode,
		"message": message,
	}

	writeJSON(w, response)
}
