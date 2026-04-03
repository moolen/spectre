package namespacegraph

import (
	"context"
	"time"
)

// refreshLoop runs in the background for periodic maintenance tasks.
func (c *Cache) refreshLoop(ctx context.Context) {
	defer c.wg.Done()

	c.syncWithMetadata(ctx)

	syncPeriod := c.config.PeriodicSyncPeriod
	if syncPeriod <= 0 {
		syncPeriod = c.config.RefreshTTL
	}

	ticker := time.NewTicker(syncPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.syncWithMetadata(ctx)
			c.refreshExpired(ctx)
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// syncWithMetadata synchronizes the cache with the metadata cache.
func (c *Cache) syncWithMetadata(ctx context.Context) {
	if c.metadataCache == nil {
		return
	}

	metadata, err := c.metadataCache.Get()
	if err != nil {
		c.logger.Debug("Failed to get metadata for namespace sync: %v", err)
		return
	}
	if len(metadata.Namespaces) == 0 {
		c.logger.Debug("No namespaces found in metadata cache")
		return
	}

	currentNamespaces := make(map[string]bool)
	for _, ns := range metadata.Namespaces {
		currentNamespaces[ns] = true
	}

	c.evictDeletedNamespaces(currentNamespaces)

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
		if c.shouldStop(ctx) {
			return
		}
		c.preWarmNamespace(ctx, ns)
	}
}

func (c *Cache) evictDeletedNamespaces(currentNamespaces map[string]bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for ns := range c.cache {
		if currentNamespaces[ns] {
			continue
		}

		c.usedMemory -= c.cache[ns].Size
		delete(c.cache, ns)
		c.logger.Info("Evicted namespace graph cache: namespace=%s (namespace deleted)", ns)
	}
}

// preWarmNamespace computes and caches the graph for a single namespace.
func (c *Cache) preWarmNamespace(ctx context.Context, namespace string) {
	warmCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	result, err := c.analyzer.Analyze(warmCtx, c.cacheAnalyzeInput(namespace))
	cancel()
	if err != nil {
		c.logger.Warn("Failed to pre-warm namespace graph cache for %s: %v", namespace, err)
		return
	}

	c.set(namespace, result)
	c.logger.Info("Pre-warmed namespace graph cache: namespace=%s, nodes=%d, edges=%d",
		namespace, result.Metadata.NodeCount, result.Metadata.EdgeCount)
}

// refreshExpired refreshes all expired cache entries.
func (c *Cache) refreshExpired(ctx context.Context) {
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
		if c.shouldStop(ctx) {
			return
		}

		refreshCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		result, err := c.analyzer.Analyze(refreshCtx, c.cacheAnalyzeInput(ns))
		cancel()
		if err != nil {
			c.logger.Warn("Failed to refresh namespace graph cache for %s: %v", ns, err)
			continue
		}

		c.set(ns, result)
		c.logger.Debug("Refreshed namespace graph cache: namespace=%s", ns)
	}
}

func (c *Cache) cacheAnalyzeInput(namespace string) AnalyzeInput {
	return AnalyzeInput{
		Namespace: namespace,
		Timestamp: time.Now().UnixNano(),
		MaxDepth:  DefaultMaxDepth,
		Limit:     MaxLimit,
	}
}

func (c *Cache) shouldStop(ctx context.Context) bool {
	select {
	case <-c.stopCh:
		return true
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
