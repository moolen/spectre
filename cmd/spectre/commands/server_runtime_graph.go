package commands

import (
	"context"
	"time"

	"github.com/mark3labs/mcp-go/server"
	analysisfalkor "github.com/moolen/spectre/internal/analysis/store/falkor"
	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/apiserver"
	appgraph "github.com/moolen/spectre/internal/app/graph"
	"github.com/moolen/spectre/internal/config"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graph/reconciler"
	"github.com/moolen/spectre/internal/graph/sync"
	"github.com/moolen/spectre/internal/graphservice"
	"github.com/moolen/spectre/internal/integration"
	"github.com/moolen/spectre/internal/lifecycle"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/mcp"
	"github.com/moolen/spectre/internal/tracing"
	"github.com/moolen/spectre/internal/watcher"
)

func runGraphServerRuntime(cfg *config.Config, mode serverRuntimeMode, manager *lifecycle.Manager, tracingProvider *tracing.TracingProvider, logger *logging.Logger) {
	if mode.AuditOnly {
		logger.Info("Running in audit-only mode - no graph database, events written to: %s", auditLogPath)
	}

	auditLogWriter := newAuditLogWriter(logger)
	graphServiceComponent, graphClient, graphPipeline, graphQueryExecutor, analysisStore := initializeGraphRuntime(mode, logger)
	watcherComponent := initializeGraphWatcher(cfg, mode, graphPipeline, auditLogWriter, logger)
	readinessChecker := graphReadinessChecker(watcherComponent)

	if mode.AuditOnly {
		runAuditOnlyRuntime(manager, watcherComponent, auditLogWriter, logger)
		return
	}
	if !mode.StartGraph {
		logger.Info("Graph runtime disabled - skipping graph-only startup")
		return
	}

	logger.Info("Timeline query source: GRAPH")
	apiComponent := apiserver.NewWithStorageGraphAndPipeline(
		cfg.APIPort,
		nil,
		graphQueryExecutor,
		api.TimelineQuerySourceGraph,
		nil,
		graphClient,
		analysisStore,
		graphPipeline,
		readinessChecker,
		tracingProvider,
		time.Duration(metadataCacheRefreshSeconds)*time.Second,
		apiserver.NamespaceGraphCacheConfig{
			Enabled:     namespaceGraphCacheEnabled,
			RefreshTTL:  time.Duration(namespaceGraphCacheRefreshSeconds) * time.Second,
			MaxMemoryMB: int64(namespaceGraphCacheMemoryMB),
		},
		integrationsConfigPath,
		nil,
		nil,
	)
	logger.Info("API server component created (graph-only)")

	mcpServer, integrationMgr := initializeGraphMCP(apiComponent, graphServiceComponent, graphClient, tracingProvider, manager, logger)
	if integrationMgr != nil {
		if err := apiComponent.RegisterIntegrationHandlers(integrationMgr); err != nil {
			logger.Error("Failed to register integration config handlers: %v", err)
			HandleError(err, "Integration handler registration error")
		}
		logger.Info("Integration config handlers registered")
	}
	if err := apiComponent.RegisterMCPEndpoint(mcpServer); err != nil {
		logger.Error("Failed to register MCP endpoint: %v", err)
		HandleError(err, "MCP endpoint registration error")
	}
	logger.Info("MCP endpoint registered on API server")

	if graphServiceComponent != nil && apiComponent.GetNamespaceGraphCache() != nil {
		graphServiceComponent.RegisterCacheInvalidator(apiComponent.GetNamespaceGraphCache())
		logger.Info("Registered namespace graph cache for event-driven invalidation")
	}

	registerGraphRuntimeComponents(manager, graphServiceComponent, watcherComponent, graphClient, logger)
	if err := manager.Register(apiComponent); err != nil {
		logger.Error("Failed to register API server component: %v", err)
		HandleError(err, "API server registration error")
	}

	logger.Info("All components registered with dependencies")
	cancel := startManagedComponents(manager, logger)
	runGraphStartupImport(graphPipeline, logger)
	startStdioTransport(mcpServer, logger)
	logger.Info("Application started successfully")
	logger.Info("Listening for events and API requests...")
	waitForShutdown(manager, cancel, auditLogWriter, logger)
}

