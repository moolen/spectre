package storage

import (
	"testing"

	"github.com/moolen/spectre/internal/models"
)

// TestNewBlockCache tests cache creation
func TestNewBlockCache(t *testing.T) {
	tests := []struct {
		name      string
		maxMemMB  int64
		shouldErr bool
	}{
		{
			name:      "Valid 100MB cache",
			maxMemMB:  100,
			shouldErr: false,
		},
		{
			name:      "Valid 1MB cache",
			maxMemMB:  1,
			shouldErr: false,
		},
		{
			name:      "Valid 1000MB cache",
			maxMemMB:  1000,
			shouldErr: false,
		},
		{
			name:      "Invalid 0MB cache",
			maxMemMB:  0,
			shouldErr: true,
		},
		{
			name:      "Invalid negative cache",
			maxMemMB:  -1,
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache, err := NewBlockCache(tt.maxMemMB)
			if tt.shouldErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if cache == nil {
					t.Errorf("expected cache, got nil")
				}
				expectedMaxMemory := tt.maxMemMB * 1024 * 1024
				if cache.maxMemory != expectedMaxMemory {
					t.Errorf("expected maxMemory %d, got %d", expectedMaxMemory, cache.maxMemory)
				}
			}
		})
	}
}

// TestBlockCacheGetPut tests basic get/put operations
func TestBlockCacheGetPut(t *testing.T) {
	cache, err := NewBlockCache(10) // 10MB cache
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Create test block with parsed events
	block := &CachedBlock{
		BlockID: "file1:0",
		Events: []*models.Event{
			{ID: "event1", Timestamp: 1000},
			{ID: "event2", Timestamp: 2000},
		},
		Metadata: nil,
		Size:     1024,
		Filename: "file1",
		ID:       0,
	}

	// Put block in cache
	err = cache.Put("file1", 0, block)
	if err != nil {
		t.Fatalf("failed to put block: %v", err)
	}

	// Get block from cache
	retrieved := cache.Get("file1", 0)
	if retrieved == nil {
		t.Errorf("expected to get block, got nil")
	}
	if retrieved.BlockID != block.BlockID {
		t.Errorf("expected BlockID %s, got %s", block.BlockID, retrieved.BlockID)
	}
	if len(retrieved.Events) != len(block.Events) {
		t.Errorf("expected %d events, got %d", len(block.Events), len(retrieved.Events))
	}
}

