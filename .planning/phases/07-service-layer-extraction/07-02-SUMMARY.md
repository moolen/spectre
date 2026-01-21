---
phase: 07-service-layer-extraction
plan: 02
subsystem: api
tags: [graphservice, mcp, graph-analysis, falkordb, anomaly-detection, causal-paths]

# Dependency graph
requires:
  - phase: 07-01
    provides: TimelineService pattern for service layer extraction
provides:
  - GraphService wrapping FalkorDB graph analysis operations
  - MCP graph tools using GraphService directly (no HTTP)
  - Shared graph service for REST and MCP
affects: [07-03, 07-04, 07-05]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - GraphService facade pattern over analysis modules
    - Dual-mode tool constructors (service vs HTTP client)
    - Service sharing between REST handlers and MCP tools

key-files:
  created:
    - internal/api/graph_service.go
  modified:
    - internal/api/handlers/causal_paths_handler.go
    - internal/api/handlers/anomaly_handler.go
    - internal/api/handlers/namespace_graph_handler.go
    - internal/api/handlers/register.go
    - internal/mcp/tools/causal_paths.go
    - internal/mcp/tools/detect_anomalies.go
    - internal/mcp/server.go
    - cmd/spectre/commands/server.go

key-decisions:
  - "GraphService wraps existing analyzers rather than reimplementing logic"
  - "Dual constructors (WithService/WithClient) for backward compatibility"
  - "Timeline integration deferred for detect_anomalies (uses HTTP for now)"

patterns-established:
  - "GraphService facade: DiscoverCausalPaths, DetectAnomalies, AnalyzeNamespaceGraph methods"
  - "REST handlers delegate to services, MCP tools call services directly"
  - "Server initializes GraphService and passes to both REST and MCP"

# Metrics
duration: 12min
completed: 2026-01-21
---

# Phase 7 Plan 2: GraphService Extraction Summary

**GraphService wrapping FalkorDB operations with direct service calls from MCP graph tools (causal_paths, detect_anomalies)**

## Performance

- **Duration:** 12 min
- **Started:** 2026-01-21T19:24:11Z
- **Completed:** 2026-01-21T19:35:46Z
- **Tasks:** 3
- **Files modified:** 9

## Accomplishments
- GraphService wraps causalpaths.PathDiscoverer, anomaly.AnomalyDetector, namespacegraph.Analyzer
- REST graph handlers refactored to use GraphService
- MCP causal_paths and detect_anomalies tools call GraphService directly (no HTTP)
- Server wires GraphService to both REST and MCP layers

## Task Commits

Each task was committed atomically:

1. **Task 1: Create GraphService** - `48fff1a` (feat)
   - Created internal/api/graph_service.go with facade over analyzers
   - Methods: DiscoverCausalPaths, DetectAnomalies, AnalyzeNamespaceGraph

2. **Task 2: Refactor REST handlers** - `1988750` (refactor)
   - Updated CausalPathsHandler, AnomalyHandler, NamespaceGraphHandler to use GraphService
   - Removed direct analyzer dependencies from handlers
   - GraphService created in register.go and passed to handlers

3. **Task 3: Wire MCP tools** - `ba0bda2` + `e213fcb` (feat)
   - Updated CausalPathsTool and DetectAnomaliesTool to use GraphService
   - Added WithClient constructors for backward compatibility
   - MCP server passes GraphService to tools via ServerOptions
   - Server initialization creates and wires GraphService

## Files Created/Modified
- `internal/api/graph_service.go` - GraphService facade over analysis modules
- `internal/api/handlers/causal_paths_handler.go` - Refactored to use GraphService
- `internal/api/handlers/anomaly_handler.go` - Refactored to use GraphService
- `internal/api/handlers/namespace_graph_handler.go` - Refactored to use GraphService
- `internal/api/handlers/register.go` - Creates and passes GraphService to handlers
- `internal/mcp/tools/causal_paths.go` - Calls GraphService.DiscoverCausalPaths directly
- `internal/mcp/tools/detect_anomalies.go` - Calls GraphService.DetectAnomalies directly
- `internal/mcp/server.go` - Accepts GraphService via ServerOptions
- `cmd/spectre/commands/server.go` - Creates GraphService and passes to MCP server

