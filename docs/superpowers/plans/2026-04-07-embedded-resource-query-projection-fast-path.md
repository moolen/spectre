# Embedded Resource Query Projection Fast Path Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the current cold per-UID lookup bottleneck from embedded resource timeline queries so recent `/v1/timeline` opens are dominated by in-memory work instead of segment reads.

**Architecture:** Keep Event-kind timeline queries on the existing planner/cache path, but answer non-`Event` resource timeline rows directly from compact projection resource versions. The projection already holds ordered per-resource versions needed to build timeline rows and pre-window anchors, so the executor can preserve existing pagination semantics while avoiding planner `planResourceEvents(...)` calls for the resource half of the request.

**Tech Stack:** Go, `internal/embeddedstore` projection/query executor code, Prometheus query metrics, existing embedded parity/unit tests, homelab deploy verification

---

## Spec Reference

- Prior design context: `/home/moritz/dev/spectre-via-ssh/.worktrees/checkpoint-compression/docs/superpowers/specs/2026-04-05-embedded-near-instant-startup-design.md`
- Implement with `@superpowers:test-driven-development`
- Before claiming success, use `@superpowers:verification-before-completion`

## File Map

**Modify:**

- `internal/embeddedstore/projection_query.go`
  - Add a direct resource-version window extractor that preserves pre-existing anchor semantics without reconstructing full event history.
- `internal/embeddedstore/query_executor_resources.go`
  - Route resource timeline pagination through the projection fast path instead of planner per-UID fetches.
- `internal/embeddedstore/query_executor_resource_parity_test.go`
  - Cover duplicate-ID parity and compact-projection behavior on the resource fast path.
- `internal/embeddedstore/query_metrics_test.go`
  - Assert resource timeline queries no longer record cold UID lookups when projection state is sufficient.

## Task 1: Prove Resource Queries Still Hit Cold UID Lookups

**Files:**
- Modify: `internal/embeddedstore/query_metrics_test.go`
- Test: `internal/embeddedstore/query_metrics_test.go`

- [ ] **Step 1: Add a failing metric test for cold-only resource queries**

Add a focused test that queries a flushed resource timeline and asserts zero `spectre_embedded_uid_disk_lookups_total` for `query_family=resource_events`.

- [ ] **Step 2: Run the focused metric test to verify it fails**

Run: `go test ./internal/embeddedstore -run 'TestQueryMetrics_(ResourceEventStoreMixes|ResourcePaginationStopsAfterPageWindow)' -count=1`
Expected: FAIL because resource queries still go through planner cold UID lookups.

## Task 2: Implement Projection-Backed Resource Timeline Extraction

**Files:**
- Modify: `internal/embeddedstore/projection_query.go`
- Modify: `internal/embeddedstore/query_executor_resources.go`
- Modify: `internal/embeddedstore/query_executor_resource_parity_test.go`
- Test: `internal/embeddedstore/query_executor_resource_parity_test.go`

- [ ] **Step 1: Add a failing parity test that depends on duplicate-ID handling surviving the fast path**

Extend resource parity coverage so a resource timeline still dedupes duplicate IDs deterministically when served from projection versions.

- [ ] **Step 2: Run the focused parity test to verify it fails once the new expectation is present**

Run: `go test ./internal/embeddedstore -run 'TestQueryExecutor_ResourceTimelinePlannerParity' -count=1`
Expected: FAIL after the new expectation is added and before the projection fast path is implemented.

- [ ] **Step 3: Implement the projection resource fast path**

Add a helper on `Projection` that:

- walks one `resourceRecord.versions` slice with binary search
- returns the latest pre-window non-delete version as `PreExisting=true`
- returns in-window versions in stable order
- dedupes duplicate event IDs with the existing duplicate preference helper

Update `QueryExecutor.collectPaginatedResources(...)` / `resourceEvents(...)` so non-`Event` resource timeline queries use that helper and report `projectionUsed=true`.

- [ ] **Step 4: Re-run the focused parity and metric tests**

Run: `go test ./internal/embeddedstore -run 'TestQueryExecutor_ResourceTimelinePlannerParity|TestQueryMetrics_(ResourceEventStoreMixes|ResourcePaginationStopsAfterPageWindow)' -count=1`
Expected: PASS.

## Task 3: Verify And Measure

**Files:**
- Modify: `internal/embeddedstore/query_metrics_test.go` if any broader expectations need adjustment

- [ ] **Step 1: Run the local verification suite**

Run: `go test ./internal/embeddedstore/... ./cmd/spectre/commands -count=1`
Expected: PASS.

- [ ] **Step 2: Build, push, deploy, and remeasure on homelab**

Run the existing image build/push workflow, deploy to the `homelab-admin@kubernetes` context, then measure:

- cold-ish `1h` `/v1/timeline`
- cold-ish `4h` `/v1/timeline`
- pod memory after steady state

- [ ] **Step 3: Review remaining bottlenecks**

Use logs and measurements to decide whether the next dominant cost is:

- scanning `orderedResources` in memory
- attached Kubernetes Event lookups
- startup migration / disk pressure side effects
