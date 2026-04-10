package handlers

import (
	"net/http"

	namespacegraph "github.com/moolen/spectre/internal/analysis/namespace_graph"
	analysisstore "github.com/moolen/spectre/internal/analysis/store"
	"github.com/moolen/spectre/internal/api"
	appgraph "github.com/moolen/spectre/internal/app/graph"
	apptimeline "github.com/moolen/spectre/internal/app/timeline"
	"github.com/moolen/spectre/internal/logging"
	"go.opentelemetry.io/otel/trace"
)

// RegisterHandlers registers all HTTP handlers on the given router.
func RegisterHandlers(
	router *http.ServeMux,
	storageExecutor api.QueryExecutor,
	timelineService *apptimeline.Service,
	analysisStore analysisstore.AnalysisStore,
	importIngestor api.BatchIngestor,
	metadataCache *api.MetadataCache,
	namespaceGraphCache *namespacegraph.Cache,
	logger *logging.Logger,
	tracer trace.Tracer,
	withMethod func(string, http.HandlerFunc) http.HandlerFunc,
) {
	timelineHandler := NewTimelineHandler(timelineService, logger, tracer)

	logger.Info("Metadata service using STORAGE query executor")
	metadataService := api.NewMetadataService(storageExecutor, metadataCache, logger, tracer)
	metadataHandler := NewMetadataHandler(metadataService, logger, tracer)

	router.HandleFunc("/v1/timeline", withMethod(http.MethodGet, timelineHandler.Handle))
	router.HandleFunc("/v1/metadata", withMethod(http.MethodGet, metadataHandler.Handle))

	if analysisStore != nil {
		graphService := appgraph.NewService(analysisStore, logger, tracer)
		if namespaceGraphCache != nil {
			namespaceGraphHandler := NewNamespaceGraphHandlerWithCache(graphService, namespaceGraphCache, logger, tracer)
			router.HandleFunc("/v1/namespace-graph", withMethod(http.MethodGet, namespaceGraphHandler.Handle))
			logger.Info("Registered /v1/namespace-graph endpoint (with caching)")
		} else {
			namespaceGraphHandler := NewNamespaceGraphHandler(graphService, logger, tracer)
			router.HandleFunc("/v1/namespace-graph", withMethod(http.MethodGet, namespaceGraphHandler.Handle))
			logger.Info("Registered /v1/namespace-graph endpoint")
		}
	}

	if importIngestor != nil {
		importHandler := NewImportHandler(importIngestor, logger)
		router.HandleFunc("/v1/storage/import", withMethod(http.MethodPost, importHandler.Handle))
		logger.Info("Registered /v1/storage/import endpoint for event imports")
	}

	if storageExecutor != nil {
		exportHandler := NewExportHandler(storageExecutor, logger)
		router.HandleFunc("/v1/storage/export", withMethod(http.MethodGet, exportHandler.Handle))
		logger.Info("Registered /v1/storage/export endpoint for event exports")
	}
}
