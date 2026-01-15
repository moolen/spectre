package namespacegraph

import (
	"context"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// MetadataProvider interface for getting namespace list from metadata cache
type MetadataProvider interface {
	Get() (*models.MetadataResponse, error)
}

// CacheInvalidator interface for event-driven cache invalidation
// Implemented by Cache to allow the NamespaceChangeDetector to mark namespaces as stale
type CacheInvalidator interface {
	// InvalidateNamespaces marks the given namespaces as stale
	// The cache will recompute these on next request
	InvalidateNamespaces(namespaces []string)

	// InvalidateAll marks all cached entries as stale
	// Used for cluster-scoped resource changes that may affect all namespaces
	InvalidateAll()
}

// CacheConfig contains configuration for the namespace graph cache
type CacheConfig struct {
	// RefreshTTL is how long cached entries are valid before periodic refresh kicks in
	// With event-driven invalidation, this can be much longer (default 5 minutes)
	// Event-driven invalidation will mark namespaces dirty when changes occur
	RefreshTTL time.Duration

	// MaxMemoryMB is the maximum memory to use for cache (default 256MB)
	MaxMemoryMB int64

	// PeriodicSyncPeriod is how often to sync with metadata for namespace discovery
	// This is independent of RefreshTTL and handles new/deleted namespace detection
	// Default: 5 minutes (same as RefreshTTL)
	PeriodicSyncPeriod time.Duration
}

// DefaultCacheConfig returns the default cache configuration
// With event-driven invalidation enabled, the default TTL is much longer
// since cache freshness is maintained by the NamespaceChangeDetector
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		RefreshTTL:         5 * time.Minute, // Was 20s - now event-driven handles freshness
		MaxMemoryMB:        256,
		PeriodicSyncPeriod: 5 * time.Minute,
	}
}

// CachedEntry holds a cached namespace graph response
type CachedEntry struct {
	Response  *NamespaceGraphResponse
	Size      int64
	ExpiresAt time.Time
	UpdatedAt time.Time
}

