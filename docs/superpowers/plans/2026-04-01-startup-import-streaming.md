# Startup Import Streaming Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a streamed startup import path for `spectre server --import-path`, add a synthetic skewed dataset generator, and provide a repeatable benchmark against a real FalkorDB instance.

**Architecture:** Keep the current graph semantics by continuing to use `sync.Pipeline.ProcessBatch(...)`, but change the startup importer to feed the pipeline fixed-size chunks instead of one giant in-memory slice. Make the streaming parser the primary internal import primitive, then layer a dataset generator and a benchmark harness on top so throughput tuning is driven by measured timings.

**Tech Stack:** Go, Cobra, FalkorDB, testcontainers-go, existing `internal/importexport`, existing graph sync pipeline, shell benchmark script

---

## Spec Reference

- Spec: `/home/moritz/dev/spectre-via-ssh/docs/superpowers/specs/2026-04-01-startup-import-streaming-design.md`
- Implement with `@superpowers:test-driven-development`
- Before claiming success, use `@superpowers:verification-before-completion`

## File Map

**Create:**

- `internal/importexport/stream_import.go`
  - Streaming import API for chunked parsing and per-chunk callbacks
- `internal/importexport/stream_import_test.go`
  - Unit tests for chunk flushing, multi-file traversal, validation/enrichment behavior
- `cmd/spectre/commands/server_import.go`
  - Startup import runner, timing aggregation, benchmark report writing
- `cmd/spectre/commands/server_import_test.go`
  - Unit tests with a fake pipeline to verify chunk loop, totals, and report output
- `internal/importexport/synthetic/generator.go`
  - Synthetic dataset generator and summary emission
- `internal/importexport/synthetic/generator_test.go`
  - Distribution sanity tests and determinism-by-seed checks
- `cmd/spectre/commands/debug_generate_import_data.go`
  - `spectre debug generate-import-data` command that writes synthetic importer-ready data
- `tests/integration/graph/startup_import_stream_test.go`
  - Real FalkorDB integration test that streams generated data into the pipeline in chunks
- `hack/benchmark-startup-import.sh`
  - End-to-end benchmark harness for local startup import timing

**Modify:**

- `internal/importexport/json_import.go`
  - Refactor existing `Import(...)` to reuse the new streaming parser internally
- `internal/importexport/fileio/fileio.go`
  - Expose a deterministic iterator or document-and-test lexical traversal assumptions
- `cmd/spectre/commands/server.go`
  - Add `--import-chunk-size`, `--import-benchmark-log`, `--import-mode`; swap current one-shot import for the startup import runner
- `cmd/spectre/commands/debug.go`
  - Register the new debug generator subcommand
- `Makefile`
  - Add a convenience target for the startup import benchmark

## Task 1: Build The Streaming Import Primitive

**Files:**

- Create: `internal/importexport/stream_import.go`
- Create: `internal/importexport/stream_import_test.go`
- Modify: `internal/importexport/json_import.go`
- Modify: `internal/importexport/fileio/fileio.go`
- Test: `internal/importexport/stream_import_test.go`
- Test: `internal/importexport/json_import_test.go`

- [ ] **Step 1: Write the failing chunked import tests**

```go
func TestImportInChunks_FlushesFinalPartialChunk(t *testing.T) {
	logger := logging.GetLogger("test")
	tmpDir := t.TempDir()
	writeEventFile(t, filepath.Join(tmpDir, "a.json"), 3)
	writeEventFile(t, filepath.Join(tmpDir, "b.json"), 2)

	var chunkSizes []int
	err := ImportInChunks(
		FromPath(tmpDir),
		2,
		func(events []models.Event) error {
			chunkSizes = append(chunkSizes, len(events))
			return nil
		},
		WithLogger(logger),
	)

	require.NoError(t, err)
	require.Equal(t, []int{2, 2, 1}, chunkSizes)
}
```

- [ ] **Step 2: Run the importexport tests to verify failure**

Run: `go test ./internal/importexport -run 'TestImportInChunks|TestParseJSONEvents|TestImportFromPath' -count=1`
Expected: FAIL with missing `ImportInChunks` symbols and/or unchanged one-shot behavior

- [ ] **Step 3: Implement the streaming parser and chunk callback API**

```go
type ChunkHandler func([]models.Event) error

func ImportInChunks(source ImportSource, chunkSize int, handler ChunkHandler, opts ...ImportOption) error {
	options := &ImportOptions{logger: logging.GetLogger("importexport")}
	for _, opt := range opts {
		opt(options)
	}
	return streamSource(source, chunkSize, handler, options.logger)
}
```

