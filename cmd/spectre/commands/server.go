package commands

import (
	"time"

	"github.com/moolen/spectre/internal/config"
	"github.com/moolen/spectre/internal/lifecycle"
	"github.com/moolen/spectre/internal/logging"
	"github.com/spf13/cobra"
)

var (
	apiPort                             int
	watcherConfigPath                   string
	watcherEnabled                      bool
	maxConcurrentRequests               int
	importPath                          string
	importChunkSize                     int
	importBenchmarkLog                  string
	importMode                          bool
	startupImportDisableCausality       bool
	startupImportTimelineOnly           bool
	embeddedProjectionHistoryFallback   bool
	embeddedFlushInterval               time.Duration
	embeddedRetentionDays               int
	embeddedCheckpointInterval          time.Duration
	embeddedCheckpointRetentionCount    int
	embeddedCheckpointRetentionCountSet bool
	embeddedCheckpointMaxTailEvents     int
	embeddedCheckpointMaxTailBytes      int64
	embeddedCheckpointOnShutdown        bool
	embeddedCheckpointOnShutdownSet     bool
	embeddedSegmentTargetBytes          int64
	embeddedCompactionMinSegments       int
	embeddedAutoCompaction              bool
	dataDir                             string
	pprofEnabled                        bool
	pprofPort                           int
	pprofReadTimeout                    time.Duration
	pprofWriteTimeout                   time.Duration
	pprofIdleTimeout                    time.Duration
	tracingEnabled                      bool
	tracingEndpoint                     string
	tracingTLSCAPath                    string
	tracingTLSInsecure                  bool
	// Audit log flag
	auditLogPath string
	// Metadata cache configuration
	metadataCacheRefreshSeconds int
	// Namespace graph cache configuration
	namespaceGraphCacheEnabled        bool
	namespaceGraphCacheRefreshSeconds int
	namespaceGraphCacheMemoryMB       int
	// Reconciler configuration
	reconcilerEnabled      bool
	reconcilerIntervalMins int
	reconcilerBatchSize    int
	// MCP server configuration
	stdioEnabled bool
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
	serverCmd.Flags().IntVar(&importChunkSize, "import-chunk-size", defaultStartupImportChunkSize, "Chunk size used for startup imports")
	serverCmd.Flags().StringVar(&importBenchmarkLog, "import-benchmark-log", "", "Path to write startup import benchmark report as JSON")
	serverCmd.Flags().BoolVar(&importMode, "import-mode", false, "Enable startup import opt-in mode (reserved for future tuning)")
	serverCmd.Flags().BoolVar(&startupImportDisableCausality, "startup-import-disable-causality", false, "Disable causality inference during startup import only")
	serverCmd.Flags().BoolVar(&startupImportTimelineOnly, "startup-import-timeline-only", false, "Import only timeline-critical graph data during startup import")
	serverCmd.Flags().BoolVar(&embeddedProjectionHistoryFallback, "embedded-projection-history-fallback", false, "Temporarily enable projection history fallback in embedded mode (rollback switch)")
	serverCmd.Flags().DurationVar(
		&embeddedFlushInterval,
		"embedded-flush-interval",
		30*time.Second,
		"Interval between embedded hot-store flushes into segment files",
	)
	serverCmd.Flags().IntVar(
		&embeddedRetentionDays,
		"embedded-retention-days",
		0,
		"Retention window in days for embedded raw events and checkpoint history; 0 disables time-based retention",
	)
	serverCmd.Flags().DurationVar(
		&embeddedCheckpointInterval,
		"embedded-checkpoint-interval",
		15*time.Minute,
		"Interval between durable embedded checkpoints; set to 0 to disable periodic checkpoints",
	)
	serverCmd.Flags().IntVar(
		&embeddedCheckpointRetentionCount,
		"embedded-checkpoint-retention-count",
		3,
		"Number of embedded checkpoints to retain; set to 0 to disable checkpoint pruning",
	)
	serverCmd.Flags().IntVar(
		&embeddedCheckpointMaxTailEvents,
		"embedded-checkpoint-max-tail-events",
		2048,
		"Maximum embedded tail events before forcing checkpoint compaction",
	)
	serverCmd.Flags().Int64Var(
		&embeddedCheckpointMaxTailBytes,
		"embedded-checkpoint-max-tail-bytes",
		16<<20,
		"Maximum embedded tail bytes before forcing checkpoint compaction",
	)
	serverCmd.Flags().BoolVar(
		&embeddedCheckpointOnShutdown,
		"embedded-checkpoint-on-shutdown",
		true,
		"Write an embedded checkpoint during graceful shutdown",
	)
	serverCmd.Flags().Int64Var(
		&embeddedSegmentTargetBytes,
		"embedded-segment-target-bytes",
		16<<20,
		"Approximate embedded segment size target in bytes for size-triggered flushes",
	)
	serverCmd.Flags().IntVar(
		&embeddedCompactionMinSegments,
		"embedded-compaction-min-segments",
		4,
		"Minimum embedded segment count required before compaction runs",
	)
	serverCmd.Flags().BoolVar(
		&embeddedAutoCompaction,
		"embedded-auto-compaction",
		true,
		"Enable automatic embedded compaction after successful checkpoints",
	)
	serverCmd.Flags().StringVar(&dataDir, "data-dir", "./data", "Directory for embedded persistent state")
	serverCmd.Flags().BoolVar(&pprofEnabled, "pprof-enabled", false, "Enable pprof profiling server (default: false)")
	serverCmd.Flags().IntVar(&pprofPort, "pprof-port", 9999, "Port the pprof server listens on (default: 9999)")
	serverCmd.Flags().DurationVar(&pprofReadTimeout, "pprof-read-timeout", 15*time.Second, "Read timeout for pprof server (default: 15s)")
	serverCmd.Flags().DurationVar(&pprofWriteTimeout, "pprof-write-timeout", 15*time.Second, "Write timeout for pprof server (default: 15s)")
	serverCmd.Flags().DurationVar(&pprofIdleTimeout, "pprof-idle-timeout", 60*time.Second, "Idle timeout for pprof server (default: 60s)")
	serverCmd.Flags().BoolVar(&tracingEnabled, "tracing-enabled", false, "Enable OpenTelemetry tracing (default: false)")
	serverCmd.Flags().StringVar(&tracingEndpoint, "tracing-endpoint", "", "OTLP gRPC endpoint for traces (e.g., victorialogs:4317)")
	serverCmd.Flags().StringVar(&tracingTLSCAPath, "tracing-tls-ca", "", "Path to CA certificate for TLS verification (optional)")
	serverCmd.Flags().BoolVar(&tracingTLSInsecure, "tracing-tls-insecure", false, "Skip TLS certificate verification (insecure, use only for testing)")

	// Audit log flag
	serverCmd.Flags().StringVar(&auditLogPath, "audit-log", "",
		"Path to write event audit log (JSONL format) for test fixtures. "+
			"If empty, audit logging is disabled.")

	// Metadata cache configuration
	serverCmd.Flags().IntVar(&metadataCacheRefreshSeconds, "metadata-cache-refresh-seconds", 30,
		"Metadata cache refresh period in seconds (default: 30)")

	// Namespace graph cache configuration
	serverCmd.Flags().BoolVar(&namespaceGraphCacheEnabled, "namespace-graph-cache-enabled", true,
		"Enable namespace graph caching for fast responses (default: true)")
	serverCmd.Flags().IntVar(&namespaceGraphCacheRefreshSeconds, "namespace-graph-cache-refresh-seconds", 120,
		"Namespace graph cache refresh period in seconds (default: 120)")
	serverCmd.Flags().IntVar(&namespaceGraphCacheMemoryMB, "namespace-graph-cache-memory-mb", 256,
		"Maximum memory for namespace graph cache in MB (default: 256)")

	// Reconciler configuration
	serverCmd.Flags().BoolVar(&reconcilerEnabled, "reconciler-enabled", true,
		"Enable graph reconciler to detect missed DELETE events (default: true)")
	serverCmd.Flags().IntVar(&reconcilerIntervalMins, "reconciler-interval-minutes", 5,
		"Reconciliation interval in minutes (default: 5)")
	serverCmd.Flags().IntVar(&reconcilerBatchSize, "reconciler-batch-size", 100,
		"Maximum resources to check per reconciliation cycle (default: 100)")

	// MCP server configuration
	serverCmd.Flags().BoolVar(&stdioEnabled, "stdio", false, "Enable stdio MCP transport alongside HTTP (default: false)")
}

