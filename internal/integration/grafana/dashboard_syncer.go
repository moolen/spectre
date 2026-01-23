package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/integration"
	"github.com/moolen/spectre/internal/logging"
)

// GrafanaClientInterface defines the interface for Grafana API operations
type GrafanaClientInterface interface {
	ListDashboards(ctx context.Context) ([]DashboardMeta, error)
	GetDashboard(ctx context.Context, uid string) (map[string]interface{}, error)
	ListAlertRules(ctx context.Context) ([]AlertRule, error)
}

// DashboardSyncer orchestrates incremental dashboard synchronization
type DashboardSyncer struct {
	grafanaClient GrafanaClientInterface
	graphClient   graph.Client
	graphBuilder  *GraphBuilder
	logger        *logging.Logger

	syncInterval time.Duration
	ctx          context.Context
	cancel       context.CancelFunc
	stopped      chan struct{}

	// Thread-safe sync status
	mu             sync.RWMutex
	lastSyncTime   time.Time
	dashboardCount int
	lastError      error
	inProgress     bool
}

// NewDashboardSyncer creates a new dashboard syncer instance
func NewDashboardSyncer(
	grafanaClient GrafanaClientInterface,
	graphClient graph.Client,
	config *Config,
	integrationName string,
	syncInterval time.Duration,
	logger *logging.Logger,
) *DashboardSyncer {
	return &DashboardSyncer{
		grafanaClient:  grafanaClient,
		graphClient:    graphClient,
		graphBuilder:   NewGraphBuilder(graphClient, config, integrationName, logger),
		logger:         logger,
		syncInterval:   syncInterval,
		stopped:        make(chan struct{}),
		dashboardCount: 0,
	}
}

// Start begins the sync loop (initial sync + periodic sync)
func (ds *DashboardSyncer) Start(ctx context.Context) error {
	ds.logger.Info("Starting dashboard syncer (interval: %s)", ds.syncInterval)

	// Create cancellable context
	ds.ctx, ds.cancel = context.WithCancel(ctx)

	// Run initial sync
	if err := ds.syncAll(ds.ctx); err != nil {
		ds.logger.Warn("Initial dashboard sync failed: %v (will retry on schedule)", err)
		ds.setLastError(err)
	}

	// Start background sync loop
	go ds.syncLoop(ds.ctx)

	ds.logger.Info("Dashboard syncer started successfully")
	return nil
}

// Stop gracefully stops the sync loop
func (ds *DashboardSyncer) Stop() {
	ds.logger.Info("Stopping dashboard syncer")

	if ds.cancel != nil {
		ds.cancel()
	}

	// Wait for sync loop to stop (with timeout)
	select {
	case <-ds.stopped:
		ds.logger.Info("Dashboard syncer stopped")
	case <-time.After(5 * time.Second):
		ds.logger.Warn("Dashboard syncer stop timeout")
	}
}

// GetSyncStatus returns current sync status (thread-safe)
func (ds *DashboardSyncer) GetSyncStatus() *integration.SyncStatus {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	status := &integration.SyncStatus{
		DashboardCount: ds.dashboardCount,
		InProgress:     ds.inProgress,
	}

	if !ds.lastSyncTime.IsZero() {
		status.LastSyncTime = &ds.lastSyncTime
	}

	if ds.lastError != nil {
		status.LastError = ds.lastError.Error()
	}

	return status
}

// syncLoop runs periodic sync on ticker interval
func (ds *DashboardSyncer) syncLoop(ctx context.Context) {
	defer close(ds.stopped)

	ticker := time.NewTicker(ds.syncInterval)
	defer ticker.Stop()

	ds.logger.Debug("Sync loop started (interval: %s)", ds.syncInterval)

	for {
		select {
		case <-ctx.Done():
			ds.logger.Debug("Sync loop stopped (context cancelled)")
			return

		case <-ticker.C:
			ds.logger.Debug("Periodic sync triggered")
			if err := ds.syncAll(ctx); err != nil {
				ds.logger.Error("Periodic dashboard sync failed: %v", err)
				ds.setLastError(err)
			}
		}
	}
}

