# Phase 25: Baseline & Anomaly Detection - Context

**Gathered:** 2026-01-29
**Status:** Ready for planning

<domain>
## Phase Boundary

Build rolling baseline statistics for signal anchors and detect anomalies using z-score/percentile comparison. Bootstraps thresholds from Grafana alerts. Aggregates anomaly scores upward from metrics to signals to workloads to namespaces to clusters.

</domain>

<decisions>
## Implementation Decisions

### Baseline Statistics
- 7-day retention window (matches existing anomaly detection patterns from v1.3/v1.4)
- Cold start handling: mark as "unknown" with confidence = 0, no anomaly score until baseline exists
- No time-of-day bucketing — single rolling baseline per signal
- Minimum 10 samples before baseline is considered valid

### Anomaly Scoring
- Combine z-score and percentile comparison using MAX of both — anomaly if EITHER method flags it
- Grafana alert firing → override anomaly score to 1.0 (human already decided)
- Anomaly threshold: 0.5 — above this = anomalous
- Confidence indicator = min(sampleConfidence, qualityScore) — reflects both statistical validity and dashboard quality

### Collection Strategy
- Forward collection frequency: 5 minutes (match typical Prometheus scrape interval)
- Backfill triggered automatically on signal creation
- Backfill limit: 7 days max (match baseline retention window)
- Rate limiting: fixed hardcoded limit to protect Grafana API

### Aggregation Behavior
- Aggregation method: MAX score — workload anomaly = worst signal anomaly
- Quality weighting: tiebreaker only — same score prefers high-quality signal as source
- Scope filter: all signals included in rollup (no filtering)
- Caching: aggregated scores cached with TTL, refresh periodically

### Claude's Discretion
- Exact rate limit value for Grafana API protection
- Cache TTL duration for aggregated scores
- Internal data structures for rolling statistics (reservoir sampling, streaming algorithms, etc.)
- Specific z-score threshold for anomaly detection
- Percentile thresholds for anomaly flagging

</decisions>

<specifics>
## Specific Ideas

- Pattern consistency: follow 7-day baseline approach used in v1.3 metrics anomaly detection
- Pattern consistency: follow TTL-based caching from existing alert analysis
- Alert state as "strong signal" — firing alert is definitive, not probabilistic

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 25-baseline-anomaly-detection*
*Context gathered: 2026-01-29*