func runServer(cmd *cobra.Command, args []string) {
	syncEmbeddedCheckpointOnShutdownFlagState(cmd)

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

	if err := cfg.Validate(); err != nil {
		HandleError(err, "Configuration error")
	}

	if err := setupLog(cfg.LogLevelFlags); err != nil {
		HandleError(err, "Failed to setup logging")
	}
	logger := logging.GetLogger("server")

	logger.Info("Starting Spectre v%s", Version)
	logger.Debug("Configuration loaded: APIPort=%d", cfg.APIPort)

	mode, err := resolveServerRuntimeMode(serverModeInput{
		WatcherEnabled: watcherEnabled,
		ImportPath:     importPath,
	})
	if err != nil {
		logger.Error("Runtime mode validation failed: %v", err)
		HandleError(err, "Configuration error")
	}
	logger.Info("Server runtime mode: %s", mode.Name)

	manager := lifecycle.NewManager()
	logger.Info("Lifecycle manager created")

	tracingProvider := initializeTracingProvider(cfg, manager, logger)
	startPprofServer(logger)
	runEmbeddedServerRuntime(cfg, mode, manager, tracingProvider, logger)
}

func syncEmbeddedCheckpointOnShutdownFlagState(cmd *cobra.Command) {
	if cmd == nil {
		embeddedCheckpointRetentionCountSet = false
		embeddedCheckpointOnShutdownSet = false
		return
	}
	embeddedCheckpointRetentionCountSet = cmd.Flags().Changed("embedded-checkpoint-retention-count")
	embeddedCheckpointOnShutdownSet = cmd.Flags().Changed("embedded-checkpoint-on-shutdown")
}
