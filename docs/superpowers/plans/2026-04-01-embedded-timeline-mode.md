# Embedded Timeline Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `--embedded` server mode that loads events from `--import-path` into an in-memory executor at startup and serves the existing timeline UI without watcher, graph, MCP, or HTTP import dependencies.

**Architecture:** `--embedded` is a hard runtime mode selected in `cmd/spectre/commands/server.go`. It bypasses graph and watcher startup, reuses `internal/importexport` for startup loading, and swaps in a new in-memory `api.QueryExecutor` implementation that serves timeline and metadata reads from indexed `models.Event` slices. Existing API and Connect timeline handlers remain in place; unsupported graph-backed surfaces are omitted by passing nil graph dependencies and not creating the MCP server.

**Tech Stack:** Go, Cobra, existing `internal/importexport`, `internal/api`, `internal/apiserver`, stdlib maps/slices/sort, existing timeline/metadata handlers and tests

---

## File Map

- Create: `cmd/spectre/commands/server_mode.go`
  Runtime mode resolution and validation for graph, audit-only, and embedded server shapes.
- Modify: `cmd/spectre/commands/server.go`
  Add `--embedded`, route startup through the runtime-mode helper, and wire the embedded executor path.
- Create: `cmd/spectre/commands/server_mode_test.go`
  Unit tests for mode validation and embedded-specific invariants.
- Create: `internal/embedded/query_executor.go`
  In-memory event indexes plus `Execute`, `ExecutePaginated`, `SetSharedCache`, and metadata query support.
- Create: `internal/embedded/query_executor_test.go`
  Tests for filtering, pagination, metadata extraction, pre-window anchor behavior, and Kubernetes `Event` handling.
- Create: `internal/apiserver/server_embedded_test.go`
  Route-level tests proving embedded mode exposes timeline/metadata surfaces and omits graph/import/MCP endpoints.
- Create: `tests/integration/api/embedded_timeline_test.go`
  API contract test that loads a small imported dataset into the embedded executor and verifies the timeline response shape.

### Task 1: Add Runtime Mode Resolution And CLI Validation

**Files:**
- Create: `cmd/spectre/commands/server_mode.go`
- Create: `cmd/spectre/commands/server_mode_test.go`
- Modify: `cmd/spectre/commands/server.go`
- Test: `cmd/spectre/commands/server_mode_test.go`

- [ ] **Step 1: Write failing runtime-mode tests**

```go
func TestResolveServerRuntimeMode_EmbeddedRequiresImportPath(t *testing.T) {
	mode, err := resolveServerRuntimeMode(serverModeInput{
		Embedded:  true,
		ImportPath: "",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "--import-path")
	require.False(t, mode.Embedded)
}

func TestResolveServerRuntimeMode_EmbeddedBypassesGraphRequirement(t *testing.T) {
	mode, err := resolveServerRuntimeMode(serverModeInput{
		Embedded:   true,
		ImportPath: "/tmp/events.json",
	})
	require.NoError(t, err)
	require.True(t, mode.Embedded)
	require.False(t, mode.StartGraph)
	require.False(t, mode.StartWatcher)
	require.False(t, mode.StartMCP)
}
```

- [ ] **Step 2: Run the mode tests and verify they fail**

Run: `go test ./cmd/spectre/commands -run 'TestResolveServerRuntimeMode' -v`
Expected: FAIL because `resolveServerRuntimeMode` and embedded-specific validation do not exist yet.

- [ ] **Step 3: Add the embedded flag and extract runtime-mode logic**

```go
type serverModeInput struct {
	Embedded      bool
	GraphEnabled  bool
	WatcherEnabled bool
	ImportPath    string
	AuditLogPath  string
}

type serverRuntimeMode struct {
	Name         string
	Embedded     bool
	AuditOnly    bool
	StartGraph   bool
	StartWatcher bool
	StartMCP     bool
}

func resolveServerRuntimeMode(in serverModeInput) (serverRuntimeMode, error) {
	if in.Embedded {
		if in.ImportPath == "" {
			return serverRuntimeMode{}, fmt.Errorf("--embedded requires --import-path")
		}
		return serverRuntimeMode{
			Name:         "embedded",
			Embedded:     true,
			StartGraph:   false,
			StartWatcher: false,
			StartMCP:     false,
		}, nil
	}

	auditOnly := !in.GraphEnabled && in.AuditLogPath != "" && in.WatcherEnabled
	if !in.GraphEnabled && !auditOnly {
		return serverRuntimeMode{}, fmt.Errorf("graph-enabled flag must be set to true, or use audit-only mode")
	}

	return serverRuntimeMode{
		Name:         "graph",
		AuditOnly:    auditOnly,
		StartGraph:   !auditOnly,
		StartWatcher: in.WatcherEnabled,
		StartMCP:     !auditOnly,
	}, nil
}
```

