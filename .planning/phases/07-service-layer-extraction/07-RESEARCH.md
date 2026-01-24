# Phase 7: Service Layer Extraction - Research

**Researched:** 2026-01-21
**Domain:** Go service layer architecture for shared REST and MCP tool access
**Confidence:** HIGH

## Summary

This phase involves extracting business logic from REST handlers and making it accessible to both HTTP endpoints and MCP tools through shared service interfaces. Currently, MCP tools make HTTP self-calls to localhost:8080 to access functionality. The goal is to eliminate these HTTP calls by having both REST handlers and MCP tools directly invoke in-process service methods.

**Current state:**
- REST handlers contain inline business logic (timeline building, graph queries, metadata operations)
- MCP tools use HTTP client (`internal/mcp/client/client.go`) to call REST endpoints
- A partial TimelineService already exists (`internal/api/timeline_service.go`) but is only used by gRPC/Connect RPC services
- Handlers depend on QueryExecutor interface, graph.Client, logging, and tracing infrastructure

**Target state:**
- Four service interfaces: TimelineService, GraphService, SearchService, MetadataService
- Services encapsulate all business logic currently in handlers
- Both REST handlers and MCP tools call services directly
- No HTTP self-calls from MCP tools

**Primary recommendation:** Follow the existing TimelineService pattern for new services, use constructor injection, define interfaces alongside implementations in `internal/api`, and refactor one service at a time starting with Timeline.

## Standard Stack

The established libraries/tools for this domain:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Standard lib (net/http) | Go 1.x | HTTP handlers | Already used throughout codebase |
| context.Context | Go 1.x | Context propagation | Go standard for cancellation/timeouts |
| go.opentelemetry.io/otel | Current | Distributed tracing | Already integrated for observability |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/moolen/spectre/internal/logging | Current | Structured logging | All service operations |
| github.com/moolen/spectre/internal/models | Current | Domain models | Request/response types |
| github.com/moolen/spectre/internal/graph | Current | FalkorDB client | Graph query operations |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Constructor injection | Service locator pattern | Constructor injection is simpler, more explicit |
| Flat service hierarchy | Layered services | Flat is appropriate given current scope |
| Interfaces in api package | Separate services package | Co-location with implementations is Go-idiomatic |

**Installation:**
No additional dependencies needed - all infrastructure already exists.

## Architecture Patterns

### Recommended Project Structure
```
internal/
├── api/
│   ├── timeline_service.go      # TimelineService implementation (already exists)
│   ├── graph_service.go          # NEW: GraphService for FalkorDB operations
│   ├── search_service.go         # NEW: SearchService for unified search
│   ├── metadata_service.go       # NEW: MetadataService for resource metadata
│   ├── handlers/                 # REST handlers refactored to use services
│   └── interfaces.go             # Shared interfaces (QueryExecutor, etc.)
├── mcp/
│   ├── tools/                    # MCP tools refactored to use services directly
│   └── client/                   # DELETE after migration (HTTP client)
└── graph/
    └── client.go                 # FalkorDB client interface
```

### Pattern 1: Service Interface with Constructor Injection
**What:** Services defined as structs with dependencies injected via constructor
**When to use:** All new services in this phase
**Example:**
```go
// Source: internal/api/timeline_service.go (existing pattern)
type TimelineService struct {
	storageExecutor QueryExecutor
	graphExecutor   QueryExecutor
	querySource     TimelineQuerySource
	logger          *logging.Logger
	tracer          trace.Tracer
	validator       *Validator
}

func NewTimelineService(queryExecutor QueryExecutor, logger *logging.Logger, tracer trace.Tracer) *TimelineService {
	return &TimelineService{
		storageExecutor: queryExecutor,
		querySource:     TimelineQuerySourceStorage,
		logger:          logger,
		validator:       NewValidator(),
		tracer:          tracer,
	}
}
```

