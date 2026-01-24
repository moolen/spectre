---
phase: 23-mcp-tools
plan: 01
subsystem: mcp-tools
tags: [grafana, alerts, mcp, flappiness, progressive-disclosure]

# Dependency graph
requires:
  - phase: 22-historical-analysis
    provides: AlertAnalysisService with flappiness scoring and categorization
  - phase: 21-alert-states
    provides: Alert state tracking in graph with STATE_TRANSITION edges
  - phase: 20-alert-rules
    provides: Alert rule sync with labels and annotations in graph
provides:
  - grafana_{name}_alerts_overview MCP tool for AI-driven alert triage
  - AlertsOverviewTool with severity-based aggregation
  - Flappiness indicators in overview response (>0.7 threshold)
  - Optional filtering by severity, cluster, service, namespace
affects: [23-02-alerts-list, 23-03-alerts-analysis, mcp-tools]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Progressive disclosure: overview → list → analyze pattern"
    - "Optional filters with empty required array in MCP schema"
    - "Graceful degradation: nil AlertAnalysisService handled transparently"
    - "ErrInsufficientData checked with errors.As for new alerts"
    - "Label extraction from JSON strings via json.Unmarshal"

key-files:
  created:
    - internal/integration/grafana/tools_alerts_overview.go
  modified:
    - internal/integration/grafana/grafana.go

key-decisions:
  - "All filter parameters optional (no required fields) for maximum flexibility"
  - "Flappiness threshold 0.7 from Phase 22-02 categorization logic"
  - "Tool name includes integration name: grafana_{name}_alerts_overview"
  - "Handle nil AlertAnalysisService (graph disabled) gracefully"
  - "Severity case normalization with strings.ToLower for matching"
  - "Return minimal AlertSummary (name + firing_duration) to minimize tokens"
  - "Group by severity in response for easy triage scanning"

patterns-established:
  - "Pattern 1: AlertsOverviewTool follows Phase 18 OverviewTool structure"
  - "Pattern 2: All MCP tool filters optional when filtering is secondary concern"
  - "Pattern 3: Graceful degradation when optional services (analysis) unavailable"

# Metrics
duration: 2min
completed: 2026-01-23
---

# Phase 23 Plan 01: Alerts Overview Tool Summary

**MCP tool for AI-driven alert triage with severity-based aggregation, flappiness indicators, and optional filtering by severity/cluster/service/namespace**

## Performance

- **Duration:** 2 min
- **Started:** 2026-01-23T14:52:12Z
- **Completed:** 2026-01-23T14:54:42Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Created AlertsOverviewTool with filtering and severity-based grouping
- Integrated flappiness detection using AlertAnalysisService (0.7 threshold)
- Registered tool as grafana_{name}_alerts_overview with all optional parameters
- Graceful handling of nil AlertAnalysisService and ErrInsufficientData

## Task Commits

Each task was committed atomically:

1. **Task 1: Create Overview Tool with Filtering and Aggregation** - `bb026f3` (feat)
2. **Task 2: Register Overview Tool in Integration** - `ba1767e` (feat)

## Files Created/Modified
- `internal/integration/grafana/tools_alerts_overview.go` - Overview tool implementation with filtering, aggregation, and flappiness detection
- `internal/integration/grafana/grafana.go` - Tool registration in RegisterTools method (updated count to 4 tools)

## Decisions Made

**1. All filter parameters optional**
- Rationale: Enables "show me all alerts" query without requiring filters
- Implementation: Empty `required: []` array in MCP schema

**2. Flappiness threshold 0.7**
- Rationale: Consistent with Phase 22-02 categorization logic
- Implementation: `if analysis.FlappinessScore > 0.7` in groupBySeverity

**3. Graceful degradation for nil AlertAnalysisService**
- Rationale: Tool still useful even without flappiness data (graph disabled)
- Implementation: Check `if t.analysisService != nil` before calling AnalyzeAlert

**4. ErrInsufficientData handling with errors.As**
- Rationale: New alerts don't have 24h history - not an error condition
- Implementation: `errors.As(err, &insufficientErr)` to distinguish from real errors

**5. Severity case normalization**
- Rationale: User may type "Critical" or "CRITICAL", should match "critical" label
- Implementation: `strings.ToLower()` on both input parameter and label matching

**6. Minimal AlertSummary response**
- Rationale: Reduce token usage in MCP responses for AI efficiency
- Implementation: Only name + firing_duration + optional labels (cluster/service/namespace)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - implementation followed Phase 18 metrics overview tool patterns closely.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

**Ready for Phase 23-02 (Alerts List Tool):**
- AlertsOverviewTool provides high-level triage interface
- Next tool will provide detailed alert list with full state information
- Pattern established for progressive disclosure: overview → list → analyze

**Ready for Phase 23-03 (Alerts Analysis Tool):**
- AlertAnalysisService integration pattern proven
- Flappiness threshold consistent across all tools
- ErrInsufficientData handling pattern established

**Architecture verification:**
- Tool uses GetAnalysisService() accessor from Phase 22-03
- Shares graphClient with other components (no separate client)
- Follows Phase 18 progressive disclosure pattern (overview first, details later)

---
*Phase: 23-mcp-tools*
*Completed: 2026-01-23*
