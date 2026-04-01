package importexport

import (
	"errors"
	"fmt"

	"github.com/moolen/spectre/internal/importexport/fileio"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

const defaultImportChunkSize = 1024

// ChunkCallback receives chunks of imported events.
type ChunkCallback func(chunk []models.Event) error

type streamImportSource interface {
	streamLoad(logger *logging.Logger, chunkSize int, onChunk ChunkCallback) error
}

type chunkCallbackError struct {
	err error
}

func (e chunkCallbackError) Error() string {
	return e.err.Error()
}

func (e chunkCallbackError) Unwrap() error {
	return e.err
}

func wrapChunkCallbackError(err error) error {
	if err == nil {
		return nil
	}
	var callbackErr chunkCallbackError
	if errors.As(err, &callbackErr) {
		return err
	}
	return chunkCallbackError{err: err}
}

// ImportInChunks imports from source and invokes callback with fixed-size chunks.
func ImportInChunks(source ImportSource, chunkSize int, onChunk ChunkCallback, opts ...ImportOption) error {
	if source == nil {
		return fmt.Errorf("import source cannot be nil")
	}
	if chunkSize <= 0 {
		return fmt.Errorf("chunk size must be greater than zero")
	}
	if onChunk == nil {
		return fmt.Errorf("chunk callback cannot be nil")
	}

	options := &ImportOptions{
		logger: logging.GetLogger("importexport"),
	}
	for _, opt := range opts {
		opt(options)
	}

	if streamSource, ok := source.(streamImportSource); ok {
		return streamSource.streamLoad(options.logger, chunkSize, onChunk)
	}

	events, err := source.Load(options.logger)
	if err != nil {
		return err
	}

	return emitChunks(events, chunkSize, onChunk)
}

func collectAllFromStream(source streamImportSource, logger *logging.Logger) ([]models.Event, error) {
	var all []models.Event
	err := source.streamLoad(logger, defaultImportChunkSize, func(chunk []models.Event) error {
		all = append(all, chunk...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

func emitChunks(events []models.Event, chunkSize int, onChunk ChunkCallback) error {
	for start := 0; start < len(events); start += chunkSize {
		end := start + chunkSize
		if end > len(events) {
			end = len(events)
		}
		if err := onChunk(events[start:end]); err != nil {
			return wrapChunkCallbackError(err)
		}
	}
	return nil
}

func (s *fileSource) streamLoad(logger *logging.Logger, chunkSize int, onChunk ChunkCallback) error {
	logger.InfoWithFields("Loading events from file",
		logging.Field("path", s.path))

	reader := fileio.NewFileReader(logger)
	file, err := reader.ReadFile(s.path)
	if err != nil {
		logger.ErrorWithFields("Failed to read file",
			logging.Field("path", s.path),
			logging.Field("error", err))
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Warn("Failed to close file %s: %v", s.path, closeErr)
		}
	}()

	err = parseJSONEventsInChunks(file, chunkSize, logger, onChunk)
	if err != nil {
		logger.ErrorWithFields("Failed to parse JSON events",
			logging.Field("path", s.path),
			logging.Field("error", err))
		return err
	}

	logger.InfoWithFields("Successfully loaded events from file",
		logging.Field("path", s.path))

	return nil
}

func (s *readerSource) streamLoad(logger *logging.Logger, chunkSize int, onChunk ChunkCallback) error {
	logger.Debug("Loading events from reader")

	err := parseJSONEventsInChunks(s.reader, chunkSize, logger, onChunk)
	if err != nil {
		logger.ErrorWithFields("Failed to parse JSON events from reader",
			logging.Field("error", err))
		return err
	}
	logger.InfoWithFields("Successfully loaded events from reader")
	return nil
}

func (s *directorySource) streamLoad(logger *logging.Logger, chunkSize int, onChunk ChunkCallback) error {
	logger.InfoWithFields("Loading events from directory",
		logging.Field("path", s.path))

	walker := fileio.NewDirectoryWalker(logger)
	files, err := walker.WalkJSON(s.path)
	if err != nil {
		logger.ErrorWithFields("Failed to walk directory",
			logging.Field("path", s.path),
			logging.Field("error", err))
		return err
	}

	logger.InfoWithFields("Found JSON files in directory",
		logging.Field("path", s.path),
		logging.Field("file_count", len(files)))

	buffer := make([]models.Event, 0, chunkSize)
	successCount := 0
	failureCount := 0
	totalEvents := 0

	flushBuffer := func(force bool) error {
		for len(buffer) >= chunkSize || (force && len(buffer) > 0) {
			size := chunkSize
			if len(buffer) < size {
				size = len(buffer)
			}
			if err := onChunk(buffer[:size]); err != nil {
				return wrapChunkCallbackError(err)
			}
			totalEvents += size
			buffer = buffer[size:]
		}
		return nil
	}

	for _, file := range files {
		logger.DebugWithFields("Importing file",
			logging.Field("path", file.FilePath),
			logging.Field("size_bytes", file.Size))

		fileSource := fileSource{path: file.FilePath}
		loadErr := fileSource.streamLoad(logger, chunkSize, func(chunk []models.Event) error {
			buffer = append(buffer, chunk...)
			return flushBuffer(false)
		})
		if loadErr != nil {
			var callbackErr chunkCallbackError
			if errors.As(loadErr, &callbackErr) {
				return callbackErr
			}
			failureCount++
			logger.WarnWithFields("Failed to import file, skipping",
				logging.Field("path", file.FilePath),
				logging.Field("error", loadErr))
			continue
		}
		successCount++
	}

	if err := flushBuffer(true); err != nil {
		return err
	}

	if totalEvents == 0 {
		logger.ErrorWithFields("No events imported from directory",
			logging.Field("path", s.path),
			logging.Field("files_found", len(files)),
			logging.Field("failures", failureCount))
		return fmt.Errorf("no events found in directory %s (processed %d files, %d failures)", s.path, len(files), failureCount)
	}

	logger.InfoWithFields("Successfully loaded events from directory",
		logging.Field("path", s.path),
		logging.Field("event_count", totalEvents),
		logging.Field("files_processed", successCount),
		logging.Field("files_failed", failureCount))

	return nil
}

func (s *pathSource) streamLoad(logger *logging.Logger, chunkSize int, onChunk ChunkCallback) error {
	logger.DebugWithFields("Detecting path type",
		logging.Field("path", s.path))

	pathType, err := fileio.DetectPathType(s.path)
	if err != nil {
		logger.ErrorWithFields("Failed to detect path type",
			logging.Field("path", s.path),
			logging.Field("error", err))
		return err
	}

	switch pathType {
	case fileio.PathTypeDirectory:
		logger.Debug("Path is a directory, using directory import")
		directorySource := directorySource{path: s.path}
		return directorySource.streamLoad(logger, chunkSize, onChunk)
	case fileio.PathTypeFile:
		logger.Debug("Path is a file, using file import")
		fileSource := fileSource{path: s.path}
		return fileSource.streamLoad(logger, chunkSize, onChunk)
	case fileio.PathTypeUnknown:
		return fmt.Errorf("unknown path type for %s", s.path)
	default:
		return fmt.Errorf("unknown path type for %s", s.path)
	}
}
