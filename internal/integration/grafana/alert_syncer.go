package grafana

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// AlertSyncer orchestrates incremental alert rule synchronization
type AlertSyncer struct {
	client          GrafanaClientInterface
	graphClient     graph.Client
	builder         *GraphBuilder
	integrationName string
	logger          *logging.Logger

	syncInterval time.Duration
	ctx          context.Context
	cancel       context.CancelFunc
	stopped      chan struct{}

	// Thread-safe sync status
	mu           sync.RWMutex
	lastSyncTime time.Time
	alertCount   int
	lastError    error
	inProgress   bool
}

// NewAlertSyncer creates a new alert syncer instance
func NewAlertSyncer(
	client GrafanaClientInterface,
	graphClient graph.Client,
	builder *GraphBuilder,
	integrationName string,
	logger *logging.Logger,
) *AlertSyncer {
	return &AlertSyncer{
		client:          client,
		graphClient:     graphClient,
		builder:         builder,
		integrationName: integrationName,
		logger:          logger,
		syncInterval:    time.Hour, // Default 1 hour
		stopped:         make(chan struct{}),
	}
}

// Start begins the sync loop (initial sync + periodic sync)
func (as *AlertSyncer) Start(ctx context.Context) error {
	as.logger.Info("Starting alert syncer (interval: %s)", as.syncInterval)

	// Create cancellable context
	as.ctx, as.cancel = context.WithCancel(ctx)

	// Run initial sync
	if err := as.syncAlerts(); err != nil {
		as.logger.Warn("Initial alert sync failed: %v (will retry on schedule)", err)
		as.setLastError(err)
	}

	// Start background sync loop
	go as.syncLoop(as.ctx)

	as.logger.Info("Alert syncer started successfully")
	return nil
}

// Stop gracefully stops the sync loop
func (as *AlertSyncer) Stop() {
	as.logger.Info("Stopping alert syncer")

	if as.cancel != nil {
		as.cancel()
	}

	// Wait for sync loop to stop (with timeout)
	select {
	case <-as.stopped:
		as.logger.Info("Alert syncer stopped")
	case <-time.After(5 * time.Second):
		as.logger.Warn("Alert syncer stop timeout")
	}
}

// syncLoop runs periodic sync on ticker interval
func (as *AlertSyncer) syncLoop(ctx context.Context) {
	defer close(as.stopped)

	ticker := time.NewTicker(as.syncInterval)
	defer ticker.Stop()

	as.logger.Debug("Alert sync loop started (interval: %s)", as.syncInterval)

	for {
		select {
		case <-ctx.Done():
			as.logger.Debug("Alert sync loop stopped (context cancelled)")
			return

		case <-ticker.C:
			as.logger.Debug("Periodic alert sync triggered")
			if err := as.syncAlerts(); err != nil {
				as.logger.Error("Periodic alert sync failed: %v", err)
				as.setLastError(err)
			}
		}
	}
}

// syncAlerts performs incremental alert rule synchronization
func (as *AlertSyncer) syncAlerts() error {
	startTime := time.Now()
	as.logger.Info("Starting alert sync")

	// Set inProgress flag
	as.mu.Lock()
	as.inProgress = true
	as.mu.Unlock()

	defer func() {
		as.mu.Lock()
		as.inProgress = false
		as.mu.Unlock()
	}()

	// Get list of all alert rules
	alertRules, err := as.client.ListAlertRules(as.ctx)
	if err != nil {
		return fmt.Errorf("failed to list alert rules: %w", err)
	}

	as.logger.Info("Found %d alert rules to process", len(alertRules))

	syncedCount := 0
	skippedCount := 0
	errorCount := 0

	// Process each alert rule
	for i, alertRule := range alertRules {
		// Log progress
		if (i+1)%10 == 0 || i == len(alertRules)-1 {
			as.logger.Debug("Processing alert rule %d of %d: %s", i+1, len(alertRules), alertRule.Title)
		}

		// Check if alert rule needs sync (timestamp comparison)
		needsSync, err := as.needsSync(alertRule)
		if err != nil {
			as.logger.Warn("Failed to check sync status for alert %s: %v (skipping)", alertRule.UID, err)
			errorCount++
			continue
		}

		if !needsSync {
			as.logger.Debug("Alert rule %s is up-to-date (skipping)", alertRule.UID)
			skippedCount++
			continue
		}

		// Sync alert rule to graph
		if err := as.builder.BuildAlertGraph(alertRule); err != nil {
			as.logger.Warn("Failed to sync alert rule %s: %v (continuing with others)", alertRule.UID, err)
			errorCount++
			continue
		}

		syncedCount++
	}

	// Update sync status
	as.mu.Lock()
	as.lastSyncTime = time.Now()
	as.alertCount = len(alertRules)
	if errorCount == 0 {
		as.lastError = nil
	}
	as.mu.Unlock()

	duration := time.Since(startTime)
	as.logger.Info("Alert sync complete: %d synced, %d skipped, %d errors (duration: %s)",
		syncedCount, skippedCount, errorCount, duration)

	if errorCount > 0 {
		return fmt.Errorf("sync completed with %d errors", errorCount)
	}

	return nil
}

// needsSync checks if an alert rule needs synchronization based on Updated timestamp
func (as *AlertSyncer) needsSync(alertRule AlertRule) (bool, error) {
	// Query graph for existing Alert node
	query := `
		MATCH (a:Alert {uid: $uid, integration: $integration})
		RETURN a.updated as updated
	`

	result, err := as.graphClient.ExecuteQuery(as.ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"uid":         alertRule.UID,
			"integration": as.integrationName,
		},
	})
	if err != nil {
		return false, fmt.Errorf("failed to query alert updated timestamp: %w", err)
	}

	// If alert doesn't exist in graph, needs sync
	if len(result.Rows) == 0 {
		as.logger.Debug("Alert %s not found in graph (needs sync)", alertRule.UID)
		return true, nil
	}

	// Parse updated timestamp from result
	if len(result.Rows[0]) == 0 {
		// No updated field, needs sync
		return true, nil
	}

	existingUpdated, ok := result.Rows[0][0].(string)
	if !ok {
		// Can't parse updated, assume needs sync
		as.logger.Debug("Alert %s has unparseable updated timestamp (needs sync)", alertRule.UID)
		return true, nil
	}

	// Compare ISO8601 timestamps (string comparison works for RFC3339 format)
	currentUpdated := alertRule.Updated.Format(time.RFC3339)
	needsSync := currentUpdated > existingUpdated

	if needsSync {
		as.logger.Debug("Alert %s timestamp changed: %s -> %s (needs sync)",
			alertRule.UID, existingUpdated, currentUpdated)
	}

	return needsSync, nil
}

// setLastError updates the last error (thread-safe)
func (as *AlertSyncer) setLastError(err error) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.lastError = err
}
