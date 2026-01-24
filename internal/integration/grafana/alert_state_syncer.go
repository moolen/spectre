package grafana

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// AlertStateSyncer orchestrates periodic alert state synchronization
type AlertStateSyncer struct {
	client          GrafanaClientInterface
	graphClient     graph.Client
	builder         *GraphBuilder
	integrationName string
	logger          *logging.Logger

	syncInterval time.Duration // 5 minutes per CONTEXT.md
	ctx          context.Context
	cancel       context.CancelFunc
	stopped      chan struct{}

	// Thread-safe sync status
	mu              sync.RWMutex
	lastSyncTime    time.Time
	transitionCount int
	lastError       error
	inProgress      bool
}

// NewAlertStateSyncer creates a new alert state syncer instance
func NewAlertStateSyncer(
	client GrafanaClientInterface,
	graphClient graph.Client,
	builder *GraphBuilder,
	integrationName string,
	logger *logging.Logger,
) *AlertStateSyncer {
	return &AlertStateSyncer{
		client:          client,
		graphClient:     graphClient,
		builder:         builder,
		integrationName: integrationName,
		logger:          logger,
		syncInterval:    5 * time.Minute, // 5-minute interval per CONTEXT.md
		stopped:         make(chan struct{}),
	}
}

// Start begins the sync loop (initial sync + periodic sync)
func (ass *AlertStateSyncer) Start(ctx context.Context) error {
	ass.logger.Info("Starting alert state syncer (interval: %s)", ass.syncInterval)

	// Create cancellable context
	ass.ctx, ass.cancel = context.WithCancel(ctx)

	// Run initial sync
	if err := ass.syncStates(); err != nil {
		ass.logger.Warn("Initial alert state sync failed: %v (will retry on schedule)", err)
		ass.setLastError(err)
	}

	// Start background sync loop
	go ass.syncLoop(ass.ctx)

	ass.logger.Info("Alert state syncer started successfully")
	return nil
}

// Stop gracefully stops the sync loop
func (ass *AlertStateSyncer) Stop() {
	ass.logger.Info("Stopping alert state syncer")

	if ass.cancel != nil {
		ass.cancel()
	}

	// Wait for sync loop to stop (with timeout)
	select {
	case <-ass.stopped:
		ass.logger.Info("Alert state syncer stopped")
	case <-time.After(5 * time.Second):
		ass.logger.Warn("Alert state syncer stop timeout")
	}
}

// syncLoop runs periodic sync on ticker interval
func (ass *AlertStateSyncer) syncLoop(ctx context.Context) {
	defer close(ass.stopped)

	ticker := time.NewTicker(ass.syncInterval)
	defer ticker.Stop()

	ass.logger.Debug("Alert state sync loop started (interval: %s)", ass.syncInterval)

	for {
		select {
		case <-ctx.Done():
			ass.logger.Debug("Alert state sync loop stopped (context cancelled)")
			return

		case <-ticker.C:
			ass.logger.Debug("Periodic alert state sync triggered")
			if err := ass.syncStates(); err != nil {
				ass.logger.Warn("Periodic alert state sync failed: %v", err)
				ass.setLastError(err)
			}
		}
	}
}

