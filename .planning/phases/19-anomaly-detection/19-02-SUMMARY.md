---
phase: 19-anomaly-detection
plan: 02
subsystem: metrics
tags: [grafana, falkordb, caching, baseline, anomaly-detection]

# Dependency graph
requires:
  - phase: 19-01
    provides: Baseline type and statistical detector
provides:
  - Graph-backed baseline cache with TTL
  - FalkorDB storage for computed baselines
  - Weekday/weekend context-aware caching
affects: [19-03-baseline-computation, 19-04-integration]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "FalkorDB-based caching with TTL via expires_at timestamp"
    - "MERGE upsert pattern for cache storage"
    - "Weekday/weekend separation for time-of-day baselines"

key-files:
  created:
    - internal/integration/grafana/baseline_cache.go
  modified: []

key-decisions:
  - "TTL implementation via expires_at Unix timestamp in graph (no application-side cleanup)"
  - "Weekday/weekend separation for different baseline patterns"
  - "MERGE-based upsert semantics following Phase 16 pattern"

patterns-established:
  - "Cache queries filter by expires_at > now in WHERE clause"
  - "1-hour granularity baselines stored per metric, hour, day-type"

# Metrics
duration: 2min
completed: 2026-01-23
---

# Phase 19 Plan 02: Baseline Cache Summary

**FalkorDB-backed baseline cache with 1-hour TTL, weekday/weekend separation, and MERGE upsert semantics**

## Performance

- **Duration:** 2 min
- **Started:** 2026-01-23T06:29:23Z
- **Completed:** 2026-01-23T06:31:03Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments
- BaselineCache type with FalkorDB graph storage
- Get method with TTL filtering (WHERE expires_at > now)
- Set method using MERGE for upsert semantics
- Weekday/weekend day-type classification
- Helper functions for time handling (getDayType, isWeekend)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create baseline cache with FalkorDB storage** - `54c3628` (feat)

## Files Created/Modified
- `internal/integration/grafana/baseline_cache.go` - Graph-backed baseline cache with Get/Set methods, TTL support via expires_at timestamp, weekday/weekend separation

## Decisions Made

**TTL Implementation Strategy**
- Store expires_at as Unix timestamp (int64) in graph
- Filter expired baselines in WHERE clause, not application-side
- FalkorDB handles timestamp comparison efficiently
- Follows pattern from RESEARCH.md analysis

**Weekday/Weekend Separation**
- Different baseline patterns for weekends vs weekdays
- getDayType helper returns "weekend" or "weekday"
- Stored as day_type field in Baseline node

**MERGE Upsert Semantics**
- Follows Phase 16 decision for consistent pattern
- Creates or updates baseline nodes atomically
- Composite key: metric_name + window_hour + day_type

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Ready for Phase 19 Plan 03 (baseline computation).

**What's ready:**
- Cache infrastructure complete
- Get/Set methods ready for integration
- TTL filtering operational
- Weekday/weekend context handling in place

**What's next:**
- Baseline computation logic (19-03)
- Integration with anomaly detector (19-04)

---
*Phase: 19-anomaly-detection*
*Completed: 2026-01-23*
