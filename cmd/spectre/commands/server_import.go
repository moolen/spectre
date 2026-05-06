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
	"github.com/moolen/spectre/internal/scrub"
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
	Scrubber         *scrub.Scrubber
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
	session := importexport.NewIngestSession(batchIngestor, logger, opts.Scrubber)

	streamErr := importInChunks(
		importexport.FromPath(opts.Path),
		chunkSize,
		func(chunk []models.Event) error {
			return session.ProcessChunk(ctx, chunk)
		},
		importexport.WithLogger(logger),
	)
	if streamErr != nil {
		return streamErr
	}

	stats := session.Stats()
	totalDuration := time.Since(totalStart)
	parseDuration := totalDuration - stats.ProcessDuration
	if parseDuration < 0 {
		parseDuration = 0
	}

	logger.InfoWithFields("Import completed",
		logging.Field("path", opts.Path),
		logging.Field("chunk_size", chunkSize),
		logging.Field("event_count", stats.TotalEvents),
		logging.Field("chunk_count", stats.TotalChunks),
		logging.Field("parse_duration", parseDuration),
		logging.Field("process_duration", stats.ProcessDuration),
		logging.Field("total_duration", totalDuration))

	if opts.BenchmarkLogPath != "" {
		report := startupImportBenchmarkReport{
			ImportPath:           opts.Path,
			ImportMode:           opts.ImportMode,
			ChunkSize:            chunkSize,
			TotalEvents:          stats.TotalEvents,
			TotalChunks:          stats.TotalChunks,
			ParseDurationNanos:   parseDuration.Nanoseconds(),
			ProcessDurationNanos: stats.ProcessDuration.Nanoseconds(),
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
