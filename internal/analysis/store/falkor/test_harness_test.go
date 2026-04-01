package falkor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/moolen/spectre/internal/graph"
	graphsync "github.com/moolen/spectre/internal/graph/sync"
	"github.com/moolen/spectre/internal/models"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type testHarness struct {
	client    graph.Client
	pipeline  graphsync.Pipeline
	container testcontainers.Container
	graphName string
}

func newTestHarness(t *testing.T) (*testHarness, error) {
	ctx := context.Background()
	graphName := fmt.Sprintf("test-%s", uuid.New().String()[:8])

	req := testcontainers.ContainerRequest{
		Image:        "falkordb/falkordb:v4.2.0",
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

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get container host: %w", err)
	}

	port, err := container.MappedPort(ctx, "6379")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get container port: %w", err)
	}

	config := graph.DefaultClientConfig()
	config.Host = host
	config.Port = port.Int()
	config.GraphName = graphName
	config.DialTimeout = 10 * time.Second

	client := graph.NewClient(config)
	if err := client.Connect(ctx); err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to connect to FalkorDB: %w", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if err := client.Ping(ctx); err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err := client.Ping(ctx); err != nil {
		_ = client.Close()
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("FalkorDB not ready after ping attempts: %w", err)
	}

	if err := client.InitializeSchema(ctx); err != nil {
		_ = client.Close()
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	pipeline := graphsync.NewPipeline(graphsync.DefaultPipelineConfig(), client)
	if err := pipeline.Start(ctx); err != nil {
		_ = client.Close()
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to start pipeline: %w", err)
	}

	h := &testHarness{
		client:    client,
		pipeline:  pipeline,
		container: container,
		graphName: graphName,
	}

	t.Cleanup(func() {
		_ = h.Cleanup(context.Background())
	})

	return h, nil
}

func (h *testHarness) SeedEventsFromAuditLog(ctx context.Context, auditLogPath string) error {
	events, err := loadAuditLog(auditLogPath)
	if err != nil {
		return fmt.Errorf("failed to load audit log: %w", err)
	}
	if len(events) == 0 {
		return nil
	}

	const batchSize = 1000
	for i := 0; i < len(events); i += batchSize {
		end := i + batchSize
		if end > len(events) {
			end = len(events)
		}
		if err := h.pipeline.ProcessBatch(ctx, events[i:end]); err != nil {
			return fmt.Errorf("failed to process event batch [%d:%d]: %w", i, end, err)
		}
	}

	return nil
}

func (h *testHarness) GetClient() graph.Client {
	return h.client
}

func (h *testHarness) Cleanup(ctx context.Context) error {
	var errs []error

	if h.pipeline != nil {
		if err := h.pipeline.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop pipeline: %w", err))
		}
	}
	if h.client != nil {
		if err := h.client.DeleteGraph(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete graph: %w", err))
		}
		if err := h.client.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close client: %w", err))
		}
	}
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

func loadAuditLog(filePath string) ([]models.Event, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}
	defer file.Close()

	var events []models.Event
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 10*1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event models.Event
		if err := json.Unmarshal(line, &event); err != nil {
			fmt.Printf("Warning: Skipping malformed JSON on line %d: %v\n", lineNum, err)
			continue
		}
		if err := event.Validate(); err != nil {
			fmt.Printf("Warning: Skipping invalid event on line %d: %v\n", lineNum, err)
			continue
		}
		events = append(events, event)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading audit log file: %w", err)
	}

	return events, nil
}
