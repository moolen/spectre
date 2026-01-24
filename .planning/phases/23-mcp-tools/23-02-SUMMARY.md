---
phase: 23-mcp-tools
plan: 02
subsystem: mcp
tags: [grafana, alerts, mcp-tools, state-timeline, graph, progressive-disclosure]

# Dependency graph
requires:
  - phase: 22-historical-analysis
    provides: AlertAnalysisService with flappiness scores, categories, and baselines
  - phase: 21-alert-state-tracking
    provides: STATE_TRANSITION edges with 7-day TTL and LOCF semantics
provides:
  - grafana_{name}_alerts_aggregated tool with compact state timeline buckets
  - grafana_{name}_alerts_details tool with full 7-day state history
  - Progressive disclosure pattern for alert investigation
affects: [23-03-mcp-tools, future-alert-tooling]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "10-minute bucket timeline with LOCF interpolation"
    - "Progressive disclosure: overview → aggregated → details"
    - "Compact state notation: [F F N N] for readability"
    - "Analysis enrichment with categories and flappiness inline"

key-files:
  created:
    - internal/integration/grafana/tools_alerts_aggregated.go
    - internal/integration/grafana/tools_alerts_details.go
  modified:
    - internal/integration/grafana/grafana.go

key-decisions:
  - "10-minute buckets for 1h default lookback (6 buckets per hour)"
  - "Left-to-right timeline ordering (oldest→newest) for natural reading"
  - "Category format: CHRONIC + flapping for inline display"
  - "Graceful degradation for insufficient data: category = 'new (insufficient history)'"
  - "All filters optional for maximum flexibility"
  - "Details tool warns for multiple alerts (large response)"

patterns-established:
  - "buildStateTimeline helper: LOCF with 10-minute buckets"
  - "formatCategory: combines onset and pattern with + separator"
  - "StatePoint array with explicit timestamps and duration_in_state"
  - "Flexible filter parameters: all optional, combined with AND logic"

# Metrics
duration: 3min
completed: 2026-01-23
---

# Phase 23 Plan 02: Alert Tools with State Timelines Summary

**Grafana MCP tools for progressive alert drill-down: compact state timeline buckets in aggregated view, full 7-day history with timestamps in details view**

## Performance

- **Duration:** 3 minutes
- **Started:** 2026-01-23T12:18:54Z
- **Completed:** 2026-01-23T12:22:01Z
- **Tasks:** 3
- **Files modified:** 3 (2 created, 1 modified)

## Accomplishments
- AlertsAggregatedTool shows specific alerts with compact 1h state timeline [F F N N] notation
- AlertsDetailsTool provides full 7-day state history with explicit timestamps and durations
- Both tools integrate with AlertAnalysisService for flappiness scores and categories
- Progressive disclosure workflow: overview identifies issues → aggregated shows timelines → details provides deep debugging
- Complete flexibility with all filter parameters optional

## Task Commits

Each task was committed atomically:

1. **Task 1: Create Aggregated Tool with State Timeline Buckets** - `9d237cf` (feat)
2. **Task 2: Create Details Tool with Full State History** - `c05dec6` (feat)
3. **Task 3: Register Aggregated and Details Tools** - `cf5fc06` (feat)

## Files Created/Modified
- `internal/integration/grafana/tools_alerts_aggregated.go` - Aggregated tool with 10-minute bucket timelines, LOCF interpolation, analysis enrichment (430 lines)
- `internal/integration/grafana/tools_alerts_details.go` - Details tool with full state history, rule definitions, complete metadata (308 lines)
- `internal/integration/grafana/grafana.go` - Tool registration for both aggregated and details tools, updated to "6 Grafana MCP tools"

## Decisions Made

**1. 10-minute bucket size for compact timelines**
- Rationale: 6 buckets per hour provides readable timeline without excessive detail
- Default 1h lookback shows recent progression clearly
- Configurable lookback parameter allows longer views when needed

**2. Left-to-right timeline ordering (oldest→newest)**
- Rationale: Natural reading direction, matches typical timeline visualizations
- Format: [F F N N F F] - left is earliest, right is most recent

**3. Category display format: "CHRONIC + flapping"**
- Rationale: Combines onset (time-based) and pattern (behavior-based) in readable inline format
- Special case: "stable-normal" when alert never fired
- Handles insufficient data: "new (insufficient history)"

**4. All filter parameters optional**
- Rationale: Maximum flexibility for AI to explore alerts
- Filters combine with AND logic when multiple specified
- No required parameters except integration name (implicit)

**5. Details tool warns for multiple alerts**
- Rationale: Full 7-day history per alert can produce large responses
- Log warning when > 5 alerts without specific alert_uid
- AI can adjust query to narrow scope

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - all tasks completed as specified.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

**Ready for:**
- Phase 23-03: Additional alert MCP tools (alert count queries, severity aggregations)
- Integration testing of progressive disclosure workflow
- MCP client usage of alert investigation tools

**Delivered capabilities:**
- AI can view specific alerts with compact state timelines after identifying issues in overview
- AI can drill down to full state history with timestamps for deep debugging
- Analysis enrichment provides flappiness and categories inline with timelines
- Progressive disclosure pattern guides AI from overview → aggregated → details

**No blockers or concerns.**

---
*Phase: 23-mcp-tools*
*Completed: 2026-01-23*
