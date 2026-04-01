# Startup Import Timeline Only Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an explicit startup-import mode that preserves timeline data while deferring semantic relationship extraction and causality.

**Architecture:** Reuse the existing per-batch context override in `internal/graph/sync` instead of creating a second pipeline. The new startup-import flag will set a `TimelineOnly` override so `ProcessBatch` keeps Phase 1 structural writes but skips Phase 2 semantic relationship extraction and Phase 3 causality.

**Tech Stack:** Go, Cobra, existing graph sync pipeline, existing startup import runner, Go unit and integration tests

---

## Spec Reference

- Spec: `/home/moritz/dev/spectre-via-ssh/.worktrees/startup-import-streaming/docs/superpowers/specs/2026-04-01-startup-import-timeline-only-design.md`
- Implement with `@superpowers:test-driven-development`
- Before claiming success, use `@superpowers:verification-before-completion`

## File Map

**Modify:**

- `internal/graph/sync/batch_options.go`
  - Extend the batch override with `TimelineOnly`
- `internal/graph/sync/pipeline.go`
  - Skip semantic relationship extraction and causality when timeline-only is set
- `cmd/spectre/commands/server.go`
  - Add the explicit CLI flag and derive the startup-import override context
- `tests/unit/graph/sync/pipeline_test.go`
  - Prove structural edges remain while semantic edges and causality are skipped
- `cmd/spectre/commands/server_import_test.go`
  - Prove the CLI flag exists

## Task 1: Add Failing Tests

**Files:**

- Modify: `tests/unit/graph/sync/pipeline_test.go`
- Modify: `cmd/spectre/commands/server_import_test.go`

- [ ] **Step 1: Write a failing pipeline test for timeline-only overrides**
- [ ] **Step 2: Run `go test ./tests/unit/graph/sync -run TestProcessBatch_TimelineOnlyOverrideSkipsSemanticRelationships -count=1` and verify it fails**
- [ ] **Step 3: Write a failing CLI flag presence test**
- [ ] **Step 4: Run `go test ./cmd/spectre/commands -run TestServerCommandDefinesStartupImportTimelineOnlyFlag -count=1` and verify it fails**

## Task 2: Implement Timeline-Only Overrides

**Files:**

- Modify: `internal/graph/sync/batch_options.go`
- Modify: `internal/graph/sync/pipeline.go`

- [ ] **Step 1: Extend batch processing overrides with `TimelineOnly`**
- [ ] **Step 2: Skip `BuildRelationshipEdges` when timeline-only is set**
- [ ] **Step 3: Make timeline-only also skip causality**
- [ ] **Step 4: Run `go test ./tests/unit/graph/sync -run 'TestProcessBatch_(ContextDisableCausalityOverride|TimelineOnlyOverrideSkipsSemanticRelationships)' -count=1` and verify it passes**

## Task 3: Wire The Explicit CLI Flag

**Files:**

- Modify: `cmd/spectre/commands/server.go`
- Modify: `cmd/spectre/commands/server_import_test.go`

- [ ] **Step 1: Add `--startup-import-timeline-only`**
- [ ] **Step 2: Apply the timeline-only batch override only to startup import**
- [ ] **Step 3: Keep live ingestion unchanged**
- [ ] **Step 4: Run `go test ./cmd/spectre/commands -run 'Test(ServerCommandDefinesStartupImportDisableCausalityFlag|ServerCommandDefinesStartupImportTimelineOnlyFlag|RunStartupImport)' -count=1` and verify it passes**

## Task 4: Verify The Focused Change

**Files:**

- Modify: none

- [ ] **Step 1: Run `go test ./internal/importexport ./cmd/spectre/commands ./tests/integration/graph ./tests/unit/graph/sync -run 'Test(ImportInChunks|RunStartupImport|StartupImport|ProcessBatch_ContextDisableCausalityOverride|ProcessBatch_TimelineOnlyOverrideSkipsSemanticRelationships|ServerCommandDefinesStartupImportDisableCausalityFlag|ServerCommandDefinesStartupImportTimelineOnlyFlag)' -count=1`**
- [ ] **Step 2: Compare startup-import behavior with `--startup-import-timeline-only` on a modest dataset if the local graph environment is available**
