package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/config"
	"github.com/moolen/spectre/internal/importexport"
	"github.com/moolen/spectre/internal/lifecycle"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/storage"
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
			queryExecutor, err = storage.NewQueryExecutorWithCache(storageComponent, cacheMaxMB)
			if err != nil {
				logger.Error("Failed to create cache: %v", err)
				HandleError(err, "Cache initialization error")
			}
			logger.Info("Block cache enabled with max size: %dMB", cacheMaxMB)
		} else {
			queryExecutor = storage.NewQueryExecutor(storageComponent)
			logger.Info("Block cache disabled")
		}
	}

	apiComponent := api.NewWithStorage(cfg.APIPort, queryExecutor, storageComponent, watcherComponent, demo)
	logger.Info("API server component created")

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
