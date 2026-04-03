package handlers

import (
	"net/http"

	namespacegraph "github.com/moolen/spectre/internal/analysis/namespace_graph"
	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/api"
	appgraph "github.com/moolen/spectre/internal/app/graph"
	apptimeline "github.com/moolen/spectre/internal/app/timeline"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/integration"
	"github.com/moolen/spectre/internal/logging"
	"go.opentelemetry.io/otel/trace"
)

// RegisterHandlers registers all HTTP handlers on the given router
func RegisterHandlers(
	router *http.ServeMux,
	storageExecutor api.QueryExecutor,
	graphExecutor api.QueryExecutor,
	querySource api.TimelineQuerySource,
	timelineService *apptimeline.Service, // Shared timeline service
	analysisStore analysisstore.AnalysisStore,
	graphClient graph.Client,
	importIngestor api.BatchIngestor,
	metadataCache *api.MetadataCache,
	namespaceGraphCache *namespacegraph.Cache,
	configPath string,
	integrationManager *integration.Manager,
	logger *logging.Logger,
	tracer trace.Tracer,
	withMethod func(string, http.HandlerFunc) http.HandlerFunc,
) {
	// Use provided timeline service (created by apiserver for sharing between REST and MCP)
	// Create timeline handler using the service
	timelineHandler := NewTimelineHandler(timelineService, logger, tracer)

	// Create MetadataService with appropriate executor (same as timeline)
	var metadataExecutor api.QueryExecutor
	if graphExecutor != nil && querySource == api.TimelineQuerySourceGraph {
		logger.Info("Metadata service using GRAPH query executor")
		metadataExecutor = graphExecutor
	} else {
		logger.Info("Metadata service using STORAGE query executor")
		metadataExecutor = storageExecutor
	}
	metadataService := api.NewMetadataService(metadataExecutor, metadataCache, logger, tracer)
	metadataHandler := NewMetadataHandler(metadataService, logger, tracer)

	router.HandleFunc("/v1/timeline", withMethod(http.MethodGet, timelineHandler.Handle))
	router.HandleFunc("/v1/metadata", withMethod(http.MethodGet, metadataHandler.Handle))

	// Create GraphService if graph client is available (shared by graph-related handlers)
	var graphService *appgraph.Service
	switch {
	case graphClient != nil:
		graphService = appgraph.NewServiceFromGraphClient(graphClient, logger, tracer)
		logger.Info("Created GraphService for graph analysis operations")
	case analysisStore != nil:
		graphService = appgraph.NewService(analysisStore, logger, tracer)
		logger.Info("Created GraphService for embedded analysis operations")
	}

	// Register namespace graph handler if graph service is available
	if graphService != nil {
		var namespaceGraphHandler *NamespaceGraphHandler
		if namespaceGraphCache != nil {
			namespaceGraphHandler = NewNamespaceGraphHandlerWithCache(graphService, namespaceGraphCache, logger, tracer)
			logger.Info("Registered /v1/namespace-graph endpoint (with caching)")
		} else {
			namespaceGraphHandler = NewNamespaceGraphHandler(graphService, logger, tracer)
			logger.Info("Registered /v1/namespace-graph endpoint")
		}
		router.HandleFunc("/v1/namespace-graph", withMethod(http.MethodGet, namespaceGraphHandler.Handle))
	}

	// Register observatory graph handler if graph service is available
	if graphService != nil && graphService.HasObservatoryAnalyzer() {
		observatoryGraphHandler := NewObservatoryGraphHandler(graphService, logger, tracer)
		router.HandleFunc("/v1/observatory-graph", withMethod(http.MethodGet, observatoryGraphHandler.Handle))
		logger.Info("Registered /v1/observatory-graph endpoint")
	}

	// Register import handler when a backend batch ingestor is available.
	if importIngestor != nil {
		importHandler := NewImportHandler(importIngestor, logger)
		router.HandleFunc("/v1/storage/import", withMethod(http.MethodPost, importHandler.Handle))
		logger.Info("Registered /v1/storage/import endpoint for event imports")
	}

	// Register export handler when the selected query executor is available.
	var exportExecutor api.QueryExecutor
	if graphExecutor != nil && querySource == api.TimelineQuerySourceGraph {
		exportExecutor = graphExecutor
	} else {
		exportExecutor = storageExecutor
	}
	if exportExecutor != nil {
		exportHandler := NewExportHandler(exportExecutor, logger)
		router.HandleFunc("/v1/storage/export", withMethod(http.MethodGet, exportHandler.Handle))
		logger.Info("Registered /v1/storage/export endpoint for event exports")
	}

	RegisterIntegrationConfigRoutes(router, configPath, integrationManager, logger)
}
