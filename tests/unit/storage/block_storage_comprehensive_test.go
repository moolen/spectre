package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
	"github.com/moolen/spectre/internal/storage"
)

func TestBlockStorageFileWriteAndRead(t *testing.T) {
	tmpFile := t.TempDir() + "/test_write_read.bin"
	hourTimestamp := time.Now().Unix()
	blockSize := int64(256 * 1024)

	// Create and write
	bsf, err := storage.NewBlockStorageFile(tmpFile, hourTimestamp, blockSize)
	if err != nil {
		t.Fatalf("Failed to create block storage file: %v", err)
	}

	// Write multiple events
	eventCount := 500
	for i := 0; i < eventCount; i++ {
		event := createTestEvent(i, hourTimestamp)
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event %d: %v", i, err)
		}
	}

	// Close to finalize
	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	// Verify file exists and has content
	fileInfo, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	if fileInfo.Size() == 0 {
		t.Fatal("File is empty")
	}

	// Verify event count
	if bsf.GetEventCount() != int64(eventCount) {
		t.Errorf("Expected %d events, got %d", eventCount, bsf.GetEventCount())
	}

	// Verify blocks were created
	blockCount := bsf.GetSegmentCount()
	if blockCount == 0 {
		t.Fatal("No blocks created")
	}
	t.Logf("Created %d blocks for %d events", blockCount, eventCount)
}

func TestBlockStorageFileMultipleBlocks(t *testing.T) {
	tmpFile := t.TempDir() + "/test_multiple_blocks.bin"
	hourTimestamp := time.Now().Unix()
	blockSize := int64(64 * 1024) // Small block size to force multiple blocks

	bsf, err := storage.NewBlockStorageFile(tmpFile, hourTimestamp, blockSize)
	if err != nil {
		t.Fatalf("Failed to create: %v", err)
	}

	// Write enough events to create multiple blocks
	eventCount := 1000
	for i := 0; i < eventCount; i++ {
		event := createTestEvent(i, hourTimestamp)
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}

	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	blockCount := bsf.GetSegmentCount()
	if blockCount < 2 {
		t.Errorf("Expected at least 2 blocks, got %d", blockCount)
	}

	// Verify all events are accounted for
	if bsf.GetEventCount() != int64(eventCount) {
		t.Errorf("Event count mismatch: %d vs %d", bsf.GetEventCount(), eventCount)
	}
}

func TestBlockStorageFileCompression(t *testing.T) {
	tmpFile := t.TempDir() + "/test_compression.bin"
	hourTimestamp := time.Now().Unix()
	blockSize := int64(256 * 1024)

	bsf, err := storage.NewBlockStorageFile(tmpFile, hourTimestamp, blockSize)
	if err != nil {
		t.Fatalf("Failed to create: %v", err)
	}

	// Write events with varying sizes
	for i := 0; i < 200; i++ {
		event := createTestEventWithLargeData(i, hourTimestamp)
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write: %v", err)
		}
	}

	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	stats := bsf.GetCompressionStats()
	uncompressed := stats["total_uncompressed_bytes"].(int64)
	compressed := stats["total_compressed_bytes"].(int64)
	ratio := stats["compression_ratio"].(float64)

	t.Logf("Compression: %d -> %d (ratio: %.4f)", uncompressed, compressed, ratio)

	// Verify compression is effective
	if ratio >= 1.0 {
		t.Errorf("Compression ratio should be < 1.0, got %.4f", ratio)
	}

	// Verify file size matches compressed size
	fileInfo, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("Failed to stat: %v", err)
	}

	// File size should be approximately compressed size + header + footer + index
	// Allow some margin for overhead
	expectedMinSize := compressed
	if fileInfo.Size() < expectedMinSize {
		t.Errorf("File size %d is less than compressed size %d", fileInfo.Size(), expectedMinSize)
	}
}

func TestBlockStorageFileMetadata(t *testing.T) {
	tmpFile := t.TempDir() + "/test_metadata.bin"
	hourTimestamp := time.Now().Unix()
	blockSize := int64(256 * 1024)

	bsf, err := storage.NewBlockStorageFile(tmpFile, hourTimestamp, blockSize)
	if err != nil {
		t.Fatalf("Failed to create: %v", err)
	}

	// Write events with different kinds and namespaces
	kinds := []string{"Pod", "Deployment", "Service", "ConfigMap"}
	namespaces := []string{"default", "kube-system", "production", "staging"}

	for i := 0; i < 200; i++ {
		kind := kinds[i%len(kinds)]
		namespace := namespaces[i%len(namespaces)]
		event := createTestEventWithKindNamespace(i, hourTimestamp, kind, namespace)
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write: %v", err)
		}
	}

	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	metadata := bsf.GetMetadata()
	if len(metadata.Namespaces) == 0 {
		t.Error("No namespaces in metadata")
	}
	if len(metadata.ResourceTypes) == 0 {
		t.Error("No resource types in metadata")
	}

	// Verify all namespaces are present
	expectedNamespaces := map[string]bool{
		"default": true, "kube-system": true, "production": true, "staging": true,
	}
	for ns := range expectedNamespaces {
		if !metadata.Namespaces[ns] {
			t.Errorf("Namespace %s not found in metadata", ns)
		}
	}

	// Verify all kinds are present
	expectedKinds := map[string]bool{
		"Pod": true, "Deployment": true, "Service": true, "ConfigMap": true,
	}
	for kind := range expectedKinds {
		if !metadata.ResourceTypes[kind] {
			t.Errorf("Kind %s not found in metadata", kind)
		}
	}
}

