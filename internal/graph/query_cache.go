package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/moolen/spectre/internal/logging"
)

// QueryCacheConfig holds cache configuration
type QueryCacheConfig struct {
	MaxMemoryMB int64         // Max memory in MB (default: 64)
	TTL         time.Duration // Entry TTL (default: 2 minutes)
	Enabled     bool          // Enable/disable cache
}

// DefaultQueryCacheConfig returns default cache configuration
func DefaultQueryCacheConfig() QueryCacheConfig {
	return QueryCacheConfig{
		MaxMemoryMB: 64,
		TTL:         2 * time.Minute,
		Enabled:     false,
	}
}

// CachedQueryResult wraps a QueryResult with size tracking and TTL
type CachedQueryResult struct {
	Result    *QueryResult // The cached query result
	Size      int64        // Estimated memory size in bytes
	ExpiresAt time.Time    // TTL expiration timestamp
	CacheKey  string       // Cache key for debugging
}

// QueryCacheStats represents cache statistics
type QueryCacheStats struct {
	MaxMemory       int64   // Max memory in bytes
	UsedMemory      int64   // Current memory usage in bytes
	AvailableMemory int64   // Available memory in bytes
	Items           int     // Number of items in cache
	Hits            uint64  // Cache hits
	Misses          uint64  // Cache misses
	Evictions       uint64  // Items evicted due to memory pressure
	Expired         uint64  // Items expired due to TTL
	HitRate         float64 // Hit rate (0.0-1.0)
}

// QueryCache provides LRU caching for graph queries with TTL and memory limits
type QueryCache struct {
	lru        *lru.Cache[string, *CachedQueryResult]
	maxMemory  int64 // Max memory in bytes
	usedMemory int64 // Current memory usage (protected by mu for writes, atomic for reads)
	ttl        time.Duration
	mu         sync.RWMutex
	logger     *logging.Logger

	// Metrics (atomic)
	hits      uint64
	misses    uint64
	evictions uint64
	expired   uint64
}

