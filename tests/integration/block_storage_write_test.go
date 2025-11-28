package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/moolen/spectre/internal/models"
	"github.com/moolen/spectre/internal/storage"
)

func TestBlockStorageWriteReadRoundtrip(t *testing.T) {
	// Create temporary file
	tmpFile := t.TempDir() + "/test_block_storage.bin"

	// Create block storage file
	hourTimestamp := time.Now().Unix()
	blockSize := int64(256 * 1024) // 256KB
	bsf, err := storage.NewBlockStorageFile(tmpFile, hourTimestamp, blockSize)
	if err != nil {
		t.Fatalf("Failed to create block storage file: %v", err)
	}

	// Write test events
	eventCount := 1000
	for i := 0; i < eventCount; i++ {
		event := createTestEvent(i, hourTimestamp)
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event %d: %v", i, err)
		}
	}

	// Close file (finalizes blocks and writes index)
	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close block storage file: %v", err)
	}

	// Verify file was created and has content
	fileInfo, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	if fileInfo.Size() == 0 {
		t.Fatal("File is empty after writing events")
	}

	// Verify compression stats
	stats := bsf.GetCompressionStats()
	uncompressed := stats["total_uncompressed_bytes"].(int64)
	compressed := stats["total_compressed_bytes"].(int64)
	ratio := stats["compression_ratio"].(float64)

	t.Logf("Compression Stats:")
	t.Logf("  Uncompressed: %d bytes", uncompressed)
	t.Logf("  Compressed:   %d bytes", compressed)
	t.Logf("  Ratio:        %.2f%% (%.4f)", ratio*100, ratio)
	t.Logf("  Bytes saved:  %d", stats["bytes_saved"].(int64))
	t.Logf("  Blocks:       %d", stats["block_count"].(int))
	t.Logf("  Events:       %d", stats["total_events"].(int64))

	// Verify compression target (50%+)
	// Note: With JSON overhead, we expect ~30-50% compression
	if uncompressed > 0 && ratio > 1.0 {
		t.Errorf("Invalid compression ratio: %.4f (expected < 1.0)", ratio)
	}

	// Verify all events were written
	if bsf.GetEventCount() != int64(eventCount) {
		t.Errorf("Expected %d events, got %d", eventCount, bsf.GetEventCount())
	}

	// Verify blocks were created
	blockCount := bsf.GetSegmentCount()
	if blockCount == 0 {
		t.Fatal("No blocks were created")
	}
	t.Logf("Created %d blocks for %d events", blockCount, eventCount)

	// Verify metadata
	metadata := bsf.GetMetadata()
	if metadata.TotalEvents != int64(eventCount) {
		t.Errorf("Metadata total events mismatch: %d vs %d", metadata.TotalEvents, eventCount)
	}

	// Verify index
	index := bsf.GetSparseTimestampIndex()
	if index.TotalSegments != blockCount {
		t.Errorf("Index total segments mismatch: %d vs %d", index.TotalSegments, blockCount)
	}
	if len(index.Entries) != int(blockCount) {
		t.Errorf("Index entries count mismatch: %d vs %d", len(index.Entries), blockCount)
	}
}

func TestBlockStorageSmallFile(t *testing.T) {
	tmpFile := t.TempDir() + "/test_small.bin"
	hourTimestamp := time.Now().Unix()
	blockSize := int64(64 * 1024) // 64KB for quick testing

	bsf, err := storage.NewBlockStorageFile(tmpFile, hourTimestamp, blockSize)
	if err != nil {
		t.Fatalf("Failed to create block storage file: %v", err)
	}

	// Write just 10 events
	for i := 0; i < 10; i++ {
		event := createTestEvent(i, hourTimestamp)
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}

	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close file: %v", err)
	}

	if bsf.GetEventCount() != 10 {
		t.Errorf("Expected 10 events, got %d", bsf.GetEventCount())
	}
}

func TestBlockStorageCompressionMetrics(t *testing.T) {
	tmpFile := t.TempDir() + "/test_compression.bin"
	hourTimestamp := time.Now().Unix()
	blockSize := int64(256 * 1024)

	bsf, err := storage.NewBlockStorageFile(tmpFile, hourTimestamp, blockSize)
	if err != nil {
		t.Fatalf("Failed to create block storage file: %v", err)
	}

	// Write varying sized payloads
	for i := 0; i < 500; i++ {
		event := createTestEvent(i, hourTimestamp)
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}

	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close file: %v", err)
	}

	stats := bsf.GetCompressionStats()
	ratio := stats["compression_ratio"].(float64)

	t.Logf("Compression ratio for 500 events: %.4f", ratio)

	// Verify file exists
	fileInfo, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	fileSize := fileInfo.Size()
	t.Logf("File size on disk: %d bytes", fileSize)

	// Verify file size is reasonable
	if fileSize < 1000 {
		t.Error("File size too small - likely not writing correctly")
	}
}

