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

// EventCallback is called when an event is written to storage
type EventCallback func(event models.Event) error

// Storage manages persistent event storage
type Storage struct {
	dataDir     string
	logger      *logging.Logger
	currentFile *BlockStorageFile
	fileMutex   sync.RWMutex
	blockSize   int64
	fileIndex   *FileIndex                  // Index for fast file selection
	hourFiles   map[int64]*BlockStorageFile // Cache of open hourly files

	// Event callbacks
	callbacks []EventCallback
	callbackMutex sync.RWMutex

	// Background task management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Lifecycle management
	closed bool
}

// New creates a new Storage instance
func New(dataDir string, blockSize int64) (*Storage, error) {
	logger := logging.GetLogger("storage")

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil { //nolint:gosec // 0755 is standard for data directories
		logger.Error("Failed to create data directory: %v", err)
		return nil, err
	}

	// Initialize file index
	fileIndex := NewFileIndex(dataDir, logger)

	s := &Storage{
		dataDir:   dataDir,
		logger:    logger,
		blockSize: blockSize,
		fileIndex: fileIndex,
		hourFiles: make(map[int64]*BlockStorageFile),
	}

	// Rebuild index on startup if it's empty or outdated
	if fileIndex.Count() == 0 {
		logger.Info("File index is empty, rebuilding...")
		if err := fileIndex.Rebuild(dataDir, s.extractHourFromFilename); err != nil {
			logger.Warn("Failed to rebuild file index: %v", err)
		}
	}

	logger.Info("Storage initialized with directory: %s", dataDir)
	return s, nil
}

// RegisterCallback registers a callback to be called when events are written
func (s *Storage) RegisterCallback(callback EventCallback) {
	s.callbackMutex.Lock()
	defer s.callbackMutex.Unlock()
	s.callbacks = append(s.callbacks, callback)
	s.logger.Info("Registered event callback (total callbacks: %d)", len(s.callbacks))
}

// notifyCallbacks calls all registered callbacks with the event
func (s *Storage) notifyCallbacks(event models.Event) {
	s.callbackMutex.RLock()
	callbacks := s.callbacks
	s.callbackMutex.RUnlock()

	for _, callback := range callbacks {
		if err := callback(event); err != nil {
			s.logger.Warn("Event callback failed: %v", err)
		}
	}
}

// WriteEvent writes an event to storage
// Events are routed to the correct hourly file based on their timestamp
func (s *Storage) WriteEvent(event *models.Event) error {
	s.fileMutex.Lock()
	defer s.fileMutex.Unlock()

	// Determine which hour this event belongs to based on its timestamp
	eventTime := time.Unix(0, event.Timestamp)
	eventHour := time.Date(eventTime.Year(), eventTime.Month(), eventTime.Day(),
		eventTime.Hour(), 0, 0, 0, eventTime.Location())
	eventHourTimestamp := eventHour.Unix()

	// Get or create the file for this event's hour
	hourFile, err := s.getOrCreateHourFile(eventHourTimestamp)
	if err != nil {
		s.logger.Error("Failed to get or create hour file for timestamp %d: %v", eventHourTimestamp, err)
		return err
	}

	// Validate that event timestamp falls within hour boundaries
	hourStartNs := eventHourTimestamp * 1e9
	hourEndNs := hourStartNs + (3600 * 1e9)
	if event.Timestamp < hourStartNs || event.Timestamp >= hourEndNs {
		return fmt.Errorf("event timestamp %d is outside hour boundaries [%d, %d)",
			event.Timestamp, hourStartNs, hourEndNs)
	}

	// Write event to the file
	if err := hourFile.WriteEvent(event); err != nil {
		s.logger.Error("Failed to write event to file: %v", err)
		return err
	}

	// Update the current file pointer if this is the latest hour
	now := time.Now()
	currentHour := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	if eventHourTimestamp == currentHour.Unix() {
		s.currentFile = hourFile
	}

	// Notify callbacks about the new event
	s.notifyCallbacks(*event)

	return nil
}

