// Package grafana provides Grafana metrics integration for Spectre.
package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/integration"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/observatory"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func init() {
	// Register the Grafana factory with the global registry
	if err := integration.RegisterFactory("grafana", NewGrafanaIntegration); err != nil {
		// Log but don't fail - factory might already be registered in tests
		logger := logging.GetLogger("integration.grafana")
		logger.Warn("Failed to register grafana factory: %v", err)
	}
}

// GrafanaIntegration implements the Integration interface for Grafana.
type GrafanaIntegration struct {
	name           string
	config         *Config              // Full configuration (includes URL and SecretRef)
	client         *GrafanaClient       // Grafana HTTP client
	secretWatcher  *SecretWatcher       // Optional: manages API token from Kubernetes Secret
	syncer         *DashboardSyncer     // Dashboard sync orchestrator
	alertSyncer     *AlertSyncer           // Alert sync orchestrator
	stateSyncer     *AlertStateSyncer      // Alert state sync orchestrator
	metricsSyncer   *MetricsSyncer         // Curated metrics sync orchestrator
	baselineCollector *BaselineCollector    // Baseline collector for anomaly detection
	analysisService *AlertAnalysisService  // Alert analysis service for historical analysis
	graphClient     graph.Client           // Graph client for dashboard sync
	queryService    *GrafanaQueryService   // Query service for MCP tools
	anomalyService  *AnomalyService        // Anomaly detection service for MCP tools
	logger          *logging.Logger
	ctx            context.Context
	cancel         context.CancelFunc

	// Observatory services (Phase 26)
	evidenceService   *ObservatoryEvidenceService // Evidence service for explain/evidence tools
	anomalyAggregator *AnomalyAggregator          // Anomaly aggregator for scoring

	// Observatory multi-provider support (Phase 26.5)
	// Registry-based services enable multi-provider signal aggregation
	observatoryRegistry *observatory.Registry       // Multi-provider registry
	observatoryProvider *GrafanaObservatoryProvider // This integration's provider

	// Scrape target linking (links SignalAnchors to K8s workloads)
	prometheusClient        *PrometheusClient       // Direct Prometheus API client
	prometheusSecretWatcher *SecretWatcher          // Optional: manages Prometheus API token
	scrapeTargetLinker      *ScrapeTargetLinker     // Scrape target linker

	// Signal validation (correlates alerts with signal behavior)
	signalValidationJob *SignalValidationJob // Signal validation job

	// Thread-safe health status
	mu           sync.RWMutex
	healthStatus integration.HealthStatus
}

// SetGraphClient implements integration.GraphClientSetter.
// Sets the graph client for dashboard synchronization and alert syncing.
// This must be called before Start() if dashboard sync is desired.
func (g *GrafanaIntegration) SetGraphClient(client interface{}) {
	if gc, ok := client.(graph.Client); ok {
		g.graphClient = gc
		g.logger.Debug("Graph client set for integration: %s", g.name)
	} else {
		g.logger.Warn("SetGraphClient called with incompatible type: %T", client)
	}
}

