package apiserver

import (
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

// getTracer returns a tracer for the given name
func (s *Server) getTracer(name string) trace.Tracer {
	if s.tracingProvider != nil && s.tracingProvider.IsEnabled() {
		return s.tracingProvider.GetTracer(name)
	}
	return otel.GetTracerProvider().Tracer(name)
}
