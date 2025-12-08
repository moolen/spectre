package storage

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// Storage manages persistent event storage
type Storage struct {
	dataDir     string
	logger      *logging.Logger
	currentFile *BlockStorageFile
	fileMutex   sync.RWMutex
	blockSize   int64
}

// New creates a new Storage instance
func New(dataDir string, blockSize int64) (*Storage, error) {
	logger := logging.GetLogger("storage")

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		logger.Error("Failed to create data directory: %v", err)
		return nil, err
	}

	s := &Storage{
		dataDir:   dataDir,
		logger:    logger,
		blockSize: blockSize,
	}

	logger.Info("Storage initialized with directory: %s", dataDir)
	return s, nil
}

// WriteEvent writes an event to storage
func (s *Storage) WriteEvent(event *models.Event) error {
	s.fileMutex.Lock()
	defer s.fileMutex.Unlock()

	// Get or create the current hourly file
	currentFile, err := s.getOrCreateCurrentFile()
	if err != nil {
		s.logger.Error("Failed to get or create current file: %v", err)
		return err
	}

	// Write event to the file
	if err := currentFile.WriteEvent(event); err != nil {
		s.logger.Error("Failed to write event to file: %v", err)
		return err
	}

	return nil
}

// getOrCreateCurrentFile gets or creates the storage file for the current hour
func (s *Storage) getOrCreateCurrentFile() (*BlockStorageFile, error) {
	now := time.Now()
	currentHour := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	// Check if we need to create a new file
	if s.currentFile == nil || s.currentFile.hourTimestamp != currentHour.Unix() {
		// Close the previous file if it exists and capture its final states
		var carryoverStates map[string]*ResourceLastState
		if s.currentFile != nil {
			// Extract final states before closing
			carryoverStates = s.currentFile.finalResourceStates
			if err := s.currentFile.Close(); err != nil {
				s.logger.Error("Failed to close previous file: %v", err)
			}
		}

		// Create a new file for the current hour
		filePath := filepath.Join(s.dataDir, fmt.Sprintf("%04d-%02d-%02d-%02d.bin",
			now.Year(), now.Month(), now.Day(), now.Hour()))

		newFile, err := NewBlockStorageFile(filePath, currentHour.Unix(), s.blockSize)
		if err != nil {
			return nil, err
		}

		// Carry over state snapshots from previous hour
		if len(carryoverStates) > 0 {
			newFile.finalResourceStates = carryoverStates
			s.logger.Info("Carried over %d resource states from previous hour to new file", len(carryoverStates))
		}

		s.currentFile = newFile
		s.logger.Info("Created new storage file: %s", filePath)
	}

	return s.currentFile, nil
}

// Close closes the storage and flushes any pending data
func (s *Storage) Close() error {
	s.fileMutex.Lock()
	defer s.fileMutex.Unlock()

	if s.currentFile != nil {
		// Close is idempotent, so safe to call multiple times
		if err := s.currentFile.Close(); err != nil {
			s.logger.Error("Failed to close storage file: %v", err)
			// Don't return error if already closed
		}
		s.currentFile = nil
	}

	s.logger.Info("Storage closed")
	return nil
}

// getStorageFiles returns all storage files in the data directory
func (s *Storage) getStorageFiles() ([]string, error) {
	var files []string

	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read data directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".bin" {
			files = append(files, filepath.Join(s.dataDir, entry.Name()))
		}
	}

	return files, nil
}

// GetStorageStats returns statistics about the storage
func (s *Storage) GetStorageStats() (map[string]interface{}, error) {
	s.fileMutex.RLock()
	defer s.fileMutex.RUnlock()

	stats := make(map[string]interface{})

	// Get directory size
	dirSize, err := s.getDirectorySize(s.dataDir)
	if err != nil {
		return nil, err
	}

	stats["dataDir"] = s.dataDir
	stats["totalSizeBytes"] = dirSize
	stats["totalSizeMB"] = float64(dirSize) / 1024.0 / 1024.0

	// Count files
	files, err := s.getStorageFiles()
	if err != nil {
		return nil, err
	}
	stats["fileCount"] = len(files)

	return stats, nil
}

// getDirectorySize returns the total size of all files in a directory
func (s *Storage) getDirectorySize(dir string) (int64, error) {
	var totalSize int64

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			totalSize += info.Size()
		}

		return nil
	})

	return totalSize, err
}

// ListFiles returns a list of all storage files
func (s *Storage) ListFiles() ([]string, error) {
	s.fileMutex.RLock()
	defer s.fileMutex.RUnlock()

	return s.getStorageFiles()
}

