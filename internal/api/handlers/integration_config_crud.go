package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/config"
)

// HandleList handles GET /api/config/integrations - returns all integration instances with health status.
func (h *IntegrationConfigHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	responses, err := h.service.List(r.Context())
	if err != nil {
		h.respondWithServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = api.WriteJSON(w, responses)
}

// HandleGet handles GET /api/config/integrations/{name} - returns a single integration instance.
func (h *IntegrationConfigHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	name := integrationNameFromPath(r.URL.Path, "")
	if name == "" || name == r.URL.Path {
		api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Integration name required")
		return
	}

	response, err := h.service.Get(r.Context(), name)
	if err != nil {
		h.respondWithServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = api.WriteJSON(w, response)
}

// HandleCreate handles POST /api/config/integrations - creates a new integration instance.
func (h *IntegrationConfigHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var newInstance config.IntegrationConfig
	if err := json.NewDecoder(r.Body).Decode(&newInstance); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	response, err := h.service.Create(newInstance)
	if err != nil {
		h.respondWithServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = api.WriteJSON(w, response)
}

// HandleUpdate handles PUT /api/config/integrations/{name} - updates an existing integration instance.
func (h *IntegrationConfigHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	name := integrationNameFromPath(r.URL.Path, "")
	if name == "" || name == r.URL.Path {
		api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Integration name required")
		return
	}

	var updatedInstance config.IntegrationConfig
	if err := json.NewDecoder(r.Body).Decode(&updatedInstance); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	response, err := h.service.Update(name, updatedInstance)
	if err != nil {
		h.respondWithServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = api.WriteJSON(w, response)
}

// HandleDelete handles DELETE /api/config/integrations/{name} - removes an integration instance.
func (h *IntegrationConfigHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	name := integrationNameFromPath(r.URL.Path, "")
	if name == "" || name == r.URL.Path {
		api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Integration name required")
		return
	}

	if err := h.service.Delete(name); err != nil {
		h.respondWithServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
