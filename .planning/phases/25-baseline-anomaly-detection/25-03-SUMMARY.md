---
phase: 25-baseline-anomaly-detection
plan: 03
subsystem: database
tags: [falkordb, cypher, graph, baseline, syncer, rate-limiting]

# Dependency graph
requires:
  - phase: 25-01
    provides: SignalBaseline type and RollingStats computation
  - phase: 25-02
    provides: AnomalyScore type for anomaly detection
  - phase: 24-03
    provides: SignalAnchor graph storage with composite key
provides:
  - FalkorDB MERGE upsert for SignalBaseline nodes
  - HAS_BASELINE relationship linking SignalAnchor to SignalBaseline
  - BaselineCollector syncer with 5-minute interval
  - Rate-limited Grafana API queries (10 req/sec)
affects: [25-04, 26]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - MERGE ON CREATE/ON MATCH for idempotent upsert
    - Welford's online algorithm for incremental statistics
    - Ticker-based sync loop with graceful shutdown

key-files:
  created:
    - internal/integration/grafana/signal_baseline_store.go
    - internal/integration/grafana/signal_baseline_store_test.go
    - internal/integration/grafana/baseline_collector.go
    - internal/integration/grafana/baseline_collector_test.go
  modified: []

key-decisions:
  - "MERGE with composite key (metric_name + namespace + workload + integration) for idempotent upsert"
  - "HAS_BASELINE relationship direction: SignalAnchor -> SignalBaseline"
  - "Welford's online algorithm for incremental mean/variance updates"
  - "Rate limiting at 100ms interval (10 req/sec) to protect Grafana API"

patterns-established:
  - "Baseline store pattern: UpsertSignalBaseline, GetSignalBaseline, GetBaselinesByWorkload"
  - "BaselineCollector lifecycle: Start/Stop matching AlertStateSyncer"
  - "Incremental statistics: updateBaselineWithSample using Welford's algorithm"

# Metrics
duration: 7min
completed: 2026-01-29
---

# Phase 25 Plan 03: Graph Storage & Forward Collection Summary

**FalkorDB MERGE upsert for SignalBaseline with HAS_BASELINE relationship, BaselineCollector syncer on 5-minute interval with 10 req/sec rate limiting**

## Performance

- **Duration:** 7 min
- **Started:** 2026-01-29T22:48:55Z
- **Completed:** 2026-01-29T22:56:11Z
- **Tasks:** 2
- **Files created:** 4

## Accomplishments
- SignalBaseline MERGE upsert with ON CREATE/ON MATCH semantics
- HAS_BASELINE relationship links SignalAnchor to SignalBaseline
- GetSignalBaseline returns nil, nil when not found (not error)
- GetBaselinesByWorkload with TTL filtering via expires_at
- BaselineCollector with 5-minute sync interval
- Rate limiting via ticker (100ms = 10 req/sec)
- Welford's online algorithm for incremental mean/variance

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement SignalBaseline graph storage** - `072d715` (feat)
2. **Task 2: Implement BaselineCollector syncer** - `b3edd5d` (feat)

## Files Created/Modified
- `internal/integration/grafana/signal_baseline_store.go` - FalkorDB MERGE upsert, GetSignalBaseline, GetBaselinesByWorkload, GetActiveSignalAnchors
- `internal/integration/grafana/signal_baseline_store_test.go` - Unit tests for all store functions
- `internal/integration/grafana/baseline_collector.go` - BaselineCollector with Start/Stop lifecycle, collectAndUpdate, updateBaselineWithSample
- `internal/integration/grafana/baseline_collector_test.go` - Unit tests for collector lifecycle, rate limiting, incremental updates

## Decisions Made
1. **MERGE upsert with composite key** - Same composite key as SignalAnchor (metric_name + namespace + workload + integration) for identity alignment
2. **HAS_BASELINE relationship direction** - SignalAnchor -> SignalBaseline (anchor "has" a baseline)
3. **Not found returns nil, nil** - GetSignalBaseline returns nil, nil when baseline doesn't exist (not an error) for cleaner caller logic
4. **Welford's online algorithm** - Incremental mean/variance updates without storing all samples
5. **Rate limiting at 100ms** - 10 req/sec to protect Grafana API from burst load

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed function name conflict with computeStdDev**
- **Found during:** Task 2 (BaselineCollector implementation)
- **Issue:** statistical_detector.go already defined computeStdDev(values []float64, mean float64)
- **Fix:** Renamed to computeStdDevFromVariance(variance float64, n int) and use math.Sqrt
- **Files modified:** internal/integration/grafana/baseline_collector.go
- **Verification:** Build succeeds, all tests pass
- **Committed in:** b3edd5d (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Necessary to avoid redeclaration error. No scope creep.

## Issues Encountered
- Rate limiting test initially tried to call queryCurrentValue with nil queryService - refactored to test ticker behavior directly

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Graph storage for SignalBaseline complete (BASE-01)
- MERGE upsert semantics working correctly (BASE-01)
- HAS_BASELINE relationship links to SignalAnchor (BASE-01)
- Forward collection runs every 5 minutes (BASE-04)
- Rate limiting prevents API overload (BASE-04)
- Ready for 25-04: Historical Backfill (opt-in catchup mechanism)

---
*Phase: 25-baseline-anomaly-detection*
*Completed: 2026-01-29*
