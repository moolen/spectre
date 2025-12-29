package graphservice

import (
	"context"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/graph/sync"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// Service manages the graph reasoning layer
type Service struct {
	config    ServiceConfig
	client    graph.Client
	schema    *graph.Schema
	pipeline  sync.Pipeline
	listener  sync.EventListener
	rebuilder *sync.Rebuilder
	logger    *logging.Logger

	// Status
	initialized bool
	running     bool
}

// ServiceConfig configures the graph service
type ServiceConfig struct {
	// Graph database connection
	GraphConfig graph.ClientConfig

	// Sync pipeline configuration
	PipelineConfig sync.PipelineConfig

	// Rebuild options
	RebuildOnStart      bool
	RebuildWindow       time.Duration
	RebuildIfEmptyOnly  bool

	// Integration
	AutoStartPipeline bool
}

// DefaultServiceConfig returns default service configuration
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		GraphConfig:         graph.DefaultClientConfig(),
		PipelineConfig:      sync.DefaultPipelineConfig(),
		RebuildOnStart:      true,
		RebuildWindow:       24 * time.Hour,
		RebuildIfEmptyOnly:  true,
		AutoStartPipeline:   true,
	}
}

// NewService creates a new graph service
func NewService(config ServiceConfig) *Service {
	client := graph.NewClient(config.GraphConfig)
	pipeline := sync.NewPipeline(config.PipelineConfig, client)
	listener := sync.NewEventListener(config.PipelineConfig)

	return &Service{
		config:   config,
		client:   client,
		schema:   graph.NewSchema(client),
		pipeline: pipeline,
		listener: listener,
		logger:   logging.GetLogger("graph.service"),
	}
}

// Initialize connects to the graph database and initializes the schema
func (s *Service) Initialize(ctx context.Context) error {
	if s.initialized {
		return nil
	}

	s.logger.Info("Initializing graph service")

	// Connect to graph database with exponential backoff retry
	// This is necessary when running with FalkorDB as a sidecar container
	// which may take several seconds to start up
	const maxRetries = 20
	const maxBackoff = 10 * time.Second
	initialBackoff := 500 * time.Millisecond

	// First, establish the connection (this usually succeeds quickly)
	if err := s.client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to create graph database client: %w", err)
	}

	// Now retry the Ping operation with exponential backoff
	// The Ping is what actually verifies FalkorDB is ready to accept queries
	var lastErr error
	pingSucceeded := false
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := initialBackoff * time.Duration(1<<uint(attempt-1))
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			s.logger.Info("Retrying ping to graph database in %v (attempt %d/%d)", backoff, attempt+1, maxRetries)
			time.Sleep(backoff)
		}

		// Verify connection with ping
		if err := s.client.Ping(ctx); err != nil {
			lastErr = err
			if attempt == 0 {
				s.logger.Debug("Initial ping failed (FalkorDB may still be starting): %v", err)
			} else {
				s.logger.Debug("Ping attempt %d failed: %v", attempt+1, err)
			}
			continue
		}

		// Connection and ping successful
		s.logger.Info("Successfully connected to graph database")
		pingSucceeded = true
		break
	}

	if !pingSucceeded {
		return fmt.Errorf("failed to ping graph database after %d attempts: %w", maxRetries, lastErr)
	}

	// Initialize schema (create indexes)
	if err := s.schema.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	s.initialized = true
	s.logger.Info("Graph service initialized successfully")
	return nil
}

// Start starts the graph sync pipeline
func (s *Service) Start(ctx context.Context) error {
	if !s.initialized {
		return fmt.Errorf("service not initialized - call Initialize() first")
	}

	if s.running {
		return fmt.Errorf("service already running")
	}

	s.logger.Info("Starting graph service")

	// Start pipeline
	if err := s.pipeline.Start(ctx); err != nil {
		return fmt.Errorf("failed to start pipeline: %w", err)
	}
	s.logger.Info("Graph sync pipeline started and ready to process events")

	// Start event listener
	if err := s.listener.Start(ctx); err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}
	s.logger.Info("Event listener started and ready to receive events")

	// Perform rebuild if configured (must happen after pipeline is started)
	if s.config.RebuildOnStart && s.rebuilder != nil {
		s.logger.Info("Graph rebuild on start is enabled (RebuildIfEmptyOnly: %v, RebuildWindow: %v)",
			s.config.RebuildIfEmptyOnly, s.config.RebuildWindow)
		
		opts := sync.RebuildOptions{
			TimeWindow:       s.config.RebuildWindow,
			BatchSize:        s.config.PipelineConfig.BatchSize,
			IncludeK8sEvents: true,
		}

		if s.config.RebuildIfEmptyOnly {
			// Check if graph is empty before rebuilding
			s.logger.Info("Checking if graph is empty before rebuilding...")
			if err := s.rebuilder.RebuildIfEmpty(ctx, opts, s.IsGraphEmpty); err != nil {
				s.logger.Warn("Failed to rebuild graph: %v", err)
				// Don't fail startup - continue with current graph state
			}
		} else {
			// Always rebuild
			s.logger.Info("Forcing graph rebuild (RebuildIfEmptyOnly is false)")
			if err := s.rebuilder.Rebuild(ctx, opts); err != nil {
				s.logger.Warn("Failed to rebuild graph: %v", err)
				// Don't fail startup - continue with current graph state
			}
		}
	} else if s.config.RebuildOnStart {
		s.logger.Warn("Graph rebuild on start is enabled but rebuilder is not initialized")
	} else {
		s.logger.Info("Graph rebuild on start is disabled")
	}

	// Start consuming batches
	go s.consumeBatches(ctx)

	s.running = true
	s.logger.Info("Graph service started - now listening for events from storage")
	return nil
}

