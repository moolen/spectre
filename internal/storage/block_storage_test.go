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

	index := bsf.GetSparseTimestampIndex()
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

// TestBlockStorageFileRestoreFromCompleteFile tests restoring state from a gracefully closed file
// This simulates a graceful shutdown followed by restart in the same hour
func TestBlockStorageFileRestoreFromCompleteFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	hourTimestamp := time.Now().Unix()

	// Phase 1: Create file, write events, and close gracefully (simulating graceful shutdown)
	t.Log("Phase 1: Creating file and writing initial events")
	bsf1, err := NewBlockStorageFile(filePath, hourTimestamp, DefaultBlockSize)
	if err != nil {
		t.Fatalf("failed to create block storage file: %v", err)
	}

	// Write some events
	initialEventCount := 5
	eventNames := make([]string, initialEventCount)
	for i := 0; i < initialEventCount; i++ {
		eventName := "pod-" + string(rune('a'+i))
		eventNames[i] = eventName
		event := createTestEvent(eventName, "default", "Pod", time.Now().Add(time.Duration(i)*time.Second).UnixNano())
		if err := bsf1.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event %d: %v", i, err)
		}
	}

	// Verify initial state before close
	if bsf1.totalEvents != int64(initialEventCount) {
		t.Errorf("expected %d total events before close, got %d", initialEventCount, bsf1.totalEvents)
	}

	// Close gracefully (simulating graceful shutdown) - this finalizes blocks
	if err := bsf1.Close(); err != nil {
		t.Fatalf("failed to close file: %v", err)
	}

	// Read the file to get the actual final state after close
	reader1, err := NewBlockReader(filePath)
	if err != nil {
		t.Fatalf("failed to create block reader: %v", err)
	}
	fileData1, err := reader1.ReadFile()
	reader1.Close()
	if err != nil {
		t.Fatalf("failed to read closed file: %v", err)
	}

	// Get initial state from the closed file
	initialBlockCount := len(fileData1.IndexSection.BlockMetadata)
	initialStats := fileData1.IndexSection.Statistics
	initialTotalUncompressed := initialStats.TotalUncompressedBytes
	initialTotalCompressed := initialStats.TotalCompressedBytes

	t.Logf("Closed file with %d events, %d blocks", initialEventCount, initialBlockCount)

	// Phase 2: Reopen the file and verify state is restored
	t.Log("Phase 2: Reopening file and verifying state restoration")
	bsf2, err := NewBlockStorageFile(filePath, hourTimestamp, DefaultBlockSize)
	if err != nil {
		t.Fatalf("failed to reopen block storage file: %v", err)
	}
	defer bsf2.Close()

	// Verify state is restored
	if len(bsf2.blockMetadataList) != initialBlockCount {
		t.Errorf("expected %d restored blocks, got %d", initialBlockCount, len(bsf2.blockMetadataList))
	}

	if bsf2.totalEvents != int64(initialEventCount) {
		t.Errorf("expected %d restored events, got %d", initialEventCount, bsf2.totalEvents)
	}

	if bsf2.totalUncompressed != initialTotalUncompressed {
		t.Errorf("expected %d restored uncompressed bytes, got %d", initialTotalUncompressed, bsf2.totalUncompressed)
	}

	if bsf2.totalCompressed != initialTotalCompressed {
		t.Errorf("expected %d restored compressed bytes, got %d", initialTotalCompressed, bsf2.totalCompressed)
	}

	// Verify next block ID continues from where we left off
	expectedNextBlockID := int32(initialBlockCount)
	if bsf2.blockID != expectedNextBlockID {
		t.Errorf("expected next block ID %d, got %d", expectedNextBlockID, bsf2.blockID)
	}

	// Phase 3: Write new events and verify they're appended
	t.Log("Phase 3: Writing new events after restoration")
	additionalEventCount := 3
	for i := 0; i < additionalEventCount; i++ {
		eventName := "pod-" + string(rune('x'+i))
		event := createTestEvent(eventName, "default", "Pod", time.Now().Add(time.Duration(initialEventCount+i)*time.Second).UnixNano())
		if err := bsf2.WriteEvent(event); err != nil {
			t.Fatalf("failed to write new event %d: %v", i, err)
		}
	}

	// Verify total events includes both old and new
	expectedTotalEvents := int64(initialEventCount + additionalEventCount)
	if bsf2.totalEvents != expectedTotalEvents {
		t.Errorf("expected %d total events (old + new), got %d", expectedTotalEvents, bsf2.totalEvents)
	}

	// Phase 4: Close and verify file can be read completely
	t.Log("Phase 4: Closing and verifying all events are readable")
	if err := bsf2.Close(); err != nil {
		t.Fatalf("failed to close file: %v", err)
	}

	// Read the file and verify all events are present
	reader, err := NewBlockReader(filePath)
	if err != nil {
		t.Fatalf("failed to create block reader: %v", err)
	}
	defer reader.Close()

	fileData, err := reader.ReadFile()
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	allEvents, err := fileData.GetEvents(map[string]string{})
	if err != nil {
		t.Fatalf("failed to get events: %v", err)
	}

	if len(allEvents) != int(expectedTotalEvents) {
		t.Errorf("expected %d total events in file, got %d", expectedTotalEvents, len(allEvents))
	}

	// Verify event names are present
	eventNameMap := make(map[string]bool)
	for _, event := range allEvents {
		eventNameMap[event.Resource.Name] = true
	}

	// Check initial events
	for _, name := range eventNames {
		if !eventNameMap[name] {
			t.Errorf("initial event %s not found in restored file", name)
		}
	}

	// Check new events
	for i := 0; i < additionalEventCount; i++ {
		eventName := "pod-" + string(rune('x'+i))
		if !eventNameMap[eventName] {
			t.Errorf("new event %s not found in restored file", eventName)
		}
	}

	// Verify statistics match
	stats := fileData.IndexSection.Statistics
	if stats.TotalEvents != expectedTotalEvents {
		t.Errorf("expected %d total events in statistics, got %d", expectedTotalEvents, stats.TotalEvents)
	}

	t.Logf("✓ Successfully restored and appended: %d initial + %d new = %d total events",
		initialEventCount, additionalEventCount, expectedTotalEvents)
}
