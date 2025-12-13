package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/logging"
)

const (
	kindPod = "Pod"
)

func TestNewBlockReader(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")

	// Create a file first
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	file.Close()

	reader, err := NewBlockReader(filePath)
	if err != nil {
		t.Fatalf("failed to create block reader: %v", err)
	}
	defer reader.Close()

	if reader.filePath != filePath {
		t.Errorf("expected file path %s, got %s", filePath, reader.filePath)
	}

	if reader.file == nil {
		t.Error("expected file to be opened")
	}
}

func TestBlockReaderReadFileHeader(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")

	// Create a file with header
	bsf, err := NewBlockStorageFile(filePath, time.Now().Unix(), DefaultBlockSize)
	if err != nil {
		t.Fatalf("failed to create block storage file: %v", err)
	}
	bsf.Close()

	reader, err := NewBlockReader(filePath)
	if err != nil {
		t.Fatalf("failed to create block reader: %v", err)
	}
	defer reader.Close()

	header, err := reader.ReadFileHeader()
	if err != nil {
		t.Fatalf("failed to read file header: %v", err)
	}

	if header.MagicBytes != FileHeaderMagic {
		t.Errorf("expected magic bytes %s, got %s", FileHeaderMagic, header.MagicBytes)
	}

	if header.FormatVersion != DefaultFormatVersion {
		t.Errorf("expected format version %s, got %s", DefaultFormatVersion, header.FormatVersion)
	}
}

func TestBlockReaderReadFileFooter(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")

	// Create a file with events and close it
	bsf, err := NewBlockStorageFile(filePath, time.Now().Unix(), DefaultBlockSize)
	if err != nil {
		t.Fatalf("failed to create block storage file: %v", err)
	}

	event := createTestEvent("pod-1", "default", kindPod, time.Now().UnixNano())
	bsf.WriteEvent(event)
	bsf.Close()

	reader, err := NewBlockReader(filePath)
	if err != nil {
		t.Fatalf("failed to create block reader: %v", err)
	}
	defer reader.Close()

	footer, err := reader.ReadFileFooter()
	if err != nil {
		t.Fatalf("failed to read file footer: %v", err)
	}

	if footer.MagicBytes != FileFooterMagic {
		t.Errorf("expected magic bytes %s, got %s", FileFooterMagic, footer.MagicBytes)
	}

	if footer.IndexSectionOffset <= 0 {
		t.Error("expected positive index section offset")
	}
}

func TestBlockReaderReadIndexSection(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")

	// Create a file with events
	bsf, err := NewBlockStorageFile(filePath, time.Now().Unix(), DefaultBlockSize)
	if err != nil {
		t.Fatalf("failed to create block storage file: %v", err)
	}

	event := createTestEvent("pod-1", "default", kindPod, time.Now().UnixNano())
	bsf.WriteEvent(event)
	bsf.Close()

	reader, err := NewBlockReader(filePath)
	if err != nil {
		t.Fatalf("failed to create block reader: %v", err)
	}
	defer reader.Close()

	footer, err := reader.ReadFileFooter()
	if err != nil {
		t.Fatalf("failed to read footer: %v", err)
	}

	indexSection, err := reader.ReadIndexSection(footer.IndexSectionOffset, footer.IndexSectionLength)
	if err != nil {
		t.Fatalf("failed to read index section: %v", err)
	}

	if indexSection.FormatVersion != DefaultFormatVersion {
		t.Errorf("expected format version %s, got %s", DefaultFormatVersion, indexSection.FormatVersion)
	}

	if len(indexSection.BlockMetadata) == 0 {
		t.Error("expected at least one block metadata")
	}
}

