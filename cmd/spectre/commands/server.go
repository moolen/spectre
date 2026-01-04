package commands

import (
	"context"
	"fmt"
	"net/http"

	//nolint:gosec // We are using pprof for debugging
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/apiserver"
	"github.com/moolen/spectre/internal/config"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graphservice"
	"github.com/moolen/spectre/internal/importexport"
	"github.com/moolen/spectre/internal/lifecycle"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/tracing"
	"github.com/moolen/spectre/internal/watcher"
	"github.com/spf13/cobra"
)

var (
	apiPort               int
	watcherConfigPath     string
	watcherEnabled        bool
	maxConcurrentRequests int
	importPath            string
	pprofEnabled          bool
	pprofPort             int
	pprofReadTimeout      time.Duration
	pprofWriteTimeout     time.Duration
	pprofIdleTimeout      time.Duration
	tracingEnabled        bool
	tracingEndpoint       string
	tracingTLSCAPath      string
	tracingTLSInsecure    bool
	// Graph reasoning layer flags
	graphEnabled            bool
	graphHost               string
	graphPort               int
	graphName               string
	graphRetentionHours     int
	graphRebuildOnStart     bool
	graphRebuildIfEmpty     bool
	graphRebuildWindowHours int
	// Audit log flag
	auditLogPath string
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the Spectre server",
	Long: `Start the Spectre server which watches Kubernetes events,
stores them, and provides an API for querying and analysis.`,
	Run: runServer,
}

func init() {
	serverCmd.Flags().IntVar(&apiPort, "api-port", 8080, "Port the API server listens on")
	serverCmd.Flags().StringVar(&watcherConfigPath, "watcher-config", "watcher.yaml", "Path to the YAML file containing watcher configuration")
	serverCmd.Flags().BoolVar(&watcherEnabled, "watcher-enabled", true, "Enable Kubernetes watcher (default: true)")
	serverCmd.Flags().IntVar(&maxConcurrentRequests, "max-concurrent-requests", 100, "Maximum number of concurrent API requests")
	serverCmd.Flags().StringVar(&importPath, "import-path", "", "Path to the binary file containing events to import on startup")
	serverCmd.Flags().BoolVar(&pprofEnabled, "pprof-enabled", false, "Enable pprof profiling server (default: false)")
	serverCmd.Flags().IntVar(&pprofPort, "pprof-port", 9999, "Port the pprof server listens on (default: 9999)")
	serverCmd.Flags().DurationVar(&pprofReadTimeout, "pprof-read-timeout", 15*time.Second, "Read timeout for pprof server (default: 15s)")
	serverCmd.Flags().DurationVar(&pprofWriteTimeout, "pprof-write-timeout", 15*time.Second, "Write timeout for pprof server (default: 15s)")
	serverCmd.Flags().DurationVar(&pprofIdleTimeout, "pprof-idle-timeout", 60*time.Second, "Idle timeout for pprof server (default: 60s)")
	serverCmd.Flags().BoolVar(&tracingEnabled, "tracing-enabled", false, "Enable OpenTelemetry tracing (default: false)")
	serverCmd.Flags().StringVar(&tracingEndpoint, "tracing-endpoint", "", "OTLP gRPC endpoint for traces (e.g., victorialogs:4317)")
	serverCmd.Flags().StringVar(&tracingTLSCAPath, "tracing-tls-ca", "", "Path to CA certificate for TLS verification (optional)")
	serverCmd.Flags().BoolVar(&tracingTLSInsecure, "tracing-tls-insecure", false, "Skip TLS certificate verification (insecure, use only for testing)")

	// Graph reasoning layer flags
	serverCmd.Flags().BoolVar(&graphEnabled, "graph-enabled", false, "Enable graph-based reasoning layer (default: false)")
	serverCmd.Flags().StringVar(&graphHost, "graph-host", "localhost", "FalkorDB host (default: localhost)")
	serverCmd.Flags().IntVar(&graphPort, "graph-port", 6379, "FalkorDB port (default: 6379)")
	serverCmd.Flags().StringVar(&graphName, "graph-name", "spectre", "FalkorDB graph name (default: spectre)")
	serverCmd.Flags().IntVar(&graphRetentionHours, "graph-retention-hours", 168, "Graph data retention window in hours (default: 168 = 7 days)")
	serverCmd.Flags().BoolVar(&graphRebuildOnStart, "graph-rebuild-on-start", false, "Rebuild graph on startup (default: false)")
	serverCmd.Flags().BoolVar(&graphRebuildIfEmpty, "graph-rebuild-if-empty", true, "Only rebuild if graph is empty (default: true)")
	serverCmd.Flags().IntVar(&graphRebuildWindowHours, "graph-rebuild-window-hours", 168, "Time window for graph rebuild in hours (default: 168 = 7 days)")

	// Audit log flag
	serverCmd.Flags().StringVar(&auditLogPath, "audit-log", "",
		"Path to write event audit log (JSONL format) for test fixtures. "+
			"If empty, audit logging is disabled.")
}

