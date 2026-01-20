package sync

import (
	"context"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// CacheInvalidator interface for caches that support event-driven invalidation
type CacheInvalidator interface {
	// InvalidateNamespaces marks the given namespaces as stale
	InvalidateNamespaces(namespaces []string)

	// InvalidateAll marks all cached entries as stale
	InvalidateAll()
}

// NamespaceChangeDetectorConfig contains configuration for the change detector
type NamespaceChangeDetectorConfig struct {
	// DebounceWindow batches invalidations within this time window (default 5s)
	DebounceWindow time.Duration

	// FlushInterval is how often to flush pending invalidations (default 1s)
	FlushInterval time.Duration
}

// DefaultNamespaceChangeDetectorConfig returns default configuration
func DefaultNamespaceChangeDetectorConfig() NamespaceChangeDetectorConfig {
	return NamespaceChangeDetectorConfig{
		DebounceWindow: 5 * time.Second,
		FlushInterval:  1 * time.Second,
	}
}

// NamespaceChangeDetector watches events and notifies caches of affected namespaces
// It debounces rapid invalidations to avoid excessive cache churn
type NamespaceChangeDetector struct {
	mu          sync.Mutex
	subscribers []CacheInvalidator

	// Debounce state: namespaces waiting to be invalidated
	pendingNamespaces map[string]time.Time // namespace -> first dirty time
	pendingAll        bool                 // true if InvalidateAll is pending

	// Graph client for cluster-scoped resource queries
	graphClient graph.Client

	config NamespaceChangeDetectorConfig
	logger *logging.Logger

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewNamespaceChangeDetector creates a new change detector
func NewNamespaceChangeDetector(config NamespaceChangeDetectorConfig, graphClient graph.Client, logger *logging.Logger) *NamespaceChangeDetector {
	if config.DebounceWindow <= 0 {
		config.DebounceWindow = 5 * time.Second
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = 1 * time.Second
	}

	return &NamespaceChangeDetector{
		subscribers:       make([]CacheInvalidator, 0),
		pendingNamespaces: make(map[string]time.Time),
		graphClient:       graphClient,
		config:            config,
		logger:            logger,
		stopCh:            make(chan struct{}),
	}
}

// Subscribe registers a CacheInvalidator to receive invalidation notifications
func (d *NamespaceChangeDetector) Subscribe(invalidator CacheInvalidator) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.subscribers = append(d.subscribers, invalidator)
	d.logger.Info("Registered cache invalidator subscriber (total: %d)", len(d.subscribers))
}

// Start begins the background flush loop
func (d *NamespaceChangeDetector) Start(ctx context.Context) {
	d.wg.Add(1)
	go d.flushLoop(ctx)
	d.logger.Info("NamespaceChangeDetector started with debounce window: %v", d.config.DebounceWindow)
}

// Stop gracefully stops the change detector
func (d *NamespaceChangeDetector) Stop() {
	close(d.stopCh)
	d.wg.Wait()
	d.logger.Info("NamespaceChangeDetector stopped")
}

// OnEventBatch is called after the pipeline processes a batch of events
// It extracts affected namespaces and marks them for invalidation
func (d *NamespaceChangeDetector) OnEventBatch(ctx context.Context, events []models.Event) {
	if len(events) == 0 {
		return
	}

	affectedNamespaces := d.extractAffectedNamespaces(ctx, events)
	if len(affectedNamespaces) == 0 {
		return
	}

	d.markDirty(affectedNamespaces)
}

// extractAffectedNamespaces extracts namespaces affected by the events
func (d *NamespaceChangeDetector) extractAffectedNamespaces(ctx context.Context, events []models.Event) []string {
	namespaces := make(map[string]bool)

	for _, event := range events {
		ns := event.Resource.Namespace
		if ns != "" {
			// Namespaced resource - mark its namespace dirty
			namespaces[ns] = true
		} else {
			// Cluster-scoped resource - query graph for related namespaces
			relatedNs := d.findRelatedNamespaces(ctx, event)
			for _, rns := range relatedNs {
				namespaces[rns] = true
			}
		}
	}

	result := make([]string, 0, len(namespaces))
	for ns := range namespaces {
		result = append(result, ns)
	}
	return result
}

// findRelatedNamespaces queries the graph to find namespaces related to a cluster-scoped resource
func (d *NamespaceChangeDetector) findRelatedNamespaces(ctx context.Context, event models.Event) []string {
	if d.graphClient == nil {
		return nil
	}

	kind := event.Resource.Kind
	uid := event.Resource.UID

	if uid == "" {
		return nil
	}

	var query string
	switch kind {
	case "Node":
		// Find namespaces with Pods scheduled on this Node
		query = `
			MATCH (p:ResourceIdentity)-[:SCHEDULED_ON]->(n:ResourceIdentity {uid: $uid})
			WHERE NOT p.deleted AND p.namespace <> ''
			RETURN DISTINCT p.namespace as namespace
			LIMIT 100
		`
	case "ClusterRole":
		// Find namespaces with RoleBindings referencing this ClusterRole
		query = `
			MATCH (rb:ResourceIdentity)-[:BINDS_ROLE]->(cr:ResourceIdentity {uid: $uid})
			WHERE NOT rb.deleted AND rb.namespace <> ''
			RETURN DISTINCT rb.namespace as namespace
			LIMIT 100
		`
	case "ClusterRoleBinding":
		// ClusterRoleBindings can affect any namespace - but we query for subjects
		query = `
			MATCH (crb:ResourceIdentity {uid: $uid})-[:GRANTS_TO]->(subj:ResourceIdentity)
			WHERE NOT subj.deleted AND subj.namespace <> ''
			RETURN DISTINCT subj.namespace as namespace
			LIMIT 100
		`
	case "PersistentVolume":
		// Find namespace of bound PVC
		query = `
			MATCH (pvc:ResourceIdentity)-[:BOUND_TO]->(pv:ResourceIdentity {uid: $uid})
			WHERE NOT pvc.deleted AND pvc.namespace <> ''
			RETURN DISTINCT pvc.namespace as namespace
			LIMIT 100
		`
	case "StorageClass", "IngressClass", "PriorityClass":
		// These are referenced by resources in many namespaces
		// For now, we skip these as they rarely change and would cause broad invalidation
		d.logger.Debug("Skipping cluster-scoped resource %s/%s - changes rarely affect namespace graphs", kind, event.Resource.Name)
		return nil
	default:
		// Unknown cluster-scoped resource - log and skip
		d.logger.Debug("Unknown cluster-scoped resource kind: %s, skipping invalidation", kind)
		return nil
	}

	// Execute the query with a short timeout
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := d.graphClient.ExecuteQuery(queryCtx, graph.GraphQuery{
		Query: query,
		Parameters: map[string]interface{}{
			"uid": uid,
		},
		Timeout: 5000, // 5 seconds
	})

	if err != nil {
		d.logger.Debug("Failed to query related namespaces for %s/%s: %v", kind, event.Resource.Name, err)
		return nil
	}

	namespaces := make([]string, 0, len(result.Rows))
	for _, row := range result.Rows {
		if len(row) > 0 {
			if ns, ok := row[0].(string); ok && ns != "" {
				namespaces = append(namespaces, ns)
			}
		}
	}

	if len(namespaces) > 0 {
		d.logger.Debug("Found %d related namespaces for cluster-scoped %s/%s", len(namespaces), kind, event.Resource.Name)
	}

	return namespaces
}

// markDirty adds namespaces to the pending invalidation set
func (d *NamespaceChangeDetector) markDirty(namespaces []string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	for _, ns := range namespaces {
		if _, exists := d.pendingNamespaces[ns]; !exists {
			d.pendingNamespaces[ns] = now
		}
	}
}

// flushLoop periodically flushes pending invalidations to subscribers
func (d *NamespaceChangeDetector) flushLoop(ctx context.Context) {
	defer d.wg.Done()

	ticker := time.NewTicker(d.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.flushPending()

		case <-d.stopCh:
			// Final flush before stopping
			d.flushPending()
			return

		case <-ctx.Done():
			return
		}
	}
}

