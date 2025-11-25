package storage

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/moritz/rpk/internal/storage"
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
	if readHeader.FormatVersion != "1.0" {
		t.Errorf("Expected version 1.0, got %s", readHeader.FormatVersion)
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
		FormatVersion:   "1.0",
		BlockMetadata:   []*storage.BlockMetadata{},
		InvertedIndexes: &storage.InvertedIndex{},
		Statistics: &storage.IndexStatistics{
			TotalBlocks:     0,
			TotalEvents:     0,
			CompressionRatio: 1.0,
		},
	}

	indexData, _ := json.MarshalIndent(indexSection, "", "  ")
	indexOffset, _ := file.Seek(0, 1)

	if _, err := file.Write(indexData); err != nil {
		t.Fatalf("Failed to write index: %v", err)
	}

	// Write footer
	footer := &storage.FileFooter{
		IndexSectionOffset: indexOffset,
		IndexSectionLength: int32(len(indexData)),
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

	if indexSection2.FormatVersion != "1.0" {
		t.Errorf("Expected format version 1.0, got %s", indexSection2.FormatVersion)
	}
}

func TestComputeChecksum(t *testing.T) {
	data1 := []byte("test data 1")
	data2 := []byte("test data 2")
	data1Again := []byte("test data 1")

	checksum1 := storage.ComputeChecksum(data1)
	checksum2 := storage.ComputeChecksum(data2)
	checksum1Again := storage.ComputeChecksum(data1Again)

	// Same data should produce same checksum
	if checksum1 != checksum1Again {
		t.Errorf("Same data produced different checksums: %s vs %s", checksum1, checksum1Again)
	}

	// Different data should produce different checksums
	if checksum1 == checksum2 {
		t.Errorf("Different data produced same checksum")
	}
}
