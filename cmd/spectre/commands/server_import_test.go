package commands

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/moolen/spectre/internal/importexport"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

type fakeStartupImportPipeline struct {
	batches [][]models.Event
	err     error
}

func (f *fakeStartupImportPipeline) ProcessBatch(ctx context.Context, events []models.Event) error {
	if f.err != nil {
		return f.err
	}
	chunk := make([]models.Event, len(events))
	copy(chunk, events)
	f.batches = append(f.batches, chunk)
	return nil
}

func TestRunStartupImportProcessesChunksAndWritesReport(t *testing.T) {
	t.Parallel()

	logger := logging.GetLogger("test_startup_import")
	pipeline := &fakeStartupImportPipeline{}
	reportPath := filepath.Join(t.TempDir(), "import-report.json")

	var gotChunkSize int
	streamCalls := 0
	streamFn := func(source importexport.ImportSource, chunkSize int, onChunk importexport.ChunkCallback, opts ...importexport.ImportOption) error {
		streamCalls++
		gotChunkSize = chunkSize
		if err := onChunk([]models.Event{{ID: "a"}, {ID: "b"}}); err != nil {
			return err
		}
		return onChunk([]models.Event{{ID: "c"}})
	}

	err := runStartupImport(context.Background(), startupImportOptions{
		Path:             "fixtures/import.json",
		ChunkSize:        99,
		BenchmarkLogPath: reportPath,
		ImportMode:       true,
		Logger:           logger,
		Pipeline:         pipeline,
		Stream:           streamFn,
	})
	if err != nil {
		t.Fatalf("runStartupImport returned error: %v", err)
	}

	if streamCalls != 1 {
		t.Fatalf("expected stream function to be called once, got %d", streamCalls)
	}
	if gotChunkSize != 99 {
		t.Fatalf("expected chunk size 99, got %d", gotChunkSize)
	}

	gotBatchSizes := []int{len(pipeline.batches[0]), len(pipeline.batches[1])}
	wantBatchSizes := []int{2, 1}
	if !reflect.DeepEqual(gotBatchSizes, wantBatchSizes) {
		t.Fatalf("unexpected batch sizes: got %v, want %v", gotBatchSizes, wantBatchSizes)
	}

	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("failed to read report file: %v", err)
	}

	var report startupImportBenchmarkReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("failed to unmarshal report JSON: %v", err)
	}

	if report.TotalEvents != 3 {
		t.Fatalf("expected total_events 3, got %d", report.TotalEvents)
	}
	if report.TotalChunks != 2 {
		t.Fatalf("expected total_chunks 2, got %d", report.TotalChunks)
	}
	if report.ChunkSize != 99 {
		t.Fatalf("expected chunk_size 99, got %d", report.ChunkSize)
	}
	if !report.ImportMode {
		t.Fatalf("expected import_mode true, got %v", report.ImportMode)
	}
}

func TestRunStartupImportPropagatesPipelineError(t *testing.T) {
	t.Parallel()

	logger := logging.GetLogger("test_startup_import_pipeline_error")
	errPipeline := errors.New("pipeline failed")

	streamFn := func(source importexport.ImportSource, chunkSize int, onChunk importexport.ChunkCallback, opts ...importexport.ImportOption) error {
		return onChunk([]models.Event{{ID: "a"}})
	}

	err := runStartupImport(context.Background(), startupImportOptions{
		Path:       "fixtures/import.json",
		ChunkSize:  10,
		Logger:     logger,
		Pipeline:   &fakeStartupImportPipeline{err: errPipeline},
		Stream:     streamFn,
		ImportMode: false,
	})
	if !errors.Is(err, errPipeline) {
		t.Fatalf("expected pipeline error, got: %v", err)
	}
}

func TestRunStartupImportPropagatesStreamError(t *testing.T) {
	t.Parallel()

	logger := logging.GetLogger("test_startup_import_stream_error")
	errStream := errors.New("stream failed")

	streamFn := func(source importexport.ImportSource, chunkSize int, onChunk importexport.ChunkCallback, opts ...importexport.ImportOption) error {
		return errStream
	}

	err := runStartupImport(context.Background(), startupImportOptions{
		Path:       "fixtures/import.json",
		ChunkSize:  10,
		Logger:     logger,
		Pipeline:   &fakeStartupImportPipeline{},
		Stream:     streamFn,
		ImportMode: false,
	})
	if !errors.Is(err, errStream) {
		t.Fatalf("expected stream error, got: %v", err)
	}
}

func TestRunStartupImportEmptyBenchmarkPathDoesNotWriteFile(t *testing.T) {
	t.Parallel()

	logger := logging.GetLogger("test_startup_import_no_benchmark")
	pipeline := &fakeStartupImportPipeline{}
	reportPath := filepath.Join(t.TempDir(), "import-report.json")

	streamFn := func(source importexport.ImportSource, chunkSize int, onChunk importexport.ChunkCallback, opts ...importexport.ImportOption) error {
		return onChunk([]models.Event{{ID: "a"}})
	}

	err := runStartupImport(context.Background(), startupImportOptions{
		Path:             "fixtures/import.json",
		ChunkSize:        10,
		BenchmarkLogPath: "",
		Logger:           logger,
		Pipeline:         pipeline,
		Stream:           streamFn,
		ImportMode:       false,
	})
	if err != nil {
		t.Fatalf("runStartupImport returned error: %v", err)
	}

	_, statErr := os.Stat(reportPath)
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected no report file to be written, stat error: %v", statErr)
	}
}