// getOrCreateHourFile gets or creates a storage file for a specific hour
// This allows writing events to any hour, not just the current one
func (s *Storage) getOrCreateHourFile(hourTimestamp int64) (*BlockStorageFile, error) {
	// Check if file is already open
	if existingFile, ok := s.hourFiles[hourTimestamp]; ok {
		return existingFile, nil
	}

	// Create time from hour timestamp
	hourTime := time.Unix(hourTimestamp, 0)

	// Create file path
	filePath := filepath.Join(s.dataDir, fmt.Sprintf("%04d-%02d-%02d-%02d.bin",
		hourTime.Year(), hourTime.Month(), hourTime.Day(), hourTime.Hour()))

	// Check if file already exists on disk
	if _, err := os.Stat(filePath); err == nil {
		// File exists, open it for appending
		s.logger.Debug("Opening existing file for hour %s", hourTime.Format("2006-01-02 15:04"))

		reader, err := NewBlockReader(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to open reader for existing file: %w", err)
		}

		fileData, err := reader.ReadFile()
		if closeErr := reader.Close(); closeErr != nil {
			s.logger.Warn("Failed to close reader: %v", closeErr)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read existing file data: %w", err)
		}

		newFile, err := openExistingBlockStorageFile(filePath, fileData, hourTimestamp, s.blockSize)
		if err != nil {
			return nil, fmt.Errorf("failed to reopen file for appending: %w", err)
		}

		s.hourFiles[hourTimestamp] = newFile
		return newFile, nil
	}

	// File doesn't exist, create new one
	s.logger.Info("Creating new storage file for hour %s: %s",
		hourTime.Format("2006-01-02 15:04"), filePath)

	// Get state carryover from previous hour if creating current hour file
	var carryoverStates map[string]*ResourceLastState
	prevHourTimestamp := hourTimestamp - 3600

	// First try to get from open files (in-memory)
	if prevFile, ok := s.hourFiles[prevHourTimestamp]; ok {
		// Previous hour file is still open - extract states dynamically
		var err error
		carryoverStates, err = prevFile.extractFinalResourceStates()
		if err != nil {
			s.logger.Warn("Failed to extract states from open previous hour file: %v", err)
			carryoverStates = make(map[string]*ResourceLastState)
		} else {
			s.logger.Debug("Extracted %d resource states from previous hour (in-memory) to carry over", len(carryoverStates))
		}
	} else {
		// If previous hour file is not open, try to load it from disk
		prevHourTime := time.Unix(prevHourTimestamp, 0)
		prevFilePath := filepath.Join(s.dataDir, fmt.Sprintf("%04d-%02d-%02d-%02d.bin",
			prevHourTime.Year(), prevHourTime.Month(), prevHourTime.Day(), prevHourTime.Hour()))

		s.logger.Debug("Checking for previous hour file to load states: %s", prevFilePath)
		if _, err := os.Stat(prevFilePath); err == nil {
			// Previous hour file exists, load its final resource states
			s.logger.Debug("Previous hour file exists, loading final resource states...")
			reader, err := NewBlockReader(prevFilePath)
			if err != nil {
				s.logger.Warn("Failed to open previous hour file for state carryover: %v", err)
			} else {
				fileData, err := reader.ReadFile()
				_ = reader.Close()
				if err != nil {
					s.logger.Warn("Failed to read previous hour file for state carryover: %v", err)
				} else if len(fileData.IndexSection.FinalResourceStates) > 0 {
					carryoverStates = fileData.IndexSection.FinalResourceStates
					s.logger.Info("Loaded %d resource states from previous hour file on disk: %s",
						len(carryoverStates), prevFilePath)
				} else {
					s.logger.Debug("Previous hour file has no final resource states")
				}
			}
		} else {
			s.logger.Debug("Previous hour file does not exist: %s", prevFilePath)
		}
	}

	newFile, err := NewBlockStorageFile(filePath, hourTimestamp, s.blockSize)
	if err != nil {
		return nil, err
	}

	// Carry over state snapshots from previous hour
	// Filter out deleted resources - they should not be carried forward
	if len(carryoverStates) > 0 {
		filteredStates := make(map[string]*ResourceLastState)
		deletedCount := 0
		for key, state := range carryoverStates {
			if state.EventType != string(models.EventTypeDelete) {
				filteredStates[key] = state
			} else {
				deletedCount++
			}
		}
		newFile.finalResourceStates = filteredStates
		s.logger.Info("Carried over %d resource states to new file (%d deleted resources excluded)",
			len(filteredStates), deletedCount)
	}

	s.hourFiles[hourTimestamp] = newFile

	// Update file index
	hourEnd := hourTimestamp + 3600
	meta := &FileMetadata{
		FilePath:     filePath,
		HourStart:    hourTimestamp,
		HourEnd:      hourEnd,
		TimestampMin: hourTimestamp * 1e9,
		TimestampMax: hourEnd * 1e9,
		EventCount:   0,
		FileSize:     0,
	}
	if err := s.fileIndex.AddOrUpdate(meta); err != nil {
		s.logger.Warn("Failed to update file index: %v", err)
	}

	return newFile, nil
}

