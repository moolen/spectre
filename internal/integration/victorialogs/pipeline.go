package victorialogs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

// Pipeline is a backpressure-aware log ingestion pipeline for VictoriaLogs.
// It batches log entries and sends them to VictoriaLogs in groups, with bounded
// memory usage via a buffered channel.
//
// Key characteristics:
// - Bounded buffer (1000 entries) provides natural backpressure (blocks when full)
// - Batching (100 entries) reduces HTTP overhead
// - Periodic flushing (1 second) prevents partial batches from stalling
// - Graceful shutdown with timeout ensures no data loss
type Pipeline struct {
	logChan   chan LogEntry       // Bounded channel for backpressure
	batchSize int                 // Number of entries per batch (fixed: 100)
	client    *Client             // VictoriaLogs HTTP client
	metrics   *Metrics            // Prometheus metrics
	logger    *logging.Logger     // Component logger
	wg        sync.WaitGroup      // Worker coordination
	ctx       context.Context     // Cancellation context
	cancel    context.CancelFunc  // Cancellation function
}

// NewPipeline creates a new log ingestion pipeline for a VictoriaLogs instance.
// The pipeline must be started with Start() before ingesting logs.
func NewPipeline(client *Client, metrics *Metrics, instanceName string) *Pipeline {
	logger := logging.GetLogger(fmt.Sprintf("victorialogs.pipeline.%s", instanceName))
	return &Pipeline{
		client:    client,
		metrics:   metrics,
		batchSize: 100, // Fixed batch size for consistent memory usage
		logger:    logger,
	}
}

// Start begins the batch processing pipeline.
// It creates the bounded channel and starts the background worker goroutine.
func (p *Pipeline) Start(ctx context.Context) error {
	// Create cancellable context for pipeline lifecycle
	p.ctx, p.cancel = context.WithCancel(ctx)

	// Create bounded channel - size 1000 provides backpressure
	p.logChan = make(chan LogEntry, 1000)

	// Start batch processor worker
	p.wg.Add(1)
	go p.batchProcessor()

	p.logger.Info("Pipeline started with buffer=1000, batchSize=100")
	return nil
}

// Ingest accepts a log entry for processing.
// This method BLOCKS when the buffer is full, providing natural backpressure.
// Returns error only if the pipeline has been stopped.
func (p *Pipeline) Ingest(entry LogEntry) error {
	select {
	case p.logChan <- entry:
		// Successfully enqueued - update queue depth metric
		p.metrics.QueueDepth.Set(float64(len(p.logChan)))
		return nil
	case <-p.ctx.Done():
		// Pipeline stopped - reject new entries
		return fmt.Errorf("pipeline stopped")
	}
	// NOTE: No default case - this is intentional! We want to block when the buffer is full.
}

// batchProcessor is the background worker that accumulates and sends batches.
// It runs in a goroutine and flushes batches when:
// 1. Batch reaches target size (100 entries)
// 2. Timeout occurs (1 second - prevents partial batches from stalling)
// 3. Pipeline stops (graceful shutdown - flushes remaining entries)
func (p *Pipeline) batchProcessor() {
	defer p.wg.Done()

	// Allocate batch buffer with capacity for full batch
	batch := make([]LogEntry, 0, p.batchSize)

	// Create ticker for periodic flushing (prevents partial batches from waiting forever)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	p.logger.Debug("Batch processor started")

	for {
		select {
		case entry, ok := <-p.logChan:
			if !ok {
				// Channel closed - flush remaining batch and exit
				if len(batch) > 0 {
					p.logger.Info("Flushing final batch of %d logs on shutdown", len(batch))
					p.sendBatch(batch)
				}
				p.logger.Debug("Batch processor stopped")
				return
			}

			// Add entry to batch
			batch = append(batch, entry)

			// Update queue depth metric
			p.metrics.QueueDepth.Set(float64(len(p.logChan)))

			// Send batch when it reaches target size
			if len(batch) >= p.batchSize {
				p.sendBatch(batch)
				batch = batch[:0] // Reset batch (reuse underlying array)
			}

		case <-ticker.C:
			// Periodic flush - send partial batch if any entries exist
			if len(batch) > 0 {
				p.logger.Debug("Flushing partial batch of %d logs (timeout)", len(batch))
				p.sendBatch(batch)
				batch = batch[:0] // Reset batch
			}

		case <-p.ctx.Done():
			// Pipeline stopped - flush remaining batch and exit
			if len(batch) > 0 {
				p.logger.Info("Flushing remaining batch of %d logs on cancellation", len(batch))
				p.sendBatch(batch)
			}
			p.logger.Debug("Batch processor stopped (cancelled)")
			return
		}
	}
}

// sendBatch sends a batch of log entries to VictoriaLogs.
// Errors are logged and counted but do not crash the pipeline (resilience).
func (p *Pipeline) sendBatch(batch []LogEntry) {
	// Call client to ingest batch
	err := p.client.IngestBatch(p.ctx, batch)
	if err != nil {
		// Log error and increment error counter
		p.logger.Error("Failed to send batch: %v", err)
		p.metrics.ErrorsTotal.Inc()
		return
	}

	// Success - increment counter by number of logs (not batch count!)
	p.metrics.BatchesTotal.Add(float64(len(batch)))
	p.logger.Debug("Sent batch of %d logs", len(batch))
}

// Stop gracefully shuts down the pipeline with a timeout.
// It drains the buffer and waits for the worker to finish flushing.
// Returns error if shutdown timeout is exceeded.
func (p *Pipeline) Stop(ctx context.Context) error {
	p.logger.Info("Stopping pipeline, draining buffer...")

	// Signal cancellation to worker
	p.cancel()

	// Close channel to drain remaining entries
	close(p.logChan)

	// Wait for worker to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info("Pipeline stopped cleanly")
		return nil
	case <-ctx.Done():
		p.logger.Error("Pipeline shutdown timeout")
		return fmt.Errorf("shutdown timeout")
	}
}