// NewGrafanaIntegration creates a new Grafana integration instance.
// Note: Client is initialized in Start() to follow lifecycle pattern.
func NewGrafanaIntegration(name string, configMap map[string]interface{}) (integration.Integration, error) {
	// Parse config map into Config struct
	// First marshal to JSON, then unmarshal to Config (handles nested structures)
	configJSON, err := json.Marshal(configMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	var config Config
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate config
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &GrafanaIntegration{
		name:         name,
		config:       &config,
		client:       nil, // Initialized in Start()
		secretWatcher: nil, // Initialized in Start() if config uses SecretRef
		logger:       logging.GetLogger("integration.grafana." + name),
		healthStatus: integration.Stopped,
	}, nil
}

// Metadata returns the integration's identifying information.
func (g *GrafanaIntegration) Metadata() integration.IntegrationMetadata {
	return integration.IntegrationMetadata{
		Name:        g.name,
		Version:     "1.0.0",
		Description: "Grafana metrics integration",
		Type:        "grafana",
	}
}

// Start initializes the integration and validates connectivity.
func (g *GrafanaIntegration) Start(ctx context.Context) error {
	g.logger.Info("Starting Grafana integration: %s (url: %s)", g.name, g.config.URL)

	// Store context for lifecycle management
	g.ctx, g.cancel = context.WithCancel(ctx)

	// Create SecretWatcher if config uses secret ref
	if g.config.UsesSecretRef() {
		g.logger.Info("Creating SecretWatcher for secret: %s, key: %s",
			g.config.APITokenRef.SecretName, g.config.APITokenRef.Key)

		// Create in-cluster Kubernetes client
		k8sConfig, err := rest.InClusterConfig()
		if err != nil {
			return fmt.Errorf("failed to get in-cluster config: %w", err)
		}
		clientset, err := kubernetes.NewForConfig(k8sConfig)
		if err != nil {
			return fmt.Errorf("failed to create Kubernetes clientset: %w", err)
		}

		// Get current namespace (read from ServiceAccount mount)
		namespace, err := getCurrentNamespace()
		if err != nil {
			return fmt.Errorf("failed to determine namespace: %w", err)
		}

		// Create SecretWatcher
		secretWatcher, err := NewSecretWatcher(
			clientset,
			namespace,
			g.config.APITokenRef.SecretName,
			g.config.APITokenRef.Key,
			g.logger,
		)
		if err != nil {
			return fmt.Errorf("failed to create secret watcher: %w", err)
		}

		// Start SecretWatcher
		if err := secretWatcher.Start(g.ctx); err != nil {
			return fmt.Errorf("failed to start secret watcher: %w", err)
		}

		g.secretWatcher = secretWatcher
		g.logger.Info("SecretWatcher started successfully")
	}

	// Create HTTP client (pass secretWatcher if exists)
	g.client = NewGrafanaClient(g.config, g.secretWatcher, g.logger)

	// Test connectivity (warn on failure but continue - degraded state with auto-recovery)
	if err := g.testConnection(g.ctx); err != nil {
		g.logger.Warn("Failed initial connectivity test (will retry on health checks): %v", err)
		g.setHealthStatus(integration.Degraded)
	} else {
		g.setHealthStatus(integration.Healthy)
	}

	// Start dashboard syncer if graph client is available
	if g.graphClient != nil {
		g.logger.Info("Starting dashboard syncer (sync interval: 1 hour)")
		g.syncer = NewDashboardSyncer(
			g.client,
			g.graphClient,
			g.config,
			g.name, // Integration name
			time.Hour, // Sync interval
			g.logger,
		)
		if err := g.syncer.Start(g.ctx); err != nil {
			g.logger.Warn("Failed to start dashboard syncer: %v (continuing without sync)", err)
			// Don't fail startup - syncer is optional enhancement
		}

		// Start alert syncer
		g.logger.Info("Starting alert syncer (sync interval: 1 hour)")
		graphBuilder := NewGraphBuilder(g.graphClient, g.config, g.name, g.logger)
		g.alertSyncer = NewAlertSyncer(
			g.client,
			g.graphClient,
			graphBuilder,
			g.name, // Integration name
			g.logger,
		)
		if err := g.alertSyncer.Start(g.ctx); err != nil {
			g.logger.Warn("Failed to start alert syncer: %v (continuing without sync)", err)
			// Don't fail startup - syncer is optional enhancement
		}

		// Alert state syncer runs independently from rule syncer (5-min vs 1-hour interval)
		g.logger.Info("Starting alert state syncer (sync interval: 5 minutes)")
		g.stateSyncer = NewAlertStateSyncer(
			g.client,
			g.graphClient,
			graphBuilder,
			g.name, // Integration name
			g.logger,
		)
		if err := g.stateSyncer.Start(g.ctx); err != nil {
			g.logger.Warn("Failed to start alert state syncer: %v (continuing without state tracking)", err)
			// Non-fatal - alert rules still work, just no state timeline
		}

		// Create query service for MCP tools (requires graph client)
		g.queryService = NewGrafanaQueryService(g.client, g.graphClient, g.logger)
		g.logger.Info("Query service created for MCP tools")

		// Create anomaly detection service (requires query service and graph client)
		detector := &StatisticalDetector{}
		baselineCache := NewBaselineCache(g.graphClient, g.logger)
		g.anomalyService = NewAnomalyService(g.queryService, detector, baselineCache, g.logger)
		g.logger.Info("Anomaly detection service created for MCP tools")

		// Create alert analysis service (shares graph client)
		g.analysisService = NewAlertAnalysisService(
			g.graphClient,
			g.name,
			g.logger,
		)
		g.logger.Info("Alert analysis service created for integration %s", g.name)

		// Create and start baseline collector for anomaly detection
		g.baselineCollector = NewBaselineCollector(
			g.client,
			g.queryService,
			g.graphClient,
			g.name,
			g.logger,
		)
		if err := g.baselineCollector.Start(g.ctx); err != nil {
			g.logger.Warn("Failed to start baseline collector: %v (continuing without baseline collection)", err)
			// Non-fatal - anomaly detection still works with existing baselines
		} else {
			g.logger.Info("Baseline collector started for integration %s", g.name)
		}

		// Create and start metrics syncer for curated metric ingestion
		if g.config.IsMetricsSyncEnabled() {
			syncConfig := MetricsSyncerConfig{
				SyncInterval:      g.config.GetMetricsSyncInterval(),
				RateLimitInterval: 100 * time.Millisecond, // 10 req/sec
				DatasourceUID:     g.config.MetricsDatasourceUID,
			}
			g.metricsSyncer = NewMetricsSyncerWithConfig(
				g.client,
				g.graphClient,
				g.name,
				g.logger,
				syncConfig,
			)
			if err := g.metricsSyncer.Start(g.ctx); err != nil {
				g.logger.Warn("Failed to start metrics syncer: %v (continuing without curated metric sync)", err)
				// Non-fatal - dashboard-based signals still work
			} else {
				g.logger.Info("Metrics syncer started for integration %s (interval: %s)", g.name, syncConfig.SyncInterval)
			}
		} else {
			g.logger.Info("Metrics sync disabled for integration %s", g.name)
		}

		// Initialize Observatory services (Phase 26)
		g.anomalyAggregator = NewAnomalyAggregator(g.graphClient, g.name, g.logger)
		g.logger.Info("Anomaly aggregator created for integration %s", g.name)

		g.evidenceService = NewObservatoryEvidenceService(
			g.graphClient,
			g.queryService,
			g.name,
			g.logger,
		)
		g.logger.Info("Observatory evidence service created for integration %s", g.name)

		// Initialize Observatory multi-provider registry (Phase 26.5)
		// Create provider that implements observatory.Provider interface
		g.observatoryProvider = NewGrafanaObservatoryProvider(
			g.graphClient,
			g.name,
			g.logger,
		)
		// Set the Grafana client for live metric queries (enables GetCurrentValue)
		if g.client != nil {
			g.observatoryProvider.SetGrafanaClient(g.client)
		}
		g.logger.Info("Observatory provider created for integration %s", g.name)

		// Create registry and register this integration's provider
		g.observatoryRegistry = observatory.NewRegistry()
		if err := g.observatoryRegistry.Register(g.observatoryProvider); err != nil {
			g.logger.Warn("Failed to register observatory provider: %v", err)
		} else {
			g.logger.Info("Observatory registry initialized with provider %s", g.name)
		}

		// Initialize Prometheus client and scrape target linker if URL configured
		if g.config.PrometheusURL != "" {
			g.logger.Info("Initializing Prometheus client (url: %s)", g.config.PrometheusURL)

			// Create SecretWatcher for Prometheus if config uses secret ref
			if g.config.UsesPrometheusSecretRef() {
				g.logger.Info("Creating SecretWatcher for Prometheus secret: %s, key: %s",
					g.config.PrometheusAPITokenRef.SecretName, g.config.PrometheusAPITokenRef.Key)

				// Reuse the Kubernetes client from the main secret watcher setup
				k8sConfig, err := rest.InClusterConfig()
				if err != nil {
					g.logger.Warn("Failed to get in-cluster config for Prometheus secret watcher: %v", err)
				} else {
					clientset, err := kubernetes.NewForConfig(k8sConfig)
					if err != nil {
						g.logger.Warn("Failed to create Kubernetes clientset for Prometheus: %v", err)
					} else {
						namespace, err := getCurrentNamespace()
						if err != nil {
							g.logger.Warn("Failed to determine namespace for Prometheus secret: %v", err)
						} else {
							prometheusSecretWatcher, err := NewSecretWatcher(
								clientset,
								namespace,
								g.config.PrometheusAPITokenRef.SecretName,
								g.config.PrometheusAPITokenRef.Key,
								g.logger,
							)
							if err != nil {
								g.logger.Warn("Failed to create Prometheus secret watcher: %v", err)
							} else {
								if err := prometheusSecretWatcher.Start(g.ctx); err != nil {
									g.logger.Warn("Failed to start Prometheus secret watcher: %v", err)
								} else {
									g.prometheusSecretWatcher = prometheusSecretWatcher
									g.logger.Info("Prometheus SecretWatcher started successfully")
								}
							}
						}
					}
				}
			}

			// Create Prometheus client
			g.prometheusClient = NewPrometheusClient(
				g.config.PrometheusURL,
				g.config.PrometheusAPITokenRef,
				g.prometheusSecretWatcher,
				g.logger,
			)
			g.logger.Info("Prometheus client created for integration %s", g.name)

			// Create and start scrape target linker if enabled
			if g.config.IsScrapeTargetLinkingEnabled() {
				linkerConfig := ScrapeTargetLinkerConfig{
					SyncInterval:      g.config.GetScrapeTargetLinkingInterval(),
					RateLimitInterval: 100 * time.Millisecond,
					StaleTTL:          7 * 24 * time.Hour,
				}
				g.scrapeTargetLinker = NewScrapeTargetLinker(
					g.prometheusClient, g.graphClient, g.name, g.logger, linkerConfig,
				)
				if err := g.scrapeTargetLinker.Start(g.ctx); err != nil {
					g.logger.Warn("Failed to start scrape target linker: %v (continuing without workload linking)", err)
				} else {
					g.logger.Info("Scrape target linker started for integration %s (interval: %s)", g.name, linkerConfig.SyncInterval)

					// Register callback with metrics syncer for event-driven linking
					if g.metricsSyncer != nil {
						g.metricsSyncer.RegisterCallback(g.scrapeTargetLinker)
						g.logger.Info("Registered scrape target linker callback with metrics syncer")
					}
				}
			} else {
				g.logger.Info("Scrape target linking disabled for integration %s", g.name)
			}

			// Create and start signal validation job if enabled
			if g.config.IsSignalValidationEnabled() {
				svConfig := g.config.GetSignalValidationConfig()
				g.signalValidationJob = NewSignalValidationJob(
					g.client,
					g.graphClient,
					g.name,
					g.config.MetricsDatasourceUID,
					svConfig,
					g.logger,
				)
				if err := g.signalValidationJob.Start(g.ctx); err != nil {
					g.logger.Warn("Failed to start signal validation job: %v (continuing without signal validation)", err)
				} else {
					g.logger.Info("Signal validation job started for integration %s (interval: %s)", g.name, svConfig.GetRunInterval())
				}
			} else {
				g.logger.Info("Signal validation disabled for integration %s", g.name)
			}
		}
	} else {
		g.logger.Info("Graph client not available - dashboard sync and MCP tools disabled")
	}

	g.logger.Info("Grafana integration started successfully (health: %s)", g.getHealthStatus().String())
	return nil
}

// Stop gracefully shuts down the integration.
func (g *GrafanaIntegration) Stop(ctx context.Context) error {
	g.logger.Info("Stopping Grafana integration: %s", g.name)

	// Cancel context
	if g.cancel != nil {
		g.cancel()
	}

	// Stop signal validation job first (depends on Prometheus/Grafana)
	if g.signalValidationJob != nil {
		g.logger.Info("Stopping signal validation job for integration %s", g.name)
		g.signalValidationJob.Stop()
	}

	// Stop scrape target linker (depends on Prometheus client)
	if g.scrapeTargetLinker != nil {
		g.logger.Info("Stopping scrape target linker for integration %s", g.name)
		g.scrapeTargetLinker.Stop()
	}

	// Stop Prometheus secret watcher if it exists
	if g.prometheusSecretWatcher != nil {
		if err := g.prometheusSecretWatcher.Stop(); err != nil {
			g.logger.Error("Error stopping Prometheus secret watcher: %v", err)
		}
	}

	// Stop metrics syncer (no dependencies on other services)
	if g.metricsSyncer != nil {
		g.logger.Info("Stopping metrics syncer for integration %s", g.name)
		g.metricsSyncer.Stop()
	}

	// Stop baseline collector (depends on query service and graph client)
	if g.baselineCollector != nil {
		g.logger.Info("Stopping baseline collector for integration %s", g.name)
		g.baselineCollector.Stop()
	}

	// Stop alert state syncer if it exists
	if g.stateSyncer != nil {
		g.logger.Info("Stopping alert state syncer for integration %s", g.name)
		g.stateSyncer.Stop()
	}

	// Clear alert analysis service (no Stop method needed - stateless)
	if g.analysisService != nil {
		g.logger.Info("Clearing alert analysis service for integration %s", g.name)
		g.analysisService = nil
	}

	// Stop alert syncer if it exists
	if g.alertSyncer != nil {
		g.logger.Info("Stopping alert syncer for integration %s", g.name)
		g.alertSyncer.Stop()
	}

	// Stop dashboard syncer if it exists
	if g.syncer != nil {
		g.syncer.Stop()
	}

	// Stop secret watcher if it exists
	if g.secretWatcher != nil {
		if err := g.secretWatcher.Stop(); err != nil {
			g.logger.Error("Error stopping secret watcher: %v", err)
		}
	}

	// Clear references
	g.client = nil
	g.secretWatcher = nil
	g.syncer = nil
	g.alertSyncer = nil
	g.stateSyncer = nil
	g.metricsSyncer = nil
	g.baselineCollector = nil
	g.queryService = nil

	// Clear observatory services (no Stop method needed - stateless)
	g.evidenceService = nil
	g.anomalyAggregator = nil

	// Clear observatory multi-provider support
	if g.observatoryRegistry != nil && g.observatoryProvider != nil {
		g.observatoryRegistry.Unregister(g.observatoryProvider.Name())
	}
	g.observatoryRegistry = nil
	g.observatoryProvider = nil

	// Clear scrape target linking and signal validation
	g.prometheusClient = nil
	g.prometheusSecretWatcher = nil
	g.scrapeTargetLinker = nil
	g.signalValidationJob = nil

	// Update health status
	g.setHealthStatus(integration.Stopped)

	g.logger.Info("Grafana integration stopped")
	return nil
}

// Health returns the current cached health status.
// This method is called frequently (e.g., SSE polling every 2s) so it returns
// cached status rather than testing connectivity. Actual connectivity tests
// happen during Start() and periodic health checks by the integration manager.
func (g *GrafanaIntegration) Health(ctx context.Context) integration.HealthStatus {
	// If client is nil, integration hasn't been started or has been stopped
	if g.client == nil {
		return integration.Stopped
	}

	// If using secret ref, check if token is available
	if g.secretWatcher != nil && !g.secretWatcher.IsHealthy() {
		g.setHealthStatus(integration.Degraded)
		return integration.Degraded
	}

	// Return cached health status - connectivity is tested by manager's periodic health checks
	return g.getHealthStatus()
}

// CheckConnectivity implements integration.ConnectivityChecker.
// Called by the manager during periodic health checks (every 30s) to verify actual connectivity.
func (g *GrafanaIntegration) CheckConnectivity(ctx context.Context) error {
	if g.client == nil {
		g.setHealthStatus(integration.Stopped)
		return fmt.Errorf("client not initialized")
	}

	if err := g.testConnection(ctx); err != nil {
		g.setHealthStatus(integration.Degraded)
		return err
	}

	g.setHealthStatus(integration.Healthy)
	return nil
}

// RegisterTools registers MCP tools with the server for this integration instance.
func (g *GrafanaIntegration) RegisterTools(registry integration.ToolRegistry) error {
	g.logger.Info("Registering Grafana MCP tools for instance: %s", g.name)

	// Check if query service is initialized (requires graph client)
	if g.queryService == nil {
		g.logger.Warn("Query service not initialized, skipping tool registration")
		return nil
	}

	// Register Overview tool: grafana_{name}_metrics_overview
	overviewTool := NewOverviewTool(g.queryService, g.anomalyService, g.graphClient, g.logger)
	overviewName := fmt.Sprintf("grafana_%s_metrics_overview", g.name)
	overviewSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"from": map[string]interface{}{
				"type":        "string",
				"description": "Start time (ISO8601: 2026-01-23T10:00:00Z)",
			},
			"to": map[string]interface{}{
				"type":        "string",
				"description": "End time (ISO8601: 2026-01-23T11:00:00Z)",
			},
			"cluster": map[string]interface{}{
				"type":        "string",
				"description": "Cluster name (required for scoping)",
			},
			"region": map[string]interface{}{
				"type":        "string",
				"description": "Region name (required for scoping)",
			},
		},
		"required": []string{"from", "to", "cluster", "region"},
	}
	if err := registry.RegisterTool(overviewName, "Get overview of key metrics from overview-level dashboards (first 5 panels per dashboard). Use this for high-level anomaly detection across all services.", overviewTool.Execute, overviewSchema); err != nil {
		return fmt.Errorf("failed to register overview tool: %w", err)
	}
	g.logger.Info("Registered tool: %s", overviewName)

	// Register Aggregated tool: grafana_{name}_metrics_aggregated
	aggregatedTool := NewAggregatedTool(g.queryService, g.graphClient, g.logger)
	aggregatedName := fmt.Sprintf("grafana_%s_metrics_aggregated", g.name)
	aggregatedSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"from": map[string]interface{}{
				"type":        "string",
				"description": "Start time (ISO8601: 2026-01-23T10:00:00Z)",
			},
			"to": map[string]interface{}{
				"type":        "string",
				"description": "End time (ISO8601: 2026-01-23T11:00:00Z)",
			},
			"cluster": map[string]interface{}{
				"type":        "string",
				"description": "Cluster name (required for scoping)",
			},
			"region": map[string]interface{}{
				"type":        "string",
				"description": "Region name (required for scoping)",
			},
			"service": map[string]interface{}{
				"type":        "string",
				"description": "Service name (optional, specify service OR namespace)",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Namespace name (optional, specify service OR namespace)",
			},
		},
		"required": []string{"from", "to", "cluster", "region"},
	}
	if err := registry.RegisterTool(aggregatedName, "Get aggregated metrics for a specific service or namespace from drill-down dashboards. Use this to focus on a particular service or namespace after detecting issues in overview.", aggregatedTool.Execute, aggregatedSchema); err != nil {
		return fmt.Errorf("failed to register aggregated tool: %w", err)
	}
	g.logger.Info("Registered tool: %s", aggregatedName)

	// Register Details tool: grafana_{name}_metrics_details
	detailsTool := NewDetailsTool(g.queryService, g.graphClient, g.logger)
	detailsName := fmt.Sprintf("grafana_%s_metrics_details", g.name)
	detailsSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"from": map[string]interface{}{
				"type":        "string",
				"description": "Start time (ISO8601: 2026-01-23T10:00:00Z)",
			},
			"to": map[string]interface{}{
				"type":        "string",
				"description": "End time (ISO8601: 2026-01-23T11:00:00Z)",
			},
			"cluster": map[string]interface{}{
				"type":        "string",
				"description": "Cluster name (required for scoping)",
			},
			"region": map[string]interface{}{
				"type":        "string",
				"description": "Region name (required for scoping)",
			},
		},
		"required": []string{"from", "to", "cluster", "region"},
	}
	if err := registry.RegisterTool(detailsName, "Get detailed metrics from detail-level dashboards (all panels). Use this for deep investigation of specific issues after narrowing scope with aggregated tool.", detailsTool.Execute, detailsSchema); err != nil {
		return fmt.Errorf("failed to register details tool: %w", err)
	}
	g.logger.Info("Registered tool: %s", detailsName)

	// Register Alerts Overview tool: grafana_{name}_alerts_overview
	alertsOverviewTool := NewAlertsOverviewTool(g.graphClient, g.name, g.analysisService, g.logger)
	alertsOverviewName := fmt.Sprintf("grafana_%s_alerts_overview", g.name)
	alertsOverviewSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"severity": map[string]interface{}{
				"type":        "string",
				"description": "Filter by severity level (optional: critical, warning, info)",
			},
			"cluster": map[string]interface{}{
				"type":        "string",
				"description": "Filter by cluster name (optional)",
			},
			"service": map[string]interface{}{
				"type":        "string",
				"description": "Filter by service name (optional)",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Filter by namespace (optional)",
			},
		},
		"required": []string{},
	}
	if err := registry.RegisterTool(alertsOverviewName, "Get overview of firing and pending alerts grouped by severity. Returns alert counts, flapping indicators, and minimal context (name + firing duration) for triage. All filters are optional.", alertsOverviewTool.Execute, alertsOverviewSchema); err != nil {
		return fmt.Errorf("failed to register alerts overview tool: %w", err)
	}
	g.logger.Info("Registered tool: %s", alertsOverviewName)

	// Register Alerts Aggregated tool: grafana_{name}_alerts_aggregated
	alertsAggregatedTool := NewAlertsAggregatedTool(g.graphClient, g.name, g.analysisService, g.logger)
	alertsAggregatedName := fmt.Sprintf("grafana_%s_alerts_aggregated", g.name)
	alertsAggregatedSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"lookback": map[string]interface{}{
				"type":        "string",
				"description": "Lookback duration (default: 1h, examples: 30m, 2h, 24h)",
			},
			"severity": map[string]interface{}{
				"type":        "string",
				"description": "Filter by severity level (optional: critical, warning, info)",
			},
			"cluster": map[string]interface{}{
				"type":        "string",
				"description": "Filter by cluster name (optional)",
			},
			"service": map[string]interface{}{
				"type":        "string",
				"description": "Filter by service name (optional)",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Filter by namespace (optional)",
			},
		},
		"required": []string{},
	}
	if err := registry.RegisterTool(alertsAggregatedName, "Get specific alerts with compact state timeline ([F F N N] format) and analysis categories. Shows 1h state progression in 10-minute buckets using LOCF interpolation. Use after identifying issues in overview to investigate specific alerts without loading full history.", alertsAggregatedTool.Execute, alertsAggregatedSchema); err != nil {
		return fmt.Errorf("failed to register alerts aggregated tool: %w", err)
	}
	g.logger.Info("Registered tool: %s", alertsAggregatedName)

	// Register Alerts Details tool: grafana_{name}_alerts_details
	alertsDetailsTool := NewAlertsDetailsTool(g.graphClient, g.name, g.analysisService, g.logger)
	alertsDetailsName := fmt.Sprintf("grafana_%s_alerts_details", g.name)
	alertsDetailsSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"alert_uid": map[string]interface{}{
				"type":        "string",
				"description": "Specific alert UID to fetch (optional, provide UID or filters)",
			},
			"severity": map[string]interface{}{
				"type":        "string",
				"description": "Filter by severity level (optional: critical, warning, info)",
			},
			"cluster": map[string]interface{}{
				"type":        "string",
				"description": "Filter by cluster name (optional)",
			},
			"service": map[string]interface{}{
				"type":        "string",
				"description": "Filter by service name (optional)",
			},
			"namespace": map[string]interface{}{
				"type":        "string",
				"description": "Filter by namespace (optional)",
			},
		},
		"required": []string{},
	}
	if err := registry.RegisterTool(alertsDetailsName, "Get full state timeline (7 days) with timestamps, alert rule definition, and complete metadata (labels, annotations). Use for deep debugging of specific issues after narrowing scope with aggregated tool. WARNING: can produce large responses for multiple alerts.", alertsDetailsTool.Execute, alertsDetailsSchema); err != nil {
		return fmt.Errorf("failed to register alerts details tool: %w", err)
	}
	g.logger.Info("Registered tool: %s", alertsDetailsName)

	g.logger.Info("Successfully registered 6 Grafana MCP tools")

	// Register Observatory tools (Phase 26)
	// These tools enable AI-driven incident investigation with progressive disclosure
	if g.observatoryRegistry != nil && g.evidenceService != nil {
		if err := g.registerObservatoryTools(registry); err != nil {
			return fmt.Errorf("failed to register observatory tools: %w", err)
		}
		g.logger.Info("Successfully registered 8 Observatory MCP tools")
	} else {
		g.logger.Warn("Observatory registry not initialized, skipping observatory tool registration")
	}

	return nil
}