func TestBlockReaderReadBlock(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")

	// Create a file with events
	bsf, err := NewBlockStorageFile(filePath, time.Now().Unix(), DefaultBlockSize)
	if err != nil {
		t.Fatalf("failed to create block storage file: %v", err)
	}

	event := createTestEvent("pod-1", "default", kindPod, time.Now().UnixNano())
	bsf.WriteEvent(event)
	bsf.Close()

	reader, err := NewBlockReader(filePath)
	if err != nil {
		t.Fatalf("failed to create block reader: %v", err)
	}
	defer reader.Close()

	// Get block metadata from index
	footer, _ := reader.ReadFileFooter()
	indexSection, _ := reader.ReadIndexSection(footer.IndexSectionOffset, footer.IndexSectionLength)

	if len(indexSection.BlockMetadata) == 0 {
		t.Fatal("expected at least one block metadata")
	}

	metadata := indexSection.BlockMetadata[0]
	data, err := reader.ReadBlock(metadata)
	if err != nil {
		t.Fatalf("failed to read block: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected block data")
	}
}

func TestBlockReaderReadBlockEvents(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")

	// Create a file with events
	bsf, err := NewBlockStorageFile(filePath, time.Now().Unix(), DefaultBlockSize)
	if err != nil {
		t.Fatalf("failed to create block storage file: %v", err)
	}

	event := createTestEvent("pod-1", "default", kindPod, time.Now().UnixNano())
	bsf.WriteEvent(event)
	bsf.Close()

	reader, err := NewBlockReader(filePath)
	if err != nil {
		t.Fatalf("failed to create block reader: %v", err)
	}
	defer reader.Close()

	// Get block metadata
	footer, _ := reader.ReadFileFooter()
	indexSection, _ := reader.ReadIndexSection(footer.IndexSectionOffset, footer.IndexSectionLength)

	if len(indexSection.BlockMetadata) == 0 {
		t.Fatal("expected at least one block metadata")
	}

	metadata := indexSection.BlockMetadata[0]
	events, err := reader.ReadBlockEvents(metadata)
	if err != nil {
		t.Fatalf("failed to read block events: %v", err)
	}

	if len(events) == 0 {
		t.Error("expected at least one event")
	}

	if events[0].Resource.Kind != kindPod {
		t.Errorf("expected Pod, got %s", events[0].Resource.Kind)
	}
}

func TestBlockReaderReadFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")

	// Create a file with events
	bsf, err := NewBlockStorageFile(filePath, time.Now().Unix(), DefaultBlockSize)
	if err != nil {
		t.Fatalf("failed to create block storage file: %v", err)
	}

	event := createTestEvent("pod-1", "default", kindPod, time.Now().UnixNano())
	bsf.WriteEvent(event)
	bsf.Close()

	reader, err := NewBlockReader(filePath)
	if err != nil {
		t.Fatalf("failed to create block reader: %v", err)
	}
	defer reader.Close()

	fileData, err := reader.ReadFile()
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if fileData.Header == nil {
		t.Error("expected file header")
	}

	if fileData.Footer == nil {
		t.Error("expected file footer")
	}

	if fileData.IndexSection == nil {
		t.Error("expected index section")
	}
}

func TestStorageFileDataGetEvents(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")

	// Create a file with events
	bsf, err := NewBlockStorageFile(filePath, time.Now().Unix(), DefaultBlockSize)
	if err != nil {
		t.Fatalf("failed to create block storage file: %v", err)
	}

	event1 := createTestEvent("pod-1", "default", kindPod, time.Now().UnixNano())
	event2 := createTestEvent("svc-1", "default", "Service", time.Now().UnixNano())
	bsf.WriteEvent(event1)
	bsf.WriteEvent(event2)
	bsf.Close()

	reader, err := NewBlockReader(filePath)
	if err != nil {
		t.Fatalf("failed to create block reader: %v", err)
	}
	defer reader.Close()

	fileData, err := reader.ReadFile()
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	// Get all events
	events, err := fileData.GetEvents(map[string]string{})
	if err != nil {
		t.Fatalf("failed to get events: %v", err)
	}

	if len(events) < 2 {
		t.Errorf("expected at least 2 events, got %d", len(events))
	}

	// Get events with filter
	events, err = fileData.GetEvents(map[string]string{"kind": kindPod})
	if err != nil {
		t.Fatalf("failed to get filtered events: %v", err)
	}

	if len(events) == 0 {
		t.Error("expected at least one Pod event")
	}

	// Verify all returned events match filter
	for _, event := range events {
		if event.Resource.Kind != kindPod {
			t.Errorf("expected Pod, got %s", event.Resource.Kind)
		}
	}
}

func TestVerifyBlockChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")

	// Create a file with events
	bsf, err := NewBlockStorageFile(filePath, time.Now().Unix(), DefaultBlockSize)
	if err != nil {
		t.Fatalf("failed to create block storage file: %v", err)
	}

	event := createTestEvent("pod-1", "default", kindPod, time.Now().UnixNano())
	bsf.WriteEvent(event)
	bsf.Close()

	// Get a block
	blocks := bsf.GetBlocks()
	if len(blocks) == 0 {
		t.Fatal("expected at least one block")
	}

	block := blocks[0]
	metadata := block.Metadata

	// Verify checksum (if enabled)
	if metadata.Checksum != "" {
		if err := VerifyBlockChecksum(block, metadata); err != nil {
			t.Errorf("checksum verification failed: %v", err)
		}
	}
}

func TestComputeChecksum(t *testing.T) {
	data := []byte("test data")
	checksum1 := ComputeChecksum(data)
	checksum2 := ComputeChecksum(data)

	if checksum1 != checksum2 {
		t.Error("expected same checksum for same data")
	}

	if checksum1 == "" {
		t.Error("expected non-empty checksum")
	}

	// Different data should have different checksum
	otherData := []byte("different data")
	checksum3 := ComputeChecksum(otherData)
	if checksum1 == checksum3 {
		t.Error("expected different checksum for different data")
	}
}

func TestBlockReaderReadBlockWithCache_CacheHit(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	filename := "test.bin"

	// Create a file with events
	bsf, err := NewBlockStorageFile(filePath, time.Now().Unix(), DefaultBlockSize)
	if err != nil {
		t.Fatalf("failed to create block storage file: %v", err)
	}

	event := createTestEvent("pod-1", "default", kindPod, time.Now().UnixNano())
	bsf.WriteEvent(event)
	bsf.Close()

	reader, err := NewBlockReader(filePath)
	if err != nil {
		t.Fatalf("failed to create block reader: %v", err)
	}
	defer reader.Close()

	// Get block metadata
	footer, err := reader.ReadFileFooter()
	if err != nil {
		t.Fatalf("failed to read footer: %v", err)
	}

	indexSection, err := reader.ReadIndexSection(footer.IndexSectionOffset, footer.IndexSectionLength)
	if err != nil {
		t.Fatalf("failed to read index section: %v", err)
	}

	if len(indexSection.BlockMetadata) == 0 {
		t.Fatal("expected at least one block metadata")
	}

	metadata := indexSection.BlockMetadata[0]

	// Create cache and pre-populate it with a cached block
	cache, err := NewBlockCache(100, logging.GetLogger("test")) // 100MB cache
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Read the block first to populate cache
	firstRead, err := reader.ReadBlockWithCache(filename, metadata, cache)
	if err != nil {
		t.Fatalf("failed to read block with cache (first read): %v", err)
	}

	// Verify cache stats show a miss for first read
	stats := cache.Stats()
	if stats.Misses == 0 {
		t.Error("expected at least one cache miss for first read")
	}

	// Now read again - this should be a cache hit
	secondRead, err := reader.ReadBlockWithCache(filename, metadata, cache)
	if err != nil {
		t.Fatalf("failed to read block with cache (second read): %v", err)
	}

	// Verify it's the same block (same pointer or same content)
	if secondRead.BlockID != firstRead.BlockID {
		t.Errorf("expected same block ID, got %s vs %s", secondRead.BlockID, firstRead.BlockID)
	}

	if len(secondRead.Events) != len(firstRead.Events) {
		t.Errorf("expected same number of events, got %d vs %d", len(secondRead.Events), len(firstRead.Events))
	}

	// Verify cache stats show a hit for second read
	stats = cache.Stats()
	if stats.Hits == 0 {
		t.Error("expected at least one cache hit for second read")
	}

	// Verify the events are the same
	if len(secondRead.Events) > 0 && len(firstRead.Events) > 0 {
		if secondRead.Events[0].Resource.Kind != firstRead.Events[0].Resource.Kind {
			t.Errorf("expected same event kind, got %s vs %s",
				secondRead.Events[0].Resource.Kind, firstRead.Events[0].Resource.Kind)
		}
	}
}

