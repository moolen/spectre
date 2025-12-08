package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// BlockStorageFile represents a block-based storage file for one hour
type BlockStorageFile struct {
	path                string
	hourTimestamp       int64
	file                *os.File
	blocks              []*Block
	currentBuffer       *EventBuffer
	blockID             int32
	logger              *logging.Logger
	blockSize           int64
	mutex               sync.Mutex
	blockMetadataList   []*BlockMetadata
	index               *InvertedIndex
	startOffset         int64
	encodingFormat      string                        // "json" or "protobuf"
	finalResourceStates map[string]*ResourceLastState // Final state snapshots for resources in this file

	// Metrics
	totalUncompressed int64
	totalCompressed   int64
	totalEvents       int64
}

// openExistingBlockStorageFile opens an existing complete file for appending
func openExistingBlockStorageFile(path string, fileData *StorageFileData, hourTimestamp int64, blockSizeBytes int64) (*BlockStorageFile, error) {
	logger := logging.GetLogger("block_storage")

	header := fileData.Header
	indexSection := fileData.IndexSection
	footer := fileData.Footer

	// Calculate where blocks end (before index section)
	blocksEndOffset := footer.IndexSectionOffset

	// Open file for read/write
	file, err := os.OpenFile(path, os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open existing file: %w", err)
	}

	// Truncate at blocks end (removes old index section and footer)
	if err := file.Truncate(blocksEndOffset); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to truncate file for appending: %w", err)
	}

	// Seek to end of blocks for appending
	if _, err := file.Seek(blocksEndOffset, 0); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to seek to end of blocks: %w", err)
	}

	// Reconstruct BlockStorageFile state from disk
	blockMetadataList := indexSection.BlockMetadata
	nextBlockID := int32(len(blockMetadataList))

	// Reconstruct inverted index from metadata
	invertedIndex := indexSection.InvertedIndexes
	if invertedIndex == nil {
		// Rebuild if missing
		invertedIndex = BuildInvertedIndexes(blockMetadataList)
	}

	stats := indexSection.Statistics
	totalEvents := int64(0)
	totalUncompressed := int64(0)
	totalCompressed := int64(0)

	if stats != nil {
		totalEvents = stats.TotalEvents
		totalUncompressed = stats.TotalUncompressedBytes
		totalCompressed = stats.TotalCompressedBytes
	}

	// Get block size from header or use provided
	actualBlockSize := blockSizeBytes
	if header.BlockSize > 0 {
		actualBlockSize = int64(header.BlockSize)
	}

	// Restore final resource states from index section
	finalResourceStates := indexSection.FinalResourceStates
	if finalResourceStates == nil {
		finalResourceStates = make(map[string]*ResourceLastState)
	}

	bsf := &BlockStorageFile{
		path:                path,
		hourTimestamp:       hourTimestamp,
		file:                file,
		blocks:              make([]*Block, 0), // We don't need actual block data in memory
		currentBuffer:       NewEventBuffer(actualBlockSize),
		blockID:             nextBlockID,
		logger:              logger,
		blockSize:           actualBlockSize,
		blockMetadataList:   blockMetadataList,
		index:               invertedIndex,
		startOffset:         int64(FileHeaderSize),
		encodingFormat:      "protobuf",
		finalResourceStates: finalResourceStates,
		totalEvents:         totalEvents,
		totalUncompressed:   totalUncompressed,
		totalCompressed:     totalCompressed,
	}

	logger.InfoWithFields("Restored existing complete file for appending",
		logging.Field("file", path),
		logging.Field("existing_blocks", len(blockMetadataList)),
		logging.Field("existing_events", totalEvents),
		logging.Field("next_block_id", nextBlockID))

	return bsf, nil
}

