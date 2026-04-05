package commands

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/apiserver"
	appgraph "github.com/moolen/spectre/internal/app/graph"
	"github.com/moolen/spectre/internal/config"
	"github.com/moolen/spectre/internal/embeddedstore"
	"github.com/moolen/spectre/internal/lifecycle"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/mcp"
	"github.com/moolen/spectre/internal/tracing"
	"github.com/moolen/spectre/internal/watcher"
	"github.com/prometheus/client_golang/prometheus"
)

func runEmbeddedServerRuntime(cfg *config.Config, mode serverRuntimeMode, manager *lifecycle.Manager, tracingProvider *tracing.TracingProvider, logger *logging.Logger) {
	embeddedCfg := embeddedStoreConfig()
	effectiveEmbeddedCfg, err := embeddedCfg.EffectiveEngineConfig()
	if err != nil {
		logger.Error("Invalid embedded engine configuration: %v", err)
		HandleError(err, "Embedded backend configuration error")
	}

	logger.Info("Running in embedded mode with data dir: %s", dataDir)
	logger.Info("Embedded engine config: %s", describeEmbeddedEngineConfig(effectiveEmbeddedCfg))

	embeddedBackend, err := embeddedstore.Open(embeddedCfg)
	if err != nil {
		logger.Error("Failed to open embedded backend: %v", err)
		HandleError(err, "Embedded backend initialization error")
	}
	if err := manager.Register(embeddedBackend); err != nil {
		logger.Error("Failed to register embedded backend: %v", err)
		HandleError(err, "Embedded backend registration error")
	}

	auditLogWriter := newAuditLogWriter(logger)
	runEmbeddedStartupImport(embeddedBackend, logger)
	validateEmbeddedImportMode(mode, embeddedBackend, logger)
	skipInitialListReplay := shouldSkipEmbeddedInitialListReplay(embeddedBackend, logger)

	watcherComponent := registerEmbeddedWatcher(mode, cfg, manager, embeddedBackend, auditLogWriter, skipInitialListReplay, logger)
	apiComponent, mcpServer := newEmbeddedAPI(cfg, mode, tracingProvider, embeddedBackend, watcherComponent, logger)

	if err := manager.Register(apiComponent, embeddedBackend); err != nil {
		logger.Error("Failed to register embedded API server component: %v", err)
		HandleError(err, "API server registration error")
	}

	logger.Info("All components registered (embedded mode)")
	cancel := startManagedComponents(manager, logger)
	startStdioTransport(mcpServer, logger)
	logger.Info("Embedded mode started - serving API requests")
	waitForShutdown(manager, cancel, auditLogWriter, logger)
}

func embeddedStoreConfig() embeddedstore.Config {
	return embeddedstore.Config{
		DataDir:                   dataDir,
		CheckpointInterval:        embeddedCheckpointInterval,
		CheckpointMaxTailEvents:   embeddedCheckpointMaxTailEvents,
		CheckpointMaxTailBytes:    embeddedCheckpointMaxTailBytes,
		CheckpointOnShutdown:      embeddedCheckpointOnShutdown,
		CheckpointOnShutdownSet:   embeddedCheckpointOnShutdownSet,
		MetricsRegisterer:         prometheus.DefaultRegisterer,
		ProjectionHistoryFallback: embeddedProjectionHistoryFallback,
	}
}

func newAuditLogWriter(logger *logging.Logger) *watcher.FileAuditLogWriter {
	if auditLogPath == "" {
		return nil
	}

	logger.Info("Event audit logging enabled: %s", auditLogPath)
	auditLogWriter, err := watcher.NewFileAuditLogWriter(auditLogPath)
	if err != nil {
		logger.Error("Failed to create audit log writer: %v", err)
		HandleError(err, "Audit log initialization error")
	}
	return auditLogWriter
}

func runEmbeddedStartupImport(embeddedBackend *embeddedstore.Backend, logger *logging.Logger) {
	if importPath == "" {
		return
	}

	importCtx, importCancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer importCancel()

	if err := runStartupImport(importCtx, startupImportOptions{
		Path:             importPath,
		ChunkSize:        importChunkSize,
		BenchmarkLogPath: importBenchmarkLog,
		ImportMode:       importMode,
		Logger:           logger,
		BatchIngestor:    embeddedBackend,
	}); err != nil {
		logger.Error("Failed to run embedded startup import: %v", err)
		HandleError(err, "Import error")
	}
}

func validateEmbeddedImportMode(mode serverRuntimeMode, embeddedBackend *embeddedstore.Backend, logger *logging.Logger) {
	if !mode.ImportOnly {
		return
	}

	usable, err := hasUsableEmbeddedBackend(embeddedBackend)
	if err != nil {
		logger.Error("Failed to validate embedded events: %v", err)
		HandleError(err, "Embedded executor error")
	}
	if usable {
		return
	}

	source := embeddedImportSourceDescription(importPath, dataDir)
	logger.Error("No usable embedded events found in %s", source)
	HandleError(fmt.Errorf("no usable embedded events found in %s", source), "Import error")
}

