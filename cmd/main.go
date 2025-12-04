package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/config"
	"github.com/moolen/spectre/internal/lifecycle"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/storage"
	"github.com/moolen/spectre/internal/watcher"
)

// Version is the application version
const Version = "0.1.0"

func main() {
	// Parse command line flags
	version := flag.Bool("version", false, "Show version and exit")
	dataDir := flag.String("data-dir", "/data", "Directory where events are stored")
	apiPort := flag.Int("api-port", 8080, "Port the API server listens on")
	logLevel := flag.String("log-level", "info", "Logging level (debug, info, warn, error)")
	watcherConfigPath := flag.String("watcher-config", "watcher.yaml", "Path to the YAML file containing watcher configuration")
	segmentSize := flag.Int64("segment-size", 10*1024*1024, "Target size for compression segments in bytes (default: 10MB)")
	maxConcurrentRequests := flag.Int("max-concurrent-requests", 100, "Maximum number of concurrent API requests")
	cacheMaxMB := flag.Int64("cache-max-mb", 100, "Maximum memory for block cache in MB (default: 100MB)")
	cacheEnabled := flag.Bool("cache-enabled", true, "Enable block cache (default: true)")

	flag.Parse()

	// Handle version flag
	if *version {
		fmt.Printf("Kubernetes Event Monitor v%s\n", Version)
		os.Exit(0)
	}

	// Load configuration
	cfg := config.LoadConfig(
		*dataDir,
		*apiPort,
		*logLevel,
		*watcherConfigPath,
		*segmentSize,
		*maxConcurrentRequests,
		*cacheMaxMB,
		*cacheEnabled,
	)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	logging.Initialize(cfg.LogLevel)
	logger := logging.GetLogger("main")

	logger.Info("Starting Kubernetes Event Monitor v%s", Version)
	logger.Debug("Configuration loaded: DataDir=%s, APIPort=%d, LogLevel=%s", cfg.DataDir, cfg.APIPort, cfg.LogLevel)

	manager := lifecycle.NewManager()
	logger.Info("Lifecycle manager created")

	storageComponent, err := storage.New(cfg.DataDir, cfg.SegmentSize)
	if err != nil {
		logger.Error("Failed to create storage component: %v", err)
		fmt.Fprintf(os.Stderr, "Storage initialization error: %v\n", err)
		os.Exit(1)
	}
	logger.Info("Storage component created")

	watcherComponent, err := watcher.New(watcher.NewEventCaptureHandler(storageComponent), cfg.WatcherConfigPath)
	if err != nil {
		logger.Error("Failed to create watcher component: %v", err)
		fmt.Fprintf(os.Stderr, "Watcher initialization error: %v\n", err)
		os.Exit(1)
	}
	logger.Info("Watcher component created")

	// Create query executor with or without cache based on CLI flag
	var queryExecutor *storage.QueryExecutor
	if *cacheEnabled {
		var err error
		queryExecutor, err = storage.NewQueryExecutorWithCache(storageComponent, *cacheMaxMB)
		if err != nil {
			logger.Error("Failed to create cache: %v", err)
			fmt.Fprintf(os.Stderr, "Cache initialization error: %v\n", err)
			os.Exit(1)
		}
		logger.Info("Block cache enabled with max size: %dMB", *cacheMaxMB)
	} else {
		queryExecutor = storage.NewQueryExecutor(storageComponent)
		logger.Info("Block cache disabled")
	}

	apiComponent := api.NewWithStorage(cfg.APIPort, queryExecutor, storageComponent, watcherComponent)
	logger.Info("API server component created")

	if err := manager.Register(storageComponent); err != nil {
		logger.Error("Failed to register storage component: %v", err)
		fmt.Fprintf(os.Stderr, "Storage registration error: %v\n", err)
		os.Exit(1)
	}

	if err := manager.Register(watcherComponent, storageComponent); err != nil {
		logger.Error("Failed to register watcher component: %v", err)
		fmt.Fprintf(os.Stderr, "Watcher registration error: %v\n", err)
		os.Exit(1)
	}

	if err := manager.Register(apiComponent, storageComponent); err != nil {
		logger.Error("Failed to register API server component: %v", err)
		fmt.Fprintf(os.Stderr, "API server registration error: %v\n", err)
		os.Exit(1)
	}

	logger.Info("All components registered with dependencies")
	ctx, cancel := context.WithCancel(context.Background())
	if err := manager.Start(ctx); err != nil {
		logger.Error("Failed to start components: %v", err)
		fmt.Fprintf(os.Stderr, "Startup error: %v\n", err)
		os.Exit(1)
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

	manager.Stop(shutdownCtx)
	logger.Info("Shutdown complete")
	os.Exit(0)
}
