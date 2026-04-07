# Embedded Recent Resource Window Fast Path Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the remaining `4h` resource-side `/v1/timeline` latency by avoiding full historical `orderedResources` walks for recent moving windows while preserving exact pagination semantics.

**Architecture:** For resource windows whose `start` is inside the compact projection lookback horizon, build the page from two exact inputs:

- the current active resource set in lexical order
- the set of UIDs that changed since `start`

Resources unchanged since `start` can be served directly as pre-existing rows from the current active set. Resources that changed since `start` are handled separately so creates, deletes, and recently updated resources preserve exact behavior. Older windows keep the existing full-scan fallback.

**Tech Stack:** Go, `internal/embeddedstore` projection/query executor code, existing parity/metrics tests, homelab deploy verification

---

## Spec Reference

- Prior design context: `/home/moritz/dev/spectre-via-ssh/.worktrees/checkpoint-compression/docs/superpowers/specs/2026-04-05-embedded-near-instant-startup-design.md`
- Prior optimization slice: `/home/moritz/dev/spectre-via-ssh/.worktrees/checkpoint-compression/docs/superpowers/plans/2026-04-07-embedded-resource-query-projection-fast-path.md`
- Implement with `@superpowers:test-driven-development`
- Before claiming success, use `@superpowers:verification-before-completion`

## File Map

**Modify:**

- `internal/embeddedstore/projection.go`
  - Add bounded recent-resource change indexing and a current active ordered resource view.
- `internal/embeddedstore/projection_build.go`
  - Maintain the new indexes during build/apply/replay.
- `internal/embeddedstore/projection_snapshot.go`
  - Rebuild derived indexes when restoring compact checkpoints.
- `internal/embeddedstore/projection_query.go`
  - Add recent-window candidate helpers for changed UIDs and pre-existing active resources.
- `internal/embeddedstore/query_executor_resources.go`
  - Route recent resource timeline pagination through the new fast path, with exact fallback for older windows.
- `internal/embeddedstore/query_executor_resource_parity_test.go`
  - Prove recent-window pagination still matches planner semantics for creates, deletes, and updates.

## Task 1: Prove The Recent-Window Helper Requirements

**Files:**
- Modify: `internal/embeddedstore/query_executor_resource_parity_test.go`
- Test: `internal/embeddedstore/query_executor_resource_parity_test.go`

- [ ] **Step 1: Add a failing test for recent-window exactness**

Add a focused test that expects the recent-window resource path to:

- include unchanged currently-active resources as pre-existing rows
- include deleted resources that were active at window start
- include resources created inside the window
- exclude resources deleted before the window

- [ ] **Step 2: Run the focused parity test to verify it fails**

Run: `go test ./internal/embeddedstore -run 'TestQueryExecutor_ResourceTimelineRecentWindowParity' -count=1`
Expected: FAIL before the helper exists.

## Task 2: Implement The Recent-Window Fast Path

**Files:**
- Modify: `internal/embeddedstore/projection.go`
- Modify: `internal/embeddedstore/projection_build.go`
- Modify: `internal/embeddedstore/projection_snapshot.go`
- Modify: `internal/embeddedstore/projection_query.go`
- Modify: `internal/embeddedstore/query_executor_resources.go`
- Test: `internal/embeddedstore/query_executor_resource_parity_test.go`

- [ ] **Step 1: Add bounded projection indexes**

Track:

- current active resources in lexical order
- recent non-Event resource changes in timestamp order, pruned to the compact lookback horizon

- [ ] **Step 2: Add a recent-window candidate helper**

For windows whose `start` is within the retained recent horizon:

- gather UIDs changed since `start`
- build exact delta candidates from those UIDs
- merge them with unchanged current active resources in lexical order
- preserve cursor semantics and duplicate-name pagination behavior

- [ ] **Step 3: Re-run the focused recent-window parity test**

Run: `go test ./internal/embeddedstore -run 'TestQueryExecutor_ResourceTimeline(PlannerParity|RecentWindowParity)' -count=1`
Expected: PASS.

## Task 3: Verify And Measure

- [ ] **Step 1: Run the local verification suite**

Run: `go test ./internal/embeddedstore/... ./cmd/spectre/commands -count=1`
Expected: PASS.

- [ ] **Step 2: Build, push, deploy, and remeasure on homelab**

Measure:

- `1h` `/v1/timeline`
- `4h` `/v1/timeline`
- steady-state pod memory

- [ ] **Step 3: Review the next bottleneck**

Use homelab logs and timings to decide whether the next dominant cost is:

- attached Kubernetes Event lookup
- response serialization
- startup repair fallback on the active tail
