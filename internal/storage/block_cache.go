package storage

import (
	"fmt"
	"sync"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/moolen/spectre/internal/models"
)

// CachedBlock represents a decompressed and parsed block
type CachedBlock struct {
	BlockID  string              // "filename:block_id"
	Events   []*models.Event     // Parsed events (pre-unmarshalled for cache hits)
	Metadata interface{}         // Block metadata for filtering
	Size     int64               // Memory size in bytes
	Filename string              // Source file
	ID       int32               // Block ID
}

// BlockCache manages in-memory LRU cache of blocks
type BlockCache struct {
	lru        *lru.Cache[string, *CachedBlock] // golang-lru/v2
	maxMemory  int64                            // Max memory in bytes (e.g., 100MB)
	usedMemory int64                            // Current memory usage
	mu         sync.RWMutex                     // Thread safety

	// Metrics
	hits       uint64 // Cache hits
	misses     uint64 // Cache misses
	evictions  uint64 // Items evicted
	bytesRead  uint64 // Total bytes decompressed
}

// NewBlockCache creates a new block cache with specified max memory in MB
func NewBlockCache(maxMemoryMB int64) (*BlockCache, error) {
	if maxMemoryMB <= 0 {
		return nil, fmt.Errorf("maxMemoryMB must be positive, got %d", maxMemoryMB)
	}

	bc := &BlockCache{
		maxMemory: maxMemoryMB * 1024 * 1024, // Convert to bytes
	}

	// Create LRU cache with eviction callback
	// Use a high initial capacity to avoid resizing
	// golang-lru requires a positive size parameter
	lruCache, err := lru.NewWithEvict[string, *CachedBlock](10000, func(key string, value *CachedBlock) {
		bc.onEvict(key, value)
	})
	if err != nil {
		return nil, err
	}

	bc.lru = lruCache
	return bc, nil
}

// onEvict is called when an item is evicted from the LRU cache
func (bc *BlockCache) onEvict(_ string, block *CachedBlock) {
	atomic.AddUint64(&bc.evictions, 1)
	atomic.AddInt64(&bc.usedMemory, -block.Size)
}

// Get retrieves a cached block or returns nil
func (bc *BlockCache) Get(filename string, blockID int32) *CachedBlock {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	key := makeKey(filename, blockID)
	if block, ok := bc.lru.Get(key); ok {
		atomic.AddUint64(&bc.hits, 1)
		return block
	}

	atomic.AddUint64(&bc.misses, 1)
	return nil
}

// Put stores a block in cache, evicting if necessary
func (bc *BlockCache) Put(filename string, blockID int32, block *CachedBlock) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	key := makeKey(filename, blockID)
	blockSize := block.Size

	// Check if adding this block would exceed memory limit
	if bc.usedMemory+blockSize > bc.maxMemory {
		// Evict oldest item if it helps
		if bc.lru.Len() > 0 {
			bc.lru.RemoveOldest()
		}

		// If still over limit after eviction, reject
		if bc.usedMemory+blockSize > bc.maxMemory {
			return fmt.Errorf("block size %d exceeds remaining memory %d",
				blockSize, bc.maxMemory-bc.usedMemory)
		}
	}

	bc.lru.Add(key, block)
	bc.usedMemory += blockSize
	atomic.AddUint64(&bc.bytesRead, uint64(blockSize)) //nolint:gosec // safe conversion: block size is positive

	return nil
}

// CacheStats represents cache statistics
type CacheStats struct {
	MaxMemory           int64
	UsedMemory          int64
	AvailableMemory     int64
	Items               int
	Hits                uint64
	Misses              uint64
	Evictions           uint64
	BytesDecompressed   uint64
	HitRate             float64
}

// Stats returns cache statistics
func (bc *BlockCache) Stats() CacheStats {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	total := atomic.LoadUint64(&bc.hits) + atomic.LoadUint64(&bc.misses)
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(atomic.LoadUint64(&bc.hits)) / float64(total)
	}

	return CacheStats{
		MaxMemory:         bc.maxMemory,
		UsedMemory:        bc.usedMemory,
		AvailableMemory:   bc.maxMemory - bc.usedMemory,
		Items:             bc.lru.Len(),
		Hits:              atomic.LoadUint64(&bc.hits),
		Misses:            atomic.LoadUint64(&bc.misses),
		Evictions:         atomic.LoadUint64(&bc.evictions),
		BytesDecompressed: atomic.LoadUint64(&bc.bytesRead),
		HitRate:           hitRate,
	}
}

// Clear removes all cached blocks
func (bc *BlockCache) Clear() {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	bc.lru.Purge()
	bc.usedMemory = 0
}

// makeKey creates a cache key from filename and block ID
func makeKey(filename string, blockID int32) string {
	return fmt.Sprintf("%s:%d", filename, blockID)
}
