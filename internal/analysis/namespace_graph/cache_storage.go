package namespacegraph

import "time"

// getIfValid returns the cached entry if it exists and hasn't expired.
func (c *Cache) getIfValid(key cacheKey) *CachedEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.cache[key]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return nil
	}

	return entry
}

// set stores a response in the cache.
func (c *Cache) set(key cacheKey, response *NamespaceGraphResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	size := estimateResponseSize(response)
	maxBytes := c.config.MaxMemoryMB * 1024 * 1024
	if old, ok := c.cache[key]; ok {
		c.usedMemory -= old.Size
		delete(c.cache, key)
	}
	for c.usedMemory+size > maxBytes && len(c.cache) > 0 {
		c.evictOldestLocked()
	}

	now := time.Now()
	c.cache[key] = &CachedEntry{
		Response:  response,
		Size:      size,
		ExpiresAt: now.Add(c.config.RefreshTTL),
		UpdatedAt: now,
	}
	c.usedMemory += size

	c.logger.Debug("Cached namespace graph: namespace=%s, size=%dKB, total_memory=%dMB",
		key.namespace, size/1024, c.usedMemory/(1024*1024))
}

// InvalidateNamespaces drops the cached responses of the given namespaces.
func (c *Cache) InvalidateNamespaces(namespaces []string) {
	if len(namespaces) == 0 {
		return
	}
	stale := make(map[string]bool, len(namespaces))
	for _, ns := range namespaces {
		stale[ns] = true
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	for key, entry := range c.cache {
		if stale[key.namespace] {
			c.usedMemory -= entry.Size
			delete(c.cache, key)
		}
	}

	c.logger.Debug("Invalidated %d namespaces via event-driven update: %v", len(namespaces), namespaces)
}

// InvalidateAll drops every cached response.
func (c *Cache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := len(c.cache)
	clear(c.cache)
	c.usedMemory = 0
	c.logger.Info("Invalidated all %d namespace graph cache entries", count)
}

// evictOldestLocked removes the oldest cache entry (caller must hold lock).
func (c *Cache) evictOldestLocked() {
	var oldest cacheKey
	var oldestTime time.Time
	found := false

	for key, entry := range c.cache {
		if !found || entry.UpdatedAt.Before(oldestTime) {
			oldest = key
			oldestTime = entry.UpdatedAt
			found = true
		}
	}

	if !found {
		return
	}

	entry := c.cache[oldest]
	c.usedMemory -= entry.Size
	delete(c.cache, oldest)
	c.logger.Debug("Evicted cache entry: namespace=%s, age=%v",
		oldest.namespace, time.Since(oldestTime))
}