func initializeGraphRuntime(mode serverRuntimeMode, logger *logging.Logger) (*graphservice.Service, graph.Client, sync.Pipeline, api.QueryExecutor, *analysisfalkor.Store) {
	if !mode.StartGraph {
		return nil, nil, nil, nil, nil
	}

	logger.Info("Initializing graph service")
	serviceConfig := graphservice.ServiceConfig{
		GraphConfig: graph.ClientConfig{
			Host:               graphHost,
			Port:               graphPort,
			GraphName:          graphName,
			MaxRetries:         10,
			DialTimeout:        10 * time.Second,
			ReadTimeout:        120 * time.Second,
			WriteTimeout:       120 * time.Second,
			PoolSize:           10,
			QueryCacheEnabled:  true,
			QueryCacheMemoryMB: 128,
			QueryCacheTTL:      30 * time.Second,
		},
		PipelineConfig:    graphservice.DefaultServiceConfig().PipelineConfig,
		AutoStartPipeline: true,
	}
	serviceConfig.PipelineConfig.RetentionWindow = time.Duration(graphRetentionHours) * time.Hour

	graphServiceComponent := graphservice.NewService(serviceConfig)
	if err := graphServiceComponent.Initialize(context.Background()); err != nil {
		logger.Error("Failed to initialize graph service: %v", err)
		HandleError(err, "Graph service initialization error")
	}
	logger.Info("Graph service initialized successfully")

	graphClient := graphServiceComponent.GetClient()
	logger.Info("Graph query executor created")
	return graphServiceComponent, graphClient, graphServiceComponent.GetPipeline(), graph.NewQueryExecutor(graphClient), analysisfalkor.New(graphClient)
}

func initializeGraphWatcher(cfg *config.Config, mode serverRuntimeMode, graphPipeline sync.Pipeline, auditLogWriter *watcher.FileAuditLogWriter, logger *logging.Logger) *watcher.Watcher {
	if !mode.StartWatcher {
		logger.Info("Watcher disabled - running in read-only mode")
		return nil
	}

	var eventHandler *watcher.EventCaptureHandler
	if mode.AuditOnly {
		eventHandler = watcher.NewEventCaptureHandler(nil)
		eventHandler.SetAuditLog(auditLogWriter)
		logger.Info("Creating watcher in audit-only mode")
	} else {
		eventHandler = watcher.NewEventCaptureHandlerWithMode(nil, graphPipeline, watcher.TimelineModeGraph)
		if auditLogWriter != nil {
			eventHandler.SetAuditLog(auditLogWriter)
		}
	}

	watcherComponent, err := watcher.New(eventHandler, cfg.WatcherConfigPath)
	if err != nil {
		logger.Error("Failed to create watcher component: %v", err)
		HandleError(err, "Watcher initialization error")
	}
	if mode.AuditOnly {
		logger.Info("Watcher component created (audit-only mode)")
	} else {
		logger.Info("Watcher component created (graph-only mode)")
	}
	return watcherComponent
}

func graphReadinessChecker(watcherComponent *watcher.Watcher) apiserver.ReadinessChecker {
	if watcherComponent != nil {
		return watcherComponent
	}
	return &apiserver.NoOpReadinessChecker{}
}

func runAuditOnlyRuntime(manager *lifecycle.Manager, watcherComponent *watcher.Watcher, auditLogWriter *watcher.FileAuditLogWriter, logger *logging.Logger) {
	if watcherComponent != nil {
		if err := manager.Register(watcherComponent); err != nil {
			logger.Error("Failed to register watcher component: %v", err)
			HandleError(err, "Watcher registration error")
		}
	}

	logger.Info("All components registered (audit-only mode)")
	cancel := startManagedComponents(manager, logger)
	logger.Info("Audit-only mode started - watching events and writing to: %s", auditLogPath)
	waitForShutdown(manager, cancel, auditLogWriter, logger)
}

