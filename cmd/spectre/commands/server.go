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
	"github.com/moolen/spectre/internal/config"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graphservice"
	"github.com/moolen/spectre/internal/importexport"
	"github.com/moolen/spectre/internal/lifecycle"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"github.com/moolen/spectre/internal/storage"
	"github.com/moolen/spectre/internal/tracing"
	"github.com/moolen/spectre/internal/watcher"
	"github.com/spf13/cobra"
)

var (
	demo                  bool
	dataDir               string
	apiPort               int
	watcherConfigPath     string
	watcherEnabled        bool
	segmentSize           int64
	maxConcurrentRequests int
	cacheMaxMB            int64
	cacheEnabled          bool
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
	// Timeline mode flags
	timelineMode        string // storage, graph, or both
	timelineQuerySource string // storage or graph
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the Spectre server",
	Long: `Start the Spectre server which watches Kubernetes events,
stores them, and provides an API for querying and analysis.`,
	Run: runServer,
}

func init() {
	serverCmd.Flags().BoolVar(&demo, "demo", false, "Run in demo mode with embedded demo data")
	serverCmd.Flags().StringVar(&dataDir, "data-dir", "/data", "Directory where events are stored")
	serverCmd.Flags().IntVar(&apiPort, "api-port", 8080, "Port the API server listens on")
	serverCmd.Flags().StringVar(&watcherConfigPath, "watcher-config", "watcher.yaml", "Path to the YAML file containing watcher configuration")
	serverCmd.Flags().BoolVar(&watcherEnabled, "watcher-enabled", true, "Enable Kubernetes watcher (default: true)")
	serverCmd.Flags().Int64Var(&segmentSize, "segment-size", 10*1024*1024, "Target size for compression segments in bytes (default: 10MB)")
	serverCmd.Flags().IntVar(&maxConcurrentRequests, "max-concurrent-requests", 100, "Maximum number of concurrent API requests")
	serverCmd.Flags().Int64Var(&cacheMaxMB, "cache-max-mb", 100, "Maximum memory for block cache in MB (default: 100MB)")
	serverCmd.Flags().BoolVar(&cacheEnabled, "cache-enabled", true, "Enable block cache (default: true)")
	serverCmd.Flags().StringVar(&importPath, "import", "", "Import JSON event file or directory before starting server")
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
	serverCmd.Flags().BoolVar(&graphRebuildOnStart, "graph-rebuild-on-start", true, "Rebuild graph from storage on startup (default: true)")
	serverCmd.Flags().BoolVar(&graphRebuildIfEmpty, "graph-rebuild-if-empty", true, "Only rebuild if graph is empty (default: true)")
	serverCmd.Flags().IntVar(&graphRebuildWindowHours, "graph-rebuild-window-hours", 168, "Time window for graph rebuild in hours (default: 168 = 7 days)")

	// Timeline migration flags
	serverCmd.Flags().StringVar(&timelineMode, "timeline-mode", "graph", "Timeline write mode: storage, graph, or both")
	serverCmd.Flags().StringVar(&timelineQuerySource, "timeline-query-source", "graph", "Timeline query source: storage or graph")
}

