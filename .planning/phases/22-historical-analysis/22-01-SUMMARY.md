---
phase: 22
plan: 01
subsystem: alert-historical-analysis
tags: [statistical-analysis, flappiness-detection, baseline-comparison, tdd, gonum]
completed: 2026-01-23
duration: 9 minutes

requires:
  - phases: [21]
    reason: "State transition data from alert sync pipeline"

provides:
  - Statistical flappiness score computation (0.0-1.0 range)
  - Rolling baseline calculation with LOCF interpolation
  - Deviation analysis (standard deviations from baseline)
  - Robust edge case handling (<24h data, gaps, boundary conditions)

affects:
  - phases: [22-02]
    impact: "AlertAnalysisService will use these functions for categorization"

tech-stack:
  added:
    - gonum.org/v1/gonum/stat: "Sample variance, mean, standard deviation"
  patterns:
    - "TDD RED-GREEN-REFACTOR cycle with comprehensive test coverage"
    - "LOCF (Last Observation Carried Forward) for gap interpolation"
    - "Exponential scaling for flappiness sensitivity (1 - exp(-k*n))"
    - "Sample variance (N-1) for unbiased standard deviation"

key-files:
  created:
    - internal/integration/grafana/flappiness.go: "Flappiness score computation"
    - internal/integration/grafana/flappiness_test.go: "9 test cases, >95% coverage"
    - internal/integration/grafana/baseline_test.go: "13 test cases, >90% coverage"
  modified:
    - internal/integration/grafana/baseline.go: "Added baseline and deviation functions"
    - go.mod: "Added gonum v0.17.0"
    - go.sum: "Updated checksums"

decisions:
  - slug: flappiness-exponential-scaling
    what: "Use exponential scaling (1 - exp(-k*count)) instead of linear ratio"
    why: "Makes scores more sensitive to flapping - 5 transitions ≈ 0.5, 10+ ≈ 0.8-1.0"
    trade-offs: "More tuning required (k=0.15) but better discrimination of flapping severity"

  - slug: duration-multipliers
    what: "Apply multipliers based on avg state duration ratio"
    why: "Penalize short-lived states (annoying pattern) vs long-lived states"
    trade-offs: "Step function (1.3x, 1.1x, 1.0x, 0.8x) vs continuous - simpler but less smooth"

  - slug: locf-daily-buckets
    what: "Compute daily distributions with state carryover between days"
    why: "Enables standard deviation calculation across days while handling gaps"
    trade-offs: "More complex than single-window calculation but required for multi-day variance"

  - slug: 24h-minimum-data
    what: "Require at least 24 hours of data for baseline computation"
    why: "Less than 1 day isn't statistically meaningful for daily pattern baselines"
    trade-offs: "Can't analyze new alerts immediately, but prevents misleading baselines"

  - slug: inclusive-boundary-timestamps
    what: "Transitions at period start are included (not excluded)"
    why: "Alert states at exact window boundaries are valid data points"
    trade-offs: "Requires careful timestamp comparison logic but more accurate"
---

# Phase 22 Plan 01: Statistical Functions for Flappiness and Baseline

**One-liner:** Exponential-scaled flappiness scoring and rolling baseline computation with LOCF gap filling using gonum statistical functions

## What Was Built

Created two core statistical analysis modules following TDD methodology:

### Flappiness Score Computation
- **ComputeFlappinessScore**: Calculates normalized 0.0-1.0 flappiness score
  - Exponential scaling: `1 - exp(-0.15 * transitionCount)` for sensitivity
  - Duration multipliers: 1.3x for short states (<10% window), 0.8x for long states (>50%)
  - Uses `gonum.org/v1/gonum/stat.Mean` for average state duration
  - Filters transitions to analysis window (e.g., 6 hours)

### Baseline Computation & Deviation Analysis
- **ComputeRollingBaseline**: 7-day rolling average with daily bucketing
  - StateDistribution: % normal, % pending, % firing across time period
  - LOCF interpolation fills gaps (state carries forward until next transition)
  - Sample standard deviation (N-1) via `gonum.org/v1/gonum/stat.StdDev`
  - InsufficientDataError for <24h history with clear diagnostics

- **CompareToBaseline**: Deviation score in standard deviations
  - Formula: `abs(current.PercentFiring - baseline.PercentFiring) / stdDev`
  - Returns 0.0 for zero stdDev (avoids division by zero)
  - Enables 2σ threshold detection for abnormal behavior

### Edge Case Handling
- Transitions at exact window boundaries (inclusive at period start)
- State carryover between daily buckets for accurate multi-day baseline
- Partial data (24h-7d) handled gracefully without error
- Empty transition arrays (stable alerts) return 0.0 flappiness score
- Extreme flapping capped at 1.0 (normalization)

## TDD Cycle

### RED Phase (Commits: df8348b, 223114f)
- **Flappiness tests**: 9 comprehensive test cases
  - Empty transitions, single transition, moderate/high flapping
  - Short vs long-lived states comparison
  - Window filtering, normalization, monotonicity
