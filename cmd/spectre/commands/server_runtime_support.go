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

	"github.com/mark3labs/mcp-go/server"
	"github.com/moolen/spectre/internal/config"
	"github.com/moolen/spectre/internal/lifecycle"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/tracing"
	"github.com/moolen/spectre/internal/watcher"
	"go.opentelemetry.io/otel/trace"
)

func ensureDefaultIntegrationsConfig(mode serverRuntimeMode, logger *logging.Logger) {
	if mode.Embedded || integrationsConfigPath == "" {
		return
	}
	if _, err := os.Stat(integrationsConfigPath); !os.IsNotExist(err) {
		return
	}

	logger.Info("Creating default integrations config file: %s", integrationsConfigPath)
	defaultConfig := &config.IntegrationsFile{
		SchemaVersion: "v1",
		Instances:     []config.IntegrationConfig{},
	}
	if err := config.WriteIntegrationsFile(integrationsConfigPath, defaultConfig); err != nil {
		logger.Error("Failed to create default integrations config: %v", err)
		HandleError(err, "Integration config creation error")
	}
}

func initializeTracingProvider(cfg *config.Config, manager *lifecycle.Manager, logger *logging.Logger) *tracing.TracingProvider {
	tracingProvider, err := tracing.NewTracingProvider(tracing.Config{
		Enabled:     cfg.TracingEnabled,
		Endpoint:    cfg.TracingEndpoint,
		TLSCAPath:   cfg.TracingTLSCAPath,
		TLSInsecure: cfg.TracingTLSInsecure,
	})
	if err != nil {
		logger.Warn("Failed to initialize tracing (continuing without tracing): %v", err)
		return nil
	}
	if tracingProvider == nil {
		return nil
	}
	if err := manager.Register(tracingProvider); err != nil {
		logger.Error("Failed to register tracing provider: %v", err)
		HandleError(err, "Tracing registration error")
	}
	return tracingProvider
}

func startPprofServer(logger *logging.Logger) {
	if !pprofEnabled {
		return
	}

	go func() {
		pprofAddr := fmt.Sprintf(":%d", pprofPort)
		logger.Info("Starting pprof server on %s", pprofAddr)
		if err := http.ListenAndServe(pprofAddr, nil); err != nil { //nolint:gosec // We are using pprof for debugging
			logger.Error("pprof server failed: %v", err)
		}
	}()
}

func startManagedComponents(manager *lifecycle.Manager, logger *logging.Logger) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	if err := manager.Start(ctx); err != nil {
		logger.Error("Failed to start components: %v", err)
		HandleError(err, "Startup error")
	}
	return cancel
}

func startStdioTransport(mcpServer *server.MCPServer, logger *logging.Logger) {
	if !stdioEnabled || mcpServer == nil {
		return
	}

	logger.Info("Starting stdio MCP transport alongside HTTP")
	go func() {
		if err := server.ServeStdio(mcpServer); err != nil {
			logger.Error("Stdio transport error: %v", err)
		}
	}()
}

func waitForShutdown(manager *lifecycle.Manager, cancel context.CancelFunc, auditLogWriter *watcher.FileAuditLogWriter, logger *logging.Logger) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutdown signal received, gracefully shutting down...")
	if cancel != nil {
		cancel()
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := manager.Stop(shutdownCtx); err != nil {
		logger.Error("Error during shutdown: %v", err)
	}
	if auditLogWriter != nil {
		if err := auditLogWriter.Close(); err != nil {
			logger.Error("Failed to close audit log: %v", err)
		}
	}

	logger.Info("Shutdown complete")
}

func getTracingProviderTracer(provider interface {
	GetTracer(string) trace.Tracer
	IsEnabled() bool
}, name string) trace.Tracer {
	if provider == nil {
		return nil
	}
	return provider.GetTracer(name)
}
