package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/config"
	"github.com/moolen/spectre/internal/integration"
	_ "github.com/moolen/spectre/internal/integration/grafana"
	"github.com/moolen/spectre/internal/logging"
)

// IntegrationConfigHandler handles REST API requests for integration config CRUD operations.
type IntegrationConfigHandler struct {
	configPath string
	manager    *integration.Manager
	logger     *logging.Logger
}

// NewIntegrationConfigHandler creates a new integration config handler.
func NewIntegrationConfigHandler(configPath string, manager *integration.Manager, logger *logging.Logger) *IntegrationConfigHandler {
	return &IntegrationConfigHandler{
		configPath: configPath,
		manager:    manager,
		logger:     logger,
	}
}

// IntegrationInstanceResponse represents a single integration instance with health status enrichment.
type IntegrationInstanceResponse struct {
	Name      string                 `json:"name"`
	Type      string                 `json:"type"`
	Enabled   bool                   `json:"enabled"`
	Config    map[string]interface{} `json:"config"`
	Health    string                 `json:"health"`    // "healthy", "degraded", "stopped", "not_started"
	DateAdded string                 `json:"dateAdded"` // ISO8601 timestamp
}

// TestConnectionRequest represents the request body for testing a connection.
type TestConnectionRequest struct {
	Name    string                 `json:"name"`
	Type    string                 `json:"type"`
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config"`
}

