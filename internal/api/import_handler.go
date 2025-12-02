package api

import (
	"encoding/json"
	"net/http"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/storage"
)

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

	h.logger.InfoWithFields("Starting import",
		logging.Field("validate_files", validateFiles),
		logging.Field("overwrite", overwrite))

	// Read archive from request body
	report, err := h.storage.Import(r.Body, opts)
	if err != nil {
		h.logger.Error("Import failed: %v", err)
		writeError(w, http.StatusInternalServerError, "IMPORT_FAILED", err.Error())
		return
	}

	h.logger.InfoWithFields("Import completed",
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

