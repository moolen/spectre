package api

import (
	"compress/flate"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"github.com/moolen/spectre/internal/storage"
)

const (
	// ContentTypeEventsBinary is the content-type for spectre binary event archives
	ContentTypeEventsBinary = "application/vnd.spectre.events.v1+bin"
	ContentTypeEventsJSON   = "application/vnd.spectre.events.v1+json"
	// MaxPayloadSize is the maximum allowed request body size (30 MB)
	MaxPayloadSize = 30 * 1024 * 1024
)

// BatchEventImportRequest represents a JSON request to import a batch of events
type BatchEventImportRequest struct {
	Events []*models.Event `json:"events"`
}

// ImportHandler handles storage import requests
type ImportHandler struct {
	storage *storage.Storage
	logger  *logging.Logger
}

// NewImportHandler creates a new import handler
func NewImportHandler(storage *storage.Storage, logger *logging.Logger) *ImportHandler {
	return &ImportHandler{
		storage: storage,
		logger:  logger,
	}
}

// Handle processes import requests
func (h *ImportHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	validateFilesStr := r.URL.Query().Get("validate")
	overwriteStr := r.URL.Query().Get("overwrite")

	// Parse boolean flags
	validateFiles := validateFilesStr == "true" || validateFilesStr == "1" || validateFilesStr == ""
	overwrite := overwriteStr == "true" || overwriteStr == "1"

	opts := storage.ImportOptions{
		ValidateFiles:     validateFiles,
		OverwriteExisting: overwrite,
	}

	contentType := r.Header.Get("Content-Type")

	// empty content-type for backwards compat
	if contentType == "" || strings.HasPrefix(contentType, ContentTypeEventsBinary) {
		h.handleArchiveImport(w, r, opts)
	}
	if strings.HasPrefix(contentType, ContentTypeEventsJSON) {
		h.handleJSONEventImport(w, r, opts)
	} else {
		h.logger.Error("Unsupported Content-Type: %s", contentType)
		writeError(w, http.StatusBadRequest, "UNSUPPORTED_CONTENT_TYPE", fmt.Sprintf("Content-Type %s not supported", contentType))
		return
	}
}

// handleJSONEventImport processes a JSON batch event import request
func (h *ImportHandler) handleJSONEventImport(w http.ResponseWriter, r *http.Request, opts storage.ImportOptions) {
	h.logger.InfoWithFields("Starting JSON event batch import",
		logging.Field("validate_files", opts.ValidateFiles),
		logging.Field("overwrite", opts.OverwriteExisting))

	// Prepare to read request body with size limit
	limitedBody := io.LimitReader(r.Body, int64(MaxPayloadSize))
	defer r.Body.Close()

	// Decompress if needed based on Content-Encoding header
	var decompressedBody io.Reader = limitedBody
	contentEncoding := r.Header.Get("Content-Encoding")

	switch contentEncoding {
	case "gzip":
		gzipReader, err := gzip.NewReader(limitedBody)
		if err != nil {
			h.logger.Error("Failed to create gzip reader: %v", err)
			writeError(w, http.StatusBadRequest, "INVALID_ENCODING", "Failed to decompress gzip data")
			return
		}
		defer gzipReader.Close()
		decompressedBody = gzipReader

	case "deflate":
		deflateReader := flate.NewReader(limitedBody)
		defer deflateReader.Close()
		decompressedBody = deflateReader

	case "":
		// No compression, use as-is
		break

	default:
		h.logger.Error("Unsupported Content-Encoding: %s", contentEncoding)
		writeError(w, http.StatusBadRequest, "UNSUPPORTED_ENCODING", fmt.Sprintf("Content-Encoding %s not supported", contentEncoding))
		return
	}

	// Parse JSON request
	var req BatchEventImportRequest
	decoder := json.NewDecoder(decompressedBody)
	if err := decoder.Decode(&req); err != nil {
		h.logger.Error("Failed to parse JSON: %v", err)
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "INVALID_JSON", "Empty request body")
		} else {
			writeError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("Failed to parse JSON: %v", err))
		}
		return
	}

	if len(req.Events) == 0 {
		h.logger.Warn("Empty events array in import request")
		writeError(w, http.StatusBadRequest, "NO_EVENTS", "Events array is empty")
		return
	}

	h.logger.InfoWithFields("Parsed JSON import request",
		logging.Field("event_count", len(req.Events)))

	// Call storage engine to ingest events
	report, err := h.storage.AddEventsBatch(req.Events, opts)
	if err != nil {
		h.logger.Error("Event batch ingestion failed: %v", err)
		writeError(w, http.StatusInternalServerError, "INGEST_FAILED", err.Error())
		return
	}

	h.logger.InfoWithFields("JSON event batch import completed",
		logging.Field("total_events", report.TotalEvents),
		logging.Field("merged_hours", report.MergedHours),
		logging.Field("errors", len(report.Errors)))

	// Return import report
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]interface{}{
		"status":         "success",
		"total_events":   report.TotalEvents,
		"merged_hours":   report.MergedHours,
		"imported_files": report.ImportedFiles,
		"duration":       report.Duration.String(),
		"errors":         report.Errors,
	}

	json.NewEncoder(w).Encode(response)
}

// handleArchiveImport processes a binary archive import request (existing behavior)
func (h *ImportHandler) handleArchiveImport(w http.ResponseWriter, r *http.Request, opts storage.ImportOptions) {
	h.logger.InfoWithFields("Starting archive import",
		logging.Field("validate_files", opts.ValidateFiles),
		logging.Field("overwrite", opts.OverwriteExisting))

	defer r.Body.Close()

	// Read archive from request body with size limit
	limitedBody := io.LimitReader(r.Body, int64(MaxPayloadSize))

	report, err := h.storage.Import(limitedBody, opts)
	if err != nil {
		h.logger.Error("Archive import failed: %v", err)
		writeError(w, http.StatusInternalServerError, "IMPORT_FAILED", err.Error())
		return
	}

	h.logger.InfoWithFields("Archive import completed",
		logging.Field("total_files", report.TotalFiles),
		logging.Field("imported", report.ImportedFiles),
		logging.Field("merged_hours", report.MergedHours),
		logging.Field("failed", report.FailedFiles),
		logging.Field("events", report.TotalEvents))

	// Return import report
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]interface{}{
		"status":         "success",
		"total_files":    report.TotalFiles,
		"imported_files": report.ImportedFiles,
		"merged_hours":   report.MergedHours,
		"skipped_files":  report.SkippedFiles,
		"failed_files":   report.FailedFiles,
		"total_events":   report.TotalEvents,
		"duration":       report.Duration.String(),
		"errors":         report.Errors,
	}

	json.NewEncoder(w).Encode(response)
}
