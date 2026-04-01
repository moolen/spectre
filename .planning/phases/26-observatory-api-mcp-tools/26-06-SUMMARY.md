---
phase: 26
plan: 06
subsystem: observatory-mcp-tools
tags: [grafana, observatory, mcp, investigate, signal-detail, compare]
depends_on:
  requires: [26-02]
  provides: [ObservatorySignalDetailTool, ObservatoryCompareTool]
  affects: [26-07, 26-08]
tech_stack:
  added: []
  patterns: [tool-wrapper-pattern, service-composition, graceful-degradation]
key_files:
  created:
    - internal/integration/grafana/tools_observatory_signal_detail.go
    - internal/integration/grafana/tools_observatory_compare.go
    - internal/integration/grafana/tools_observatory_investigate_test.go
  modified: []
decisions:
  - key: partial-data-on-cold-start
    choice: Return response with confidence=0 for insufficient baseline
    reason: Graceful degradation - tool succeeds with indication of data quality
  - key: max-lookback-cap
    choice: Silently cap lookback at 168h (7 days)
    reason: Consistent with existing TimeRange validation pattern
  - key: strings-contains-for-error-detection
    choice: Use strings.Contains for error message detection
    reason: Avoid name collision with existing contains helper in test files
metrics:
  duration: 8 min
  completed: 2026-01-30
---

# Phase 26 Plan 06: Investigate Stage MCP Tools Summary

Two Investigate stage MCP tools for deep signal inspection: observatory_signal_detail and observatory_compare.

## What Was Built

### ObservatorySignalDetailTool (`tools_observatory_signal_detail.go`)

MCP tool for deep signal inspection:

1. **Parameters (all required)**
   - `namespace`: Kubernetes namespace
   - `workload`: Workload name
   - `metric_name`: PromQL metric name

2. **Response (per TOOL-09, TOOL-10)**
   ```go
   type ObservatorySignalDetailResponse struct {
       MetricName      string                   `json:"metric_name"`
       Role            string                   `json:"role"`
       CurrentValue    float64                  `json:"current_value"`
       Baseline        ObservatoryBaselineStats `json:"baseline"`
       AnomalyScore    float64                  `json:"anomaly_score"`
       Confidence      float64                  `json:"confidence"`
       SourceDashboard string                   `json:"source_dashboard"`
       QualityScore    float64                  `json:"quality_score"`
       Timestamp       string                   `json:"timestamp"`
   }
   ```

3. **Error handling**
   - Missing params: validation error
   - Signal not found: error with clear message
   - Insufficient baseline: partial response with confidence=0

### ObservatoryCompareTool (`tools_observatory_compare.go`)

MCP tool for time-based signal comparison:

1. **Parameters**
   - `namespace`: Required
   - `workload`: Required
   - `metric_name`: Required
   - `lookback`: Optional duration (default "24h", max "168h"/7d)

2. **Response (per TOOL-11, TOOL-12)**
   ```go
   type ObservatoryCompareResponse struct {
       MetricName    string  `json:"metric_name"`
       CurrentValue  float64 `json:"current_value"`
       CurrentScore  float64 `json:"current_score"`
       PastValue     float64 `json:"past_value"`
       PastScore     float64 `json:"past_score"`
       ScoreDelta    float64 `json:"score_delta"` // positive = worsening
       LookbackHours int     `json:"lookback_hours"`
       Timestamp     string  `json:"timestamp"`
   }
   ```

3. **Lookback handling**
   - Default: 24 hours
   - Maximum: 168 hours (7 days) - silently capped
   - Accepts Go duration strings: "1h", "12h", "24h", etc.

## Key Implementation Details

### Service Composition Pattern

Both tools wrap ObservatoryInvestigateService (from 26-02):

```go
// tools_observatory_signal_detail.go
detail, err := t.investigateService.GetSignalDetail(ctx, namespace, workload, metricName)

// tools_observatory_compare.go
comparison, err := t.investigateService.CompareSignal(ctx, namespace, workload, metricName, lookback)
```

### Graceful Degradation

Signal detail handles cold start scenario per RESEARCH.md pitfall guidance:

```go
if containsInsufficientBaseline(err) {
    return &ObservatorySignalDetailResponse{
        MetricName:   params.MetricName,
        Confidence:   0, // Indicate insufficient data
        // ... partial data
    }, nil
}
```

### Numeric-Only Responses

Per CONTEXT.md: "No categorical labels - just numeric scores"

- ScoreDelta is the "correlation" indicator
- Positive ScoreDelta = worsening (current worse than past)
- Negative ScoreDelta = improving

## Test Coverage

10 test cases covering all scenarios:

### ObservatorySignalDetailTool (4 tests)
| Test | Purpose |
|------|---------|
| Execute_Success | Returns full detail with baseline stats |
| Execute_NotFound | Returns error for missing signal |
| Execute_InsufficientBaseline | Returns partial data with confidence=0 |
| Execute_MissingParams | Validates required parameters |

### ObservatoryCompareTool (6 tests)
| Test | Purpose |
|------|---------|
| Execute_Success | Returns score comparison with delta |
| Execute_DefaultLookback | Uses 24h when not specified |
| Execute_ScoreDelta | Verifies positive=worsening, negative=improving |
| Execute_MaxLookback | Caps at 168h (7 days) |
| Execute_MissingParams | Validates required parameters |
| Execute_InvalidLookback | Rejects invalid duration strings |

## Deviations from Plan

None - plan executed exactly as written.

## Key Links Verified

| From | To | Via | Pattern |
|------|-----|-----|---------|
| tools_observatory_signal_detail.go | observatory_investigate_service.go | Service composition | `investigateService.GetSignalDetail` |
| tools_observatory_compare.go | observatory_investigate_service.go | Service composition | `investigateService.CompareSignal` |

## Files Changed

- `internal/integration/grafana/tools_observatory_signal_detail.go` (152 lines) - Signal detail tool
- `internal/integration/grafana/tools_observatory_compare.go` (139 lines) - Compare tool
- `internal/integration/grafana/tools_observatory_investigate_test.go` (620 lines) - Unit tests

## Next Phase Readiness

Ready for 26-07 (Hypothesize/Verify stage tools) or 26-08 (integration testing):
- Investigate stage tools complete
- Pattern established for tool → service composition
- Response types consistent with other Observatory tools

## Commits

1. `feat(26-06): implement ObservatorySignalDetailTool` - 1b0b3c7
2. `feat(26-06): implement ObservatoryCompareTool` - 751ed56
3. `test(26-06): add unit tests for Investigate stage tools` - 31040d6