### Pattern 2: Context-First Method Signatures
**What:** Methods that perform I/O take context.Context as first parameter
**When to use:** Methods that query databases, make network calls, or have cancellation semantics
**Example:**
```go
// Source: internal/api/timeline_service.go
func (s *TimelineService) ExecuteConcurrentQueries(ctx context.Context, query *models.QueryRequest) (*models.QueryResult, *models.QueryResult, error) {
	// Create child span for concurrent execution
	ctx, span := s.tracer.Start(ctx, "timeline.executeConcurrentQueries")
	defer span.End()

	// Use context for cancellation
	executor := s.GetActiveExecutor()
	if executor == nil {
		return nil, nil, fmt.Errorf("no query executor available")
	}
	// ... rest of implementation
}
```

### Pattern 3: Domain Error Types
**What:** Services return domain-specific error types that callers map to transport-specific codes
**When to use:** Error conditions that have semantic meaning (not found, validation failed, etc.)
**Example:**
```go
// Source: internal/api/validation.go (existing pattern)
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func NewValidationError(format string, args ...interface{}) error {
	return &ValidationError{
		Message: fmt.Sprintf(format, args...),
	}
}

// Handler maps to HTTP status:
// if _, ok := err.(*api.ValidationError); ok {
//     return http.StatusBadRequest
// }
```

### Pattern 4: Observability Integration
**What:** Services use OpenTelemetry spans for distributed tracing
**When to use:** All service methods that perform meaningful operations
**Example:**
```go
// Source: internal/api/timeline_service.go
ctx, span := s.tracer.Start(ctx, "timeline.executeConcurrentQueries")
defer span.End()

span.SetAttributes(
	attribute.String("query.source", string(s.querySource)),
	attribute.Int("resource_count", int(resourceResult.Count)),
)

if err != nil {
	span.RecordError(err)
	span.SetStatus(codes.Error, "Query execution failed")
	return nil, nil, err
}
```

### Anti-Patterns to Avoid
- **HTTP self-calls within services:** Services should never make HTTP calls to localhost - this is what we're eliminating
- **Tight coupling to HTTP concerns:** Services should not import net/http or handle HTTP-specific logic (status codes, headers)
- **Shared mutable state:** Services should be stateless or use explicit concurrency control
- **God services:** Keep services focused on a single domain (timeline, graph, search, metadata)

## Don't Hand-Roll

Problems that look simple but have existing solutions:

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Query result transformation | Custom mappers per handler | Shared service methods (e.g., BuildTimelineResponse) | TimelineService already implements complex resource building logic with status segment inference |
| Concurrent query execution | Ad-hoc goroutines in handlers | Service method with WaitGroup | TimelineService.ExecuteConcurrentQueries already handles concurrent resource+event queries safely |
| Timestamp parsing/validation | Custom validation in each handler | Centralized api.ParseTimestamp | Already exists and handles multiple formats (RFC3339, Unix seconds/ms/ns) |
| Graph query building | String concatenation in handlers | GraphService methods | Graph queries require proper escaping, parameterization, and error handling |
| Metadata caching | Per-handler caching logic | MetadataCache (already exists) | internal/api/metadata_cache.go already implements background refresh and concurrent access |

**Key insight:** Much of the business logic for timeline, metadata, and graph operations already exists but is scattered across handlers. The extraction work is primarily moving code, not rewriting it.

## Common Pitfalls

### Pitfall 1: Forgetting to Delete HTTP Client Code
**What goes wrong:** After wiring services to MCP tools, the old HTTP client code remains unused but not removed
**Why it happens:** Migration is incremental and cleanup is easy to forget
**How to avoid:** Delete `internal/mcp/client/client.go` and HTTP call code in tools immediately after each service is wired
**Warning signs:** Import of `internal/mcp/client` still exists in tool files

### Pitfall 2: Mixing HTTP Concerns into Services
**What goes wrong:** Service methods return http.Response types or handle HTTP headers
**Why it happens:** When extracting from handlers, HTTP-specific code gets pulled in
**How to avoid:** Services should return domain models (`models.QueryResult`, `models.SearchResponse`), handlers convert to HTTP responses
**Warning signs:** Service imports `net/http`, methods accept `http.ResponseWriter`

### Pitfall 3: Incomplete Dependency Injection
**What goes wrong:** Services access global state or create their own dependencies instead of receiving them
**Why it happens:** Easier to add a global logger than thread it through constructors
**How to avoid:** Use constructor injection for all dependencies (logger, tracer, clients), avoid package-level globals
**Warning signs:** Service calls `logging.GetLogger()` instead of using `s.logger`

