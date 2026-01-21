---
phase: 05-progressive-disclosure-mcp-tools
plan: 03
subsystem: mcp-tools
tags: [victorialogs, mcp, drain, template-mining, novelty-detection]

# Dependency graph
requires:
  - phase: 04-log-template-mining
    provides: TemplateStore with Drain clustering and CompareTimeWindows method
  - phase: 05-01
    provides: MCP tool registration infrastructure and ToolRegistry

provides:
  - Patterns MCP tool for template aggregation with novelty detection
  - High-volume namespace sampling for efficient template mining
  - Time-window batching for previous/current comparison

affects: [05-04]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "On-demand template mining (stateless per query)"
    - "Sampling threshold: targetSamples * 10 for high-volume namespaces"
    - "Novelty detection via pattern comparison (current vs previous window)"

key-files:
  created:
    - internal/integration/victorialogs/tools_patterns.go
  modified:
    - internal/logprocessing/store.go
    - internal/integration/victorialogs/victorialogs.go

key-decisions:
  - "CompareTimeWindows compares by Pattern not ID for semantic novelty"
  - "Per-instance template store (not global) for independent mining"
  - "Stateless design: TemplateStore populated on-demand per query"
  - "Sampling threshold = targetSamples * 10 (default 50 * 10 = 500 logs)"
  - "Time-window batching via single QueryLogs call per window"

patterns-established:
  - "Novelty via pattern comparison between equal-duration windows"
  - "Compact response: one sample log per template"
  - "Graceful degradation: empty previous = all templates novel"

# Metrics
duration: 3 min
completed: 2026-01-21
---

# Phase 5 Plan 3: Patterns Tool Summary

**Template aggregation with novelty detection via Drain clustering and time-window comparison**

## Performance

- **Duration:** 3 min
- **Started:** 2026-01-21T15:31:53Z
- **Completed:** 2026-01-21T15:35:44Z
- **Tasks:** 3 (plus Task 4 already complete)
- **Files modified:** 3

## Accomplishments

- CompareTimeWindows method for novelty detection in TemplateStore
- TemplateStore integration into VictoriaLogs lifecycle (Start/Stop)
- PatternsTool with sampling, mining, and novelty detection
- High-volume namespace sampling (MINE-05) with threshold detection
- Time-window batching (MINE-06) for efficient current/previous comparison
- Graceful error handling (previous window fetch failures)

## Task Commits

Each task was committed atomically:

1. **Task 1: CompareTimeWindows novelty detection** - `5349dce` (feat)
   - CompareTimeWindows method in store.go
   - Compares current to previous templates by Pattern
   - Returns map of templateID -> isNovel boolean

2. **Task 2: TemplateStore integration** - `0cd32b6` (feat)
   - Added templateStore field to VictoriaLogsIntegration
   - Initialized in Start() with Drain config (depth=4, simTh=0.4)
   - Cleared in Stop() for proper lifecycle

3. **Task 3: Patterns tool implementation** - `7ce324c` (feat)
   - PatternsTool with Execute method
   - fetchLogsWithSampling for high-volume efficiency
   - mineTemplates processes logs through TemplateStore
   - Novelty detection via CompareTimeWindows

4. **Task 4: Register patterns tool** - Already complete (from Plan 02)
   - RegisterTools already includes patterns tool registration
   - Includes nil check for templateStore
   - Tool naming: victorialogs_{instance}_patterns

**Note:** Task 4 (tool registration) was already completed during Plan 02 execution.

## Files Created/Modified

- `internal/logprocessing/store.go` - Added CompareTimeWindows method
- `internal/integration/victorialogs/victorialogs.go` - Added templateStore lifecycle
- `internal/integration/victorialogs/tools_patterns.go` - Complete patterns tool implementation

## Decisions Made

**CompareTimeWindows design:**
- Compare by Pattern not ID for semantic novelty detection
- Pattern comparison detects "this log message never appeared before"
- Considered: Levenshtein similarity. Rejected: exact pattern match sufficient for v1

**TemplateStore lifecycle:**
- Per-instance template store (not global)
- Rationale: Different VictoriaLogs instances have different log characteristics
- No persistence: Ephemeral mining per query (stateless design from CONTEXT.md)
- Phase 4's PersistenceManager NOT used (different use case)

**Sampling strategy (MINE-05):**
- Threshold: targetSamples * 10 (default 500 logs triggers sampling)
- Sample size: targetSamples * 2 (default 100 for better coverage)
- Balances template accuracy with query performance

**Time-window batching (MINE-06):**
- Single QueryLogs call per window (not streaming)
- Previous window = same duration before current window
- Graceful degradation: empty previous = all templates marked novel

## Deviations from Plan

None - plan executed exactly as written. Task 4 was already complete from Plan 02.

## Issues Encountered

**Issue:** DrainConfig field name mismatch
- Plan specified `Depth` but actual field is `LogClusterDepth`
- Fixed immediately in Task 2 commit

**Issue:** Duplicate tools_common.go file with conflicting definitions
- Found untracked duplicate with wrong TimeRangeParams type (string vs int64)
- Removed duplicate, used correct tools.go definitions

## Next Phase Readiness

- Patterns tool complete and registered
- Phase 5 Plan 3 requirements fulfilled
- Ready for Plan 4: Detail logs tool (if needed)

---
*Phase: 05-progressive-disclosure-mcp-tools*
*Completed: 2026-01-21*