// TestBlockCacheStats tests cache statistics
func TestBlockCacheStats(t *testing.T) {
	cache, err := NewBlockCache(10) // 10MB cache
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	block1 := &CachedBlock{
		BlockID:   "file1:0",
		Events:    []*models.Event{},
		Metadata:  nil,
		Size:      1000,
		Filename:  "file1",
		ID:        0,
	}

	block2 := &CachedBlock{
		BlockID:   "file1:1",
		Events:    []*models.Event{},
		Metadata:  nil,
		Size:      2000,
		Filename:  "file1",
		ID:        1,
	}

	cache.Put("file1", 0, block1)
	cache.Put("file1", 1, block2)

	// Hit cache
	cache.Get("file1", 0)
	cache.Get("file1", 0)

	// Miss cache
	cache.Get("file1", 99)

	stats := cache.Stats()

	if stats.Items != 2 {
		t.Errorf("expected 2 items, got %d", stats.Items)
	}
	if stats.UsedMemory != 3000 {
		t.Errorf("expected 3000 used memory, got %d", stats.UsedMemory)
	}
	if stats.Hits != 2 {
		t.Errorf("expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}

	expectedHitRate := 2.0 / 3.0
	if stats.HitRate < expectedHitRate-0.01 || stats.HitRate > expectedHitRate+0.01 {
		t.Errorf("expected hit rate ~%.3f, got %.3f", expectedHitRate, stats.HitRate)
	}
}

// TestBlockCacheLRUEviction tests LRU eviction behavior
func TestBlockCacheLRUEviction(t *testing.T) {
	cache, err := NewBlockCache(1) // 1MB cache
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	block1 := &CachedBlock{
		BlockID:   "file1:0",
		Events:    []*models.Event{},
		Metadata:  nil,
		Size:      400000, // 400KB
		Filename:  "file1",
		ID:        0,
	}

	block2 := &CachedBlock{
		BlockID:   "file1:1",
		Events:    []*models.Event{},
		Metadata:  nil,
		Size:      400000, // 400KB
		Filename:  "file1",
		ID:        1,
	}

	block3 := &CachedBlock{
		BlockID:   "file1:2",
		Events:    []*models.Event{},
		Metadata:  nil,
		Size:      400000, // 400KB
		Filename:  "file1",
		ID:        2,
	}

	// Put first two blocks (800KB total)
	cache.Put("file1", 0, block1)
	cache.Put("file1", 1, block2)

	// Put third block (should evict block1, the oldest)
	cache.Put("file1", 2, block3)

	// block1 should be evicted
	if cache.Get("file1", 0) != nil {
		t.Errorf("expected block1 to be evicted, but it's still in cache")
	}

	// block2 and block3 should be in cache
	if cache.Get("file1", 1) == nil {
		t.Errorf("expected block2 to be in cache")
	}
	if cache.Get("file1", 2) == nil {
		t.Errorf("expected block3 to be in cache")
	}

	stats := cache.Stats()
	if stats.Evictions != 1 {
		t.Errorf("expected 1 eviction, got %d", stats.Evictions)
	}
}

// TestBlockCacheMemoryLimit tests that cache respects memory limit
func TestBlockCacheMemoryLimit(t *testing.T) {
	cache, err := NewBlockCache(1) // 1MB cache
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	block := &CachedBlock{
		BlockID:   "file1:0",
		Events:    []*models.Event{},
		Metadata:  nil,
		Size:      2 * 1024 * 1024, // 2MB (exceeds cache limit)
		Filename:  "file1",
		ID:        0,
	}

	// Should fail because block exceeds cache limit
	err = cache.Put("file1", 0, block)
	if err == nil {
		t.Errorf("expected error when putting oversized block, got nil")
	}
	if cache.lru.Len() != 0 {
		t.Errorf("expected 0 items in cache after failed put")
	}
}

// TestBlockCacheClear tests cache clearing
func TestBlockCacheClear(t *testing.T) {
	cache, err := NewBlockCache(10) // 10MB cache
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	block1 := &CachedBlock{
		BlockID:   "file1:0",
		Events:    []*models.Event{},
		Metadata:  nil,
		Size:      1000,
		Filename:  "file1",
		ID:        0,
	}

	block2 := &CachedBlock{
		BlockID:   "file1:1",
		Events:    []*models.Event{},
		Metadata:  nil,
		Size:      2000,
		Filename:  "file1",
		ID:        1,
	}

	cache.Put("file1", 0, block1)
	cache.Put("file1", 1, block2)

	stats := cache.Stats()
	if stats.Items != 2 {
		t.Errorf("expected 2 items before clear, got %d", stats.Items)
	}
	if stats.UsedMemory != 3000 {
		t.Errorf("expected 3000 used memory before clear, got %d", stats.UsedMemory)
	}

	// Clear cache
	cache.Clear()

	stats = cache.Stats()
	if stats.Items != 0 {
		t.Errorf("expected 0 items after clear, got %d", stats.Items)
	}
	if stats.UsedMemory != 0 {
		t.Errorf("expected 0 used memory after clear, got %d", stats.UsedMemory)
	}
}

// TestBlockCacheMakeKey tests key generation
func TestBlockCacheMakeKey(t *testing.T) {
	key := makeKey("test_file.bin", 42)
	expected := "test_file.bin:42"
	if key != expected {
		t.Errorf("expected key %q, got %q", expected, key)
	}
}

// TestBlockCacheConcurrent tests concurrent access
func TestBlockCacheConcurrent(t *testing.T) {
	cache, err := NewBlockCache(100) // 100MB cache
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Create multiple blocks
	blocks := make([]*CachedBlock, 10)
	for i := 0; i < 10; i++ {
		blocks[i] = &CachedBlock{
			BlockID:   makeKey("file1", int32(i)),
			Events:    []*models.Event{},
			Metadata:  nil,
			Size:      100000,
			Filename:  "file1",
			ID:        int32(i),
		}
	}

	// Put blocks concurrently using goroutines
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			cache.Put("file1", int32(idx), blocks[idx])
			done <- true
		}(i)
	}

	// Wait for all puts to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Get blocks concurrently
	for i := 0; i < 10; i++ {
		go func(idx int) {
			retrieved := cache.Get("file1", int32(idx))
			if retrieved == nil {
				t.Errorf("expected block %d to be in cache", idx)
			}
			done <- true
		}(i)
	}

	// Wait for all gets to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	stats := cache.Stats()
	if stats.Items != 10 {
		t.Errorf("expected 10 items, got %d", stats.Items)
	}
}

// TestBlockCacheHitRateCalc tests hit rate calculation
func TestBlockCacheHitRateCalc(t *testing.T) {
	cache, err := NewBlockCache(10) // 10MB cache
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	block := &CachedBlock{
		BlockID:   "file1:0",
		Events:    []*models.Event{},
		Metadata:  nil,
		Size:      1000,
		Filename:  "file1",
		ID:        0,
	}

	cache.Put("file1", 0, block)

	// 4 hits, 2 misses = 0.667 hit rate
	cache.Get("file1", 0) // hit
	cache.Get("file1", 0) // hit
	cache.Get("file1", 0) // hit
	cache.Get("file1", 0) // hit
	cache.Get("file1", 1) // miss
	cache.Get("file1", 2) // miss

	stats := cache.Stats()
	expectedRate := 4.0 / 6.0

	if stats.HitRate < expectedRate-0.01 || stats.HitRate > expectedRate+0.01 {
		t.Errorf("expected hit rate ~%.3f, got %.3f", expectedRate, stats.HitRate)
	}
}

// TestBlockCacheZeroHitRate tests hit rate when no accesses
func TestBlockCacheZeroHitRate(t *testing.T) {
	cache, err := NewBlockCache(10) // 10MB cache
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	block := &CachedBlock{
		BlockID:   "file1:0",
		Events:    []*models.Event{},
		Metadata:  nil,
		Size:      1000,
		Filename:  "file1",
		ID:        0,
	}

	cache.Put("file1", 0, block)

	stats := cache.Stats()
	if stats.HitRate != 0.0 {
		t.Errorf("expected hit rate 0.0 with no accesses, got %.3f", stats.HitRate)
	}
}
