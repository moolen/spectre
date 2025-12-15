package storage

import (
	"bytes"
	"os"
	"testing"

	"github.com/moolen/spectre/internal/storage"
)

func TestReadFileHeader(t *testing.T) {
	// Create and serialize a header
	header := storage.NewFileHeader()
	header.CompressionAlgorithm = "gzip"
	header.BlockSize = 256 * 1024

	buf := &bytes.Buffer{}
	if err := storage.WriteFileHeader(buf, header); err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	// Read it back
	reader := bytes.NewReader(buf.Bytes())
	readHeader, err := storage.ReadFileHeader(reader)
	if err != nil {
		t.Fatalf("Failed to read header: %v", err)
	}

	if readHeader.MagicBytes != "RPKBLOCK" {
		t.Errorf("Expected RPKBLOCK, got %s", readHeader.MagicBytes)
	}
	if readHeader.FormatVersion != storage.FormatVersionV2_0 {
		t.Errorf("Expected version %s, got %s", storage.FormatVersionV2_0, readHeader.FormatVersion)
	}
	if readHeader.CompressionAlgorithm != "gzip" {
		t.Errorf("Expected gzip, got %s", readHeader.CompressionAlgorithm)
	}
}

func TestReadFileFooter(t *testing.T) {
	// Create and serialize a footer
	footer := &storage.FileFooter{
		IndexSectionOffset: 1000000,
		IndexSectionLength: 50000,
		Checksum:           "abc123",
		MagicBytes:         "RPKEND",
	}

	buf := &bytes.Buffer{}
	if err := storage.WriteFileFooter(buf, footer); err != nil {
		t.Fatalf("Failed to write footer: %v", err)
	}

	// Read it back
	reader := bytes.NewReader(buf.Bytes())
	readFooter, err := storage.ReadFileFooter(reader)
	if err != nil {
		t.Fatalf("Failed to read footer: %v", err)
	}

	if readFooter.IndexSectionOffset != 1000000 {
		t.Errorf("Expected offset 1000000, got %d", readFooter.IndexSectionOffset)
	}
	if readFooter.IndexSectionLength != 50000 {
		t.Errorf("Expected length 50000, got %d", readFooter.IndexSectionLength)
	}
	if readFooter.MagicBytes != "RPKEND" {
		t.Errorf("Expected RPKEND, got %s", readFooter.MagicBytes)
	}
}

func TestDecompressBlock(t *testing.T) {
	// Create a block with some data
	originalData := []byte("Hello, this is some test data for compression and decompression!")

	block := &storage.Block{
		ID:                 0,
		CompressedData:     originalData,
		UncompressedLength: int64(len(originalData)),
	}

	// Compress the block
	compressedBlock, err := storage.CompressBlock(block)
	if err != nil {
		t.Fatalf("Failed to compress block: %v", err)
	}

	// Decompress it
	decompressed, err := storage.DecompressBlock(compressedBlock)
	if err != nil {
		t.Fatalf("Failed to decompress block: %v", err)
	}

	// Verify we got back the original data
	if !bytes.Equal(decompressed, originalData) {
		t.Errorf("Decompressed data doesn't match original")
	}
}

func TestBlockReaderRoundtrip(t *testing.T) {
	// Create a temporary storage file
	tmpFile := t.TempDir() + "/test_reader.bin"

	// Write a simple file with header and footer
	file, err := os.Create(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Write header
	header := storage.NewFileHeader()
	if err := storage.WriteFileHeader(file, header); err != nil {
		t.Fatalf("Failed to write header: %v", err)
	}

	// Write some block data
	blockData := []byte("Test block content")
	if _, err := file.Write(blockData); err != nil {
		t.Fatalf("Failed to write block: %v", err)
	}

	// Write a simple index section
	indexSection := &storage.IndexSection{
		FormatVersion:   storage.FormatVersionV2_0,
		BlockMetadata:   []*storage.BlockMetadata{},
		InvertedIndexes: &storage.InvertedIndex{},
		Statistics: &storage.IndexStatistics{
			TotalBlocks:      0,
			TotalEvents:      0,
			CompressionRatio: 1.0,
		},
		FinalResourceStates: make(map[string]*storage.ResourceLastState),
	}

	indexOffset, _ := file.Seek(0, 1)
	indexLength, err := storage.WriteIndexSection(file, indexSection)
	if err != nil {
		t.Fatalf("Failed to write index: %v", err)
	}

	// Write footer
	footer := &storage.FileFooter{
		IndexSectionOffset: indexOffset,
		IndexSectionLength: int32(indexLength),
		MagicBytes:         "RPKEND",
	}

	if err := storage.WriteFileFooter(file, footer); err != nil {
		t.Fatalf("Failed to write footer: %v", err)
	}

	file.Close()

	// Now test reading it back
	reader, err := storage.NewBlockReader(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// Read header
	readHeader, err := reader.ReadFileHeader()
	if err != nil {
		t.Fatalf("Failed to read header: %v", err)
	}

	if readHeader.MagicBytes != "RPKBLOCK" {
		t.Errorf("Expected RPKBLOCK, got %s", readHeader.MagicBytes)
	}

	// Read footer
	readFooter, err := reader.ReadFileFooter()
	if err != nil {
		t.Fatalf("Failed to read footer: %v", err)
	}

	if readFooter.MagicBytes != "RPKEND" {
		t.Errorf("Expected RPKEND, got %s", readFooter.MagicBytes)
	}

	// Read index section
	indexSection2, err := reader.ReadIndexSection(readFooter.IndexSectionOffset, readFooter.IndexSectionLength)
	if err != nil {
		t.Fatalf("Failed to read index section: %v", err)
	}

	if indexSection2.FormatVersion != storage.FormatVersionV2_0 {
		t.Errorf("Expected format version %s, got %s", storage.FormatVersionV2_0, indexSection2.FormatVersion)
	}
}