// Cache provides fast in-memory access to namespace graph responses
// It caches one "view" per namespace and refreshes in the background
// Implements CacheInvalidator for event-driven invalidation
type Cache struct {
	mu         sync.RWMutex
	cache      map[string]*CachedEntry
	dirtySet   map[string]bool // Namespaces marked for recomputation by event-driven invalidation
	usedMemory int64

	config        CacheConfig
	analyzer      *Analyzer
	metadataCache MetadataProvider // For discovering namespaces to pre-warm
	logger        *logging.Logger

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewCache creates a new namespace graph cache
// metadataCache is optional - if provided, enables automatic pre-warming and namespace sync
func NewCache(config CacheConfig, analyzer *Analyzer, metadataCache MetadataProvider, logger *logging.Logger) *Cache {
	if config.RefreshTTL <= 0 {
		config.RefreshTTL = 20 * time.Second
	}
	if config.MaxMemoryMB <= 0 {
		config.MaxMemoryMB = 256
	}

	return &Cache{
		cache:         make(map[string]*CachedEntry),
		dirtySet:      make(map[string]bool),
		config:        config,
		analyzer:      analyzer,
		metadataCache: metadataCache,
		logger:        logger,
		stopCh:        make(chan struct{}),
	}
}

// Start begins the background refresh loop
func (c *Cache) Start(ctx context.Context) error {
	c.logger.Info("Starting namespace graph cache with refresh TTL: %v, max memory: %dMB",
		c.config.RefreshTTL, c.config.MaxMemoryMB)

	c.wg.Add(1)
	go c.refreshLoop(ctx)

	return nil
}

// Stop gracefully stops the background refresh loop
func (c *Cache) Stop() {
	c.logger.Info("Stopping namespace graph cache")
	close(c.stopCh)
	c.wg.Wait()
	c.logger.Info("Namespace graph cache stopped")
}

// Analyze returns a cached response if available, otherwise computes and caches it
// This is the main entry point for the handler
func (c *Cache) Analyze(ctx context.Context, input AnalyzeInput) (*NamespaceGraphResponse, error) {
	// Check if namespace is dirty (marked for recomputation by event-driven invalidation)
	c.mu.RLock()
	isDirty := c.dirtySet[input.Namespace]
	c.mu.RUnlock()

	// If dirty, recompute even if cache entry exists
	if isDirty {
		c.logger.Debug("Cache DIRTY: namespace=%s, recomputing due to event-driven invalidation", input.Namespace)
		return c.recompute(ctx, input)
	}

	// Try to get from cache
	if entry := c.getIfValid(input.Namespace); entry != nil {
		c.logger.Debug("Cache HIT: namespace=%s, age=%v",
			input.Namespace, time.Since(entry.UpdatedAt))

		// Return cached response with updated metadata
		// We create a shallow copy to avoid mutation
		response := *entry.Response
		response.Metadata.Cached = true
		response.Metadata.CacheAge = time.Since(entry.UpdatedAt).Milliseconds()
		return &response, nil
	}

	c.logger.Debug("Cache MISS: namespace=%s, computing...", input.Namespace)

	// Cache miss - compute the result
	return c.recompute(ctx, input)
}

// recompute computes the namespace graph and caches the result
func (c *Cache) recompute(ctx context.Context, input AnalyzeInput) (*NamespaceGraphResponse, error) {
	result, err := c.analyzer.Analyze(ctx, input)
	if err != nil {
		return nil, err
	}

	// Cache the result and clear dirty flag
	c.mu.Lock()
	delete(c.dirtySet, input.Namespace)
	c.mu.Unlock()

	c.set(input.Namespace, result)

	return result, nil
}

// getIfValid returns the cached entry if it exists and hasn't expired
func (c *Cache) getIfValid(namespace string) *CachedEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.cache[namespace]
	if !ok {
		return nil
	}

	if time.Now().After(entry.ExpiresAt) {
		return nil // Expired
	}

	return entry
}

// set stores a response in the cache
func (c *Cache) set(namespace string, response *NamespaceGraphResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Estimate size
	size := estimateResponseSize(response)

	// Check if we need to evict entries to make room
	maxBytes := c.config.MaxMemoryMB * 1024 * 1024
	for c.usedMemory+size > maxBytes && len(c.cache) > 0 {
		c.evictOldestLocked()
	}

	// Remove old entry size if updating existing
	if old, ok := c.cache[namespace]; ok {
		c.usedMemory -= old.Size
	}

	now := time.Now()
	c.cache[namespace] = &CachedEntry{
		Response:  response,
		Size:      size,
		ExpiresAt: now.Add(c.config.RefreshTTL),
		UpdatedAt: now,
	}
	c.usedMemory += size

	c.logger.Debug("Cached namespace graph: namespace=%s, size=%dKB, total_memory=%dMB",
		namespace, size/1024, c.usedMemory/(1024*1024))
}

// InvalidateNamespaces marks the given namespaces as stale
// They will be recomputed on next access
// Implements CacheInvalidator interface
func (c *Cache) InvalidateNamespaces(namespaces []string) {
	if len(namespaces) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, ns := range namespaces {
		c.dirtySet[ns] = true
	}

	c.logger.Debug("Invalidated %d namespaces via event-driven update: %v", len(namespaces), namespaces)
}

// InvalidateAll marks all cached entries as stale
// Used for cluster-scoped resource changes that may affect all namespaces
// Implements CacheInvalidator interface
func (c *Cache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Mark all cached namespaces as dirty
	for ns := range c.cache {
		c.dirtySet[ns] = true
	}

	c.logger.Info("Invalidated all %d namespace graph caches", len(c.cache))
}

