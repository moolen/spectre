package watcher

import (
	"context"
	"fmt"
	"sync"

	"github.com/moritz/rpk/internal/logging"
	"github.com/moritz/rpk/internal/models"
)

// EventQueue buffers events and handles concurrent event arrivals
type EventQueue struct {
	queue      chan *models.Event
	logger     *logging.Logger
	wg         sync.WaitGroup
	maxQueueSize int
	processFunc func(*models.Event) error
}

// NewEventQueue creates a new event queue
func NewEventQueue(maxSize int, processFunc func(*models.Event) error) *EventQueue {
	return &EventQueue{
		queue:        make(chan *models.Event, maxSize),
		logger:       logging.GetLogger("event_queue"),
		maxQueueSize: maxSize,
		processFunc:  processFunc,
	}
}

// Enqueue adds an event to the queue
func (eq *EventQueue) Enqueue(event *models.Event) error {
	// Validate the event first
	if err := event.Validate(); err != nil {
		eq.logger.Error("Invalid event: %v", err)
		return err
	}

	select {
	case eq.queue <- event:
		return nil
	default:
		eq.logger.Warn("Event queue is full, dropping event for %s/%s", event.Resource.Kind, event.Resource.Name)
		return fmt.Errorf("event queue is full")
	}
}

// Start begins processing events from the queue
func (eq *EventQueue) Start(ctx context.Context) {
	eq.logger.Info("Starting event queue processor")

	eq.wg.Add(1)
	go func() {
		defer eq.wg.Done()

		for {
			select {
			case event := <-eq.queue:
				if event == nil {
					// Channel closed
					return
				}

				// Process the event
				if err := eq.processFunc(event); err != nil {
					eq.logger.Error("Error processing event for %s/%s: %v", event.Resource.Kind, event.Resource.Name, err)
					// Log but continue processing other events
				}

			case <-ctx.Done():
				eq.logger.Info("Event queue processor stopped")
				return
			}
		}
	}()
}

// Stop stops the event queue processor
func (eq *EventQueue) Stop() {
	eq.logger.Info("Stopping event queue...")
	close(eq.queue)
	eq.wg.Wait()
	eq.logger.Info("Event queue stopped")
}

// GetQueueSize returns the current number of events in the queue
func (eq *EventQueue) GetQueueSize() int {
	return len(eq.queue)
}

// GetQueueCapacity returns the maximum queue size
func (eq *EventQueue) GetQueueCapacity() int {
	return eq.maxQueueSize
}

// GetQueueLoad returns the queue load as a percentage
func (eq *EventQueue) GetQueueLoad() float64 {
	return float64(eq.GetQueueSize()) / float64(eq.maxQueueSize) * 100.0
}

// IsNearCapacity checks if the queue is nearly full (>80%)
func (eq *EventQueue) IsNearCapacity() bool {
	return eq.GetQueueLoad() > 80.0
}

// IsAtCapacity checks if the queue is at capacity
func (eq *EventQueue) IsAtCapacity() bool {
	return eq.GetQueueSize() >= eq.maxQueueSize
}
