# Phase 7: Service Layer Extraction - Context

**Gathered:** 2026-01-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Extract shared service interfaces (TimelineService, GraphService, SearchService, MetadataService) so REST handlers and MCP tools call common in-process methods. Eliminates MCP tools' HTTP self-calls to localhost. Does NOT add new functionality — restructures existing code for shared access.

</domain>

<decisions>
## Implementation Decisions

### Service Boundaries
- **TimelineService:** Full timeline operations (queries + any mutations)
- **GraphService:** Separate service for all FalkorDB queries (neighbors, paths, traversals)
- **SearchService:** Dedicated service for unified search across VictoriaLogs + FalkorDB
- **MetadataService:** Just resource metadata (labels, annotations, timestamps, resource info lookups) — search stays in SearchService

### Interface Design
- **Error handling:** Domain error types (NotFoundError, ValidationError, etc.) that callers map to HTTP status codes or gRPC codes
- **Context propagation:** Only methods that do I/O or long operations take context.Context as first parameter
- **Method signatures:** One method per operation (granular: GetTimeline, QueryGraph, SearchLogs)
- **Package location:** Interfaces defined alongside implementations in internal/api (not a separate services package)

### Migration Strategy
- **Order:** REST handlers refactored first, then MCP tools wired to use the extracted services
- **Structure:** One service at a time — complete TimelineService, then GraphService, then SearchService, then MetadataService
- **Transition:** Delete HTTP self-call code immediately as each service is wired up (no feature flag toggle)
- **Service priority:** Timeline → Graph → Search → Metadata

### Dependency Injection
- **Pattern:** Constructor injection (NewTimelineService(graphClient, logger, tracer))
- **Registry:** No central container — each handler/tool receives only the services it needs
- **Service coupling:** Flat hierarchy — services only depend on infrastructure (clients, loggers), not each other

### Claude's Discretion
- Where service instantiation happens (cmd/spectre vs internal/apiserver)
- Exact method names and signatures for each service
- Internal implementation details within each service

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard Go service patterns.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 07-service-layer-extraction*
*Context gathered: 2026-01-21*
