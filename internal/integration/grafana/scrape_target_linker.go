package grafana

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// ScrapeTargetLinkerConfig holds configuration for the scrape target linker.
type ScrapeTargetLinkerConfig struct {
	// SyncInterval is how often to refresh scrape target links.
	// Default: 5 minutes
	SyncInterval time.Duration

	// RateLimitInterval is the minimum time between graph operations.
	// Default: 100ms (10 ops/sec)
	RateLimitInterval time.Duration

	// StaleTTL is how long to keep stale links before garbage collection.
	// Default: 7 days (matches SignalAnchor TTL)
	StaleTTL time.Duration
}

// DefaultScrapeTargetLinkerConfig returns default configuration.
func DefaultScrapeTargetLinkerConfig() ScrapeTargetLinkerConfig {
	return ScrapeTargetLinkerConfig{
		SyncInterval:      5 * time.Minute,
		RateLimitInterval: 100 * time.Millisecond,
		StaleTTL:          7 * 24 * time.Hour,
	}
}

// ResourceIdentityRef holds a reference to a ResourceIdentity node.
type ResourceIdentityRef struct {
	UID       string
	Kind      string
	Name      string
	Namespace string
}

// ScrapeTargetLinkerStatus holds the current status of the linker.
type ScrapeTargetLinkerStatus struct {
	LastSyncTime   time.Time
	LinksCreated   int
	LinksConfirmed int
	LinksStale     int
	LinksDeleted   int
	LastError      string
	InProgress     bool
}

// ScrapeTargetLinker links SignalAnchors to K8s workloads using Prometheus scrape target metadata.
// It fetches scrape targets from Prometheus, resolves workloads via app labels or Pod→Owner traversal,
// and creates/updates MONITORS_WORKLOAD edges in the graph.
type ScrapeTargetLinker struct {
	prometheusClient *PrometheusClient
	graphClient      graph.Client
	integrationName  string
	logger           *logging.Logger
	config           ScrapeTargetLinkerConfig

	// Lifecycle
	ctx     context.Context
	cancel  context.CancelFunc
	stopped chan struct{}

	// Rate limiting
	rateLimiter *time.Ticker

	// Thread-safe status
	mu             sync.RWMutex
	lastSyncTime   time.Time
	linksCreated   int
	linksConfirmed int
	linksStale     int
	linksDeleted   int
	lastError      error
	inProgress     bool
}

// NewScrapeTargetLinker creates a new scrape target linker.
func NewScrapeTargetLinker(
	prometheusClient *PrometheusClient,
	graphClient graph.Client,
	integrationName string,
	logger *logging.Logger,
	config ScrapeTargetLinkerConfig,
) *ScrapeTargetLinker {
	return &ScrapeTargetLinker{
		prometheusClient: prometheusClient,
		graphClient:      graphClient,
		integrationName:  integrationName,
		logger:           logger,
		config:           config,
		rateLimiter:      time.NewTicker(config.RateLimitInterval),
		stopped:          make(chan struct{}),
	}
}

// Start begins the sync loop (initial sync + periodic sync).
func (l *ScrapeTargetLinker) Start(ctx context.Context) error {
	l.logger.Info("Starting scrape target linker (interval: %s)", l.config.SyncInterval)

	// Create cancellable context
	l.ctx, l.cancel = context.WithCancel(ctx)

	// Run initial sync (with graceful failure)
	if err := l.syncAll(l.ctx); err != nil {
		l.logger.Warn("Initial scrape target sync failed: %v (will retry on schedule)", err)
		l.setLastError(err)
	}

	// Start background sync loop
	go l.syncLoop(l.ctx)

	l.logger.Info("Scrape target linker started successfully")
	return nil
}

// Stop gracefully stops the sync loop.
func (l *ScrapeTargetLinker) Stop() {
	l.logger.Info("Stopping scrape target linker")

	if l.cancel != nil {
		l.cancel()
	}

	// Stop rate limiter
	if l.rateLimiter != nil {
		l.rateLimiter.Stop()
	}

	// Wait for sync loop to stop (with timeout)
	select {
	case <-l.stopped:
		l.logger.Info("Scrape target linker stopped")
	case <-time.After(5 * time.Second):
		l.logger.Warn("Scrape target linker stop timeout")
	}
}

// SyncNow triggers an immediate sync (for MCP cache bypass).
func (l *ScrapeTargetLinker) SyncNow(ctx context.Context) error {
	l.logger.Info("Manual scrape target sync triggered")
	return l.syncAll(ctx)
}

