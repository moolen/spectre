package storage

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// ExportManifest describes the contents of an exported archive
type ExportManifest struct {
	// Version of the export format
	Version string `json:"version"`

	// ExportedAt is the Unix timestamp when the export was created
	ExportedAt int64 `json:"exported_at"`

	// SourceClusterID is an optional identifier for the source cluster
	SourceClusterID string `json:"source_cluster_id,omitempty"`

	// SourceInstanceID is an optional identifier for the source instance
	SourceInstanceID string `json:"source_instance_id,omitempty"`

	// Files contains metadata about each exported storage file
	Files []ExportedFileEntry `json:"files"`
}

// ExportedFileEntry describes a single storage file in the export
type ExportedFileEntry struct {
	// OriginalPath is the original file path on the source system
	OriginalPath string `json:"original_path"`

	// ArchivePath is the path within the archive (e.g., "2024-12-02-14.bin")
	ArchivePath string `json:"archive_path"`

	// HourTimestamp is the Unix timestamp of the hour this file represents
	HourTimestamp int64 `json:"hour_timestamp"`

	// SizeBytes is the file size in bytes
	SizeBytes int64 `json:"size_bytes"`

	// EventCount is the number of events in the file (if available)
	EventCount int64 `json:"event_count,omitempty"`

	// TimestampMin is the minimum event timestamp in the file (nanoseconds)
	TimestampMin int64 `json:"timestamp_min,omitempty"`

	// TimestampMax is the maximum event timestamp in the file (nanoseconds)
	TimestampMax int64 `json:"timestamp_max,omitempty"`
}

// ExportOptions configures the export operation
type ExportOptions struct {
	// StartTime is the start of the time range to export (Unix seconds)
	// If 0, exports from the beginning
	StartTime int64

	// EndTime is the end of the time range to export (Unix seconds)
	// If 0, exports to the end
	EndTime int64

	// IncludeOpenHour determines whether to include the currently open hour file
	// If true, the current file will be closed before export
	IncludeOpenHour bool

	// ClusterID is an optional identifier for the source cluster
	ClusterID string

	// InstanceID is an optional identifier for the source instance
	InstanceID string

	// Compression enables gzip compression of the tar archive
	Compression bool
}

// ImportOptions configures the import operation
type ImportOptions struct {
	// ValidateFiles enables validation of imported files before merging
	ValidateFiles bool

	// OverwriteExisting determines behavior when target files exist
	// If true, performs merge; if false, skips existing hours
	OverwriteExisting bool
}

// ImportReport contains the results of an import operation
type ImportReport struct {
	// TotalFiles is the number of files in the archive
	TotalFiles int

	// ImportedFiles is the number of files successfully imported
	ImportedFiles int

	// MergedHours is the number of hourly files that were merged
	MergedHours int

	// SkippedFiles is the number of files skipped
	SkippedFiles int

	// FailedFiles is the number of files that failed to import
	FailedFiles int

	// Errors contains any errors encountered during import
	Errors []string

	// TotalEvents is the total number of events imported
	TotalEvents int64

	// Duration is how long the import took
	Duration time.Duration
}

const (
	// ExportFormatVersion is the current version of the export format
	ExportFormatVersion = "1.0"

	// ManifestFileName is the name of the manifest file in the archive
	ManifestFileName = "manifest.json"
)

