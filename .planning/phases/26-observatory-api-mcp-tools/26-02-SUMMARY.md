---
phase: 26
plan: 02
subsystem: observatory-api
tags: [grafana, observatory, mcp, signals, anomaly-detection]
depends_on:
  requires: [25-02, 25-03]
  provides: [ObservatoryInvestigateService, GetWorkloadSignals, GetSignalDetail, CompareSignal, QueryService-interface]
  affects: [26-03, 26-04]
tech_stack:
  added: []
  patterns: [service-layer, interface-abstraction, column-mapping, graceful-degradation]
key_files:
  created:
    - internal/integration/grafana/observatory_investigate_service.go
    - internal/integration/grafana/observatory_investigate_service_test.go
  modified: []
decisions:
  - key: QueryService-interface
    choice: Abstract metric fetching behind interface
    reason: Enables unit testing without Grafana dependency
  - key: baseline-fallback
    choice: Use baseline mean when query service fails
    reason: Graceful degradation - service continues with approximate value
  - key: default-lookback-24h
    choice: Default time comparison to 24 hours
    reason: Captures daily patterns per RESEARCH.md recommendation
metrics:
  duration: 3 min
  completed: 2026-01-30
---

# Phase 26 Plan 02: Observatory Investigate Service Summary

ObservatoryInvestigateService for Narrow and Investigate stage queries with 9 passing tests.

## What Was Built

### ObservatoryInvestigateService (`observatory_investigate_service.go`)

Service layer for deep signal inspection during incident investigation:

1. **GetWorkloadSignals(ctx, namespace, workload)** - Returns all signals for a workload with current anomaly scores
   - Queries graph for SignalAnchors with baselines
   - Computes anomaly score for each signal via `ComputeAnomalyScore`
   - Skips signals with cold start (< 10 samples)
   - Returns flat list sorted by score descending (per CONTEXT.md)

2. **GetSignalDetail(ctx, namespace, workload, metricName)** - Returns detailed baseline and anomaly info
   - Queries specific SignalAnchor with baseline and dashboard source
   - Fetches current value from Grafana via QueryService interface
   - Falls back to baseline mean if Grafana unavailable
   - Returns baseline stats, anomaly score, confidence, source dashboard

3. **CompareSignal(ctx, namespace, workload, metricName, lookback)** - Time-based comparison
   - Per CONTEXT.md: "Compare tool compares across time only (current vs N hours/days ago)"
   - Default lookback: 24 hours
   - Computes anomaly scores for current and historical values
   - Returns ScoreDelta (positive = getting worse)

### Response Types

Minimal response structures per CONTEXT.md ("facts only, AI interprets meaning"):

- `WorkloadSignalsResult` - List of signals with scope identifier
- `SignalSummary` - MetricName, Role, Score, Confidence
- `SignalDetailResult` - Full baseline stats, current value, source dashboard
- `BaselineStats` - Mean, StdDev, P50, P90, P99, SampleCount
- `SignalComparisonResult` - Current vs past values with score delta

### QueryService Interface

Abstraction for Grafana metric fetching (enables unit testing):

```go
type QueryService interface {
    FetchCurrentValue(ctx, metricName, namespace, workload string) (float64, error)
    FetchHistoricalValue(ctx, metricName, namespace, workload string, lookback time.Duration) (float64, error)
}
```

## Key Implementation Details

### Graph Queries

Uses existing graph infrastructure with column mapping pattern:

```cypher
MATCH (sig:SignalAnchor {
    workload_namespace: $namespace,
    workload_name: $workload,
    integration: $integration
})
WHERE sig.expires_at > $now
OPTIONAL MATCH (sig)-[:HAS_BASELINE]->(b:SignalBaseline)
OPTIONAL MATCH (sig)-[:EXTRACTED_FROM]->(q:Query)-[:BELONGS_TO]->(p:Panel)-[:BELONGS_TO]->(d:Dashboard)
RETURN sig.role, sig.quality_score, d.uid, b.mean, b.std_dev, ...
```

### Cold Start Handling

Graceful handling per RESEARCH.md pitfall guidance:

```go
score, err := ComputeAnomalyScore(currentValue, baseline, qualityScore)
if err != nil {
    var insufficientErr *InsufficientSamplesError
    if errors.As(err, &insufficientErr) {
        continue // Skip cold-start signals silently
    }
    return nil, err // Other errors propagate
}
```

### Constants

- `DefaultLookback = 24 * time.Hour` - Default time comparison window
- `AnomalyThreshold = 0.5` - Per CONTEXT.md: "Fixed anomaly score threshold internally"

## Test Coverage

9 test cases covering all required scenarios:

| Test | Purpose |
|------|---------|
| GetWorkloadSignals_Success | Returns signals sorted by score |
| GetWorkloadSignals_SkipsColdStart | Signals with insufficient samples skipped |
| GetSignalDetail_Success | Returns full detail with baseline |
| GetSignalDetail_NotFound | Returns error for missing signal |
| CompareSignal_Success | Shows score delta across time |
| CompareSignal_DefaultLookback | Uses 24h when not specified |
| EmptyParams | Validates required parameters |
| GetSignalDetail_FallbackToBaseline | Falls back when query service fails |
| GetWorkloadSignals_EmptyResult | Handles empty result gracefully |

## Deviations from Plan

None - plan executed exactly as written.

## Key Links Verified

| From | To | Via | Pattern |
|------|-----|-----|---------|
| observatory_investigate_service.go | anomaly_scorer.go | ComputeAnomalyScore | 4 usages |
| observatory_investigate_service.go | (future) query_service.go | QueryService interface | Interface abstraction |

## Files Changed

- `internal/integration/grafana/observatory_investigate_service.go` (518 lines) - Service implementation
- `internal/integration/grafana/observatory_investigate_service_test.go` (444 lines) - Unit tests

## Next Phase Readiness

Ready for 26-03 (observatory_evidence_service.go):
- Service pattern established
- QueryService interface defined for real implementation
- Response types provide template for Evidence service

## Commits

1. `feat(26-02): implement ObservatoryInvestigateService` - 1cf5790
2. `test(26-02): add unit tests for ObservatoryInvestigateService` - fe92661
