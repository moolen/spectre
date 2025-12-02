package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/storage"
)

// ExportHandler handles storage export requests
type ExportHandler struct {
	storage *storage.Storage
	logger  *logging.Logger
}

// NewExportHandler creates a new export handler
func NewExportHandler(storage *storage.Storage, logger *logging.Logger) *ExportHandler {
	return &ExportHandler{
		storage: storage,
		logger:  logger,
	}
}

// Handle processes export requests
func (h *ExportHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	startTimeStr := r.URL.Query().Get("from")
	endTimeStr := r.URL.Query().Get("to")
	includeOpenHourStr := r.URL.Query().Get("include_open_hour")
	compressionStr := r.URL.Query().Get("compression")
	clusterID := r.URL.Query().Get("cluster_id")
	instanceID := r.URL.Query().Get("instance_id")

	// Parse timestamps
	var startTime, endTime int64
	var err error

	if startTimeStr != "" {
		startTime, err = strconv.ParseInt(startTimeStr, 10, 64)
		if err != nil {
			h.logger.Warn("Invalid start time parameter: %v", err)
			writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid 'from' parameter: must be Unix timestamp")
			return
		}
	}

	if endTimeStr != "" {
		endTime, err = strconv.ParseInt(endTimeStr, 10, 64)
		if err != nil {
			h.logger.Warn("Invalid end time parameter: %v", err)
			writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "Invalid 'to' parameter: must be Unix timestamp")
			return
		}
	}

	// Validate time range
	if startTime > 0 && endTime > 0 && startTime > endTime {
		writeError(w, http.StatusBadRequest, "INVALID_PARAMETER", "Start time must be before end time")
		return
	}

	// Parse boolean flags
	includeOpenHour := includeOpenHourStr == "true" || includeOpenHourStr == "1"
	compression := compressionStr == "true" || compressionStr == "1" || compressionStr == ""

	opts := storage.ExportOptions{
		StartTime:       startTime,
		EndTime:         endTime,
		IncludeOpenHour: includeOpenHour,
		ClusterID:       clusterID,
		InstanceID:      instanceID,
		Compression:     compression,
	}

	h.logger.InfoWithFields("Starting export",
		logging.Field("start_time", startTime),
		logging.Field("end_time", endTime),
		logging.Field("include_open_hour", includeOpenHour),
		logging.Field("compression", compression))

	// Set response headers
	filename := fmt.Sprintf("spectre-export-%d.tar", time.Now().Unix())
	if compression {
		filename += ".gz"
	}

	w.Header().Set("Content-Type", "application/x-tar")
	if compression {
		w.Header().Set("Content-Type", "application/gzip")
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	// Stream export directly to response
	if err := h.storage.Export(w, opts); err != nil {
		h.logger.Error("Export failed: %v", err)
		// Can't write error response after streaming has started
		// The client will see an incomplete/corrupted archive
		return
	}

	h.logger.Info("Export completed successfully")
}
