package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/observatory"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Shared container for all Observatory tests (initialized once)
var (
	sharedContainer     testcontainers.Container
	sharedContainerOnce sync.Once
	sharedContainerErr  error
	sharedHost          string
	sharedPort          int
)

// ObservatoryTestHarness manages test infrastructure for Observatory integration tests.
// Uses a shared FalkorDB container for performance, with unique graph names per test.
//
// The harness uses the registry-based Observatory services via adapters, matching
// the production code path for accurate integration testing.
type ObservatoryTestHarness struct {
	t               *testing.T
	ctx             context.Context
	graphClient     graph.Client
	graphName       string
	integrationName string
	logger          *logging.Logger

	// Registry-based services (Phase 26.5)
	registry           *observatory.Registry
	testProvider       *observatory.TestProvider
	observatorySvc     ObservatoryServiceInterface           // Adapter wrapping observatory.Service
	investigateSvc     ObservatoryInvestigateServiceInterface // Adapter wrapping observatory.InvestigateService
	evidenceService    *ObservatoryEvidenceService           // Grafana-specific for graph operations
	anomalyAggregator  *AnomalyAggregator                    // For cache operations

	currentValues map[string]float64 // metric|ns|workload -> value (synced to testProvider)
	alertStates   map[string]string  // metric|ns|workload -> state (synced to testProvider)
}

// NewObservatoryTestHarness creates a new harness with a unique graph for this test.
// Uses a shared FalkorDB container (started once) for performance.
//
// The harness uses the registry-based Observatory services via adapters,
// matching the production code path for accurate integration testing.
func NewObservatoryTestHarness(t *testing.T) (*ObservatoryTestHarness, error) {
	ctx := context.Background()

	// Start shared container once
	sharedContainerOnce.Do(func() {
		sharedContainer, sharedHost, sharedPort, sharedContainerErr = startSharedContainer(ctx)
	})

	if sharedContainerErr != nil {
		return nil, fmt.Errorf("failed to start shared container: %w", sharedContainerErr)
	}

	// Create unique graph name for this test
	graphName := fmt.Sprintf("obs-test-%s", uuid.New().String()[:8])
	integrationName := "test-grafana"

	// Create graph client config
	config := graph.DefaultClientConfig()
	config.Host = sharedHost
	config.Port = sharedPort
	config.GraphName = graphName
	config.DialTimeout = 10 * time.Second

	// Create and connect client
	client := graph.NewClient(config)
	if err := client.Connect(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to FalkorDB: %w", err)
	}

	// Initialize schema
	if err := client.InitializeSchema(ctx); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	logger := logging.GetLogger("test.observatory")

	// Create anomaly aggregator (needed for cache operations)
	anomalyAgg := NewAnomalyAggregator(client, integrationName, logger)

	harness := &ObservatoryTestHarness{
		t:                 t,
		ctx:               ctx,
		graphClient:       client,
		graphName:         graphName,
		integrationName:   integrationName,
		logger:            logger,
		anomalyAggregator: anomalyAgg,
		currentValues:     make(map[string]float64),
		alertStates:       make(map[string]string),
	}

	// Create registry-based Observatory services (Phase 26.5)
	// This mirrors the production code path in grafana.go
	harness.testProvider = observatory.NewTestProvider(integrationName)
	harness.registry = observatory.NewRegistry()
	if err := harness.registry.Register(harness.testProvider); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to register test provider: %w", err)
	}

	// Create services from registry and wrap with adapters
	obsService := observatory.NewService(harness.registry)
	invService := observatory.NewInvestigateService(harness.registry)
	harness.observatorySvc = NewObservatoryServiceAdapter(obsService)
	harness.investigateSvc = NewObservatoryInvestigateServiceAdapter(invService)

	// Create evidence service (Grafana-specific for graph operations)
	harness.evidenceService = NewObservatoryEvidenceService(client, nil, integrationName, logger)

	// Cleanup on test completion
	t.Cleanup(func() {
		harness.Cleanup()
	})

	return harness, nil
}

// startSharedContainer starts the FalkorDB container (called once via sync.Once)
func startSharedContainer(ctx context.Context) (testcontainers.Container, string, int, error) {
	req := testcontainers.ContainerRequest{
		Image:        "falkordb/falkordb:latest",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(60 * time.Second),
		AutoRemove:   true,
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", 0, fmt.Errorf("failed to start FalkorDB container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		container.Terminate(ctx)
		return nil, "", 0, fmt.Errorf("failed to get container host: %w", err)
	}

	port, err := container.MappedPort(ctx, "6379")
	if err != nil {
		container.Terminate(ctx)
		return nil, "", 0, fmt.Errorf("failed to get container port: %w", err)
	}

	return container, host, port.Int(), nil
}

// SetCurrentValue sets a current value for a specific metric/namespace/workload.
// Syncs to both local map and the testProvider for registry-based services.
func (h *ObservatoryTestHarness) SetCurrentValue(metricName, namespace, workload string, value float64) {
	key := fmt.Sprintf("%s|%s|%s", metricName, namespace, workload)
	h.currentValues[key] = value
	// Sync to testProvider for registry-based services
	h.testProvider.SetCurrentValue(metricName, namespace, workload, value)
}