// DeleteOldFiles deletes storage files older than the specified age
// This method uses the hour timestamp from the filename rather than file modification time
// to properly handle imported historical data
func (s *Storage) DeleteOldFiles(maxAgeHours int) error {
	s.fileMutex.Lock()
	defer s.fileMutex.Unlock()

	files, err := s.getStorageFiles()
	if err != nil {
		return err
	}

	now := time.Now()
	maxAge := time.Duration(maxAgeHours) * time.Hour
	cutoffTime := now.Add(-maxAge)

	for _, filePath := range files {
		// Extract hour timestamp from filename to handle imported data correctly
		hourTimestamp, err := s.extractHourFromFilename(filePath)
		if err != nil {
			// If we can't parse the filename, fall back to file modification time
			s.logger.Warn("Failed to extract hour from filename %s, using mod time: %v", filePath, err)
			info, statErr := os.Stat(filePath)
			if statErr != nil {
				continue
			}
			if now.Sub(info.ModTime()) > maxAge {
				if err := os.Remove(filePath); err != nil {
					s.logger.Error("Failed to delete old file %s: %v", filePath, err)
				} else {
					s.logger.Info("Deleted old storage file: %s", filePath)
				}
			}
			continue
		}

		// Check if the hour represented by this file is older than the cutoff
		fileTime := time.Unix(hourTimestamp, 0)
		if fileTime.Before(cutoffTime) {
			if err := os.Remove(filePath); err != nil {
				s.logger.Error("Failed to delete old file %s: %v", filePath, err)
			} else {
				s.logger.Info("Deleted old storage file: %s (hour: %s)", filePath, fileTime.Format("2006-01-02 15:04"))
			}
		}
	}

	return nil
}

// CleanupOldStateSnapshots removes state snapshots older than maxAgeDays from all storage files
// This keeps the state snapshots current and prevents them from growing indefinitely
func (s *Storage) CleanupOldStateSnapshots(maxAgeDays int) error {
	s.fileMutex.Lock()
	defer s.fileMutex.Unlock()

	files, err := s.getStorageFiles()
	if err != nil {
		return err
	}

	now := time.Now()
	maxAge := time.Duration(maxAgeDays) * 24 * time.Hour
	cutoffTime := now.Add(-maxAge)
	cutoffTimestampNs := cutoffTime.UnixNano()

	for _, filePath := range files {
		// Read the file to get its index section
		reader, err := NewBlockReader(filePath)
		if err != nil {
			s.logger.Warn("Failed to read file %s for state cleanup: %v", filePath, err)
			continue
		}

		fileData, err := reader.ReadFile()
		reader.Close()

		if err != nil {
			s.logger.Warn("Failed to read index section from file %s: %v", filePath, err)
			continue
		}

		// Check if there are any state snapshots to clean
		if len(fileData.IndexSection.FinalResourceStates) == 0 {
			continue
		}

		// Filter out states older than cutoff
		cleanedStates := make(map[string]*ResourceLastState)
		statesRemoved := 0

		for key, state := range fileData.IndexSection.FinalResourceStates {
			// Keep states that are:
			// 1. Newer than the cutoff time, OR
			// 2. Represent non-deleted resources (they're still relevant for consistent view)
			if state.Timestamp > cutoffTimestampNs {
				cleanedStates[key] = state
			} else if state.EventType != "DELETE" {
				// Keep non-deleted resources even if old - they might still be relevant
				cleanedStates[key] = state
			} else {
				// Remove deleted resources that are older than cutoff
				statesRemoved++
			}
		}

		// If nothing was removed, skip rewriting
		if statesRemoved == 0 {
			continue
		}

		// Rewrite the index section with cleaned states
		if err := s.rewriteFileIndexSection(filePath, fileData, cleanedStates); err != nil {
			s.logger.Error("Failed to rewrite index section for file %s: %v", filePath, err)
			continue
		}

		s.logger.Info("Cleaned %d old state snapshots from file %s", statesRemoved, filePath)
	}

	return nil
}

// rewriteFileIndexSection rewrites the index section of a file with updated state snapshots
func (s *Storage) rewriteFileIndexSection(filePath string, fileData *StorageFileData, cleanedStates map[string]*ResourceLastState) error {
	// Update the cleaned states in the index section
	fileData.IndexSection.FinalResourceStates = cleanedStates

	// Open file for writing
	file, err := os.OpenFile(filePath, os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file for rewriting: %w", err)
	}
	defer file.Close()

	// Seek to where the old index section started
	indexOffset := fileData.Footer.IndexSectionOffset
	if _, err := file.Seek(indexOffset, 0); err != nil {
		return fmt.Errorf("failed to seek to index section: %w", err)
	}

	// Truncate file at the start of index section
	if err := file.Truncate(indexOffset); err != nil {
		return fmt.Errorf("failed to truncate file: %w", err)
	}

	// Write new index section
	bytesWritten, err := WriteIndexSection(file, fileData.IndexSection)
	if err != nil {
		return fmt.Errorf("failed to write index section: %w", err)
	}

	// Write new footer
	footer := &FileFooter{
		IndexSectionOffset: indexOffset,
		IndexSectionLength: int32(bytesWritten),
		MagicBytes:         FileFooterMagic,
	}

	if err := WriteFileFooter(file, footer); err != nil {
		return fmt.Errorf("failed to write file footer: %w", err)
	}

	return nil
}