// NewBlockStorageFile creates a new block-based storage file or opens existing complete file
func NewBlockStorageFile(path string, hourTimestamp int64, blockSizeBytes int64) (*BlockStorageFile, error) {
	logger := logging.GetLogger("block_storage")

	// Check if file already exists
	if _, err := os.Stat(path); err == nil {
		// File exists - check if it's complete (has footer)
		reader, err := NewBlockReader(path)
		if err == nil {
			// Try to read complete file structure
			fileData, err := reader.ReadFile()
			reader.Close()

			if err == nil {
				// File is complete - restore state and open for appending
				logger.Info("Found existing complete file, restoring state: %s", path)
				return openExistingBlockStorageFile(path, fileData, hourTimestamp, blockSizeBytes)
			}
			// File exists but is incomplete (no footer) - likely from a crash
			// Rename the incomplete file to preserve it (with timestamp suffix)
			timestamp := time.Now().Unix()
			backupPath := fmt.Sprintf("%s.incomplete.%d", path, timestamp)
			if renameErr := os.Rename(path, backupPath); renameErr != nil {
				logger.Warn("Failed to rename incomplete file %s to %s: %v", path, backupPath, renameErr)
				return nil, fmt.Errorf("file %s exists but is incomplete and could not be backed up: %w", path, renameErr)
			}
			logger.Warn("Found incomplete file %s from previous run, renamed to %s", path, backupPath)
		} else {
			// File exists but couldn't read it - might be corrupted or wrong format
			// Rename it to be safe
			timestamp := time.Now().Unix()
			backupPath := fmt.Sprintf("%s.corrupted.%d", path, timestamp)
			if renameErr := os.Rename(path, backupPath); renameErr != nil {
				logger.Warn("Failed to rename existing file %s to %s: %v", path, backupPath, renameErr)
				return nil, fmt.Errorf("file %s exists and could not be backed up: %w", path, renameErr)
			}
			logger.Warn("Found existing file %s that could not be read, renamed to %s", path, backupPath)
		}
	}

	// Create or open the file
	file, err := os.Create(path)
	if err != nil {
		logger.Error("Failed to create block storage file %s: %v", path, err)
		return nil, err
	}

	bsf := &BlockStorageFile{
		path:                path,
		hourTimestamp:       hourTimestamp,
		file:                file,
		blocks:              make([]*Block, 0),
		currentBuffer:       NewEventBuffer(blockSizeBytes),
		blockID:             0,
		logger:              logger,
		blockSize:           blockSizeBytes,
		blockMetadataList:   make([]*BlockMetadata, 0),
		index:               &InvertedIndex{},
		startOffset:         0,
		encodingFormat:      "protobuf",
		finalResourceStates: make(map[string]*ResourceLastState),
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
	block, err := bsf.currentBuffer.Finalize(bsf.blockID, "gzip", bsf.encodingFormat)
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

	// Check if already closed (idempotent)
	if bsf.file == nil {
		return nil
	}

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
		// Check if error is due to file already being closed
		// Different OSes return different error messages
		errStr := err.Error()
		if errStr != "file already closed" && errStr != "use of closed file" && errStr != "invalid argument" {
			bsf.logger.Error("Failed to close file: %v", err)
			// Don't return error for already-closed files, but mark as closed
		}
	}

	// Mark file as closed to make subsequent calls idempotent
	bsf.file = nil

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

// extractFinalResourceStates extracts the final state of each resource from all blocks
// Returns a map where key is "group/version/kind/namespace/name" and value is the ResourceLastState
func (bsf *BlockStorageFile) extractFinalResourceStates() (map[string]*ResourceLastState, error) {
	finalStates := make(map[string]*ResourceLastState)

	// If no blocks exist, return empty map
	if len(bsf.blockMetadataList) == 0 {
		return finalStates, nil
	}

	// For each block, read its events and track the latest state per resource
	reader, err := NewBlockReader(bsf.path)
	if err != nil {
		return nil, fmt.Errorf("failed to create block reader: %w", err)
	}
	defer reader.Close()

	for _, metadata := range bsf.blockMetadataList {
		events, err := reader.ReadBlockEvents(metadata)
		if err != nil {
			bsf.logger.Warn("Failed to read block %d for state extraction: %v", metadata.ID, err)
			continue
		}

		// For each event, update the final state for that resource
		for _, event := range events {
			// Create resource key: group/version/kind/namespace/name
			resourceKey := fmt.Sprintf("%s/%s/%s/%s/%s",
				event.Resource.Group,
				event.Resource.Version,
				event.Resource.Kind,
				event.Resource.Namespace,
				event.Resource.Name,
			)

			// Store or update the final state
			// Events are processed in order within a block, so later events overwrite earlier ones
			finalStates[resourceKey] = &ResourceLastState{
				UID:          event.Resource.UID,
				EventType:    string(event.Type),
				Timestamp:    event.Timestamp,
				ResourceData: event.Data,
			}
		}
	}

	return finalStates, nil
}

// writeIndexSection writes the index section to the end of the file
func (bsf *BlockStorageFile) writeIndexSection() error {
	// Get current file offset (start of index section)
	indexOffset, err := bsf.file.Seek(0, 1)
	if err != nil {
		return fmt.Errorf("failed to get index offset: %w", err)
	}

	// Create statistics
	// Use blockMetadataList length as it works for both normal operation and restored files
	blockCount := len(bsf.blockMetadataList)
	if blockCount == 0 && len(bsf.blocks) > 0 {
		// Fallback to blocks if metadata list is empty (shouldn't happen normally)
		blockCount = len(bsf.blocks)
	}
	stats := &IndexStatistics{
		TotalBlocks:            int32(blockCount),
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

	// Extract final resource states for consistent view across hour boundaries
	finalResourceStates, err := bsf.extractFinalResourceStates()
	if err != nil {
		bsf.logger.Warn("Failed to extract final resource states: %v", err)
		// Continue without state snapshots - not fatal
		finalResourceStates = make(map[string]*ResourceLastState)
	}

	// Create index section
	indexSection := &IndexSection{
		FormatVersion:       DefaultFormatVersion,
		BlockMetadata:       bsf.blockMetadataList,
		InvertedIndexes:     bsf.index,
		Statistics:          stats,
		FinalResourceStates: finalResourceStates,
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
	// Check blockMetadataList first (used when restored from disk)
	if len(bsf.blockMetadataList) > 0 {
		minTs := bsf.blockMetadataList[0].TimestampMin
		for _, metadata := range bsf.blockMetadataList {
			if metadata.TimestampMin > 0 && metadata.TimestampMin < minTs {
				minTs = metadata.TimestampMin
			}
		}
		return minTs
	}

	// Fall back to blocks (used during normal operation)
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
	// Check blockMetadataList first (used when restored from disk)
	if len(bsf.blockMetadataList) > 0 {
		maxTs := bsf.blockMetadataList[0].TimestampMax
		for _, metadata := range bsf.blockMetadataList {
			if metadata.TimestampMax > maxTs {
				maxTs = metadata.TimestampMax
			}
		}
		return maxTs
	}

	// Fall back to blocks (used during normal operation)
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
		CreatedAt:              time.Now().Unix(),
		TotalEvents:            bsf.totalEvents,
		TotalUncompressedBytes: bsf.totalUncompressed,
		TotalCompressedBytes:   bsf.totalCompressed,
		CompressionRatio:       float32(bsf.totalCompressed) / float32(bsf.totalUncompressed+1),
		ResourceTypes:          bsf.getResourceTypes(),
		Namespaces:             bsf.getNamespaces(),
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

// GetSparseTimestampIndex returns a compatible index structure
func (bsf *BlockStorageFile) GetSparseTimestampIndex() models.SparseTimestampIndex {
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
		Entries:       entries,
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
