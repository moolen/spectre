package validation

import (
	"context"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/graph"
	"github.com/moolen/spectre/internal/logging"
)

// CachedResourceLookup provides cached resource lookups to reduce graph query load
type CachedResourceLookup struct {
	underlying graph.Client
	cache      *LRUCache
	logger     *logging.Logger
	
	// Statistics
	mu          sync.RWMutex
	hits        int64
	misses      int64
	evictions   int64
}

// CacheConfig holds configuration for the cache
type CacheConfig struct {
	// MaxSize is the maximum number of cached entries
	MaxSize int
	
	// TTL is the time-to-live for cache entries
	TTL time.Duration
}

// DefaultCacheConfig returns the default cache configuration
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		MaxSize: 10000,
		TTL:     5 * time.Minute,
	}
}

// NewCachedResourceLookup creates a new cached resource lookup
func NewCachedResourceLookup(client graph.Client, config CacheConfig) *CachedResourceLookup {
	return &CachedResourceLookup{
		underlying: client,
		cache:      NewLRUCache(config.MaxSize, config.TTL),
		logger:     logging.GetLogger("graph.cache"),
	}
}

// FindResourceByUID looks up a resource by UID with caching
func (c *CachedResourceLookup) FindResourceByUID(ctx context.Context, uid string) (*graph.ResourceIdentity, error) {
	cacheKey := "uid:" + uid
	
	// Check cache
	if cached, ok := c.cache.Get(cacheKey); ok {
		c.recordHit()
		if res, ok := cached.(*graph.ResourceIdentity); ok {
			return res, nil
		}
	}
	
	c.recordMiss()
	
	// Query graph
	query := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity {uid: $uid})
			RETURN r
			LIMIT 1
		`,
		Parameters: map[string]interface{}{
			"uid": uid,
		},
	}
	
	result, err := c.underlying.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, err
	}
	
	if len(result.Rows) == 0 {
		return nil, nil
	}
	
	// Parse result
	node, ok := result.Rows[0][0].(map[string]interface{})
	if !ok {
		return nil, nil
	}
	
	resource := parseResourceIdentity(node)
	
	// Cache result
	c.cache.Put(cacheKey, resource)
	
	return resource, nil
}

// FindResourceByNamespace looks up a resource by namespace and name with caching
func (c *CachedResourceLookup) FindResourceByNamespace(ctx context.Context, namespace, kind, name string) (*graph.ResourceIdentity, error) {
	cacheKey := namespace + ":" + kind + ":" + name
	
	// Check cache
	if cached, ok := c.cache.Get(cacheKey); ok {
		c.recordHit()
		if res, ok := cached.(*graph.ResourceIdentity); ok {
			return res, nil
		}
	}
	
	c.recordMiss()
	
	// Query graph
	query := graph.GraphQuery{
		Query: `
			MATCH (r:ResourceIdentity {namespace: $namespace, kind: $kind, name: $name})
			WHERE r.deleted = false
			RETURN r
			LIMIT 1
		`,
		Parameters: map[string]interface{}{
			"namespace": namespace,
			"kind":      kind,
			"name":      name,
		},
	}
	
	result, err := c.underlying.ExecuteQuery(ctx, query)
	if err != nil {
		return nil, err
	}
	
	if len(result.Rows) == 0 {
		return nil, nil
	}
	
	// Parse result
	node, ok := result.Rows[0][0].(map[string]interface{})
	if !ok {
		return nil, nil
	}
	
	resource := parseResourceIdentity(node)
	
	// Cache result
	c.cache.Put(cacheKey, resource)
	
	return resource, nil
}

// GetStats returns cache statistics
func (c *CachedResourceLookup) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	total := c.hits + c.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(c.hits) / float64(total)
	}
	
	return map[string]interface{}{
		"hits":      c.hits,
		"misses":    c.misses,
		"evictions": c.evictions,
		"hitRate":   hitRate,
		"size":      c.cache.Size(),
	}
}

// InvalidateUID invalidates a cache entry by UID
func (c *CachedResourceLookup) InvalidateUID(uid string) {
	c.cache.Delete("uid:" + uid)
}

// InvalidateResource invalidates a cache entry by namespace/kind/name
func (c *CachedResourceLookup) InvalidateResource(namespace, kind, name string) {
	c.cache.Delete(namespace + ":" + kind + ":" + name)
}

// Clear clears the entire cache
func (c *CachedResourceLookup) Clear() {
	c.cache.Clear()
	
	c.mu.Lock()
	c.hits = 0
	c.misses = 0
	c.evictions = 0
	c.mu.Unlock()
}

func (c *CachedResourceLookup) recordHit() {
	c.mu.Lock()
	c.hits++
	c.mu.Unlock()
}

func (c *CachedResourceLookup) recordMiss() {
	c.mu.Lock()
	c.misses++
	c.mu.Unlock()
}

// parseResourceIdentity converts a graph node to a ResourceIdentity
func parseResourceIdentity(node map[string]interface{}) *graph.ResourceIdentity {
	resource := &graph.ResourceIdentity{}
	
	if uid, ok := node["uid"].(string); ok {
		resource.UID = uid
	}
	if kind, ok := node["kind"].(string); ok {
		resource.Kind = kind
	}
	if apiGroup, ok := node["apiGroup"].(string); ok {
		resource.APIGroup = apiGroup
	}
	if version, ok := node["version"].(string); ok {
		resource.Version = version
	}
	if namespace, ok := node["namespace"].(string); ok {
		resource.Namespace = namespace
	}
	if name, ok := node["name"].(string); ok {
		resource.Name = name
	}
	if labels, ok := node["labels"].(map[string]string); ok {
		resource.Labels = labels
	}
	if firstSeen, ok := node["firstSeen"].(int64); ok {
		resource.FirstSeen = firstSeen
	}
	if lastSeen, ok := node["lastSeen"].(int64); ok {
		resource.LastSeen = lastSeen
	}
	if deleted, ok := node["deleted"].(bool); ok {
		resource.Deleted = deleted
	}
	if deletedAt, ok := node["deletedAt"].(int64); ok {
		resource.DeletedAt = deletedAt
	}
	
	return resource
}

// LRUCache implements a simple LRU cache with TTL
type LRUCache struct {
	mu       sync.RWMutex
	maxSize  int
	ttl      time.Duration
	items    map[string]*cacheItem
	lruList  []string // Most recently used at the end
}

type cacheItem struct {
	value     interface{}
	expiresAt time.Time
}

// NewLRUCache creates a new LRU cache
func NewLRUCache(maxSize int, ttl time.Duration) *LRUCache {
	return &LRUCache{
		maxSize: maxSize,
		ttl:     ttl,
		items:   make(map[string]*cacheItem),
		lruList: make([]string, 0, maxSize),
	}
}

// Get retrieves a value from the cache
func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	item, ok := c.items[key]
	if !ok {
		return nil, false
	}
	
	// Check if expired
	if time.Now().After(item.expiresAt) {
		delete(c.items, key)
		c.removeFromLRU(key)
		return nil, false
	}
	
	// Move to end of LRU list (most recently used)
	c.removeFromLRU(key)
	c.lruList = append(c.lruList, key)
	
	return item.value, true
}

// Put adds a value to the cache
func (c *LRUCache) Put(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Check if already exists
	if _, ok := c.items[key]; ok {
		c.removeFromLRU(key)
	} else if len(c.items) >= c.maxSize {
		// Evict least recently used
		if len(c.lruList) > 0 {
			evictKey := c.lruList[0]
			delete(c.items, evictKey)
			c.lruList = c.lruList[1:]
		}
	}
	
	// Add new item
	c.items[key] = &cacheItem{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.lruList = append(c.lruList, key)
}

// Delete removes a value from the cache
func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	delete(c.items, key)
	c.removeFromLRU(key)
}

// Clear removes all items from the cache
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.items = make(map[string]*cacheItem)
	c.lruList = make([]string, 0, c.maxSize)
}

// Size returns the current number of items in the cache
func (c *LRUCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return len(c.items)
}

// removeFromLRU removes a key from the LRU list
func (c *LRUCache) removeFromLRU(key string) {
	for i, k := range c.lruList {
		if k == key {
			c.lruList = append(c.lruList[:i], c.lruList[i+1:]...)
			break
		}
	}
}
