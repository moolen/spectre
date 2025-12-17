package storage

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

func TestSharedFileDataCache_Basic(t *testing.T) {
	logging.Initialize("debug")

	cache := NewSharedFileDataCache()

	if cache.Size() != 0 {
		t.Errorf("Expected empty cache, got size %d", cache.Size())
	}

	// Create test file data
	testData := &StorageFileData{
		Header: &FileHeader{MagicBytes: "TEST"},
		Footer: &FileFooter{},
		IndexSection: &IndexSection{
			BlockMetadata: []*BlockMetadata{},
		},
	}

	// Load file data
	loadCount := 0
	loader := func() (*StorageFileData, error) {
		loadCount++
		return testData, nil
	}

	// First call should load
	data1, err := cache.GetOrLoad("/test/file1.bin", loader)
	if err != nil {
		t.Fatalf("Failed to load file data: %v", err)
	}
	if data1 == nil {
		t.Fatal("Expected file data, got nil")
	}
	if loadCount != 1 {
		t.Errorf("Expected loader to be called once, was called %d times", loadCount)
	}

	// Second call should use cache
	data2, err := cache.GetOrLoad("/test/file1.bin", loader)
	if err != nil {
		t.Fatalf("Failed to get cached file data: %v", err)
	}
	if data2 != data1 {
		t.Error("Expected same data from cache")
	}
	if loadCount != 1 {
		t.Errorf("Expected loader to be called once (cached), was called %d times", loadCount)
	}

	if cache.Size() != 1 {
		t.Errorf("Expected cache size 1, got %d", cache.Size())
	}
}

func TestSharedFileDataCache_ConcurrentLoads(t *testing.T) {
	logging.Initialize("debug")

	cache := NewSharedFileDataCache()
	
	var loadCount int32
	var wg sync.WaitGroup
	
	loader := func() (*StorageFileData, error) {
		atomic.AddInt32(&loadCount, 1)
		// Simulate slow load
		time.Sleep(50 * time.Millisecond)
		return &StorageFileData{
			Header: &FileHeader{MagicBytes: "TEST"},
			Footer: &FileFooter{},
			IndexSection: &IndexSection{
				BlockMetadata: []*BlockMetadata{},
			},
		}, nil
	}

	// Launch 10 concurrent goroutines trying to load the same file
	numGoroutines := 10
	wg.Add(numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			_, err := cache.GetOrLoad("/test/file1.bin", loader)
			if err != nil {
				t.Errorf("Failed to load file: %v", err)
			}
		}()
	}
	
	wg.Wait()
	
	// Loader should only be called once despite 10 concurrent requests
	finalLoadCount := atomic.LoadInt32(&loadCount)
	if finalLoadCount != 1 {
		t.Errorf("Expected loader to be called once, was called %d times", finalLoadCount)
	}
	
	if cache.Size() != 1 {
		t.Errorf("Expected cache size 1, got %d", cache.Size())
	}
}

func TestSharedFileDataCache_MultipleFiles(t *testing.T) {
	logging.Initialize("debug")

	cache := NewSharedFileDataCache()
	
	numFiles := 5
	for i := 0; i < numFiles; i++ {
		filePath := fmt.Sprintf("/test/file%d.bin", i)
		
		_, err := cache.GetOrLoad(filePath, func() (*StorageFileData, error) {
			return &StorageFileData{
				Header: &FileHeader{MagicBytes: fmt.Sprintf("FILE%d", i)},
				Footer: &FileFooter{},
				IndexSection: &IndexSection{
					BlockMetadata: []*BlockMetadata{},
				},
			}, nil
		})
		
		if err != nil {
			t.Fatalf("Failed to load file %s: %v", filePath, err)
		}
	}
	
	if cache.Size() != numFiles {
		t.Errorf("Expected cache size %d, got %d", numFiles, cache.Size())
	}
	
	// Verify all files are cached
	for i := 0; i < numFiles; i++ {
		filePath := fmt.Sprintf("/test/file%d.bin", i)
		
		loadCount := 0
		data, err := cache.GetOrLoad(filePath, func() (*StorageFileData, error) {
			loadCount++
			return nil, fmt.Errorf("should not be called")
		})
		
		if err != nil {
			t.Fatalf("Failed to get cached file %s: %v", filePath, err)
		}
		if data == nil {
			t.Errorf("Expected file data for %s, got nil", filePath)
		}
		if loadCount != 0 {
			t.Errorf("Loader should not be called for cached file, was called %d times", loadCount)
		}
	}
}

