package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewBlockStorageFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	hourTimestamp := time.Now().Unix()

	bsf, err := NewBlockStorageFile(filePath, hourTimestamp, DefaultBlockSize)
	if err != nil {
		t.Fatalf("failed to create block storage file: %v", err)
	}
	defer bsf.Close()

	if bsf.path != filePath {
		t.Errorf("expected path %s, got %s", filePath, bsf.path)
	}

	if bsf.hourTimestamp != hourTimestamp {
		t.Errorf("expected hour timestamp %d, got %d", hourTimestamp, bsf.hourTimestamp)
	}

	if bsf.blockSize != DefaultBlockSize {
		t.Errorf("expected block size %d, got %d", DefaultBlockSize, bsf.blockSize)
	}

	if bsf.blockID != 0 {
		t.Errorf("expected initial block ID 0, got %d", bsf.blockID)
	}

	// Verify file was created
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("expected file to be created")
	}
}

func TestBlockStorageFileWriteEvent(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	hourTimestamp := time.Now().Unix()

	bsf, err := NewBlockStorageFile(filePath, hourTimestamp, DefaultBlockSize)
	if err != nil {
		t.Fatalf("failed to create block storage file: %v", err)
	}
	defer bsf.Close()

	event := createTestEvent("pod-1", "default", "Pod", time.Now().UnixNano())

	if err := bsf.WriteEvent(event); err != nil {
		t.Fatalf("failed to write event: %v", err)
	}

	if bsf.totalEvents != 1 {
		t.Errorf("expected 1 total event, got %d", bsf.totalEvents)
	}
}

func TestBlockStorageFileWriteMultipleEvents(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	hourTimestamp := time.Now().Unix()

	bsf, err := NewBlockStorageFile(filePath, hourTimestamp, DefaultBlockSize)
	if err != nil {
		t.Fatalf("failed to create block storage file: %v", err)
	}
	defer bsf.Close()

	// Write multiple events
	for i := 0; i < 10; i++ {
		event := createTestEvent("pod-"+string(rune(i)), "default", "Pod", time.Now().UnixNano())
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event %d: %v", i, err)
		}
	}

	if bsf.totalEvents != 10 {
		t.Errorf("expected 10 total events, got %d", bsf.totalEvents)
	}
}

func TestBlockStorageFileFinalizeBlock(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	hourTimestamp := time.Now().Unix()

	bsf, err := NewBlockStorageFile(filePath, hourTimestamp, 1024) // Small block size
	if err != nil {
		t.Fatalf("failed to create block storage file: %v", err)
	}
	defer bsf.Close()

	// Write events until block is full
	eventCount := 0
	for i := 0; i < 100; i++ {
		event := createTestEvent("pod-"+string(rune(i)), "default", "Pod", time.Now().UnixNano())
		if err := bsf.WriteEvent(event); err != nil {
			break
		}
		eventCount++
	}

	// Close to finalize last block
	if err := bsf.Close(); err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	blocks := bsf.GetBlocks()
	if len(blocks) == 0 {
		t.Error("expected at least one block")
	}
}

func TestBlockStorageFileClose(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	hourTimestamp := time.Now().Unix()

	bsf, err := NewBlockStorageFile(filePath, hourTimestamp, DefaultBlockSize)
	if err != nil {
		t.Fatalf("failed to create block storage file: %v", err)
	}

	// Write some events
	for i := 0; i < 5; i++ {
		event := createTestEvent("pod-"+string(rune(i)), "default", "Pod", time.Now().UnixNano())
		bsf.WriteEvent(event)
	}

	// Close should finalize blocks and write index
	if err := bsf.Close(); err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	// Verify file exists and has content
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}

	if info.Size() == 0 {
		t.Error("expected file to have content")
	}

	// Verify index was built
	index := bsf.GetInvertedIndex()
	if index == nil {
		t.Error("expected inverted index to be built")
	}
}

