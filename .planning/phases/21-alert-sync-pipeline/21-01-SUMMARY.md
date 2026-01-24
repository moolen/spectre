---
phase: 21-alert-sync-pipeline
plan: 01
subsystem: api
tags: [grafana, alerting, graph, state-tracking, falkordb]

# Dependency graph
requires:
  - phase: 20-alert-api-client
    provides: Alert node schema, GraphBuilder, AlertSyncer patterns
provides:
  - GetAlertStates API method to fetch current alert states from Grafana
  - CreateStateTransitionEdge method with 7-day TTL via expires_at property
  - getLastKnownState method for state deduplication
  - Prometheus-compatible alert state types (AlertState, AlertInstance)
affects: [21-02, alert-state-sync, state-tracking, temporal-queries]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "TTL via expires_at RFC3339 timestamp with WHERE filtering (no cleanup job)"
    - "Self-edge pattern for state transitions: (Alert)-[STATE_TRANSITION]->(Alert)"
    - "Return 'unknown' for missing state (not error) to handle first sync gracefully"
    - "MERGE for Alert node in state sync to handle race with rule sync"

key-files:
  created: []
  modified:
    - internal/integration/grafana/client.go
    - internal/integration/grafana/graph_builder.go

key-decisions:
  - "Prometheus-compatible /api/prometheus/grafana/api/v1/rules endpoint for alert states"
  - "7-day TTL calculated from timestamp (168 hours) using RFC3339 format"
  - "State deduplication via lastKnownState comparison (caller responsibility)"
  - "Map 'alerting' to 'firing' state, normalize to lowercase"
  - "Extract UID from grafana_uid label in Prometheus response"

patterns-established:
  - "TTL filtering: WHERE t.expires_at > $now in Cypher queries"
  - "Self-edges model state transitions: (a)-[STATE_TRANSITION]->(a)"
  - "getLastKnownState returns 'unknown' for missing state (not error)"
  - "Integration field in all Alert queries for multi-Grafana support"

# Metrics
duration: 4min
completed: 2026-01-23
---

# Phase 21 Plan 01: Alert State API & Graph Foundation Summary

**Alert state fetching via Prometheus-compatible API and graph storage with TTL-based state transitions and deduplication support**

## Performance

- **Duration:** 4 min
- **Started:** 2026-01-23T10:06:33Z
- **Completed:** 2026-01-23T10:10:18Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- GetAlertStates method fetches current alert states from Grafana's Prometheus-compatible endpoint
- CreateStateTransitionEdge stores state transitions as self-edges with 7-day TTL
- getLastKnownState enables state deduplication by retrieving most recent state
- TTL enforcement via expires_at RFC3339 timestamp with query-time filtering

## Task Commits

Each task was committed atomically:

1. **Task 1: Add GetAlertStates API client method** - `daa023e` (feat)
2. **Task 2: Add state transition graph methods with deduplication** - `e7111a6` (feat)

## Files Created/Modified
- `internal/integration/grafana/client.go` - Added GetAlertStates method, AlertState/AlertInstance types, Prometheus response types
- `internal/integration/grafana/graph_builder.go` - Added CreateStateTransitionEdge and getLastKnownState methods

## Decisions Made

**API endpoint selection:**
- Used `/api/prometheus/grafana/api/v1/rules` (Prometheus-compatible format) instead of provisioning API
- Provides alert rules WITH instances in single call (more efficient than separate requests)

**State normalization:**
- Map Grafana "alerting" state to "firing" for consistency with Prometheus terminology
- Normalize all states to lowercase for consistent comparison

**UID extraction:**
- Extract alert UID from `grafana_uid` label in Prometheus response
- Skip rules without UID (not Grafana-managed alerts)

**TTL implementation:**
- 7-day retention via expires_at timestamp property (matches Phase 19 baseline cache pattern)
- RFC3339 string format for timestamp comparison in Cypher queries
- No cleanup job needed - filter expired edges in queries: `WHERE t.expires_at > $now`

**State deduplication approach:**
- getLastKnownState returns "unknown" (not error) when no previous state exists
- Enables graceful handling of first sync (no prior state is valid scenario)
- Caller compares current vs last state to decide if transition should be created

**Multi-Grafana support:**
- Include integration field in Alert node matching for all queries
- Enables multiple Grafana instances to track state independently

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

**Ready for Plan 21-02 (Alert State Syncer):**
- API method available to fetch current alert states
- Graph methods ready to store state transitions
- TTL and deduplication logic in place
- Pattern established: self-edges with expires_at property

**Foundation complete:**
- Alert state types defined with JSON mapping for Prometheus format
- State transition edge creation with 7-day TTL
- Last known state query with expired edge filtering
- MERGE pattern handles race with rule sync (Alert node may not exist yet)

**No blockers.** Implementation follows established patterns from Phase 19 (baseline cache TTL) and Phase 20 (Alert sync).

---
*Phase: 21-alert-sync-pipeline*
*Completed: 2026-01-23*
