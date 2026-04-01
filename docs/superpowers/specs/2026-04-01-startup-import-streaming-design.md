# Startup Import Streaming Design

**Date:** 2026-04-01

**Status:** Approved in design conversation, pending written spec review

## Goal

Improve the existing `spectre server --import-path` startup import path for large historical imports by:

- generating realistic skewed test data on demand
- measuring the real startup import flow against a real FalkorDB instance
- changing import from full in-memory load to streamed chunk processing
- adding instrumentation so follow-up tuning is grounded in measured bottlenecks

This phase intentionally preserves current graph semantics by continuing to use the existing graph pipeline for each chunk. Import-mode tuning that defers or disables expensive work is explicitly deferred to a follow-up step and will be gated behind a flag.

## Problem Statement

The current startup import flow in [cmd/spectre/commands/server.go](/home/moritz/dev/spectre-via-ssh/cmd/spectre/commands/server.go) does the following:

1. Reads the entire import source into a single `[]models.Event` via `internal/importexport`
2. Logs parse duration
3. Calls `graphPipeline.ProcessBatch(...)` once for the full event set

For the target dataset shape, this creates two immediate issues:

- peak memory scales with the full import size rather than a controlled chunk size
- import timing is opaque beyond coarse parse/process durations, making bottlenecks hard to attribute

The graph pipeline already performs internal sub-batching for node and edge writes, but the startup importer still feeds it one very large batch. That makes startup imports harder to tune and reason about than necessary.

## Scope

### In Scope

- Add an on-demand dataset generator for the transformed Spectre import format
- Produce a dataset with approximately:
  - 55 kinds
  - 5k resources
  - heavily skewed resource distribution by kind
  - heavily skewed event distribution by kind
  - roughly the approved density profile and Pareto concentration
- Add a reproducible startup-flow benchmark path against real FalkorDB
- Change startup import to stream events in configurable chunks
- Add structured timing and throughput logs for startup import
- Add tests for chunked import behavior and generator summaries

### Out of Scope

- Changing the transformed file format
- Replacing the graph pipeline with a separate bulk-ingest implementation
- Enabling import-mode behavior changes by default
- Dropping expensive graph work in this first pass

## Existing Entry Points

### Startup import

- [cmd/spectre/commands/server.go](/home/moritz/dev/spectre-via-ssh/cmd/spectre/commands/server.go)
- Current flow uses `importexport.Import(importexport.FromPath(importPath), ...)`
- Current processing uses `graphPipeline.ProcessBatch(importCtx, eventValues)`

### Import parsing

- [internal/importexport/json_import.go](/home/moritz/dev/spectre-via-ssh/internal/importexport/json_import.go)
- [internal/importexport/fileio/fileio.go](/home/moritz/dev/spectre-via-ssh/internal/importexport/fileio/fileio.go)

### Graph batch processing

- [internal/graph/sync/pipeline.go](/home/moritz/dev/spectre-via-ssh/internal/graph/sync/pipeline.go)
- The pipeline already uses two-phase processing and internal sub-batching with `maxBatchSize = 1000`

## Proposed Architecture

### 1. Streaming import source

Extend `internal/importexport` with a chunk-oriented API for startup imports.

The current `Import(...) ([]models.Event, error)` API stays for existing call sites and tests. A new streaming API is added alongside it for the startup import path. Conceptually:

- open the import source
- iterate files in deterministic order
- parse events from each file
- accumulate up to `import-chunk-size`
- validate and enrich the chunk
- hand the chunk to a callback or iterator consumer
- continue until EOF

This preserves compatibility while allowing the startup path to avoid loading the entire dataset into memory.

### 2. Startup import loop in server command

Update [cmd/spectre/commands/server.go](/home/moritz/dev/spectre-via-ssh/cmd/spectre/commands/server.go) to:

- start the application as today so the graph pipeline is initialized
- if `--import-path` is set, create an import runner for startup mode
- consume event chunks from the streaming importer
- call `graphPipeline.ProcessBatch(...)` once per chunk
- accumulate totals and per-chunk timings
- log a final import summary with throughput and timing breakdown

This keeps import orchestration in the CLI layer and avoids coupling the graph pipeline directly to filesystem traversal.

### 3. Dataset generator

Add a generator command or utility that writes transformed import data to a user-specified output directory outside git.

Requirements:

- deterministic given a seed
- generate realistic skew rather than uniform distributions
- emit a machine-readable summary file with realized counts
- keep output format compatible with the current Spectre importer

The generator is part of the benchmark workflow, not a checked-in fixture source.

### 4. Benchmark harness

Add a reproducible benchmark script or test-oriented utility that:

