package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/moolen/spectre/internal/api"
	"github.com/moolen/spectre/internal/importexport"
	"github.com/moolen/spectre/internal/logging"
	"github.com/moolen/spectre/internal/models"
)

type fakeStartupImportBatchIngestor struct {
	batches [][]models.Event
	err     error
}

var _ api.BatchIngestor = (*fakeStartupImportBatchIngestor)(nil)

func captureStartupImportOutput(t *testing.T, fn func()) string {
	t.Helper()

	oldLogWriter := log.Writer()
	defer log.SetOutput(oldLogWriter)

	var stdoutBuf bytes.Buffer
	log.SetOutput(&stdoutBuf)

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}
	os.Stderr = w

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close stderr writer: %v", err)
	}
	os.Stderr = oldStderr

	var stderrBuf bytes.Buffer
	if _, err := io.Copy(&stderrBuf, r); err != nil {
		t.Fatalf("failed to read stderr buffer: %v", err)
	}

	return stdoutBuf.String() + stderrBuf.String()
}

func (f *fakeStartupImportBatchIngestor) ProcessBatch(ctx context.Context, events []models.Event) error {
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
	pipeline := &fakeStartupImportBatchIngestor{}
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
		BatchIngestor:    pipeline,
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
		Path:          "fixtures/import.json",
		ChunkSize:     10,
		Logger:        logger,
		BatchIngestor: &fakeStartupImportBatchIngestor{err: errPipeline},
		Stream:        streamFn,
		ImportMode:    false,
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
		Path:          "fixtures/import.json",
		ChunkSize:     10,
		Logger:        logger,
		BatchIngestor: &fakeStartupImportBatchIngestor{},
		Stream:        streamFn,
		ImportMode:    false,
	})
	if !errors.Is(err, errStream) {
		t.Fatalf("expected stream error, got: %v", err)
	}
}

func TestRunStartupImportEmptyBenchmarkPathDoesNotWriteFile(t *testing.T) {
	t.Parallel()

	logger := logging.GetLogger("test_startup_import_no_benchmark")
	pipeline := &fakeStartupImportBatchIngestor{}
	reportPath := filepath.Join(t.TempDir(), "import-report.json")

	streamFn := func(source importexport.ImportSource, chunkSize int, onChunk importexport.ChunkCallback, opts ...importexport.ImportOption) error {
		return onChunk([]models.Event{{ID: "a"}})
	}

	err := runStartupImport(context.Background(), startupImportOptions{
		Path:             "fixtures/import.json",
		ChunkSize:        10,
		BenchmarkLogPath: "",
		Logger:           logger,
		BatchIngestor:    pipeline,
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

func TestRunStartupImportLogsImportCompletedMessage(t *testing.T) {
	os.Setenv("LOG_TIMESTAMP", "2024-01-01T12:00:00Z")
	defer os.Unsetenv("LOG_TIMESTAMP")

	logger := logging.GetLogger("test_startup_import_logs")
	pipeline := &fakeStartupImportBatchIngestor{}
	streamFn := func(source importexport.ImportSource, chunkSize int, onChunk importexport.ChunkCallback, opts ...importexport.ImportOption) error {
		return onChunk([]models.Event{{ID: "a"}})
	}

	output := captureStartupImportOutput(t, func() {
		err := runStartupImport(context.Background(), startupImportOptions{
			Path:          "fixtures/import.json",
			ChunkSize:     10,
			Logger:        logger,
			BatchIngestor: pipeline,
			Stream:        streamFn,
			ImportMode:    false,
		})
		if err != nil {
			t.Fatalf("runStartupImport returned error: %v", err)
		}
	})

	if !strings.Contains(output, "Import completed") {
		t.Fatalf("expected startup import logs to contain %q, got %q", "Import completed", output)
	}
}

func TestServerCommandDefinesStartupImportDisableCausalityFlag(t *testing.T) {
	t.Parallel()

	flag := serverCmd.Flags().Lookup("startup-import-disable-causality")
	if flag == nil {
		t.Fatal("expected startup-import-disable-causality flag to be registered")
	}
}

func TestServerCommandDefinesStartupImportTimelineOnlyFlag(t *testing.T) {
	t.Parallel()

	flag := serverCmd.Flags().Lookup("startup-import-timeline-only")
	if flag == nil {
		t.Fatal("expected startup-import-timeline-only flag to be registered")
	}
}