// syncAll performs full dashboard sync with incremental version checking
func (ds *DashboardSyncer) syncAll(ctx context.Context) error {
	startTime := time.Now()
	ds.logger.Info("Starting dashboard sync")

	// Set inProgress flag
	ds.mu.Lock()
	ds.inProgress = true
	ds.mu.Unlock()

	defer func() {
		ds.mu.Lock()
		ds.inProgress = false
		ds.mu.Unlock()
	}()

	// Get list of all dashboards
	dashboards, err := ds.grafanaClient.ListDashboards(ctx)
	if err != nil {
		return fmt.Errorf("failed to list dashboards: %w", err)
	}

	ds.logger.Info("Found %d dashboards to process", len(dashboards))

	syncedCount := 0
	skippedCount := 0
	errorCount := 0

	// Process each dashboard
	for i, dashboardMeta := range dashboards {
		// Log progress
		if (i+1)%10 == 0 || i == len(dashboards)-1 {
			ds.logger.Debug("Syncing dashboard %d of %d: %s", i+1, len(dashboards), dashboardMeta.Title)
		}

		// Check if dashboard needs sync (version comparison)
		needsSync, err := ds.needsSync(ctx, dashboardMeta.UID)
		if err != nil {
			ds.logger.Warn("Failed to check sync status for dashboard %s: %v (skipping)", dashboardMeta.UID, err)
			errorCount++
			continue
		}

		if !needsSync {
			ds.logger.Debug("Dashboard %s is up-to-date (skipping)", dashboardMeta.UID)
			skippedCount++
			continue
		}

		// Get full dashboard details
		dashboardData, err := ds.grafanaClient.GetDashboard(ctx, dashboardMeta.UID)
		if err != nil {
			ds.logger.Warn("Failed to get dashboard %s: %v (skipping)", dashboardMeta.UID, err)
			errorCount++
			continue
		}

		// Parse dashboard JSON into struct
		dashboard, err := ds.parseDashboard(dashboardData, dashboardMeta)
		if err != nil {
			ds.logger.Warn("Failed to parse dashboard %s: %v (skipping)", dashboardMeta.UID, err)
			errorCount++
			continue
		}

		// Sync dashboard to graph
		if err := ds.syncDashboard(ctx, dashboard); err != nil {
			ds.logger.Warn("Failed to sync dashboard %s: %v (continuing with others)", dashboardMeta.UID, err)
			errorCount++
			continue
		}

		syncedCount++
	}

	// Update sync status
	ds.mu.Lock()
	ds.lastSyncTime = time.Now()
	ds.dashboardCount = len(dashboards)
	if errorCount == 0 {
		ds.lastError = nil
	}
	ds.mu.Unlock()

	duration := time.Since(startTime)
	ds.logger.Info("Dashboard sync complete: %d synced, %d skipped, %d errors (duration: %s)",
		syncedCount, skippedCount, errorCount, duration)

	if errorCount > 0 {
		return fmt.Errorf("sync completed with %d errors", errorCount)
	}

	return nil
}

