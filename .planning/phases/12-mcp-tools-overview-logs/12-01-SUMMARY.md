---
phase: 12-mcp-tools-overview-logs
plan: 01
subsystem: integration
tags: [logzio, elasticsearch, secret-management, mcp]

# Dependency graph
requires:
  - phase: 11-secret-file-management
    provides: SecretWatcher for dynamic token management
provides:
  - Logzio integration with factory registration
  - Elasticsearch DSL query builder with .keyword suffix handling
  - X-API-TOKEN authentication via SecretWatcher
  - Regional endpoint support (5 regions)
  - Query validation rejecting leading wildcards
affects: [12-02-mcp-tools-implementation]

# Tech tracking
tech-stack:
  added: [none - reused existing SecretWatcher from victorialogs]
  patterns:
    - Elasticsearch DSL construction with bool queries
    - .keyword suffix for exact-match fields in ES
    - X-API-TOKEN header authentication (not Bearer)
    - Regional endpoint selection via config

key-files:
  created:
    - internal/integration/logzio/logzio.go
    - internal/integration/logzio/types.go
    - internal/integration/logzio/severity.go
    - internal/integration/logzio/client.go
    - internal/integration/logzio/query.go
    - internal/integration/logzio/query_test.go
  modified: []

key-decisions:
  - "Reused victorialogs.SecretWatcher for token management (shared pattern)"
  - "X-API-TOKEN header instead of Authorization: Bearer (Logz.io API requirement)"
  - ".keyword suffix on exact-match fields (kubernetes.namespace.keyword, etc)"
  - "ValidateQueryParams rejects leading wildcards (ES performance protection)"

patterns-established:
  - "Regional endpoint mapping via Config.GetBaseURL()"
  - "Elasticsearch DSL with bool queries and must clauses"
  - "Terms aggregations with size 1000 and _count ordering"
  - "parseLogzioHit normalizes ES _source to common LogEntry schema"

# Metrics
duration: 5min
completed: 2026-01-22
---

# Phase 12 Plan 01: Logzio Integration Bootstrap Summary

**Elasticsearch DSL query builder with X-API-TOKEN authentication, regional endpoints, and SecretWatcher integration**

## Performance

- **Duration:** 5 min
- **Started:** 2026-01-22T14:34:31Z
- **Completed:** 2026-01-22T14:39:34Z
- **Tasks:** 2
- **Files created:** 6

## Accomplishments

- Logzio integration registered with factory system (discoverable as "logzio" type)
- Elasticsearch DSL query builder generating valid queries with .keyword suffixes
- X-API-TOKEN authentication header (not Bearer token per Logz.io API)
- Regional endpoint support (us, eu, uk, au, ca) via Config.GetBaseURL()
- Query validation rejecting leading wildcards for performance protection
- Severity patterns copied from VictoriaLogs (proven across 1000s of logs)
- SecretWatcher lifecycle managed (Start/Stop) for dynamic token rotation

## Task Commits

Each task was committed atomically:

1. **Task 1: Create Logzio integration skeleton** - `4a9274f` (feat)
   - Factory registration in init()
   - NewLogzioIntegration with config validation
   - Start/Stop lifecycle with SecretWatcher
   - Health check with SecretWatcher validation
   - Config types with regional endpoint mapping
   - Severity patterns (ErrorPattern, WarningPattern)

2. **Task 2: Implement Elasticsearch DSL query builder** - `91d35af` (feat)
   - Client with QueryLogs and QueryAggregation
   - X-API-TOKEN header authentication
   - BuildLogsQuery with bool query structure
   - BuildAggregationQuery with terms aggregation
   - ValidateQueryParams rejecting leading wildcards
   - Comprehensive test suite (10 tests, all passing)

## Files Created/Modified

**Created:**
- `internal/integration/logzio/logzio.go` - Integration lifecycle, factory registration, SecretWatcher management
- `internal/integration/logzio/types.go` - Config with regional endpoints, QueryParams, LogEntry, response types
- `internal/integration/logzio/severity.go` - Error/warning patterns (copied from VictoriaLogs)
- `internal/integration/logzio/client.go` - HTTP client with X-API-TOKEN auth, QueryLogs/QueryAggregation methods
- `internal/integration/logzio/query.go` - Elasticsearch DSL builders (BuildLogsQuery, BuildAggregationQuery, ValidateQueryParams)
- `internal/integration/logzio/query_test.go` - Test suite with 10 tests covering query structure, filters, validation

**Modified:** None

## Decisions Made

**1. Reused victorialogs.SecretWatcher for token management**
- **Rationale:** SecretWatcher is integration-agnostic, handles token rotation and lifecycle correctly
- **Benefit:** No code duplication, proven reliability from Phase 11
- **Implementation:** Import victorialogs.SecretWatcher in logzio package, use same lifecycle pattern

**2. X-API-TOKEN header instead of Authorization: Bearer**
- **Rationale:** Logz.io API explicitly requires X-API-TOKEN header (documented in Phase 12 research)
- **CRITICAL:** Added comments warning against Bearer token to prevent future mistakes
- **Verification:** grep confirms no Bearer pattern in code (only warning comments)

**3. .keyword suffix on exact-match fields**
- **Rationale:** Elasticsearch requires .keyword suffix for exact matching on text fields
- **Applied to:** kubernetes.namespace, kubernetes.pod_name, kubernetes.container_name, level
- **Not applied to:** @timestamp (date type), message (regexp uses base field)
- **Verification:** Tests confirm .keyword suffix present in generated queries

**4. ValidateQueryParams purpose clarified**
- **Purpose:** Validates internal regex patterns used by overview tool for severity detection (GetErrorPattern, GetWarningPattern)
- **Not for users:** logs tool doesn't expose regex field to users (Plan 02 context)
- **Protection:** Rejects leading wildcards (*prefix, ?prefix) for ES performance
- **Max limit:** Enforces 500 max (but Plan 02 tools will use 100)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - implementation followed VictoriaLogs reference patterns exactly.

## Test Coverage

- **Query builder tests:** 10 tests covering all scenarios
- **Coverage:** 20.8% (focused on query.go logic)
- **All tests passing:** Query structure, filters, time ranges, aggregations, validation

**Test categories:**
1. Basic query structure (size, sort, bool query)
2. Filters with .keyword suffixes (namespace, pod, container, level)
3. Time range RFC3339 formatting
4. Regexp clause with case_insensitive flag
5. Aggregation with terms, size 1000, _count ordering
6. Leading wildcard validation (rejects *prefix, ?prefix)
7. Max limit enforcement (500)

## Next Phase Readiness

**Ready for Plan 02 (MCP Tools Implementation):**
- Client.QueryLogs ready for logs tool
- Client.QueryAggregation ready for overview tool
- Config.GetBaseURL provides regional endpoints
- SecretWatcher provides dynamic token rotation
- ValidateQueryParams protects against leading wildcards in severity patterns

**No blockers or concerns.**

---
*Phase: 12-mcp-tools-overview-logs*
*Completed: 2026-01-22*
