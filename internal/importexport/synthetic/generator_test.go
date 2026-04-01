package synthetic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/moolen/spectre/internal/importexport"
	"github.com/moolen/spectre/internal/logging"
)

func TestGenerateDatasetSummaryDeterministicAcrossOutputDirs(t *testing.T) {
	t.Parallel()

	config := Config{
		Seed:           42,
		KindCount:      7,
		ResourceCount:  30,
		NamespaceCount: 3,
	}

	outputDirOne := filepath.Join(t.TempDir(), "out-one")
	summaryOne, err := GenerateDataset(outputDirOne, config)
	if err != nil {
		t.Fatalf("GenerateDataset() returned error: %v", err)
	}

	outputDirTwo := filepath.Join(t.TempDir(), "out-two")
	summaryTwo, err := GenerateDataset(outputDirTwo, config)
	if err != nil {
		t.Fatalf("GenerateDataset() returned error: %v", err)
	}

	if !reflect.DeepEqual(summaryOne, summaryTwo) {
		t.Fatalf("summary mismatch for same seed/config across output dirs\nsummaryOne=%+v\nsummaryTwo=%+v", summaryOne, summaryTwo)
	}
}

func TestGenerateDatasetWritesSummaryAndImporterReadyJSON(t *testing.T) {
	t.Parallel()

	outputDir := t.TempDir()
	config := Config{
		Seed:           7,
		KindCount:      6,
		ResourceCount:  24,
		NamespaceCount: 2,
	}

	summary, err := GenerateDataset(outputDir, config)
	if err != nil {
		t.Fatalf("GenerateDataset() returned error: %v", err)
	}

	summaryPath := filepath.Join(outputDir, "summary.json")
	summaryBytes, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("expected summary file at %q: %v", summaryPath, err)
	}

	var summaryFromDisk Summary
	if err := json.Unmarshal(summaryBytes, &summaryFromDisk); err != nil {
		t.Fatalf("failed to decode summary.json: %v", err)
	}

	if !reflect.DeepEqual(summary, summaryFromDisk) {
		t.Fatalf("summary mismatch between return value and summary.json\nreturned=%+v\non_disk=%+v", summary, summaryFromDisk)
	}

	var files []string
	err = filepath.WalkDir(filepath.Join(outputDir, "events"), func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() && filepath.Ext(path) == ".json" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk generated json files: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("expected at least one importer-ready JSON file under events/")
	}

	logger := logging.GetLogger("synthetic-generator-test")
	events, err := importexport.Import(importexport.FromDirectory(filepath.Join(outputDir, "events")), importexport.WithLogger(logger))
	if err != nil {
		t.Fatalf("generated files are not importer-compatible: %v", err)
	}
	if len(events) == 0 {
		t.Fatalf("expected importer to load at least one event")
	}
}

func TestGenerateDatasetSummaryHasNonZeroTotalsAndStableTopKindOrdering(t *testing.T) {
	t.Parallel()

	outputDir := t.TempDir()
	config := Config{
		Seed:           42,
		KindCount:      8,
		ResourceCount:  40,
		NamespaceCount: 2,
	}

	summary, err := GenerateDataset(outputDir, config)
	if err != nil {
		t.Fatalf("GenerateDataset() returned error: %v", err)
	}

	if summary.TotalKinds == 0 || summary.TotalResources == 0 || summary.TotalEvents == 0 {
		t.Fatalf("expected non-zero totals, got %+v", summary)
	}

	if len(summary.TopKindsByEvents) < 3 {
		t.Fatalf("expected at least 3 top kinds, got %d", len(summary.TopKindsByEvents))
	}

	expectedTop := []string{"Kind-008", "Kind-006", "Kind-002"}
	if !reflect.DeepEqual(summary.TopKindsByEvents[:3], expectedTop) {
		t.Fatalf("unexpected top kind ordering, got %v want %v", summary.TopKindsByEvents[:3], expectedTop)
	}
}
