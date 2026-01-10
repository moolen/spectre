package api

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graph/sync"
	"github.com/moolen/spectre/internal/models"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestHarness manages a test FalkorDB instance and provides utilities for testing API handlers
type TestHarness struct {
	client    graph.Client
	pipeline  sync.Pipeline
	container testcontainers.Container
	config    graph.ClientConfig
	ctx       context.Context
	t         *testing.T
	graphName string
}

// NewTestHarness creates a new test harness with a fresh FalkorDB container
func NewTestHarness(t *testing.T) (*TestHarness, error) {
	ctx := context.Background()

	// Create unique graph name for this test
	graphName := fmt.Sprintf("test-%s", uuid.New().String()[:8])

	// Start FalkorDB container
	req := testcontainers.ContainerRequest{
		Image:        "falkordb/falkordb:latest",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(30 * time.Second),
		AutoRemove:   true,
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start FalkorDB container: %w", err)
	}

	// Get container host and port
	host, err := container.Host(ctx)
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	port, err := container.MappedPort(ctx, "6379")
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get container port: %w", err)
	}

	// Create graph client config
	config := graph.DefaultClientConfig()
	config.Host = host
	config.Port = port.Int()
	config.GraphName = graphName
	config.DialTimeout = 10 * time.Second

	// Create and connect client
	client := graph.NewClient(config)
	if err := client.Connect(ctx); err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to connect to FalkorDB: %w", err)
	}

	// Wait for FalkorDB to be ready with ping
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if err := client.Ping(ctx); err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if err := client.Ping(ctx); err != nil {
		client.Close()
		container.Terminate(ctx)
		return nil, fmt.Errorf("FalkorDB not ready after ping attempts: %w", err)
	}

	// Initialize schema
	if err := client.InitializeSchema(ctx); err != nil {
		client.Close()
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Create pipeline
	pipelineConfig := sync.DefaultPipelineConfig()
	pipeline := sync.NewPipeline(pipelineConfig, client)

	// Start pipeline
	if err := pipeline.Start(ctx); err != nil {
		client.Close()
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to start pipeline: %w", err)
	}

	harness := &TestHarness{
		client:    client,
		pipeline:  pipeline,
		container: container,
		config:    config,
		ctx:       ctx,
		t:         t,
		graphName: graphName,
	}

	// Cleanup on test failure
	t.Cleanup(func() {
		harness.Cleanup(ctx)
	})

	return harness, nil
}

// SeedEventsFromAuditLog loads events from an audit log and processes them
func (h *TestHarness) SeedEventsFromAuditLog(ctx context.Context, auditLogPath string) error {
	events, err := LoadAuditLog(auditLogPath)
	if err != nil {
		return fmt.Errorf("failed to load audit log: %w", err)
	}

	if len(events) == 0 {
		return nil
	}

	// Process events in batches for efficiency
	batchSize := 1000
	for i := 0; i < len(events); i += batchSize {
		end := i + batchSize
		if end > len(events) {
			end = len(events)
		}

		batch := events[i:end]
		if err := h.pipeline.ProcessBatch(ctx, batch); err != nil {
			return fmt.Errorf("failed to process event batch [%d:%d]: %w", i, end, err)
		}
	}

	return nil
}

// GetClient returns the graph client for direct queries
func (h *TestHarness) GetClient() graph.Client {
	return h.client
}

// GetPipeline returns the sync pipeline
func (h *TestHarness) GetPipeline() sync.Pipeline {
	return h.pipeline
}

// Cleanup cleans up test resources
func (h *TestHarness) Cleanup(ctx context.Context) error {
	var errs []error

	// Stop pipeline
	if h.pipeline != nil {
		if err := h.pipeline.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop pipeline: %w", err))
		}
	}

	// Delete graph data
	if h.client != nil {
		if err := h.client.DeleteGraph(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete graph: %w", err))
		}
		if err := h.client.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close client: %w", err))
		}
	}

	// Terminate container
	if h.container != nil {
		if err := h.container.Terminate(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to terminate container: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	return nil
}

// extractTimestampAndPodUID extracts the timestamp from the last event and pod UID from events
func extractTimestampAndPodUID(events []models.Event) (int64, string) {
	var lastTimestamp int64
	var podUID string

	for _, event := range events {
		// Track the last timestamp
		if event.Timestamp > lastTimestamp {
			lastTimestamp = event.Timestamp
		}

		// Extract pod UID if this is a Pod resource
		if event.Resource.Kind == "Pod" {
			podUID = event.Resource.UID
		}
	}

	return lastTimestamp, podUID
}
