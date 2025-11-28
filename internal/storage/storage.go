package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/moritz/rpk/internal/logging"
	"github.com/moritz/rpk/internal/models"
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

	s.logger.Debug("Event written for %s/%s", event.Resource.Kind, event.Resource.Name)
	return nil
}

// getOrCreateCurrentFile gets or creates the storage file for the current hour
func (s *Storage) getOrCreateCurrentFile() (*BlockStorageFile, error) {
	now := time.Now()
	currentHour := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	// Check if we need to create a new file
	if s.currentFile == nil || s.currentFile.hourTimestamp != currentHour.Unix() {
		// Close the previous file if it exists
		if s.currentFile != nil {
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
func (s *Storage) DeleteOldFiles(maxAgeHours int) error {
	s.fileMutex.Lock()
	defer s.fileMutex.Unlock()

	files, err := s.getStorageFiles()
	if err != nil {
		return err
	}

	now := time.Now()
	maxAge := time.Duration(maxAgeHours) * time.Hour

	for _, filePath := range files {
		info, err := os.Stat(filePath)
		if err != nil {
			continue
		}

		if now.Sub(info.ModTime()) > maxAge {
			if err := os.Remove(filePath); err != nil {
				s.logger.Error("Failed to delete old file %s: %v", filePath, err)
			} else {
				s.logger.Info("Deleted old storage file: %s", filePath)
			}
		}
	}

	return nil
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
