---
phase: 25-baseline-anomaly-detection
plan: 02
subsystem: anomaly-detection
tags: [z-score, percentile, statistics, anomaly-scoring, alert-override, tdd]

# Dependency graph
requires:
  - phase: 25-baseline-anomaly-detection
    plan: 01
    provides: SignalBaseline type with rolling statistics
provides:
  - AnomalyScore type with Score, Confidence, Method, ZScore fields
  - ComputeAnomalyScore function (hybrid z-score + percentile)
  - ApplyAlertOverride function for alert state integration
  - Cold start handling via InsufficientSamplesError
affects: [25-03-baseline-store, 25-04-baseline-collector, 25-05-anomaly-aggregator, 26-observatory-api]

# Tech tracking
tech-stack:
  added: []
  patterns: [hybrid-anomaly-scoring, sigmoid-normalization, max-aggregation, alert-override]

key-files:
  created:
    - internal/integration/grafana/anomaly_scorer.go
    - internal/integration/grafana/anomaly_scorer_test.go
  modified: []

key-decisions:
  - "Z-score normalized via sigmoid: 1 - exp(-|z|/2) maps to 0-1 range"
  - "Percentile score starts at 0.5 for P99 boundary, scales linearly"
  - "Final score = MAX(zScoreNormalized, percentileScore) per CONTEXT.md"
  - "Confidence = MIN(sampleConfidence, qualityScore) per CONTEXT.md"
  - "Alert firing overrides to score=1.0, confidence=1.0"

patterns-established:
  - "Hybrid anomaly scoring: combine multiple methods with MAX aggregation"
  - "Sigmoid normalization for unbounded values to 0-1 range"
  - "Alert state as definitive signal (not probabilistic)"

# Metrics
duration: 2.5min
completed: 2026-01-29
---

# Phase 25 Plan 02: Hybrid Anomaly Scorer Summary

**Z-score + percentile hybrid anomaly scoring with sigmoid normalization, confidence weighting, and Grafana alert override**

## Performance

- **Duration:** 2.5 min
- **Started:** 2026-01-29T22:42:24Z
- **Completed:** 2026-01-29T22:44:51Z
- **Tasks:** 2 (TDD: RED + GREEN)
- **Files created:** 2

## Accomplishments

- Implemented ComputeAnomalyScore with hybrid z-score + percentile comparison
- Z-score normalized to 0-1 using sigmoid formula: 1 - exp(-|z|/2)
- Percentile method detects values above P99 or below Min
- Final score uses MAX of both methods (per CONTEXT.md)
- Cold start returns InsufficientSamplesError for < 10 samples
- ApplyAlertOverride sets score=1.0 for firing alerts
- 18 comprehensive TDD tests covering all scoring paths

## Task Commits

Each task was committed atomically (TDD cycle):

1. **RED: Add failing tests for anomaly scoring** - `0948894` (test)
   - 18 test cases covering z-score, percentile, hybrid, confidence, cold start, alert override
2. **GREEN: Implement hybrid anomaly scoring** - `0917225` (feat)
   - AnomalyScore type and ComputeAnomalyScore/ApplyAlertOverride functions

_No refactoring needed - implementation was clean on first pass_

## Files Created

- `internal/integration/grafana/anomaly_scorer.go` - Core anomaly scoring functions (148 lines)
- `internal/integration/grafana/anomaly_scorer_test.go` - TDD tests (427 lines)

## Decisions Made

1. **Sigmoid normalization formula:** Used `1.0 - exp(-|z|/2.0)` for smooth mapping:
   - z=0 -> 0.0 (normal)
   - z=2 -> ~0.63
   - z=3 -> ~0.78
   - z->infinity -> 1.0

2. **Percentile scoring:** Score starts at 0.5 at P99 boundary, scales linearly with distance:
   - excess = currentValue - P99
   - score = 0.5 + (excess / (P99-P50)) * 0.5

3. **Hybrid aggregation:** MAX of both methods ensures anomaly is flagged if EITHER method detects it

4. **Confidence formula:** `sampleConfidence = min(1.0, 0.5 + (sampleCount-10)/180.0)`
   - 10 samples -> 0.5 confidence
   - 190 samples -> 1.0 confidence
   - Final confidence capped by dashboard quality score

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- AnomalyScore type ready for use by baseline collector (25-04)
- ComputeAnomalyScore ready for integration with graph storage (25-03)
- ApplyAlertOverride ready for alert state integration
- All requirements met: ANOM-01 (z-score), ANOM-02 (percentile), ANOM-03 (confidence), ANOM-04 (cold start), ANOM-06 (alert override)

---
*Phase: 25-baseline-anomaly-detection*
*Completed: 2026-01-29*
