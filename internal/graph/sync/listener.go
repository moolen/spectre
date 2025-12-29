package sync

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// eventListener implements the EventListener interface
type eventListener struct {
	config   PipelineConfig
	logger   *logging.Logger
	eventCh  chan models.Event
	batchCh  chan EventBatch
	stopCh   chan struct{}
	wg       sync.WaitGroup

	// Statistics
	stats      ListenerStats
	statsLock  sync.RWMutex
}

// NewEventListener creates a new event listener
func NewEventListener(config PipelineConfig) EventListener {
	return &eventListener{
		config:  config,
		logger:  logging.GetLogger("graph.sync.listener"),
		eventCh: make(chan models.Event, config.BufferSize),
		batchCh: make(chan EventBatch, 10), // Buffer up to 10 batches
		stopCh:  make(chan struct{}),
		stats:   ListenerStats{},
	}
}

// Start begins listening for events
func (l *eventListener) Start(ctx context.Context) error {
	l.logger.Info("Starting event listener (batchSize: %d, batchTimeout: %v)",
		l.config.BatchSize, l.config.BatchTimeout)

	l.wg.Add(1)
	go l.batchProcessor(ctx)

	return nil
}

// Stop stops listening
func (l *eventListener) Stop(ctx context.Context) error {
	l.logger.Info("Stopping event listener")

	close(l.stopCh)

	// Wait for batch processor to finish
	done := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		l.logger.Info("Event listener stopped gracefully")
	case <-ctx.Done():
		l.logger.Warn("Event listener stop timed out")
		return ctx.Err()
	}

	// Close channels
	close(l.eventCh)
	close(l.batchCh)

	return nil
}

// Subscribe returns a channel of event batches
func (l *eventListener) Subscribe() <-chan EventBatch {
	return l.batchCh
}

// GetStats returns listener statistics
func (l *eventListener) GetStats() ListenerStats {
	l.statsLock.RLock()
	defer l.statsLock.RUnlock()
	return l.stats
}

// OnEvent is called when a new event is received
// This is the main entry point for events from Spectre storage
func (l *eventListener) OnEvent(event models.Event) error {
	l.logger.Debug("Received event from storage: %s (type: %s, resource: %s/%s)",
		event.ID, event.Type, event.Resource.Kind, event.Resource.Name)
	
	select {
	case l.eventCh <- event:
		atomic.AddInt64(&l.stats.EventsReceived, 1)
		l.updateStats(event)
		l.logger.Debug("Event %s queued for processing (buffer: %d/%d)",
			event.ID, len(l.eventCh), cap(l.eventCh))
		return nil
	case <-l.stopCh:
		return fmt.Errorf("listener stopped")
	default:
		// Buffer full - drop event or block?
		// For now, we'll try to send with a short timeout
		select {
		case l.eventCh <- event:
			atomic.AddInt64(&l.stats.EventsReceived, 1)
			l.updateStats(event)
			l.logger.Debug("Event %s queued for processing (buffer: %d/%d)",
				event.ID, len(l.eventCh), cap(l.eventCh))
			return nil
		case <-time.After(100 * time.Millisecond):
			l.logger.Warn("Event buffer full, dropping event %s", event.ID)
			return fmt.Errorf("event buffer full")
		}
	}
}

// batchProcessor collects events into batches and sends them to subscribers
func (l *eventListener) batchProcessor(ctx context.Context) {
	defer l.wg.Done()

	batch := make([]models.Event, 0, l.config.BatchSize)
	ticker := time.NewTicker(l.config.BatchTimeout)
	defer ticker.Stop()

	sendBatch := func() {
		if len(batch) == 0 {
			return
		}

		eventBatch := EventBatch{
			Events:    batch,
			Timestamp: time.Now(),
			BatchID:   uuid.New().String(),
		}

		select {
		case l.batchCh <- eventBatch:
			atomic.AddInt64(&l.stats.BatchesCreated, 1)
			l.logger.Debug("Sent batch %s with %d events", eventBatch.BatchID, len(batch))
		case <-ctx.Done():
			return
		case <-l.stopCh:
			return
		default:
			l.logger.Warn("Batch channel full, dropping batch with %d events", len(batch))
		}

		// Reset batch
		batch = make([]models.Event, 0, l.config.BatchSize)
	}

	for {
		select {
		case <-ctx.Done():
			l.logger.Info("Batch processor context cancelled")
			sendBatch() // Send any remaining events
			return

		case <-l.stopCh:
			l.logger.Info("Batch processor stopped")
			sendBatch() // Send any remaining events
			return

		case event, ok := <-l.eventCh:
			if !ok {
				l.logger.Info("Event channel closed")
				sendBatch() // Send any remaining events
				return
			}

			batch = append(batch, event)

			// Send batch if it's full
			if len(batch) >= l.config.BatchSize {
				sendBatch()
			}

		case <-ticker.C:
			// Send batch on timeout even if not full
			sendBatch()
		}
	}
}

// updateStats updates listener statistics
func (l *eventListener) updateStats(event models.Event) {
	l.statsLock.Lock()
	defer l.statsLock.Unlock()

	l.stats.LastEventTime = time.Unix(0, event.Timestamp)
	l.stats.BufferSize = len(l.eventCh)
	l.stats.BufferCapacity = cap(l.eventCh)
}
