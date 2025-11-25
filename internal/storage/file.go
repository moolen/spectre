package storage

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/moritz/rpk/internal/logging"
	"github.com/moritz/rpk/internal/models"
)

// StorageFile represents a single hourly storage file
type StorageFile struct {
	path           string
	hourTimestamp  int64
	file           *os.File
	segments       []*Segment
	currentSegment *Segment
	segmentID      int32
	logger         *logging.Logger
	compressor     *Compressor
	segmentSize    int64
	mutex          sync.Mutex
	metadata       models.FileMetadata
	index          models.SparseTimestampIndex
}

// NewStorageFile creates a new storage file for the specified hour
func NewStorageFile(path string, hourTimestamp int64, compressor *Compressor, segmentSize int64) (*StorageFile, error) {
	logger := logging.GetLogger("file")

	// Create or open the file
	file, err := os.Create(path)
	if err != nil {
		logger.Error("Failed to create storage file %s: %v", path, err)
		return nil, err
	}

	sf := &StorageFile{
		path:          path,
		hourTimestamp: hourTimestamp,
		file:          file,
		segments:      make([]*Segment, 0),
		segmentID:     0,
		logger:        logger,
		compressor:    compressor,
		segmentSize:   segmentSize,
		metadata: models.FileMetadata{
			CreatedAt:     timeNow().Unix(),
			ResourceTypes: make(map[string]bool),
			Namespaces:    make(map[string]bool),
		},
		index: models.SparseTimestampIndex{
			Entries:       make([]models.IndexEntry, 0),
			TotalSegments: 0,
		},
	}

	return sf, nil
}

// WriteEvent writes an event to the storage file
func (sf *StorageFile) WriteEvent(event *models.Event) error {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()

	// Create a new segment if needed
	if sf.currentSegment == nil {
		sf.currentSegment = NewSegment(sf.segmentID, sf.compressor, sf.segmentSize)
		sf.segmentID++
	}

	// Add event to current segment
	if err := sf.currentSegment.AddEvent(event); err != nil {
		return err
	}

	// Check if segment should be finalized
	if sf.currentSegment.IsReady() || timeNow().Unix() > sf.hourTimestamp+3600 {
		if err := sf.finalizeSegment(); err != nil {
			return err
		}

		// Create new segment
		sf.currentSegment = NewSegment(sf.segmentID, sf.compressor, sf.segmentSize)
		sf.segmentID++
	}

	// Update file metadata
	sf.metadata.TotalEvents++
	sf.metadata.ResourceTypes[event.Resource.Kind] = true
	sf.metadata.Namespaces[event.Resource.Namespace] = true

	return nil
}

// finalizeSegment finalizes the current segment and adds it to the file
func (sf *StorageFile) finalizeSegment() error {
	if sf.currentSegment == nil || sf.currentSegment.GetEventCount() == 0 {
		return nil
	}

	// Finalize the segment
	if err := sf.currentSegment.Finalize(); err != nil {
		return err
	}

	// Write segment to file
	offset, err := sf.file.Seek(0, 2) // Seek to end
	if err != nil {
		return fmt.Errorf("failed to seek in file: %w", err)
	}

	segmentData := sf.currentSegment.WriteToBuffer()
	if _, err := sf.file.Write(segmentData.Bytes()); err != nil {
		return fmt.Errorf("failed to write segment to file: %w", err)
	}

	// Add index entry
	indexEntry := models.IndexEntry{
		Timestamp: sf.currentSegment.startTimestamp,
		SegmentID: sf.currentSegment.ID,
		Offset:    offset,
	}
	sf.index.Entries = append(sf.index.Entries, indexEntry)
	sf.index.TotalSegments++

	// Update file metadata
	sf.metadata.TotalUncompressedBytes += sf.currentSegment.GetUncompressedSize()
	sf.metadata.TotalCompressedBytes += sf.currentSegment.GetCompressedSize()

	// Add segment to segments list
	sf.segments = append(sf.segments, sf.currentSegment)

	sf.logger.DebugWithFields("Finalized segment",
		logging.Field("segment_id", sf.currentSegment.ID),
		logging.Field("file", sf.path),
		logging.Field("uncompressed_bytes", sf.currentSegment.GetUncompressedSize()),
		logging.Field("compressed_bytes", sf.currentSegment.GetCompressedSize()))

	return nil
}

// Close closes the storage file and flushes pending data
func (sf *StorageFile) Close() error {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()

	// Finalize current segment
	if sf.currentSegment != nil && sf.currentSegment.GetEventCount() > 0 {
		if err := sf.finalizeSegment(); err != nil {
			sf.logger.Error("Failed to finalize segment: %v", err)
		}
	}

	// Write file metadata and index to file
	if err := sf.writeMetadata(); err != nil {
		sf.logger.Error("Failed to write metadata: %v", err)
	}

	// Close the file
	if err := sf.file.Close(); err != nil {
		sf.logger.Error("Failed to close file: %v", err)
		return err
	}

	sf.logger.InfoWithFields("Storage file closed and synced",
		logging.Field("file", sf.path),
		logging.Field("total_compressed_bytes", sf.metadata.TotalCompressedBytes),
		logging.Field("total_uncompressed_bytes", sf.metadata.TotalUncompressedBytes),
		logging.Field("segment_count", len(sf.segments)))
	return nil
}

// writeMetadata writes the file metadata and index to the end of the file
func (sf *StorageFile) writeMetadata() error {
	// Set finalized timestamp
	sf.metadata.FinalizedAt = timeNow().Unix()

	// Calculate compression ratio
	if sf.metadata.TotalUncompressedBytes > 0 {
		sf.metadata.CompressionRatio = float32(sf.metadata.TotalCompressedBytes) / float32(sf.metadata.TotalUncompressedBytes)
	}

	// Marshal metadata to JSON
	metadataJSON, err := json.Marshal(sf.metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Marshal index to JSON
	indexJSON, err := json.Marshal(sf.index)
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	// Write metadata length and data
	if err := binary.Write(sf.file, binary.LittleEndian, int32(len(metadataJSON))); err != nil {
		return fmt.Errorf("failed to write metadata length: %w", err)
	}
	if _, err := sf.file.Write(metadataJSON); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	// Write index length and data
	if err := binary.Write(sf.file, binary.LittleEndian, int32(len(indexJSON))); err != nil {
		return fmt.Errorf("failed to write index length: %w", err)
	}
	if _, err := sf.file.Write(indexJSON); err != nil {
		return fmt.Errorf("failed to write index: %w", err)
	}

	// Write file footer marker
	footer := []byte("RPK_EOF")
	if _, err := sf.file.Write(footer); err != nil {
		return fmt.Errorf("failed to write footer: %w", err)
	}

	return nil
}

// GetMetadata returns the file metadata
func (sf *StorageFile) GetMetadata() models.FileMetadata {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	return sf.metadata
}

// GetIndex returns the sparse timestamp index
func (sf *StorageFile) GetIndex() models.SparseTimestampIndex {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	return sf.index
}

// GetSegmentCount returns the number of segments in the file
func (sf *StorageFile) GetSegmentCount() int32 {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	return int32(len(sf.segments))
}

// GetEventCount returns the total number of events in the file
func (sf *StorageFile) GetEventCount() int64 {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	return sf.metadata.TotalEvents
}

// timeNow returns the current time
func timeNow() time.Time {
	return time.Now()
}
