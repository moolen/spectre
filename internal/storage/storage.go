package storage

import (
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
	compressor  *Compressor
	currentFile *StorageFile
	fileMutex   sync.RWMutex
	segmentSize int64
}

// New creates a new Storage instance
func New(dataDir string, segmentSize int64) (*Storage, error) {
	logger := logging.GetLogger("storage")

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		logger.Error("Failed to create data directory: %v", err)
		return nil, err
	}

	s := &Storage{
		dataDir:     dataDir,
		logger:      logger,
		compressor:  NewCompressor(),
		segmentSize: segmentSize,
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
func (s *Storage) getOrCreateCurrentFile() (*StorageFile, error) {
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

		newFile, err := NewStorageFile(filePath, currentHour.Unix(), s.compressor, s.segmentSize)
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
		if err := s.currentFile.Close(); err != nil {
			s.logger.Error("Failed to close storage file: %v", err)
			return err
		}
	}

	s.logger.Info("Storage closed")
	return nil
}

// GetEventsByTimeRange retrieves events within a time range
func (s *Storage) GetEventsByTimeRange(startTime, endTime int64, filters models.QueryFilters) ([]models.Event, error) {
	s.fileMutex.RLock()
	defer s.fileMutex.RUnlock()

	var results []models.Event

	// Find all storage files that overlap with the time range
	files, err := s.getStorageFiles()
	if err != nil {
		return nil, err
	}

	// Filter events from each file
	for _, filePath := range files {
		fileEvents, err := s.readEventsFromFile(filePath, startTime, endTime, filters)
		if err != nil {
			s.logger.Error("Failed to read events from file %s: %v", filePath, err)
			continue
		}

		results = append(results, fileEvents...)
	}

	return results, nil
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

// readEventsFromFile reads and filters events from a specific file
func (s *Storage) readEventsFromFile(filePath string, startTime, endTime int64, filters models.QueryFilters) ([]models.Event, error) {
	// TODO: Implement file reading and filtering logic
	// For now, return empty results
	return []models.Event{}, nil
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