// Export exports storage files to a tar archive
func (s *Storage) Export(w io.Writer, opts ExportOptions) error {
	startTime := time.Now()
	s.logger.Info("Starting storage export")

	// Acquire write lock if we need to close the current file
	// This prevents concurrent writes from reopening files during export
	var closedCurrentFile bool
	if opts.IncludeOpenHour {
		s.fileMutex.Lock()
		if s.currentFile != nil {
			if err := s.currentFile.Close(); err != nil {
				s.logger.Warn("Failed to close current file for export: %v", err)
			}
			s.currentFile = nil
			closedCurrentFile = true

			// CRITICAL FIX: Extend endTime to include the hour we just closed
			// The current file represents the current hour, so we need to extend
			// the endTime to encompass the entire hour that was just closed
			now := time.Now()
			currentHourStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location()).Unix()
			currentHourEnd := currentHourStart + 3600
			if opts.EndTime < currentHourEnd {
				opts.EndTime = currentHourEnd
				s.logger.DebugWithFields("Extended export time range to include open hour",
					logging.Field("new_end_time", opts.EndTime),
					logging.Field("current_hour_end", currentHourEnd))
			}
		}
		// Downgrade to read lock for file enumeration
		// Keep read lock held to prevent file reopening during export
		s.fileMutex.Unlock()
		s.fileMutex.RLock()
		defer s.fileMutex.RUnlock()
	} else {
		// Just acquire read lock for file enumeration
		s.fileMutex.RLock()
		defer s.fileMutex.RUnlock()
	}

	// Get all storage files
	allFiles, err := s.getStorageFiles()
	if err != nil {
		return fmt.Errorf("failed to list storage files: %w", err)
	}

	// Filter files by time range
	selectedFiles := s.filterFilesByTimeRange(allFiles, opts.StartTime, opts.EndTime)

	if len(selectedFiles) == 0 {
		s.logger.Warn("No files match the export criteria")
		return fmt.Errorf("no files to export")
	}

	s.logger.DebugWithFields("Exporting files",
		logging.Field("count", len(selectedFiles)),
		logging.Field("closed_current", closedCurrentFile))

	// Create the archive writer
	var archiveWriter io.Writer = w
	var gzipWriter *gzip.Writer

	if opts.Compression {
		gzipWriter = gzip.NewWriter(w)
		archiveWriter = gzipWriter
	}

	tarWriter := tar.NewWriter(archiveWriter)

	// Build manifest
	manifest := ExportManifest{
		Version:          ExportFormatVersion,
		ExportedAt:       time.Now().Unix(),
		SourceClusterID:  opts.ClusterID,
		SourceInstanceID: opts.InstanceID,
		Files:            make([]ExportedFileEntry, 0, len(selectedFiles)),
	}

	// Export each file
	for _, filePath := range selectedFiles {
		entry, err := s.exportFile(tarWriter, filePath)
		if err != nil {
			s.logger.Error("Failed to export file %s: %v", filePath, err)
			return fmt.Errorf("failed to export file %s: %w", filePath, err)
		}
		manifest.Files = append(manifest.Files, entry)
	}

	// Write manifest as the last entry
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	manifestHeader := &tar.Header{
		Name:    ManifestFileName,
		Mode:    0644,
		Size:    int64(len(manifestJSON)),
		ModTime: time.Now(),
	}

	if err := tarWriter.WriteHeader(manifestHeader); err != nil {
		return fmt.Errorf("failed to write manifest header: %w", err)
	}

	if _, err := tarWriter.Write(manifestJSON); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}

	if gzipWriter != nil {
		if err := gzipWriter.Close(); err != nil {
			return fmt.Errorf("failed to close gzip writer: %w", err)
		}
	}

	s.logger.InfoWithFields("Storage export completed",
		logging.Field("files", len(manifest.Files)),
		logging.Field("duration", time.Since(startTime)))

	return nil
}

// exportFile exports a single storage file to the tar archive
func (s *Storage) exportFile(tw *tar.Writer, filePath string) (ExportedFileEntry, error) {
	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return ExportedFileEntry{}, fmt.Errorf("failed to stat file: %w", err)
	}

	// Extract hour timestamp from filename
	hourTimestamp, err := s.extractHourFromFilename(filePath)
	if err != nil {
		s.logger.Warn("Failed to extract hour from filename %s: %v", filePath, err)
		hourTimestamp = 0
	}

	// Try to read file metadata for additional info
	var eventCount int64
	var timestampMin, timestampMax int64

	reader, err := NewBlockReader(filePath)
	if err == nil {
		fileData, err := reader.ReadFile()
		_ = reader.Close()
		if err == nil && fileData.IndexSection != nil && fileData.IndexSection.Statistics != nil {
			stats := fileData.IndexSection.Statistics
			eventCount = stats.TotalEvents
			timestampMin = stats.TimestampMin
			timestampMax = stats.TimestampMax
		}
	}

	// Create archive path (just the filename)
	archivePath := filepath.Base(filePath)

	// Create tar header
	header := &tar.Header{
		Name:    archivePath,
		Mode:    0644,
		Size:    fileInfo.Size(),
		ModTime: fileInfo.ModTime(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return ExportedFileEntry{}, fmt.Errorf("failed to write tar header: %w", err)
	}

	// Copy file contents
	file, err := os.Open(filePath) //nolint:gosec // filePath is validated before use
	if err != nil {
		return ExportedFileEntry{}, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			// Log error but don't fail the operation
		}
	}()

	written, err := io.Copy(tw, file)
	if err != nil {
		return ExportedFileEntry{}, fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Verify we wrote the expected number of bytes
	if written != fileInfo.Size() {
		return ExportedFileEntry{}, fmt.Errorf("size mismatch: wrote %d bytes but file size is %d", written, fileInfo.Size())
	}

	entry := ExportedFileEntry{
		OriginalPath:  filePath,
		ArchivePath:   archivePath,
		HourTimestamp: hourTimestamp,
		SizeBytes:     fileInfo.Size(),
		EventCount:    eventCount,
		TimestampMin:  timestampMin,
		TimestampMax:  timestampMax,
	}

	s.logger.DebugWithFields("Exported file",
		logging.Field("path", filePath),
		logging.Field("size", fileInfo.Size()),
		logging.Field("events", eventCount))

	return entry, nil
}