func initializeGraphMCP(apiComponent *apiserver.Server, graphServiceComponent *graphservice.Service, graphClient graph.Client, tracingProvider *tracing.TracingProvider, manager *lifecycle.Manager, logger *logging.Logger) (*server.MCPServer, *integration.Manager) {
	logger.Info("Initializing MCP server with TimelineService and GraphService")
	var graphService *appgraph.Service
	if graphClient != nil {
		graphService = appgraph.NewServiceFromGraphClient(
			graphClient,
			logger,
			getTracingProviderTracer(tracingProvider, "graph_service"),
		)
		logger.Info("Created GraphService for MCP graph tools")
	}

	spectreServer, err := mcp.NewSpectreServerWithOptions(mcp.ServerOptions{
		Version:         Version,
		TimelineService: apiComponent.GetTimelineService(),
		GraphService:    graphService,
	})
	if err != nil {
		logger.Error("Failed to create MCP server: %v", err)
		HandleError(err, "MCP server initialization error")
	}
	mcpServer := spectreServer.GetMCPServer()
	logger.Info("MCP server created with direct TimelineService and GraphService access")

	if integrationsConfigPath == "" {
		return mcpServer, nil
	}

	logger.Info("Initializing integration manager from: %s", integrationsConfigPath)
	integrationMgr, err := integration.NewManagerWithMCPRegistry(integration.ManagerConfig{
		ConfigPath:            integrationsConfigPath,
		MinIntegrationVersion: minIntegrationVersion,
		GraphClient:           graphClient,
	}, mcp.NewMCPToolRegistry(mcpServer))
	if err != nil {
		logger.Error("Failed to create integration manager: %v", err)
		HandleError(err, "Integration manager initialization error")
	}
	if err := manager.Register(integrationMgr); err != nil {
		logger.Error("Failed to register integration manager: %v", err)
		HandleError(err, "Integration manager registration error")
	}
	logger.Info("Integration manager registered")
	return mcpServer, integrationMgr
}

func registerGraphRuntimeComponents(manager *lifecycle.Manager, graphServiceComponent *graphservice.Service, watcherComponent *watcher.Watcher, graphClient graph.Client, logger *logging.Logger) {
	if graphServiceComponent != nil {
		if err := manager.Register(graphServiceComponent); err != nil {
			logger.Error("Failed to register graph service component: %v", err)
			HandleError(err, "Graph service registration error")
		}
	}
	if watcherComponent != nil {
		if err := manager.Register(watcherComponent); err != nil {
			logger.Error("Failed to register watcher component: %v", err)
			HandleError(err, "Watcher registration error")
		}
	}

	if !reconcilerEnabled {
		return
	}
	if graphClient == nil || watcherComponent == nil {
		logger.Info("Reconciler disabled: requires both graph and watcher to be enabled")
		return
	}

	restConfig := watcherComponent.GetRestConfig()
	if restConfig == nil {
		logger.Warn("Cannot initialize reconciler: watcher REST config not available")
		return
	}

	reconcilerComponent, err := reconciler.New(reconciler.Config{
		Enabled:   true,
		Interval:  time.Duration(reconcilerIntervalMins) * time.Minute,
		BatchSize: reconcilerBatchSize,
	}, graphClient, restConfig)
	if err != nil {
		logger.Error("Failed to create reconciler: %v", err)
		return
	}
	if err := manager.Register(reconcilerComponent, graphServiceComponent); err != nil {
		logger.Error("Failed to register reconciler component: %v", err)
		return
	}

	logger.Info("Reconciler component registered (interval: %dm, batch: %d)", reconcilerIntervalMins, reconcilerBatchSize)
}

func runGraphStartupImport(graphPipeline sync.Pipeline, logger *logging.Logger) {
	if importPath == "" {
		return
	}

	importCtx, importCancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer importCancel()

	if startupImportTimelineOnly || startupImportDisableCausality {
		importCtx = sync.WithBatchProcessingOptions(importCtx, sync.BatchProcessingOptions{
			DisableCausality: startupImportDisableCausality,
			TimelineOnly:     startupImportTimelineOnly,
		})
		if startupImportTimelineOnly {
			logger.Info("Startup import configured for timeline-only mode")
		} else {
			logger.Info("Startup import configured to skip causality inference")
		}
	}

	if err := runStartupImport(importCtx, startupImportOptions{
		Path:             importPath,
		ChunkSize:        importChunkSize,
		BenchmarkLogPath: importBenchmarkLog,
		ImportMode:       importMode,
		Logger:           logger,
		Pipeline:         graphPipeline,
	}); err != nil {
		logger.Error("Failed to run startup import: %v", err)
		HandleError(err, "Import error")
	}
}