// Close closes the storage and all open hour files
func (s *Storage) Close() error {
	s.fileMutex.Lock()
	defer s.fileMutex.Unlock()

	// Check if already closed to make this method idempotent
	if s.closed {
		return nil
	}
	s.closed = true

	// Close all open hour files
	for hourTimestamp, file := range s.hourFiles {
		s.logger.Debug("Closing hour file: %d", hourTimestamp)
		if err := file.Close(); err != nil {
			s.logger.Error("Failed to close hour file %d: %v", hourTimestamp, err)
		}

		// Update file index with final stats
		if file.path != "" {
			meta := &FileMetadata{
				FilePath:     file.path,
				HourStart:    hourTimestamp,
				HourEnd:      hourTimestamp + 3600,
				TimestampMin: hourTimestamp * 1e9,
				TimestampMax: (hourTimestamp + 3600) * 1e9,
				EventCount:   file.totalEvents,
			}
			if info, err := os.Stat(file.path); err == nil {
				meta.FileSize = info.Size()
			}
			if err := s.fileIndex.AddOrUpdate(meta); err != nil {
				s.logger.Warn("Failed to update file index on close: %v", err)
			}
		}
	}

	s.hourFiles = make(map[int64]*BlockStorageFile)
	s.currentFile = nil

	// Save file index
	if err := s.fileIndex.Save(); err != nil {
		s.logger.Warn("Failed to save file index: %v", err)
	}

	s.logger.Info("Storage closed")
	return nil
}

// CloseOldHourFiles closes hour files that are older than the specified duration
// This prevents keeping too many files open
func (s *Storage) CloseOldHourFiles(olderThan time.Duration) error {
	s.fileMutex.Lock()
	defer s.fileMutex.Unlock()

	now := time.Now()
	cutoff := now.Add(-olderThan)

	for hourTimestamp, file := range s.hourFiles {
		hourTime := time.Unix(hourTimestamp, 0)
		if hourTime.Before(cutoff) {
			s.logger.Debug("Closing old hour file: %s", hourTime.Format("2006-01-02 15:04"))
			if err := file.Close(); err != nil {
				s.logger.Error("Failed to close old hour file: %v", err)
			}

			// Update file index with final stats
			if file.path != "" {
				meta := &FileMetadata{
					FilePath:     file.path,
					HourStart:    hourTimestamp,
					HourEnd:      hourTimestamp + 3600,
					TimestampMin: hourTimestamp * 1e9,
					TimestampMax: (hourTimestamp + 3600) * 1e9,
					EventCount:   file.totalEvents,
				}
				if info, err := os.Stat(file.path); err == nil {
					meta.FileSize = info.Size()
				}
				if err := s.fileIndex.AddOrUpdate(meta); err != nil {
					s.logger.Warn("Failed to update file index: %v", err)
				}
			}

			delete(s.hourFiles, hourTimestamp)
		}
	}

	return nil
}

// runFileCloser runs in the background and periodically closes old hour files
func (s *Storage) runFileCloser() {
	defer s.wg.Done()

	// Close files older than 2 hours every 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	s.logger.Info("Started background file closer (interval: 5m, close files older than: 2h)")

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Background file closer stopped")
			return
		case <-ticker.C:
			if err := s.CloseOldHourFiles(2 * time.Hour); err != nil {
				s.logger.Error("Failed to close old hour files: %v", err)
			}
		}
	}
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
	stats["indexedFiles"] = s.fileIndex.Count()
	stats["openHourFiles"] = len(s.hourFiles)

	return stats, nil
}

// GetFileIndex returns the file index for query optimization
func (s *Storage) GetFileIndex() *FileIndex {
	return s.fileIndex
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
		_ = reader.Close()

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
			} else if state.EventType != string(models.EventTypeDelete) {
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
	file, err := os.OpenFile(filePath, os.O_RDWR, 0644) //nolint:gosec // filePath is validated before use
	if err != nil {
		return fmt.Errorf("failed to open file for rewriting: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			// Log error but don't fail the operation
		}
	}()

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
		IndexSectionLength: int32(bytesWritten), //nolint:gosec // safe conversion: bytes written is reasonable
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

	// Create a context for background tasks
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// Start background goroutine to periodically close old hour files
	s.wg.Add(1)
	go s.runFileCloser()

	// Storage is already initialized in New(), so just verify it's ready
	s.logger.Info("Storage component ready")
	return nil
}