// extractHourFromFilename extracts the hour timestamp from a filename
// Expected format: YYYY-MM-DD-HH.bin
func (s *Storage) extractHourFromFilename(filePath string) (int64, error) {
	filename := filepath.Base(filePath)
	// Remove .bin extension
	filename = strings.TrimSuffix(filename, ".bin")

	// Parse the date components
	var year, month, day, hour int
	_, err := fmt.Sscanf(filename, "%04d-%02d-%02d-%02d", &year, &month, &day, &hour)
	if err != nil {
		return 0, fmt.Errorf("invalid filename format: %w", err)
	}

	// Create time and convert to Unix timestamp using local timezone (matches getOrCreateCurrentFile)
	t := time.Date(year, time.Month(month), day, hour, 0, 0, 0, time.Local)
	return t.Unix(), nil
}

// Start implements the lifecycle.Component interface
// Initializes the storage component for use
func (s *Storage) Start(ctx context.Context) error {
	s.logger.Info("Starting storage component")

	// Check context isn't already cancelled
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Storage is already initialized in New(), so just verify it's ready
	s.logger.Info("Storage component ready")
	return nil
}

// Stop implements the lifecycle.Component interface
// Gracefully shuts down the storage component
func (s *Storage) Stop(ctx context.Context) error {
	s.logger.Info("Stopping storage component")

	done := make(chan error, 1)

	go func() {
		done <- s.Close()
	}()

	select {
	case err := <-done:
		if err != nil {
			s.logger.Error("Storage component shutdown error: %v", err)
			return err
		}
		s.logger.Info("Storage component stopped")
		return nil
	case <-ctx.Done():
		s.logger.Warn("Storage component shutdown timeout")
		return ctx.Err()
	}
}

// Name implements the lifecycle.Component interface
// Returns the human-readable name of the storage component
func (s *Storage) Name() string {
	return "Storage"
}

// GetInMemoryEvents returns events from the current in-memory buffer that match the query
// This allows querying unflushed/buffered data for low-latency queries
// For restored files, it also includes events from restored blocks that are still on disk
func (s *Storage) GetInMemoryEvents(query *models.QueryRequest) ([]models.Event, error) {
	s.fileMutex.RLock()
	defer s.fileMutex.RUnlock()

	if s.currentFile == nil {
		return []models.Event{}, nil
	}

	var allEvents []*models.Event

	// Get events from restored blocks if file was restored (has blockMetadataList but blocks is empty)
	s.currentFile.mutex.Lock()
	hasRestoredBlocks := len(s.currentFile.blockMetadataList) > 0 && len(s.currentFile.blocks) == 0
	if hasRestoredBlocks {
		// File was restored - read events from restored blocks on disk
		restoredEvents, err := s.readEventsFromRestoredBlocks(s.currentFile, query)
		if err != nil {
			s.currentFile.mutex.Unlock()
			s.logger.Warn("Failed to read events from restored blocks: %v", err)
			// Continue with just buffer events
		} else {
			allEvents = append(allEvents, restoredEvents...)
		}
	}

	// Get events from finalized blocks that are in memory (current file still being written)
	// These blocks have been finalized to disk but the file hasn't been closed with a footer yet
	if len(s.currentFile.blocks) > 0 {
		startTimeNs := query.StartTimestamp * 1e9
		endTimeNs := query.EndTimestamp * 1e9

		for _, block := range s.currentFile.blocks {
			if block.Metadata == nil {
				continue
			}

			// Check time range overlap
			if block.Metadata.TimestampMax < startTimeNs || block.Metadata.TimestampMin > endTimeNs {
				continue
			}

			// Decompress the block data
			decompressedData, err := DecompressBlock(block)
			if err != nil {
				s.logger.Warn("Failed to decompress finalized block: %v", err)
				continue
			}

			// Parse events from length-prefixed protobuf format
			offset := 0
			for offset < len(decompressedData) {
				// Parse varint length
				length, n := binary.Uvarint(decompressedData[offset:])
				if n <= 0 {
					break // End of data
				}
				offset += n

				if offset+int(length) > len(decompressedData) {
					s.logger.Warn("Invalid message length in finalized block: %d at offset %d", length, offset)
					break
				}

				messageData := decompressedData[offset : offset+int(length)]
				offset += int(length)

				event := &models.Event{}
				if err := event.UnmarshalProtobuf(messageData); err != nil {
					s.logger.Warn("Failed to unmarshal event from finalized block: %v", err)
					continue
				}

				// Filter by time range and resource filters
				if event.Timestamp < startTimeNs || event.Timestamp > endTimeNs {
					continue
				}
				if !query.Filters.Matches(event.Resource) {
					continue
				}
				allEvents = append(allEvents, event)
			}
		}
	}

	// Get events from the current buffer
	currentBuffer := s.currentFile.currentBuffer
	if currentBuffer != nil && currentBuffer.GetEventCount() > 0 {
		bufferEvents, err := currentBuffer.GetEvents()
		if err != nil {
			s.currentFile.mutex.Unlock()
			s.logger.Error("Failed to get events from buffer: %v", err)
			return []models.Event{}, err
		}
		allEvents = append(allEvents, bufferEvents...)
	}
	s.currentFile.mutex.Unlock()

	// Filter events based on the query
	if len(allEvents) == 0 {
		return []models.Event{}, nil
	}

	return s.filterEvents(allEvents, query), nil
}