// registerObservatoryTools registers the 8 observatory MCP tools for AI-driven investigation.
// Tools follow progressive disclosure pattern: Orient -> Narrow -> Investigate -> Hypothesize -> Verify
//
// Tools use the registry-based Observatory services via adapters, enabling multi-provider support.
func (g *GrafanaIntegration) registerObservatoryTools(registry integration.ToolRegistry) error {
	// Create registry-based services via adapters
	if g.observatoryRegistry == nil {
		return fmt.Errorf("observatory registry not initialized")
	}

	obsService := g.NewObservatoryServiceFromRegistry()
	invService := g.NewObservatoryInvestigateServiceFromRegistry()
	if obsService == nil || invService == nil {
		return fmt.Errorf("failed to create observatory services from registry")
	}

	observatorySvc := NewObservatoryServiceAdapter(obsService)
	investigateSvc := NewObservatoryInvestigateServiceAdapter(invService)

	// Create tool instances with registry-based services
	statusTool := NewObservatoryStatusTool(observatorySvc, g.logger)
	changesTool := NewObservatoryChangesTool(g.graphClient, g.name, g.logger)
	scopeTool := NewObservatoryScopeTool(observatorySvc, g.logger)
	signalsTool := NewObservatorySignalsTool(investigateSvc, g.logger)
	signalDetailTool := NewObservatorySignalDetailTool(investigateSvc, g.logger)
	compareTool := NewObservatoryCompareTool(investigateSvc, g.logger)
	explainTool := NewObservatoryExplainTool(g.evidenceService, g.logger)
	evidenceTool := NewObservatoryEvidenceTool(g.evidenceService, g.logger)

	// ============================================================================
	// Orient Stage Tools - Cluster-wide situation awareness
	// ============================================================================

	// observatory_status: Top 5 anomaly hotspots
	if err := registry.RegisterTool(
		"observatory_status",
		"Get cluster-wide anomaly summary with top 5 hotspots by namespace/workload. Returns numeric scores (0.0-1.0) and empty array when nothing is anomalous.",
		statusTool.Execute,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cluster":   map[string]interface{}{"type": "string", "description": "Optional: filter to specific cluster"},
				"namespace": map[string]interface{}{"type": "string", "description": "Optional: filter to specific namespace"},
			},
		},
	); err != nil {
		return fmt.Errorf("failed to register observatory_status: %w", err)
	}
	g.logger.Info("Registered tool: observatory_status")

	// observatory_changes: Recent K8s deployment and config changes
	if err := registry.RegisterTool(
		"observatory_changes",
		"Get recent K8s changes (deployments, config updates, Flux reconciliations) that could explain anomalies. Returns max 20 changes.",
		changesTool.Execute,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"namespace": map[string]interface{}{"type": "string", "description": "Optional: filter to specific namespace"},
				"lookback":  map[string]interface{}{"type": "string", "description": "Lookback duration (default: 1h, max: 24h). Format: 30m, 1h, 2h, etc."},
			},
		},
	); err != nil {
		return fmt.Errorf("failed to register observatory_changes: %w", err)
	}
	g.logger.Info("Registered tool: observatory_changes")

	// ============================================================================
	// Narrow Stage Tools - Workload scoping
	// ============================================================================

	// observatory_scope: Namespace or workload anomaly scoping
	if err := registry.RegisterTool(
		"observatory_scope",
		"Get anomalies for a namespace or specific workload, ranked by severity. Returns flat list sorted by anomaly score.",
		scopeTool.Execute,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"namespace": map[string]interface{}{"type": "string", "description": "Kubernetes namespace (required)"},
				"workload":  map[string]interface{}{"type": "string", "description": "Optional: narrow to specific workload within namespace"},
			},
			"required": []string{"namespace"},
		},
	); err != nil {
		return fmt.Errorf("failed to register observatory_scope: %w", err)
	}
	g.logger.Info("Registered tool: observatory_scope")

	// observatory_signals: Workload signal enumeration
	if err := registry.RegisterTool(
		"observatory_signals",
		"Get all signal anchors for a workload with current anomaly state. Returns metric name, role, score, confidence, and quality.",
		signalsTool.Execute,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"namespace": map[string]interface{}{"type": "string", "description": "Kubernetes namespace (required)"},
				"workload":  map[string]interface{}{"type": "string", "description": "Workload name (required)"},
			},
			"required": []string{"namespace", "workload"},
		},
	); err != nil {
		return fmt.Errorf("failed to register observatory_signals: %w", err)
	}
	g.logger.Info("Registered tool: observatory_signals")

	// ============================================================================
	// Investigate Stage Tools - Deep signal inspection
	// ============================================================================

	// observatory_signal_detail: Baseline stats and source dashboard
	if err := registry.RegisterTool(
		"observatory_signal_detail",
		"Get detailed signal info: baseline stats (mean, std_dev, percentiles), current value, anomaly score, confidence, and source dashboard.",
		signalDetailTool.Execute,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"namespace":   map[string]interface{}{"type": "string", "description": "Kubernetes namespace (required)"},
				"workload":    map[string]interface{}{"type": "string", "description": "Workload name (required)"},
				"metric_name": map[string]interface{}{"type": "string", "description": "Metric name (required)"},
			},
			"required": []string{"namespace", "workload", "metric_name"},
		},
	); err != nil {
		return fmt.Errorf("failed to register observatory_signal_detail: %w", err)
	}
	g.logger.Info("Registered tool: observatory_signal_detail")

	// observatory_compare: Time-based signal comparison
	if err := registry.RegisterTool(
		"observatory_compare",
		"Compare signal value and anomaly score between current and past time. ScoreDelta positive means worsening.",
		compareTool.Execute,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"namespace":   map[string]interface{}{"type": "string", "description": "Kubernetes namespace (required)"},
				"workload":    map[string]interface{}{"type": "string", "description": "Workload name (required)"},
				"metric_name": map[string]interface{}{"type": "string", "description": "Metric name (required)"},
				"lookback":    map[string]interface{}{"type": "string", "description": "Comparison lookback (default: 24h, max: 7d). Format: 1h, 12h, 24h, etc."},
			},
			"required": []string{"namespace", "workload", "metric_name"},
		},
	); err != nil {
		return fmt.Errorf("failed to register observatory_compare: %w", err)
	}
	g.logger.Info("Registered tool: observatory_compare")

	// ============================================================================
	// Hypothesize Stage Tools - Root cause analysis
	// ============================================================================

	// observatory_explain: K8s graph candidates
	if err := registry.RegisterTool(
		"observatory_explain",
		"Get candidate root causes: upstream K8s dependencies (2-hop traversal) and recent changes (last 1h) for an anomalous signal.",
		explainTool.Execute,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"namespace":   map[string]interface{}{"type": "string", "description": "Kubernetes namespace (required)"},
				"workload":    map[string]interface{}{"type": "string", "description": "Workload name (required)"},
				"metric_name": map[string]interface{}{"type": "string", "description": "Anomalous metric name (required)"},
			},
			"required": []string{"namespace", "workload", "metric_name"},
		},
	); err != nil {
		return fmt.Errorf("failed to register observatory_explain: %w", err)
	}
	g.logger.Info("Registered tool: observatory_explain")

	// ============================================================================
	// Verify Stage Tools - Evidence gathering
	// ============================================================================

	// observatory_evidence: Raw metric values, alerts, logs
	if err := registry.RegisterTool(
		"observatory_evidence",
		"Get raw evidence for hypothesis verification: metric values, alert states, and log excerpts (ERROR level, 5-min window).",
		evidenceTool.Execute,
		map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"namespace":   map[string]interface{}{"type": "string", "description": "Kubernetes namespace (required)"},
				"workload":    map[string]interface{}{"type": "string", "description": "Workload name (required)"},
				"metric_name": map[string]interface{}{"type": "string", "description": "Metric name (required)"},
				"lookback":    map[string]interface{}{"type": "string", "description": "Evidence lookback (default: 1h). Format: 30m, 1h, 2h, etc."},
			},
			"required": []string{"namespace", "workload", "metric_name"},
		},
	); err != nil {
		return fmt.Errorf("failed to register observatory_evidence: %w", err)
	}
	g.logger.Info("Registered tool: observatory_evidence")

	return nil
}