func TestBlockStorageFileInvertedIndex(t *testing.T) {
	tmpFile := t.TempDir() + "/test_inverted_index.bin"
	hourTimestamp := time.Now().Unix()
	blockSize := int64(100 * 1024) // Small blocks to ensure multiple blocks

	bsf, err := storage.NewBlockStorageFile(tmpFile, hourTimestamp, blockSize)
	if err != nil {
		t.Fatalf("Failed to create: %v", err)
	}

	// Write events with specific patterns
	kinds := []string{"Pod", "Deployment", "Service"}
	namespaces := []string{"default", "kube-system"}
	groups := []string{"", "apps"}

	for i := 0; i < 300; i++ {
		kind := kinds[i%len(kinds)]
		namespace := namespaces[i%len(namespaces)]
		group := groups[i%len(groups)]
		event := createTestEventWithAllFields(i, hourTimestamp, kind, namespace, group)
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write: %v", err)
		}
	}

	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	index := bsf.GetInvertedIndex()
	if index == nil {
		t.Fatal("Inverted index is nil")
	}

	// Verify kind index
	if len(index.KindToBlocks) == 0 {
		t.Fatal("Kind index is empty")
	}
	for _, kind := range kinds {
		if blocks, ok := index.KindToBlocks[kind]; !ok || len(blocks) == 0 {
			t.Errorf("Kind %s not found in index or has no blocks", kind)
		}
	}

	// Verify namespace index
	if len(index.NamespaceToBlocks) == 0 {
		t.Fatal("Namespace index is empty")
	}
	for _, ns := range namespaces {
		if blocks, ok := index.NamespaceToBlocks[ns]; !ok || len(blocks) == 0 {
			t.Errorf("Namespace %s not found in index or has no blocks", ns)
		}
	}

	// Verify group index
	if len(index.GroupToBlocks) == 0 {
		t.Fatal("Group index is empty")
	}
}

func TestBlockStorageFileEmptyFile(t *testing.T) {
	tmpFile := t.TempDir() + "/test_empty.bin"
	hourTimestamp := time.Now().Unix()
	blockSize := int64(256 * 1024)

	bsf, err := storage.NewBlockStorageFile(tmpFile, hourTimestamp, blockSize)
	if err != nil {
		t.Fatalf("Failed to create: %v", err)
	}

	// Close without writing any events
	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close empty file: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(tmpFile); err != nil {
		t.Fatalf("File should exist: %v", err)
	}

	// Verify event count is 0
	if bsf.GetEventCount() != 0 {
		t.Errorf("Expected 0 events, got %d", bsf.GetEventCount())
	}

	// Verify block count is 0
	if bsf.GetSegmentCount() != 0 {
		t.Errorf("Expected 0 blocks, got %d", bsf.GetSegmentCount())
	}
}

func TestBlockStorageFileSingleEvent(t *testing.T) {
	tmpFile := t.TempDir() + "/test_single_event.bin"
	hourTimestamp := time.Now().Unix()
	blockSize := int64(256 * 1024)

	bsf, err := storage.NewBlockStorageFile(tmpFile, hourTimestamp, blockSize)
	if err != nil {
		t.Fatalf("Failed to create: %v", err)
	}

	// Write single event
	event := createTestEvent(0, hourTimestamp)
	if err := bsf.WriteEvent(event); err != nil {
		t.Fatalf("Failed to write: %v", err)
	}

	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	if bsf.GetEventCount() != 1 {
		t.Errorf("Expected 1 event, got %d", bsf.GetEventCount())
	}

	if bsf.GetSegmentCount() != 1 {
		t.Errorf("Expected 1 block, got %d", bsf.GetSegmentCount())
	}
}

func TestBlockStorageFileTimestampRange(t *testing.T) {
	tmpFile := t.TempDir() + "/test_timestamps.bin"
	hourTimestamp := time.Now().Unix()
	blockSize := int64(256 * 1024)

	bsf, err := storage.NewBlockStorageFile(tmpFile, hourTimestamp, blockSize)
	if err != nil {
		t.Fatalf("Failed to create: %v", err)
	}

	// Write events with varying timestamps
	baseTime := hourTimestamp * 1e9 // Convert to nanoseconds
	for i := 0; i < 100; i++ {
		event := createTestEventWithTimestamp(i, baseTime+int64(i*1000000)) // 1ms apart
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write: %v", err)
		}
	}

	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	metadata := bsf.GetMetadata()
	// Metadata should have valid timestamps
	if metadata.TotalEvents == 0 {
		t.Error("Expected events in metadata")
	}

	// Check block metadata timestamps
	blockMetadata := bsf.GetBlockMetadata()
	if len(blockMetadata) == 0 {
		t.Fatal("No block metadata")
	}

	for _, bm := range blockMetadata {
		if bm.TimestampMin == 0 {
			t.Error("Block has zero min timestamp")
		}
		if bm.TimestampMax == 0 {
			t.Error("Block has zero max timestamp")
		}
		if bm.TimestampMin > bm.TimestampMax {
			t.Errorf("Block min timestamp %d > max timestamp %d", bm.TimestampMin, bm.TimestampMax)
		}
	}
}

