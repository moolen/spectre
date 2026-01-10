package handlers

import (
	"net/http"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graph/sync"
	"github.com/moolen/spectre/internal/logging"
	"go.opentelemetry.io/otel/trace"
)

// RegisterHandlers registers all HTTP handlers on the given router
func RegisterHandlers(
	router *http.ServeMux,
	storageExecutor api.QueryExecutor,
	graphExecutor api.QueryExecutor,
	querySource api.TimelineQuerySource,
	graphClient graph.Client,
	graphPipeline sync.Pipeline,
	metadataCache *api.MetadataCache,
	logger *logging.Logger,
	tracer trace.Tracer,
	withMethod func(string, http.HandlerFunc) http.HandlerFunc,
) {
	// Select appropriate executor for search handler
	var searchExecutor api.QueryExecutor
	if graphExecutor != nil && querySource == api.TimelineQuerySourceGraph {
		searchExecutor = graphExecutor
	} else {
		searchExecutor = storageExecutor
	}
	searchHandler := NewSearchHandler(searchExecutor, logger, tracer)

	// Create timeline handler with appropriate executor(s)
	var timelineHandler *TimelineHandler
	if graphExecutor != nil && querySource == api.TimelineQuerySourceGraph {
		// Use dual-executor mode with graph as primary
		logger.Info("Timeline handler using GRAPH query executor")
		timelineHandler = NewTimelineHandlerWithMode(storageExecutor, graphExecutor, querySource, logger, tracer)
	} else if graphExecutor != nil {
		// Graph available but using storage - enable both for A/B testing
		logger.Info("Timeline handler using STORAGE query executor (graph available for comparison)")
		timelineHandler = NewTimelineHandlerWithMode(storageExecutor, graphExecutor, api.TimelineQuerySourceStorage, logger, tracer)
	} else {
		// Storage only
		logger.Info("Timeline handler using STORAGE query executor only")
		timelineHandler = NewTimelineHandler(storageExecutor, logger, tracer)
	}

	// Select appropriate executor for metadata handler (same as timeline)
	var metadataExecutor api.QueryExecutor
	if graphExecutor != nil && querySource == api.TimelineQuerySourceGraph {
		logger.Info("Metadata handler using GRAPH query executor")
		metadataExecutor = graphExecutor
	} else {
		logger.Info("Metadata handler using STORAGE query executor")
		metadataExecutor = storageExecutor
	}
	metadataHandler := NewMetadataHandler(metadataExecutor, metadataCache, logger, tracer)

	router.HandleFunc("/v1/search", withMethod(http.MethodGet, searchHandler.Handle))
	router.HandleFunc("/v1/timeline", withMethod(http.MethodGet, timelineHandler.Handle))
	router.HandleFunc("/v1/metadata", withMethod(http.MethodGet, metadataHandler.Handle))

	// Register A/B test comparison endpoint if both executors are available
	if storageExecutor != nil && graphExecutor != nil {
		compareHandler := NewTimelineCompareHandler(storageExecutor, graphExecutor, logger)
		router.HandleFunc("/v1/timeline/compare", withMethod(http.MethodGet, compareHandler.Handle))
		logger.Info("Registered /v1/timeline/compare endpoint for A/B testing")
	}

	// Register causal graph handler if graph client is available
	if graphClient != nil {
		causalGraphHandler := NewCausalGraphHandler(graphClient, logger, tracer)
		router.HandleFunc("/v1/causal-graph", withMethod(http.MethodGet, causalGraphHandler.Handle))
		logger.Info("Registered /v1/causal-graph endpoint")
	}

	// Register anomaly handler if graph client is available
	if graphClient != nil {
		anomalyHandler := NewAnomalyHandler(graphClient, logger, tracer)
		router.HandleFunc("/v1/anomalies", withMethod(http.MethodGet, anomalyHandler.Handle))
		logger.Info("Registered /v1/anomalies endpoint")
	}

	// Register causal paths handler if graph client is available
	if graphClient != nil {
		causalPathsHandler := NewCausalPathsHandler(graphClient, logger, tracer)
		router.HandleFunc("/v1/causal-paths", withMethod(http.MethodGet, causalPathsHandler.Handle))
		logger.Info("Registered /v1/causal-paths endpoint")
	}

	// Register namespace graph handler if graph client is available
	if graphClient != nil {
		namespaceGraphHandler := NewNamespaceGraphHandler(graphClient, logger, tracer)
		router.HandleFunc("/v1/namespace-graph", withMethod(http.MethodGet, namespaceGraphHandler.Handle))
		logger.Info("Registered /v1/namespace-graph endpoint")
	}

	// Register import handler if graph pipeline is available
	if graphPipeline != nil {
		importHandler := NewImportHandler(graphPipeline, logger)
		router.HandleFunc("/v1/storage/import", withMethod(http.MethodPost, importHandler.Handle))
		logger.Info("Registered /v1/storage/import endpoint for event imports")
	}

	// Register export handler if graph query executor is available
	if graphExecutor != nil {
		exportHandler := NewExportHandler(graphExecutor, logger)
		router.HandleFunc("/v1/storage/export", withMethod(http.MethodGet, exportHandler.Handle))
		logger.Info("Registered /v1/storage/export endpoint for event exports")
	}
}
