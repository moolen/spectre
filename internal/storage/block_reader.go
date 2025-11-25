package storage

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/moritz/rpk/internal/models"
)

// BlockReader handles reading and decompressing blocks from storage files
type BlockReader struct {
	filePath string
	file     *os.File
}

// NewBlockReader creates a new reader for a storage file
func NewBlockReader(filePath string) (*BlockReader, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return &BlockReader{
		filePath: filePath,
		file:     file,
	}, nil
}

// ReadFileHeader reads and validates the file header
func (br *BlockReader) ReadFileHeader() (*FileHeader, error) {
	if _, err := br.file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek to start: %w", err)
	}

	return ReadFileHeader(br.file)
}

// ReadFileFooter reads the file footer from the end of file
func (br *BlockReader) ReadFileFooter() (*FileFooter, error) {
	// Seek to end minus footer size
	fileSize, err := br.file.Seek(0, 2) // Seek to end
	if err != nil {
		return nil, fmt.Errorf("failed to seek to end: %w", err)
	}

	if fileSize < int64(FileFooterSize) {
		return nil, fmt.Errorf("file too small: expected at least %d bytes, got %d", FileFooterSize, fileSize)
	}

	footerOffset := fileSize - int64(FileFooterSize)
	if _, err := br.file.Seek(footerOffset, 0); err != nil {
		return nil, fmt.Errorf("failed to seek to footer: %w", err)
	}

	return ReadFileFooter(br.file)
}

// ReadIndexSection reads the index section from the specified offset and length
func (br *BlockReader) ReadIndexSection(offset int64, length int32) (*IndexSection, error) {
	if _, err := br.file.Seek(offset, 0); err != nil {
		return nil, fmt.Errorf("failed to seek to index: %w", err)
	}

	// Read exactly length bytes
	buf := make([]byte, length)
	if _, err := io.ReadFull(br.file, buf); err != nil {
		return nil, fmt.Errorf("failed to read index section: %w", err)
	}

	// Unmarshal JSON
	var section IndexSection
	if err := json.Unmarshal(buf, &section); err != nil {
		return nil, fmt.Errorf("failed to unmarshal index section: %w", err)
	}

	return &section, nil
}

// ReadBlock reads and decompresses a block from file
func (br *BlockReader) ReadBlock(metadata *BlockMetadata) ([]byte, error) {
	if _, err := br.file.Seek(metadata.Offset, 0); err != nil {
		return nil, fmt.Errorf("failed to seek to block: %w", err)
	}

	// Read compressed data
	compressedData := make([]byte, metadata.CompressedLength)
	if _, err := io.ReadFull(br.file, compressedData); err != nil {
		return nil, fmt.Errorf("failed to read block data: %w", err)
	}

	// Decompress using gzip
	decompressed, err := DecompressBlock(&Block{CompressedData: compressedData})
	if err != nil {
		return nil, fmt.Errorf("failed to decompress block: %w", err)
	}

	return decompressed, nil
}

// ReadBlockEvents reads and decompresses a block, then parses events
func (br *BlockReader) ReadBlockEvents(metadata *BlockMetadata) ([]*models.Event, error) {
	decompressedData, err := br.ReadBlock(metadata)
	if err != nil {
		return nil, err
	}

	// Parse events from newline-delimited JSON (NDJSON)
	var events []*models.Event
	lines := bytes.Split(decompressedData, []byte("\n"))

	for _, line := range lines {
		if len(line) == 0 {
			continue // Skip empty lines
		}

		var event *models.Event
		if err := json.Unmarshal(line, &event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal event: %w", err)
		}

		events = append(events, event)
	}

	return events, nil
}

// ReadFile reads complete file structure and returns all data
func (br *BlockReader) ReadFile() (*StorageFileData, error) {
	// Read header
	header, err := br.ReadFileHeader()
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	// Read footer
	footer, err := br.ReadFileFooter()
	if err != nil {
		return nil, fmt.Errorf("failed to read footer: %w", err)
	}

	// Read index section
	indexSection, err := br.ReadIndexSection(footer.IndexSectionOffset, footer.IndexSectionLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read index section: %w", err)
	}

	return &StorageFileData{
		Header:       header,
		Footer:       footer,
		IndexSection: indexSection,
		BlockReader:  br,
	}, nil
}

// Close closes the underlying file
func (br *BlockReader) Close() error {
	if br.file != nil {
		return br.file.Close()
	}
	return nil
}

// StorageFileData represents a completely read storage file
type StorageFileData struct {
	Header       *FileHeader
	Footer       *FileFooter
	IndexSection *IndexSection
	BlockReader  *BlockReader
}

// GetEvents reads all events from blocks matching the given filters
// Returns all events if filters is nil or empty
func (sfd *StorageFileData) GetEvents(filters map[string]string) ([]*models.Event, error) {
	candidateBlocks := GetCandidateBlocks(sfd.IndexSection.InvertedIndexes, filters)
	if len(candidateBlocks) == 0 && len(filters) > 0 {
		// No matching blocks
		return []*models.Event{}, nil
	}

	// If no filters, return all blocks
	if len(candidateBlocks) == 0 && len(filters) == 0 {
		candidateBlocks = make([]int32, len(sfd.IndexSection.BlockMetadata))
		for i, metadata := range sfd.IndexSection.BlockMetadata {
			candidateBlocks[i] = metadata.ID
		}
	}

	var allEvents []*models.Event

	for _, blockID := range candidateBlocks {
		// Find metadata for this block
		var metadata *BlockMetadata
		for _, bm := range sfd.IndexSection.BlockMetadata {
			if bm.ID == blockID {
				metadata = bm
				break
			}
		}

		if metadata == nil {
			continue
		}

		// Read block events
		events, err := sfd.BlockReader.ReadBlockEvents(metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to read block %d: %w", blockID, err)
		}

		allEvents = append(allEvents, events...)
	}

	return allEvents, nil
}


// VerifyBlockChecksum verifies a block's checksum if checksums are enabled
func VerifyBlockChecksum(block *Block, metadata *BlockMetadata) error {
	if metadata.Checksum == "" {
		// No checksum, nothing to verify
		return nil
	}

	// Compute checksum of decompressed data
	decompressed, err := DecompressBlock(block)
	if err != nil {
		return fmt.Errorf("failed to decompress block for verification: %w", err)
	}

	computed := ComputeChecksum(decompressed)
	if computed != metadata.Checksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", metadata.Checksum, computed)
	}

	return nil
}

// ComputeChecksum computes an MD5 checksum for data
// MD5 is used for integrity checking rather than cryptographic security
func ComputeChecksum(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}