func TestBlockStorageFileBuildIndexes(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	hourTimestamp := time.Now().Unix()

	bsf, err := NewBlockStorageFile(filePath, hourTimestamp, DefaultBlockSize)
	if err != nil {
		t.Fatalf("failed to create block storage file: %v", err)
	}
	defer bsf.Close()

	// Write events with different kinds and namespaces
	events := []struct {
		name      string
		namespace string
		kind      string
	}{
		{"pod-1", "default", "Pod"},
		{"svc-1", "default", "Service"},
		{"pod-2", "kube-system", "Pod"},
	}

	for _, e := range events {
		event := createTestEvent(e.name, e.namespace, e.kind, time.Now().UnixNano())
		bsf.WriteEvent(event)
	}

	// Close to build indexes
	bsf.Close()

	index := bsf.GetInvertedIndex()
	if index == nil {
		t.Fatal("expected inverted index")
	}

	// Verify index has entries
	if len(index.KindToBlocks) == 0 {
		t.Error("expected kind index to have entries")
	}
	if len(index.NamespaceToBlocks) == 0 {
		t.Error("expected namespace index to have entries")
	}
}

func TestBlockStorageFileGetMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	hourTimestamp := time.Now().Unix()

	bsf, err := NewBlockStorageFile(filePath, hourTimestamp, DefaultBlockSize)
	if err != nil {
		t.Fatalf("failed to create block storage file: %v", err)
	}
	defer bsf.Close()

	// Write some events
	for i := 0; i < 5; i++ {
		event := createTestEvent("pod-"+string(rune(i)), "default", "Pod", time.Now().UnixNano())
		bsf.WriteEvent(event)
	}

	metadata := bsf.GetMetadata()
	if metadata.TotalEvents != 5 {
		t.Errorf("expected 5 total events, got %d", metadata.TotalEvents)
	}
}

func TestBlockStorageFileGetIndex(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	hourTimestamp := time.Now().Unix()

	bsf, err := NewBlockStorageFile(filePath, hourTimestamp, DefaultBlockSize)
	if err != nil {
		t.Fatalf("failed to create block storage file: %v", err)
	}
	defer bsf.Close()

	// Write some events
	for i := 0; i < 5; i++ {
		event := createTestEvent("pod-"+string(rune(i)), "default", "Pod", time.Now().UnixNano())
		bsf.WriteEvent(event)
	}

	bsf.Close()

	index := bsf.GetIndex()
	if index.TotalSegments == 0 {
		t.Error("expected at least one segment in index")
	}
}

func TestBlockStorageFileGetCompressionStats(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	hourTimestamp := time.Now().Unix()

	bsf, err := NewBlockStorageFile(filePath, hourTimestamp, DefaultBlockSize)
	if err != nil {
		t.Fatalf("failed to create block storage file: %v", err)
	}
	defer bsf.Close()

	// Write some events
	for i := 0; i < 10; i++ {
		event := createTestEvent("pod-"+string(rune(i)), "default", "Pod", time.Now().UnixNano())
		bsf.WriteEvent(event)
	}

	bsf.Close()

	stats := bsf.GetCompressionStats()
	if stats["total_events"].(int64) != 10 {
		t.Errorf("expected 10 total events, got %d", stats["total_events"])
	}

	if stats["block_count"].(int) == 0 {
		t.Error("expected at least one block")
	}
}

func TestBlockStorageFileMultipleBlocks(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	hourTimestamp := time.Now().Unix()

	// Use small block size to force multiple blocks
	bsf, err := NewBlockStorageFile(filePath, hourTimestamp, 1024)
	if err != nil {
		t.Fatalf("failed to create block storage file: %v", err)
	}
	defer bsf.Close()

	// Write many events to fill multiple blocks
	eventCount := 0
	for i := 0; i < 1000; i++ {
		event := createTestEvent("pod-"+string(rune(i)), "default", "Pod", time.Now().UnixNano())
		if err := bsf.WriteEvent(event); err != nil {
			// Event might be too large, continue
			continue
		}
		eventCount++
	}

	bsf.Close()

	blocks := bsf.GetBlocks()
	if len(blocks) == 0 {
		t.Error("expected at least one block")
	}

	if bsf.totalEvents != int64(eventCount) {
		t.Errorf("expected %d total events, got %d", eventCount, bsf.totalEvents)
	}
}


