package namespacegraph

import (
	"context"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

// NewCache creates a new namespace graph cache
func NewCache(config CacheConfig, analyzer *Analyzer, logger *logging.Logger) *Cache {
	if config.RefreshTTL <= 0 {
		config.RefreshTTL = 20 * time.Second
	}
	if config.MaxMemoryMB <= 0 {
		config.MaxMemoryMB = 256
	}

	return &Cache{
		cache:    make(map[cacheKey]*CachedEntry),
		config:   config,
		analyzer: analyzer,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the background loop evicting expired entries
func (c *Cache) Start(ctx context.Context) error {
	c.logger.Info("Starting namespace graph cache with TTL: %v, max memory: %dMB",
		c.config.RefreshTTL, c.config.MaxMemoryMB)

	c.wg.Add(1)
	go c.evictionLoop(ctx)

	return nil
}

// Stop gracefully stops the background loop
func (c *Cache) Stop() {
	c.logger.Info("Stopping namespace graph cache")
	close(c.stopCh)
	c.wg.Wait()
	c.logger.Info("Namespace graph cache stopped")
}

// Analyze returns the cached response for this exact request if there is one, otherwise
// computes and caches it.
func (c *Cache) Analyze(ctx context.Context, input AnalyzeInput) (*NamespaceGraphResponse, error) {
	key := newCacheKey(input)
	if entry := c.getIfValid(key); entry != nil {
		c.logger.Debug("Cache HIT: namespace=%s, age=%v", input.Namespace, time.Since(entry.UpdatedAt))

		response := *entry.Response
		response.Metadata.Cached = true
		response.Metadata.CacheAge = time.Since(entry.UpdatedAt).Milliseconds()
		return &response, nil
	}

	c.logger.Debug("Cache MISS: namespace=%s, computing...", input.Namespace)
	result, err := c.analyzer.Analyze(ctx, input)
	if err != nil {
		return nil, err
	}
	c.set(key, result)
	return result, nil
}
