package storage

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
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

	// Acquire read lock to enumerate files
	s.fileMutex.RLock()

	// If IncludeOpenHour is true, we need to close the current file
	if opts.IncludeOpenHour && s.currentFile != nil {
		s.fileMutex.RUnlock()
		s.fileMutex.Lock()
		if err := s.currentFile.Close(); err != nil {
			s.logger.Warn("Failed to close current file for export: %v", err)
		}
		s.currentFile = nil
		s.fileMutex.Unlock()
		s.fileMutex.RLock()
	}

	// Get all storage files
	allFiles, err := s.getStorageFiles()
	if err != nil {
		s.fileMutex.RUnlock()
		return fmt.Errorf("failed to list storage files: %w", err)
	}

	// Filter files by time range
	selectedFiles := s.filterFilesByTimeRange(allFiles, opts.StartTime, opts.EndTime)
	s.fileMutex.RUnlock()

	if len(selectedFiles) == 0 {
		s.logger.Warn("No files match the export criteria")
		return fmt.Errorf("no files to export")
	}

	// Create the archive writer
	var archiveWriter io.Writer = w
	var gzipWriter *gzip.Writer

	if opts.Compression {
		gzipWriter = gzip.NewWriter(w)
		archiveWriter = gzipWriter
		defer gzipWriter.Close()
	}

	tarWriter := tar.NewWriter(archiveWriter)
	defer tarWriter.Close()

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
			continue
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
		reader.Close()
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
	file, err := os.Open(filePath)
	if err != nil {
		return ExportedFileEntry{}, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(tw, file); err != nil {
		return ExportedFileEntry{}, fmt.Errorf("failed to copy file contents: %w", err)
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

	var filtered []string
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
		if endTime > 0 && hourTimestamp > endTime {
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
		defer gzipReader.Close()
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
	defer os.RemoveAll(tempDir)

	var manifest *ExportManifest
	extractedFiles := make(map[string]string) // archive path -> temp file path

	// Extract all files from archive
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
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
		tempFile, err := os.Create(tempFilePath)
		if err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("failed to create temp file for %s: %v", header.Name, err))
			continue
		}

		if _, err := io.Copy(tempFile, tarReader); err != nil {
			tempFile.Close()
			report.Errors = append(report.Errors, fmt.Sprintf("failed to extract %s: %v", header.Name, err))
			continue
		}
		tempFile.Close()

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
	defer reader.Close()

	// Try to read the complete file structure
	_, err = reader.ReadFile()
	if err != nil {
		return fmt.Errorf("failed to read file structure: %w", err)
	}

	return nil
}

// importHourGroup merges all files for a given hour into a single canonical file
func (s *Storage) importHourGroup(hourTimestamp int64, importedFilePaths []string, opts ImportOptions, report *ImportReport) error {
	s.fileMutex.Lock()
	defer s.fileMutex.Unlock()

	// Determine the canonical filename for this hour
	hourTime := time.Unix(hourTimestamp, 0).UTC()
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
		os.Remove(tempMergedPath)
		return fmt.Errorf("failed to merge files: %w", err)
	}

	report.TotalEvents += eventCount

	// Atomically replace the canonical file with the merged one
	if err := os.Rename(tempMergedPath, canonicalPath); err != nil {
		os.Remove(tempMergedPath)
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
			newFile.Close()
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
	defer reader.Close()

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