- [ ] **Step 4: Use the helper from `runServer`**

Update `cmd/spectre/commands/server.go` so `runServer`:

- registers `--embedded`
- resolves the runtime mode once near startup
- logs the selected mode
- branches off the runtime mode instead of open-coding graph vs audit-only decisions across the function

- [ ] **Step 5: Re-run the mode tests**

Run: `go test ./cmd/spectre/commands -run 'TestResolveServerRuntimeMode' -v`
Expected: PASS with explicit coverage for embedded validation.

- [ ] **Step 6: Commit the runtime-mode slice**

```bash
git add cmd/spectre/commands/server.go cmd/spectre/commands/server_mode.go cmd/spectre/commands/server_mode_test.go
git commit -m "feat: add embedded server runtime mode"
```

### Task 2: Build The In-Memory Embedded Query Executor

**Files:**
- Create: `internal/embedded/query_executor.go`
- Create: `internal/embedded/query_executor_test.go`
- Test: `internal/embedded/query_executor_test.go`

- [ ] **Step 1: Write failing executor tests**

```go
func TestQueryExecutor_ExecuteFiltersResources(t *testing.T) {
	exec := newTestExecutor(t, fixtureEvents())
	result, err := exec.Execute(context.Background(), &models.QueryRequest{
		StartTimestamp: 1700000000,
		EndTimestamp:   1700003600,
		Filters: models.QueryFilters{
			Namespaces: []string{"default"},
			Kinds:      []string{"Pod"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, int32(2), result.Count)
}

func TestQueryExecutor_AddsPreExistingAnchor(t *testing.T) {
	exec := newTestExecutor(t, fixtureEvents())
	result, err := exec.Execute(context.Background(), &models.QueryRequest{
		StartTimestamp: 1700000100,
		EndTimestamp:   1700003600,
		Filters:        models.QueryFilters{Kinds: []string{"Deployment"}},
	})
	require.NoError(t, err)
	require.True(t, result.Events[0].PreExisting)
}

func TestQueryExecutor_ExecutePaginatedByResource(t *testing.T) {
	exec := newTestExecutor(t, fixtureEvents())
	query := &models.QueryRequest{StartTimestamp: 1700000000, EndTimestamp: 1700003600}
	result, page, err := exec.ExecutePaginated(context.Background(), query, &models.PaginationRequest{PageSize: 1})
	require.NoError(t, err)
	require.Len(t, distinctUIDs(result.Events), 1)
	require.True(t, page.HasMore)
	require.NotEmpty(t, page.NextCursor)
}

func TestQueryExecutor_QueryDistinctMetadata(t *testing.T) {
	exec := newTestExecutor(t, fixtureEvents())
	namespaces, kinds, minTime, maxTime, err := exec.QueryDistinctMetadata(context.Background(), 0, math.MaxInt64)
	require.NoError(t, err)
	require.Contains(t, namespaces, "default")
	require.Contains(t, kinds, "Pod")
	require.NotZero(t, minTime)
	require.NotZero(t, maxTime)
}
```

- [ ] **Step 2: Run the embedded executor tests and verify they fail**

Run: `go test ./internal/embedded -run 'TestQueryExecutor' -v`
Expected: FAIL because the package and executor do not exist yet.

- [ ] **Step 3: Implement the embedded executor and indexes**

```go
type QueryExecutor struct {
	logger                *logging.Logger
	eventsByResourceUID   map[string][]models.Event
	resourceMetaByUID     map[string]models.ResourceMetadata
	k8sEventsByInvolvedUID map[string][]models.Event
	orderedResources      []resourceKey
	minTimestampNs        int64
	maxTimestampNs        int64
}

func NewQueryExecutor(events []models.Event) (*QueryExecutor, error) {
	qe := &QueryExecutor{
		logger:                 logging.GetLogger("embedded.query"),
		eventsByResourceUID:    map[string][]models.Event{},
		resourceMetaByUID:      map[string]models.ResourceMetadata{},
		k8sEventsByInvolvedUID: map[string][]models.Event{},
	}
	// index events, sort slices, populate orderedResources, min/max timestamps
	return qe, nil
}

func (qe *QueryExecutor) Execute(ctx context.Context, query *models.QueryRequest) (*models.QueryResult, error) {
	result, _, err := qe.ExecutePaginated(ctx, query, query.Pagination)
	return result, err
}

func (qe *QueryExecutor) SetSharedCache(cache interface{}) {}
```