// Stop implements the lifecycle.Component interface
// Gracefully shuts down the storage component
func (s *Storage) Stop(ctx context.Context) error {
	s.logger.Info("Stopping storage component")

	// Cancel background tasks
	if s.cancel != nil {
		s.cancel()
	}

	done := make(chan error, 1)

	go func() {
		// Wait for background goroutines to finish
		s.wg.Wait()
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

// getOpenFileByPath returns the open BlockStorageFile for a given path if it exists.
func (s *Storage) getOpenFileByPath(path string) *BlockStorageFile {
	for _, file := range s.hourFiles {
		if file != nil && file.path == path {
			return file
		}
	}
	return nil
}

// collectEventsFromOpenFile gathers events from an open block storage file.
// It includes:
// - Restored blocks (on disk) when the file was reopened for appending
// - Finalized blocks kept in memory (file still open, no footer yet)
// - Current buffer events (not yet finalized)
func (s *Storage) collectEventsFromOpenFile(bsf *BlockStorageFile, query *models.QueryRequest) ([]*models.Event, error) {
	var allEvents []*models.Event

	startTimeNs := query.StartTimestamp * 1e9
	endTimeNs := query.EndTimestamp * 1e9

	bsf.mutex.Lock()
	defer bsf.mutex.Unlock()

	// Restored blocks: blockMetadataList populated, blocks slice empty
	hasRestoredBlocks := len(bsf.blockMetadataList) > 0 && len(bsf.blocks) == 0
	if hasRestoredBlocks {
		restoredEvents, err := s.readEventsFromRestoredBlocks(bsf, query)
		if err != nil {
			s.logger.Warn("Failed to read events from restored blocks: %v", err)
		} else {
			allEvents = append(allEvents, restoredEvents...)
		}
	}

	// Finalized blocks that are still in memory (file open, no footer yet)
	if len(bsf.blocks) > 0 {
		for _, block := range bsf.blocks {
			if block.Metadata == nil {
				continue
			}

			// Check time range overlap
			if block.Metadata.TimestampMax < startTimeNs || block.Metadata.TimestampMin > endTimeNs {
				continue
			}

			decompressedData, err := DecompressBlock(block)
			if err != nil {
				s.logger.Warn("Failed to decompress finalized block: %v", err)
				continue
			}

			offset := 0
			for offset < len(decompressedData) {
				length, n := binary.Uvarint(decompressedData[offset:])
				if n <= 0 {
					break
				}
				offset += n

				if offset+int(length) > len(decompressedData) { //nolint:gosec // length validated below
					s.logger.Warn("Invalid message length in finalized block: %d at offset %d", length, offset)
					break
				}

				messageData := decompressedData[offset : offset+int(length)] //nolint:gosec // length validated
				offset += int(length)                                        //nolint:gosec // length validated

				event := &models.Event{}
				if err := event.UnmarshalProtobuf(messageData); err != nil {
					s.logger.Warn("Failed to unmarshal event from finalized block: %v", err)
					continue
				}

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

	// Current buffer events
	currentBuffer := bsf.currentBuffer
	if currentBuffer != nil && currentBuffer.GetEventCount() > 0 {
		bufferEvents, err := currentBuffer.GetEvents()
		if err != nil {
			s.logger.Error("Failed to get events from buffer: %v", err)
			return nil, err
		}
		allEvents = append(allEvents, bufferEvents...)
	}

	return allEvents, nil
}

// GetEventsFromOpenFile returns events from an open file (identified by path).
// It returns found=false when the file is not currently open.
func (s *Storage) GetEventsFromOpenFile(path string, query *models.QueryRequest) ([]models.Event, bool, error) {
	s.fileMutex.RLock()
	defer s.fileMutex.RUnlock()

	openFile := s.getOpenFileByPath(path)
	if openFile == nil {
		return nil, false, nil
	}

	events, err := s.collectEventsFromOpenFile(openFile, query)
	if err != nil {
		return nil, true, err
	}

	if len(events) == 0 {
		return []models.Event{}, true, nil
	}

	return s.filterEvents(events, query), true, nil
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

	allEvents, err := s.collectEventsFromOpenFile(s.currentFile, query)
	if err != nil {
		return nil, err
	}

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

	matchingEvents := make([]models.Event, 0, len(events))
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
	defer func() {
		if err := reader.Close(); err != nil {
			// Log error but don't fail the operation
		}
	}()

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