### Pitfall 4: Breaking Existing Functionality During Migration
**What goes wrong:** REST endpoints or MCP tools stop working when services are extracted
**Why it happens:** Subtle differences in error handling, validation, or data transformation
**How to avoid:** Migrate one service at a time, run integration tests after each service, keep existing tests passing
**Warning signs:** Handler tests fail, MCP tool behavior changes

### Pitfall 5: Service Method Signatures Too Handler-Specific
**What goes wrong:** Service methods take `*http.Request` or return handler-specific types
**Why it happens:** Extracting code mechanically without adapting interfaces
**How to avoid:** Service methods should accept domain types (`models.QueryRequest`), not HTTP types
**Warning signs:** Service depends on HTTP request parsing, query parameter extraction

## Code Examples

Verified patterns from official sources:

### Existing TimelineService Pattern
```go
// Source: internal/api/timeline_service.go (lines 21-53)
type TimelineService struct {
	storageExecutor QueryExecutor
	graphExecutor   QueryExecutor
	querySource     TimelineQuerySource
	logger          *logging.Logger
	tracer          trace.Tracer
	validator       *Validator
}

func NewTimelineService(queryExecutor QueryExecutor, logger *logging.Logger, tracer trace.Tracer) *TimelineService {
	return &TimelineService{
		storageExecutor: queryExecutor,
		querySource:     TimelineQuerySourceStorage,
		logger:          logger,
		validator:       NewValidator(),
		tracer:          tracer,
	}
}

func NewTimelineServiceWithMode(storageExecutor, graphExecutor QueryExecutor, querySource TimelineQuerySource, logger *logging.Logger, tracer trace.Tracer) *TimelineService {
	return &TimelineService{
		storageExecutor: storageExecutor,
		graphExecutor:   graphExecutor,
		querySource:     querySource,
		logger:          logger,
		validator:       NewValidator(),
		tracer:          tracer,
	}
}
```

### Current Handler Using QueryExecutor Directly
```go
// Source: internal/api/handlers/timeline_handler.go (lines 31-63)
type TimelineHandler struct {
	storageExecutor api.QueryExecutor
	graphExecutor   api.QueryExecutor
	querySource     TimelineQuerySource
	logger          *logging.Logger
	validator       *api.Validator
	tracer          trace.Tracer
}

func NewTimelineHandler(queryExecutor api.QueryExecutor, logger *logging.Logger, tracer trace.Tracer) *TimelineHandler {
	return &TimelineHandler{
		storageExecutor: queryExecutor,
		querySource:     TimelineQuerySourceStorage,
		logger:          logger,
		validator:       api.NewValidator(),
		tracer:          tracer,
	}
}

// After service extraction, handler will be:
type TimelineHandler struct {
	service *api.TimelineService  // Changed: single dependency
	logger  *logging.Logger
	tracer  trace.Tracer
}
```

### Current MCP Tool Making HTTP Call
```go
// Source: internal/mcp/tools/resource_timeline.go (lines 86-153)
func (t *ResourceTimelineTool) Execute(ctx context.Context, input json.RawMessage) (interface{}, error) {
	var params ResourceTimelineInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Currently makes HTTP call via client:
	response, err := t.client.QueryTimeline(startTime, endTime, filters, 1000)
	if err != nil {
		return nil, fmt.Errorf("failed to query timeline: %w", err)
	}

	// After service extraction:
	// query := &models.QueryRequest{
	//     StartTimestamp: startTime,
	//     EndTimestamp:   endTime,
	//     Filters:        models.QueryFilters{...},
	// }
	// queryResult, eventResult, err := t.timelineService.ExecuteConcurrentQueries(ctx, query)
	// response := t.timelineService.BuildTimelineResponse(queryResult, eventResult)
}
```

