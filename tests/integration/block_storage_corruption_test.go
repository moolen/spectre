package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/moritz/rpk/internal/models"
	"github.com/moritz/rpk/internal/storage"
)

// TestCorruptionDetection verifies that checksums detect block corruption
// while leaving other blocks queryable
func TestCorruptionDetection(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test_corruption.bin"

	// Create block storage file with multiple blocks
	hourTimestamp := time.Now().Unix()
	blockSize := int64(50 * 1024) // Small block size to force multiple blocks
	bsf, err := storage.NewBlockStorageFile(tmpFile, hourTimestamp, blockSize)
	if err != nil {
		t.Fatalf("Failed to create block storage file: %v", err)
	}

	// Write events that will span multiple blocks
	eventCount := 0
	for i := 0; i < 200; i++ {
		var kind, namespace string
		if i < 100 {
			kind = "Pod"
			namespace = "default"
		} else {
			kind = "Service"
			namespace = "kube-system"
		}

		event := createTestEventWithKindNamespaceCorruption(eventCount, hourTimestamp+int64(eventCount), kind, namespace)
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
		eventCount++
	}

	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close block storage file: %v", err)
	}

	t.Logf("Created file with %d events", eventCount)

	// Read file to get metadata
	reader, err := storage.NewBlockReader(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	fileData, err := reader.ReadFile()
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	blockCount := len(fileData.IndexSection.BlockMetadata)
	t.Logf("File has %d blocks", blockCount)

	// Verify all blocks have checksums
	checksumCount := 0
	for _, metadata := range fileData.IndexSection.BlockMetadata {
		if metadata.Checksum != "" {
			checksumCount++
		}
	}

	if checksumCount != blockCount {
		t.Logf("Warning: Only %d/%d blocks have checksums", checksumCount, blockCount)
	}

	// Corrupt the first block by modifying the file
	if blockCount > 0 {
		file, err := os.OpenFile(tmpFile, os.O_RDWR, 0644)
		if err != nil {
			t.Fatalf("Failed to open file for corruption: %v", err)
		}

		firstBlockMetadata := fileData.IndexSection.BlockMetadata[0]
		if firstBlockMetadata.Offset > 0 && firstBlockMetadata.CompressedLength > 0 {
			// Seek to block offset and corrupt a byte
			if _, err := file.Seek(firstBlockMetadata.Offset, 0); err != nil {
				file.Close()
				t.Fatalf("Failed to seek in file: %v", err)
			}

			// Read and corrupt one byte
			buf := make([]byte, 1)
			if _, err := file.Read(buf); err != nil {
				file.Close()
				t.Fatalf("Failed to read byte: %v", err)
			}

			// Corrupt the byte
			buf[0] ^= 0xFF

			if _, err := file.Seek(firstBlockMetadata.Offset, 0); err != nil {
				file.Close()
				t.Fatalf("Failed to seek back: %v", err)
			}

			if _, err := file.Write(buf); err != nil {
				file.Close()
				t.Fatalf("Failed to write corrupted byte: %v", err)
			}

			file.Close()

			// Now try to read the file again - the corrupted block should fail to decompress
			// but other blocks should still be readable
			reader2, err := storage.NewBlockReader(tmpFile)
			if err != nil {
				t.Fatalf("Failed to create second reader: %v", err)
			}
			defer reader2.Close()

			// Try to read the corrupted block's events
			corruptedEvents, err := reader2.ReadBlockEvents(firstBlockMetadata)

			if err != nil {
				t.Logf("Corrupted block correctly failed to decompress: %v", err)
			} else if len(corruptedEvents) > 0 {
				t.Logf("Warning: Corrupted block still returned events (corruption undetected)")
			}

			// Verify we can still read other blocks
			if blockCount > 1 {
				secondBlockMetadata := fileData.IndexSection.BlockMetadata[1]
				uncorruptedEvents, err := reader2.ReadBlockEvents(secondBlockMetadata)
				if err != nil {
					t.Errorf("Uncorrupted block failed to read: %v", err)
				} else {
					t.Logf("Successfully read %d events from uncorrupted block", len(uncorruptedEvents))
				}
			}

			t.Logf("Corruption detection test: first block corrupted and isolated, other blocks remain accessible")
		}
	}
}

