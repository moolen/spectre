package api

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// ExportHandler handles event export requests using the graph query executor
type ExportHandler struct {
	queryExecutor QueryExecutor
	logger        *logging.Logger
}

// NewExportHandler creates a new export handler
func NewExportHandler(queryExecutor QueryExecutor, logger *logging.Logger) *ExportHandler {
	return &ExportHandler{
		queryExecutor: queryExecutor,
		logger:        logger,
	}
}

// Handle processes export requests
func (h *ExportHandler) Handle(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Parse query parameters
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	clusterID := r.URL.Query().Get("cluster_id")
	instanceID := r.URL.Query().Get("instance_id")

	// Parse and validate time range
	from, err := strconv.ParseInt(fromStr, 10, 64)
	if err != nil {
		h.logger.Error("Invalid 'from' parameter: %v", err)
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid 'from' parameter: must be Unix timestamp in seconds")
		return
	}

	to, err := strconv.ParseInt(toStr, 10, 64)
	if err != nil {
		h.logger.Error("Invalid 'to' parameter: %v", err)
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid 'to' parameter: must be Unix timestamp in seconds")
		return
	}

	// Validate time range
	if from < 0 || to < 0 {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "Timestamps must be non-negative")
		return
	}

	if from > to {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "Start timestamp must be less than or equal to end timestamp")
		return
	}

	h.logger.InfoWithFields("Starting export request",
		logging.Field("from", from),
		logging.Field("to", to),
		logging.Field("cluster_id", clusterID),
		logging.Field("instance_id", instanceID),
		logging.Field("remote_addr", r.RemoteAddr))

	// Create query request with empty filters to get ALL events
	query := &models.QueryRequest{
		StartTimestamp: from,
		EndTimestamp:   to,
		Filters:        models.QueryFilters{}, // Empty filters = all events
	}

	// Validate query request
	if err := query.Validate(); err != nil {
		h.logger.Error("Invalid query request: %v", err)
		writeError(w, http.StatusBadRequest, "INVALID_QUERY", err.Error())
		return
	}

	// Execute query with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	h.logger.DebugWithFields("Executing export query (fetching all events)",
		logging.Field("start_timestamp", from),
		logging.Field("end_timestamp", to))

	queryStartTime := time.Now()

	// Collect all events by paginating through all pages
	// Use MaxPageSize to minimize the number of queries needed
	var allEvents []models.Event
	var cursor string
	pageCount := 0
	maxPageSize := models.MaxPageSize // 500 resources per page

	// Check if executor supports ExecutePaginated (graph executor)
	type paginatedExecutor interface {
		ExecutePaginated(context.Context, *models.QueryRequest, *models.PaginationRequest) (*models.QueryResult, *models.PaginationResponse, error)
	}

	paginatedExec, supportsPagination := h.queryExecutor.(paginatedExecutor)

	for {
		pageCount++
		var result *models.QueryResult
		var paginationResp *models.PaginationResponse
		var err error

		if supportsPagination {
			// Use paginated execution with large page size
			pagination := &models.PaginationRequest{
				PageSize: maxPageSize,
				Cursor:   cursor,
			}
			result, paginationResp, err = paginatedExec.ExecutePaginated(ctx, query, pagination)
		} else {
			// Fallback to Execute (will only get first page)
			result, err = h.queryExecutor.Execute(ctx, query)
			if err == nil {
				// Create a dummy pagination response indicating no more pages
				paginationResp = &models.PaginationResponse{
					HasMore: false,
				}
			}
		}

		if err != nil {
			queryDuration := time.Since(queryStartTime)
			h.logger.ErrorWithFields("Export query failed",
				logging.Field("error", err),
				logging.Field("query_duration", queryDuration),
				logging.Field("page", pageCount))
			writeError(w, http.StatusInternalServerError, "QUERY_FAILED", fmt.Sprintf("Failed to execute export query: %v", err))
			return
		}

		// Collect events from this page
		allEvents = append(allEvents, result.Events...)
		h.logger.DebugWithFields("Fetched export page",
			logging.Field("page", pageCount),
			logging.Field("events_this_page", len(result.Events)),
			logging.Field("total_events_so_far", len(allEvents)),
			logging.Field("has_more", paginationResp != nil && paginationResp.HasMore))

		// Check if there are more pages
		if paginationResp == nil || !paginationResp.HasMore || paginationResp.NextCursor == "" {
			break
		}

		// Continue to next page
		cursor = paginationResp.NextCursor
	}

	queryDuration := time.Since(queryStartTime)
	h.logger.InfoWithFields("Export query completed (all pages)",
		logging.Field("total_events", len(allEvents)),
		logging.Field("pages_fetched", pageCount),
		logging.Field("query_duration", queryDuration))

	// Convert []Event to []*Event for JSON export (matching import format)
	events := make([]*models.Event, len(allEvents))
	for i := range allEvents {
		events[i] = &allEvents[i]
	}

	// Create export response matching import format
	exportData := map[string]interface{}{
		"events": events,
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Encoding", "gzip")
	
	// Generate filename with timestamp
	filename := fmt.Sprintf("export-%d-%d.json.gz", from, to)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	// Write gzipped JSON response
	gzipWriter := gzip.NewWriter(w)
	defer func() {
		if err := gzipWriter.Close(); err != nil {
			h.logger.Error("Failed to close gzip writer: %v", err)
		}
	}()

	encoder := json.NewEncoder(gzipWriter)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(exportData); err != nil {
		h.logger.ErrorWithFields("Failed to write export response",
			logging.Field("error", err),
			logging.Field("event_count", len(events)))
		// Response may have been partially written, can't send error response
		return
	}

	duration := time.Since(startTime)
	h.logger.InfoWithFields("Export completed",
		logging.Field("total_events", len(events)),
		logging.Field("duration", duration))
}
