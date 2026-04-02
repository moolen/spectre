package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/importexport"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

const defaultStartupImportChunkSize = 1024

type startupImportOptions struct {
	Path             string
	ChunkSize        int
	BenchmarkLogPath string
	ImportMode       bool
	Logger           *logging.Logger
	BatchIngestor    api.BatchIngestor
	Pipeline         api.BatchIngestor // Deprecated compatibility alias for existing callers
	Stream           func(source importexport.ImportSource, chunkSize int, onChunk importexport.ChunkCallback, opts ...importexport.ImportOption) error
}

type startupImportBenchmarkReport struct {
	ImportPath           string `json:"import_path"`
	ImportMode           bool   `json:"import_mode"`
	ChunkSize            int    `json:"chunk_size"`
	TotalEvents          int    `json:"total_events"`
	TotalChunks          int    `json:"total_chunks"`
	ParseDurationNanos   int64  `json:"parse_duration_nanos"`
	ProcessDurationNanos int64  `json:"process_duration_nanos"`
	TotalDurationNanos   int64  `json:"total_duration_nanos"`
}

func runStartupImport(ctx context.Context, opts startupImportOptions) error {
	if opts.Path == "" {
		return nil
	}

	logger := opts.Logger
	if logger == nil {
		logger = logging.GetLogger("server")
	}

	if opts.BatchIngestor != nil && opts.Pipeline != nil {
		logger.Warn("Both BatchIngestor and deprecated Pipeline are set; using BatchIngestor")
	}

	batchIngestor := opts.BatchIngestor
	if batchIngestor == nil {
		batchIngestor = opts.Pipeline
	}
	if batchIngestor == nil {
		return fmt.Errorf("startup import batch ingestor is required")
	}

	chunkSize := opts.ChunkSize
	if chunkSize <= 0 {
		chunkSize = defaultStartupImportChunkSize
	}

	importInChunks := opts.Stream
	if importInChunks == nil {
		importInChunks = importexport.ImportInChunks
	}

	logger.InfoWithFields("Starting startup import",
		logging.Field("path", opts.Path),
		logging.Field("chunk_size", chunkSize),
		logging.Field("import_mode", opts.ImportMode))

	totalStart := time.Now()
	processDuration := time.Duration(0)
	totalEvents := 0
	totalChunks := 0

	streamErr := importInChunks(
		importexport.FromPath(opts.Path),
		chunkSize,
		func(chunk []models.Event) error {
			totalChunks++
			totalEvents += len(chunk)

			processStart := time.Now()
			if err := batchIngestor.ProcessBatch(ctx, chunk); err != nil {
				return err
			}
			processDuration += time.Since(processStart)
			return nil
		},
		importexport.WithLogger(logger),
	)
	if streamErr != nil {
		return streamErr
	}

	totalDuration := time.Since(totalStart)
	parseDuration := totalDuration - processDuration
	if parseDuration < 0 {
		parseDuration = 0
	}

	logger.InfoWithFields("Import completed",
		logging.Field("path", opts.Path),
		logging.Field("chunk_size", chunkSize),
		logging.Field("event_count", totalEvents),
		logging.Field("chunk_count", totalChunks),
		logging.Field("parse_duration", parseDuration),
		logging.Field("process_duration", processDuration),
		logging.Field("total_duration", totalDuration))

	if opts.BenchmarkLogPath != "" {
		report := startupImportBenchmarkReport{
			ImportPath:           opts.Path,
			ImportMode:           opts.ImportMode,
			ChunkSize:            chunkSize,
			TotalEvents:          totalEvents,
			TotalChunks:          totalChunks,
			ParseDurationNanos:   parseDuration.Nanoseconds(),
			ProcessDurationNanos: processDuration.Nanoseconds(),
			TotalDurationNanos:   totalDuration.Nanoseconds(),
		}
		reportBytes, err := json.Marshal(report)
		if err != nil {
			return fmt.Errorf("marshal startup import benchmark report: %w", err)
		}
		if err := os.WriteFile(opts.BenchmarkLogPath, reportBytes, 0o600); err != nil {
			return fmt.Errorf("write startup import benchmark report: %w", err)
		}
		logger.InfoWithFields("Startup import benchmark report written",
			logging.Field("path", opts.BenchmarkLogPath))
	}

	return nil
}
