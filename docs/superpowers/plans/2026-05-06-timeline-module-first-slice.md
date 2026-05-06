# Timeline Module First Slice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move timeline execution and pagination policy into `internal/app/timeline`, then simplify the Connect adapter to consume that deeper seam.

**Architecture:** Keep transport parsing in adapters, but centralize executor-aware pagination, Kubernetes Event side-query behavior, and stream-ready entry selection inside the application timeline module. This preserves current behavior while increasing locality around the timeline seam.

**Tech Stack:** Go, Connect RPC, existing `models.QueryRequest`/`PaginationRequest`, Go test

---

### Task 1: Add App-Level Timeline Execution Tests

**Files:**
- Modify: `internal/app/timeline/service_test.go`

- [ ] **Step 1: Write the failing test**

Add tests that assert:

```go
func TestService_ExecuteTimeline_UsesExecutorPaginationWhenAvailable(t *testing.T) {}

func TestService_ExecuteTimeline_FallsBackToClientPagination(t *testing.T) {}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/timeline -run 'TestService_ExecuteTimeline_'`
Expected: FAIL with missing method or missing types

- [ ] **Step 3: Write minimal implementation**

Add a timeline execution result type plus one service method that returns:

```go
type ExecutionResult struct {
    ResourceResult *models.QueryResult
    EventResult    *models.QueryResult
    Index          *TimelineIndex
    Entries        []*TimelineResourceEntry
    Pagination     *models.PaginationResponse
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/timeline -run 'TestService_ExecuteTimeline_'`
Expected: PASS

### Task 2: Move Entry Pagination To App Timeline

**Files:**
- Modify: `internal/app/timeline/service_test.go`
- Modify: `internal/app/timeline/service.go`
- Modify: `internal/api/timeline_streaming_test.go`
- Modify: `internal/api/timeline_connect_service.go`

- [ ] **Step 1: Write the failing test**

Move entry pagination assertions to app-level tests for sorted cursor slicing.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/timeline -run TestService_PaginateEntries`
Expected: FAIL with missing pagination helper

- [ ] **Step 3: Write minimal implementation**

Add an app-level pagination helper and have the Connect adapter call it.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/app/timeline ./internal/api -run 'Test(Service_PaginateEntries|ApplyEntryPagination_)'`
Expected: PASS

### Task 3: Simplify Connect Adapter

**Files:**
- Modify: `internal/api/timeline_connect_service.go`

- [ ] **Step 1: Write the failing test**

Adjust or add tests so the Connect adapter only verifies metadata and streamed batches from the app result.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/api -run 'TestApplyEntryPagination_|TestTimelineConnect'`
Expected: FAIL until adapter uses the new app seam

- [ ] **Step 3: Write minimal implementation**

Replace inline execution/pagination orchestration with a single call into `internal/app/timeline`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/api -run 'TestApplyEntryPagination_|TestTimelineConnect'`
Expected: PASS

### Task 4: Verify Full First Slice

**Files:**
- Modify: none

- [ ] **Step 1: Run focused verification**

Run: `go test ./internal/app/timeline ./internal/api`
Expected: PASS

- [ ] **Step 2: Run integration guard**

Run: `go test ./internal/apiserver ./tests/integration/api/...`
Expected: PASS or explicit unrelated failures only