// flushPending sends pending invalidations to all subscribers
func (d *NamespaceChangeDetector) flushPending() {
	d.mu.Lock()

	// Check if there's anything to flush
	if len(d.pendingNamespaces) == 0 && !d.pendingAll {
		d.mu.Unlock()
		return
	}

	// Get namespaces that have been pending long enough (debounce)
	now := time.Now()
	toInvalidate := make([]string, 0)
	for ns, firstDirty := range d.pendingNamespaces {
		if now.Sub(firstDirty) >= d.config.DebounceWindow {
			toInvalidate = append(toInvalidate, ns)
		}
	}

	// Remove flushed namespaces from pending
	for _, ns := range toInvalidate {
		delete(d.pendingNamespaces, ns)
	}

	shouldInvalidateAll := d.pendingAll
	d.pendingAll = false

	// Copy subscribers to avoid holding lock during notification
	subscribers := make([]CacheInvalidator, len(d.subscribers))
	copy(subscribers, d.subscribers)

	d.mu.Unlock()

	// Notify subscribers
	if shouldInvalidateAll {
		d.logger.Info("Flushing InvalidateAll to %d subscribers", len(subscribers))
		for _, sub := range subscribers {
			sub.InvalidateAll()
		}
	} else if len(toInvalidate) > 0 {
		d.logger.Debug("Flushing %d namespace invalidations to %d subscribers", len(toInvalidate), len(subscribers))
		for _, sub := range subscribers {
			sub.InvalidateNamespaces(toInvalidate)
		}
	}
}

// ForceInvalidateAll immediately invalidates all caches (bypasses debounce)
// Used for testing or administrative purposes
func (d *NamespaceChangeDetector) ForceInvalidateAll() {
	d.mu.Lock()
	d.pendingAll = true
	d.mu.Unlock()

	// Immediately flush
	d.flushPending()
}
