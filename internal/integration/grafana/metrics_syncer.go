package grafana

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// MetricsSyncerConfig holds configuration for the metrics syncer.
type MetricsSyncerConfig struct {
	// SyncInterval is how often to run metrics sync.
	// Default: 1 hour
	SyncInterval time.Duration

	// RateLimitInterval is the minimum time between Grafana API calls.
	// Default: 100ms (10 req/sec)
	RateLimitInterval time.Duration

	// DatasourceUID is the Prometheus datasource to query.
	// If empty, the default Prometheus datasource is used.
	DatasourceUID string
}

// DefaultMetricsSyncerConfig returns default configuration.
func DefaultMetricsSyncerConfig() MetricsSyncerConfig {
	return MetricsSyncerConfig{
		SyncInterval:      time.Hour,
		RateLimitInterval: 100 * time.Millisecond, // 10 req/sec
		DatasourceUID:     "",                     // Use default
	}
}

// SignalAnchorCallback is called when a SignalAnchor is created or updated.
// Implementations can use this to trigger additional actions, such as
// linking the anchor to K8s workloads via scrape target metadata.
type SignalAnchorCallback interface {
	// OnSignalAnchorCreated is called after a new SignalAnchor is created.
	// metricName: the metric name of the anchor
	// workloadNamespace: the workload namespace (empty for global anchors)
	// workloadName: the workload name (empty for global anchors)
	OnSignalAnchorCreated(ctx context.Context, metricName, workloadNamespace, workloadName string) error
}

// MetricsSyncer orchestrates periodic curated metric ingestion and SignalAnchor creation.
// It fetches metric names from Prometheus via Grafana, matches against curated metrics,
// and creates/updates SignalAnchors in the graph database.
//
// Sync runs on a 1-hour interval with rate limiting to protect the Grafana API.
type MetricsSyncer struct {
	client          *GrafanaClient
	graphClient     graph.Client
	integrationName string
	logger          *logging.Logger
	config          MetricsSyncerConfig

	// Lifecycle
	ctx     context.Context
	cancel  context.CancelFunc
	stopped chan struct{}

	// Rate limiting
	rateLimiter *time.Ticker

	// Callbacks for event-driven linking
	callbacks []SignalAnchorCallback

	// Thread-safe status
	mu           sync.RWMutex
	lastSyncTime time.Time
	matchedCount int
	totalMetrics int
	createdCount int
	updatedCount int
	lastError    error
	inProgress   bool
}

// NewMetricsSyncer creates a new metrics syncer with default config.
func NewMetricsSyncer(
	client *GrafanaClient,
	graphClient graph.Client,
	integrationName string,
	logger *logging.Logger,
) *MetricsSyncer {
	return NewMetricsSyncerWithConfig(
		client,
		graphClient,
		integrationName,
		logger,
		DefaultMetricsSyncerConfig(),
	)
}

// NewMetricsSyncerWithConfig creates a new metrics syncer with custom config.
func NewMetricsSyncerWithConfig(
	client *GrafanaClient,
	graphClient graph.Client,
	integrationName string,
	logger *logging.Logger,
	config MetricsSyncerConfig,
) *MetricsSyncer {
	return &MetricsSyncer{
		client:          client,
		graphClient:     graphClient,
		integrationName: integrationName,
		logger:          logger,
		config:          config,
		rateLimiter:     time.NewTicker(config.RateLimitInterval),
		stopped:         make(chan struct{}),
	}
}

// RegisterCallback registers a callback to be invoked when SignalAnchors are created.
// Callbacks are invoked synchronously, so implementations should be fast or spawn goroutines.
func (ms *MetricsSyncer) RegisterCallback(cb SignalAnchorCallback) {
	ms.callbacks = append(ms.callbacks, cb)
}

// Start begins the sync loop (initial sync + periodic sync).
func (ms *MetricsSyncer) Start(ctx context.Context) error {
	ms.logger.Info("Starting metrics syncer (interval: %s, datasource: %s)",
		ms.config.SyncInterval, ms.datasourceDisplay())

	// Create cancellable context
	ms.ctx, ms.cancel = context.WithCancel(ctx)

	// Run initial sync (with graceful failure)
	if err := ms.syncAll(ms.ctx); err != nil {
		ms.logger.Warn("Initial metrics sync failed: %v (will retry on schedule)", err)
		ms.setLastError(err)
	}

	// Start background sync loop
	go ms.syncLoop(ms.ctx)

	ms.logger.Info("Metrics syncer started successfully")
	return nil
}

