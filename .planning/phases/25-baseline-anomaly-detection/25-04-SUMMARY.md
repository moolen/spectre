---
phase: 25-baseline-anomaly-detection
plan: 04
subsystem: integration
tags: [baseline, backfill, anomaly, aggregation, grafana, hierarchical]

# Dependency graph
requires:
  - phase: 25-01
    provides: SignalBaseline types and RollingStatistics computation
  - phase: 25-02
    provides: ComputeAnomalyScore hybrid scorer with alert override
  - phase: 25-03
    provides: SignalBaseline graph storage with HAS_BASELINE relationship
provides:
  - Historical backfill service for 7-day baseline data (BASE-05)
  - Alert threshold bootstrapping support (BASE-06)
  - Hierarchical anomaly aggregation (signal -> workload -> namespace -> cluster)
  - MAX aggregation for anomaly scores (ANOM-05)
  - Quality tiebreaker for equal anomaly scores
  - TTL-based aggregation cache with jitter
affects:
  - 26-observatory (will use anomaly aggregation for tools)
  - future alert threshold integration

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Rate-limited backfill (2 req/sec) separate from forward collection
    - Hierarchical aggregation with MAX score / MIN confidence
    - Cache with TTL + jitter to prevent thundering herd

key-files:
  created:
    - internal/integration/grafana/baseline_backfill.go
    - internal/integration/grafana/baseline_backfill_test.go
    - internal/integration/grafana/anomaly_aggregator.go
    - internal/integration/grafana/anomaly_aggregator_test.go
  modified: []

key-decisions:
  - "BackfillService rate limiting at 2 req/sec (slower than forward collection at 10 req/sec)"
  - "MAX aggregation for anomaly scores per CONTEXT.md ('worst signal')"
  - "MIN aggregation for confidence (most uncertain signal limits overall confidence)"
  - "Quality score as tiebreaker when anomaly scores equal"
  - "5-minute cache TTL with 0-30s random jitter"

patterns-established:
  - "Hierarchical aggregation: signal -> workload -> namespace -> cluster"
  - "AggregationCache pattern for expensive computations"

# Metrics
duration: 11min
completed: 2026-01-29
---

# Phase 25 Plan 04: Historical Backfill & Anomaly Aggregation Summary

**BackfillService for 7-day historical baselines (2 req/sec), AnomalyAggregator for hierarchical MAX score rollup with TTL cache**

## Performance

- **Duration:** 11 min
- **Started:** 2026-01-29T22:49:14Z
- **Completed:** 2026-01-29T23:00:02Z
- **Tasks:** 2
- **Files modified:** 4 (created)

## Accomplishments
- BackfillService fetches 7 days of historical data for new signals (BASE-05)
- Alert threshold bootstrapping checks for associated alerts (BASE-06)
- Hierarchical anomaly aggregation with MAX scores (ANOM-05)
- Quality tiebreaker ensures deterministic TopSource selection
- Aggregation cache prevents redundant computation

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement BackfillService for historical baseline** - `845526f` (feat)
2. **Task 2: Implement hierarchical anomaly aggregation** - `8a32b2e` (feat)

## Files Created/Modified

- `internal/integration/grafana/baseline_backfill.go` - BackfillService with 7-day backfill, rate limiting, alert threshold check
- `internal/integration/grafana/baseline_backfill_test.go` - 7 tests for backfill functionality
- `internal/integration/grafana/anomaly_aggregator.go` - AnomalyAggregator with hierarchical rollup and cache
- `internal/integration/grafana/anomaly_aggregator_test.go` - 9 tests for aggregation behavior

## Decisions Made

1. **Rate limiting at 2 req/sec** - Backfill is slower than forward collection (10 req/sec) to protect Grafana API during bulk operations
2. **MAX aggregation for scores** - Per CONTEXT.md, the "worst signal" anomaly bubbles up through hierarchy
3. **MIN aggregation for confidence** - Most uncertain signal determines overall confidence
4. **Quality tiebreaker** - When anomaly scores are equal, higher quality signal becomes TopSource
5. **5-minute cache TTL with jitter** - Prevents thundering herd while keeping results reasonably fresh

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed computeStdDev redeclaration conflict**
- **Found during:** Task 1 (initial build attempt)
- **Issue:** `computeStdDev` was declared in both `baseline_collector.go` and `statistical_detector.go` with different signatures
- **Fix:** Renamed `baseline_collector.go` version to `computeStdDevFromVariance` since it takes variance as input
- **Files modified:** internal/integration/grafana/baseline_collector.go
- **Verification:** Build succeeds, all tests pass
- **Committed in:** Pre-existing file was already committed

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Required to unblock build. No scope creep.

## Issues Encountered

- Mock graph client in tests needed careful query matching - query strings start with whitespace due to multi-line template literals

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 25 is now complete with all baseline and anomaly detection components
- Phase 26 (Observatory API and MCP tools) can now begin
- All foundation pieces ready: SignalAnchor, SignalBaseline, anomaly scoring, aggregation

---
*Phase: 25-baseline-anomaly-detection*
*Completed: 2026-01-29*
