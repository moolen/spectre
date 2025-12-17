package storage

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/moolen/spectre/internal/logging"
)

// CachedFileMetadata represents cached file metadata with modification time
type CachedFileMetadata struct {
	FilePath string
	Data     *StorageFileData
	ModTime  time.Time
	Size     int64
	CachedAt time.Time
}

// FileMetadataCache caches file metadata (headers, footers, index sections)
type FileMetadataCache struct {
	lru           *lru.Cache[string, *CachedFileMetadata]
	maxMemory     int64
	usedMemory    int64
	mu            sync.RWMutex
	logger        *logging.Logger
	hits          uint64
	misses        uint64
	invalidations uint64
}

// NewFileMetadataCache creates a new file metadata cache
func NewFileMetadataCache(maxMemoryMB int64, logger *logging.Logger) (*FileMetadataCache, error) {
	if maxMemoryMB <= 0 {
		return nil, fmt.Errorf("maxMemoryMB must be positive, got %d", maxMemoryMB)
	}

	fmc := &FileMetadataCache{
		maxMemory: maxMemoryMB * 1024 * 1024,
		logger:    logger,
	}

	lruCache, err := lru.NewWithEvict[string, *CachedFileMetadata](1000, func(key string, value *CachedFileMetadata) {
		fmc.onEvict(key, value)
	})
	if err != nil {
		return nil, err
	}

	fmc.lru = lruCache
	fmc.logger.Debug("File metadata cache initialized: maxMemory=%dMB", maxMemoryMB)
	return fmc, nil
}

// onEvict is called when an item is evicted from the cache
func (fmc *FileMetadataCache) onEvict(key string, cached *CachedFileMetadata) {
	atomic.AddInt64(&fmc.usedMemory, -cached.Size)
	usedMemoryMB := fmc.usedMemory / (1024 * 1024)
	maxMemoryMB := fmc.maxMemory / (1024 * 1024)
	fmc.logger.Debug("File metadata cache EVICT: key=%s, size=%dKB, usedMem=%dMB/%dMB",
		key, cached.Size/1024, usedMemoryMB, maxMemoryMB)
}

// Get retrieves cached file metadata if available and valid
func (fmc *FileMetadataCache) Get(filePath string) (*StorageFileData, error) {
	// Check file modification time
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	fmc.mu.RLock()
	cached, ok := fmc.lru.Get(filePath)
	fmc.mu.RUnlock()

	if ok && cached.ModTime.Equal(stat.ModTime()) {
		atomic.AddUint64(&fmc.hits, 1)

		total := atomic.LoadUint64(&fmc.hits) + atomic.LoadUint64(&fmc.misses)
		hitRate := 0.0
		if total > 0 {
			hitRate = float64(atomic.LoadUint64(&fmc.hits)) / float64(total)
		}

		usedMemoryMB := atomic.LoadInt64(&fmc.usedMemory) / (1024 * 1024)
		maxMemoryMB := fmc.maxMemory / (1024 * 1024)
		fmc.logger.Debug("File metadata cache GET: path=%s, hit=true, hitRate=%.2f%%, usedMem=%dMB/%dMB",
			filePath, hitRate*100, usedMemoryMB, maxMemoryMB)

		return cached.Data, nil
	}

	if ok {
		atomic.AddUint64(&fmc.invalidations, 1)
		fmc.logger.Debug("File metadata cache invalidated: path=%s (mtime changed)", filePath)
	}

	atomic.AddUint64(&fmc.misses, 1)

	total := atomic.LoadUint64(&fmc.hits) + atomic.LoadUint64(&fmc.misses)
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(atomic.LoadUint64(&fmc.hits)) / float64(total)
	}

	usedMemoryMB := atomic.LoadInt64(&fmc.usedMemory) / (1024 * 1024)
	maxMemoryMB := fmc.maxMemory / (1024 * 1024)
	fmc.logger.Debug("File metadata cache GET: path=%s, hit=false, hitRate=%.2f%%, usedMem=%dMB/%dMB",
		filePath, hitRate*100, usedMemoryMB, maxMemoryMB)

	return fmc.loadAndCache(filePath, stat.ModTime())
}

// loadAndCache loads file metadata and caches it
func (fmc *FileMetadataCache) loadAndCache(filePath string, modTime time.Time) (*StorageFileData, error) {
	reader, err := NewBlockReader(filePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = reader.Close()
	}()

	fileData, err := reader.ReadFile()
	if err != nil {
		return nil, err
	}

	// Estimate size: header + footer + index section metadata
	// Each block metadata is ~100 bytes, plus final resource states
	size := int64(FileHeaderSize + FileFooterSize)
	size += int64(len(fileData.IndexSection.BlockMetadata)) * 100
	size += int64(len(fileData.IndexSection.FinalResourceStates)) * 500

	cached := &CachedFileMetadata{
		FilePath: filePath,
		Data:     fileData,
		ModTime:  modTime,
		Size:     size,
		CachedAt: time.Now(),
	}

	// Check if we need to evict entries to make space
	fmc.mu.Lock()
	for atomic.LoadInt64(&fmc.usedMemory)+size > fmc.maxMemory && fmc.lru.Len() > 0 {
		// LRU cache will automatically evict via onEvict callback
		fmc.lru.RemoveOldest()
	}

	fmc.lru.Add(filePath, cached)
	atomic.AddInt64(&fmc.usedMemory, size)
	fmc.mu.Unlock()

	usedMemoryMB := atomic.LoadInt64(&fmc.usedMemory) / (1024 * 1024)
	maxMemoryMB := fmc.maxMemory / (1024 * 1024)
	fmc.logger.Debug("File metadata cache ADD: path=%s, size=%dKB, usedMem=%dMB/%dMB",
		filePath, size/1024, usedMemoryMB, maxMemoryMB)

	return fileData, nil
}

// Stats returns cache statistics
func (fmc *FileMetadataCache) Stats() CacheStats {
	hits := atomic.LoadUint64(&fmc.hits)
	misses := atomic.LoadUint64(&fmc.misses)
	invalidations := atomic.LoadUint64(&fmc.invalidations)
	total := hits + misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	return CacheStats{
		Hits:          hits,
		Misses:        misses,
		HitRate:       hitRate,
		UsedMemory:    atomic.LoadInt64(&fmc.usedMemory),
		MaxMemory:     fmc.maxMemory,
		Evictions:     0, // Not separately tracked for file metadata cache
		Invalidations: invalidations,
	}
}

// Clear clears the cache
func (fmc *FileMetadataCache) Clear() {
	fmc.mu.Lock()
	defer fmc.mu.Unlock()

	fmc.lru.Purge()
	atomic.StoreInt64(&fmc.usedMemory, 0)
	atomic.StoreUint64(&fmc.hits, 0)
	atomic.StoreUint64(&fmc.misses, 0)
	atomic.StoreUint64(&fmc.invalidations, 0)

	fmc.logger.Debug("File metadata cache cleared")
}