// evictOldestLocked removes the oldest cache entry (caller must hold lock)
func (c *Cache) evictOldestLocked() {
	var oldest string
	var oldestTime time.Time

	for ns, entry := range c.cache {
		if oldestTime.IsZero() || entry.UpdatedAt.Before(oldestTime) {
			oldest = ns
			oldestTime = entry.UpdatedAt
		}
	}

	if oldest != "" {
		entry := c.cache[oldest]
		c.usedMemory -= entry.Size
		delete(c.cache, oldest)
		c.logger.Debug("Evicted cache entry: namespace=%s, age=%v",
			oldest, time.Since(oldestTime))
	}
}

// refreshLoop runs in the background for periodic maintenance tasks
// With event-driven invalidation enabled, this loop handles:
// 1. Namespace discovery (new namespaces) and cleanup (deleted namespaces)
// 2. Safety-net refresh of entries that exceed RefreshTTL (shouldn't happen often with event-driven)
func (c *Cache) refreshLoop(ctx context.Context) {
	defer c.wg.Done()

	// Initial pre-warm on startup (if metadata cache is available)
	c.syncWithMetadata(ctx)

	// Use PeriodicSyncPeriod for the ticker (default 5 min)
	// This is much less aggressive than before - event-driven invalidation handles freshness
	syncPeriod := c.config.PeriodicSyncPeriod
	if syncPeriod <= 0 {
		syncPeriod = c.config.RefreshTTL // Fallback to RefreshTTL if not set
	}
	ticker := time.NewTicker(syncPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Sync with metadata (discovers new/deleted namespaces)
			c.syncWithMetadata(ctx)
			// Safety-net refresh of expired entries
			// With event-driven invalidation, this should rarely trigger
			c.refreshExpired(ctx)

		case <-c.stopCh:
			return

		case <-ctx.Done():
			return
		}
	}
}

// syncWithMetadata synchronizes the cache with the metadata cache
// It pre-warms new namespaces and evicts deleted namespaces
func (c *Cache) syncWithMetadata(ctx context.Context) {
	if c.metadataCache == nil {
		return
	}

	// 1. Get current namespaces from metadata cache
	metadata, err := c.metadataCache.Get()
	if err != nil {
		c.logger.Debug("Failed to get metadata for namespace sync: %v", err)
		return
	}

	if len(metadata.Namespaces) == 0 {
		c.logger.Debug("No namespaces found in metadata cache")
		return
	}

	// 2. Build set of current namespaces
	currentNamespaces := make(map[string]bool)
	for _, ns := range metadata.Namespaces {
		currentNamespaces[ns] = true
	}

	// 3. Find and evict deleted namespaces
	c.mu.Lock()
	for ns := range c.cache {
		if !currentNamespaces[ns] {
			// Namespace was deleted - evict from cache
			c.usedMemory -= c.cache[ns].Size
			delete(c.cache, ns)
			c.logger.Info("Evicted namespace graph cache: namespace=%s (namespace deleted)", ns)
		}
	}
	c.mu.Unlock()

	// 4. Pre-warm new namespaces (not in cache)
	var toPreWarm []string
	for ns := range currentNamespaces {
		if c.getIfValid(ns) == nil {
			toPreWarm = append(toPreWarm, ns)
		}
	}

	if len(toPreWarm) == 0 {
		return
	}

	c.logger.Info("Pre-warming %d namespace graph caches", len(toPreWarm))

	for _, ns := range toPreWarm {
		// Check if we should stop
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		default:
		}

		c.preWarmNamespace(ctx, ns)
	}
}

// preWarmNamespace computes and caches the graph for a single namespace
func (c *Cache) preWarmNamespace(ctx context.Context, namespace string) {
	input := AnalyzeInput{
		Namespace: namespace,
		Timestamp: time.Now().UnixNano(),
		MaxDepth:  DefaultMaxDepth,
		Limit:     MaxLimit, // Fetch full graph for cache
	}

	// Pre-warm with timeout
	warmCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	result, err := c.analyzer.Analyze(warmCtx, input)
	cancel()

	if err != nil {
		c.logger.Warn("Failed to pre-warm namespace graph cache for %s: %v", namespace, err)
		return
	}

	c.set(namespace, result)
	c.logger.Info("Pre-warmed namespace graph cache: namespace=%s, nodes=%d, edges=%d",
		namespace, result.Metadata.NodeCount, result.Metadata.EdgeCount)
}