// needsSync checks if a dashboard needs synchronization based on version comparison
func (ds *DashboardSyncer) needsSync(ctx context.Context, uid string) (bool, error) {
	// Query graph for existing dashboard node
	query := `
		MATCH (d:Dashboard {uid: $uid})
		RETURN d.version as version
	`

	result, err := ds.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"uid": uid,
		},
	})
	if err != nil {
		return false, fmt.Errorf("failed to query dashboard version: %w", err)
	}

	// If dashboard doesn't exist in graph, needs sync
	if len(result.Rows) == 0 {
		ds.logger.Debug("Dashboard %s not found in graph (needs sync)", uid)
		return true, nil
	}

	// Parse version from result
	if len(result.Rows[0]) == 0 {
		// No version field, needs sync
		return true, nil
	}

	var existingVersion int64
	switch v := result.Rows[0][0].(type) {
	case int64:
		existingVersion = v
	case float64:
		existingVersion = int64(v)
	default:
		// Can't parse version, assume needs sync
		ds.logger.Debug("Dashboard %s has unparseable version (needs sync)", uid)
		return true, nil
	}

	// Get dashboard metadata to compare versions
	dashboardData, err := ds.grafanaClient.GetDashboard(ctx, uid)
	if err != nil {
		return false, fmt.Errorf("failed to get dashboard for version check: %w", err)
	}

	// Extract version from dashboard data
	dashboardJSON, ok := dashboardData["dashboard"].(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("dashboard data missing 'dashboard' field")
	}

	var currentVersion int64
	if v, ok := dashboardJSON["version"].(float64); ok {
		currentVersion = int64(v)
	} else if v, ok := dashboardJSON["version"].(int64); ok {
		currentVersion = v
	} else {
		// Can't get current version, assume needs sync
		return true, nil
	}

	// Compare versions
	needsSync := currentVersion > existingVersion
	if needsSync {
		ds.logger.Debug("Dashboard %s version changed: %d -> %d (needs sync)",
			uid, existingVersion, currentVersion)
	}

	return needsSync, nil
}

// syncDashboard performs full dashboard replace (delete old panels/queries, recreate)
func (ds *DashboardSyncer) syncDashboard(ctx context.Context, dashboard *GrafanaDashboard) error {
	ds.logger.Debug("Syncing dashboard: %s (version: %d)", dashboard.UID, dashboard.Version)

	// Delete old panels and queries (full replace pattern)
	if err := ds.graphBuilder.DeletePanelsForDashboard(ctx, dashboard.UID); err != nil {
		return fmt.Errorf("failed to delete old panels: %w", err)
	}

	// Create new dashboard graph structure
	if err := ds.graphBuilder.CreateDashboardGraph(ctx, dashboard); err != nil {
		return fmt.Errorf("failed to create dashboard graph: %w", err)
	}

	ds.logger.Debug("Successfully synced dashboard: %s", dashboard.UID)
	return nil
}

// parseDashboard parses Grafana API response into GrafanaDashboard struct
func (ds *DashboardSyncer) parseDashboard(dashboardData map[string]interface{}, meta DashboardMeta) (*GrafanaDashboard, error) {
	// Extract dashboard JSON from API response
	// Grafana API returns: {"dashboard": {...}, "meta": {...}}
	dashboardJSON, ok := dashboardData["dashboard"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("dashboard data missing 'dashboard' field")
	}

	// Marshal and unmarshal to convert to struct
	// This handles nested structures and type conversions
	dashboardBytes, err := json.Marshal(dashboardJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal dashboard JSON: %w", err)
	}

	var dashboard GrafanaDashboard
	if err := json.Unmarshal(dashboardBytes, &dashboard); err != nil {
		return nil, fmt.Errorf("failed to parse dashboard JSON: %w", err)
	}

	// Fill in metadata from DashboardMeta (API list endpoint provides this)
	if dashboard.UID == "" {
		dashboard.UID = meta.UID
	}
	if dashboard.Title == "" {
		dashboard.Title = meta.Title
	}
	if len(dashboard.Tags) == 0 {
		dashboard.Tags = meta.Tags
	}

	return &dashboard, nil
}

// TriggerSync triggers a manual sync, returning error if sync already in progress
func (ds *DashboardSyncer) TriggerSync(ctx context.Context) error {
	ds.mu.RLock()
	if ds.inProgress {
		ds.mu.RUnlock()
		return fmt.Errorf("sync already in progress")
	}
	ds.mu.RUnlock()

	return ds.syncAll(ctx)
}

// setLastError updates the last error (thread-safe)
func (ds *DashboardSyncer) setLastError(err error) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.lastError = err
}
