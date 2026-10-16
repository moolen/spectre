package namespacegraph

import (
	"context"
	"time"
)

// evictionLoop drops expired entries so their memory is released without waiting for
// the size limit.
func (c *Cache) evictionLoop(ctx context.Context) {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.RefreshTTL)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.evictExpired()
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (c *Cache) evictExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.cache {
		if now.After(entry.ExpiresAt) {
			c.usedMemory -= entry.Size
			delete(c.cache, key)
		}
	}
}
