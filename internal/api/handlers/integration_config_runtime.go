package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/moolen/spectre/internal/api"
	integrationadmin "github.com/moolen/spectre/internal/app/integrationadmin"
)

// HandleSync handles POST /api/config/integrations/{name}/sync - triggers manual dashboard sync for Grafana integrations.
func (h *IntegrationConfigHandler) HandleSync(w http.ResponseWriter, r *http.Request) {
	name := integrationNameFromPath(r.URL.Path, "/sync")
	if name == "" || name == r.URL.Path {
		api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Integration name required")
		return
	}

	status, err := h.service.TriggerSync(r.Context(), name)
	if err != nil {
		h.respondWithServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = api.WriteJSON(w, status)
}

// HandleTest handles POST /api/config/integrations/{name}/test - tests an integration connection.
func (h *IntegrationConfigHandler) HandleTest(w http.ResponseWriter, r *http.Request) {
	var testReq integrationadmin.TestConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&testReq); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	response := h.service.TestConnection(testReq)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = api.WriteJSON(w, response)
}

// HandleSignalValidation handles POST /api/config/integrations/{name}/signals/validate - triggers signal validation.
func (h *IntegrationConfigHandler) HandleSignalValidation(w http.ResponseWriter, r *http.Request) {
	name := integrationNameFromPath(r.URL.Path, "/signals/validate")
	if name == "" || name == r.URL.Path {
		api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Integration name required")
		return
	}

	response, err := h.service.TriggerSignalValidation(r.Context(), name, r.URL.Query().Get("full") == "true")
	if err != nil {
		h.respondWithServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = api.WriteJSON(w, response)
}
