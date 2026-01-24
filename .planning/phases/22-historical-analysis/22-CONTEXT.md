# Phase 22: Historical Analysis - Context

**Gathered:** 2026-01-23
**Status:** Ready for planning

<domain>
## Phase Boundary

AlertAnalysisService that computes flappiness scores, baseline comparisons, and alert categorization from state transition history stored in graph. Used by Phase 23 MCP tools to provide AI with historical context about alerts.

</domain>

<decisions>
## Implementation Decisions

### Flappiness Definition
- Evaluate over 6-hour sliding window
- Threshold: 5+ state transitions indicates flapping
- Continuous score (0.0-1.0) for ranking, not binary
- Score factors in both transition frequency AND duration in each state (penalize short-lived states)

### Baseline Comparison
- Use rolling 7-day average (not time-of-day matching)
- Baseline metric: full state distribution (% normal, % pending, % firing)
- Deviation threshold: 2x standard deviation indicates abnormal
- Output: numeric deviation score (how many std devs from baseline)

### Alert Categorization
- Categories combine onset AND pattern (both dimensions)
- **Onset categories:** new (<1h), recent (<24h), persistent (>24h), chronic (>7d)
- **Pattern categories:** stable-firing, stable-normal, flapping, trending-worse, trending-better
- Trending detection: compare last 1h to prior 6h
- Chronic threshold: >80% time firing over 7 days
- Multi-label: alert can have multiple categories (e.g., both chronic and flapping)

### Data Handling
- Minimum 24h history required for analysis, otherwise return 'insufficient data'
- Use available data for alerts with 24h-7d history, compute baseline from what exists
- Interpolate gaps: assume last known state continued through any data gaps
- Cache results with 5-minute TTL to handle repeated queries
- Fail with error if Grafana API unavailable (don't fall back to stale data)

### Claude's Discretion
- Exact flappiness score formula (how to weight frequency vs duration)
- State distribution comparison math details
- Internal data structures for analysis results

</decisions>

<specifics>
## Specific Ideas

- "Flappiness should penalize alerts that fire briefly then go normal repeatedly — that's the annoying pattern"
- Deviation score lets AI rank alerts by how unusual their current behavior is
- Multi-label categorization because chronic alerts can also flap

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 22-historical-analysis*
*Context gathered: 2026-01-23*
