package handlers

import (
	"net/http"
	"strings"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/integration"
	"github.com/moolen/spectre/internal/logging"
)

func RegisterIntegrationConfigRoutes(router *http.ServeMux, configPath string, integrationManager *integration.Manager, logger *logging.Logger) {
	if configPath == "" || integrationManager == nil {
		logger.Warn("Integration config endpoints NOT registered (configPath=%q, manager=%v)", configPath, integrationManager != nil)
		return
	}

	configHandler := NewIntegrationConfigHandler(configPath, integrationManager, logger)

	router.HandleFunc("/api/config/integrations", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			configHandler.HandleList(w, r)
		case http.MethodPost:
			configHandler.HandleCreate(w, r)
		default:
			api.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Allowed: GET, POST")
		}
	})

	router.HandleFunc("/api/config/integrations/stream", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			api.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "GET required")
			return
		}
		configHandler.HandleStatusStream(w, r)
	})

	router.HandleFunc("/api/config/integrations/test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			api.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST required")
			return
		}
		configHandler.HandleTest(w, r)
	})

	router.HandleFunc("/api/config/integrations/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/api/config/integrations/")
		name = strings.TrimSuffix(name, "/")
		logger.Debug("Integration endpoint: path=%s, name=%s, method=%s", r.URL.Path, name, r.Method)
		if name == "" {
			api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Integration name required")
			return
		}

		if strings.HasSuffix(name, "/test") {
			if r.Method != http.MethodPost {
				api.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST required")
				return
			}
			configHandler.HandleTest(w, r)
			return
		}

		if strings.HasSuffix(name, "/sync") {
			if r.Method != http.MethodPost {
				api.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST required")
				return
			}
			configHandler.HandleSync(w, r)
			return
		}

		if strings.HasSuffix(name, "/signals/validate") {
			if r.Method != http.MethodPost {
				api.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST required")
				return
			}
			configHandler.HandleSignalValidation(w, r)
			return
		}

		switch r.Method {
		case http.MethodGet:
			configHandler.HandleGet(w, r)
		case http.MethodPut:
			configHandler.HandleUpdate(w, r)
		case http.MethodDelete:
			configHandler.HandleDelete(w, r)
		default:
			api.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Allowed: GET, PUT, DELETE")
		}
	})

	logger.Info("Registered /api/config/integrations endpoints")
}