```go
func parseJSONEvents(r io.Reader, logger *logging.Logger) ([]models.Event, error) {
	var all []models.Event
	err := streamJSONEvents(r, defaultCollectChunkSize, func(events []models.Event) error {
		all = append(all, events...)
		return nil
	}, logger)
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("events array is empty")
	}
	return all, nil
}
```

- [ ] **Step 4: Make directory traversal deterministic and share validation/enrichment**

```go
func (w *DirectoryWalker) WalkJSON(dirPath string) ([]WalkResult, error) {
	// existing filepath.Walk collection
	sort.Slice(results, func(i, j int) bool {
		return results[i].FilePath < results[j].FilePath
	})
	return results, nil
}
```

- [ ] **Step 5: Run the focused importexport test suite**

Run: `go test ./internal/importexport -run 'TestImportInChunks|TestParseJSONEvents|TestImportFrom(File|Directory|Path)' -count=1`
Expected: PASS

- [ ] **Step 6: Commit the streaming import primitive**

```bash
git add internal/importexport/stream_import.go \
        internal/importexport/stream_import_test.go \
        internal/importexport/json_import.go \
        internal/importexport/fileio/fileio.go
git commit -m "feat(import): add chunked streaming import API"
```

## Task 2: Wire Streamed Startup Import Into `spectre server`

**Files:**

- Create: `cmd/spectre/commands/server_import.go`
- Create: `cmd/spectre/commands/server_import_test.go`
- Modify: `cmd/spectre/commands/server.go`
- Test: `cmd/spectre/commands/server_import_test.go`

- [ ] **Step 1: Write the failing startup import runner tests**

```go
func TestRunStartupImport_ProcessesAllChunksAndWritesReport(t *testing.T) {
	tmpDir := t.TempDir()
	reportPath := filepath.Join(tmpDir, "import-report.json")
	pipeline := &fakePipeline{}

	err := runStartupImport(t.Context(), startupImportOptions{
		Path:             "testdata",
		ChunkSize:        3,
		BenchmarkLogPath: reportPath,
		Pipeline:         pipeline,
		Logger:           logging.GetLogger("test"),
		Stream: func(_ string, chunkSize int, handle func([]models.Event) error) error {
			require.Equal(t, 3, chunkSize)
			require.NoError(t, handle(makeEvents(3)))
			require.NoError(t, handle(makeEvents(2)))
			return nil
		},
	})

	require.NoError(t, err)
	require.Equal(t, []int{3, 2}, pipeline.batchSizes)
	require.FileExists(t, reportPath)
}
```

- [ ] **Step 2: Run the command tests to verify failure**

Run: `go test ./cmd/spectre/commands -run 'TestRunStartupImport' -count=1`
Expected: FAIL with missing startup import runner and flags

- [ ] **Step 3: Implement a dedicated startup import runner**

```go
type startupImportOptions struct {
	Path             string
	ChunkSize        int
	BenchmarkLogPath string
	ImportMode       bool
	Pipeline         sync.Pipeline
	Logger           *logging.Logger
	Stream           func(string, int, func([]models.Event) error) error
}

func runStartupImport(ctx context.Context, opts startupImportOptions) error {
	start := time.Now()
	var totalEvents, chunkCount int
	var parseDuration, processDuration time.Duration

	err := opts.Stream(opts.Path, opts.ChunkSize, func(events []models.Event) error {
		chunkCount++
		totalEvents += len(events)
		processStart := time.Now()
		if err := opts.Pipeline.ProcessBatch(ctx, events); err != nil {
			return err
		}
		processDuration += time.Since(processStart)
		return nil
	})
	if err != nil {
		return err
	}

	return writeStartupImportReport(opts.BenchmarkLogPath, startupImportReport{
		TotalEvents:     totalEvents,
		ChunkCount:      chunkCount,
		ChunkSize:       opts.ChunkSize,
		ParseDuration:   parseDuration,
		ProcessDuration: processDuration,
		TotalDuration:   time.Since(start),
	})
}
```

- [ ] **Step 4: Add the new server flags and replace the one-shot import block**

```go
serverCmd.Flags().IntVar(&importChunkSize, "import-chunk-size", 1000, "Number of events to process per startup import chunk")
serverCmd.Flags().StringVar(&importBenchmarkLogPath, "import-benchmark-log", "", "Optional path for machine-readable startup import timing output")
serverCmd.Flags().BoolVar(&importMode, "import-mode", false, "Enable gated import-specific pipeline tuning (disabled by default)")
```