// Stop stops the graph sync pipeline
func (s *Service) Stop(ctx context.Context) error {
	if !s.running {
		return nil
	}

	s.logger.Info("Stopping graph service")

	// Stop listener
	if err := s.listener.Stop(ctx); err != nil {
		s.logger.Warn("Error stopping listener: %v", err)
	}

	// Stop pipeline
	if err := s.pipeline.Stop(ctx); err != nil {
		s.logger.Warn("Error stopping pipeline: %v", err)
	}

	// Close graph client connection
	if err := s.client.Close(); err != nil {
		s.logger.Warn("Error closing graph client: %v", err)
	}

	s.running = false
	s.logger.Info("Graph service stopped")
	return nil
}

// InitializeWithStorage sets up the service with a storage backend for rebuild
func (s *Service) InitializeWithStorage(ctx context.Context, storage sync.StorageQuerier) error {
	if err := s.Initialize(ctx); err != nil {
		return err
	}

	// Create rebuilder (rebuild will happen in Start() after pipeline is ready)
	s.rebuilder = sync.NewRebuilder(storage, s.pipeline)
	s.logger.Info("Rebuilder initialized with storage backend")

	return nil
}

// OnEvent handles an event from Spectre storage
// This is called by storage whenever a new event is written
func (s *Service) OnEvent(event models.Event) error {
	if !s.running {
		s.logger.Warn("Received event %s but service is not running", event.ID)
		return fmt.Errorf("service not running")
	}

	s.logger.Debug("Graph service received event %s from storage, forwarding to listener", event.ID)
	// Forward to listener
	return s.listener.OnEvent(event)
}

// consumeBatches consumes event batches from the listener and processes them
func (s *Service) consumeBatches(ctx context.Context) {
	s.logger.Info("Starting batch consumer")

	batchCh := s.listener.Subscribe()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Batch consumer context cancelled")
			return

		case batch, ok := <-batchCh:
			if !ok {
				s.logger.Info("Batch channel closed")
				return
			}

			s.logger.Debug("Received batch %s with %d events", batch.BatchID, len(batch.Events))

			// Process batch through pipeline
			if err := s.pipeline.ProcessBatch(ctx, batch.Events); err != nil {
				s.logger.Error("Failed to process batch %s: %v", batch.BatchID, err)
				// Continue processing - don't fail entire service
			}
		}
	}
}

// IsGraphEmpty checks if the graph database is empty
func (s *Service) IsGraphEmpty(ctx context.Context) (bool, error) {
	// Query for any node
	query := graph.GraphQuery{
		Query: "MATCH (n) RETURN n LIMIT 1",
	}

	result, err := s.client.ExecuteQuery(ctx, query)
	if err != nil {
		return false, err
	}

	return len(result.Rows) == 0, nil
}

// GetStats returns combined statistics from all components
func (s *Service) GetStats() ServiceStats {
	return ServiceStats{
		Initialized:    s.initialized,
		Running:        s.running,
		PipelineStats:  s.pipeline.GetStats(),
		ListenerStats:  s.listener.GetStats(),
	}
}

// GetClient returns the graph client for direct queries
func (s *Service) GetClient() graph.Client {
	return s.client
}

// GetPipeline returns the sync pipeline
func (s *Service) GetPipeline() sync.Pipeline {
	return s.pipeline
}

// ServiceStats contains statistics from all service components
type ServiceStats struct {
	Initialized    bool
	Running        bool
	PipelineStats  sync.PipelineStats
	ListenerStats  sync.ListenerStats
}

// Name returns the component name for lifecycle management
func (s *Service) Name() string {
	return "graph"
}