// NewQueryCache creates a new query cache with the specified configuration
func NewQueryCache(config QueryCacheConfig, logger *logging.Logger) (*QueryCache, error) {
	if config.MaxMemoryMB <= 0 {
		return nil, fmt.Errorf("MaxMemoryMB must be positive, got %d", config.MaxMemoryMB)
	}
	if config.TTL <= 0 {
		return nil, fmt.Errorf("TTL must be positive, got %v", config.TTL)
	}

	qc := &QueryCache{
		maxMemory: config.MaxMemoryMB * 1024 * 1024, // Convert to bytes
		ttl:       config.TTL,
		logger:    logger,
	}

	// Create LRU cache with eviction callback
	// Use a high initial capacity to avoid resizing
	lruCache, err := lru.NewWithEvict[string, *CachedQueryResult](10000, func(key string, value *CachedQueryResult) {
		qc.onEvict(key, value)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create LRU cache: %w", err)
	}

	qc.lru = lruCache
	qc.logger.Debug("Query cache initialized: maxMemory=%dMB, TTL=%v", config.MaxMemoryMB, config.TTL)
	return qc, nil
}

// onEvict is called when an item is evicted from the LRU cache
func (qc *QueryCache) onEvict(key string, entry *CachedQueryResult) {
	atomic.AddUint64(&qc.evictions, 1)
	atomic.AddInt64(&qc.usedMemory, -entry.Size)

	usedMemoryKB := atomic.LoadInt64(&qc.usedMemory) / 1024
	maxMemoryKB := qc.maxMemory / 1024
	qc.logger.Debug("Query cache EVICT: key=%s, size=%dKB, usedMem=%dKB/%dKB",
		key[:16], entry.Size/1024, usedMemoryKB, maxMemoryKB)
}

// Get retrieves a cached query result by key, returning nil if not found or expired
func (qc *QueryCache) Get(key string) (*QueryResult, bool) {
	qc.mu.RLock()
	defer qc.mu.RUnlock()

	entry, ok := qc.lru.Get(key)
	if !ok {
		atomic.AddUint64(&qc.misses, 1)
		qc.logCacheAccess(key, false, false)
		return nil, false
	}

	// Check TTL expiration
	if time.Now().After(entry.ExpiresAt) {
		atomic.AddUint64(&qc.expired, 1)
		atomic.AddUint64(&qc.misses, 1)
		// Remove expired entry (need to upgrade to write lock)
		// We'll let the next Put handle cleanup or let it expire naturally
		qc.logCacheAccess(key, false, true)
		return nil, false
	}

	atomic.AddUint64(&qc.hits, 1)
	qc.logCacheAccess(key, true, false)
	return entry.Result, true
}

// Put stores a query result in the cache
func (qc *QueryCache) Put(key string, result *QueryResult) {
	qc.mu.Lock()
	defer qc.mu.Unlock()

	// Estimate result size
	size := estimateResultSize(result)

	// Check if we already have this key (update case)
	if existing, ok := qc.lru.Peek(key); ok {
		// Remove old size from tracking
		atomic.AddInt64(&qc.usedMemory, -existing.Size)
		qc.lru.Remove(key)
	}

	// Check if adding this result would exceed memory limit
	currentUsed := atomic.LoadInt64(&qc.usedMemory)
	if currentUsed+size > qc.maxMemory {
		// Evict oldest items until we have space
		for currentUsed+size > qc.maxMemory && qc.lru.Len() > 0 {
			qc.lru.RemoveOldest()
			currentUsed = atomic.LoadInt64(&qc.usedMemory)
		}

		// If still over limit after eviction, reject
		if currentUsed+size > qc.maxMemory {
			qc.logger.Warn("Query cache PUT REJECTED: key=%s, size=%dKB, reason=exceeds_memory, usedMem=%dKB/%dKB",
				key[:16], size/1024, currentUsed/1024, qc.maxMemory/1024)
			return
		}
	}

	entry := &CachedQueryResult{
		Result:    result,
		Size:      size,
		ExpiresAt: time.Now().Add(qc.ttl),
		CacheKey:  key,
	}

	qc.lru.Add(key, entry)
	atomic.AddInt64(&qc.usedMemory, size)

	usedMemoryKB := atomic.LoadInt64(&qc.usedMemory) / 1024
	maxMemoryKB := qc.maxMemory / 1024
	qc.logger.Debug("Query cache PUT: key=%s, size=%dKB, rows=%d, usedMem=%dKB/%dKB",
		key[:16], size/1024, len(result.Rows), usedMemoryKB, maxMemoryKB)
}

// Invalidate removes a specific entry from the cache
func (qc *QueryCache) Invalidate(key string) {
	qc.mu.Lock()
	defer qc.mu.Unlock()

	if entry, ok := qc.lru.Peek(key); ok {
		atomic.AddInt64(&qc.usedMemory, -entry.Size)
		qc.lru.Remove(key)
		qc.logger.Debug("Query cache INVALIDATE: key=%s", key[:16])
	}
}

// Clear removes all entries from the cache
func (qc *QueryCache) Clear() {
	qc.mu.Lock()
	defer qc.mu.Unlock()

	qc.lru.Purge()
	atomic.StoreInt64(&qc.usedMemory, 0)
	qc.logger.Debug("Query cache CLEAR: all entries removed")
}

// Stats returns cache statistics
func (qc *QueryCache) Stats() QueryCacheStats {
	qc.mu.RLock()
	defer qc.mu.RUnlock()

	hits := atomic.LoadUint64(&qc.hits)
	misses := atomic.LoadUint64(&qc.misses)
	total := hits + misses

	hitRate := 0.0
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	usedMemory := atomic.LoadInt64(&qc.usedMemory)

	return QueryCacheStats{
		MaxMemory:       qc.maxMemory,
		UsedMemory:      usedMemory,
		AvailableMemory: qc.maxMemory - usedMemory,
		Items:           qc.lru.Len(),
		Hits:            hits,
		Misses:          misses,
		Evictions:       atomic.LoadUint64(&qc.evictions),
		Expired:         atomic.LoadUint64(&qc.expired),
		HitRate:         hitRate,
	}
}

// MakeQueryKey creates a deterministic cache key from a GraphQuery
func MakeQueryKey(query GraphQuery) string {
	h := sha256.New()

	// Write query string
	h.Write([]byte(query.Query))

	// Sort parameter keys for deterministic ordering
	if len(query.Parameters) > 0 {
		keys := make([]string, 0, len(query.Parameters))
		for k := range query.Parameters {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// Write sorted parameters
		for _, k := range keys {
			h.Write([]byte(k))
			paramBytes, _ := json.Marshal(query.Parameters[k])
			h.Write(paramBytes)
		}
	}

	return hex.EncodeToString(h.Sum(nil))
}

// estimateResultSize estimates the memory footprint of a QueryResult
func estimateResultSize(result *QueryResult) int64 {
	if result == nil {
		return 0
	}

	size := int64(0)

	// Columns: estimate ~50 bytes per column name on average
	size += int64(len(result.Columns) * 50)

	// Rows: estimate based on JSON serialization
	for _, row := range result.Rows {
		rowBytes, err := json.Marshal(row)
		if err == nil {
			size += int64(len(rowBytes))
		} else {
			// Fallback: estimate 100 bytes per cell
			size += int64(len(row) * 100)
		}
	}

	// Add overhead for struct, slices, and pointers
	size += 200

	return size
}

// logCacheAccess logs cache access for debugging
func (qc *QueryCache) logCacheAccess(key string, hit, expired bool) {
	hits := atomic.LoadUint64(&qc.hits)
	misses := atomic.LoadUint64(&qc.misses)
	total := hits + misses

	hitRate := 0.0
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	usedMemoryKB := atomic.LoadInt64(&qc.usedMemory) / 1024
	maxMemoryKB := qc.maxMemory / 1024

	if expired {
		qc.logger.Debug("Query cache GET: key=%s, hit=false (expired), hitRate=%.1f%%, usedMem=%dKB/%dKB",
			key[:16], hitRate, usedMemoryKB, maxMemoryKB)
	} else {
		qc.logger.Debug("Query cache GET: key=%s, hit=%t, hitRate=%.1f%%, usedMem=%dKB/%dKB",
			key[:16], hit, hitRate, usedMemoryKB, maxMemoryKB)
	}
}