// testConnection tests connectivity to Grafana by executing minimal queries.
// Tests both dashboard access (required) and datasource access (optional, warns on failure).
func (g *GrafanaIntegration) testConnection(ctx context.Context) error {
	// Test 1: Dashboard read access (REQUIRED)
	dashboards, err := g.client.ListDashboards(ctx)
	if err != nil {
		return fmt.Errorf("dashboard access test failed: %w", err)
	}
	g.logger.Debug("Dashboard access test passed: found %d dashboards", len(dashboards))

	// Test 2: Datasource access (OPTIONAL - warn on failure, don't block)
	datasources, err := g.client.ListDatasources(ctx)
	if err != nil {
		g.logger.Warn("Datasource access test failed (non-blocking): %v", err)
		// Continue - datasource access is not critical for initial connectivity
	} else {
		g.logger.Debug("Datasource access test passed: found %d datasources", len(datasources))
	}

	return nil
}

// setHealthStatus updates the health status in a thread-safe manner.
func (g *GrafanaIntegration) setHealthStatus(status integration.HealthStatus) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.healthStatus = status
}

// getHealthStatus retrieves the health status in a thread-safe manner.
func (g *GrafanaIntegration) getHealthStatus() integration.HealthStatus {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.healthStatus
}

// GetSyncStatus returns the current sync status if syncer is available
func (g *GrafanaIntegration) GetSyncStatus() *integration.SyncStatus {
	if g.syncer == nil {
		return nil
	}
	return g.syncer.GetSyncStatus()
}