- starts or expects a local FalkorDB instance
- generates the dataset into a temp/output location
- runs `bin/spectre server --import-path ...`
- captures startup/import timing
- emits a concise summary suitable for before/after comparisons

The benchmark should measure the real startup path, not just in-process parsing or direct pipeline calls.

## Flags And Configuration

### New startup import flags

- `--import-chunk-size`
  - number of events per call to `ProcessBatch`
  - explicit tuning knob for startup import behavior
- `--import-benchmark-log` or equivalent structured output option
  - optional path or mode for machine-readable timing results
- `--import-mode`
  - reserved for follow-up optimization work
  - default remains disabled in this phase

### Behavior

- Without `--import-path`, nothing changes
- With `--import-path` and no new flags, a safe default chunk size is used
- Any future import-mode optimization is opt-in only

## Observability

Each startup import run should log:

- total events imported
- chunk size
- number of chunks processed
- total parse/read duration
- total graph processing duration
- total startup import duration
- effective events/sec
- optional pipeline counters if cheap to expose

Each chunk log should include:

- chunk index
- events in chunk
- cumulative events
- chunk parse duration
- chunk process duration

The final summary should be concise enough for local benchmarking and CI log inspection.

## Data Generator Shape

The generator should target the approved workload shape:

- 55 kinds
- 5k resources
- skewed resources-per-kind:
  - minimum near 1
  - median around 25
  - p90 around 212
  - maximum around 1267
- skewed events-per-kind:
  - minimum near 4
  - median around 187
  - p90 around 3k
  - maximum around 25609
- events per resource:
  - broad range
  - median roughly 7.8
  - mean roughly 19.6
- Pareto event concentration:
  - top 1 kind about 31%
  - top 3 kinds about 53%
  - top 10 kinds about 79%

The generator does not need exact mathematical fidelity for every run, but it must emit a summary showing the realized shape so benchmark results remain interpretable.

## Implementation Boundaries

### `internal/importexport`

Responsibilities:

- directory traversal
- per-file parsing
- chunk assembly
- validation and enrichment of chunk contents

Non-responsibilities:

- graph writes
- startup orchestration
- benchmark reporting

### `cmd/spectre/commands/server.go`

Responsibilities:

- startup import control flow
- chunk-to-pipeline handoff
- timing and final reporting
- flag handling

Non-responsibilities:

- detailed file parsing rules
- graph write semantics

### `internal/graph/sync`

Responsibilities in this phase:

- unchanged graph semantics
- existing `ProcessBatch` behavior reused per chunk
- expose any cheap metrics needed for timing attribution

Future follow-up:

- optional import-mode pipeline tuning behind a flag

## Risks And Mitigations

### Risk: chunk boundaries change behavior

Some graph semantics may depend on seeing related events within the same `ProcessBatch`.

Mitigation:

- keep chunk size reasonably large by default
- test chunked imports against small realistic scenarios
- instrument chunk counts and timings so tuning can be evidence-based

### Risk: importer API proliferation

Adding a second import API can make the package harder to follow.

Mitigation:

- keep the streaming API narrow and startup-oriented
- retain the current `Import(...)` API as a compatibility wrapper
- share validation and enrichment logic internally

### Risk: benchmark noise

Local database startup, machine load, and cache state may distort results.

Mitigation:

- keep generator deterministic by seed
- document the required benchmark environment
- emit realized dataset summary alongside timings
- prefer repeated runs for comparison

### Risk: future import-mode tuning becomes ad hoc

Mitigation:

- reserve an explicit flag now
- route future tuning through a dedicated import-mode config path rather than scattered conditionals

## Testing Strategy

### Unit tests

- chunk iterator/consumer behavior
- end-of-stream flushing
- multi-file traversal ordering
- generator summary validation

### Integration tests

- startup import with a small generated dataset against FalkorDB
- verify chunked processing completes successfully
- verify final totals and chunk logs are emitted

### Benchmark workflow

- generate shaped dataset on demand
- run real `spectre server --import-path ...` startup flow
- compare timing before and after optimizations

## Rollout Plan

### Phase 1

- add generator
- add benchmark harness
- add chunked import path
- add timing/throughput logs

### Phase 2

- inspect measured bottlenecks
- discuss import-mode pipeline tuning
- add gated flag-based deferrals or dropped work only where justified

## Open Decisions Deferred To Phase 2

- which expensive pipeline steps can be safely deferred or disabled during startup import
- whether import-mode should adjust causality, relationship extraction, or other graph work
- whether import-mode needs a separate default chunk size

## Review Notes

The normal brainstorming workflow calls for a spec-review subagent after writing the document. That review step was not dispatched here because agent delegation was not explicitly requested in this session. A manual review pass was used instead.
