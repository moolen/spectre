---
phase: 19-anomaly-detection
plan: 01
subsystem: metrics
tags: [statistics, z-score, anomaly-detection, grafana, tdd]

# Dependency graph
requires:
  - phase: 18-query-execution
    provides: Query service foundation for metrics
provides:
  - Statistical detector with z-score anomaly detection
  - Baseline data structures (Baseline, MetricAnomaly)
  - Error metric classification with lower thresholds
affects: [19-02-baseline-computation, 19-03-anomaly-mcp-tools]

# Tech tracking
tech-stack:
  added: [math stdlib for statistical functions]
  patterns: [TDD red-green-refactor, metric-aware thresholds, sample variance]

key-files:
  created:
    - internal/integration/grafana/baseline.go
    - internal/integration/grafana/statistical_detector.go
    - internal/integration/grafana/statistical_detector_test.go
  modified: []

key-decisions:
  - "Sample variance (n-1) for standard deviation computation"
  - "Error metrics use lower thresholds (2σ critical vs 3σ for normal metrics)"
  - "Absolute z-score for bidirectional anomaly detection"
  - "Pattern-based error metric detection (5xx, error, failed, failure)"

patterns-established:
  - "TDD cycle with RED (failing test) → GREEN (implement) → REFACTOR commits"
  - "Edge case handling (empty slice, zero stddev, single value)"
  - "Metric-aware thresholds based on metric semantics"

# Metrics
duration: 2min
completed: 2026-01-23
---

# Phase 19 Plan 01: Statistical Detector Summary

**Z-score anomaly detection with metric-aware severity thresholds and full TDD test coverage**

## Performance

- **Duration:** 2 min
- **Started:** 2026-01-23T06:25:16Z
- **Completed:** 2026-01-23T06:27:22Z
- **Tasks:** 1 (TDD task with 2 commits)
- **Files modified:** 3

## Accomplishments

- Statistical functions (mean, stddev, z-score) with mathematical correctness
- Metric-aware severity classification (2σ for errors, 3σ for normal metrics)
- Comprehensive edge case handling (empty data, zero variance, single values)
- Full test coverage with 402 test lines covering all functions
- TDD red-green-refactor cycle successfully executed

## Task Commits

TDD task produced 2 atomic commits:

1. **Task 1 RED: Write failing tests** - `ab0d01f` (test)
   - Created baseline.go with Baseline and MetricAnomaly types
   - Created statistical_detector_test.go with comprehensive test cases
   - Created stub statistical_detector.go with zero-value returns
   - All tests failing as expected

2. **Task 1 GREEN: Implement to pass** - `1e9becb` (feat)
   - Implemented computeMean with empty slice handling
   - Implemented computeStdDev using sample variance (n-1)
   - Implemented computeZScore with zero stddev protection
   - Implemented isErrorRateMetric with pattern matching
   - Implemented classifySeverity with metric-aware thresholds
   - Implemented Detect end-to-end method
   - All tests passing

_REFACTOR phase skipped - no refactoring needed, code already clean_

## Files Created/Modified

- `internal/integration/grafana/baseline.go` - Baseline and MetricAnomaly data structures
- `internal/integration/grafana/statistical_detector.go` - Statistical functions and detector implementation
- `internal/integration/grafana/statistical_detector_test.go` - Comprehensive test suite with 402 lines

## Decisions Made

**Sample variance (n-1) formula**
- Used sample variance rather than population variance for more conservative estimates
- Appropriate for historical baseline data which is a sample of population

**Error metrics use lower thresholds**
- Critical: 2σ for errors vs 3σ for normal metrics
- Rationale: Errors are more sensitive - even 2σ spike deserves attention
- Pattern matching: "5xx", "error", "failed", "failure" (case-insensitive)

**Absolute z-score for thresholds**
- Both positive (spikes) and negative (drops) deviations are anomalous
- CPU dropping to zero is as interesting as CPU spiking

**Zero stddev protection**
- Return z-score of 0.0 when stddev is 0 (constant baseline)
- Prevents division by zero, semantically correct (no deviation from constant)

## Deviations from Plan

None - plan executed exactly as written. TDD cycle completed successfully.

## Issues Encountered

None - implementation straightforward, all tests passed on first GREEN implementation.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Statistical foundation complete and fully tested. Ready for:
- **19-02**: Baseline computation from historical metrics
- **19-03**: MCP tools for anomaly detection queries

Key exports available:
- `StatisticalDetector` with `Detect()` method
- `Baseline` type for storing statistical baselines
- `MetricAnomaly` type for anomaly results
- All statistical functions package-private for focused API

---
*Phase: 19-anomaly-detection*
*Completed: 2026-01-23*
