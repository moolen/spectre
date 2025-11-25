package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/moritz/rpk/internal/config"
	"github.com/moritz/rpk/internal/logging"
)

// Version is the application version
const Version = "0.1.0"

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

	// Create context for graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// TODO: Initialize watchers
	// TODO: Initialize storage
	// TODO: Initialize API server
	// TODO: Start watchers
	// TODO: Start API server

	logger.Info("Application started successfully")
	logger.Info("Listening for events and API requests...")

	// Wait for shutdown signal
	<-sigChan
	logger.Info("Shutdown signal received, gracefully shutting down...")

	// Cancel context to stop all goroutines
	cancel()

	// TODO: Stop watchers
	// TODO: Stop API server
	// TODO: Flush storage

	logger.Info("Shutdown complete")
	os.Exit(0)
}