### Graph Operations Pattern
```go
// Source: internal/api/handlers/causal_paths_handler.go (lines 18-34)
type CausalPathsHandler struct {
	discoverer *causalpaths.PathDiscoverer  // Uses graph.Client internally
	logger     *logging.Logger
	validator  *api.Validator
	tracer     trace.Tracer
}

func NewCausalPathsHandler(graphClient graph.Client, logger *logging.Logger, tracer trace.Tracer) *CausalPathsHandler {
	return &CausalPathsHandler{
		discoverer: causalpaths.NewPathDiscoverer(graphClient),
		logger:     logger,
		validator:  api.NewValidator(),
		tracer:     tracer,
	}
}

// GraphService will encapsulate common graph operations:
// - Neighbor queries (MATCH (n)-[r]->(m) patterns)
// - Path discovery (used by causal paths, namespace graph)
// - Relationship traversal (OWNS, CHANGED, EMITTED_EVENT)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| HTTP self-calls from MCP | In-process service calls | Phase 7 (now) | Eliminates network overhead, simplifies error handling |
| Business logic in handlers | Business logic in services | Phase 7 (now) | Enables code reuse between REST and MCP |
| Handler-specific implementations | Shared service layer | Phase 7 (now) | Single source of truth for business logic |

**Deprecated/outdated:**
- `internal/mcp/client/client.go`: HTTP client for localhost self-calls (will be deleted in Phase 7)
- HTTP-based tool communication: MCP tools should call services directly, not via HTTP

## Operations Requiring Extraction

### Timeline Operations (TimelineService)
**Current implementations:**
- `internal/api/timeline_service.go` - Already exists with core methods:
  - `ExecuteConcurrentQueries(ctx, query)` - Concurrent resource + event queries
  - `BuildTimelineResponse(queryResult, eventResult)` - Transform to timeline format
  - `GetActiveExecutor()` - Select storage vs graph executor
  - `ResourceToProto(resource)` - Convert to protobuf (gRPC specific, may not need for REST/MCP)

**What needs extraction from handlers:**
- `internal/api/handlers/timeline_handler.go`:
  - Query parameter parsing (lines 444-493) - Move to service as domain model construction
  - Pagination parsing (lines 507-517) - Move to service
  - Response transformation logic (lines 233-441) - Already exists as `BuildTimelineResponse` in service!

**MCP tools that need service access:**
- `internal/mcp/tools/resource_timeline.go` - HTTP call at line 118: `t.client.QueryTimeline(...)`
- `internal/mcp/tools/cluster_health.go` - HTTP call at line 122: `t.client.QueryTimeline(...)`

**Dependencies:**
- QueryExecutor (storage and/or graph)
- logging.Logger
- trace.Tracer
- api.Validator

### Graph Operations (GraphService - NEW)
**Current implementations:** Scattered across handlers
- `internal/api/handlers/causal_paths_handler.go`:
  - Uses `causalpaths.PathDiscoverer` which wraps graph.Client
  - Path discovery: `discoverer.DiscoverCausalPaths(ctx, input)` (line 77)

- `internal/api/handlers/anomaly_handler.go`:
  - Uses `anomaly.AnomalyDetector` which wraps graph.Client
  - Anomaly detection: `detector.Detect(ctx, input)` (line 76)

- `internal/api/handlers/namespace_graph_handler.go`:
  - Uses `namespacegraph.Analyzer` which wraps graph.Client
  - Namespace analysis: `analyzer.Analyze(ctx, input)` (line 110)

**What needs extraction:**
- Common graph query patterns:
  - Neighbor queries: `MATCH (n)-[r]->(m)` traversals
  - Ownership chains: `MATCH (n)-[:OWNS*]->(m)` recursive patterns
  - Time-filtered queries: `WHERE e.timestamp >= $start AND e.timestamp <= $end`
  - K8s event relationships: `MATCH (r)-[:EMITTED_EVENT]->(e:K8sEvent)`

**Note:** Handlers currently use specialized analyzers (`PathDiscoverer`, `AnomalyDetector`, `Analyzer`) that encapsulate graph logic. GraphService may wrap these or provide lower-level graph query primitives.

**MCP tools that need service access:**
- `internal/mcp/tools/causal_paths.go` - HTTP call at line 77: `t.client.QueryCausalPaths(...)`
- `internal/mcp/tools/detect_anomalies.go` - HTTP call at lines 127, 205: `t.client.DetectAnomalies(...)`

**Dependencies:**
- graph.Client (FalkorDB)
- logging.Logger
- trace.Tracer

### Search Operations (SearchService - NEW)
**Current implementations:**
- `internal/api/handlers/search_handler.go`:
  - Query executor: `sh.queryExecutor.Execute(ctx, query)` (line 42)
  - Response building: `sh.buildSearchResponse(result)` (lines 59-86)
  - Query parameter parsing: `sh.parseQuery(r)` (lines 88-133)

**What needs extraction:**
- Query validation and parsing
- Search result transformation (simple version - groups events by resource UID)
- TODO comment notes: "Reimplement ResourceBuilder functionality for graph-based queries" (line 58)

**MCP tools that need service access:**
- None currently - search is only exposed via REST

**Dependencies:**
- QueryExecutor
- logging.Logger
- trace.Tracer
- api.Validator

### Metadata Operations (MetadataService - NEW)
**Current implementations:**
- `internal/api/handlers/metadata_handler.go`:
  - Direct query: `mh.queryExecutor.Execute(ctx, query)` (line 101)
  - Efficient metadata query: `QueryDistinctMetadata(ctx, startTimeNs, endTimeNs)` (line 86)
  - Cache integration: `mh.metadataCache.Get()` (line 67)
  - Response building: Extract namespaces, kinds, time range (lines 108-156)

- `internal/api/metadata_cache.go`:
  - Background refresh: Periodically queries metadata
  - Already encapsulates query logic

**What needs extraction:**
- Metadata query operations (already partially encapsulated in MetadataCache)
- Time range calculation
- Namespace/kind extraction and deduplication

**MCP tools that need service access:**
- `internal/mcp/tools/cluster_health.go` - Uses timeline indirectly, could benefit from metadata for namespace discovery
- None directly call metadata endpoint currently

**Dependencies:**
- QueryExecutor (with MetadataQueryExecutor interface)
- MetadataCache (optional)
- logging.Logger
- trace.Tracer

## Infrastructure Dependencies

### QueryExecutor Interface
**Location:** `internal/api/interfaces.go`
**Definition:**
```go
type QueryExecutor interface {
	Execute(ctx context.Context, query *models.QueryRequest) (*models.QueryResult, error)
	SetSharedCache(cache interface{})
}
```

**Implementations:**
- Storage-based executor (VictoriaLogs)
- Graph-based executor (FalkorDB)

**Services that need it:**
- TimelineService (both executors)
- SearchService (one executor)
- MetadataService (one executor with metadata optimization)

### Graph Client
**Location:** `internal/graph/client.go`
**Interface:**
```go
type Client interface {
	Connect(ctx context.Context) error
	Close() error
	Ping(ctx context.Context) error
	ExecuteQuery(ctx context.Context, query GraphQuery) (*QueryResult, error)
	CreateNode(ctx context.Context, nodeType NodeType, properties interface{}) error
	CreateEdge(ctx context.Context, edgeType EdgeType, fromUID, toUID string, properties interface{}) error
	GetNode(ctx context.Context, nodeType NodeType, uid string) (*Node, error)
	DeleteNodesByTimestamp(ctx context.Context, nodeType NodeType, timestampField string, cutoffNs int64) (int, error)
	GetGraphStats(ctx context.Context) (*GraphStats, error)
	InitializeSchema(ctx context.Context) error
	DeleteGraph(ctx context.Context) error
}
```

**Services that need it:**
- GraphService (all operations)
- Potentially TimelineService (if using graph executor)

### Logging and Tracing
**Location:** `internal/logging` and `go.opentelemetry.io/otel`
**Usage pattern:**
```go
logger.Debug("Operation completed: resources=%d", count)
logger.Error("Operation failed: %v", err)

