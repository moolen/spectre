package apiserver

import (
	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/api/handlers"
	"github.com/moolen/spectre/internal/api/pb/pbconnect"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

func (s *Server) registerHandlers() {
	s.registerConnectService()
	s.registerHTTPHandlers()
	s.registerHealthEndpoints()
	s.registerMetricsEndpoint()
	s.registerMCPHandler()
	s.registerStaticUIHandlers()
}

func (s *Server) registerConnectService() {
	tracer := s.getTracer("spectre.api.connect")
	s.logger.Info("Connect Timeline service using STORAGE query executor")
	timelineConnectService := api.NewTimelineConnectService(s.queryExecutor, s.logger, tracer)

	timelinePath, timelineHandler := pbconnect.NewTimelineServiceHandler(timelineConnectService)
	s.router.Handle(timelinePath, timelineHandler)
}

func (s *Server) registerHTTPHandlers() {
	tracer := s.getTracer("spectre.api")
	handlers.RegisterHandlers(
		s.router,
		s.queryExecutor,
		s.timelineService,
		s.analysisStore,
		s.importIngestor,
		s.metadataCache,
		s.nsGraphCache,
		s.logger,
		tracer,
		s.withMethod,
	)
}

func (s *Server) registerHealthEndpoints() {
	s.router.HandleFunc("/health", s.handleHealth)
	s.router.HandleFunc("/ready", s.handleReady)
}

func (s *Server) registerMetricsEndpoint() {
	s.router.Handle("/metrics", promhttp.Handler())
}

func (s *Server) registerStaticUIHandlers() {
	s.router.HandleFunc("/", s.serveStaticUI)
	s.router.HandleFunc("/timeline", s.serveStaticUI)
}

func (s *Server) getTracer(name string) trace.Tracer {
	if s.tracingProvider != nil && s.tracingProvider.IsEnabled() {
		return s.tracingProvider.GetTracer(name)
	}
	return otel.GetTracerProvider().Tracer(name)
}