- **Baseline tests**: 13 comprehensive test cases
  - Insufficient data (<24h), exactly 24h boundary, partial data (3 days)
  - Stable firing, alternating states, gaps with LOCF
  - All-normal scenario, deviation comparison (0σ, 2σ, 3σ)
  - Zero stdDev edge case

All tests failed initially (no implementation yet).

### GREEN Phase (Commit: 4652f1e)
- Implemented StateTransition, StateDistribution, InsufficientDataError types
- Implemented ComputeFlappinessScore with exponential scaling and duration multipliers
- Implemented ComputeRollingBaseline with daily bucketing and LOCF
- Implemented CompareToBaseline with zero-stdDev handling
- Helper functions: computeDailyDistributions, computeStateDistributionForPeriod, addDurationToState
- Iterative fixes for:
  - Timestamp boundary conditions (inclusive at period start)
  - State carryover between days
  - Data sufficiency checks (span vs coverage)
- All 22 tests passing

### REFACTOR Phase (Commit: a09ac26)
- Pre-allocated `firingPercentages` slice with capacity hint
- Addressed `prealloc` linter warning
- All tests still passing, 0 linting issues

## Test Coverage

**Flappiness**: 96.8% line coverage
- Edge cases: empty, single, moderate, high, extreme flapping
- Window filtering, duration sensitivity, normalization
- Monotonicity (more transitions → higher scores)

**Baseline**: 92.1% line coverage
- Insufficient data handling with structured error
- LOCF interpolation across gaps
- Daily distribution bucketing
- State carryover between days
- Partial data (24h-7d) support

**CompareToBaseline**: 100% coverage
- Zero/2σ/3σ deviation scenarios
- Zero stdDev edge case

**Overall**: 22 tests, >90% average coverage

## Statistical Correctness

### Sample Variance (Unbiased Estimator)
- Uses `gonum.org/v1/gonum/stat.StdDev` which implements sample variance (N-1 divisor)
- Confirmed via `go doc`: "returns the sample standard deviation"
- Consistent with Phase 19 decision on statistical correctness

### Flappiness Formula
```
frequencyScore = 1 - exp(-k * transitionCount)  // k=0.15
durationRatio = avgStateDuration / windowSize
durationMultiplier = {1.3 if ratio<0.1, 1.1 if <0.3, 1.0 if <0.5, 0.8 otherwise}
score = min(1.0, frequencyScore * durationMultiplier)
```

**Properties verified by tests**:
- Monotonic increasing with transition count
- 5 transitions in 6h ≈ 0.5 score
- 10+ transitions ≈ 0.8-1.0 score
- Short-lived states get higher scores than long-lived (same transition count)
- Capped at 1.0 for extreme cases

### Baseline Computation
- Daily bucketing: windowSize / 24h → N days
- Each day: compute % time in each state using LOCF
- Average across days: `sum(percentages) / N`
- Sample stdDev of firing percentages across days: `stat.StdDev(firingPercentages, nil)`

**Properties verified by tests**:
- 50/50 alternating pattern → ~50% firing, moderate stdDev
- Stable firing → >90% firing, low stdDev (<0.1)
- Gaps filled via LOCF (167h gap → correct distribution)
- Partial data (3 days) → baseline from available days only

## Deviations from Plan

None - plan executed exactly as written. All success criteria met:
- ✅ gonum.org/v1/gonum/stat added to go.mod
- ✅ flappiness.go exports ComputeFlappinessScore
- ✅ baseline.go exports ComputeRollingBaseline and CompareToBaseline
- ✅ 9 flappiness test cases covering edge cases
- ✅ 13 baseline test cases covering partial data and LOCF
- ✅ All tests pass: `go test ./internal/integration/grafana/... -v`
- ✅ No golangci-lint errors
- ✅ Flappiness score handles empty/single/many transitions correctly
- ✅ Baseline uses sample variance (stat.StdDev, not PopVariance)
- ✅ ErrInsufficientData for <24h with clear error message

## Next Phase Readiness

**Phase 22-02 (AlertAnalysisService)** can proceed immediately:
- Flappiness scoring ready for integration
- Baseline comparison ready for deviation detection
- All edge cases handled (insufficient data, gaps, boundaries)
- Statistical correctness verified (sample variance, proper LOCF)

**Integration points**:
- Call `ComputeFlappinessScore(transitions, 6*time.Hour, currentTime)` for flappiness
- Call `ComputeRollingBaseline(transitions, 7, currentTime)` for baseline
- Call `CompareToBaseline(current, baseline, stdDev)` for deviation score
- Check for `InsufficientDataError` to handle new alerts gracefully

No blockers or concerns.

---

**Phase:** 22-historical-analysis
**Plan:** 01
**Status:** Complete
**Completed:** 2026-01-23
**Duration:** 9 minutes
