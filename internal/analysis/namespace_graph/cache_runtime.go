package namespacegraph

import (
	"context"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

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

// Analyze returns a cached response if available, otherwise computes and caches it.
func (c *Cache) Analyze(ctx context.Context, input AnalyzeInput) (*NamespaceGraphResponse, error) {
	c.mu.RLock()
	isDirty := c.dirtySet[input.Namespace]
	c.mu.RUnlock()

	if isDirty {
		c.logger.Debug("Cache DIRTY: namespace=%s, recomputing due to event-driven invalidation", input.Namespace)
		return c.recompute(ctx, input)
	}

	if entry := c.getIfValid(input.Namespace); entry != nil {
		c.logger.Debug("Cache HIT: namespace=%s, age=%v",
			input.Namespace, time.Since(entry.UpdatedAt))

		response := *entry.Response
		response.Metadata.Cached = true
		response.Metadata.CacheAge = time.Since(entry.UpdatedAt).Milliseconds()
		return &response, nil
	}

	c.logger.Debug("Cache MISS: namespace=%s, computing...", input.Namespace)
	return c.recompute(ctx, input)
}

// recompute computes the namespace graph and caches the result.
func (c *Cache) recompute(ctx context.Context, input AnalyzeInput) (*NamespaceGraphResponse, error) {
	result, err := c.analyzer.Analyze(ctx, input)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	delete(c.dirtySet, input.Namespace)
	c.mu.Unlock()

	c.set(input.Namespace, result)
	return result, nil
}
