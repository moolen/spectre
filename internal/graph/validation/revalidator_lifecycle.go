package validation

import (
	"context"
	"time"
)

// Run starts the revalidation background job
func (r *EdgeRevalidator) Run(ctx context.Context) error {
	r.logger.Info("Starting edge revalidator with interval %v", r.interval)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// Run immediately on start
	if err := r.revalidateEdges(ctx); err != nil {
		r.logger.Warn("Initial revalidation failed: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := r.revalidateEdges(ctx); err != nil {
				r.logger.Warn("Revalidation failed: %v", err)
			}
		case <-ctx.Done():
			r.logger.Info("Edge revalidator stopped")
			return ctx.Err()
		}
	}
}

// logStats logs revalidation statistics
func (r *EdgeRevalidator) logStats(stats *RevalidationStats) {
	duration := stats.EndTime.Sub(stats.StartTime)

	r.logger.Info(
		"Revalidation complete: total=%d, revalidated=%d, invalidated=%d, decayed=%d, stale=%d, updated=%d, errors=%d, duration=%v",
		stats.TotalEdges,
		stats.RevalidatedEdges,
		stats.InvalidatedEdges,
		stats.DecayedEdges,
		stats.StaleEdges,
		stats.UpdatedEdges,
		stats.ErrorCount,
		duration,
	)
}