Implementation requirements:

- treat non-`Event` resources as primary timeline resources
- route Kubernetes `kind=Event` objects into the involved-object index
- include the latest pre-window resource event as `PreExisting=true`
- implement resource-level cursor pagination using `models.ResourceCursor`
- implement `QueryDistinctMetadata` for fast `/v1/metadata`

- [ ] **Step 4: Re-run the embedded executor tests**

Run: `go test ./internal/embedded -run 'TestQueryExecutor' -v`
Expected: PASS with filtering, pagination, metadata, and anchor behavior covered.

- [ ] **Step 5: Commit the embedded executor slice**

```bash
git add internal/embedded/query_executor.go internal/embedded/query_executor_test.go
git commit -m "feat: add embedded timeline query executor"
```

### Task 3: Wire Embedded Startup Into The Server Command

**Files:**
- Modify: `cmd/spectre/commands/server.go`
- Test: `cmd/spectre/commands/server_mode_test.go`

- [ ] **Step 1: Extend the runtime-mode tests with startup-shape expectations**

Add tests that assert embedded mode:

- never enters graph-required validation
- never enables watcher startup
- never enables MCP startup

```go
func TestResolveServerRuntimeMode_EmbeddedSkipsWatcherAndMCP(t *testing.T) {
	mode, err := resolveServerRuntimeMode(serverModeInput{
		Embedded:      true,
		ImportPath:    "/tmp/events.json",
		WatcherEnabled: true,
		GraphEnabled:  true,
	})
	require.NoError(t, err)
	require.False(t, mode.StartWatcher)
	require.False(t, mode.StartGraph)
	require.False(t, mode.StartMCP)
}
```

- [ ] **Step 2: Run the runtime-mode tests**

Run: `go test ./cmd/spectre/commands -run 'TestResolveServerRuntimeMode' -v`
Expected: FAIL until `runServer` actually follows the resolved embedded shape.

- [ ] **Step 3: Add the embedded startup path in `runServer`**

Implement the embedded branch before the graph-only startup path:

```go
if runtimeMode.Embedded {
	eventValues, err := importexport.Import(importexport.FromPath(importPath), importexport.WithLogger(logger))
	if err != nil {
		HandleError(err, "Embedded import error")
	}

	embeddedExecutor, err := embedded.NewQueryExecutor(eventValues)
	if err != nil {
		HandleError(err, "Embedded index initialization error")
	}

	apiComponent := apiserver.NewWithStorageGraphAndPipeline(
		cfg.APIPort,
		embeddedExecutor,
		nil,
		api.TimelineQuerySourceStorage,
		nil,
		nil,
		nil,
		&apiserver.NoOpReadinessChecker{},
		tracingProvider,
		time.Duration(metadataCacheRefreshSeconds)*time.Second,
		apiserver.NamespaceGraphCacheConfig{},
		"",
		nil,
		nil,
	)
	// register API component, start manager, skip watcher/graph/MCP/integration initialization
}
```

Implementation requirements:

- load `--import-path` before `manager.Start` in embedded mode
- fail startup on zero usable events
- keep the existing deferred import-after-start behavior only for graph mode
- do not create the MCP server in embedded mode
- do not initialize integration manager in embedded mode

- [ ] **Step 4: Re-run the runtime-mode tests**

Run: `go test ./cmd/spectre/commands -run 'TestResolveServerRuntimeMode' -v`
Expected: PASS with the embedded startup branch wired in.

- [ ] **Step 5: Commit the server wiring slice**

```bash
git add cmd/spectre/commands/server.go cmd/spectre/commands/server_mode_test.go
git commit -m "feat: wire embedded startup path"
```

### Task 4: Verify Embedded API Surface And Timeline Contract

**Files:**
- Create: `internal/apiserver/server_embedded_test.go`
- Create: `tests/integration/api/embedded_timeline_test.go`
- Test: `internal/apiserver/server_embedded_test.go`
- Test: `tests/integration/api/embedded_timeline_test.go`

