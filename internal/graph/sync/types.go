package sync

import (
	"context"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/models"
)

// Pipeline coordinates the synchronization of events from Spectre storage to the graph
type Pipeline interface {
	// Start begins the sync pipeline
	Start(ctx context.Context) error

	// Stop gracefully stops the sync pipeline
	Stop(ctx context.Context) error

	// ProcessEvent processes a single event (used for manual sync)
	ProcessEvent(ctx context.Context, event models.Event) error

	// ProcessBatch processes a batch of events
	ProcessBatch(ctx context.Context, events []models.Event) error

	// GetStats returns pipeline statistics
	GetStats() PipelineStats
}

// EventListener listens for new events from Spectre storage
type EventListener interface {
	// Start begins listening for events
	Start(ctx context.Context) error

	// Stop stops listening
	Stop(ctx context.Context) error

	// Subscribe returns a channel of event batches
	Subscribe() <-chan EventBatch

	// OnEvent handles a new event from storage
	OnEvent(event models.Event) error

	// GetStats returns listener statistics
	GetStats() ListenerStats
}

// GraphBuilder transforms Spectre events into graph nodes and edges
type GraphBuilder interface {
	// BuildFromEvent creates graph nodes/edges from a Spectre event (combines both phases)
	BuildFromEvent(ctx context.Context, event models.Event) (*GraphUpdate, error)

	// BuildFromBatch processes multiple events and returns graph updates
	BuildFromBatch(ctx context.Context, events []models.Event) ([]*GraphUpdate, error)

	// BuildResourceNodes creates just the resource and event nodes (Phase 1)
	BuildResourceNodes(event models.Event) (*GraphUpdate, error)

	// BuildRelationshipEdges extracts relationship edges only (Phase 2)
	BuildRelationshipEdges(ctx context.Context, event models.Event) (*GraphUpdate, error)

	// ExtractRelationships extracts relationships from resource data
	ExtractRelationships(ctx context.Context, event models.Event) ([]graph.Edge, error)
}

// CausalityEngine infers causality relationships between events
type CausalityEngine interface {
	// InferCausality analyzes events and creates TRIGGERED_BY edges
	InferCausality(ctx context.Context, events []models.Event) ([]CausalityLink, error)

	// AnalyzePair checks if two events have a causal relationship
	AnalyzePair(ctx context.Context, cause, effect models.Event) (*CausalityLink, error)

	// GetHeuristics returns the configured causality heuristics
	GetHeuristics() []CausalityHeuristic
}

// RetentionManager handles cleanup of old graph data
type RetentionManager interface {
	// Cleanup removes data older than the retention window
	Cleanup(ctx context.Context) error

	// GetRetentionWindow returns the current retention window
	GetRetentionWindow() time.Duration

	// SetRetentionWindow updates the retention window
	SetRetentionWindow(duration time.Duration)
}

// EventBatch represents a batch of events to process
type EventBatch struct {
	Events    []models.Event
	Timestamp time.Time
	BatchID   string
}

// GraphUpdate represents changes to apply to the graph
type GraphUpdate struct {
	// Nodes to create or update
	ResourceNodes []graph.ResourceIdentity
	EventNodes    []graph.ChangeEvent
	K8sEventNodes []graph.K8sEvent

	// Edges to create
	Edges []graph.Edge

	// Metadata
	SourceEventID string
	Timestamp     time.Time
}

// CausalityLink represents an inferred causal relationship
type CausalityLink struct {
	CauseEventID  string
	EffectEventID string
	Confidence    float64
	LagMs         int64
	Reason        string
	HeuristicUsed string
}

// CausalityHeuristic defines a rule for inferring causality
type CausalityHeuristic struct {
	Name        string
	Description string
	MinLagMs    int64   // Minimum time lag to consider
	MaxLagMs    int64   // Maximum time lag to consider
	Confidence  float64 // Base confidence score
	Apply       func(cause, effect models.Event) bool
}

// PipelineStats tracks sync pipeline metrics
type PipelineStats struct {
	EventsProcessed     int64
	EventsSkipped       int64
	NodesCreated        int64
	EdgesCreated        int64
	CausalityLinksFound int64
	Errors              int64
	LastEventTime       time.Time
	LastSyncTime        time.Time
	SyncLagMs           int64
	ProcessingRate      float64 // events per second
}

// ListenerStats tracks event listener metrics
type ListenerStats struct {
	EventsReceived int64
	BatchesCreated int64
	LastEventTime  time.Time
	BufferSize     int
	BufferCapacity int
}

// PipelineConfig configures the sync pipeline
type PipelineConfig struct {
	// Batch processing
	BatchSize     int           // Number of events per batch
	BatchTimeout  time.Duration // Max time to wait for batch to fill
	BufferSize    int           // Event buffer size
	WorkerCount   int           // Number of parallel workers

	// Retention
	RetentionWindow time.Duration // How long to keep events in graph

	// Causality inference
	EnableCausality      bool          // Enable causality inference
	CausalityMaxLag      time.Duration // Max time lag for causality
	CausalityMinConfidence float64     // Min confidence to create edge

	// Performance
	EnableAsync    bool          // Process events asynchronously
	SyncTimeout    time.Duration // Timeout for graph operations
	RetryAttempts  int           // Number of retries on failure
	RetryDelay     time.Duration // Delay between retries
}

// DefaultPipelineConfig returns default pipeline configuration
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		BatchSize:              100,
		BatchTimeout:           5 * time.Second,
		BufferSize:             1000,
		WorkerCount:            4,
		RetentionWindow:        24 * time.Hour,
		EnableCausality:        true,
		CausalityMaxLag:        5 * time.Minute,
		CausalityMinConfidence: 0.5,
		EnableAsync:            true,
		SyncTimeout:            10 * time.Second,
		RetryAttempts:          3,
		RetryDelay:             1 * time.Second,
	}
}
