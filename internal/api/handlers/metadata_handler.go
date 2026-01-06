package handlers

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"go.opentelemetry.io/otel/trace"
)

// MetadataHandler handles /v1/metadata requests
type MetadataHandler struct {
	queryExecutor api.QueryExecutor
	metadataCache *api.MetadataCache
	logger        *logging.Logger
	tracer        trace.Tracer
}

// NewMetadataHandler creates a new metadata handler
// metadataCache is optional - if nil, queries will go directly to the executor
func NewMetadataHandler(queryExecutor api.QueryExecutor, metadataCache *api.MetadataCache, logger *logging.Logger, tracer trace.Tracer) *MetadataHandler {
	return &MetadataHandler{
		queryExecutor: queryExecutor,
		metadataCache: metadataCache,
		logger:        logger,
		tracer:        tracer,
	}
}

// MetadataQueryExecutor interface for executors that support efficient metadata queries
type MetadataQueryExecutor interface {
	QueryDistinctMetadata(ctx context.Context, startTimeNs, endTimeNs int64) (namespaces []string, kinds []string, minTime int64, maxTime int64, err error)
}

// Handle handles metadata requests
func (mh *MetadataHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	params := r.URL.Query()
	startStr := params.Get("start")
	startTime, err := api.ParseOptionalTimestamp(startStr, 0)
	if err != nil {
		mh.respondWithError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	endStr := params.Get("end")
	endTime, err := api.ParseOptionalTimestamp(endStr, time.Now().Unix())
	if err != nil {
		mh.respondWithError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	startTimeNs := startTime * 1e9
	endTimeNs := endTime * 1e9

	// If cache is available and no time filtering requested, use cache
	// Cache contains all metadata regardless of time range
	// Note: startTime=0 and endTime=current time means "all data"
	useCache := mh.metadataCache != nil && startTime == 0

	if useCache {
		mh.logger.Debug("Using metadata cache for request")
		cachedData, err := mh.metadataCache.Get()
		if err == nil {
			// Successfully got cached data - return it immediately
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache", "HIT")
			w.WriteHeader(http.StatusOK)
			_ = api.WriteJSON(w, cachedData)
			return
		}

		// Cache failed - log and fall through to direct query
		mh.logger.Warn("Metadata cache unavailable, falling back to direct query: %v", err)
	}

	// Try to use efficient metadata query if available
	var namespacesList, kindsList []string
	var minTime, maxTime int64

	if metadataExecutor, ok := mh.queryExecutor.(MetadataQueryExecutor); ok {
		namespacesList, kindsList, minTime, maxTime, err = metadataExecutor.QueryDistinctMetadata(ctx, startTimeNs, endTimeNs)
		if err != nil {
			mh.logger.Error("Failed to query metadata: %v", err)
			mh.respondWithError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch metadata")
			return
		}
	} else {
		// Fallback to old method (shouldn't happen with current implementations)
		mh.logger.Warn("Query executor does not support QueryDistinctMetadata, using fallback")
		query := &models.QueryRequest{
			StartTimestamp: startTime,
			EndTimestamp:   endTime,
			Filters:        models.QueryFilters{},
		}

		queryResult, queryErr := mh.queryExecutor.Execute(ctx, query)
		if queryErr != nil {
			mh.logger.Error("Failed to query events: %v", queryErr)
			mh.respondWithError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch metadata")
			return
		}

		// Extract unique namespaces and kinds
		namespaces := make(map[string]bool)
		kinds := make(map[string]bool)
		minTime = -1
		maxTime = -1

		for _, event := range queryResult.Events {
			namespaces[event.Resource.Namespace] = true
			kinds[event.Resource.Kind] = true

			if minTime < 0 || event.Timestamp < minTime {
				minTime = event.Timestamp
			}
			if maxTime < 0 || event.Timestamp > maxTime {
				maxTime = event.Timestamp
			}
		}

		// Convert maps to sorted slices
		namespacesList = make([]string, 0, len(namespaces))
		for ns := range namespaces {
			namespacesList = append(namespacesList, ns)
		}
		sort.Strings(namespacesList)

		kindsList = make([]string, 0, len(kinds))
		for kind := range kinds {
			kindsList = append(kindsList, kind)
		}
		sort.Strings(kindsList)
	}

	// Convert nanoseconds to seconds for API
	if minTime < 0 {
		minTime = 0
	}
	if maxTime < 0 {
		maxTime = 0
	}

	response := models.MetadataResponse{
		Namespaces: namespacesList,
		Kinds:      kindsList,
		TimeRange: models.TimeRangeInfo{
			Earliest: minTime / 1e9,
			Latest:   maxTime / 1e9,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.WriteHeader(http.StatusOK)
	_ = api.WriteJSON(w, response)
}

// respondWithError sends an error response
func (mh *MetadataHandler) respondWithError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	api.WriteError(w, statusCode, errorCode, message)
}