func TestBlockReaderReadBlockWithCache_CacheMiss(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	filename := "test.bin"

	// Create a file with events
	bsf, err := NewBlockStorageFile(filePath, time.Now().Unix(), DefaultBlockSize)
	if err != nil {
		t.Fatalf("failed to create block storage file: %v", err)
	}

	event := createTestEvent("pod-1", "default", kindPod, time.Now().UnixNano())
	bsf.WriteEvent(event)
	bsf.Close()

	reader, err := NewBlockReader(filePath)
	if err != nil {
		t.Fatalf("failed to create block reader: %v", err)
	}
	defer reader.Close()

	// Get block metadata
	footer, err := reader.ReadFileFooter()
	if err != nil {
		t.Fatalf("failed to read footer: %v", err)
	}

	indexSection, err := reader.ReadIndexSection(footer.IndexSectionOffset, footer.IndexSectionLength)
	if err != nil {
		t.Fatalf("failed to read index section: %v", err)
	}

	if len(indexSection.BlockMetadata) == 0 {
		t.Fatal("expected at least one block metadata")
	}

	metadata := indexSection.BlockMetadata[0]

	// Create empty cache
	cache, err := NewBlockCache(100, logging.GetLogger("test")) // 100MB cache
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Verify cache is empty
	stats := cache.Stats()
	if stats.Items != 0 {
		t.Errorf("expected empty cache, got %d items", stats.Items)
	}

	// Read block with cache - should be a cache miss
	cachedBlock, err := reader.ReadBlockWithCache(filename, metadata, cache)
	if err != nil {
		t.Fatalf("failed to read block with cache: %v", err)
	}

	// Verify the block was returned
	if cachedBlock == nil {
		t.Fatal("expected non-nil cached block")
	}

	// Verify block properties
	if cachedBlock.BlockID != makeKey(filename, metadata.ID) {
		t.Errorf("expected block ID %s, got %s", makeKey(filename, metadata.ID), cachedBlock.BlockID)
	}

	if cachedBlock.Filename != filename {
		t.Errorf("expected filename %s, got %s", filename, cachedBlock.Filename)
	}

	if cachedBlock.ID != metadata.ID {
		t.Errorf("expected block ID %d, got %d", metadata.ID, cachedBlock.ID)
	}

	if cachedBlock.Metadata != metadata {
		t.Error("expected metadata to match")
	}

	// Verify events were parsed
	if len(cachedBlock.Events) == 0 {
		t.Error("expected at least one event")
	}

	if cachedBlock.Events[0].Resource.Kind != kindPod {
		t.Errorf("expected Pod, got %s", cachedBlock.Events[0].Resource.Kind)
	}

	// Verify block was stored in cache
	stats = cache.Stats()
	if stats.Items != 1 {
		t.Errorf("expected 1 item in cache, got %d", stats.Items)
	}

	if stats.Misses == 0 {
		t.Error("expected at least one cache miss")
	}

	// Verify we can retrieve it from cache directly
	retrieved := cache.Get(filename, metadata.ID)
	if retrieved == nil {
		t.Fatal("expected to retrieve block from cache")
	}

	if retrieved.BlockID != cachedBlock.BlockID {
		t.Errorf("expected same block ID, got %s vs %s", retrieved.BlockID, cachedBlock.BlockID)
	}

	if len(retrieved.Events) != len(cachedBlock.Events) {
		t.Errorf("expected same number of events, got %d vs %d", len(retrieved.Events), len(cachedBlock.Events))
	}
}
