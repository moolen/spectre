package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/moritz/rpk/internal/logging"
	"github.com/moritz/rpk/internal/models"
)

// BlockStorageFile represents a block-based storage file for one hour
type BlockStorageFile struct {
	path              string
	hourTimestamp     int64
	file              *os.File
	blocks            []*Block
	currentBuffer     *EventBuffer
	blockID           int32
	logger            *logging.Logger
	blockSize         int64
	mutex             sync.Mutex
	blockMetadataList []*BlockMetadata
	index             *InvertedIndex
	startOffset       int64

	// Metrics
	totalUncompressed int64
	totalCompressed   int64
	totalEvents       int64
}

// NewBlockStorageFile creates a new block-based storage file
func NewBlockStorageFile(path string, hourTimestamp int64, blockSizeBytes int64) (*BlockStorageFile, error) {
	logger := logging.GetLogger("block_storage")

	// Create or open the file
	file, err := os.Create(path)
	if err != nil {
		logger.Error("Failed to create block storage file %s: %v", path, err)
		return nil, err
	}

	bsf := &BlockStorageFile{
		path:              path,
		hourTimestamp:     hourTimestamp,
		file:              file,
		blocks:            make([]*Block, 0),
		currentBuffer:     NewEventBuffer(blockSizeBytes),
		blockID:           0,
		logger:            logger,
		blockSize:         blockSizeBytes,
		blockMetadataList: make([]*BlockMetadata, 0),
		index:             &InvertedIndex{},
		startOffset:       0,
	}

	// Write file header
	header := NewFileHeader()
	header.BlockSize = int32(blockSizeBytes)
	if err := WriteFileHeader(file, header); err != nil {
		logger.Error("Failed to write file header: %v", err)
		return nil, err
	}

	bsf.startOffset = int64(FileHeaderSize)

	return bsf, nil
}

// WriteEvent writes an event to the block storage file
func (bsf *BlockStorageFile) WriteEvent(event *models.Event) error {
	bsf.mutex.Lock()
	defer bsf.mutex.Unlock()

	// Serialize event to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Check if adding this event would exceed block size
	if bsf.currentBuffer.IsFull(int64(len(eventJSON))) && bsf.currentBuffer.GetEventCount() > 0 {
		// Finalize current buffer and create new block
		if err := bsf.finalizeBlock(); err != nil {
			return err
		}
		bsf.currentBuffer = NewEventBuffer(bsf.blockSize)
	}

	// Add event to buffer
	if !bsf.currentBuffer.AddEvent(eventJSON) {
		// Event is too large for block, skip or error
		bsf.logger.Error("Event too large for block: %d bytes > %d block size", len(eventJSON), bsf.blockSize)
		return fmt.Errorf("event too large for block")
	}

	bsf.totalEvents++

	return nil
}

// finalizeBlock finalizes the current event buffer and writes it as a block
func (bsf *BlockStorageFile) finalizeBlock() error {
	if bsf.currentBuffer.GetEventCount() == 0 {
		return nil
	}

	// Create block from buffer
	block, err := bsf.currentBuffer.Finalize(bsf.blockID, "gzip")
	if err != nil {
		return fmt.Errorf("failed to finalize block: %w", err)
	}

	// Compress block
	block, err = CompressBlock(block)
	if err != nil {
		return fmt.Errorf("failed to compress block: %w", err)
	}

	// Track block offset before writing
	block.Offset, err = bsf.file.Seek(0, 1) // Current position
	if err != nil {
		return fmt.Errorf("failed to get block offset: %w", err)
	}

	// Update metadata with offset and checksum
	if block.Metadata != nil {
		block.Metadata.Offset = block.Offset
		// Compute checksum of uncompressed data if checksums are enabled
		if block.UncompressedLength > 0 {
			// Reconstruct uncompressed data by decompressing the compressed block
			decompressedData, err := DecompressBlock(block)
			if err == nil {
				block.Metadata.Checksum = ComputeChecksum(decompressedData)
			}
		}
	}

	// Write compressed data to file
	if _, err := bsf.file.Write(block.CompressedData); err != nil {
		return fmt.Errorf("failed to write block data: %w", err)
	}

	// Update metrics
	bsf.totalUncompressed += block.UncompressedLength
	bsf.totalCompressed += block.Length

	// Store block and metadata
	bsf.blocks = append(bsf.blocks, block)
	bsf.blockMetadataList = append(bsf.blockMetadataList, block.Metadata)

	bsf.logger.DebugWithFields("Finalized block",
		logging.Field("block_id", bsf.blockID),
		logging.Field("file", bsf.path),
		logging.Field("event_count", block.EventCount),
		logging.Field("uncompressed_bytes", block.UncompressedLength),
		logging.Field("compressed_bytes", block.Length))

	bsf.blockID++
	return nil
}