// Status returns the current linker status.
func (l *ScrapeTargetLinker) Status() ScrapeTargetLinkerStatus {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var lastErrorStr string
	if l.lastError != nil {
		lastErrorStr = l.lastError.Error()
	}

	return ScrapeTargetLinkerStatus{
		LastSyncTime:   l.lastSyncTime,
		LinksCreated:   l.linksCreated,
		LinksConfirmed: l.linksConfirmed,
		LinksStale:     l.linksStale,
		LinksDeleted:   l.linksDeleted,
		LastError:      lastErrorStr,
		InProgress:     l.inProgress,
	}
}

// OnSignalAnchorCreated implements the callback interface for event-driven linking.
// Called when a new SignalAnchor is created to attempt immediate linking.
func (l *ScrapeTargetLinker) OnSignalAnchorCreated(ctx context.Context, metricName, workloadNamespace, workloadName string) error {
	l.logger.Debug("Signal anchor created callback: metric=%s namespace=%s workload=%s",
		metricName, workloadNamespace, workloadName)

	// Attempt to link this specific anchor
	return l.linkSingleAnchor(ctx, metricName, workloadNamespace, workloadName)
}

// syncLoop runs periodic sync on ticker interval.
func (l *ScrapeTargetLinker) syncLoop(ctx context.Context) {
	defer close(l.stopped)

	ticker := time.NewTicker(l.config.SyncInterval)
	defer ticker.Stop()

	l.logger.Debug("Scrape target sync loop started (interval: %s)", l.config.SyncInterval)

	for {
		select {
		case <-ctx.Done():
			l.logger.Debug("Scrape target sync loop stopped (context cancelled)")
			return

		case <-ticker.C:
			l.logger.Debug("Periodic scrape target sync triggered")
			if err := l.syncAll(ctx); err != nil {
				l.logger.Warn("Periodic scrape target sync failed: %v", err)
				l.setLastError(err)
			}
		}
	}
}

// syncAll performs the full sync: fetch targets, resolve workloads, create/update links.
func (l *ScrapeTargetLinker) syncAll(ctx context.Context) error {
	startTime := time.Now()
	l.logger.Info("Starting scrape target sync")

	// Set inProgress flag
	l.mu.Lock()
	l.inProgress = true
	l.mu.Unlock()

	defer func() {
		l.mu.Lock()
		l.inProgress = false
		l.mu.Unlock()
	}()

	// Step 1: Fetch current targets
	targets, err := l.prometheusClient.GetTargets(ctx)
	if err != nil {
		return fmt.Errorf("fetch targets: %w", err)
	}
	l.logger.Info("Fetched %d healthy scrape targets from Prometheus", len(targets))

	if len(targets) == 0 {
		l.logger.Warn("No healthy scrape targets found - nothing to sync")
		l.updateSyncStatus(0, 0, 0, 0, nil)
		return nil
	}

	// Step 2: Build set of active (job, namespace, workloadUID) tuples
	activeLinks := make(map[string]bool)
	created, confirmed := 0, 0

	// Step 3: Create/update links for active targets
	for _, target := range targets {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-l.rateLimiter.C:
			// Rate limit passed
		}

		// Resolve workload for this target
		workload, confidence, err := l.resolveWorkload(ctx, target)
		if err != nil {
			l.logger.Debug("Failed to resolve workload for target %s: %v", target.ScrapePool, err)
			continue
		}
		if workload == nil {
			// No matching workload found
			continue
		}

		// Find matching SignalAnchors for this target's metrics
		namespace := target.Labels["namespace"]
		job := target.ScrapePool

		// Create link key for staleness tracking
		linkKey := fmt.Sprintf("%s/%s/%s", job, namespace, workload.UID)
		activeLinks[linkKey] = true

		// Create or update link
		wasCreated, err := l.createOrUpdateLink(ctx, namespace, workload, job, confidence)
		if err != nil {
			l.logger.Debug("Failed to create link for target %s: %v", target.ScrapePool, err)
			continue
		}

		if wasCreated {
			created++
		} else {
			confirmed++
		}
	}

	// Step 4: Mark stale: links not seen in this sync
	staleCount, err := l.markStaleLinks(ctx, activeLinks)
	if err != nil {
		l.logger.Warn("Failed to mark stale links: %v", err)
	}

	// Step 5: GC: delete links stale beyond TTL
	deletedCount, err := l.gcStaleLinks(ctx)
	if err != nil {
		l.logger.Warn("Failed to GC stale links: %v", err)
	}

	duration := time.Since(startTime)
	l.logger.Info("Scrape target sync complete: %d created, %d confirmed, %d stale, %d deleted (duration: %s)",
		created, confirmed, staleCount, deletedCount, duration)

	l.updateSyncStatus(created, confirmed, staleCount, deletedCount, nil)
	return nil
}