// TriggerSync triggers a manual dashboard sync
func (g *GrafanaIntegration) TriggerSync(ctx context.Context) error {
	if g.syncer == nil {
		return fmt.Errorf("syncer not initialized")
	}
	return g.syncer.TriggerSync(ctx)
}

// Status returns the integration status including sync information
func (g *GrafanaIntegration) Status() integration.IntegrationStatus {
	status := integration.IntegrationStatus{
		Name:       g.name,
		Type:       "grafana",
		Enabled:    true, // Runtime instances are always enabled
		Health:     g.getHealthStatus().String(),
		SyncStatus: g.GetSyncStatus(),
	}
	return status
}

// GetAnalysisService returns the alert analysis service for this integration
// Returns nil if service not initialized (graph disabled or startup failed)
func (g *GrafanaIntegration) GetAnalysisService() *AlertAnalysisService {
	return g.analysisService
}

// GetObservatoryRegistry returns the Observatory multi-provider registry.
// Returns nil if not initialized (graph disabled or startup failed).
// This can be used to register additional providers or access cross-provider services.
func (g *GrafanaIntegration) GetObservatoryRegistry() *observatory.Registry {
	return g.observatoryRegistry
}

// GetObservatoryProvider returns this integration's Observatory provider.
// Returns nil if not initialized (graph disabled or startup failed).
func (g *GrafanaIntegration) GetObservatoryProvider() *GrafanaObservatoryProvider {
	return g.observatoryProvider
}