func registerEmbeddedWatcher(mode serverRuntimeMode, cfg *config.Config, manager *lifecycle.Manager, embeddedBackend *embeddedstore.Backend, auditLogWriter *watcher.FileAuditLogWriter, skipInitialListReplay bool, logger *logging.Logger) *watcher.Watcher {
	if !mode.StartWatcher {
		logger.Info("Embedded import-only mode - watcher disabled")
		return nil
	}

	eventHandler := watcher.NewEventCaptureHandler(embeddedBackend)
	if auditLogWriter != nil {
		eventHandler.SetAuditLog(auditLogWriter)
	}

	watcherComponent, err := watcher.New(eventHandler, cfg.WatcherConfigPath)
	if err != nil {
		logger.Error("Failed to create embedded watcher component: %v", err)
		HandleError(err, "Watcher initialization error")
	}
	if skipInitialListReplay {
		logger.Info("Skipping embedded watcher initial List replay because persisted embedded state already exists")
		watcherComponent.SetSkipInitialListReplay(true)
	}
	if err := manager.Register(watcherComponent, embeddedBackend); err != nil {
		logger.Error("Failed to register embedded watcher component: %v", err)
		HandleError(err, "Watcher registration error")
	}
	return watcherComponent
}

func newEmbeddedAPI(cfg *config.Config, mode serverRuntimeMode, tracingProvider *tracing.TracingProvider, embeddedBackend *embeddedstore.Backend, watcherComponent *watcher.Watcher, logger *logging.Logger) (*apiserver.Server, *server.MCPServer) {
	var readinessChecker apiserver.ReadinessChecker = embeddedBackend
	if watcherComponent != nil {
		readinessChecker = watcherComponent
	}

	var importIngestor api.BatchIngestor
	if embeddedImportAPIEnabled(mode) {
		importIngestor = embeddedBackend
	}

	apiComponent := apiserver.NewWithStorageGraphAndPipeline(
		cfg.APIPort,
		embeddedBackend.QueryExecutor(),
		nil,
		api.TimelineQuerySourceStorage,
		nil,
		nil,
		embeddedBackend.AnalysisStore(),
		importIngestor,
		readinessChecker,
		tracingProvider,
		time.Duration(metadataCacheRefreshSeconds)*time.Second,
		apiserver.NamespaceGraphCacheConfig{
			Enabled:     namespaceGraphCacheEnabled,
			RefreshTTL:  time.Duration(namespaceGraphCacheRefreshSeconds) * time.Second,
			MaxMemoryMB: int64(namespaceGraphCacheMemoryMB),
		},
		"",
		nil,
		nil,
	)
	logger.Info("API server component created (embedded)")

	if !mode.StartMCP {
		return apiComponent, nil
	}

	timelineService := apiComponent.GetTimelineService()
	graphService := appgraph.NewService(
		embeddedBackend.AnalysisStore(),
		logger,
		getTracingProviderTracer(tracingProvider, "graph_service"),
	)
	spectreServer, err := mcp.NewSpectreServerWithOptions(mcp.ServerOptions{
		Version:         Version,
		TimelineService: timelineService,
		GraphService:    graphService,
	})
	if err != nil {
		logger.Error("Failed to create embedded MCP server: %v", err)
		HandleError(err, "MCP server initialization error")
	}

	mcpServer := spectreServer.GetMCPServer()
	if err := apiComponent.RegisterMCPEndpoint(mcpServer); err != nil {
		logger.Error("Failed to register embedded MCP endpoint: %v", err)
		HandleError(err, "MCP endpoint registration error")
	}

	return apiComponent, mcpServer
}

type embeddedMetadataQuerier interface {
	QueryDistinctMetadata(ctx context.Context, startTimeNs, endTimeNs int64) ([]string, []string, int64, int64, error)
}

func describeEmbeddedEngineConfig(cfg embeddedstore.EngineConfig) string {
	checkpointValue := fmt.Sprintf("checkpoint_interval=%s", cfg.CheckpointInterval)
	if cfg.CheckpointInterval <= 0 {
		checkpointValue = "checkpoint_strategy=explicit+shutdown"
	}

	return fmt.Sprintf(
		"hot_max_events=%d hot_max_resource_versions=%d flush_interval=%s %s checkpoint_max_tail_events=%d checkpoint_max_tail_bytes=%d checkpoint_on_shutdown=%t segment_target_bytes=%d compaction_min_segments=%d",
		cfg.HotMaxEvents,
		cfg.HotMaxResourceVersions,
		cfg.FlushInterval,
		checkpointValue,
		cfg.CheckpointMaxTailEvents,
		cfg.CheckpointMaxTailBytes,
		cfg.CheckpointOnShutdown,
		cfg.SegmentTargetBytes,
		cfg.CompactionMinSegments,
	)
}

func embeddedImportAPIEnabled(mode serverRuntimeMode) bool {
	return mode.Embedded && mode.StartWatcher
}

func hasUsableEmbeddedBackend(backend *embeddedstore.Backend) (bool, error) {
	if backend == nil {
		return false, fmt.Errorf("embedded backend is nil")
	}
	executor := backend.QueryExecutor()
	if executor == nil {
		return false, fmt.Errorf("embedded query executor is nil")
	}
	return hasUsableEmbeddedEvents(executor)
}

func shouldSkipEmbeddedInitialListReplay(backend *embeddedstore.Backend, logger *logging.Logger) bool {
	if backend == nil {
		logger.Warn("Failed to determine whether embedded watcher should skip initial List replay: embedded backend is nil")
		return false
	}
	return backend.HasUsableResourceState()
}

func embeddedImportSourceDescription(importPath, dataDir string) string {
	if importPath != "" {
		return fmt.Sprintf("import path %s", importPath)
	}
	return fmt.Sprintf("data dir %s", dataDir)
}

func hasUsableEmbeddedEvents(executor embeddedMetadataQuerier) (bool, error) {
	if executor == nil {
		return false, fmt.Errorf("embedded executor is nil")
	}

	namespaces, kinds, _, _, err := executor.QueryDistinctMetadata(context.Background(), 0, math.MaxInt64)
	if err != nil {
		return false, err
	}
	if len(namespaces) == 0 && len(kinds) == 0 {
		return false, nil
	}
	return true, nil
}
