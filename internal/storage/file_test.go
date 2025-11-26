package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewStorageFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	hourTimestamp := time.Now().Unix()
	compressor := NewCompressor()

	sf, err := NewStorageFile(filePath, hourTimestamp, compressor, 1024)
	if err != nil {
		t.Fatalf("failed to create storage file: %v", err)
	}
	defer sf.Close()

	if sf.path != filePath {
		t.Errorf("expected path %s, got %s", filePath, sf.path)
	}

	if sf.hourTimestamp != hourTimestamp {
		t.Errorf("expected hour timestamp %d, got %d", hourTimestamp, sf.hourTimestamp)
	}

	if sf.segmentSize != 1024 {
		t.Errorf("expected segment size 1024, got %d", sf.segmentSize)
	}

	// Verify file was created
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("expected file to be created")
	}
}

func TestStorageFileWriteEvent(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	hourTimestamp := time.Now().Unix()
	compressor := NewCompressor()

	sf, err := NewStorageFile(filePath, hourTimestamp, compressor, 10240)
	if err != nil {
		t.Fatalf("failed to create storage file: %v", err)
	}
	defer sf.Close()

	event := createTestEvent("pod-1", "default", "Pod", time.Now().UnixNano())

	if err := sf.WriteEvent(event); err != nil {
		t.Fatalf("failed to write event: %v", err)
	}

	if sf.GetEventCount() != 1 {
		t.Errorf("expected 1 event, got %d", sf.GetEventCount())
	}
}

func TestStorageFileWriteMultipleEvents(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	hourTimestamp := time.Now().Unix()
	compressor := NewCompressor()

	sf, err := NewStorageFile(filePath, hourTimestamp, compressor, 10240)
	if err != nil {
		t.Fatalf("failed to create storage file: %v", err)
	}
	defer sf.Close()

	// Write multiple events
	for i := 0; i < 10; i++ {
		event := createTestEvent("pod-"+string(rune(i)), "default", "Pod", time.Now().UnixNano())
		if err := sf.WriteEvent(event); err != nil {
			t.Fatalf("failed to write event %d: %v", i, err)
		}
	}

	if sf.GetEventCount() != 10 {
		t.Errorf("expected 10 events, got %d", sf.GetEventCount())
	}
}

func TestStorageFileFinalizeSegment(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	hourTimestamp := time.Now().Unix()
	compressor := NewCompressor()

	sf, err := NewStorageFile(filePath, hourTimestamp, compressor, 1024) // Small segment size
	if err != nil {
		t.Fatalf("failed to create storage file: %v", err)
	}
	defer sf.Close()

	// Write events until segment is finalized
	for i := 0; i < 100; i++ {
		event := createTestEvent("pod-"+string(rune(i)), "default", "Pod", time.Now().UnixNano())
		if err := sf.WriteEvent(event); err != nil {
			break
		}
	}

	// Close to finalize last segment
	if err := sf.Close(); err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	if sf.GetSegmentCount() == 0 {
		t.Error("expected at least one segment")
	}
}

func TestStorageFileClose(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	hourTimestamp := time.Now().Unix()
	compressor := NewCompressor()

	sf, err := NewStorageFile(filePath, hourTimestamp, compressor, 10240)
	if err != nil {
		t.Fatalf("failed to create storage file: %v", err)
	}

	// Write some events
	for i := 0; i < 5; i++ {
		event := createTestEvent("pod-"+string(rune(i)), "default", "Pod", time.Now().UnixNano())
		sf.WriteEvent(event)
	}

	// Close should finalize segments and write metadata
	if err := sf.Close(); err != nil {
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
}

func TestStorageFileGetMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	hourTimestamp := time.Now().Unix()
	compressor := NewCompressor()

	sf, err := NewStorageFile(filePath, hourTimestamp, compressor, 10240)
	if err != nil {
		t.Fatalf("failed to create storage file: %v", err)
	}
	defer sf.Close()

	// Write some events
	for i := 0; i < 5; i++ {
		event := createTestEvent("pod-"+string(rune(i)), "default", "Pod", time.Now().UnixNano())
		sf.WriteEvent(event)
	}

	metadata := sf.GetMetadata()
	if metadata.TotalEvents != 5 {
		t.Errorf("expected 5 total events, got %d", metadata.TotalEvents)
	}

	if len(metadata.ResourceTypes) == 0 {
		t.Error("expected resource types to be tracked")
	}

	if len(metadata.Namespaces) == 0 {
		t.Error("expected namespaces to be tracked")
	}
}

func TestStorageFileGetIndex(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	hourTimestamp := time.Now().Unix()
	compressor := NewCompressor()

	sf, err := NewStorageFile(filePath, hourTimestamp, compressor, 10240)
	if err != nil {
		t.Fatalf("failed to create storage file: %v", err)
	}
	defer sf.Close()

	// Write some events
	for i := 0; i < 5; i++ {
		event := createTestEvent("pod-"+string(rune(i)), "default", "Pod", time.Now().UnixNano())
		sf.WriteEvent(event)
	}

	sf.Close()

	index := sf.GetIndex()
	if index.TotalSegments == 0 {
		t.Error("expected at least one segment in index")
	}

	if len(index.Entries) == 0 {
		t.Error("expected index entries")
	}
}

func TestStorageFileMultipleSegments(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.bin")
	hourTimestamp := time.Now().Unix()
	compressor := NewCompressor()

	// Use small segment size to force multiple segments
	sf, err := NewStorageFile(filePath, hourTimestamp, compressor, 1024)
	if err != nil {
		t.Fatalf("failed to create storage file: %v", err)
	}
	defer sf.Close()

	// Write many events to fill multiple segments
	for i := 0; i < 100; i++ {
		event := createTestEvent("pod-"+string(rune(i)), "default", "Pod", time.Now().UnixNano())
		if err := sf.WriteEvent(event); err != nil {
			break
		}
	}

	sf.Close()

	if sf.GetSegmentCount() == 0 {
		t.Error("expected at least one segment")
	}
}