// syncStates performs alert state synchronization with deduplication
func (ass *AlertStateSyncer) syncStates() error {
	startTime := time.Now()
	ass.logger.Info("Starting alert state sync")

	// Set inProgress flag
	ass.mu.Lock()
	ass.inProgress = true
	ass.mu.Unlock()

	defer func() {
		ass.mu.Lock()
		ass.inProgress = false
		ass.mu.Unlock()
	}()

	// Get current alert states from Grafana
	alertStates, err := ass.client.GetAlertStates(ass.ctx)
	if err != nil {
		// On API error: log warning, set lastError, DON'T update lastSyncTime
		ass.logger.Warn("Failed to get alert states from Grafana API: %v", err)
		return fmt.Errorf("failed to get alert states: %w", err)
	}

	ass.logger.Info("Found %d alerts to process", len(alertStates))

	transitionCount := 0
	skippedCount := 0
	errorCount := 0

	// Process each alert state
	for _, alertState := range alertStates {
		// Aggregate instance states to worst case
		currentState := ass.aggregateInstanceStates(alertState.Instances)

		ass.logger.Debug("Alert %s current state: %s (from %d instances)",
			alertState.UID, currentState, len(alertState.Instances))

		// Get last known state from graph
		lastState, err := ass.builder.getLastKnownState(ass.ctx, alertState.UID)
		if err != nil {
			// Log error but continue with other alerts
			ass.logger.Warn("Failed to get last known state for alert %s: %v (skipping)", alertState.UID, err)
			errorCount++
			continue
		}

		// Compare current vs last state (deduplication)
		if currentState == lastState {
			// No state change - skip transition creation
			ass.logger.Debug("Alert %s state unchanged (%s), skipping transition", alertState.UID, currentState)
			skippedCount++

			// Still update last_synced_at (successful sync even if no state change)
			if err := ass.updateLastSyncedAt(alertState.UID); err != nil {
				ass.logger.Warn("Failed to update last_synced_at for alert %s: %v", alertState.UID, err)
				errorCount++
			}
			continue
		}

		// State changed - create transition edge
		ass.logger.Debug("Alert %s: %s -> %s", alertState.UID, lastState, currentState)

		if err := ass.builder.CreateStateTransitionEdge(
			ass.ctx,
			alertState.UID,
			lastState,
			currentState,
			time.Now(),
		); err != nil {
			// Log error but continue with other alerts
			ass.logger.Warn("Failed to create state transition for alert %s: %v (continuing)", alertState.UID, err)
			errorCount++
			continue
		}

		transitionCount++

		// Update last_synced_at timestamp (per-alert granularity)
		if err := ass.updateLastSyncedAt(alertState.UID); err != nil {
			ass.logger.Warn("Failed to update last_synced_at for alert %s: %v", alertState.UID, err)
			errorCount++
		}
	}

	// Update sync status
	ass.mu.Lock()
	ass.lastSyncTime = time.Now()
	ass.transitionCount = transitionCount
	if errorCount == 0 {
		ass.lastError = nil
	}
	ass.mu.Unlock()

	duration := time.Since(startTime)
	ass.logger.Info("Alert state sync complete: %d transitions stored, %d skipped (no change), %d errors (duration: %s)",
		transitionCount, skippedCount, errorCount, duration)

	if errorCount > 0 {
		return fmt.Errorf("sync completed with %d errors", errorCount)
	}

	return nil
}

// aggregateInstanceStates aggregates instance states to worst case
// Priority: firing > pending > normal
func (ass *AlertStateSyncer) aggregateInstanceStates(instances []AlertInstance) string {
	if len(instances) == 0 {
		return "normal"
	}

	// Check for firing state (highest priority)
	for _, instance := range instances {
		if instance.State == "firing" || instance.State == "alerting" {
			return "firing"
		}
	}

	// Check for pending state (medium priority)
	for _, instance := range instances {
		if instance.State == "pending" {
			return "pending"
		}
	}

	// Default to normal (all instances normal)
	return "normal"
}

// updateLastSyncedAt updates the last_synced_at timestamp for an alert node
func (ass *AlertStateSyncer) updateLastSyncedAt(alertUID string) error {
	now := time.Now().Format(time.RFC3339)

	query := `
		MERGE (a:Alert {uid: $uid, integration: $integration})
		SET a.last_synced_at = $now
	`

	_, err := ass.graphClient.ExecuteQuery(ass.ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"uid":         alertUID,
			"integration": ass.integrationName,
			"now":         now,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to update last_synced_at: %w", err)
	}

	return nil
}

// setLastError updates the last error (thread-safe)
func (ass *AlertStateSyncer) setLastError(err error) {
	ass.mu.Lock()
	defer ass.mu.Unlock()
	ass.lastError = err
}
