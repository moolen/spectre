package sync

import (
	"context"
	"sync"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// maxBatchSize is the maximum number of items to process in a single FalkorDB query.
// FalkorDB performs better with smaller batches; large batches can cause timeouts or partial writes.
// Using 1000 instead of 5000 for better reliability with large imports.
const maxBatchSize = 1000

// batchQueryTimeout is the timeout for batch queries in milliseconds.
const batchQueryTimeout = 60000 // 60 seconds

// pipeline implements the Pipeline interface
type pipeline struct {
	config    PipelineConfig
	client    graph.Client
	schema    *graph.Schema
	builder   GraphBuilder
	causality CausalityEngine
	retention RetentionManager
	logger    *logging.Logger

	// Statistics (atomic counters)
	stats     PipelineStats
	statsLock sync.RWMutex

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewPipeline creates a new sync pipeline
func NewPipeline(config PipelineConfig, client graph.Client) Pipeline {
	p := &pipeline{
		config:    config,
		client:    client,
		schema:    graph.NewSchema(client),
		builder:   NewGraphBuilderWithClientAndCacheSize(client, config.StateCacheSize),
		causality: NewCausalityEngine(config.CausalityMaxLag, config.CausalityMinConfidence),
		retention: NewRetentionManager(client, config.RetentionWindow),
		logger:    logging.GetLogger("graph.sync.pipeline"),
		stats:     PipelineStats{},
	}

	return p
}
