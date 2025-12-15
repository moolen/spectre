package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// FileMetadata contains cached metadata about a storage file
type FileMetadata struct {
	FilePath     string `json:"file_path"`
	HourStart    int64  `json:"hour_start"`    // Unix timestamp of hour start (inclusive)
	HourEnd      int64  `json:"hour_end"`      // Unix timestamp of hour end (exclusive)
	TimestampMin int64  `json:"timestamp_min"` // Minimum event timestamp (ns)
	TimestampMax int64  `json:"timestamp_max"` // Maximum event timestamp (ns)
	EventCount   int64  `json:"event_count"`
	FileSize     int64  `json:"file_size"`
	LastUpdated  int64  `json:"last_updated"` // When metadata was last updated
}

// FileIndex maintains an in-memory index of storage files with their time boundaries
// This allows fast file selection for queries without reading file metadata
type FileIndex struct {
	mu        sync.RWMutex
	files     map[string]*FileMetadata // key is file path
	indexPath string                   // path to persisted index
	logger    interface {
		Info(string, ...interface{})
		Debug(string, ...interface{})
		Warn(string, ...interface{})
	}
	autoSave    bool
	strictHours bool // if true, enforce strict hour boundaries
}

// NewFileIndex creates a new file index
func NewFileIndex(dataDir string, logger interface {
	Info(string, ...interface{})
	Debug(string, ...interface{})
	Warn(string, ...interface{})
}) *FileIndex {
	indexPath := filepath.Join(dataDir, ".file_index.json")
	fi := &FileIndex{
		files:       make(map[string]*FileMetadata),
		indexPath:   indexPath,
		logger:      logger,
		autoSave:    true,
		strictHours: true, // Enable strict hour boundaries by default
	}

	// Try to load existing index
	if err := fi.Load(); err != nil {
		logger.Debug("Could not load file index (will create new): %v", err)
	}

	return fi
}

// GetFilesByTimeRange returns files that overlap with the given time range (in nanoseconds)
// Returns files sorted by hour start time
func (fi *FileIndex) GetFilesByTimeRange(startNs, endNs int64) []string {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	var matching []string
	for filePath, meta := range fi.files {
		hourStartNs := meta.HourStart * 1e9
		hourEndNs := meta.HourEnd * 1e9

		// If strict hours are enabled, we can rely solely on hour boundaries
		if fi.strictHours {
			// File hour overlaps with query range
			if hourEndNs > startNs && hourStartNs < endNs {
				matching = append(matching, filePath)
			}
		} else {
			// Need to check actual event timestamps (fallback for legacy files)
			if meta.TimestampMax >= startNs && meta.TimestampMin <= endNs {
				matching = append(matching, filePath)
			}
		}
	}

	// Sort by hour start
	sort.Slice(matching, func(i, j int) bool {
		return fi.files[matching[i]].HourStart < fi.files[matching[j]].HourStart
	})

	return matching
}

// GetFileBeforeTime returns the most recent file before the given time (for state snapshots)
func (fi *FileIndex) GetFileBeforeTime(timeNs int64) string {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	var mostRecent string
	var mostRecentHour int64

	for filePath, meta := range fi.files {
		hourEndNs := meta.HourEnd * 1e9
		if hourEndNs <= timeNs && meta.HourStart > mostRecentHour {
			mostRecent = filePath
			mostRecentHour = meta.HourStart
		}
	}

	return mostRecent
}

// AddOrUpdate adds or updates file metadata in the index
func (fi *FileIndex) AddOrUpdate(meta *FileMetadata) error {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	meta.LastUpdated = time.Now().Unix()
	fi.files[meta.FilePath] = meta

	if fi.autoSave {
		return fi.saveUnlocked()
	}
	return nil
}

// Remove removes a file from the index
func (fi *FileIndex) Remove(filePath string) error {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	delete(fi.files, filePath)

	if fi.autoSave {
		return fi.saveUnlocked()
	}
	return nil
}

// Get retrieves metadata for a specific file
func (fi *FileIndex) Get(filePath string) (*FileMetadata, bool) {
	fi.mu.RLock()
	defer fi.mu.RUnlock()

	meta, ok := fi.files[filePath]
	return meta, ok
}

// Count returns the number of files in the index
func (fi *FileIndex) Count() int {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return len(fi.files)
}

