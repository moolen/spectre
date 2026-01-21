package handlers

import (
	"net/http"
	"strings"

	namespacegraph "github.com/moolen/spectre/internal/analysis/namespace_graph"
	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graph/sync"
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
	graphClient graph.Client,
	graphPipeline sync.Pipeline,
	metadataCache *api.MetadataCache,
	namespaceGraphCache *namespacegraph.Cache,
	configPath string,
	integrationManager *integration.Manager,
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
		var namespaceGraphHandler *NamespaceGraphHandler
		if namespaceGraphCache != nil {
			namespaceGraphHandler = NewNamespaceGraphHandlerWithCache(graphClient, namespaceGraphCache, logger, tracer)
			logger.Info("Registered /v1/namespace-graph endpoint (with caching)")
		} else {
			namespaceGraphHandler = NewNamespaceGraphHandler(graphClient, logger, tracer)
			logger.Info("Registered /v1/namespace-graph endpoint")
		}
		router.HandleFunc("/v1/namespace-graph", withMethod(http.MethodGet, namespaceGraphHandler.Handle))
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

	// Register integration config management endpoints
	if configPath != "" && integrationManager != nil {
		configHandler := NewIntegrationConfigHandler(configPath, integrationManager, logger)

		// Collection endpoints
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

		// Test endpoint for unsaved integrations (must be registered before the trailing-slash route)
		router.HandleFunc("/api/config/integrations/test", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				api.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST required")
				return
			}
			configHandler.HandleTest(w, r)
		})

		// Instance-specific endpoints with path parameter
		router.HandleFunc("/api/config/integrations/", func(w http.ResponseWriter, r *http.Request) {
			name := strings.TrimPrefix(r.URL.Path, "/api/config/integrations/")
			if name == "" {
				api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "Integration name required")
				return
			}

			// Check for /test suffix (for saved integrations: /api/config/integrations/{name}/test)
			if strings.HasSuffix(name, "/test") {
				if r.Method != http.MethodPost {
					api.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "POST required")
					return
				}
				configHandler.HandleTest(w, r)
				return
			}

			// Route by method for /{name} operations
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
	} else {
		logger.Warn("Integration config endpoints NOT registered (configPath=%q, manager=%v)", configPath, integrationManager != nil)
	}
}
