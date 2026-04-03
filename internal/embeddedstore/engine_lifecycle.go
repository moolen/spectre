package embeddedstore

import (
	"context"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/models"
)

func (e *Engine) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if e == nil {
		return fmt.Errorf("start embedded engine: engine is nil")
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return fmt.Errorf("start embedded engine: engine is closed")
	}
	if e.config.FlushInterval <= 0 || e.periodicFlushStop != nil {
		return nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	e.periodicFlushStop = cancel
	go e.runPeriodicFlush(runCtx, e.config.FlushInterval)

	return nil
}

func (e *Engine) Stop(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if e == nil {
		return fmt.Errorf("stop embedded engine: engine is nil")
	}

	return e.Close()
}

func (e *Engine) Close() error {
	if e == nil {
		return nil
	}

	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil
	}
	if e.periodicFlushStop != nil {
		e.periodicFlushStop()
		e.periodicFlushStop = nil
	}
	e.mu.Unlock()

	if err := e.Flush(context.Background()); err != nil {
		return err
	}
	if err := e.Checkpoint(context.Background()); err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil
	}
	e.closed = true
	e.ready.Store(false)

	return nil
}

func (e *Engine) ProcessEvent(ctx context.Context, event models.Event) error {
	return e.ProcessBatch(ctx, []models.Event{event})
}

func (e *Engine) ProcessBatch(ctx context.Context, events []models.Event) (err error) {
	if len(events) == 0 {
		return nil
	}
	if e == nil {
		return fmt.Errorf("process embedded batch: engine is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	start := time.Now()
	defer func() {
		eventCount := 0
		if err == nil {
			eventCount = len(events)
		}
		e.metrics.RecordIngest(eventCount, time.Since(start), err)
	}()

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return fmt.Errorf("process embedded batch: engine is closed")
	}

	wasReady := e.ready.Load()
	for i := range events {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := applyProjectionEvent(e.projection, events[i]); err != nil {
			e.ready.Store(false)
			return fmt.Errorf("process embedded batch: apply event %d: %w", i, err)
		}
		e.hot.Append([]models.Event{events[i]})
		e.nextHighWaterMark++
	}
	if wasReady {
		e.ready.Store(true)
	}
	if e.shouldFlushHotBySizeLocked() {
		if err := e.flushLocked(ctx); err != nil {
			e.logAutoFlushError("size-triggered flush failed", err)
		}
	}

	return nil
}

func (e *Engine) runPeriodicFlush(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.Flush(context.Background()); err != nil {
				e.logAutoFlushError("periodic flush failed", err)
			}
		}
	}
}
