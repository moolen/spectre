package storage

import (
	"sync"
	"time"
)

// SharedFileDataCache provides thread-safe sharing of file metadata across concurrent queries
// This is used by the timeline handler to avoid reading the same files multiple times
// when executing concurrent resource and Event queries
type SharedFileDataCache struct {
	cache map[string]*SharedCachedFileData
	mu    sync.RWMutex
}

// SharedCachedFileData represents cached file data shared across queries
type SharedCachedFileData struct {
	Data     *StorageFileData
	LoadedAt time.Time
}

// NewSharedFileDataCache creates a new shared file data cache
func NewSharedFileDataCache() *SharedFileDataCache {
	return &SharedFileDataCache{
		cache: make(map[string]*SharedCachedFileData),
	}
}

// GetOrLoad retrieves cached file data or loads it using the provided loader function
// This ensures each file is only loaded once even when accessed concurrently
func (sfc *SharedFileDataCache) GetOrLoad(filePath string, loader func() (*StorageFileData, error)) (*StorageFileData, error) {
	// Try to get from cache first (read lock)
	sfc.mu.RLock()
	cached, ok := sfc.cache[filePath]
	sfc.mu.RUnlock()

	if ok {
		return cached.Data, nil
	}

	// Not in cache, need to load (write lock)
	sfc.mu.Lock()
	defer sfc.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine might have loaded it)
	cached, ok = sfc.cache[filePath]
	if ok {
		return cached.Data, nil
	}

	// Load file data
	data, err := loader()
	if err != nil {
		return nil, err
	}

	// Cache it
	sfc.cache[filePath] = &SharedCachedFileData{
		Data:     data,
		LoadedAt: time.Now(),
	}

	return data, nil
}

// Size returns the number of cached files
func (sfc *SharedFileDataCache) Size() int {
	sfc.mu.RLock()
	defer sfc.mu.RUnlock()
	return len(sfc.cache)
}

// Clear removes all cached entries
func (sfc *SharedFileDataCache) Clear() {
	sfc.mu.Lock()
	defer sfc.mu.Unlock()
	sfc.cache = make(map[string]*SharedCachedFileData)
}