func runServer(cmd *cobra.Command, args []string) {
	// Load configuration
	cfg := config.LoadConfig(
		dataDir,
		apiPort,
		logLevel,
		watcherConfigPath,
		segmentSize,
		maxConcurrentRequests,
		cacheMaxMB,
		cacheEnabled,
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
	if err := setupLog(cfg.LogLevel); err != nil {
		HandleError(err, "Failed to setup logging")
	}
	logger := logging.GetLogger("server")

	if demo {
		logger.Info("Starting Spectre v%s [DEMO MODE]", Version)
	} else {
		logger.Info("Starting Spectre v%s", Version)
	}
	logger.Debug("Configuration loaded: DataDir=%s, APIPort=%d, LogLevel=%s", cfg.DataDir, cfg.APIPort, cfg.LogLevel)

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

	// In demo mode, skip storage and watcher initialization
	var storageComponent *storage.Storage
	var watcherComponent *watcher.Watcher
	var queryExecutor api.QueryExecutor

	if demo {
		logger.Info("Demo mode enabled - using embedded demo data")
		demoExecutor := api.NewDemoQueryExecutor()
		queryExecutor = demoExecutor
		logger.Info("Demo query executor created")
	} else {
		var err error
		storageComponent, err = storage.New(cfg.DataDir, cfg.SegmentSize)
		if err != nil {
			logger.Error("Failed to create storage component: %v", err)
			HandleError(err, "Storage initialization error")
		}
		logger.Info("Storage component created")

		// Handle import if --import flag is provided
		if importPath != "" {
			// Check if path is a file or directory
			info, err := os.Stat(importPath)
			if err != nil {
				logger.Error("Failed to access import path: %v", err)
				HandleError(err, "Import path error")
			}

			importOpts := storage.ImportOptions{
				ValidateFiles:     true,
				OverwriteExisting: true,
			}

			var report *importexport.ImportReport

			if info.IsDir() {
				// Import directory
				logger.Info("Starting import from directory: %s", importPath)
				fmt.Printf("Importing events from directory: %s\n", importPath)

				filesProcessed := 0
				progressCallback := func(filename string, eventCount int) {
					filesProcessed++
					fmt.Printf("  [%d] Loaded %d events from %s\n", filesProcessed, eventCount, filename)
				}

				report, err = importexport.WalkAndImportJSON(importPath, storageComponent, importOpts, progressCallback)
				if err != nil {
					logger.Error("Import failed: %v", err)
					HandleError(err, "Import error")
				}
			} else {
				// Import single file
				logger.Info("Starting import from file: %s", importPath)
				fmt.Printf("Importing events from file: %s\n", importPath)

				startTime := time.Now()

				// Read the JSON file
				events, err := importexport.ImportJSONFile(importPath)
				if err != nil {
					logger.Error("Failed to read file: %v", err)
					HandleError(err, "Import file error")
				}

				fmt.Printf("  Loaded %d events from %s\n", len(events), importPath)

				// Import the events
				storageReport, err := storageComponent.AddEventsBatch(events, importOpts)
				if err != nil {
					logger.Error("Import failed: %v", err)
					HandleError(err, "Import error")
				}

				// Convert storage report to import report
				report = &importexport.ImportReport{
					TotalFiles:    1,
					ImportedFiles: storageReport.ImportedFiles,
					MergedHours:   storageReport.MergedHours,
					SkippedFiles:  storageReport.SkippedFiles,
					FailedFiles:   storageReport.FailedFiles,
					TotalEvents:   storageReport.TotalEvents,
					Errors:        storageReport.Errors,
					Duration:      time.Since(startTime),
				}
			}

			fmt.Println("\n" + importexport.FormatImportReport(report))
			logger.Info("Import completed successfully")
		}

		// Only initialize watcher if enabled
		if watcherEnabled {
			watcherComponent, err = watcher.New(watcher.NewEventCaptureHandler(storageComponent), cfg.WatcherConfigPath)
			if err != nil {
				logger.Error("Failed to create watcher component: %v", err)
				HandleError(err, "Watcher initialization error")
			}
			logger.Info("Watcher component created")
		} else {
			logger.Info("Watcher disabled - running in read-only mode")
		}

		// Create query executor with or without cache based on CLI flag
		if cacheEnabled {
			var err error
			queryExecutor, err = storage.NewQueryExecutorWithCache(storageComponent, cacheMaxMB, tracingProvider)
			if err != nil {
				logger.Error("Failed to create cache: %v", err)
				HandleError(err, "Cache initialization error")
			}
			logger.Info("Block cache enabled with max size: %dMB", cacheMaxMB)
		} else {
			queryExecutor = storage.NewQueryExecutor(storageComponent, tracingProvider)
			logger.Info("Block cache disabled")
		}
	}

	// Initialize graph service if enabled
	var graphServiceComponent *graphservice.Service
	var graphQueryExecutor api.QueryExecutor
	if graphEnabled && !demo {
		logger.Info("Graph reasoning layer enabled - initializing graph service")

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

		graphServiceComponent = graphservice.NewService(serviceConfig)

		// Create storage adapter for graph service
		storageAdapter := &graphStorageAdapter{
			executor: storage.NewQueryExecutor(storageComponent, tracingProvider),
		}

		// Initialize graph service with storage backend
		if err := graphServiceComponent.InitializeWithStorage(context.Background(), storageAdapter); err != nil {
			logger.Error("Failed to initialize graph service: %v", err)
			logger.Warn("Graph reasoning layer will be disabled - continuing without graph support")
			graphServiceComponent = nil
		} else {
			logger.Info("Graph service initialized successfully")

			// Create graph query executor
			graphClient := graphServiceComponent.GetClient()
			graphQueryExecutor = graph.NewQueryExecutor(graphClient)
			logger.Info("Graph query executor created")

			// Determine timeline mode
			var mode watcher.TimelineMode
			switch timelineMode {
			case "graph":
				mode = watcher.TimelineModeGraph
				logger.Info("Timeline mode: GRAPH (direct writes to graph, bypassing storage)")
			case "both":
				mode = watcher.TimelineModeBoth
				logger.Info("Timeline mode: BOTH (writes to both storage and graph for validation)")
			default:
				mode = watcher.TimelineModeStorage
				logger.Info("Timeline mode: STORAGE (default, writes to storage only)")
			}

			// Update watcher handler if watcher is enabled
			if watcherEnabled && watcherComponent != nil {
				logger.Info("Updating watcher with dual-mode handler (mode: %s)", timelineMode)

				// Get the graph pipeline from the service
				pipeline := graphServiceComponent.GetPipeline()

				// Create new handler with mode
				eventHandler := watcher.NewEventCaptureHandlerWithMode(storageComponent, pipeline, mode)

				// Recreate watcher with new handler
				watcherComponent, err = watcher.New(eventHandler, cfg.WatcherConfigPath)
				if err != nil {
					logger.Error("Failed to recreate watcher with dual-mode handler: %v", err)
					HandleError(err, "Watcher reinitialization error")
				}
				logger.Info("Watcher updated with dual-mode handler")
			}

			// Only register storage callback if mode is "storage" (not "both" or "graph")
			// When mode is "both", the watcher handler directly writes to the graph pipeline,
			// so we don't need the storage callback to also forward events (which would cause double processing)
			// When mode is "graph", no storage writes happen, so the callback wouldn't fire anyway
			if mode == watcher.TimelineModeStorage {
				storageComponent.RegisterCallback(func(event models.Event) error {
					logger.Debug("Storage callback: forwarding event %s to graph service", event.ID)
					return graphServiceComponent.OnEvent(event)
				})
				logger.Info("Registered graph service callback with storage")
			}
		}
	}

	// Set up readiness checker: use watcher if available, otherwise use no-op
	var readinessChecker api.ReadinessChecker
	if watcherComponent != nil {
		readinessChecker = watcherComponent
	} else {
		readinessChecker = &api.NoOpReadinessChecker{}
	}

	// Get graph client if graph service is available
	var graphClient graph.Client
	if graphServiceComponent != nil {
		graphClient = graphServiceComponent.GetClient()
	}

	// Determine timeline query source
	var querySource api.TimelineQuerySource
	switch timelineQuerySource {
	case "graph":
		if graphQueryExecutor != nil {
			querySource = api.TimelineQuerySourceGraph
			logger.Info("Timeline query source: GRAPH")
		} else {
			querySource = api.TimelineQuerySourceStorage
			logger.Warn("Graph query source requested but not available, using storage")
		}
	default:
		querySource = api.TimelineQuerySourceStorage
		logger.Info("Timeline query source: STORAGE (default)")
	}

	apiComponent := api.NewWithStorageAndGraph(
		cfg.APIPort,
		queryExecutor,
		graphQueryExecutor,
		querySource,
		storageComponent,
		graphClient,
		readinessChecker,
		demo,
		tracingProvider,
	)
	logger.Info("API server component created (query source: %s)", timelineQuerySource)

	// Register components based on demo mode
	if !demo {
		if err := manager.Register(storageComponent); err != nil {
			logger.Error("Failed to register storage component: %v", err)
			HandleError(err, "Storage registration error")
		}

		// Only register watcher if it was initialized
		if watcherComponent != nil {
			if err := manager.Register(watcherComponent, storageComponent); err != nil {
				logger.Error("Failed to register watcher component: %v", err)
				HandleError(err, "Watcher registration error")
			}
		}

		// Register graph service if initialized (depends on storage)
		if graphServiceComponent != nil {
			if err := manager.Register(graphServiceComponent, storageComponent); err != nil {
				logger.Error("Failed to register graph service component: %v", err)
				HandleError(err, "Graph service registration error")
			}
		}

		if err := manager.Register(apiComponent, storageComponent); err != nil {
			logger.Error("Failed to register API server component: %v", err)
			HandleError(err, "API server registration error")
		}
	} else {
		if err := manager.Register(apiComponent); err != nil {
			logger.Error("Failed to register API server component: %v", err)
			HandleError(err, "API server registration error")
		}
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
	logger.Info("Shutdown complete")
}

// graphStorageAdapter adapts Spectre's QueryExecutor to the graph sync StorageQuerier interface
type graphStorageAdapter struct {
	executor api.QueryExecutor
}

func (a *graphStorageAdapter) Query(ctx context.Context, request models.QueryRequest) (*models.QueryResult, error) {
	return a.executor.Execute(ctx, &request)
}
