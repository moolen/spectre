package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// retentionManager implements the RetentionManager interface
type retentionManager struct {
	client          graph.Client
	logger          *logging.Logger
	retentionWindow time.Duration
}

// NewRetentionManager creates a new retention manager
func NewRetentionManager(client graph.Client, retentionWindow time.Duration) RetentionManager {
	return &retentionManager{
		client:          client,
		logger:          logging.GetLogger("graph.sync.retention"),
		retentionWindow: retentionWindow,
	}
}

// Cleanup removes data older than the retention window
func (r *retentionManager) Cleanup(ctx context.Context) error {
	r.logger.Info("Starting retention cleanup (window: %v)", r.retentionWindow)

	cutoffTime := time.Now().Add(-r.retentionWindow)
	cutoffNs := cutoffTime.UnixNano()

	// Delete old ChangeEvent nodes
	changeEventCount, err := r.cleanupChangeEvents(ctx, cutoffNs)
	if err != nil {
		return fmt.Errorf("failed to cleanup change events: %w", err)
	}

	// Delete old K8sEvent nodes
	k8sEventCount, err := r.cleanupK8sEvents(ctx, cutoffNs)
	if err != nil {
		return fmt.Errorf("failed to cleanup k8s events: %w", err)
	}

	// Clean up orphaned ResourceIdentity nodes (optional)
	// These are resources that have no events and were deleted
	orphanCount, err := r.cleanupOrphanedResources(ctx, cutoffNs)
	if err != nil {
		r.logger.Warn("Failed to cleanup orphaned resources: %v", err)
	}

	r.logger.Info("Retention cleanup complete: deleted %d change events, %d k8s events, %d orphaned resources",
		changeEventCount, k8sEventCount, orphanCount)

	return nil
}

// GetRetentionWindow returns the current retention window
func (r *retentionManager) GetRetentionWindow() time.Duration {
	return r.retentionWindow
}

// SetRetentionWindow updates the retention window
func (r *retentionManager) SetRetentionWindow(duration time.Duration) {
	r.logger.Info("Updating retention window from %v to %v", r.retentionWindow, duration)
	r.retentionWindow = duration
}

// cleanupChangeEvents deletes ChangeEvent nodes older than cutoff
func (r *retentionManager) cleanupChangeEvents(ctx context.Context, cutoffNs int64) (int, error) {
	query := graph.DeleteOldChangeEventsQuery(cutoffNs)

	result, err := r.client.ExecuteQuery(ctx, query)
	if err != nil {
		return 0, err
	}

	return result.Stats.NodesDeleted, nil
}

// cleanupK8sEvents deletes K8sEvent nodes older than cutoff
func (r *retentionManager) cleanupK8sEvents(ctx context.Context, cutoffNs int64) (int, error) {
	query := graph.DeleteOldK8sEventsQuery(cutoffNs)

	result, err := r.client.ExecuteQuery(ctx, query)
	if err != nil {
		return 0, err
	}

	return result.Stats.NodesDeleted, nil
}

// cleanupOrphanedResources deletes ResourceIdentity nodes that:
// 1. Were marked as deleted
// 2. Have no associated events
// 3. Were deleted before the cutoff time
func (r *retentionManager) cleanupOrphanedResources(ctx context.Context, cutoffNs int64) (int, error) {
	query := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity)
			WHERE r.deleted = true
			  AND r.deletedAt < $cutoffNs
			  AND NOT (r)-[:CHANGED]->(:ChangeEvent)
			  AND NOT (r)-[:EMITTED_EVENT]->(:K8sEvent)
			DETACH DELETE r
		`,
		Parameters: map[string]interface{}{
			"cutoffNs": cutoffNs,
		},
	}

	result, err := r.client.ExecuteQuery(ctx, query)
	if err != nil {
		return 0, err
	}

	return result.Stats.NodesDeleted, nil
}

// RunPeriodicCleanup runs cleanup periodically
func (r *retentionManager) RunPeriodicCleanup(ctx context.Context, interval time.Duration) error {
	r.logger.Info("Starting periodic retention cleanup (interval: %v)", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("Stopping periodic cleanup")
			return ctx.Err()

		case <-ticker.C:
			if err := r.Cleanup(ctx); err != nil {
				r.logger.Error("Retention cleanup failed: %v", err)
				// Continue running despite errors
			}
		}
	}
}