// refreshExpired refreshes all expired cache entries
func (c *Cache) refreshExpired(ctx context.Context) {
	// Get list of namespaces to refresh
	c.mu.RLock()
	var toRefresh []string
	for ns, entry := range c.cache {
		if time.Now().After(entry.ExpiresAt) {
			toRefresh = append(toRefresh, ns)
		}
	}
	c.mu.RUnlock()

	if len(toRefresh) == 0 {
		return
	}

	c.logger.Debug("Refreshing %d expired namespace graph entries", len(toRefresh))

	for _, ns := range toRefresh {
		// Check if we should stop
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		default:
		}

		// Create input for refresh
		input := AnalyzeInput{
			Namespace: ns,
			Timestamp: time.Now().UnixNano(),
			MaxDepth:  DefaultMaxDepth,
			Limit:     MaxLimit, // Fetch full graph for cache
		}

		// Refresh with timeout
		refreshCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		result, err := c.analyzer.Analyze(refreshCtx, input)
		cancel()

		if err != nil {
			c.logger.Warn("Failed to refresh namespace graph cache for %s: %v", ns, err)
			continue
		}

		c.set(ns, result)
		c.logger.Debug("Refreshed namespace graph cache: namespace=%s", ns)
	}
}

// GetStats returns cache statistics
func (c *Cache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := CacheStats{
		EntryCount:     len(c.cache),
		UsedMemoryMB:   c.usedMemory / (1024 * 1024),
		MaxMemoryMB:    c.config.MaxMemoryMB,
		RefreshTTLSecs: int64(c.config.RefreshTTL.Seconds()),
	}

	for ns, entry := range c.cache {
		stats.Entries = append(stats.Entries, CacheEntryStats{
			Namespace: ns,
			NodeCount: entry.Response.Metadata.NodeCount,
			EdgeCount: entry.Response.Metadata.EdgeCount,
			SizeKB:    entry.Size / 1024,
			Age:       time.Since(entry.UpdatedAt).Milliseconds(),
			ExpiresIn: time.Until(entry.ExpiresAt).Milliseconds(),
		})
	}

	return stats
}

// CacheStats contains cache statistics
type CacheStats struct {
	EntryCount     int               `json:"entryCount"`
	UsedMemoryMB   int64             `json:"usedMemoryMB"`
	MaxMemoryMB    int64             `json:"maxMemoryMB"`
	RefreshTTLSecs int64             `json:"refreshTTLSecs"`
	Entries        []CacheEntryStats `json:"entries,omitempty"`
}

// CacheEntryStats contains statistics for a single cache entry
type CacheEntryStats struct {
	Namespace string `json:"namespace"`
	NodeCount int    `json:"nodeCount"`
	EdgeCount int    `json:"edgeCount"`
	SizeKB    int64  `json:"sizeKB"`
	Age       int64  `json:"ageMs"`
	ExpiresIn int64  `json:"expiresInMs"`
}

// estimateResponseSize estimates the memory size of a response
func estimateResponseSize(response *NamespaceGraphResponse) int64 {
	if response == nil {
		return 0
	}

	// Estimate based on content
	// - Each node: ~500 bytes (UID, Kind, Name, Labels, etc.)
	// - Each edge: ~100 bytes (IDs and type)
	// - Each anomaly: ~200 bytes
	// - Base overhead: ~1KB

	size := int64(1024) // Base overhead
	size += int64(len(response.Graph.Nodes)) * 500
	size += int64(len(response.Graph.Edges)) * 100
	size += int64(len(response.Anomalies)) * 200
	size += int64(len(response.CausalPaths)) * 500

	return size
}
