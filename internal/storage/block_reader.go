package storage

import (
	"crypto/md5" //nolint:gosec // MD5 used for checksum, not cryptographic purposes
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
	"google.golang.org/protobuf/proto"
)

// BlockReader handles reading and decompressing blocks from storage files
type BlockReader struct {
	filePath string
	file     *os.File
	logger   *logging.Logger
}

// NewBlockReader creates a new reader for a storage file
func NewBlockReader(filePath string) (*BlockReader, error) {
	file, err := os.Open(filePath) //nolint:gosec // filePath is validated before use
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return &BlockReader{
		filePath: filePath,
		file:     file,
		logger:   logging.GetLogger("block_reader"),
	}, nil
}

// ReadFileHeader reads and validates the file header
func (br *BlockReader) ReadFileHeader() (*FileHeader, error) {
	if _, err := br.file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek to start: %w", err)
	}

	header, err := ReadFileHeader(br.file)
	if err != nil {
		return nil, err
	}

	// Validate version
	if err := ValidateVersion(header.FormatVersion); err != nil {
		return nil, fmt.Errorf("version validation failed: %w", err)
	}

	return header, nil
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

	// Unmarshal protobuf
	var pbSection PBIndexSection
	if err := proto.Unmarshal(buf, &pbSection); err != nil {
		return nil, fmt.Errorf("failed to unmarshal index section: %w", err)
	}

	return convertFromProto(&pbSection), nil
}

// ReadBlock reads and decompresses a block from file
func (br *BlockReader) ReadBlock(metadata *BlockMetadata) ([]byte, error) {
	// Time file seek operation
	seekStart := time.Now()
	if _, err := br.file.Seek(metadata.Offset, 0); err != nil {
		return nil, fmt.Errorf("failed to seek to block: %w", err)
	}
	seekTime := time.Since(seekStart)

	// Time file read operation
	readStart := time.Now()
	compressedData := make([]byte, metadata.CompressedLength)
	if _, err := io.ReadFull(br.file, compressedData); err != nil {
		return nil, fmt.Errorf("failed to read block data: %w", err)
	}
	readTime := time.Since(readStart)

	// Log slow I/O operations (threshold: 100ms)
	if seekTime.Milliseconds() > 100 || readTime.Milliseconds() > 100 {
		br.logger.Warn("Slow file I/O: file=%s, blockID=%d, seekTime=%dms, readTime=%dms, size=%dKB",
			br.filePath, metadata.ID, seekTime.Milliseconds(), readTime.Milliseconds(),
			metadata.CompressedLength/1024)
	}

	// Decompress using gzip
	decompressed, err := DecompressBlock(&Block{CompressedData: compressedData})
	if err != nil {
		return nil, fmt.Errorf("failed to decompress block: %w", err)
	}

	return decompressed, nil
}

// ReadBlockWithCache reads a block, checking the cache first
func (br *BlockReader) ReadBlockWithCache(filename string, metadata *BlockMetadata, cache *BlockCache) (*CachedBlock, error) {
	// Check cache first
	if cached := cache.Get(filename, metadata.ID); cached != nil {
		return cached, nil
	}

	// Cache miss: read and decompress
	decompressStart := time.Now()
	decompressed, err := br.ReadBlock(metadata)
	decompressTime := time.Since(decompressStart)

	if err != nil {
		br.logger.Warn("Failed to read block: file=%s, blockID=%d, error=%v",
			filename, metadata.ID, err)
		return nil, err
	}

	br.logger.Debug("Block decompressed: file=%s, blockID=%d, compressedSize=%d, decompressedSize=%d, time=%dms",
		filename, metadata.ID, metadata.CompressedLength, len(decompressed), decompressTime.Milliseconds())

	// Parse events from decompressed protobuf data
	events, err := br.readBlockEventsProtobuf(decompressed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse protobuf events: %w", err)
	}

	cachedBlock := &CachedBlock{
		BlockID:  makeKey(filename, metadata.ID),
		Events:   events,
		Metadata: metadata,
		Size:     int64(len(decompressed)),
		Filename: filename,
		ID:       metadata.ID,
	}

	// Store in cache
	if err := cache.Put(filename, metadata.ID, cachedBlock); err != nil {
		// If cache full, log but continue (don't fail query)
		br.logger.Warn("Failed to cache block: file=%s, blockID=%d, blockSize=%dKB, error=%v",
			filename, metadata.ID, cachedBlock.Size/1024, err)
	}

	return cachedBlock, nil
}

// ReadBlockEvents reads and decompresses a block, then parses events
// Events are decoded using protobuf format
func (br *BlockReader) ReadBlockEvents(metadata *BlockMetadata) ([]*models.Event, error) {
	decompressedData, err := br.ReadBlock(metadata)
	if err != nil {
		return nil, err
	}

	if len(decompressedData) == 0 {
		return nil, fmt.Errorf("decompressed data is empty")
	}

	return br.readBlockEventsProtobuf(decompressedData)
}

// readBlockEventsProtobuf reads events encoded as length-prefixed protobuf messages
func (br *BlockReader) readBlockEventsProtobuf(decompressedData []byte) ([]*models.Event, error) {
	var events []*models.Event
	offset := 0

	// Read length-prefixed protobuf messages
	for offset < len(decompressedData) {
		// Parse varint length
		length, n := binary.Uvarint(decompressedData[offset:])
		if n <= 0 {
			break // End of data
		}
		offset += n

		// Extract message bytes
		if offset+int(length) > len(decompressedData) { //nolint:gosec // safe conversion: length is validated
			return nil, fmt.Errorf("invalid message length: %d at offset %d", length, offset)
		}

		messageData := decompressedData[offset : offset+int(length)] //nolint:gosec // safe conversion: length is validated
		offset += int(length)                                        //nolint:gosec // safe conversion: length is validated

		// Unmarshal event
		event := &models.Event{}
		if err := event.UnmarshalProtobuf(messageData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal event at offset %d: %w", offset, err)
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

		// Filter events based on the filter criteria
		// Block-level filtering is just an optimization - we still need to filter individual events
		for _, event := range events {
			if matchesFilters(event, filters) {
				allEvents = append(allEvents, event)
			}
		}
	}

	return allEvents, nil
}

// matchesFilters checks if an event matches all filter criteria
func matchesFilters(event *models.Event, filters map[string]string) bool {
	if len(filters) == 0 {
		return true
	}

	if kind, ok := filters["kind"]; ok {
		if event.Resource.Kind != kind {
			return false
		}
	}

	if ns, ok := filters["namespace"]; ok {
		if event.Resource.Namespace != ns {
			return false
		}
	}

	if group, ok := filters["group"]; ok {
		if event.Resource.Group != group {
			return false
		}
	}

	return true
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
	hash := md5.Sum(data) //nolint:gosec // MD5 used for checksum, not cryptographic purposes
	return hex.EncodeToString(hash[:])
}