// TestConnectionResponse represents the response from testing a connection.
type TestConnectionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// HandleList handles GET /api/config/integrations - returns all integration instances with health status.
func (h *IntegrationConfigHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	// Load current config file
	integrationsFile, err := config.LoadIntegrationsFile(h.configPath)
	if err != nil {
		h.logger.Error("Failed to load integrations config: %v", err)
		api.WriteError(w, http.StatusInternalServerError, "LOAD_ERROR", fmt.Sprintf("Failed to load config: %v", err))
		return
	}

	// Enrich with health status from manager
	registry := h.manager.GetRegistry()
	responses := make([]IntegrationInstanceResponse, 0, len(integrationsFile.Instances))

	for _, instance := range integrationsFile.Instances {
		response := IntegrationInstanceResponse{
			Name:      instance.Name,
			Type:      instance.Type,
			Enabled:   instance.Enabled,
			Config:    instance.Config,
			Health:    "not_started",
			DateAdded: time.Now().Format(time.RFC3339), // TODO: Track actual creation time in config
		}

		// Query runtime health if instance is registered
		if runtimeInstance, ok := registry.Get(instance.Name); ok {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			healthStatus := runtimeInstance.Health(ctx)
			response.Health = healthStatus.String()
		}

		responses = append(responses, response)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = api.WriteJSON(w, responses)
}

// HandleGet handles GET /api/config/integrations/{name} - returns a single integration instance.
func (h *IntegrationConfigHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	// Extract name from URL path
	name := strings.TrimPrefix(r.URL.Path, "/api/config/integrations/")
	if name == "" || name == r.URL.Path {
		api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Integration name required")
		return
	}

	// Load config
	integrationsFile, err := config.LoadIntegrationsFile(h.configPath)
	if err != nil {
		h.logger.Error("Failed to load integrations config: %v", err)
		api.WriteError(w, http.StatusInternalServerError, "LOAD_ERROR", fmt.Sprintf("Failed to load config: %v", err))
		return
	}

	// Find instance by name
	var found *config.IntegrationConfig
	for i := range integrationsFile.Instances {
		if integrationsFile.Instances[i].Name == name {
			found = &integrationsFile.Instances[i]
			break
		}
	}

	if found == nil {
		api.WriteError(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("Integration %q not found", name))
		return
	}

	// Enrich with health status
	response := IntegrationInstanceResponse{
		Name:      found.Name,
		Type:      found.Type,
		Enabled:   found.Enabled,
		Config:    found.Config,
		Health:    "not_started",
		DateAdded: time.Now().Format(time.RFC3339),
	}

	registry := h.manager.GetRegistry()
	if runtimeInstance, ok := registry.Get(found.Name); ok {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		healthStatus := runtimeInstance.Health(ctx)
		response.Health = healthStatus.String()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = api.WriteJSON(w, response)
}

// HandleCreate handles POST /api/config/integrations - creates a new integration instance.
func (h *IntegrationConfigHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var newInstance config.IntegrationConfig
	if err := json.NewDecoder(r.Body).Decode(&newInstance); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	// Load current config
	integrationsFile, err := config.LoadIntegrationsFile(h.configPath)
	if err != nil {
		h.logger.Error("Failed to load integrations config: %v", err)
		api.WriteError(w, http.StatusInternalServerError, "LOAD_ERROR", fmt.Sprintf("Failed to load config: %v", err))
		return
	}

	// Check for duplicate name
	for _, instance := range integrationsFile.Instances {
		if instance.Name == newInstance.Name {
			api.WriteError(w, http.StatusConflict, "DUPLICATE_NAME", fmt.Sprintf("Integration %q already exists", newInstance.Name))
			return
		}
	}

	// Validate the new instance
	testFile := &config.IntegrationsFile{
		SchemaVersion: integrationsFile.SchemaVersion,
		Instances:     append(integrationsFile.Instances, newInstance),
	}
	if err := testFile.Validate(); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_CONFIG", fmt.Sprintf("Validation failed: %v", err))
		return
	}

	// Append new instance
	integrationsFile.Instances = append(integrationsFile.Instances, newInstance)

	// Write atomically
	if err := config.WriteIntegrationsFile(h.configPath, integrationsFile); err != nil {
		h.logger.Error("Failed to write integrations config: %v", err)
		api.WriteError(w, http.StatusInternalServerError, "WRITE_ERROR", fmt.Sprintf("Failed to save config: %v", err))
		return
	}

	h.logger.Info("Created integration instance: %s (type: %s)", newInstance.Name, newInstance.Type)

	// Return created instance
	response := IntegrationInstanceResponse{
		Name:      newInstance.Name,
		Type:      newInstance.Type,
		Enabled:   newInstance.Enabled,
		Config:    newInstance.Config,
		Health:    "not_started",
		DateAdded: time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = api.WriteJSON(w, response)
}

// HandleUpdate handles PUT /api/config/integrations/{name} - updates an existing integration instance.
func (h *IntegrationConfigHandler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	// Extract name from URL path
	name := strings.TrimPrefix(r.URL.Path, "/api/config/integrations/")
	if name == "" || name == r.URL.Path {
		api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Integration name required")
		return
	}

	// Parse request body
	var updatedInstance config.IntegrationConfig
	if err := json.NewDecoder(r.Body).Decode(&updatedInstance); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	// Load current config
	integrationsFile, err := config.LoadIntegrationsFile(h.configPath)
	if err != nil {
		h.logger.Error("Failed to load integrations config: %v", err)
		api.WriteError(w, http.StatusInternalServerError, "LOAD_ERROR", fmt.Sprintf("Failed to load config: %v", err))
		return
	}

	// Find and replace instance
	found := false
	for i := range integrationsFile.Instances {
		if integrationsFile.Instances[i].Name == name {
			// Preserve name (can't change via update)
			updatedInstance.Name = name
			integrationsFile.Instances[i] = updatedInstance
			found = true
			break
		}
	}

	if !found {
		api.WriteError(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("Integration %q not found", name))
		return
	}

	// Validate updated config
	if err := integrationsFile.Validate(); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_CONFIG", fmt.Sprintf("Validation failed: %v", err))
		return
	}

	// Write atomically
	if err := config.WriteIntegrationsFile(h.configPath, integrationsFile); err != nil {
		h.logger.Error("Failed to write integrations config: %v", err)
		api.WriteError(w, http.StatusInternalServerError, "WRITE_ERROR", fmt.Sprintf("Failed to save config: %v", err))
		return
	}

	h.logger.Info("Updated integration instance: %s", name)

	// Return updated instance
	response := IntegrationInstanceResponse{
		Name:      updatedInstance.Name,
		Type:      updatedInstance.Type,
		Enabled:   updatedInstance.Enabled,
		Config:    updatedInstance.Config,
		Health:    "not_started",
		DateAdded: time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = api.WriteJSON(w, response)
}

// HandleDelete handles DELETE /api/config/integrations/{name} - removes an integration instance.
func (h *IntegrationConfigHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	// Extract name from URL path
	name := strings.TrimPrefix(r.URL.Path, "/api/config/integrations/")
	if name == "" || name == r.URL.Path {
		api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Integration name required")
		return
	}

	// Load current config
	integrationsFile, err := config.LoadIntegrationsFile(h.configPath)
	if err != nil {
		h.logger.Error("Failed to load integrations config: %v", err)
		api.WriteError(w, http.StatusInternalServerError, "LOAD_ERROR", fmt.Sprintf("Failed to load config: %v", err))
		return
	}

	// Filter out instance by name
	found := false
	newInstances := make([]config.IntegrationConfig, 0, len(integrationsFile.Instances))
	for _, instance := range integrationsFile.Instances {
		if instance.Name == name {
			found = true
			continue
		}
		newInstances = append(newInstances, instance)
	}

	if !found {
		api.WriteError(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("Integration %q not found", name))
		return
	}

	integrationsFile.Instances = newInstances

	// Write atomically
	if err := config.WriteIntegrationsFile(h.configPath, integrationsFile); err != nil {
		h.logger.Error("Failed to write integrations config: %v", err)
		api.WriteError(w, http.StatusInternalServerError, "WRITE_ERROR", fmt.Sprintf("Failed to save config: %v", err))
		return
	}

	h.logger.Info("Deleted integration instance: %s", name)

	w.WriteHeader(http.StatusNoContent)
}

// HandleTest handles POST /api/config/integrations/{name}/test - tests an integration connection.
func (h *IntegrationConfigHandler) HandleTest(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var testReq TestConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&testReq); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	// Validate config using IntegrationsFile validator
	testFile := &config.IntegrationsFile{
		SchemaVersion: "v1",
		Instances: []config.IntegrationConfig{
			{
				Name:    testReq.Name,
				Type:    testReq.Type,
				Enabled: testReq.Enabled,
				Config:  testReq.Config,
			},
		},
	}
	if err := testFile.Validate(); err != nil {
		response := TestConnectionResponse{
			Success: false,
			Message: fmt.Sprintf("Validation failed: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = api.WriteJSON(w, response)
		return
	}

	// Look up factory
	factory, ok := integration.GetFactory(testReq.Type)
	if !ok {
		response := TestConnectionResponse{
			Success: false,
			Message: fmt.Sprintf("Unknown integration type: %s", testReq.Type),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = api.WriteJSON(w, response)
		return
	}

	// Test connection with panic recovery
	success, message := h.testConnection(factory, testReq)

	response := TestConnectionResponse{
		Success: success,
		Message: message,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = api.WriteJSON(w, response)
}

// HandleStatusStream handles GET /api/config/integrations/stream - SSE endpoint for real-time status updates.
func (h *IntegrationConfigHandler) HandleStatusStream(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Check if flusher is supported
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.logger.Error("SSE not supported: ResponseWriter doesn't implement Flusher")
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	h.logger.Debug("SSE client connected for integration status stream")

	// Track last known status to only send changes
	lastStatus := make(map[string]string)

	// Poll interval
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Send initial status immediately
	h.sendStatusUpdate(w, flusher, lastStatus)

	for {
		select {
		case <-r.Context().Done():
			h.logger.Debug("SSE client disconnected")
			return
		case <-ticker.C:
			h.sendStatusUpdate(w, flusher, lastStatus)
		}
	}
}

// sendStatusUpdate sends an SSE event if any integration status has changed.
func (h *IntegrationConfigHandler) sendStatusUpdate(w http.ResponseWriter, flusher http.Flusher, lastStatus map[string]string) {
	// Load current config
	integrationsFile, err := config.LoadIntegrationsFile(h.configPath)
	if err != nil {
		h.logger.Error("SSE: Failed to load integrations config: %v", err)
		return
	}

	registry := h.manager.GetRegistry()
	hasChanges := false
	responses := make([]IntegrationInstanceResponse, 0, len(integrationsFile.Instances))

	// Check for removed integrations
	currentNames := make(map[string]bool)
	for _, instance := range integrationsFile.Instances {
		currentNames[instance.Name] = true
	}
	for name := range lastStatus {
		if !currentNames[name] {
			delete(lastStatus, name)
			hasChanges = true
		}
	}

	for _, instance := range integrationsFile.Instances {
		health := "not_started"

		// Query runtime health if instance is registered
		if runtimeInstance, ok := registry.Get(instance.Name); ok {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			healthStatus := runtimeInstance.Health(ctx)
			cancel()
			health = healthStatus.String()
		}

		// Check if status changed
		if lastHealth, exists := lastStatus[instance.Name]; !exists || lastHealth != health {
			hasChanges = true
			lastStatus[instance.Name] = health
		}

		responses = append(responses, IntegrationInstanceResponse{
			Name:      instance.Name,
			Type:      instance.Type,
			Enabled:   instance.Enabled,
			Config:    instance.Config,
			Health:    health,
			DateAdded: time.Now().Format(time.RFC3339),
		})
	}

	// Only send if there are changes or this is the first send (lastStatus was empty)
	if hasChanges || len(lastStatus) == 0 {
		data, err := json.Marshal(responses)
		if err != nil {
			h.logger.Error("SSE: Failed to marshal status: %v", err)
			return
		}

		// Write SSE event
		fmt.Fprintf(w, "event: status\ndata: %s\n\n", data)
		flusher.Flush()
	}
}

// testConnection attempts to create and test an integration instance with panic recovery.
func (h *IntegrationConfigHandler) testConnection(factory integration.IntegrationFactory, testReq TestConnectionRequest) (success bool, message string) {
	// Recover from panics
	defer func() {
		if r := recover(); r != nil {
			success = false
			message = fmt.Sprintf("Test panicked: %v", r)
			h.logger.Error("Integration test panicked: %v", r)
		}
	}()

	// Create instance
	instance, err := factory(testReq.Name, testReq.Config)
	if err != nil {
		return false, fmt.Sprintf("Failed to create instance: %v", err)
	}

	// Start with 5-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := instance.Start(ctx); err != nil {
		return false, fmt.Sprintf("Failed to start: %v", err)
	}

	// Check health
	healthCtx, healthCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer healthCancel()

	healthStatus := instance.Health(healthCtx)
	if healthStatus != integration.Healthy {
		// Still stop cleanly
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopCancel()
		_ = instance.Stop(stopCtx)

		return false, fmt.Sprintf("Health check failed: %s", healthStatus.String())
	}

	// Stop instance after successful test
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()

	if err := instance.Stop(stopCtx); err != nil {
		h.logger.Warn("Failed to stop test instance cleanly: %v", err)
	}

	return true, "Connection successful"
}