// NewObservatoryServiceFromRegistry creates an observatory.Service using the registry.
// This allows using the new multi-provider Observatory service instead of
// the legacy Grafana-specific services. Returns nil if registry not initialized.
func (g *GrafanaIntegration) NewObservatoryServiceFromRegistry() *observatory.Service {
	if g.observatoryRegistry == nil {
		return nil
	}
	return observatory.NewService(g.observatoryRegistry)
}

// NewObservatoryInvestigateServiceFromRegistry creates an observatory.InvestigateService.
// This allows using the new multi-provider investigation service.
// Returns nil if registry not initialized.
func (g *GrafanaIntegration) NewObservatoryInvestigateServiceFromRegistry() *observatory.InvestigateService {
	if g.observatoryRegistry == nil {
		return nil
	}
	return observatory.NewInvestigateService(g.observatoryRegistry)
}

// SignalValidationJob returns the signal validation job for API access.
// Returns nil if not initialized (PrometheusURL not configured or startup failed).
func (g *GrafanaIntegration) SignalValidationJob() *SignalValidationJob {
	return g.signalValidationJob
}

// getCurrentNamespace reads the namespace from the ServiceAccount mount.
// This file is automatically mounted by Kubernetes in all pods at a well-known path.
func getCurrentNamespace() (string, error) {
	const namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
	data, err := os.ReadFile(namespaceFile)
	if err != nil {
		return "", fmt.Errorf("failed to read namespace file: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}
