package sync

import (
	"context"
	"fmt"
	"time"
)

// Start begins the sync pipeline.
func (p *pipeline) Start(ctx context.Context) error {
	p.logger.Info("Starting graph sync pipeline")

	p.ctx, p.cancel = context.WithCancel(ctx)

	if err := p.schema.Initialize(p.ctx); err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	if err := p.bootstrapLabelIndex(p.ctx); err != nil {
		p.logger.Warn("Failed to bootstrap label index: %v (selector lookups will use graph queries initially)", err)
	}

	if p.config.RetentionWindow > 0 {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()

			ticker := time.NewTicker(1 * time.Hour)
			defer ticker.Stop()

			for {
				select {
				case <-p.ctx.Done():
					p.logger.Info("Stopping periodic cleanup")
					return
				case <-ticker.C:
					if err := p.retention.Cleanup(p.ctx); err != nil {
						p.logger.Error("Retention cleanup failed: %v", err)
					}
				}
			}
		}()
	}

	p.logger.Info("Graph sync pipeline started")
	return nil
}

// Stop gracefully stops the sync pipeline.
func (p *pipeline) Stop(ctx context.Context) error {
	p.logger.Info("Stopping graph sync pipeline")

	if p.cancel != nil {
		p.cancel()
	}

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info("Graph sync pipeline stopped gracefully")
	case <-ctx.Done():
		p.logger.Warn("Graph sync pipeline stop timed out")
		return ctx.Err()
	}

	return nil
}

// GetStats returns pipeline statistics.
func (p *pipeline) GetStats() PipelineStats {
	p.statsLock.RLock()
	defer p.statsLock.RUnlock()
	return p.stats
}

// updateProcessingRate updates the processing rate statistic.
func (p *pipeline) updateProcessingRate() {
	p.statsLock.Lock()
	defer p.statsLock.Unlock()

	if p.stats.LastEventTime.IsZero() {
		p.stats.ProcessingRate = 0
		return
	}

	duration := time.Since(p.stats.LastSyncTime)
	if duration > 0 {
		p.stats.ProcessingRate = float64(p.stats.EventsProcessed) / duration.Seconds()
	}
}