```go
if importPath != "" {
	importCtx, importCancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer importCancel()

	if err := runStartupImport(importCtx, startupImportOptions{
		Path:             importPath,
		ChunkSize:        importChunkSize,
		BenchmarkLogPath: importBenchmarkLogPath,
		ImportMode:       importMode,
		Pipeline:         graphPipeline,
		Logger:           logger,
		Stream: func(path string, chunkSize int, handle func([]models.Event) error) error {
			return importexport.ImportPathInChunks(path, chunkSize, handle, importexport.WithLogger(logger))
		},
	}); err != nil {
		HandleError(err, "Import processing error")
	}
}
```

- [ ] **Step 5: Run the focused command tests**

Run: `go test ./cmd/spectre/commands -run 'TestRunStartupImport' -count=1`
Expected: PASS

- [ ] **Step 6: Commit the startup import runner**

```bash
git add cmd/spectre/commands/server.go \
        cmd/spectre/commands/server_import.go \
        cmd/spectre/commands/server_import_test.go
git commit -m "feat(server): stream startup imports in chunks"
```

## Task 3: Add The Synthetic Dataset Generator

**Files:**

- Create: `internal/importexport/synthetic/generator.go`
- Create: `internal/importexport/synthetic/generator_test.go`
- Create: `cmd/spectre/commands/debug_generate_import_data.go`
- Modify: `cmd/spectre/commands/debug.go`
- Test: `internal/importexport/synthetic/generator_test.go`

- [ ] **Step 1: Write the failing generator tests**

```go
func TestGenerateDatasetSummary_IsDeterministicBySeed(t *testing.T) {
	cfg := Config{
		Seed:           42,
		KindCount:      55,
		ResourceCount:  5000,
		OutputDir:      t.TempDir(),
	}

	summaryA, err := Generate(cfg)
	require.NoError(t, err)

	cfg.OutputDir = t.TempDir()
	summaryB, err := Generate(cfg)
	require.NoError(t, err)

	require.Equal(t, summaryA.TotalResources, summaryB.TotalResources)
	require.Equal(t, summaryA.TotalEvents, summaryB.TotalEvents)
	require.Equal(t, summaryA.TopKindsByEventCount, summaryB.TopKindsByEventCount)
}
```

- [ ] **Step 2: Run the generator tests to verify failure**

Run: `go test ./internal/importexport/synthetic -run 'TestGenerateDatasetSummary' -count=1`
Expected: FAIL with missing package or symbols

- [ ] **Step 3: Implement the generator and summary writer**

```go
type Config struct {
	OutputDir     string
	Seed          int64
	KindCount     int
	ResourceCount int
}

type Summary struct {
	TotalKinds            int                    `json:"total_kinds"`
	TotalResources        int                    `json:"total_resources"`
	TotalEvents           int                    `json:"total_events"`
	ResourcesPerKind      DistributionSummary    `json:"resources_per_kind"`
	EventsPerKind         DistributionSummary    `json:"events_per_kind"`
	EventsPerResource     DistributionSummary    `json:"events_per_resource"`
	TopKindsByEventCount  []KindSummary          `json:"top_kinds_by_event_count"`
}

func Generate(cfg Config) (Summary, error) {
	// build skewed kind weights
	// assign 5k resources across 55 kinds
	// assign event counts with Pareto concentration
	// write one importer-ready .json file per resource:
	//   <output>/<group>/<version>/<kind>/<namespace>/<name>.json
	// plus summary.json
}
```

- [ ] **Step 4: Add a debug command for local generation**

```go
var generateImportDataCmd = &cobra.Command{
	Use:   "generate-import-data",
	Short: "Generate synthetic startup import benchmark data",
	RunE: func(cmd *cobra.Command, args []string) error {
		summary, err := synthetic.Generate(cfg)
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(summary)
	},
}
```

- [ ] **Step 5: Run the focused generator tests**

Run: `go test ./internal/importexport/synthetic -run 'TestGenerateDatasetSummary' -count=1`
Expected: PASS

- [ ] **Step 6: Smoke-test the command manually**

Run: `go run ./cmd/spectre debug generate-import-data --output-dir /tmp/spectre-import-data --seed 42`
Expected: JSON summary on stdout and `/tmp/spectre-import-data/summary.json` plus generated `.json` files

- [ ] **Step 7: Commit the generator**

```bash
git add internal/importexport/synthetic/generator.go \
        internal/importexport/synthetic/generator_test.go \
        cmd/spectre/commands/debug.go \
        cmd/spectre/commands/debug_generate_import_data.go
git commit -m "feat(import): add synthetic startup import data generator"
```

