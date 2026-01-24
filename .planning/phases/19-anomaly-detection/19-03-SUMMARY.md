---
phase: 19-anomaly-detection
plan: 03
subsystem: metrics
tags: [grafana, anomaly-detection, z-score, statistical-analysis, baseline-cache, time-series]

# Dependency graph
requires:
  - phase: 19-01
    provides: StatisticalDetector with z-score computation and severity thresholds
  - phase: 19-02
    provides: BaselineCache with TTL and weekday/weekend separation
  - phase: 18-01
    provides: GrafanaQueryService with ExecuteDashboard method
provides:
  - AnomalyService orchestrating detection flow (fetch metrics, compute/retrieve baselines, detect, rank)
  - 7-day historical baseline computation with time-of-day matching
  - Overview tool integration with anomaly detection and minimal context response
affects: [19-04]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Anomaly detection orchestration with graceful degradation
    - Minimal context responses (only essential anomaly fields)
    - Historical data parsing from DataFrame.Data.Values arrays

key-files:
  created:
    - internal/integration/grafana/anomaly_service.go
  modified:
    - internal/integration/grafana/tools_metrics_overview.go
    - internal/integration/grafana/grafana.go

key-decisions:
  - DataFrame parsing clarification: ExecuteDashboard returns time-series data in Values arrays, not single snapshots
  - Metric name extraction via __name__ label with fallback to label pair construction
  - Omit dashboard results when anomalies found (minimal context optimization)
  - Run anomaly detection on first dashboard only (primary overview dashboard)

patterns-established:
  - "AnomalyService orchestration: query → cache check → compute baseline → detect → rank → limit"
  - "HistoricalDataPoint type for time-series data extraction from DataFrame responses"
  - "Graceful degradation pattern: anomaly detection failure logs warning but continues with non-anomaly response"

# Metrics
duration: 3.7min
completed: 2026-01-23
---

# Phase 19 Plan 03: Anomaly Detection Service Summary

**AnomalyService orchestrates 7-day baseline computation with time-of-day matching, ranks anomalies by severity, and integrates with Overview tool for AI-driven metrics analysis**

## Performance

- **Duration:** 3 minutes 41 seconds
- **Started:** 2026-01-23T06:33:19Z
- **Completed:** 2026-01-23T06:37:00Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- AnomalyService orchestrates detection flow: fetch current metrics, compute/retrieve baselines, detect anomalies, rank results
- 7-day historical baseline computation with time-of-day matching (1-hour granularity, weekday/weekend separation)
- Overview tool returns top 20 anomalies with minimal context (metric name, value, baseline, z-score, severity)
- Graceful error handling: skip metrics with insufficient data, track skip count, log warnings on failures

## Task Commits

Each task was committed atomically:

1. **Task 1: Create AnomalyService with baseline computation** - `7d63cee` (feat)
2. **Task 2: Update Overview tool with anomaly detection** - `888605d` (feat)

## Files Created/Modified
- `internal/integration/grafana/anomaly_service.go` - Anomaly detection orchestration with baseline computation from 7-day history
- `internal/integration/grafana/tools_metrics_overview.go` - Updated to call anomaly detection and format minimal context responses
- `internal/integration/grafana/grafana.go` - Initialize anomaly service with detector and baseline cache

## Decisions Made

**1. DataFrame parsing clarification**
- ExecuteDashboard returns time-series data spanning full time range in DataFrame.Data.Values arrays
- Values[0] contains timestamps (epoch milliseconds), Values[1] contains metric values
- For 7-day baseline queries, this returns ~10k data points, not single-value snapshots
- Clarifies historical data extraction approach in computeBaseline

**2. Metric name extraction strategy**
- Prefer __name__ label from Prometheus conventions
- Fallback to constructing name from first label pair when __name__ missing
- Handles cases where labels don't include standard __name__ field

**3. Minimal context optimization**
- When anomalies detected, omit dashboard results from response (set to nil)
- Only return: anomalies array, summary stats, time range
- Reduces token usage in AI responses per CONTEXT.md progressive disclosure principle

**4. Single dashboard anomaly detection**
- Run detection on first dashboard only (typically primary overview dashboard)
- Avoids redundant detection across multiple overview dashboards
- Reduces query load while maintaining coverage

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - implementation proceeded smoothly with existing infrastructure.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Anomaly detection service fully operational
- Overview tool enhanced with AI-driven anomaly analysis
- Ready for Phase 19 Plan 04 (MCP tool registration and integration testing)
- All ANOM-* requirements satisfied (ANOM-06 addressed via skip behavior for metrics with insufficient data)

---
*Phase: 19-anomaly-detection*
*Completed: 2026-01-23*