## Decisions Made

1. **GraphService as Facade**: Wraps existing analyzers rather than reimplementing logic
   - **Rationale:** Existing analyzers (PathDiscoverer, AnomalyDetector, Analyzer) already work correctly. GraphService provides unified interface without duplicating functionality.

2. **Dual Constructors (WithService/WithClient)**: Both patterns supported during transition
   - **Rationale:** Agent tools still use HTTP client, MCP tools use services. Backward compatibility enables gradual migration.

3. **Timeline Integration Deferred**: detect_anomalies uses HTTP client for timeline queries
   - **Rationale:** TimelineService integration requires ParseQueryParameters + ExecuteConcurrentQueries pattern (complex). Deferring to keep plan focused on graph operations.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed normalizeToNanoseconds duplication**
- **Found during:** Task 2 (refactoring handlers)
- **Issue:** normalizeToNanoseconds function duplicated in causal_paths_handler.go and namespace_graph_handler.go causing compilation error
- **Fix:** Removed duplicate from namespace_graph_handler.go, kept single definition in causal_paths_handler.go
- **Files modified:** internal/api/handlers/namespace_graph_handler.go
- **Verification:** Handlers compile successfully, tests pass
- **Committed in:** 1988750 (Task 2 commit)

**2. [Rule 2 - Missing Critical] Fixed unused import cleanup**
- **Found during:** Task 2 (refactoring handlers)
- **Issue:** Handlers no longer use graph.Client directly but still imported it
- **Fix:** Removed unused graph.Client imports from three handler files
- **Files modified:** causal_paths_handler.go, anomaly_handler.go, namespace_graph_handler.go
- **Verification:** Go build succeeds without unused import errors
- **Committed in:** 1988750 (Task 2 commit)

**3. [Rule 1 - Bug] Fixed anomaly type conversions**
- **Found during:** Task 3 (MCP tool updates)
- **Issue:** anomaly.AnomalyCategory and anomaly.Severity are typed strings, cannot assign directly to string fields
- **Fix:** Cast to string: `string(a.Category)`, `string(a.Severity)`
- **Files modified:** internal/mcp/tools/detect_anomalies.go
- **Verification:** MCP tools compile successfully
- **Committed in:** ba0bda2 (Task 3 commit)

**4. [Rule 1 - Bug] Fixed metadata field name mismatch**
- **Found during:** Task 3 (MCP tool updates)
- **Issue:** client.AnomalyMetadata uses ExecTimeMs but typed as ExecutionTimeMs in transform
- **Fix:** Use correct field name ExecTimeMs for HTTP client response
- **Files modified:** internal/mcp/tools/detect_anomalies.go
- **Verification:** Server compiles successfully
- **Committed in:** ba0bda2 (Task 3 commit)

**5. [Rule 2 - Missing Critical] Fixed agent tool constructor calls**
- **Found during:** Task 3 verification (server build)
- **Issue:** Agent tools still called old constructors without WithClient suffix
- **Fix:** Updated to use NewCausalPathsToolWithClient and NewDetectAnomaliesToolWithClient
- **Files modified:** internal/agent/tools/registry.go
- **Verification:** Full server build succeeds
- **Committed in:** ba0bda2 (Task 3 commit)

---

**Total deviations:** 5 auto-fixed (3 bugs, 2 missing critical)
**Impact on plan:** All auto-fixes necessary for compilation and correct type handling. No scope creep - all fixes were corrections to enable plan execution.

## Issues Encountered
- Type system required explicit casts for custom string types (AnomalyCategory, Severity) - handled by casting to string
- TimelineService integration more complex than anticipated - deferred timeline queries to HTTP client in detect_anomalies to keep plan focused

## Next Phase Readiness
- GraphService pattern established and working for graph operations
- Ready to replicate for SearchService (07-03) and MetadataService (07-04)
- MCP tools successfully use direct service calls (no HTTP overhead for graph operations)
- REST handlers and MCP tools share same business logic via services

---
*Phase: 07-service-layer-extraction*
*Completed: 2026-01-21*
