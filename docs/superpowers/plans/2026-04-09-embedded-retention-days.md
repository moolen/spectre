# Embedded Retention Days Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `--embedded-retention-days` so embedded raw event storage and checkpoint/projection history follow the same time-based retention window, with `0` disabling retention.

**Architecture:** Thread a single retention-days config through Cobra, runtime config, and Helm. In `internal/embeddedstore`, compute a retention cutoff from wall clock time, reconcile on-disk segments/checkpoints/tail at startup, prune hot/projection state after checkpoints, and replace the hardcoded 24 hour history window with the configured retention window.

**Tech Stack:** Go, Cobra, Helm chart templates/tests, embeddedstore engine/query/projection code, `go test`, helm-unittest

---

## Spec Reference

- Spec: `/home/moritz/dev/spectre-via-ssh/docs/superpowers/specs/2026-04-09-embedded-retention-days-design.md`
- Implement with `@superpowers:test-driven-development`
- Before claiming success, use `@superpowers:verification-before-completion`

### Task 1: Add Retention Config Plumbing

**Files:**
- Modify: `internal/embeddedstore/config.go`
- Modify: `internal/embeddedstore/backend.go`
- Modify: `cmd/spectre/commands/server.go`
- Modify: `cmd/spectre/commands/server_runtime_embedded.go`
- Modify: `cmd/spectre/commands/server_embedded_config_test.go`

- [ ] **Step 1: Write the failing config tests**
- [ ] **Step 2: Run the focused config tests to verify they fail**
- [ ] **Step 3: Add `EmbeddedRetentionDays` validation/default plumbing and server flag wiring**
- [ ] **Step 4: Re-run the focused config tests to verify they pass**

### Task 2: Add Embedded Retention Enforcement

**Files:**
- Create: `internal/embeddedstore/retention.go`
- Modify: `internal/embeddedstore/engine.go`
- Modify: `internal/embeddedstore/engine_persistence.go`
- Modify: `internal/embeddedstore/engine_state.go`
- Modify: `internal/embeddedstore/compaction.go`
- Modify: `internal/embeddedstore/projection_snapshot.go`
- Modify: `internal/embeddedstore/projection_build.go`
- Modify: `internal/embeddedstore/analysis_store.go`
- Modify: `internal/embeddedstore/analysis_store_namespace_graph.go`
- Modify: `internal/embeddedstore/query_executor.go`
- Modify: `internal/embeddedstore/query_executor_events.go`
- Modify: `internal/embeddedstore/compaction_test.go`
- Modify: `internal/embeddedstore/engine_test.go`
- Modify: `internal/embeddedstore/projection_compact_checkpoint_test.go`
- Modify: `internal/embeddedstore/backend_test.go`

- [ ] **Step 1: Write failing tests for disabled retention, raw-segment pruning, mixed-segment rewriting, startup reconciliation, and retained history semantics**
- [ ] **Step 2: Run the focused embeddedstore tests to verify they fail**
- [ ] **Step 3: Implement startup/checkpoint retention and configurable history windows**
- [ ] **Step 4: Re-run the focused embeddedstore tests to verify they pass**

### Task 3: Expose Retention In Helm

**Files:**
- Modify: `chart/values.yaml`
- Modify: `chart/templates/deployment.yaml`
- Modify: `chart/tests/deployment_embedded_test.yaml`

- [ ] **Step 1: Write the failing Helm unittest expectation**
- [ ] **Step 2: Run the focused Helm test to verify it fails**
- [ ] **Step 3: Add the chart value and deployment arg wiring**
- [ ] **Step 4: Re-run the focused Helm test to verify it passes**

### Task 4: Verification

**Files:**
- Test: `internal/embeddedstore/compaction_test.go`
- Test: `internal/embeddedstore/engine_test.go`
- Test: `internal/embeddedstore/projection_compact_checkpoint_test.go`
- Test: `cmd/spectre/commands/server_embedded_config_test.go`
- Test: `chart/tests/deployment_embedded_test.yaml`

- [ ] **Step 1: Run `go test ./internal/embeddedstore ./cmd/spectre/commands -count=1`**
- [ ] **Step 2: Run `helm unittest ./chart -f tests/deployment_embedded_test.yaml`**
- [ ] **Step 3: Summarize expected disk impact and operator semantics for the new flag**