// filterFilesByTimeRange filters files based on the time range
func (s *Storage) filterFilesByTimeRange(files []string, startTime, endTime int64) []string {
	if startTime == 0 && endTime == 0 {
		return files
	}

	filtered := make([]string, 0, len(files))
	for _, filePath := range files {
		hourTimestamp, err := s.extractHourFromFilename(filePath)
		if err != nil {
			s.logger.Warn("Failed to extract hour from filename %s: %v", filePath, err)
			continue
		}

		// Check if this hour falls within the range
		// Each file represents one hour, so check if the hour overlaps with [startTime, endTime]
		hourEnd := hourTimestamp + 3600 // One hour later

		if startTime > 0 && hourEnd < startTime {
			continue
		}
		if endTime > 0 && hourTimestamp >= endTime {
			continue
		}

		filtered = append(filtered, filePath)
	}

	return filtered
}

// Import imports storage files from a tar archive
func (s *Storage) Import(r io.Reader, opts ImportOptions) (*ImportReport, error) {
	startTime := time.Now()
	s.logger.Info("Starting storage import")

	report := &ImportReport{
		Errors: make([]string, 0),
	}

	// Try to detect if the stream is gzip-compressed
	gzipReader, err := gzip.NewReader(r)
	var tarReader *tar.Reader
	if err == nil {
		// Successfully opened as gzip
		defer func() {
			if err := gzipReader.Close(); err != nil {
				// Log error but don't fail the operation
			}
		}()
		tarReader = tar.NewReader(gzipReader)
	} else {
		// Not gzip, treat as plain tar
		tarReader = tar.NewReader(r)
	}

	// Extract archive to temporary directory
	tempDir, err := os.MkdirTemp("", "spectre-import-*")
	if err != nil {
		return report, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			// Log error but don't fail the operation
		}
	}()

	var manifest *ExportManifest
	extractedFiles := make(map[string]string) // archive path -> temp file path

	// Extract all files from archive
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return report, fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Handle manifest separately
		if header.Name == ManifestFileName {
			manifestData, err := io.ReadAll(tarReader)
			if err != nil {
				return report, fmt.Errorf("failed to read manifest: %w", err)
			}

			manifest = &ExportManifest{}
			if err := json.Unmarshal(manifestData, manifest); err != nil {
				return report, fmt.Errorf("failed to parse manifest: %w", err)
			}
			continue
		}

		// Extract storage file to temp directory
		tempFilePath := filepath.Join(tempDir, filepath.Base(header.Name))
		tempFile, err := os.Create(tempFilePath) //nolint:gosec // tempFilePath is validated before use
		if err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("failed to create temp file for %s: %v", header.Name, err))
			continue
		}

		if _, err := io.Copy(tempFile, tarReader); err != nil { //nolint:gosec // decompression size is limited by block size
			_ = tempFile.Close()
			report.Errors = append(report.Errors, fmt.Sprintf("failed to extract %s: %v", header.Name, err))
			continue
		}
		_ = tempFile.Close()

		extractedFiles[header.Name] = tempFilePath
		report.TotalFiles++
	}

	if manifest == nil {
		return report, fmt.Errorf("manifest not found in archive")
	}

	// Validate manifest version
	if manifest.Version != ExportFormatVersion {
		s.logger.Warn("Import manifest version %s differs from current version %s", manifest.Version, ExportFormatVersion)
	}

	// Group files by hour timestamp
	hourGroups := make(map[int64][]string) // hour timestamp -> list of temp file paths
	for _, entry := range manifest.Files {
		tempPath, ok := extractedFiles[entry.ArchivePath]
		if !ok {
			report.Errors = append(report.Errors, fmt.Sprintf("file %s in manifest but not extracted", entry.ArchivePath))
			report.FailedFiles++
			continue
		}

		// Validate file if requested
		if opts.ValidateFiles {
			if err := s.validateImportedFile(tempPath); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("validation failed for %s: %v", entry.ArchivePath, err))
				report.FailedFiles++
				continue
			}
		}

		hourGroups[entry.HourTimestamp] = append(hourGroups[entry.HourTimestamp], tempPath)
	}

	// Process each hour group
	for hourTimestamp, filePaths := range hourGroups {
		if err := s.importHourGroup(hourTimestamp, filePaths, opts, report); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("failed to import hour %d: %v", hourTimestamp, err))
			report.FailedFiles += len(filePaths)
		} else {
			report.ImportedFiles += len(filePaths)
			report.MergedHours++
		}
	}

	report.Duration = time.Since(startTime)

	// Rebuild file index to make new files discoverable by queries
	if err := s.fileIndex.Rebuild(s.dataDir, s.extractHourFromFilename); err != nil {
		s.logger.Warn("Failed to rebuild file index after import: %v", err)
		// Don't fail the import if index rebuild fails - the data is still there
	} else {
		s.logger.Info("File index rebuilt after import")
	}

	s.logger.InfoWithFields("Storage import completed",
		logging.Field("total_files", report.TotalFiles),
		logging.Field("imported", report.ImportedFiles),
		logging.Field("merged_hours", report.MergedHours),
		logging.Field("failed", report.FailedFiles),
		logging.Field("events", report.TotalEvents),
		logging.Field("duration", report.Duration))

	return report, nil
}

