package graph

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/moolen/spectre/internal/importexport"
	"github.com/moolen/spectre/internal/models"
)

func TestStartupImport_StreamedChunksAgainstRealPipeline(t *testing.T) {
	harness, err := NewTestHarness(t)
	if err != nil {
		t.Fatalf("failed to create test harness: %v", err)
	}
	defer harness.Cleanup(context.Background())

	ctx := context.Background()
	importDir := generateImportDataset(t)

	totalEvents, totalChunks, err := runStreamedImport(ctx, harness, importDir, 7)
	if err != nil {
		t.Fatalf("failed to run streamed import: %v", err)
	}

	if totalEvents <= 0 {
		t.Fatalf("expected imported events > 0, got %d", totalEvents)
	}
	if totalChunks < 2 {
		t.Fatalf("expected streamed import to process multiple chunks, got %d", totalChunks)
	}

	resourceCount := CountResources(t, harness.GetClient())
	if resourceCount < 20 {
		t.Fatalf("expected at least 20 resources from synthetic dataset, got %d", resourceCount)
	}
}

func generateImportDataset(t *testing.T) string {
	t.Helper()

	outputDir := filepath.Join(t.TempDir(), "startup-import-data")
	cmd := exec.Command(
		"go", "run", "./cmd/spectre", "debug", "generate-import-data",
		"--output-dir", outputDir,
		"--seed", "42",
		"--kinds", "5",
		"--resources", "25",
		"--namespaces", "3",
	)
	cmd.Dir = repoRootFromTestFile(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate import dataset failed: %v\noutput:\n%s", err, string(out))
	}

	return outputDir
}

func runStreamedImport(ctx context.Context, harness *TestHarness, importDir string, chunkSize int) (int, int, error) {
	totalEvents := 0
	totalChunks := 0

	err := importexport.ImportInChunks(importexport.FromPath(importDir), chunkSize, func(chunk []models.Event) error {
		totalChunks++
		totalEvents += len(chunk)
		return harness.GetPipeline().ProcessBatch(ctx, chunk)
	})
	if err != nil {
		return 0, 0, fmt.Errorf("stream import failed: %w", err)
	}

	return totalEvents, totalChunks, nil
}

func repoRootFromTestFile(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test file path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
}