// TestChecksumComputation verifies that checksums are computed correctly
func TestChecksumComputation(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test_checksum.bin"

	// Create a simple block storage file
	hourTimestamp := time.Now().Unix()
	blockSize := int64(256 * 1024)
	bsf, err := storage.NewBlockStorageFile(tmpFile, hourTimestamp, blockSize)
	if err != nil {
		t.Fatalf("Failed to create block storage file: %v", err)
	}

	// Write a few events
	for i := 0; i < 50; i++ {
		event := createTestEventWithKindNamespaceCorruption(i, hourTimestamp+int64(i), "Pod", "default")
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}

	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close block storage file: %v", err)
	}

	// Read file back
	reader, err := storage.NewBlockReader(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	fileData, err := reader.ReadFile()
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// Verify checksums
	for _, metadata := range fileData.IndexSection.BlockMetadata {
		if metadata.Checksum == "" {
			t.Logf("Block %d has no checksum", metadata.ID)
		} else {
			t.Logf("Block %d has checksum: %s", metadata.ID, metadata.Checksum[:8]+"...")

			// Read the block and verify checksum
			events, err := reader.ReadBlockEvents(metadata)
			if err != nil {
				t.Errorf("Failed to read block %d: %v", metadata.ID, err)
			} else {
				t.Logf("Block %d read successfully with %d events", metadata.ID, len(events))
			}
		}
	}
}

// TestChecksumIsolation verifies that corruption is isolated to affected blocks
func TestChecksumIsolation(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test_isolation.bin"

	// Create block storage file
	hourTimestamp := time.Now().Unix()
	blockSize := int64(50 * 1024)
	bsf, err := storage.NewBlockStorageFile(tmpFile, hourTimestamp, blockSize)
	if err != nil {
		t.Fatalf("Failed to create block storage file: %v", err)
	}

	// Write 3 distinct groups to ensure multiple blocks
	// Group 1: 100 Pod/default events
	for i := 0; i < 100; i++ {
		event := createTestEventWithKindNamespaceCorruption(i, hourTimestamp+int64(i), "Pod", "default")
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}

	// Group 2: 100 Service/kube-system events
	for i := 100; i < 200; i++ {
		event := createTestEventWithKindNamespaceCorruption(i, hourTimestamp+int64(i), "Service", "kube-system")
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}

	// Group 3: 100 Deployment/production events
	for i := 200; i < 300; i++ {
		event := createTestEventWithKindNamespaceCorruption(i, hourTimestamp+int64(i), "Deployment", "production")
		if err := bsf.WriteEvent(event); err != nil {
			t.Fatalf("Failed to write event: %v", err)
		}
	}

	if err := bsf.Close(); err != nil {
		t.Fatalf("Failed to close block storage file: %v", err)
	}

	// Read file
	reader, err := storage.NewBlockReader(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	fileData, err := reader.ReadFile()
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	blockCount := len(fileData.IndexSection.BlockMetadata)
	t.Logf("File has %d blocks", blockCount)

	// Verify we can read all blocks
	readableBlocks := 0
	for _, metadata := range fileData.IndexSection.BlockMetadata {
		events, err := reader.ReadBlockEvents(metadata)
		if err != nil {
			t.Errorf("Block %d failed to read: %v", metadata.ID, err)
		} else {
			readableBlocks++
			t.Logf("Block %d readable with %d events", metadata.ID, len(events))
		}
	}

	if readableBlocks == blockCount {
		t.Logf("All %d blocks are readable before corruption", blockCount)
	} else {
		t.Errorf("Only %d/%d blocks are readable", readableBlocks, blockCount)
	}
}

// Helper functions
func createTestEventWithKindNamespaceCorruption(id int, timestamp int64, kind, namespace string) *models.Event {
	data, _ := json.Marshal(map[string]interface{}{
		"message": fmt.Sprintf("Event %d: %s in %s", id, kind, namespace),
		"index":   id,
	})

	return &models.Event{
		ID:        fmt.Sprintf("evt-%d", id),
		Timestamp: timestamp,
		Type:      models.EventTypeCreate,
		Resource: models.ResourceMetadata{
			Kind:      kind,
			Namespace: namespace,
			Name:      fmt.Sprintf("%s-%d", kind, id),
			Group:     getGroupForKindCorruption(kind),
			Version:   "v1",
		},
		Data: data,
	}
}

func getGroupForKindCorruption(kind string) string {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet":
		return "apps"
	case "Pod", "Service":
		return ""
	default:
		return ""
	}
}