// Close finalizes the file and writes all metadata
func (bsf *BlockStorageFile) Close() error {
	bsf.mutex.Lock()
	defer bsf.mutex.Unlock()

	// Finalize current buffer if it has events
	if bsf.currentBuffer.GetEventCount() > 0 {
		if err := bsf.finalizeBlock(); err != nil {
			bsf.logger.Error("Failed to finalize last block: %v", err)
		}
	}

	// Build inverted indexes
	if err := bsf.buildIndexes(); err != nil {
		bsf.logger.Error("Failed to build indexes: %v", err)
	}

	// Write index section
	if err := bsf.writeIndexSection(); err != nil {
		bsf.logger.Error("Failed to write index section: %v", err)
	}

	// Close the file
	if err := bsf.file.Close(); err != nil {
		bsf.logger.Error("Failed to close file: %v", err)
		return err
	}

	ratio := 0.0
	if bsf.totalUncompressed > 0 {
		ratio = float64(bsf.totalCompressed) / float64(bsf.totalUncompressed)
	}

	bsf.logger.InfoWithFields("Block storage file closed",
		logging.Field("file", bsf.path),
		logging.Field("block_count", len(bsf.blocks)),
		logging.Field("total_events", bsf.totalEvents),
		logging.Field("total_uncompressed_bytes", bsf.totalUncompressed),
		logging.Field("total_compressed_bytes", bsf.totalCompressed),
		logging.Field("compression_ratio", ratio))

	return nil
}

// buildIndexes builds the inverted indexes from block metadata
func (bsf *BlockStorageFile) buildIndexes() error {
	if len(bsf.blockMetadataList) == 0 {
		bsf.index = &InvertedIndex{
			KindToBlocks:      make(map[string][]int32),
			NamespaceToBlocks: make(map[string][]int32),
			GroupToBlocks:     make(map[string][]int32),
		}
		return nil
	}

	bsf.index = BuildInvertedIndexes(bsf.blockMetadataList)
	return nil
}

// writeIndexSection writes the index section to the end of the file
func (bsf *BlockStorageFile) writeIndexSection() error {
	// Get current file offset (start of index section)
	indexOffset, err := bsf.file.Seek(0, 1)
	if err != nil {
		return fmt.Errorf("failed to get index offset: %w", err)
	}

	// Create statistics
	stats := &IndexStatistics{
		TotalBlocks:            int32(len(bsf.blocks)),
		TotalEvents:            bsf.totalEvents,
		TotalUncompressedBytes: bsf.totalUncompressed,
		TotalCompressedBytes:   bsf.totalCompressed,
		TimestampMin:           bsf.getMinTimestamp(),
		TimestampMax:           bsf.getMaxTimestamp(),
	}

	if bsf.totalUncompressed > 0 {
		stats.CompressionRatio = float32(bsf.totalCompressed) / float32(bsf.totalUncompressed)
	}

	// Count unique values
	uniqueKinds := make(map[string]bool)
	uniqueNamespaces := make(map[string]bool)
	uniqueGroups := make(map[string]bool)

	for _, metadata := range bsf.blockMetadataList {
		for _, kind := range metadata.KindSet {
			uniqueKinds[kind] = true
		}
		for _, ns := range metadata.NamespaceSet {
			uniqueNamespaces[ns] = true
		}
		for _, group := range metadata.GroupSet {
			uniqueGroups[group] = true
		}
	}

	stats.UniqueKinds = int32(len(uniqueKinds))
	stats.UniqueNamespaces = int32(len(uniqueNamespaces))
	stats.UniqueGroups = int32(len(uniqueGroups))

	// Create index section
	indexSection := &IndexSection{
		FormatVersion:    DefaultFormatVersion,
		BlockMetadata:    bsf.blockMetadataList,
		InvertedIndexes:  bsf.index,
		Statistics:       stats,
	}

	// Write index section
	bytesWritten, err := WriteIndexSection(bsf.file, indexSection)
	if err != nil {
		return fmt.Errorf("failed to write index section: %w", err)
	}

	// Write file footer
	footer := &FileFooter{
		IndexSectionOffset: indexOffset,
		IndexSectionLength: int32(bytesWritten),
		MagicBytes:         FileFooterMagic,
	}

	if err := WriteFileFooter(bsf.file, footer); err != nil {
		return fmt.Errorf("failed to write file footer: %w", err)
	}

	return nil
}

