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
	config         ServiceConfig
	client         graph.Client
	schema         *graph.Schema
	pipeline       sync.Pipeline
	listener       sync.EventListener
	changeDetector *sync.NamespaceChangeDetector
	logger         *logging.Logger

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

	// Integration
	AutoStartPipeline bool
}

// DefaultServiceConfig returns default service configuration
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		GraphConfig:       graph.DefaultClientConfig(),
		PipelineConfig:    sync.DefaultPipelineConfig(),
		AutoStartPipeline: true,
	}
}

// NewService creates a new graph service
func NewService(config ServiceConfig) *Service {
	client := graph.NewClient(config.GraphConfig)
	pipeline := sync.NewPipeline(config.PipelineConfig, client)
	listener := sync.NewEventListener(config.PipelineConfig)
	logger := logging.GetLogger("graph.service")

	// Create change detector for event-driven cache invalidation
	changeDetector := sync.NewNamespaceChangeDetector(
		sync.DefaultNamespaceChangeDetectorConfig(),
		client,
		logging.GetLogger("graph.change_detector"),
	)

	return &Service{
		config:         config,
		client:         client,
		schema:         graph.NewSchema(client),
		pipeline:       pipeline,
		listener:       listener,
		changeDetector: changeDetector,
		logger:         logger,
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
			// Exponential backoff calculation - attempt is bounded by maxRetries (20)
			// #nosec G115 -- attempt-1 is bounded by maxRetries and will never overflow
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

	// Start change detector for event-driven cache invalidation
	if s.changeDetector != nil {
		s.changeDetector.Start(ctx)
		s.logger.Info("Namespace change detector started")
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

	// Stop change detector first (stops watching for events)
	if s.changeDetector != nil {
		s.changeDetector.Stop()
	}

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

// InitializeWithStorage is deprecated - storage package removed
// This method is kept for compatibility but does nothing
func (s *Service) InitializeWithStorage(ctx context.Context, storage interface{}) error {
	// Storage rebuild is no longer supported
	return s.Initialize(ctx)
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

			// Notify change detector for event-driven cache invalidation
			// This happens AFTER pipeline processing so the graph is up-to-date
			// when caches query for related namespaces
			if s.changeDetector != nil {
				s.changeDetector.OnEventBatch(ctx, batch.Events)
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

// RegisterCacheInvalidator registers a CacheInvalidator to receive
// event-driven invalidation notifications from the NamespaceChangeDetector.
// This should be called after the GraphService is created but before Start().
func (s *Service) RegisterCacheInvalidator(invalidator sync.CacheInvalidator) {
	if s.changeDetector != nil {
		s.changeDetector.Subscribe(invalidator)
	}
}