// validateImportedFile validates a storage file before import
func (s *Storage) validateImportedFile(filePath string) error {
	reader, err := NewBlockReader(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			// Log error but don't fail the operation
		}
	}()

	// Try to read the complete file structure
	_, err = reader.ReadFile()
	if err != nil {
		return fmt.Errorf("failed to read file structure: %w", err)
	}

	return nil
}

// importHourGroup merges all files for a given hour into a single canonical file
func (s *Storage) importHourGroup(hourTimestamp int64, importedFilePaths []string, _ ImportOptions, report *ImportReport) error {
	s.fileMutex.Lock()
	defer s.fileMutex.Unlock()

	// Determine the canonical filename for this hour
	// Use Local timezone to match extractHourFromFilename and getOrCreateCurrentFile
	hourTime := time.Unix(hourTimestamp, 0).Local()
	canonicalFilename := fmt.Sprintf("%04d-%02d-%02d-%02d.bin",
		hourTime.Year(), hourTime.Month(), hourTime.Day(), hourTime.Hour())
	canonicalPath := filepath.Join(s.dataDir, canonicalFilename)

	// Check if this is the currently open hour
	if s.currentFile != nil && s.currentFile.hourTimestamp == hourTimestamp {
		// Close the current file so we can merge into it
		if err := s.currentFile.Close(); err != nil {
			s.logger.Warn("Failed to close current file for import merge: %v", err)
		}
		s.currentFile = nil
	}

	// Collect all source files (existing + imported)
	var sourceFiles []string

	// Add existing file if it exists
	if _, err := os.Stat(canonicalPath); err == nil {
		sourceFiles = append(sourceFiles, canonicalPath)
	}

	// Add all imported files
	sourceFiles = append(sourceFiles, importedFilePaths...)

	// Create temporary file for the merged result
	tempMergedPath := filepath.Join(s.dataDir, fmt.Sprintf("%s.importing-%d", canonicalFilename, time.Now().Unix()))

	// Perform the merge
	eventCount, err := s.mergeFilesIntoNew(sourceFiles, tempMergedPath, hourTimestamp)
	if err != nil {
		// Clean up temp file on error
		_ = os.Remove(tempMergedPath)
		return fmt.Errorf("failed to merge files: %w", err)
	}

	report.TotalEvents += eventCount

	// Atomically replace the canonical file with the merged one
	if err := os.Rename(tempMergedPath, canonicalPath); err != nil {
		_ = os.Remove(tempMergedPath)
		return fmt.Errorf("failed to replace canonical file: %w", err)
	}

	s.logger.InfoWithFields("Merged hour group",
		logging.Field("hour", hourTime.Format("2006-01-02-15")),
		logging.Field("source_files", len(sourceFiles)),
		logging.Field("events", eventCount))

	return nil
}