// Stop gracefully stops the sync loop.
func (ms *MetricsSyncer) Stop() {
	ms.logger.Info("Stopping metrics syncer")

	if ms.cancel != nil {
		ms.cancel()
	}

	// Stop rate limiter
	if ms.rateLimiter != nil {
		ms.rateLimiter.Stop()
	}

	// Wait for sync loop to stop (with timeout)
	select {
	case <-ms.stopped:
		ms.logger.Info("Metrics syncer stopped")
	case <-time.After(5 * time.Second):
		ms.logger.Warn("Metrics syncer stop timeout")
	}
}

// syncLoop runs periodic sync on ticker interval.
func (ms *MetricsSyncer) syncLoop(ctx context.Context) {
	defer close(ms.stopped)

	ticker := time.NewTicker(ms.config.SyncInterval)
	defer ticker.Stop()

	ms.logger.Debug("Metrics sync loop started (interval: %s)", ms.config.SyncInterval)

	for {
		select {
		case <-ctx.Done():
			ms.logger.Debug("Metrics sync loop stopped (context cancelled)")
			return

		case <-ticker.C:
			ms.logger.Debug("Periodic metrics sync triggered")
			if err := ms.syncAll(ctx); err != nil {
				ms.logger.Warn("Periodic metrics sync failed: %v", err)
				ms.setLastError(err)
			}
		}
	}
}

// syncAll performs the full sync: fetch metrics, match against curated, upsert anchors.
func (ms *MetricsSyncer) syncAll(ctx context.Context) error {
	startTime := time.Now()
	ms.logger.Info("Starting metrics sync")

	// Set inProgress flag
	ms.mu.Lock()
	ms.inProgress = true
	ms.mu.Unlock()

	defer func() {
		ms.mu.Lock()
		ms.inProgress = false
		ms.mu.Unlock()
	}()

	// Step 1: Fetch all metric names from Prometheus
	ms.logger.Info("Fetching metric names from Prometheus datasource")
	grafanaMetrics, err := ms.client.ListMetricNames(ctx, ms.config.DatasourceUID)
	if err != nil {
		return fmt.Errorf("fetch metric names: %w", err)
	}
	ms.logger.Info("Fetched %d metric names from Prometheus", len(grafanaMetrics))

	if len(grafanaMetrics) == 0 {
		ms.logger.Warn("No metrics found in Prometheus - nothing to sync")
		ms.updateSyncStatus(0, 0, 0, 0, nil)
		return nil
	}

	// Step 2: Match against curated metrics
	ms.logger.Info("Matching metrics against curated definitions")
	matches := MatchMetricsToCurated(grafanaMetrics)
	stats := ComputeMatchStats(grafanaMetrics, matches)
	ms.logger.Info("Matched %d metrics (%d exact, %d suffix) out of %d total",
		stats.TotalMatched, stats.ExactMatches, stats.SuffixMatches, stats.TotalGrafanaMetrics)

	if len(matches) == 0 {
		ms.logger.Info("No metrics matched curated definitions - nothing to upsert")
		ms.updateSyncStatus(len(grafanaMetrics), 0, 0, 0, nil)
		return nil
	}

	// Step 3: Upsert SignalAnchors to graph
	ms.logger.Info("Upserting %d SignalAnchors to graph", len(matches))
	createdCount, updatedCount, err := ms.upsertAnchors(ctx, matches)
	if err != nil {
		return fmt.Errorf("upsert anchors: %w", err)
	}

	duration := time.Since(startTime)
	ms.logger.Info("Metrics sync complete: %d matched, %d created, %d updated (duration: %s)",
		len(matches), createdCount, updatedCount, duration)

	ms.updateSyncStatus(len(grafanaMetrics), len(matches), createdCount, updatedCount, nil)
	return nil
}

// upsertAnchors creates or updates SignalAnchors in the graph for matched metrics.
func (ms *MetricsSyncer) upsertAnchors(ctx context.Context, matches []MatchResult) (created, updated int, err error) {
	now := time.Now().Unix()
	expiresAt := time.Now().Add(7 * 24 * time.Hour).Unix() // 7-day TTL

	for _, match := range matches {
		// Rate limit before graph operation (for future multi-query scenarios)
		select {
		case <-ctx.Done():
			return created, updated, ctx.Err()
		case <-ms.rateLimiter.C:
			// Rate limit passed
		}

		wasCreated, err := ms.upsertSingleAnchor(ctx, match, now, expiresAt)
		if err != nil {
			ms.logger.Debug("Failed to upsert anchor for %s: %v", match.GrafanaMetric, err)
			continue
		}

		if wasCreated {
			created++
		} else {
			updated++
		}
	}

	return created, updated, nil
}