ctx, span := tracer.Start(ctx, "service.method")
defer span.End()
span.SetAttributes(attribute.String("key", "value"))
span.RecordError(err)
```

**Services that need it:**
- All services (logging and tracing are cross-cutting)

## MCP Tool HTTP Self-Calls Inventory

All MCP tools currently use `internal/mcp/client/client.go` which provides:

### Timeline Queries
- **Method:** `QueryTimeline(startTime, endTime int64, filters map[string]string, pageSize int)`
- **Endpoint:** `GET /v1/timeline`
- **Used by:**
  - `resource_timeline.go` (line 118)
  - `cluster_health.go` (line 122)
  - `detect_anomalies.go` (line 152 - for resource discovery)

### Metadata Queries
- **Method:** `GetMetadata()`
- **Endpoint:** `GET /v1/metadata`
- **Used by:** None directly (could be useful for namespace/kind discovery)

### Anomaly Detection
- **Method:** `DetectAnomalies(resourceUID string, start, end int64)`
- **Endpoint:** `GET /v1/anomalies`
- **Used by:**
  - `detect_anomalies.go` (lines 127, 205)

### Causal Paths
- **Method:** `QueryCausalPaths(resourceUID string, failureTimestamp int64, lookbackMinutes, maxDepth, maxPaths int)`
- **Endpoint:** `GET /v1/causal-paths`
- **Used by:**
  - `causal_paths.go` (line 77)

### Health Check
- **Method:** `Ping()` and `PingWithRetry(logger Logger)`
- **Endpoint:** `GET /health`
- **Used by:** Server startup for MCP tool availability check

**After Phase 7:**
- All these HTTP calls will be replaced with direct service method calls
- `internal/mcp/client/client.go` will be deleted
- Tools will receive service instances via constructor injection

## Migration Strategy

### Order of Extraction

**Decision from CONTEXT.md:** Timeline → Graph → Search → Metadata

**Rationale:**
1. **Timeline first:** Most complex, already has partial service implementation, used by most MCP tools
2. **Graph second:** Used by multiple analysis features (causal paths, anomalies, namespace graph)
3. **Search third:** Simpler transformation logic, fewer dependencies
4. **Metadata last:** Simplest, already mostly encapsulated in MetadataCache

### Per-Service Migration Steps

For each service (Timeline, Graph, Search, Metadata):

1. **Define/verify service interface** in `internal/api/`
   - For Timeline: Interface already exists, verify completeness
   - For others: Define new interface with methods from handlers

2. **Extract business logic to service**
   - Move query building, validation, transformation from handler to service
   - Add context parameter to methods that do I/O
   - Add tracing spans and logging

3. **Refactor REST handler to use service**
   - Replace inline logic with service method calls
   - Keep HTTP-specific concerns (parsing, response writing) in handler
   - Run handler tests to verify behavior unchanged

4. **Wire service to MCP tools**
   - Add service as dependency to tool constructors
   - Replace HTTP client calls with direct service method calls
   - Update tool initialization in `internal/mcp/server.go`

5. **Delete HTTP client code**
   - Remove HTTP call from tool implementation
   - After all tools migrated, delete `internal/mcp/client/client.go`

6. **Verify integration**
   - Run MCP tool tests
   - Manual testing of both REST endpoints and MCP tools
   - Check tracing spans are correct

## Open Questions

None - research found clear existing patterns and complete information about current implementations.

## Sources

### Primary (HIGH confidence)
- `internal/api/timeline_service.go` - Existing service implementation pattern
- `internal/api/handlers/*.go` - Current handler implementations with business logic
- `internal/mcp/tools/*.go` - MCP tool implementations with HTTP calls
- `internal/mcp/client/client.go` - HTTP client used by MCP tools
- `internal/graph/client.go` - FalkorDB client interface
- `internal/api/interfaces.go` - QueryExecutor interface definition

### Secondary (MEDIUM confidence)
- `cmd/spectre/commands/server.go` - Service instantiation and wiring patterns
- User decisions in `.planning/phases/07-service-layer-extraction/07-CONTEXT.md`

### Tertiary (LOW confidence)
- None - all findings verified with codebase

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - All dependencies already in use
- Architecture: HIGH - Existing TimelineService provides clear pattern
- Pitfalls: HIGH - Common service extraction issues are well-known
- Operations inventory: HIGH - Complete code review of handlers and tools

**Research date:** 2026-01-21
**Valid until:** Estimate 60 days (stable architecture, low churn expected)
