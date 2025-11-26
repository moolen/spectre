package api

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/moritz/rpk/internal/logging"
	"github.com/moritz/rpk/internal/models"
	"github.com/moritz/rpk/internal/storage"
)

// MetadataHandler handles /v1/metadata requests
type MetadataHandler struct {
	queryExecutor *storage.QueryExecutor
	logger        *logging.Logger
}

// NewMetadataHandler creates a new metadata handler
func NewMetadataHandler(queryExecutor *storage.QueryExecutor, logger *logging.Logger) *MetadataHandler {
	return &MetadataHandler{
		queryExecutor: queryExecutor,
		logger:        logger,
	}
}

// Handle handles metadata requests
func (mh *MetadataHandler) Handle(w http.ResponseWriter, r *http.Request) {
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

	// Query all events (use very large time range if not specified)
	if startTime < 0 {
		startTime = 0
	}
	if endTime < 0 {
		endTime = 9999999999
	}

	query := &models.QueryRequest{
		StartTimestamp: startTime,
		EndTimestamp:   endTime,
		Filters:        models.QueryFilters{},
	}

	queryResult, err := mh.queryExecutor.Execute(query)
	if err != nil {
		mh.logger.Error("Failed to query events: %v", err)
		mh.respondWithError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch metadata")
		return
	}

	// Extract unique namespaces, kinds, and groups
	namespaces := make(map[string]bool)
	kinds := make(map[string]bool)
	groups := make(map[string]bool)
	resourceCounts := make(map[string]int)
	var minTime, maxTime int64 = -1, -1

	for _, event := range queryResult.Events {
		namespaces[event.Resource.Namespace] = true
		kinds[event.Resource.Kind] = true
		groups[event.Resource.Group] = true
		resourceCounts[event.Resource.Kind]++

		if minTime < 0 || event.Timestamp < minTime {
			minTime = event.Timestamp
		}
		if maxTime < 0 || event.Timestamp > maxTime {
			maxTime = event.Timestamp
		}
	}

	// Convert maps to sorted slices
	namespacesList := make([]string, 0, len(namespaces))
	for ns := range namespaces {
		namespacesList = append(namespacesList, ns)
	}
	sort.Strings(namespacesList)

	kindsList := make([]string, 0, len(kinds))
	for kind := range kinds {
		kindsList = append(kindsList, kind)
	}
	sort.Strings(kindsList)

	groupsList := make([]string, 0, len(groups))
	for group := range groups {
		groupsList = append(groupsList, group)
	}
	sort.Strings(groupsList)

	// Convert nanoseconds to seconds for API
	if minTime < 0 {
		minTime = 0
	}
	if maxTime < 0 {
		maxTime = 0
	}

	response := models.MetadataResponse{
		Namespaces:     namespacesList,
		Kinds:          kindsList,
		Groups:         groupsList,
		ResourceCounts: resourceCounts,
		TotalEvents:    len(queryResult.Events),
		TimeRange: models.TimeRangeInfo{
			Earliest: minTime / 1e9,
			Latest:   maxTime / 1e9,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	writeJSON(w, response)
}

// respondWithError sends an error response
func (mh *MetadataHandler) respondWithError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]string{
		"error":   errorCode,
		"message": message,
	}

	writeJSON(w, response)
}
