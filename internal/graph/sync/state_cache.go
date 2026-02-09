package sync

import (
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru/v2"
)

// DefaultStateCacheSize is the default number of resource states to cache
const DefaultStateCacheSize = 10000

// ResourceState holds the cached state for a resource
type ResourceState struct {
	Data      []byte // Last known JSON snapshot
	Timestamp int64  // Event timestamp
	EventType string // CREATE, UPDATE, DELETE
}

// StateCache provides an LRU cache for recent resource states.
// This eliminates the need to query the database for change detection
// on UPDATE events, as we can compare against the cached previous state.
type StateCache struct {
	cache   *lru.Cache[string, *ResourceState]
	hits    atomic.Int64
	misses  atomic.Int64
	maxSize int
}

// NewStateCache creates a new state cache with the given max size.
// Returns an error if the cache cannot be created.
func NewStateCache(maxSize int) (*StateCache, error) {
	if maxSize <= 0 {
		maxSize = DefaultStateCacheSize
	}

	cache, err := lru.New[string, *ResourceState](maxSize)
	if err != nil {
		return nil, err
	}

	return &StateCache{
		cache:   cache,
		maxSize: maxSize,
	}, nil
}

// Get retrieves the previous state for a resource by UID.
// Returns nil if the resource is not in the cache.
// This is the primary method used during change detection.
func (c *StateCache) Get(uid string) *ResourceState {
	if state, ok := c.cache.Get(uid); ok {
		c.hits.Add(1)
		return state
	}
	c.misses.Add(1)
	return nil
}

// Put stores the state for a resource.
// This should be called after processing each non-DELETE event.
func (c *StateCache) Put(uid string, data []byte, timestamp int64, eventType string) {
	// Make a copy of data to avoid holding references to potentially large buffers
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	c.cache.Add(uid, &ResourceState{
		Data:      dataCopy,
		Timestamp: timestamp,
		EventType: eventType,
	})
}

// Remove removes a resource from the cache.
// This should be called on DELETE events.
func (c *StateCache) Remove(uid string) {
	c.cache.Remove(uid)
}

// Contains checks if a resource is in the cache without updating LRU order.
func (c *StateCache) Contains(uid string) bool {
	return c.cache.Contains(uid)
}

// Len returns the current number of items in the cache.
func (c *StateCache) Len() int {
	return c.cache.Len()
}

// GetStats returns cache statistics: hits, misses, and current size.
func (c *StateCache) GetStats() (hits, misses int64, size int) {
	return c.hits.Load(), c.misses.Load(), c.cache.Len()
}

// ResetStats resets the hit/miss counters to zero.
func (c *StateCache) ResetStats() {
	c.hits.Store(0)
	c.misses.Store(0)
}

// Clear empties the cache and resets statistics.
func (c *StateCache) Clear() {
	c.cache.Purge()
	c.ResetStats()
}

// HitRate returns the cache hit rate as a percentage (0-100).
// Returns 0 if no lookups have been performed.
func (c *StateCache) HitRate() float64 {
	hits := c.hits.Load()
	misses := c.misses.Load()
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total) * 100
}
