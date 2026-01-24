package handlers

import (
	"net/http"
	"time"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/logging"
	"go.opentelemetry.io/otel/trace"
)

// MetadataHandler handles /v1/metadata requests
type MetadataHandler struct {
	metadataService *api.MetadataService
	logger          *logging.Logger
	tracer          trace.Tracer
}

// NewMetadataHandler creates a new metadata handler
func NewMetadataHandler(metadataService *api.MetadataService, logger *logging.Logger, tracer trace.Tracer) *MetadataHandler {
	return &MetadataHandler{
		metadataService: metadataService,
		logger:          logger,
		tracer:          tracer,
	}
}

// Handle handles metadata requests
func (mh *MetadataHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
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

	// Always try to use cache (metadata changes infrequently)
	useCache := true

	// Call service to get metadata
	response, cacheHit, err := mh.metadataService.GetMetadata(ctx, useCache, startTimeNs, endTimeNs)
	if err != nil {
		mh.logger.Error("Failed to fetch metadata: %v", err)
		mh.respondWithError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch metadata")
		return
	}

	// Set appropriate cache header
	w.Header().Set("Content-Type", "application/json")
	if cacheHit {
		w.Header().Set("X-Cache", "HIT")
	} else {
		w.Header().Set("X-Cache", "MISS")
	}
	w.WriteHeader(http.StatusOK)
	_ = api.WriteJSON(w, response)
}

// respondWithError sends an error response
func (mh *MetadataHandler) respondWithError(w http.ResponseWriter, statusCode int, errorCode, message string) {
	api.WriteError(w, statusCode, errorCode, message)
}