// SetAlertState sets an alert state for a specific metric/namespace/workload.
// Syncs to both local map and the testProvider for registry-based services.
func (h *ObservatoryTestHarness) SetAlertState(metricName, namespace, workload, state string) {
	key := fmt.Sprintf("%s|%s|%s", metricName, namespace, workload)
	h.alertStates[key] = state
	// Sync to testProvider for registry-based services
	h.testProvider.SetAlertState(metricName, namespace, workload, state)
}

// ClearTestState clears current values, alert states, and cache before a scenario.
func (h *ObservatoryTestHarness) ClearTestState() {
	h.currentValues = make(map[string]float64)
	h.alertStates = make(map[string]string)
	h.testProvider.ClearAll()
	h.anomalyAggregator.cache.Clear()
}

// GetGraphClient returns the graph client for direct operations
func (h *ObservatoryTestHarness) GetGraphClient() graph.Client {
	return h.graphClient
}

// GetAnomalyAggregator returns the anomaly aggregator
func (h *ObservatoryTestHarness) GetAnomalyAggregator() *AnomalyAggregator {
	return h.anomalyAggregator
}

// GetObservatoryService returns the observatory service interface
func (h *ObservatoryTestHarness) GetObservatoryService() ObservatoryServiceInterface {
	return h.observatorySvc
}

// GetInvestigateService returns the investigate service interface
func (h *ObservatoryTestHarness) GetInvestigateService() ObservatoryInvestigateServiceInterface {
	return h.investigateSvc
}

// GetEvidenceService returns the evidence service
func (h *ObservatoryTestHarness) GetEvidenceService() *ObservatoryEvidenceService {
	return h.evidenceService
}

// GetTestProvider returns the test provider for direct manipulation
func (h *ObservatoryTestHarness) GetTestProvider() *observatory.TestProvider {
	return h.testProvider
}

// GetRegistry returns the observatory registry
func (h *ObservatoryTestHarness) GetRegistry() *observatory.Registry {
	return h.registry
}

// ExecuteTool executes an Observatory MCP tool and returns the result.
// Tools use the registry-based services via adapters, matching the production code path.
func (h *ObservatoryTestHarness) ExecuteTool(ctx context.Context, toolName string, params any) (any, error) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}

	switch toolName {
	case "observatory_status":
		tool := NewObservatoryStatusTool(h.observatorySvc, h.logger)
		return tool.Execute(ctx, paramsJSON)

	case "observatory_scope":
		tool := NewObservatoryScopeTool(h.observatorySvc, h.logger)
		return tool.Execute(ctx, paramsJSON)

	case "observatory_signals":
		tool := NewObservatorySignalsTool(h.investigateSvc, h.logger)
		return tool.Execute(ctx, paramsJSON)

	case "observatory_signal_detail":
		tool := NewObservatorySignalDetailTool(h.investigateSvc, h.logger)
		return tool.Execute(ctx, paramsJSON)

	case "observatory_compare":
		tool := NewObservatoryCompareTool(h.investigateSvc, h.logger)
		return tool.Execute(ctx, paramsJSON)

	case "observatory_changes":
		tool := NewObservatoryChangesTool(h.graphClient, h.integrationName, h.logger)
		return tool.Execute(ctx, paramsJSON)

	case "observatory_explain":
		tool := NewObservatoryExplainTool(h.evidenceService, h.logger)
		return tool.Execute(ctx, paramsJSON)

	case "observatory_evidence":
		tool := NewObservatoryEvidenceTool(h.evidenceService, h.logger)
		return tool.Execute(ctx, paramsJSON)

	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

// Cleanup cleans up test resources (graph data, not container)
func (h *ObservatoryTestHarness) Cleanup() {
	if h.graphClient != nil {
		// Delete this test's graph data
		h.graphClient.DeleteGraph(h.ctx)
		h.graphClient.Close()
	}
}

// testQueryService implements QueryService interface for testing
type testQueryService struct {
	harness *ObservatoryTestHarness
}

// FetchCurrentValue returns injected current value or baseline mean fallback
func (s *testQueryService) FetchCurrentValue(ctx context.Context, metricName, namespace, workload string) (float64, error) {
	key := fmt.Sprintf("%s|%s|%s", metricName, namespace, workload)
	if val, ok := s.harness.currentValues[key]; ok {
		return val, nil
	}
	// Return 0 if not found - caller should handle this case
	return 0, fmt.Errorf("no current value configured for %s", key)
}

// FetchHistoricalValue returns historical value (uses current value for testing)
func (s *testQueryService) FetchHistoricalValue(ctx context.Context, metricName, namespace, workload string, lookback time.Duration) (float64, error) {
	// For tests, return current value minus some delta to simulate historical
	key := fmt.Sprintf("%s|%s|%s", metricName, namespace, workload)
	if val, ok := s.harness.currentValues[key]; ok {
		return val * 0.9, nil // Historical is 90% of current for testing
	}
	return 0, fmt.Errorf("no historical value configured for %s", key)
}