// mergeFilesIntoNew reads events from multiple source files and writes them to a new file
func (s *Storage) mergeFilesIntoNew(sourceFiles []string, targetPath string, hourTimestamp int64) (int64, error) {
	// Collect all events from source files
	var allEvents []*models.Event

	for _, sourcePath := range sourceFiles {
		events, err := s.readAllEventsFromFile(sourcePath)
		if err != nil {
			s.logger.Warn("Failed to read events from %s: %v", sourcePath, err)
			continue
		}
		allEvents = append(allEvents, events...)
	}

	if len(allEvents) == 0 {
		return 0, fmt.Errorf("no events to merge")
	}

	// Sort events by timestamp for better compression and query performance
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Timestamp < allEvents[j].Timestamp
	})

	// Create new storage file
	newFile, err := NewBlockStorageFile(targetPath, hourTimestamp, s.blockSize)
	if err != nil {
		return 0, fmt.Errorf("failed to create new storage file: %w", err)
	}

	// Write all events
	for _, event := range allEvents {
		if err := newFile.WriteEvent(event); err != nil {
			_ = newFile.Close()
			return 0, fmt.Errorf("failed to write event: %w", err)
		}
	}

	// Close to finalize (writes index and footer)
	if err := newFile.Close(); err != nil {
		return 0, fmt.Errorf("failed to close new file: %w", err)
	}

	return int64(len(allEvents)), nil
}

// readAllEventsFromFile reads all events from a storage file
func (s *Storage) readAllEventsFromFile(filePath string) ([]*models.Event, error) {
	reader, err := NewBlockReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if err := reader.Close(); err != nil {
			// Log error but don't fail the operation
		}
	}()

	fileData, err := reader.ReadFile()
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var allEvents []*models.Event

	// Read events from each block
	for _, blockMeta := range fileData.IndexSection.BlockMetadata {
		events, err := reader.ReadBlockEvents(blockMeta)
		if err != nil {
			s.logger.Warn("Failed to read block %d: %v", blockMeta.ID, err)
			continue
		}
		allEvents = append(allEvents, events...)
	}

	return allEvents, nil
}

// AddEventsBatch ingests a batch of events and merges them into storage blocks
func (s *Storage) AddEventsBatch(events []*models.Event, opts ImportOptions) (*ImportReport, error) {
	startTime := time.Now()
	report := &ImportReport{
		Errors: make([]string, 0),
	}

	if len(events) == 0 {
		report.Duration = time.Since(startTime)
		return report, nil
	}

	s.logger.InfoWithFields("Starting batch event ingestion",
		logging.Field("event_count", len(events)))

	// Validate all events first
	for i, event := range events {
		if event == nil {
			report.Errors = append(report.Errors, fmt.Sprintf("event %d is nil", i))
			continue
		}
		if err := event.Validate(); err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("event %d validation failed: %v", i, err))
			continue
		}
	}

	if len(report.Errors) > 0 {
		s.logger.Warn("Batch event validation encountered %d errors", len(report.Errors))
	}

	// Group events by hour timestamp
	hourGroups := make(map[int64][]*models.Event)
	for _, event := range events {
		if event == nil {
			continue
		}
		hourTime := time.Unix(0, event.Timestamp).UTC()
		// Round down to hour boundary
		hourTimestamp := hourTime.Unix() - int64(hourTime.Second()) - int64(hourTime.Minute())*60
		hourGroups[hourTimestamp] = append(hourGroups[hourTimestamp], event)
	}

	// Process each hour group
	totalEvents := int64(0)
	for hourTimestamp, hourEvents := range hourGroups {
		if err := s.ingestHourEvents(hourTimestamp, hourEvents, opts); err != nil {
			hourTime := time.Unix(hourTimestamp, 0).Local()
			report.Errors = append(report.Errors, fmt.Sprintf("failed to ingest hour %s: %v", hourTime.Format("2006-01-02-15"), err))
		} else {
			totalEvents += int64(len(hourEvents))
			report.MergedHours++
		}
	}

	report.TotalEvents = totalEvents
	report.ImportedFiles = len(events) // Track individual events as items imported
	report.Duration = time.Since(startTime)

	// Rebuild file index to make new files discoverable by queries
	if err := s.fileIndex.Rebuild(s.dataDir, s.extractHourFromFilename); err != nil {
		s.logger.Warn("Failed to rebuild file index after batch import: %v", err)
		// Don't fail the import if index rebuild fails - the data is still there
	} else {
		s.logger.Info("File index rebuilt after batch import")
	}

	s.logger.InfoWithFields("Batch event ingestion completed",
		logging.Field("total_events", totalEvents),
		logging.Field("merged_hours", report.MergedHours),
		logging.Field("errors", len(report.Errors)),
		logging.Field("duration", report.Duration))

	return report, nil
}