func TestBlockStorageFileGetBlocks(t *testing.T) {
	tmpFile := t.TempDir() + "/test_get_blocks.bin"
	hourTimestamp := time.Now().Unix()
	blockSize := int64(100 * 1024) // Small blocks

	bsf, err := storage.NewBlockStorageFile(tmpFile, hourTimestamp, blockSize)
	if err != nil {
		t.Fatalf("Failed to create: %v", err)
	}

	// Write events to create multiple blocks
	for i := 0; i < 200; i++ {
		event := createTestEvent(i, hourTimestamp)
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write: %v", err)
		}
	}

	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close: %v", err)
	}

	blocks := bsf.GetBlocks()
	if len(blocks) == 0 {
		t.Fatal("No blocks returned")
	}

	// Verify block properties
	for i, block := range blocks {
		if block.ID != int32(i) {
			t.Errorf("Block ID mismatch: expected %d, got %d", i, block.ID)
		}
		if block.EventCount == 0 {
			t.Errorf("Block %d has no events", i)
		}
		if block.Length == 0 {
			t.Errorf("Block %d has zero length", i)
		}
		if block.UncompressedLength == 0 {
			t.Errorf("Block %d has zero uncompressed length", i)
		}
		if block.Metadata == nil {
			t.Errorf("Block %d has nil metadata", i)
		}
	}
}

// Helper functions

func createTestEvent(id int, hourTimestamp int64) *models.Event {
	data, _ := json.Marshal(map[string]interface{}{
		"message": fmt.Sprintf("Event %d", id),
		"index":   id,
	})

	return &models.Event{
		ID:        fmt.Sprintf("evt-%d", id),
		Timestamp: hourTimestamp*1e9 + int64(id*1000000), // nanoseconds, 1ms apart
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Kind:      "Pod",
			Namespace: "default",
			Name:      fmt.Sprintf("pod-%d", id),
			Group:     "",
			Version:   "v1",
		},
		Data: data,
	}
}

func createTestEventWithLargeData(id int, hourTimestamp int64) *models.Event {
	// Create event with larger data payload
	largeData := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		largeData[fmt.Sprintf("field%d", i)] = fmt.Sprintf("value-%d-%d", id, i)
	}
	data, _ := json.Marshal(largeData)

	return &models.Event{
		ID:        fmt.Sprintf("evt-%d", id),
		Timestamp: hourTimestamp*1e9 + int64(id*1000000),
		Type:      models.EventTypeUpdate,
		Resource: models.ResourceMetadata{
			Kind:      "Pod",
			Namespace: "default",
			Name:      fmt.Sprintf("pod-%d", id),
			Group:     "",
			Version:   "v1",
		},
		Data: data,
	}
}

func createTestEventWithKindNamespace(id int, hourTimestamp int64, kind, namespace string) *models.Event {
	data, _ := json.Marshal(map[string]interface{}{
		"message": fmt.Sprintf("Event %d: %s in %s", id, kind, namespace),
	})

	return &models.Event{
		ID:        fmt.Sprintf("evt-%d", id),
		Timestamp: hourTimestamp*1e9 + int64(id*1000000),
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Kind:      kind,
			Namespace: namespace,
			Name:      fmt.Sprintf("%s-%d", kind, id),
			Group:     getGroupForKind(kind),
			Version:   "v1",
		},
		Data: data,
	}
}

func createTestEventWithAllFields(id int, hourTimestamp int64, kind, namespace, group string) *models.Event {
	data, _ := json.Marshal(map[string]interface{}{
		"message": fmt.Sprintf("Event %d", id),
	})

	return &models.Event{
		ID:        fmt.Sprintf("evt-%d", id),
		Timestamp: hourTimestamp*1e9 + int64(id*1000000),
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Kind:      kind,
			Namespace: namespace,
			Name:      fmt.Sprintf("%s-%d", kind, id),
			Group:     group,
			Version:   "v1",
		},
		Data: data,
	}
}

func createTestEventWithTimestamp(id int, timestampNs int64) *models.Event {
	data, _ := json.Marshal(map[string]interface{}{
		"message": fmt.Sprintf("Event %d", id),
	})

	return &models.Event{
		ID:        fmt.Sprintf("evt-%d", id),
		Timestamp: timestampNs,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Kind:      "Pod",
			Namespace: "default",
			Name:      fmt.Sprintf("pod-%d", id),
			Group:     "",
			Version:   "v1",
		},
		Data: data,
	}
}

func getGroupForKind(kind string) string {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet":
		return "apps"
	case "Pod", "Service", "ConfigMap":
		return ""
	default:
		return ""
	}
}
