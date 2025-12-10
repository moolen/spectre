package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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