// filterEvents filters a list of events based on the query
func (s *Storage) filterEvents(events []*models.Event, query *models.QueryRequest) []models.Event {
	if len(events) == 0 {
		return []models.Event{}
	}

	startTimeNs := query.StartTimestamp * 1e9
	endTimeNs := query.EndTimestamp * 1e9

	var matchingEvents []models.Event
	for _, event := range events {
		// Filter by time range
		if event.Timestamp < startTimeNs || event.Timestamp > endTimeNs {
			continue
		}

		// Filter by resource filters using the same logic as FilterEngine
		if !query.Filters.Matches(event.Resource) {
			continue
		}

		matchingEvents = append(matchingEvents, *event)
	}

	return matchingEvents
}

// readEventsFromRestoredBlocks reads events from restored blocks on disk
// This is used when a file was restored from a complete file and is open for appending
func (s *Storage) readEventsFromRestoredBlocks(bsf *BlockStorageFile, query *models.QueryRequest) ([]*models.Event, error) {
	if len(bsf.blockMetadataList) == 0 {
		return []*models.Event{}, nil
	}

	// Open file for reading (separate from the write handle)
	reader, err := NewBlockReader(bsf.path)
	if err != nil {
		return nil, fmt.Errorf("failed to create block reader: %w", err)
	}
	defer reader.Close()

	var allEvents []*models.Event
	startTimeNs := query.StartTimestamp * 1e9
	endTimeNs := query.EndTimestamp * 1e9

	// Convert query filters to map format for block matching
	filters := make(map[string]string)
	if query.Filters.Kind != "" {
		filters["kind"] = query.Filters.Kind
	}
	if query.Filters.Namespace != "" {
		filters["namespace"] = query.Filters.Namespace
	}
	if query.Filters.Group != "" {
		filters["group"] = query.Filters.Group
	}

	// Get candidate blocks using inverted index (if available)
	var candidateBlockIDs []int32
	if bsf.index != nil && len(filters) > 0 {
		candidateBlockIDs = GetCandidateBlocks(bsf.index, filters)
	} else {
		// No filters or no index - include all blocks
		for _, meta := range bsf.blockMetadataList {
			candidateBlockIDs = append(candidateBlockIDs, meta.ID)
		}
	}

	// Read events from candidate blocks
	for _, blockID := range candidateBlockIDs {
		// Find metadata for this block
		var metadata *BlockMetadata
		for _, bm := range bsf.blockMetadataList {
			if bm.ID == blockID {
				metadata = bm
				break
			}
		}

		if metadata == nil {
			continue
		}

		// Check time range overlap
		if metadata.TimestampMax < startTimeNs || metadata.TimestampMin > endTimeNs {
			continue
		}

		// Read block events
		events, err := reader.ReadBlockEvents(metadata)
		if err != nil {
			s.logger.Warn("Failed to read restored block %d: %v", blockID, err)
			continue
		}

		// Filter events based on the query
		for _, event := range events {
			if event.Timestamp < startTimeNs || event.Timestamp > endTimeNs {
				continue
			}
			if !query.Filters.Matches(event.Resource) {
				continue
			}
			allEvents = append(allEvents, event)
		}
	}

	return allEvents, nil
}
