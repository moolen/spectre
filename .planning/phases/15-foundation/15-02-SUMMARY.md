---
phase: 15-foundation
plan: 02
subsystem: database
tags: [falkordb, graph, grafana, dashboard, cypher]

# Dependency graph
requires:
  - phase: 15-01
    provides: Grafana API client and integration factory
provides:
  - Dashboard node schema in FalkorDB with uid-based indexing
  - Named graph database management (create/delete/exists)
  - UpsertDashboardNode function for idempotent dashboard storage
affects: [15-03, 16-dashboard-ingestion]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Named graph databases: spectre_grafana_{name} convention"
    - "Dashboard node with MERGE-based upsert (ON CREATE/ON MATCH SET)"

key-files:
  created: []
  modified:
    - internal/graph/schema.go
    - internal/graph/models.go
    - internal/graph/client.go
    - internal/graph/cached_client.go

key-decisions:
  - "Index only on Dashboard.uid for Phase 15 (folder/tags indexes deferred to Phase 16)"
  - "Named graph convention: spectre_grafana_{integration_name} for isolation"
  - "Dashboard nodes store tags as JSON string (array serialization)"

patterns-established:
  - "Multiple isolated graph databases per integration instance"
  - "Dashboard MERGE pattern with firstSeen/lastSeen timestamps"

# Metrics
duration: 3min
completed: 2026-01-22
---

# Phase 15 Plan 02: Graph Schema for Dashboards Summary

**FalkorDB schema supports Dashboard nodes with uid-based indexing and isolated graph databases per Grafana integration instance**

## Performance

- **Duration:** 3 min
- **Started:** 2026-01-22T20:15:35Z
- **Completed:** 2026-01-22T20:17:53Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Dashboard node schema with uid, title, version, tags, folder, URL, and timestamps
- Index on Dashboard.uid for efficient lookup
- Named graph database support (CreateGraph, DeleteGraphByName, GraphExists)
- UpsertDashboardNode function with idempotent MERGE queries

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Dashboard Node Schema** - `4200ad5` (feat)
2. **Task 2: Add Named Graph Management** - `460e57a` (feat)
3. **Fix: CachedClient interface compliance** - `3005845` (fix)

## Files Created/Modified
- `internal/graph/schema.go` - Added UpsertDashboardNode function with MERGE query using ON CREATE/MATCH SET clauses
- `internal/graph/models.go` - Added DashboardNode struct and NodeTypeDashboard constant
- `internal/graph/client.go` - Added Dashboard index creation, CreateGraph, DeleteGraphByName, GraphExists methods
- `internal/graph/cached_client.go` - Added graph management method delegates to satisfy Client interface

## Decisions Made

**1. Index strategy for Dashboard nodes**
- Start with index only on uid (primary lookup)
- Defer folder and tags indexes to Phase 16 if query performance requires
- Rationale: Research recommendation - optimize for actual query patterns seen in production

**2. Named graph database convention**
- Pattern: `spectre_grafana_{integration_name}`
- Example: "grafana-prod" → graph "spectre_grafana_prod"
- Rationale: Avoid data collision between integration instances, enable clean deletion

**3. Tags serialization**
- Store tags as JSON string array in graph
- Deserialize when needed for filtering
- Rationale: Follow existing pattern from ResourceIdentity labels field

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] CachedClient missing new interface methods**
- **Found during:** Task 2 (Build verification)
- **Issue:** CachedClient wrapper didn't implement CreateGraph, DeleteGraphByName, GraphExists from Client interface
- **Fix:** Added delegate methods to CachedClient that pass through to underlying client, clearing cache on DeleteGraphByName
- **Files modified:** internal/graph/cached_client.go
- **Verification:** `go build ./internal/graph/...` succeeds
- **Committed in:** 3005845 (separate fix commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Essential fix for interface compliance. No scope creep.

## Issues Encountered
None

## Next Phase Readiness
- Graph schema ready for dashboard ingestion in Phase 16
- Named graph management enables multiple Grafana integration instances
- Index on Dashboard.uid provides efficient lookup foundation

**Blockers:** None

**Concerns:** None

---
*Phase: 15-foundation*
*Completed: 2026-01-22*