## Task 4: Add Integration Coverage And A Repeatable Benchmark Harness

**Files:**

- Create: `tests/integration/graph/startup_import_stream_test.go`
- Create: `hack/benchmark-startup-import.sh`
- Modify: `Makefile`
- Test: `tests/integration/graph/startup_import_stream_test.go`

- [ ] **Step 1: Write the failing integration test**

```go
func TestStartupImport_StreamedChunksAgainstRealPipeline(t *testing.T) {
	harness, err := NewTestHarness(t)
	require.NoError(t, err)
	defer harness.Cleanup(context.Background())

	outputDir := t.TempDir()
	_, err = synthetic.Generate(synthetic.Config{
		OutputDir:     outputDir,
		Seed:          42,
		KindCount:     6,
		ResourceCount: 40,
	})
	require.NoError(t, err)

	var total int
	err = importexport.ImportPathInChunks(outputDir, 10, func(events []models.Event) error {
		total += len(events)
		return harness.GetPipeline().ProcessBatch(context.Background(), events)
	})
	require.NoError(t, err)
	require.Greater(t, total, 0)
	assert.GreaterOrEqual(t, CountResources(t, harness.GetClient()), 40)
}
```

- [ ] **Step 2: Run the integration test to verify failure**

Run: `go test ./tests/integration/graph -run 'TestStartupImport_StreamedChunksAgainstRealPipeline' -count=1`
Expected: FAIL until the generator and chunked importer are wired together

- [ ] **Step 3: Implement the benchmark script and Make target**

```bash
#!/usr/bin/env bash
set -euo pipefail

OUT_DIR="${OUT_DIR:-/tmp/spectre-startup-import-bench}"
SEED="${SEED:-42}"
GRAPH_NAME="${GRAPH_NAME:-spectre_benchmark}"

make build
make graph-up
go run ./cmd/spectre debug generate-import-data --output-dir "$OUT_DIR/data" --seed "$SEED"
./bin/spectre server \
  --graph-enabled=true \
  --graph-host=localhost \
  --graph-port=6379 \
  --graph-name="$GRAPH_NAME" \
  --watcher-enabled=false \
  --reconciler-enabled=false \
  --import-path "$OUT_DIR/data" \
  --import-chunk-size 1000 \
  --import-benchmark-log "$OUT_DIR/import-report.json"
```

```make
benchmark-startup-import:
	@./hack/benchmark-startup-import.sh
```

- [ ] **Step 4: Run the integration test and the benchmark harness**

Run: `go test ./tests/integration/graph -run 'TestStartupImport_StreamedChunksAgainstRealPipeline' -count=1`
Expected: PASS

Run: `make benchmark-startup-import`
Expected: generated dataset, successful startup import, and a machine-readable report such as:

```json
{
  "total_events": 98000,
  "chunk_size": 1000,
  "chunk_count": 98,
  "parse_duration_ms": 420,
  "process_duration_ms": 18350,
  "total_duration_ms": 18810,
  "events_per_second": 5210.0
}
```

- [ ] **Step 5: Record baseline numbers in the PR or task notes**

Run: `cat /tmp/spectre-startup-import-bench/import-report.json`
Expected: concrete baseline to compare with follow-up `--import-mode` tuning

- [ ] **Step 6: Commit the benchmark harness and integration coverage**

```bash
git add tests/integration/graph/startup_import_stream_test.go \
        hack/benchmark-startup-import.sh \
        Makefile
git commit -m "test(import): add startup import benchmark harness"
```

## Final Verification

- [ ] Run: `go test ./internal/importexport -count=1`
  Expected: PASS
- [ ] Run: `go test ./cmd/spectre/commands -count=1`
  Expected: PASS
- [ ] Run: `go test ./tests/integration/graph -run 'TestStartupImport_StreamedChunksAgainstRealPipeline|TestPerformance_LargeEventBatch' -count=1`
  Expected: PASS
- [ ] Run: `make benchmark-startup-import`
  Expected: successful startup import timing report written to disk
- [ ] Run: `git status --short`
  Expected: only intended tracked changes remain

## Review Notes

- The `@superpowers:writing-plans` workflow calls for a plan-document-reviewer subagent. That review was not dispatched here because agent delegation was not explicitly requested in this session.
- Perform a manual review of this plan before execution:
  - confirm the generator output layout is importer-ready and not dependent on the user's separate transformation CLI
  - confirm `watcher-enabled=false` and `reconciler-enabled=false` are acceptable for the benchmark path
  - confirm `ImportInChunks` becomes the internal primitive reused by the old one-shot API rather than a side path