func runServer(cmd *cobra.Command, args []string) {
	// Load configuration
	cfg := config.LoadConfig(
		apiPort,
		logLevelFlags,
		watcherConfigPath,
		maxConcurrentRequests,
		tracingEnabled,
		tracingEndpoint,
		tracingTLSCAPath,
		tracingTLSInsecure,
	)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		HandleError(err, "Configuration error")
	}

	// Setup logging
	if err := setupLog(cfg.LogLevelFlags); err != nil {
		HandleError(err, "Failed to setup logging")
	}
	logger := logging.GetLogger("server")

	logger.Info("Starting Spectre v%s", Version)
	logger.Debug("Configuration loaded: APIPort=%d", cfg.APIPort)

	manager := lifecycle.NewManager()
	logger.Info("Lifecycle manager created")

	// Initialize tracing provider
	tracingCfg := tracing.Config{
		Enabled:     cfg.TracingEnabled,
		Endpoint:    cfg.TracingEndpoint,
		TLSCAPath:   cfg.TracingTLSCAPath,
		TLSInsecure: cfg.TracingTLSInsecure,
	}
	tracingProvider, err := tracing.NewTracingProvider(tracingCfg)
	if err != nil {
		logger.Warn("Failed to initialize tracing (continuing without tracing): %v", err)
		tracingProvider = nil
	}

	// Register tracing provider (no dependencies)
	if tracingProvider != nil {
		if err := manager.Register(tracingProvider); err != nil {
			logger.Error("Failed to register tracing provider: %v", err)
			HandleError(err, "Tracing registration error")
		}
	}

	// Start pprof server if enabled
	if pprofEnabled {
		go func() {
			pprofAddr := fmt.Sprintf(":%d", pprofPort)
			logger.Info("Starting pprof server on %s", pprofAddr)
			if err := http.ListenAndServe(pprofAddr, nil); err != nil { //nolint:gosec // We are using pprof for debugging
				logger.Error("pprof server failed: %v", err)
			}
		}()
	}

	// Graph is required - check if enabled
	if !graphEnabled {
		logger.Error("Graph must be enabled - graph is now the only storage backend")
		HandleError(fmt.Errorf("graph-enabled flag must be set to true"), "Configuration error")
	}

	var watcherComponent *watcher.Watcher
	var graphQueryExecutor api.QueryExecutor
	var auditLogWriter *watcher.FileAuditLogWriter

	// Initialize audit log if enabled
	if auditLogPath != "" {
		logger.Info("Event audit logging enabled: %s", auditLogPath)
		var err error
		auditLogWriter, err = watcher.NewFileAuditLogWriter(auditLogPath)
		if err != nil {
			logger.Error("Failed to create audit log writer: %v", err)
			HandleError(err, "Audit log initialization error")
		}
	}

	// Initialize graph service
	logger.Info("Initializing graph service")

	graphConfig := graph.ClientConfig{
		Host:         graphHost,
		Port:         graphPort,
		GraphName:    graphName,
		MaxRetries:   10,               // Increased to wait for FalkorDB sidecar to be ready
		DialTimeout:  10 * time.Second, // Increased to allow sidecar startup time
		ReadTimeout:  30 * time.Second, // Increased for complex root cause queries
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
	}

	serviceConfig := graphservice.ServiceConfig{
		GraphConfig:        graphConfig,
		PipelineConfig:     graphservice.DefaultServiceConfig().PipelineConfig,
		RebuildOnStart:     graphRebuildOnStart,
		RebuildWindow:      time.Duration(graphRebuildWindowHours) * time.Hour,
		RebuildIfEmptyOnly: graphRebuildIfEmpty,
		AutoStartPipeline:  true,
	}

	// Set retention window from flag
	serviceConfig.PipelineConfig.RetentionWindow = time.Duration(graphRetentionHours) * time.Hour

	graphServiceComponent := graphservice.NewService(serviceConfig)

	// Initialize graph service (no storage rebuild)
	if err := graphServiceComponent.Initialize(context.Background()); err != nil {
		logger.Error("Failed to initialize graph service: %v", err)
		HandleError(err, "Graph service initialization error")
	}
	logger.Info("Graph service initialized successfully")

	// Create graph query executor
	graphClient := graphServiceComponent.GetClient()
	graphQueryExecutor = graph.NewQueryExecutor(graphClient)
	logger.Info("Graph query executor created")

	// Initialize watcher if enabled
	if watcherEnabled {
		// Get the graph pipeline from the service
		pipeline := graphServiceComponent.GetPipeline()

		// Create handler with graph-only mode
		eventHandler := watcher.NewEventCaptureHandlerWithMode(nil, pipeline, watcher.TimelineModeGraph)
		if auditLogWriter != nil {
			eventHandler.SetAuditLog(auditLogWriter)
		}

		var err error
		watcherComponent, err = watcher.New(eventHandler, cfg.WatcherConfigPath)
		if err != nil {
			logger.Error("Failed to create watcher component: %v", err)
			HandleError(err, "Watcher initialization error")
		}
		logger.Info("Watcher component created (graph-only mode)")
	} else {
		logger.Info("Watcher disabled - running in read-only mode")
	}

	// Set up readiness checker: use watcher if available, otherwise use no-op
	var readinessChecker apiserver.ReadinessChecker
	if watcherComponent != nil {
		readinessChecker = watcherComponent
	} else {
		readinessChecker = &apiserver.NoOpReadinessChecker{}
	}

	// Use graph as the query source
	querySource := api.TimelineQuerySourceGraph
	logger.Info("Timeline query source: GRAPH")

	// Get graph pipeline for imports
	graphPipeline := graphServiceComponent.GetPipeline()

	// Import events from file or directory if import path is specified
	if importPath != "" {
		logger.Info("Importing events from path: %s", importPath)
		importStartTime := time.Now()

		eventValues, err := importexport.Import(importexport.FromPath(importPath), importexport.WithLogger(logger))
		if err != nil {
			logger.Error("Failed to import events from path: %v", err)
			HandleError(err, "Import error")
		}

		logger.InfoWithFields("Parsed import path",
			logging.Field("event_count", len(eventValues)),
			logging.Field("parse_duration", time.Since(importStartTime)))

		// Process events through graph pipeline
		importCtx, importCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer importCancel()

		processStartTime := time.Now()
		if err := graphPipeline.ProcessBatch(importCtx, eventValues); err != nil {
			logger.Error("Failed to process imported events: %v", err)
			HandleError(err, "Import processing error")
		}

		processDuration := time.Since(processStartTime)
		totalDuration := time.Since(importStartTime)
		logger.InfoWithFields("Import completed",
			logging.Field("event_count", len(eventValues)),
			logging.Field("process_duration", processDuration),
			logging.Field("total_duration", totalDuration))
	}

	apiComponent := apiserver.NewWithStorageGraphAndPipeline(
		cfg.APIPort,
		nil, // No storage executor
		graphQueryExecutor,
		querySource,
		nil, // No storage component
		graphClient,
		graphPipeline,
		readinessChecker,
		false, // No demo mode
		tracingProvider,
	)
	logger.Info("API server component created (graph-only)")

	// Register components
	// Only register watcher if it was initialized
	if watcherComponent != nil {
		if err := manager.Register(watcherComponent); err != nil {
			logger.Error("Failed to register watcher component: %v", err)
			HandleError(err, "Watcher registration error")
		}
	}

	// Register graph service
	if err := manager.Register(graphServiceComponent); err != nil {
		logger.Error("Failed to register graph service component: %v", err)
		HandleError(err, "Graph service registration error")
	}

	if err := manager.Register(apiComponent); err != nil {
		logger.Error("Failed to register API server component: %v", err)
		HandleError(err, "API server registration error")
	}

	logger.Info("All components registered with dependencies")
	ctx, cancel := context.WithCancel(context.Background())
	if err := manager.Start(ctx); err != nil {
		logger.Error("Failed to start components: %v", err)
		HandleError(err, "Startup error")
	}

	logger.Info("Application started successfully")
	logger.Info("Listening for events and API requests...")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigChan
	logger.Info("Shutdown signal received, gracefully shutting down...")
	cancel()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := manager.Stop(shutdownCtx); err != nil {
		logger.Error("Error during shutdown: %v", err)
	}

	// Close audit log if it was initialized
	if auditLogWriter != nil {
		if err := auditLogWriter.Close(); err != nil {
			logger.Error("Failed to close audit log: %v", err)
		}
	}

	logger.Info("Shutdown complete")
}
