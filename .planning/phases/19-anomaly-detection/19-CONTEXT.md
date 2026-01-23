# Phase 19: Anomaly Detection & Progressive Disclosure - Context

**Gathered:** 2026-01-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Detect anomalies in Grafana metrics against a 7-day baseline, classify by severity, and enable progressive disclosure from overview to details. AI can detect what's abnormal and drill down to investigate.

</domain>

<decisions>
## Implementation Decisions

### Severity thresholds
- Critical: 3+ sigma (standard statistical threshold)
- Metric-aware thresholds: error-rate metrics (5xx, failures) use 2+ sigma for critical
- Both directions flagged: AI decides if high/low is good or bad
- Uniform thresholds for non-error metrics

### Baseline behavior
- 1-hour window granularity for time-of-day matching
- Weekday/weekend separation: Monday 10am compares to other weekday 10am, not Sunday 10am
- Minimum 3 matching windows required before computing baseline
- Silently skip metrics with insufficient history (don't flag as "insufficient data")

### AI output format
- Ranking: severity first, then z-score within severity
- Minimal context per anomaly: metric name, current value, baseline, z-score, severity
- Limit to top 20 anomalies in overview
- When no anomalies: return summary stats only (metrics checked, time range), no explicit "healthy" message

### Missing data handling
- Missing metrics handled separately from value anomalies (different category)
- Scrape status included as a note field in anomaly output
- Fail fast on query errors: skip immediately, continue with other metrics
- Include skip count in output: "15 anomalies found, 3 metrics skipped due to errors"

### Claude's Discretion
- Z-score thresholds for info vs warning (given critical is 3+ sigma / 2+ for errors)
- Exact algorithm for weekday/weekend day-type detection
- Format of summary stats when no anomalies detected
- How to identify error-rate metrics (naming patterns, metric type heuristics)

</decisions>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches for z-score calculation and statistical baseline computation.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 19-anomaly-detection*
*Context gathered: 2026-01-23*