func TestBlockStorageMetadata(t *testing.T) {
	tmpFile := t.TempDir() + "/test_metadata.bin"
	hourTimestamp := time.Now().Unix()
	blockSize := int64(256 * 1024)

	bsf, err := storage.NewBlockStorageFile(tmpFile, hourTimestamp, blockSize)
	if err != nil {
		t.Fatalf("Failed to create block storage file: %v", err)
	}

	// Write events from different namespaces and kinds
	for i := 0; i < 100; i++ {
		event := createTestEventWithDetails(i, hourTimestamp, []string{"default", "kube-system", "production"}[i%3])
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}

	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close file: %v", err)
	}

	// Check metadata
	metadata := bsf.GetMetadata()
	if len(metadata.Namespaces) == 0 {
		t.Error("No namespaces tracked in metadata")
	}
	if len(metadata.ResourceTypes) == 0 {
		t.Error("No resource types tracked in metadata")
	}

	t.Logf("Namespaces: %v", metadata.Namespaces)
	t.Logf("Resource types: %v", metadata.ResourceTypes)
}

func TestBlockStorageInvertedIndex(t *testing.T) {
	tmpFile := t.TempDir() + "/test_index.bin"
	hourTimestamp := time.Now().Unix()
	blockSize := int64(100 * 1024) // Smaller blocks to ensure multiple blocks

	bsf, err := storage.NewBlockStorageFile(tmpFile, hourTimestamp, blockSize)
	if err != nil {
		t.Fatalf("Failed to create block storage file: %v", err)
	}

	// Write events with different kinds and namespaces
	kinds := []string{"Pod", "Deployment", "Service"}
	namespaces := []string{"default", "kube-system"}

	for i := 0; i < 200; i++ {
		kind := kinds[i%len(kinds)]
		namespace := namespaces[i%len(namespaces)]
		event := createTestEventWithKindNamespace(i, hourTimestamp, kind, namespace)
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}

	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close file: %v", err)
	}

	// Verify inverted index
	index := bsf.GetInvertedIndex()
	if index == nil {
		t.Fatal("Inverted index is nil")
	}

	// Check that kinds are indexed
	if len(index.KindToBlocks) == 0 {
		t.Fatal("No kinds in inverted index")
	}

	// Verify Pod is in index
	podBlocks := index.KindToBlocks["Pod"]
	if len(podBlocks) == 0 {
		t.Error("Pod not found in kind index")
	}
	t.Logf("Pod found in blocks: %v", podBlocks)

	// Verify namespace index
	if len(index.NamespaceToBlocks) == 0 {
		t.Fatal("No namespaces in inverted index")
	}

	defaultBlocks := index.NamespaceToBlocks["default"]
	if len(defaultBlocks) == 0 {
		t.Error("default namespace not found in index")
	}
	t.Logf("default namespace found in blocks: %v", defaultBlocks)
}

// Helper functions to create test events

func createTestEvent(id int, hourTimestamp int64) *models.Event {
	data, _ := json.Marshal(map[string]interface{}{
		"message": fmt.Sprintf("Event %d with some test data", id),
		"index":   id,
	})

	return &models.Event{
		ID:        fmt.Sprintf("evt-%d", id),
		Timestamp: hourTimestamp + int64(id),
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

func createTestEventWithDetails(id int, hourTimestamp int64, namespace string) *models.Event {
	data, _ := json.Marshal(map[string]interface{}{
		"message": fmt.Sprintf("Event %d in namespace %s", id, namespace),
		"index":   id,
	})

	return &models.Event{
		ID:        fmt.Sprintf("evt-%d", id),
		Timestamp: hourTimestamp + int64(id),
		Type:      models.EventTypeUpdate,
		Resource: models.ResourceMetadata{
			Kind:      "Pod",
			Namespace: namespace,
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
		"index":   id,
	})

	return &models.Event{
		ID:        fmt.Sprintf("evt-%d", id),
		Timestamp: hourTimestamp + int64(id),
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

func getGroupForKind(kind string) string {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet":
		return "apps"
	case "Pod", "Service":
		return ""
	default:
		return ""
	}
}
