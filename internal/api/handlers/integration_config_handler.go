package handlers

import (
	"net/http"
	"strings"

	"github.com/moolen/spectre/internal/api"
	integrationadmin "github.com/moolen/spectre/internal/app/integrationadmin"
	"github.com/moolen/spectre/internal/integration"
	"github.com/moolen/spectre/internal/logging"
)

// IntegrationConfigHandler handles REST API requests for integration config CRUD operations.
type IntegrationConfigHandler struct {
	service *integrationadmin.Service
	logger  *logging.Logger
}

// NewIntegrationConfigHandler creates a new integration config handler.
func NewIntegrationConfigHandler(configPath string, manager *integration.Manager, logger *logging.Logger) *IntegrationConfigHandler {
	return &IntegrationConfigHandler{
		service: integrationadmin.NewService(configPath, manager, logger),
		logger:  logger,
	}
}

func integrationNameFromPath(path, suffix string) string {
	name := strings.TrimPrefix(path, "/api/config/integrations/")
	if suffix != "" {
		name = strings.TrimSuffix(name, suffix)
	}
	name = strings.TrimSuffix(name, "/")
	return name
}

func (h *IntegrationConfigHandler) respondWithServiceError(w http.ResponseWriter, err error) {
	serviceErr, ok := err.(*integrationadmin.Error)
	if !ok {
		api.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	status := http.StatusInternalServerError
	switch serviceErr.Code {
	case "DUPLICATE_NAME", "SYNC_IN_PROGRESS", "VALIDATION_IN_PROGRESS":
		status = http.StatusConflict
	case "INVALID_CONFIG", "NOT_SUPPORTED", "NOT_CONFIGURED":
		status = http.StatusBadRequest
	case "NOT_FOUND":
		status = http.StatusNotFound
	}

	api.WriteError(w, status, serviceErr.Code, serviceErr.Message)
}
