package namespacegraph

import "time"

// getIfValid returns the cached entry if it exists and hasn't expired.
func (c *Cache) getIfValid(namespace string) *CachedEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.cache[namespace]
	if !ok || time.Now().After(entry.ExpiresAt) {
		return nil
	}

	return entry
}

// set stores a response in the cache.
func (c *Cache) set(namespace string, response *NamespaceGraphResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	size := estimateResponseSize(response)
	maxBytes := c.config.MaxMemoryMB * 1024 * 1024
	for c.usedMemory+size > maxBytes && len(c.cache) > 0 {
		c.evictOldestLocked()
	}

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

// InvalidateNamespaces marks the given namespaces as stale.
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

// InvalidateAll marks all cached entries as stale.
func (c *Cache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for ns := range c.cache {
		c.dirtySet[ns] = true
	}

	c.logger.Info("Invalidated all %d namespace graph caches", len(c.cache))
}

// evictOldestLocked removes the oldest cache entry (caller must hold lock).
func (c *Cache) evictOldestLocked() {
	var oldest string
	var oldestTime time.Time

	for ns, entry := range c.cache {
		if oldestTime.IsZero() || entry.UpdatedAt.Before(oldestTime) {
			oldest = ns
			oldestTime = entry.UpdatedAt
		}
	}

	if oldest == "" {
		return
	}

	entry := c.cache[oldest]
	c.usedMemory -= entry.Size
	delete(c.cache, oldest)
	c.logger.Debug("Evicted cache entry: namespace=%s, age=%v",
		oldest, time.Since(oldestTime))
}