// Load loads the index from disk
func (fi *FileIndex) Load() error {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	data, err := os.ReadFile(fi.indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Not an error, just no index yet
		}
		return fmt.Errorf("failed to read index: %w", err)
	}

	var files []*FileMetadata
	if err := json.Unmarshal(data, &files); err != nil {
		return fmt.Errorf("failed to unmarshal index: %w", err)
	}

	// Rebuild map
	fi.files = make(map[string]*FileMetadata)
	for _, meta := range files {
		fi.files[meta.FilePath] = meta
	}

	fi.logger.Info("Loaded file index with %d files", len(fi.files))
	return nil
}

// Save saves the index to disk
func (fi *FileIndex) Save() error {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	return fi.saveUnlocked()
}

// saveUnlocked saves without locking (caller must hold lock)
func (fi *FileIndex) saveUnlocked() error {
	// Convert map to slice for JSON
	files := make([]*FileMetadata, 0, len(fi.files))
	for _, meta := range fi.files {
		files = append(files, meta)
	}

	data, err := json.MarshalIndent(files, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	// Write atomically using temp file + rename
	tempPath := fi.indexPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil { //nolint:gosec // 0644 is standard
		return fmt.Errorf("failed to write index: %w", err)
	}

	if err := os.Rename(tempPath, fi.indexPath); err != nil {
		return fmt.Errorf("failed to rename index: %w", err)
	}

	return nil
}

// Rebuild rebuilds the index by scanning all storage files
// This is useful after restarts or if the index becomes corrupted
func (fi *FileIndex) Rebuild(dataDir string, extractHourFunc func(string) (int64, error)) error {
	fi.mu.Lock()
	defer fi.mu.Unlock()

	fi.logger.Info("Rebuilding file index from disk...")

	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return fmt.Errorf("failed to read data directory: %w", err)
	}

	newFiles := make(map[string]*FileMetadata)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".bin" {
			continue
		}

		filePath := filepath.Join(dataDir, entry.Name())

		// Extract hour from filename
		hourStart, err := extractHourFunc(filePath)
		if err != nil {
			fi.logger.Warn("Could not extract hour from %s: %v", filePath, err)
			continue
		}

		info, err := entry.Info()
		if err != nil {
			fi.logger.Warn("Could not stat %s: %v", filePath, err)
			continue
		}

		// Create metadata with hour boundaries
		hourEnd := hourStart + 3600 // One hour later
		meta := &FileMetadata{
			FilePath:    filePath,
			HourStart:   hourStart,
			HourEnd:     hourEnd,
			FileSize:    info.Size(),
			LastUpdated: time.Now().Unix(),
		}

		// Try to read actual min/max timestamps from file
		// This is optional - if it fails, we use hour boundaries
		if timestampMin, timestampMax, err := fi.readFileTimestamps(filePath); err == nil {
			meta.TimestampMin = timestampMin
			meta.TimestampMax = timestampMax
		} else {
			// Fallback to hour boundaries
			meta.TimestampMin = hourStart * 1e9
			meta.TimestampMax = hourEnd * 1e9
		}

		newFiles[filePath] = meta
	}

	fi.files = newFiles
	fi.logger.Info("Rebuilt file index with %d files", len(fi.files))

	if fi.autoSave {
		return fi.saveUnlocked()
	}
	return nil
}

// readFileTimestamps reads min/max timestamps from a file's index section
func (fi *FileIndex) readFileTimestamps(filePath string) (int64, int64, error) {
	reader, err := NewBlockReader(filePath)
	if err != nil {
		return 0, 0, err
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			// Log error but don't fail the operation
			fi.logger.Warn("Failed to close reader: %v", closeErr)
		}
	}()

	footer, err := reader.ReadFileFooter()
	if err != nil {
		return 0, 0, err
	}

	indexSection, err := reader.ReadIndexSection(footer.IndexSectionOffset, footer.IndexSectionLength)
	if err != nil {
		return 0, 0, err
	}

	if indexSection.Statistics != nil {
		return indexSection.Statistics.TimestampMin, indexSection.Statistics.TimestampMax, nil
	}

	return 0, 0, fmt.Errorf("no statistics in index section")
}

// SetStrictHours enables or disables strict hour boundary enforcement
func (fi *FileIndex) SetStrictHours(strict bool) {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	fi.strictHours = strict
}