func TestSharedFileDataCache_LoadError(t *testing.T) {
	logging.Initialize("debug")

	cache := NewSharedFileDataCache()
	
	expectedError := fmt.Errorf("load failed")
	
	_, err := cache.GetOrLoad("/test/file1.bin", func() (*StorageFileData, error) {
		return nil, expectedError
	})
	
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if err != expectedError {
		t.Errorf("Expected error %v, got %v", expectedError, err)
	}
	
	// Error shouldn't be cached
	if cache.Size() != 0 {
		t.Errorf("Expected cache size 0 after error, got %d", cache.Size())
	}
	
	// Next call should try to load again
	loadCount := 0
	_, _ = cache.GetOrLoad("/test/file1.bin", func() (*StorageFileData, error) {
		loadCount++
		return nil, fmt.Errorf("second attempt")
	})
	
	if loadCount != 1 {
		t.Errorf("Expected loader to be called on retry, was called %d times", loadCount)
	}
}

func TestSharedFileDataCache_Clear(t *testing.T) {
	logging.Initialize("debug")

	cache := NewSharedFileDataCache()
	
	// Add some files
	for i := 0; i < 3; i++ {
		filePath := fmt.Sprintf("/test/file%d.bin", i)
		_, _ = cache.GetOrLoad(filePath, func() (*StorageFileData, error) {
			return &StorageFileData{
				Header: &FileHeader{},
				Footer: &FileFooter{},
				IndexSection: &IndexSection{},
			}, nil
		})
	}
	
	if cache.Size() != 3 {
		t.Errorf("Expected cache size 3, got %d", cache.Size())
	}
	
	// Clear cache
	cache.Clear()
	
	if cache.Size() != 0 {
		t.Errorf("Expected cache size 0 after clear, got %d", cache.Size())
	}
	
	// Files should need to be loaded again
	loadCount := 0
	_, _ = cache.GetOrLoad("/test/file0.bin", func() (*StorageFileData, error) {
		loadCount++
		return &StorageFileData{
			Header: &FileHeader{},
			Footer: &FileFooter{},
			IndexSection: &IndexSection{},
		}, nil
	})
	
	if loadCount != 1 {
		t.Errorf("Expected loader to be called after clear, was called %d times", loadCount)
	}
}

func TestSharedFileDataCache_RealFileData(t *testing.T) {
	logging.Initialize("debug")

	tempDir := t.TempDir()
	filePath := tempDir + "/test.bin"
	
	// Create a real block storage file
	hourTimestamp := time.Date(2025, 12, 16, 20, 0, 0, 0, time.UTC).Unix()
	bsf, err := NewBlockStorageFile(filePath, hourTimestamp, DefaultBlockSize)
	if err != nil {
		t.Fatalf("Failed to create block storage file: %v", err)
	}
	
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
	
	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close file: %v", err)
	}
	
	// Use shared cache to load file
	cache := NewSharedFileDataCache()
	
	loadCount := 0
	loader := func() (*StorageFileData, error) {
		loadCount++
		reader, err := NewBlockReader(filePath)
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return reader.ReadFile()
	}
	
	// First load
	data1, err := cache.GetOrLoad(filePath, loader)
	if err != nil {
		t.Fatalf("Failed to load file: %v", err)
	}
	if data1 == nil {
		t.Fatal("Expected file data, got nil")
	}
	if len(data1.IndexSection.BlockMetadata) != 1 {
		t.Errorf("Expected 1 block, got %d", len(data1.IndexSection.BlockMetadata))
	}
	if loadCount != 1 {
		t.Errorf("Expected 1 load, got %d", loadCount)
	}
	
	// Second load (cached)
	data2, err := cache.GetOrLoad(filePath, loader)
	if err != nil {
		t.Fatalf("Failed to get cached file: %v", err)
	}
	if data2 != data1 {
		t.Error("Expected same data from cache")
	}
	if loadCount != 1 {
		t.Errorf("Expected 1 load (cached), got %d", loadCount)
	}
}