// upsertSingleAnchor creates or updates a single SignalAnchor.
// Returns true if a new anchor was created, false if existing was updated.
func (ms *MetricsSyncer) upsertSingleAnchor(ctx context.Context, match MatchResult, now, expiresAt int64) (bool, error) {
	// Convert signal role from curated metric
	role := string(match.CuratedMetric.ToSignalRole())

	// MERGE on composite key: (metric_name, workload_namespace, workload_name)
	// Global anchors use empty strings for workload fields
	query := `
		MERGE (s:SignalAnchor {
			metric_name: $metricName,
			workload_namespace: $workloadNamespace,
			workload_name: $workloadName
		})
		ON CREATE SET
			s.first_seen = $now,
			s.role = $role,
			s.confidence = $confidence,
			s.quality_score = $qualityScore,
			s.source_provider = $sourceProvider,
			s.source_ref = "curated-sync",
			s.curated_match_type = $matchType,
			s.last_seen = $now,
			s.expires_at = $expiresAt
		ON MATCH SET
			s.role = CASE WHEN $qualityScore > coalesce(s.quality_score, 0) THEN $role ELSE s.role END,
			s.confidence = CASE WHEN $qualityScore > coalesce(s.quality_score, 0) THEN $confidence ELSE s.confidence END,
			s.quality_score = CASE WHEN $qualityScore > coalesce(s.quality_score, 0) THEN $qualityScore ELSE s.quality_score END,
			s.curated_match_type = CASE
				WHEN s.source_ref = "curated-sync" THEN coalesce(s.curated_match_type, $matchType)
				ELSE s.curated_match_type
			END,
			s.last_seen = $now,
			s.expires_at = $expiresAt
		RETURN s.first_seen = $now AS was_created
	`

	params := map[string]interface{}{
		"metricName":        match.GrafanaMetric,
		"workloadNamespace": "", // Global anchor
		"workloadName":      "", // Global anchor
		"role":              role,
		"confidence":        match.CuratedMetric.Confidence,
		"qualityScore":      match.CuratedMetric.Importance,
		"sourceProvider":    ms.integrationName,
		"matchType":         match.MatchType,
		"now":               now,
		"expiresAt":         expiresAt,
	}

	result, err := ms.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query:      query,
		Parameters: params,
	})
	if err != nil {
		return false, fmt.Errorf("execute upsert query: %w", err)
	}

	// Check if a node was created using stats
	// NodesCreated > 0 means new anchor was created
	wasCreated := result.Stats.NodesCreated > 0

	// Invoke callbacks for new anchors
	if wasCreated && len(ms.callbacks) > 0 {
		for _, cb := range ms.callbacks {
			if err := cb.OnSignalAnchorCreated(ctx, match.GrafanaMetric, "", ""); err != nil {
				ms.logger.Debug("Callback failed for anchor %s: %v", match.GrafanaMetric, err)
				// Continue with other callbacks - don't fail the upsert
			}
		}
	}

	return wasCreated, nil
}

// datasourceDisplay returns a display string for the datasource config.
func (ms *MetricsSyncer) datasourceDisplay() string {
	if ms.config.DatasourceUID == "" {
		return "default"
	}
	return ms.config.DatasourceUID
}

// updateSyncStatus updates the thread-safe sync status.
func (ms *MetricsSyncer) updateSyncStatus(totalMetrics, matchedCount, createdCount, updatedCount int, err error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.lastSyncTime = time.Now()
	ms.totalMetrics = totalMetrics
	ms.matchedCount = matchedCount
	ms.createdCount = createdCount
	ms.updatedCount = updatedCount
	if err == nil {
		ms.lastError = nil
	}
}

// setLastError updates the last error (thread-safe).
func (ms *MetricsSyncer) setLastError(err error) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.lastError = err
}

// Status returns the current sync status.
func (ms *MetricsSyncer) Status() MetricsSyncerStatus {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	var lastErrorStr string
	if ms.lastError != nil {
		lastErrorStr = ms.lastError.Error()
	}

	return MetricsSyncerStatus{
		LastSyncTime: ms.lastSyncTime,
		TotalMetrics: ms.totalMetrics,
		MatchedCount: ms.matchedCount,
		CreatedCount: ms.createdCount,
		UpdatedCount: ms.updatedCount,
		LastError:    lastErrorStr,
		InProgress:   ms.inProgress,
	}
}

// MetricsSyncerStatus holds the current status of the syncer.
type MetricsSyncerStatus struct {
	LastSyncTime time.Time
	TotalMetrics int
	MatchedCount int
	CreatedCount int
	UpdatedCount int
	LastError    string
	InProgress   bool
}
