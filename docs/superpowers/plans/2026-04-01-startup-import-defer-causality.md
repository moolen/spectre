# Startup Import Defer Causality Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an explicit startup-import flag that disables causality inference only for startup-import batch processing so the timeline becomes available sooner.

**Architecture:** Use a per-batch context override in `internal/graph/sync` rather than introducing a second pipeline instance. Wire the new CLI flag in `cmd/spectre/commands/server.go` so startup import derives an override context, while all normal pipeline users keep the existing defaults.

**Tech Stack:** Go, Cobra, existing graph sync pipeline, existing startup import runner, Go unit tests

---

## Spec Reference

- Spec: `/home/moritz/dev/spectre-via-ssh/.worktrees/startup-import-streaming/docs/superpowers/specs/2026-04-01-startup-import-defer-causality-design.md`
- Implement with `@superpowers:test-driven-development`
- Before claiming success, use `@superpowers:verification-before-completion`

## File Map

**Create:**

- `internal/graph/sync/batch_options.go`
  - Context-scoped batch processing overrides for startup-import-specific tuning

**Modify:**

- `cmd/spectre/commands/server.go`
  - Add the explicit startup-import flag and derive an import-only context override
- `tests/unit/graph/sync/pipeline_test.go`
  - Add a unit test that proves causality is skipped only when the override context is present
- `cmd/spectre/commands/server_import_test.go`
  - Add a unit test that proves the CLI flag exists
- `internal/graph/sync/pipeline.go`
  - Respect the per-batch context override when deciding whether to run causality

## Task 1: Add The Failing Tests

**Files:**

- Modify: `tests/unit/graph/sync/pipeline_test.go`
- Modify: `cmd/spectre/commands/server_import_test.go`

- [ ] **Step 1: Write the failing pipeline override test**
- [ ] **Step 2: Run `go test ./tests/unit/graph/sync -run TestProcessBatch_ContextDisableCausalityOverride -count=1` and verify it fails**
- [ ] **Step 3: Write the failing CLI flag presence test**
- [ ] **Step 4: Run `go test ./cmd/spectre/commands -run TestServerCommandDefinesStartupImportDisableCausalityFlag -count=1` and verify it fails**

## Task 2: Implement The Context Override

**Files:**

- Create: `internal/graph/sync/batch_options.go`
- Modify: `internal/graph/sync/pipeline.go`

- [ ] **Step 1: Add a small exported context helper for batch processing overrides**
- [ ] **Step 2: Keep default behavior unchanged when no override is present**
- [ ] **Step 3: Use the override when deciding whether to run causality in `ProcessBatch`**
- [ ] **Step 4: Run `go test ./tests/unit/graph/sync -run TestProcessBatch_ContextDisableCausalityOverride -count=1` and verify it passes**

## Task 3: Wire The Explicit Startup-Import Flag

**Files:**

- Modify: `cmd/spectre/commands/server.go`
- Modify: `cmd/spectre/commands/server_import_test.go`

- [ ] **Step 1: Add `--startup-import-disable-causality` to `spectre server`**
- [ ] **Step 2: When the flag is set, derive the startup import context with the causality override**
- [ ] **Step 3: Leave watcher and all non-import processing on the default pipeline behavior**
- [ ] **Step 4: Run `go test ./cmd/spectre/commands -run 'TestRunStartupImport|TestServerCommandDefinesStartupImportDisableCausalityFlag' -count=1` and verify it passes**

## Task 4: Verify The Focused Change

**Files:**

- Modify: none

- [ ] **Step 1: Run `go test ./tests/unit/graph/sync ./cmd/spectre/commands -run 'Test(ProcessBatch_ContextDisableCausalityOverride|RunStartupImport|ServerCommandDefinesStartupImportDisableCausalityFlag)' -count=1`**
- [ ] **Step 2: Run `go test ./internal/importexport ./cmd/spectre/commands ./tests/integration/graph -run 'Test(ImportInChunks|RunStartupImport|StartupImport)' -count=1`**
- [ ] **Step 3: If the graph integration test environment is available, compare startup-import logs with and without the new flag**
