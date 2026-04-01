---
phase: 25-baseline-anomaly-detection
plan: 05
subsystem: testing, integration
tags: [baseline, anomaly, integration-test, lifecycle, grafana]

# Dependency graph
requires:
  - phase: 25-01
    provides: SignalBaseline types, RollingStatistics
  - phase: 25-02
    provides: AnomalyScorer with z-score + percentile hybrid
  - phase: 25-03
    provides: SignalBaselineStore, BaselineCollector
  - phase: 25-04
    provides: BackfillService, AnomalyAggregator
provides:
  - End-to-end integration test suite for baseline storage
  - BaselineCollector wired into Grafana integration lifecycle
  - Test coverage for cold start, alert override, aggregation, TTL
affects: [26-observatory-api, mcp-tools]

# Tech tracking
tech-stack:
  added: [testify/assert, testify/require]
  patterns: [mock graph client for integration tests, lifecycle test pattern]

key-files:
  created:
    - internal/integration/grafana/baseline_integration_test.go
  modified:
    - internal/integration/grafana/grafana.go

key-decisions:
  - "BaselineCollector lifecycle follows AlertStateSyncer pattern"
  - "Non-fatal collector start failure - warns but continues"
  - "Collector stopped before stateSyncer in shutdown sequence"

patterns-established:
  - "Integration test with mock graph client handling multiple query patterns"
  - "Query pattern ordering in mocks - specific patterns before general"

# Metrics
duration: 8min
completed: 2026-01-30
---

# Phase 25 Plan 05: Integration Test & Lifecycle Summary

**End-to-end integration test suite (11 tests) verifying BaselineCollector lifecycle, anomaly scoring, hierarchical aggregation, cold start, alert override, and TTL filtering**

## Performance

- **Duration:** 8 min
- **Started:** 2026-01-30T00:05:00Z
- **Completed:** 2026-01-30T00:13:00Z
- **Tasks:** 3 (2 auto + 1 checkpoint)
- **Files modified:** 2

## Accomplishments

- BaselineCollector wired into Grafana integration lifecycle (start/stop with integration)
- Comprehensive integration test suite covering all baseline/anomaly functionality
- 11 test cases passing with race detector enabled
- Test file: 947 lines with mock graph client supporting all query patterns

## Task Commits

Each task was committed atomically:

1. **Task 1: Wire BaselineCollector into Grafana integration lifecycle** - `20d082f` (feat)
2. **Task 2: Create end-to-end integration test** - `0d18570` (test)
3. **Task 3: Human verification checkpoint** - approved

**Plan metadata:** (pending)

## Files Created/Modified

- `internal/integration/grafana/grafana.go` - Added baselineCollector field, Start/Stop lifecycle
- `internal/integration/grafana/baseline_integration_test.go` - 947-line integration test suite

## Test Coverage

| Test | Purpose |
|------|---------|
| TestBaselineIntegration_EndToEnd | Full pipeline: SignalAnchor -> backfill -> SignalBaseline |
| TestBaselineIntegration_AnomalyDetection | Z-score scoring with established baseline |
| TestBaselineIntegration_ColdStart | InsufficientSamplesError handling |
| TestBaselineIntegration_AlertOverride | Alert firing overrides to score=1.0 |
| TestBaselineIntegration_HierarchicalAggregation | MAX aggregation across signals |
| TestBaselineIntegration_TTLExpiration | Expired baselines filtered |
| TestBaselineIntegration_CollectorLifecycle | Start/stop without panic |
| TestBaselineIntegration_RollingStatistics | Statistical computation (4 subtests) |
| TestBaselineIntegration_InsufficientSamplesError | Error interface |
| TestBaselineIntegration_ZScoreNormalization | 0-1 mapping (4 subtests) |
| TestBaselineIntegration_ConfidenceCalculation | Quality caps (3 subtests) |

## Decisions Made

- **BaselineCollector lifecycle follows AlertStateSyncer pattern**: Start after alert analysis service, stop before stateSyncer
- **Non-fatal collector start failure**: Logs warning but continues - anomaly detection still works with existing baselines
- **Collector stopped first in shutdown**: Depends on query service and graph client, so stopped before they're cleared

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- **Mock query pattern ordering**: The mock graph client's `GetActiveSignalAnchors` check was matching the AnomalyAggregator's query before the `HAS_BASELINE` check could run. Fixed by reordering checks: more specific patterns (OPTIONAL MATCH + HAS_BASELINE) before general patterns.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 25 COMPLETE: All baseline storage and anomaly detection functionality implemented and tested
- Ready for Phase 26: Observatory API and MCP tools
- All 12 phase 25 requirements satisfied (BASE-01 through BASE-06, ANOM-01 through ANOM-06)

**Pre-existing issue noted:** `TestComputeDashboardQuality_Freshness` has time-dependent failures unrelated to baseline integration. This is not a regression from this plan.

---
*Phase: 25-baseline-anomaly-detection*
*Completed: 2026-01-30*
