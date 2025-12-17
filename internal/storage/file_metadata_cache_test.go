package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

func TestFileMetadataCache_GetAndCache(t *testing.T) {
	logging.Initialize("debug")

	// Create temp directory
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.bin")

	// Create a test file
	createTestBlockFile(t, filePath)

	// Create cache
	cache, err := NewFileMetadataCache(10, logging.GetLogger("test"))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// First get - should be a cache miss
	data1, err := cache.Get(filePath)
	if err != nil {
		t.Fatalf("Failed to get file metadata: %v", err)
	}
	if data1 == nil {
		t.Fatal("Expected file data, got nil")
	}

	stats := cache.Stats()
	if stats.Hits != 0 {
		t.Errorf("Expected 0 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}

	// Second get - should be a cache hit
	data2, err := cache.Get(filePath)
	if err != nil {
		t.Fatalf("Failed to get file metadata on second call: %v", err)
	}
	if data2 == nil {
		t.Fatal("Expected file data, got nil")
	}

	stats = cache.Stats()
	if stats.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}

	// Verify data is the same
	if len(data1.IndexSection.BlockMetadata) != len(data2.IndexSection.BlockMetadata) {
		t.Errorf("Block metadata length mismatch: %d vs %d",
			len(data1.IndexSection.BlockMetadata), len(data2.IndexSection.BlockMetadata))
	}
}

func TestFileMetadataCache_InvalidationOnModification(t *testing.T) {
	logging.Initialize("debug")

	// Create temp directory
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.bin")

	// Create a test file
	createTestBlockFile(t, filePath)

	// Create cache
	cache, err := NewFileMetadataCache(10, logging.GetLogger("test"))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// First get - cache miss
	_, err = cache.Get(filePath)
	if err != nil {
		t.Fatalf("Failed to get file metadata: %v", err)
	}

	stats := cache.Stats()
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}

	// Modify file (touch it to update mtime)
	time.Sleep(10 * time.Millisecond) // Ensure time difference
	currentTime := time.Now()
	if err := os.Chtimes(filePath, currentTime, currentTime); err != nil {
		t.Fatalf("Failed to update file mtime: %v", err)
	}

	// Get again - should invalidate cache and reload
	_, err = cache.Get(filePath)
	if err != nil {
		t.Fatalf("Failed to get file metadata after modification: %v", err)
	}

	stats = cache.Stats()
	if stats.Misses != 2 {
		t.Errorf("Expected 2 misses after invalidation, got %d", stats.Misses)
	}
	if stats.Invalidations != 1 {
		t.Errorf("Expected 1 invalidation, got %d", stats.Invalidations)
	}
}

func TestFileMetadataCache_Eviction(t *testing.T) {
	logging.Initialize("debug")

	// Create temp directory
	tempDir := t.TempDir()

	// Create small cache (1KB)
	cache, err := NewFileMetadataCache(1, logging.GetLogger("test"))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Create multiple test files
	numFiles := 5
	for i := 0; i < numFiles; i++ {
		filePath := filepath.Join(tempDir, "test_"+string(rune('0'+i))+".bin")
		createTestBlockFile(t, filePath)
		
		_, err := cache.Get(filePath)
		if err != nil {
			t.Fatalf("Failed to get file metadata for file %d: %v", i, err)
		}
	}

	// Check that we had cache misses for all files
	stats := cache.Stats()
	if stats.Misses < uint64(numFiles) {
		t.Errorf("Expected at least %d misses, got %d", numFiles, stats.Misses)
	}

	// Memory should not exceed limit significantly
	maxMemoryBytes := int64(1 * 1024 * 1024) // 1MB
	if stats.UsedMemory > maxMemoryBytes*2 {
		t.Errorf("Memory usage too high: %d bytes (max: %d)", stats.UsedMemory, maxMemoryBytes*2)
	}
}

func TestFileMetadataCache_Stats(t *testing.T) {
	logging.Initialize("debug")

	cache, err := NewFileMetadataCache(10, logging.GetLogger("test"))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Initial stats
	stats := cache.Stats()
	if stats.Hits != 0 {
		t.Errorf("Expected 0 hits initially, got %d", stats.Hits)
	}
	if stats.Misses != 0 {
		t.Errorf("Expected 0 misses initially, got %d", stats.Misses)
	}
	if stats.HitRate != 0 {
		t.Errorf("Expected 0 hit rate initially, got %f", stats.HitRate)
	}
	if stats.MaxMemory != 10*1024*1024 {
		t.Errorf("Expected max memory 10MB, got %d", stats.MaxMemory)
	}
}

func TestFileMetadataCache_Clear(t *testing.T) {
	logging.Initialize("debug")

	// Create temp directory
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.bin")

	// Create a test file
	createTestBlockFile(t, filePath)

	// Create cache
	cache, err := NewFileMetadataCache(10, logging.GetLogger("test"))
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Get file to populate cache
	_, err = cache.Get(filePath)
	if err != nil {
		t.Fatalf("Failed to get file metadata: %v", err)
	}

	// Clear cache
	cache.Clear()

	// Check stats are reset
	stats := cache.Stats()
	if stats.Hits != 0 {
		t.Errorf("Expected 0 hits after clear, got %d", stats.Hits)
	}
	if stats.Misses != 0 {
		t.Errorf("Expected 0 misses after clear, got %d", stats.Misses)
	}
	if stats.UsedMemory != 0 {
		t.Errorf("Expected 0 used memory after clear, got %d", stats.UsedMemory)
	}

	// Get again should be a miss
	_, err = cache.Get(filePath)
	if err != nil {
		t.Fatalf("Failed to get file metadata after clear: %v", err)
	}

	stats = cache.Stats()
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss after clear and get, got %d", stats.Misses)
	}
}

// createTestBlockFile creates a simple block storage file for testing
func createTestBlockFile(t *testing.T, filePath string) {
	t.Helper()

	// Create block storage file
	hourTimestamp := time.Date(2025, 12, 16, 20, 0, 0, 0, time.UTC).Unix()
	bsf, err := NewBlockStorageFile(filePath, hourTimestamp, DefaultBlockSize)
	if err != nil {
		t.Fatalf("Failed to create block storage file: %v", err)
	}

	// Write a test event
	event := &models.Event{
		ID:        "test-event-1",
		Timestamp: hourTimestamp * 1e9,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Group:     "",
			Version:   "v1",
			Kind:      "Pod",
			Namespace: "default",
			Name:      "test-pod",
		},
	}

	if err := bsf.WriteEvent(event); err != nil {
		t.Fatalf("Failed to write event: %v", err)
	}

	// Close file to finalize it
	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close file: %v", err)
	}
}