// getMinTimestamp returns the minimum timestamp from all blocks
func (bsf *BlockStorageFile) getMinTimestamp() int64 {
	if len(bsf.blocks) == 0 {
		return 0
	}

	minTs := bsf.blocks[0].TimestampMin
	for _, block := range bsf.blocks {
		if block.TimestampMin > 0 && block.TimestampMin < minTs {
			minTs = block.TimestampMin
		}
	}
	return minTs
}

// getMaxTimestamp returns the maximum timestamp from all blocks
func (bsf *BlockStorageFile) getMaxTimestamp() int64 {
	if len(bsf.blocks) == 0 {
		return 0
	}

	maxTs := bsf.blocks[0].TimestampMax
	for _, block := range bsf.blocks {
		if block.TimestampMax > maxTs {
			maxTs = block.TimestampMax
		}
	}
	return maxTs
}

// GetMetadata returns file metadata (compatible with existing interface)
func (bsf *BlockStorageFile) GetMetadata() models.FileMetadata {
	bsf.mutex.Lock()
	defer bsf.mutex.Unlock()

	return models.FileMetadata{
		CreatedAt:                timeNow().Unix(),
		TotalEvents:              bsf.totalEvents,
		TotalUncompressedBytes:   bsf.totalUncompressed,
		TotalCompressedBytes:     bsf.totalCompressed,
		CompressionRatio:         float32(bsf.totalCompressed) / float32(bsf.totalUncompressed + 1),
		ResourceTypes:            bsf.getResourceTypes(),
		Namespaces:               bsf.getNamespaces(),
	}
}

// getResourceTypes extracts unique kinds from blocks
func (bsf *BlockStorageFile) getResourceTypes() map[string]bool {
	result := make(map[string]bool)
	for _, metadata := range bsf.blockMetadataList {
		for _, kind := range metadata.KindSet {
			result[kind] = true
		}
	}
	return result
}

// getNamespaces extracts unique namespaces from blocks
func (bsf *BlockStorageFile) getNamespaces() map[string]bool {
	result := make(map[string]bool)
	for _, metadata := range bsf.blockMetadataList {
		for _, ns := range metadata.NamespaceSet {
			result[ns] = true
		}
	}
	return result
}

// GetIndex returns a compatible index structure
func (bsf *BlockStorageFile) GetIndex() models.SparseTimestampIndex {
	bsf.mutex.Lock()
	defer bsf.mutex.Unlock()

	entries := make([]models.IndexEntry, 0)
	for _, block := range bsf.blocks {
		entries = append(entries, models.IndexEntry{
			Timestamp: block.TimestampMin,
			SegmentID: block.ID,
			Offset:    block.Offset,
		})
	}

	return models.SparseTimestampIndex{
		Entries:      entries,
		TotalSegments: int32(len(bsf.blocks)),
	}
}

// GetSegmentCount returns the number of blocks
func (bsf *BlockStorageFile) GetSegmentCount() int32 {
	bsf.mutex.Lock()
	defer bsf.mutex.Unlock()
	return int32(len(bsf.blocks))
}

// GetEventCount returns the total number of events
func (bsf *BlockStorageFile) GetEventCount() int64 {
	bsf.mutex.Lock()
	defer bsf.mutex.Unlock()
	return bsf.totalEvents
}

// GetBlocks returns all blocks (for testing and debugging)
func (bsf *BlockStorageFile) GetBlocks() []*Block {
	bsf.mutex.Lock()
	defer bsf.mutex.Unlock()
	return bsf.blocks
}

// GetBlockMetadata returns all block metadata
func (bsf *BlockStorageFile) GetBlockMetadata() []*BlockMetadata {
	bsf.mutex.Lock()
	defer bsf.mutex.Unlock()
	return bsf.blockMetadataList
}

// GetInvertedIndex returns the built inverted index
func (bsf *BlockStorageFile) GetInvertedIndex() *InvertedIndex {
	bsf.mutex.Lock()
	defer bsf.mutex.Unlock()
	return bsf.index
}

// GetCompressionStats returns compression statistics
func (bsf *BlockStorageFile) GetCompressionStats() map[string]interface{} {
	bsf.mutex.Lock()
	defer bsf.mutex.Unlock()

	ratio := 0.0
	savings := int64(0)
	if bsf.totalUncompressed > 0 {
		ratio = float64(bsf.totalCompressed) / float64(bsf.totalUncompressed)
		savings = bsf.totalUncompressed - bsf.totalCompressed
	}

	return map[string]interface{}{
		"total_uncompressed_bytes": bsf.totalUncompressed,
		"total_compressed_bytes":   bsf.totalCompressed,
		"compression_ratio":        ratio,
		"bytes_saved":              savings,
		"block_count":              len(bsf.blocks),
		"total_events":             bsf.totalEvents,
	}
}