// resolveWorkload resolves a scrape target to a K8s workload (Deployment/StatefulSet/DaemonSet).
// Returns the workload reference and confidence score (1.0 for direct match, 0.8 for Pod→Owner fallback).
func (l *ScrapeTargetLinker) resolveWorkload(ctx context.Context, target ScrapeTarget) (*ResourceIdentityRef, float64, error) {
	namespace := target.Labels["namespace"]
	if namespace == "" {
		return nil, 0, nil // Can't resolve without namespace
	}

	// Strategy 1: Direct label match (confidence: 1.0)
	ri, err := l.resolveByAppLabel(ctx, namespace, target.Labels)
	if err == nil && ri != nil {
		return ri, 1.0, nil
	}

	// Strategy 2: Pod → Owner traversal (confidence: 0.8)
	podName := target.Labels["pod"]
	if podName != "" {
		ri, err := l.resolvePodOwner(ctx, namespace, podName)
		if err == nil && ri != nil {
			return ri, 0.8, nil
		}
	}

	return nil, 0, nil
}

// resolveByAppLabel attempts to find a workload by app label match.
func (l *ScrapeTargetLinker) resolveByAppLabel(ctx context.Context, namespace string, labels map[string]string) (*ResourceIdentityRef, error) {
	// Priority order for workload identification
	appLabels := []string{
		"app_kubernetes_io_name",     // app.kubernetes.io/name (sanitized by Prometheus)
		"app",                        // common shorthand
		"app_kubernetes_io_instance", // app.kubernetes.io/instance
	}

	for _, labelKey := range appLabels {
		appName, ok := labels[labelKey]
		if !ok || appName == "" {
			continue
		}

		// Query graph for matching Deployment/StatefulSet/DaemonSet
		ri, err := l.findWorkloadByLabel(ctx, namespace, labelKey, appName)
		if err == nil && ri != nil {
			return ri, nil
		}
	}

	return nil, nil
}

// findWorkloadByLabel queries the graph for a workload with matching labels.
func (l *ScrapeTargetLinker) findWorkloadByLabel(ctx context.Context, namespace, labelKey, labelValue string) (*ResourceIdentityRef, error) {
	// Map sanitized Prometheus label names back to K8s label names
	k8sLabelKey := labelKey
	switch labelKey {
	case "app_kubernetes_io_name":
		k8sLabelKey = "app.kubernetes.io/name"
	case "app_kubernetes_io_instance":
		k8sLabelKey = "app.kubernetes.io/instance"
	}

	// Note: FalkorDB quirk - use NOT r.deleted instead of r.deleted = false
	// Also use OR chain instead of IN for array comparison
	query := `
		MATCH (r:ResourceIdentity)
		WHERE r.namespace = $namespace
		  AND (r.kind = 'Deployment' OR r.kind = 'StatefulSet' OR r.kind = 'DaemonSet')
		  AND NOT r.deleted
		  AND r.labels[$labelKey] = $labelValue
		RETURN r.uid AS uid, r.kind AS kind, r.name AS name
		LIMIT 1
	`

	result, err := l.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"namespace":  namespace,
			"labelKey":   k8sLabelKey,
			"labelValue": labelValue,
		},
	})
	if err != nil {
		return nil, err
	}

	if len(result.Rows) == 0 {
		return nil, nil
	}

	row := result.Rows[0]
	return &ResourceIdentityRef{
		UID:       row[0].(string),
		Kind:      row[1].(string),
		Name:      row[2].(string),
		Namespace: namespace,
	}, nil
}

// resolvePodOwner finds the owning workload by traversing OWNS edges from Pod.
func (l *ScrapeTargetLinker) resolvePodOwner(ctx context.Context, namespace, podName string) (*ResourceIdentityRef, error) {
	// Find Pod, then traverse OWNS edge backward to find Deployment/StatefulSet/DaemonSet
	// The *1..2 handles ReplicaSet intermediate ownership (Deployment -> ReplicaSet -> Pod)
	// Note: FalkorDB quirk - use NOT deleted instead of deleted = false
	// Also use OR chain instead of IN for array comparison
	query := `
		MATCH (owner:ResourceIdentity)-[:OWNS*1..2]->(pod:ResourceIdentity)
		WHERE pod.kind = 'Pod'
		  AND pod.namespace = $namespace
		  AND pod.name = $podName
		  AND NOT pod.deleted
		  AND (owner.kind = 'Deployment' OR owner.kind = 'StatefulSet' OR owner.kind = 'DaemonSet')
		  AND NOT owner.deleted
		RETURN owner.uid AS uid, owner.kind AS kind, owner.name AS name
		LIMIT 1
	`

	result, err := l.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"namespace": namespace,
			"podName":   podName,
		},
	})
	if err != nil {
		return nil, err
	}

	if len(result.Rows) == 0 {
		return nil, nil
	}

	row := result.Rows[0]
	return &ResourceIdentityRef{
		UID:       row[0].(string),
		Kind:      row[1].(string),
		Name:      row[2].(string),
		Namespace: namespace,
	}, nil
}

