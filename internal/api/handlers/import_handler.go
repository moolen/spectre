package handlers

import (
	"compress/flate"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/graph/sync"
	"github.com/moolen/spectre/internal/importexport"
	"github.com/moolen/spectre/internal/logging"
)

const (
	// MaxPayloadSize is the maximum allowed request body size (30 MB)
	MaxPayloadSize = 30 * 1024 * 1024
)

// ImportHandler handles event import requests using the graph pipeline
type ImportHandler struct {
	pipeline sync.Pipeline
	logger   *logging.Logger
}

// NewImportHandler creates a new import handler
func NewImportHandler(pipeline sync.Pipeline, logger *logging.Logger) *ImportHandler {
	return &ImportHandler{
		pipeline: pipeline,
		logger:   logger,
	}
}

// Handle processes import requests
func (h *ImportHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters (kept for compatibility, but not all are used in graph mode)
	validateFilesStr := r.URL.Query().Get("validate")
	overwriteStr := r.URL.Query().Get("overwrite")

	// Parse boolean flags
	validateFiles := validateFilesStr == "true" || validateFilesStr == "1" || validateFilesStr == ""
	overwrite := overwriteStr == "true" || overwriteStr == "1"

	// Log parameters (some may not apply to graph mode)
	h.logger.DebugWithFields("Import request received",
		logging.Field("validate", validateFiles),
		logging.Field("overwrite", overwrite))

	h.handleJSONEventImport(w, r, validateFiles, overwrite)
}

// handleJSONEventImport processes a JSON batch event import request
func (h *ImportHandler) handleJSONEventImport(w http.ResponseWriter, r *http.Request, validateFiles, overwrite bool) {
	startTime := time.Now()

	// Log request details for debugging
	contentLength := r.ContentLength
	h.logger.InfoWithFields("Starting JSON event batch import",
		logging.Field("validate", validateFiles),
		logging.Field("overwrite", overwrite),
		logging.Field("content_length", contentLength),
		logging.Field("content_length_mb", float64(contentLength)/(1024*1024)),
		logging.Field("remote_addr", r.RemoteAddr))

	// Prepare to read request body with size limit
	limitedBody := io.LimitReader(r.Body, int64(MaxPayloadSize))
	defer func() {
		if err := r.Body.Close(); err != nil {
			h.logger.Error("Failed to close request body: %v", err)
		}
	}()

	// Decompress if needed based on Content-Encoding header
	var decompressedBody io.Reader = limitedBody
	contentEncoding := r.Header.Get("Content-Encoding")
	h.logger.DebugWithFields("Reading request body",
		logging.Field("content_encoding", contentEncoding),
		logging.Field("max_payload_size_mb", MaxPayloadSize/(1024*1024)))

	switch contentEncoding {
	case "gzip":
		gzipReader, err := gzip.NewReader(limitedBody)
		if err != nil {
			h.logger.Error("Failed to create gzip reader: %v", err)
			api.WriteError(w, http.StatusBadRequest, "INVALID_ENCODING", "Failed to decompress gzip data")
			return
		}
		defer func() {
			if err := gzipReader.Close(); err != nil {
				h.logger.Error("Failed to close gzip reader: %v", err)
			}
		}()
		decompressedBody = gzipReader

	case "deflate":
		deflateReader := flate.NewReader(limitedBody)
		defer func() {
			if err := deflateReader.Close(); err != nil {
				h.logger.Error("Failed to close deflate reader: %v", err)
			}
		}()
		decompressedBody = deflateReader

	case "":
		// No compression, use as-is
		break

	default:
		h.logger.Error("Unsupported Content-Encoding: %s", contentEncoding)
		api.WriteError(w, http.StatusBadRequest, "UNSUPPORTED_ENCODING", fmt.Sprintf("Content-Encoding %s not supported", contentEncoding))
		return
	}

	// Parse JSON request using new Import API
	h.logger.Debug("Starting to parse JSON events from request body")
	eventValues, err := importexport.Import(importexport.FromReader(decompressedBody), importexport.WithLogger(h.logger))
	if err != nil {
		h.logger.Error("Failed to parse JSON: %v", err)
		api.WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	parseDuration := time.Since(startTime)
	h.logger.InfoWithFields("Parsed JSON import request",
		logging.Field("event_count", len(eventValues)),
		logging.Field("parse_duration", parseDuration))

	// Process events through graph pipeline
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	h.logger.InfoWithFields("Starting batch processing through pipeline",
		logging.Field("event_count", len(eventValues)),
		logging.Field("timeout", "5m"))

	processStartTime := time.Now()
	var errors []string
	if err := h.pipeline.ProcessBatch(ctx, eventValues); err != nil {
		processDuration := time.Since(processStartTime)
		h.logger.ErrorWithFields("Event batch processing failed",
			logging.Field("error", err),
			logging.Field("event_count", len(eventValues)),
			logging.Field("process_duration", processDuration))
		errors = append(errors, fmt.Sprintf("Batch processing failed: %v", err))
		api.WriteError(w, http.StatusInternalServerError, "INGEST_FAILED", err.Error())
		return
	}

	processDuration := time.Since(processStartTime)
	h.logger.InfoWithFields("Batch processing completed",
		logging.Field("event_count", len(eventValues)),
		logging.Field("process_duration", processDuration))

	duration := time.Since(startTime)

	h.logger.InfoWithFields("JSON event batch import completed",
		logging.Field("total_events", len(eventValues)),
		logging.Field("duration", duration))

	// Calculate approximate "files created" based on unique hours
	// This is for compatibility with existing tests that expect this field
	hourSet := make(map[int64]bool)
	for _, event := range eventValues {
		hour := time.Unix(0, event.Timestamp).Truncate(time.Hour).Unix()
		hourSet[hour] = true
	}
	filesCreated := len(hourSet)

	// Return import report
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]any{
		"status":         "success",
		"total_events":   len(eventValues),
		"merged_hours":   filesCreated, // Number of unique hours
		"files_created":  filesCreated, // For compatibility with tests
		"imported_files": 0,            // Not applicable in graph mode
		"duration":       duration.String(),
		"errors":         errors,
	}

	h.logger.Debug("Writing import response to client")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.ErrorWithFields("Failed to write import response",
			logging.Field("error", err),
			logging.Field("total_events", len(eventValues)))
		// Response already sent, can't send error response
		return
	}
	h.logger.Debug("Import response written successfully")
}
