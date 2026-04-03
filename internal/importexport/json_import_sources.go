package importexport

import (
	"io"

	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

// fileSource imports events from a single JSON file.
type fileSource struct {
	path string
}

// FromFile creates an import source for a single JSON file.
func FromFile(path string) ImportSource {
	return &fileSource{path: path}
}

func (s *fileSource) Load(logger *logging.Logger) ([]models.Event, error) {
	return collectAllFromStream(s, logger)
}

// readerSource imports events from an io.Reader.
type readerSource struct {
	reader io.Reader
}

// FromReader creates an import source for an io.Reader.
func FromReader(reader io.Reader) ImportSource {
	return &readerSource{reader: reader}
}

func (s *readerSource) Load(logger *logging.Logger) ([]models.Event, error) {
	return collectAllFromStream(s, logger)
}

// directorySource imports events from all JSON files in a directory (recursive).
type directorySource struct {
	path string
}

// FromDirectory creates an import source that recursively imports all JSON files
// in the specified directory.
func FromDirectory(path string) ImportSource {
	return &directorySource{path: path}
}

func (s *directorySource) Load(logger *logging.Logger) ([]models.Event, error) {
	return collectAllFromStream(s, logger)
}

// pathSource automatically detects whether the path is a file or directory.
type pathSource struct {
	path string
}

// FromPath creates an import source that automatically detects whether the path
// is a file or directory and imports accordingly.
func FromPath(path string) ImportSource {
	return &pathSource{path: path}
}

func (s *pathSource) Load(logger *logging.Logger) ([]models.Event, error) {
	return collectAllFromStream(s, logger)
}
