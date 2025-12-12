package api

import (
	"net/http"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel/trace"
)

// MetadataHandler handles /v1/metadata requests
type MetadataHandler struct {
	queryExecutor QueryExecutor
	logger        *logging.Logger
	tracer        trace.Tracer
}

// NewMetadataHandler creates a new metadata handler
func NewMetadataHandler(queryExecutor QueryExecutor, logger *logging.Logger, tracer trace.Tracer) *MetadataHandler {
	return &MetadataHandler{
		queryExecutor: queryExecutor,
		logger:        logger,
		tracer:        tracer,
	}
}

// Handle handles metadata requests
func (mh *MetadataHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	params := r.URL.Query()
	startStr := params.Get("start")
	startTime, err := ParseOptionalTimestamp(startStr, 0)
	if err != nil {
		mh.respondWithError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	endStr := params.Get("end")
	endTime, err := ParseOptionalTimestamp(endStr, time.Now().Unix())
	if err != nil {
		mh.respondWithError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	query := &models.QueryRequest{
		StartTimestamp: startTime,
		EndTimestamp:   endTime,
		Filters:        models.QueryFilters{},
	}

	queryResult, err := mh.queryExecutor.Execute(ctx, query)
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
	_ = writeJSON(w, response)
}

// respondWithError sends an error response
func (mh *MetadataHandler) respondWithError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]string{
		"error":   errorCode,
		"message": message,
	}

	_ = writeJSON(w, response)
}
