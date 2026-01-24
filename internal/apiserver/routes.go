package apiserver

import (
	"net/http"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/api/handlers"
	"github.com/moolen/spectre/internal/api/pb/pbconnect"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// registerHandlers registers all HTTP handlers
func (s *Server) registerHandlers() {
	// Register Connect Timeline service
	s.registerConnectService()

	// Register HTTP API handlers
	s.registerHTTPHandlers()

	// Register health and readiness endpoints
	s.registerHealthEndpoints()

	// Register MCP endpoint (must be before static UI catch-all)
	s.registerMCPHandler()

	// Register static UI handlers (must be last as catch-all)
	s.registerStaticUIHandlers()
}

// registerConnectService creates and registers the Connect Timeline service
func (s *Server) registerConnectService() {
	tracer := s.getTracer("spectre.api.connect")

	// Create Connect Timeline service with appropriate executors
	var timelineConnectService *api.TimelineConnectService
	if s.graphExecutor != nil && s.querySource == api.TimelineQuerySourceGraph {
		s.logger.Info("Connect Timeline service using GRAPH query executor")
		timelineConnectService = api.NewTimelineConnectServiceWithMode(s.queryExecutor, s.graphExecutor, s.querySource, s.logger, tracer)
	} else if s.graphExecutor != nil {
		s.logger.Info("Connect Timeline service using STORAGE query executor (graph available)")
		timelineConnectService = api.NewTimelineConnectServiceWithMode(s.queryExecutor, s.graphExecutor, api.TimelineQuerySourceStorage, s.logger, tracer)
	} else {
		s.logger.Info("Connect Timeline service using STORAGE query executor only")
		timelineConnectService = api.NewTimelineConnectService(s.queryExecutor, s.logger, tracer)
	}

	// Register Connect handler (supports gRPC, gRPC-Web, and Connect protocols)
	timelinePath, timelineHandler := pbconnect.NewTimelineServiceHandler(timelineConnectService)
	s.router.Handle(timelinePath, timelineHandler)
}

// registerHTTPHandlers registers all HTTP API handlers
func (s *Server) registerHTTPHandlers() {
	tracer := s.getTracer("spectre.api")

	// Register API handlers from handlers package
	handlers.RegisterHandlers(
		s.router,
		s.queryExecutor,
		s.graphExecutor,
		s.querySource,
		s.timelineService, // Pass shared timeline service
		s.graphClient,
		s.graphPipeline,
		s.metadataCache,
		s.nsGraphCache,
		s.integrationsConfigPath,
		s.integrationManager,
		s.logger,
		tracer,
		s.withMethod,
	)
}

// registerHealthEndpoints registers health and readiness check endpoints
func (s *Server) registerHealthEndpoints() {
	s.router.HandleFunc("/health", s.handleHealth)
	s.router.HandleFunc("/ready", s.handleReady)
}

// registerStaticUIHandlers registers static UI file serving handlers
// This must be called last as it acts as a catch-all
func (s *Server) registerStaticUIHandlers() {
	s.router.HandleFunc("/", s.serveStaticUI)
	s.router.HandleFunc("/timeline", s.serveStaticUI)
}

// registerIntegrationConfigHandlers registers integration config management endpoints.
// This is called after the integration manager is created (separate from initial handler registration).
func (s *Server) registerIntegrationConfigHandlers() {
	if s.integrationsConfigPath == "" || s.integrationManager == nil {
		s.logger.Warn("Integration config endpoints NOT registered (configPath=%q, manager=%v)",
			s.integrationsConfigPath, s.integrationManager != nil)
		return
	}

	configHandler := handlers.NewIntegrationConfigHandler(s.integrationsConfigPath, s.integrationManager, s.logger)

	// Collection endpoints
	s.router.HandleFunc("/api/config/integrations", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			configHandler.HandleList(w, r)
		case "POST":
			configHandler.HandleCreate(w, r)
		default:
			api.WriteError(w, 405, "METHOD_NOT_ALLOWED", "Allowed: GET, POST")
		}
	})

	// SSE endpoint for real-time status updates (must be registered before the trailing-slash route)
	s.router.HandleFunc("/api/config/integrations/stream", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			api.WriteError(w, 405, "METHOD_NOT_ALLOWED", "GET required")
			return
		}
		configHandler.HandleStatusStream(w, r)
	})

	// Test endpoint for unsaved integrations (must be registered before the trailing-slash route)
	s.router.HandleFunc("/api/config/integrations/test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			api.WriteError(w, 405, "METHOD_NOT_ALLOWED", "POST required")
			return
		}
		configHandler.HandleTest(w, r)
	})

	// Instance-specific endpoints with path parameter
	s.router.HandleFunc("/api/config/integrations/", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Path[len("/api/config/integrations/"):]
		if name == "" {
			api.WriteError(w, 404, "NOT_FOUND", "Integration name required")
			return
		}

		// Check for /test suffix (for saved integrations: /api/config/integrations/{name}/test)
		if len(name) > 5 && name[len(name)-5:] == "/test" {
			if r.Method != "POST" {
				api.WriteError(w, 405, "METHOD_NOT_ALLOWED", "POST required")
				return
			}
			configHandler.HandleTest(w, r)
			return
		}

		// Check for /sync suffix (for Grafana integrations: /api/config/integrations/{name}/sync)
		if len(name) > 5 && name[len(name)-5:] == "/sync" {
			if r.Method != "POST" {
				api.WriteError(w, 405, "METHOD_NOT_ALLOWED", "POST required")
				return
			}
			configHandler.HandleSync(w, r)
			return
		}

		// Route by method for /{name} operations
		switch r.Method {
		case "GET":
			configHandler.HandleGet(w, r)
		case "PUT":
			configHandler.HandleUpdate(w, r)
		case "DELETE":
			configHandler.HandleDelete(w, r)
		default:
			api.WriteError(w, 405, "METHOD_NOT_ALLOWED", "Allowed: GET, PUT, DELETE")
		}
	})

	s.logger.Info("Registered /api/config/integrations endpoints")
}

// getTracer returns a tracer for the given name
func (s *Server) getTracer(name string) trace.Tracer {
	if s.tracingProvider != nil && s.tracingProvider.IsEnabled() {
		return s.tracingProvider.GetTracer(name)
	}
	return otel.GetTracerProvider().Tracer(name)
}