- [ ] **Step 1: Write failing route-surface tests for embedded mode**

```go
func TestServer_EmbeddedModeOmitsUnsupportedRoutes(t *testing.T) {
	exec := newEmbeddedExecutorForTest(t)
	srv := NewWithStorageGraphAndPipeline(
		8080,
		exec,
		nil,
		api.TimelineQuerySourceStorage,
		nil,
		nil,
		nil,
		&NoOpReadinessChecker{},
		nil,
		30*time.Second,
		NamespaceGraphCacheConfig{},
		"",
		nil,
		nil,
	)

	for _, path := range []string{"/v1/storage/import", "/v1/causal-graph", "/v1/mcp"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		srv.server.Handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusNotFound, rec.Code, path)
	}
}
```

- [ ] **Step 2: Run the embedded route tests and verify they fail**

Run: `go test ./internal/apiserver -run 'TestServer_EmbeddedMode' -v`
Expected: FAIL until the embedded server shape is fully wired and test helpers exist.

- [ ] **Step 3: Add the route-surface test helpers and make the omission behavior pass**

Make the test build against the real embedded executor and ensure the constructed server:

- serves `/v1/timeline`
- serves `/v1/metadata`
- returns `404` for `/v1/storage/import`, `/v1/causal-graph`, and `/v1/mcp`

- [ ] **Step 4: Write the API contract test**

```go
func TestEmbeddedTimelineAPI_ReturnsSegmentsAndK8sEvents(t *testing.T) {
	events, err := importexport.Import(importexport.FromReader(strings.NewReader(fixtureJSON)))
	require.NoError(t, err)

	exec, err := embedded.NewQueryExecutor(events)
	require.NoError(t, err)

	srv := apiserver.NewWithStorageGraphAndPipeline(
		8080, exec, nil, api.TimelineQuerySourceStorage,
		nil, nil, nil, &apiserver.NoOpReadinessChecker{},
		nil, 30*time.Second, apiserver.NamespaceGraphCacheConfig{}, "", nil, nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/timeline?start=1700000000&end=1700003600", nil)
	req.Header.Set("Accept-Encoding", "identity")
	rec := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "\"statusSegments\"")
	require.Contains(t, rec.Body.String(), "\"events\"")
}
```

- [ ] **Step 5: Run the route and API tests**

Run: `go test ./internal/apiserver ./tests/integration/api -run 'TestServer_EmbeddedMode|TestEmbeddedTimelineAPI' -count=1 -v`
Expected: PASS with omitted unsupported routes and a valid timeline response containing status segments and attached Kubernetes events.

- [ ] **Step 6: Commit the verification slice**

```bash
git add internal/apiserver/server_embedded_test.go tests/integration/api/embedded_timeline_test.go
git commit -m "test: cover embedded timeline server mode"
```

### Task 5: Final Verification Pass

**Files:**
- Modify: none
- Test: `cmd/spectre/commands/server_mode_test.go`
- Test: `internal/embedded/query_executor_test.go`
- Test: `internal/apiserver/server_embedded_test.go`
- Test: `tests/integration/api/embedded_timeline_test.go`

- [ ] **Step 1: Run the focused package suite**

Run: `go test ./cmd/spectre/commands ./internal/embedded ./internal/apiserver ./tests/integration/api -count=1`
Expected: PASS with no failures in the embedded-specific packages.

- [ ] **Step 2: Run the broader timeline-related regression suite**

Run: `go test ./internal/api/... ./internal/mcp/... -count=1`
Expected: PASS, or if an unrelated pre-existing failure occurs, document it before merging.

- [ ] **Step 3: Smoke-test the embedded server manually**

Run:

```bash
go build -o bin/spectre ./cmd/spectre
bin/spectre server --embedded --import-path <fixture.json> --api-port 18080
curl -s 'http://localhost:18080/v1/timeline?start=1700000000&end=1700003600'
curl -i 'http://localhost:18080/v1/storage/import'
```

Expected:

- server starts without graph configuration
- timeline request returns `200`
- `/v1/storage/import` returns `404`

- [ ] **Step 4: Record the verification results in the handoff**

Document:

- exact commands run
- whether manual smoke test succeeded
- any known gaps or unrelated failing packages

- [ ] **Step 5: Commit any final test adjustments**

```bash
git add -A
git commit -m "chore: finalize embedded timeline verification"
```
