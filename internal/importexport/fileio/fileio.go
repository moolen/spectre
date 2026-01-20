// Package fileio provides file system operations for event import.
//
// This package handles:
//   - Opening and reading JSON files
//   - Directory traversal and JSON file discovery
//   - Path validation and error handling
//
// All operations include proper resource cleanup and detailed error reporting.
package fileio

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/moolen/spectre/internal/logging"
)

// FileReader handles opening and reading files with proper error handling
type FileReader struct {
	logger *logging.Logger
}

// NewFileReader creates a new file reader
func NewFileReader(logger *logging.Logger) *FileReader {
	return &FileReader{logger: logger}
}

// ReadFile opens and returns a reader for the specified file path
// Caller is responsible for closing the returned ReadCloser
func (r *FileReader) ReadFile(path string) (io.ReadCloser, error) {
	// Validate path
	if path == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}

	// Check if file exists and is readable
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file does not exist: %s", path)
		}
		return nil, fmt.Errorf("failed to stat file %s: %w", path, err)
	}

	// Check if it's actually a file
	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file: %s", path)
	}

	// Open the file
	// path is user-provided for import/export operations
	// #nosec G304 -- Import/export file path is intentionally user-configurable
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", path, err)
	}

	r.logger.Debug("Opened file: %s (size: %d bytes)", path, info.Size())
	return file, nil
}

// DirectoryWalker handles recursive directory traversal for JSON files
type DirectoryWalker struct {
	logger *logging.Logger
}

// NewDirectoryWalker creates a new directory walker
func NewDirectoryWalker(logger *logging.Logger) *DirectoryWalker {
	return &DirectoryWalker{logger: logger}
}

// WalkResult contains information about files found during traversal
type WalkResult struct {
	FilePath string
	Size     int64
}

// WalkJSON recursively finds all JSON files in the specified directory
func (w *DirectoryWalker) WalkJSON(dirPath string) ([]WalkResult, error) {
	// Validate directory path
	if dirPath == "" {
		return nil, fmt.Errorf("directory path cannot be empty")
	}

	// Check if directory exists
	info, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("directory does not exist: %s", dirPath)
		}
		return nil, fmt.Errorf("failed to stat directory %s: %w", dirPath, err)
	}

	// Check if it's actually a directory
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", dirPath)
	}

	var results []WalkResult
	var walkErrors []string

	err = filepath.Walk(dirPath, func(filePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			walkErrors = append(walkErrors, fmt.Sprintf("error accessing %s: %v", filePath, err))
			w.logger.Warn("Error walking path %s: %v", filePath, err)
			// Continue walking even if there's an error with one file
			return nil
		}

		// Skip directories
		if fileInfo.IsDir() {
			w.logger.Debug("Skipping directory: %s", filePath)
			return nil
		}

		// Only process JSON files
		if !isJSONFile(filePath) {
			w.logger.Debug("Skipping non-JSON file: %s", filePath)
			return nil
		}

		// Add to results
		w.logger.Debug("Found JSON file: %s (size: %d bytes)", filePath, fileInfo.Size())
		results = append(results, WalkResult{
			FilePath: filePath,
			Size:     fileInfo.Size(),
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", dirPath, err)
	}

	if len(results) == 0 {
		if len(walkErrors) > 0 {
			return nil, fmt.Errorf("no JSON files found in %s (encountered %d errors during traversal)", dirPath, len(walkErrors))
		}
		return nil, fmt.Errorf("no JSON files found in directory: %s", dirPath)
	}

	w.logger.InfoWithFields("Directory walk completed",
		logging.Field("directory", dirPath),
		logging.Field("files_found", len(results)),
		logging.Field("errors", len(walkErrors)))

	return results, nil
}

// isJSONFile returns true if the file has a .json extension (case-insensitive)
func isJSONFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".json")
}

// PathType represents the type of path (file or directory)
type PathType int

const (
	PathTypeUnknown PathType = iota
	PathTypeFile
	PathTypeDirectory
)

// DetectPathType determines if a path points to a file or directory
func DetectPathType(path string) (PathType, error) {
	if path == "" {
		return PathTypeUnknown, fmt.Errorf("path cannot be empty")
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return PathTypeUnknown, fmt.Errorf("path does not exist: %s", path)
		}
		return PathTypeUnknown, fmt.Errorf("failed to stat path %s: %w", path, err)
	}

	if info.IsDir() {
		return PathTypeDirectory, nil
	}
	return PathTypeFile, nil
}
