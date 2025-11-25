package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/moritz/rpk/internal/api"
	"github.com/moritz/rpk/internal/config"
	"github.com/moritz/rpk/internal/lifecycle"
	"github.com/moritz/rpk/internal/logging"
	"github.com/moritz/rpk/internal/storage"
	"github.com/moritz/rpk/internal/watcher"
	"k8s.io/apimachinery/pkg/runtime"
)

// Version is the application version
const Version = "0.1.0"

// NoOpEventHandler is a no-op implementation of the EventHandler interface
// Used during startup when the actual event handler is not yet ready
type NoOpEventHandler struct{}

// OnAdd is called when a resource is created
func (h *NoOpEventHandler) OnAdd(obj runtime.Object) error {
	return nil
}

// OnUpdate is called when a resource is updated
func (h *NoOpEventHandler) OnUpdate(oldObj, newObj runtime.Object) error {
	return nil
}

// OnDelete is called when a resource is deleted
func (h *NoOpEventHandler) OnDelete(obj runtime.Object) error {
	return nil
}

func main() {
	// Parse command line flags
	version := flag.Bool("version", false, "Show version and exit")
	healthCheck := flag.Bool("health-check", false, "Run health check and exit")
	flag.Parse()

	// Handle version flag
	if *version {
		fmt.Printf("Kubernetes Event Monitor v%s\n", Version)
		os.Exit(0)
	}

	// Load configuration
	cfg := config.LoadConfig()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// Initialize logging
	logging.Initialize(cfg.LogLevel)
	logger := logging.GetLogger("main")

	logger.Info("Starting Kubernetes Event Monitor v%s", Version)
	logger.Debug("Configuration loaded: DataDir=%s, APIPort=%d, LogLevel=%s", cfg.DataDir, cfg.APIPort, cfg.LogLevel)

	// Handle health check flag
	if *healthCheck {
		logger.Info("Health check: OK")
		os.Exit(0)
	}

	// Create lifecycle manager
	manager := lifecycle.NewManager()
	logger.Info("Lifecycle manager created")

	// Initialize storage component
	storageComponent, err := storage.New(cfg.DataDir, 10*1024*1024) // 10MB segments
	if err != nil {
		logger.Error("Failed to create storage component: %v", err)
		fmt.Fprintf(os.Stderr, "Storage initialization error: %v\n", err)
		os.Exit(1)
	}
	logger.Info("Storage component created")

	// Initialize watcher component (create mock handler for now - will be replaced with actual event handler)
	// For MVP, create a no-op event handler
	watcherComponent, err := watcher.New(&NoOpEventHandler{}, []string{"Pod", "Deployment", "Service"})
	if err != nil {
		logger.Error("Failed to create watcher component: %v", err)
		fmt.Fprintf(os.Stderr, "Watcher initialization error: %v\n", err)
		os.Exit(1)
	}
	logger.Info("Watcher component created")

	// Initialize API server component
	apiComponent := api.New(cfg.APIPort, nil) // TODO: Create QueryExecutor from storage
	logger.Info("API server component created")

	// Register components with dependencies
	// Storage has no dependencies
	if err := manager.Register(storageComponent); err != nil {
		logger.Error("Failed to register storage component: %v", err)
		fmt.Fprintf(os.Stderr, "Storage registration error: %v\n", err)
		os.Exit(1)
	}

	// Watchers depend on storage
	if err := manager.Register(watcherComponent, storageComponent); err != nil {
		logger.Error("Failed to register watcher component: %v", err)
		fmt.Fprintf(os.Stderr, "Watcher registration error: %v\n", err)
		os.Exit(1)
	}

	// API server depends on storage
	if err := manager.Register(apiComponent, storageComponent); err != nil {
		logger.Error("Failed to register API server component: %v", err)
		fmt.Fprintf(os.Stderr, "API server registration error: %v\n", err)
		os.Exit(1)
	}

	logger.Info("All components registered with dependencies")

	// Start all components in dependency order
	if err := manager.Start(context.Background()); err != nil {
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

	// Create context with 30-second timeout for graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop all components in reverse dependency order
	manager.Stop(shutdownCtx)

	logger.Info("Shutdown complete")
	os.Exit(0)
}