// ingestHourEvents ingests events for a specific hour, merging with existing data
func (s *Storage) ingestHourEvents(hourTimestamp int64, events []*models.Event, opts ImportOptions) error {
	s.fileMutex.Lock()
	defer s.fileMutex.Unlock()

	// Determine the canonical filename for this hour
	// Use Local timezone to match extractHourFromFilename and getOrCreateCurrentFile
	hourTime := time.Unix(hourTimestamp, 0).Local()
	canonicalFilename := fmt.Sprintf("%04d-%02d-%02d-%02d.bin",
		hourTime.Year(), hourTime.Month(), hourTime.Day(), hourTime.Hour())
	canonicalPath := filepath.Join(s.dataDir, canonicalFilename)

	// Check if this is the currently open hour
	if s.currentFile != nil && s.currentFile.hourTimestamp == hourTimestamp {
		// Write events directly to the current file
		for _, event := range events {
			if err := s.currentFile.WriteEvent(event); err != nil {
				return fmt.Errorf("failed to write event to current file: %w", err)
			}
		}
		return nil
	}

	// Check if the hour file exists
	_, err := os.Stat(canonicalPath)
	fileExists := err == nil

	if fileExists && !opts.OverwriteExisting {
		// Skip merge, just return success
		s.logger.DebugWithFields("Skipping merge for existing hour",
			logging.Field("hour", hourTime.Format("2006-01-02-15")))
		return nil
	}

	// Collect all events (existing + new)
	var sourceEvents []*models.Event

	// Add existing file events if it exists
	if fileExists {
		existingEvents, err := s.readAllEventsFromFile(canonicalPath)
		if err != nil {
			s.logger.Warn("Failed to read existing file %s: %v", canonicalPath, err)
		} else {
			sourceEvents = append(sourceEvents, existingEvents...)
		}
	}

	// Add new events
	sourceEvents = append(sourceEvents, events...)

	// Create temporary file for the merged result
	tempMergedPath := filepath.Join(s.dataDir, fmt.Sprintf("%s.ingesting-%d", canonicalFilename, time.Now().Unix()))

	// Perform the merge
	eventCount, err := s.mergeEventsIntoNew(sourceEvents, tempMergedPath, hourTimestamp)
	if err != nil {
		// Clean up temp file on error
		_ = os.Remove(tempMergedPath)
		return fmt.Errorf("failed to merge events: %w", err)
	}

	// Atomically replace the canonical file with the merged one
	if err := os.Rename(tempMergedPath, canonicalPath); err != nil {
		_ = os.Remove(tempMergedPath)
		return fmt.Errorf("failed to replace canonical file: %w", err)
	}

	s.logger.DebugWithFields("Ingested hour events",
		logging.Field("hour", hourTime.Format("2006-01-02-15")),
		logging.Field("events", eventCount))

	return nil
}

// mergeEventsIntoNew writes events to a new file with sorting and deduplication
func (s *Storage) mergeEventsIntoNew(events []*models.Event, targetPath string, hourTimestamp int64) (int64, error) {
	if len(events) == 0 {
		return 0, fmt.Errorf("no events to merge")
	}

	// Sort events by timestamp for better compression and query performance
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp < events[j].Timestamp
	})

	// Create new storage file
	newFile, err := NewBlockStorageFile(targetPath, hourTimestamp, s.blockSize)
	if err != nil {
		return 0, fmt.Errorf("failed to create new storage file: %w", err)
	}

	// Write all events
	for _, event := range events {
		if err := newFile.WriteEvent(event); err != nil {
			_ = newFile.Close()
			return 0, fmt.Errorf("failed to write event: %w", err)
		}
	}

	// Close to finalize (writes index and footer)
	if err := newFile.Close(); err != nil {
		return 0, fmt.Errorf("failed to close new file: %w", err)
	}

	return int64(len(events)), nil
}