// createOrUpdateLink creates or updates a MONITORS_WORKLOAD edge between SignalAnchors and workload.
// Returns true if a new link was created, false if existing was updated.
func (l *ScrapeTargetLinker) createOrUpdateLink(ctx context.Context, _ string, workload *ResourceIdentityRef, job string, confidence float64) (bool, error) {
	now := time.Now().UnixNano()

	// Link all global SignalAnchors (workload_namespace="") to the resolved workload
	// This connects curated metrics to their associated workloads
	// Note: FalkorDB requires size() = 0 for empty string comparison, not = ''
	query := `
		MATCH (s:SignalAnchor)
		WHERE size(s.workload_namespace) = 0
		  AND size(s.workload_name) = 0
		MATCH (r:ResourceIdentity {uid: $workloadUID})
		MERGE (s)-[m:MONITORS_WORKLOAD]->(r)
		ON CREATE SET
			m.first_linked = $now,
			m.last_confirmed = $now,
			m.stale = false,
			m.source = 'scrape_target',
			m.job = $job,
			m.confidence = $confidence
		ON MATCH SET
			m.last_confirmed = $now,
			m.stale = false,
			m.source = CASE WHEN m.source = 'promql_inference' THEN 'scrape_target' ELSE m.source END,
			m.confidence = CASE WHEN $confidence > m.confidence THEN $confidence ELSE m.confidence END
		RETURN m.first_linked = $now AS was_created
	`

	result, err := l.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"workloadUID": workload.UID,
			"now":         now,
			"job":         job,
			"confidence":  confidence,
		},
	})
	if err != nil {
		return false, fmt.Errorf("execute link query: %w", err)
	}

	// Check if new links were created
	return result.Stats.RelationshipsCreated > 0, nil
}

// linkSingleAnchor attempts to link a specific SignalAnchor to workloads.
// Called by the callback interface when a new anchor is created.
func (l *ScrapeTargetLinker) linkSingleAnchor(ctx context.Context, _, _, _ string) error {
	// For now, this triggers a full sync
	// Future optimization: only process targets relevant to this anchor
	return l.syncAll(ctx)
}

// markStaleLinks marks links not seen in this sync as stale.
func (l *ScrapeTargetLinker) markStaleLinks(ctx context.Context, activeLinks map[string]bool) (int, error) {
	if len(activeLinks) == 0 {
		return 0, nil
	}

	now := time.Now().UnixNano()

	// Build list of active keys
	activeKeys := make([]string, 0, len(activeLinks))
	for key := range activeLinks {
		activeKeys = append(activeKeys, key)
	}

	query := `
		MATCH (s:SignalAnchor)-[m:MONITORS_WORKLOAD]->(r:ResourceIdentity)
		WHERE m.source = 'scrape_target'
		  AND m.stale = false
		  AND NOT (m.job + '/' + coalesce(r.namespace, '') + '/' + r.uid) IN $activeKeys
		SET m.stale = true, m.stale_at = $now
		RETURN count(m) AS marked_count
	`

	result, err := l.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"activeKeys": activeKeys,
			"now":        now,
		},
	})
	if err != nil {
		return 0, fmt.Errorf("execute mark stale query: %w", err)
	}

	if len(result.Rows) > 0 && len(result.Rows[0]) > 0 {
		if count, ok := result.Rows[0][0].(int64); ok {
			return int(count), nil
		}
		if count, ok := result.Rows[0][0].(float64); ok {
			return int(count), nil
		}
	}

	return 0, nil
}

// gcStaleLinks deletes links that have been stale beyond the TTL.
func (l *ScrapeTargetLinker) gcStaleLinks(ctx context.Context) (int, error) {
	cutoff := time.Now().Add(-l.config.StaleTTL).UnixNano()

	query := `
		MATCH (s:SignalAnchor)-[m:MONITORS_WORKLOAD]->(r:ResourceIdentity)
		WHERE m.stale = true AND m.stale_at < $cutoff
		DELETE m
		RETURN count(m) AS deleted_count
	`

	result, err := l.graphClient.ExecuteQuery(ctx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"cutoff": cutoff,
		},
	})
	if err != nil {
		return 0, fmt.Errorf("execute GC stale query: %w", err)
	}

	// The deleted count is in stats, not rows
	return result.Stats.RelationshipsDeleted, nil
}

// updateSyncStatus updates the thread-safe sync status.
func (l *ScrapeTargetLinker) updateSyncStatus(created, confirmed, stale, deleted int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.lastSyncTime = time.Now()
	l.linksCreated = created
	l.linksConfirmed = confirmed
	l.linksStale = stale
	l.linksDeleted = deleted
	if err == nil {
		l.lastError = nil
	}
}

// setLastError updates the last error (thread-safe).
func (l *ScrapeTargetLinker) setLastError(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lastError = err
}
