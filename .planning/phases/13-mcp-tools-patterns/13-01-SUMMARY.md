---
phase: 13-mcp-tools-patterns
plan: 01
subsystem: mcp
tags: [logzio, mcp, pattern-mining, drain, template-store, novelty-detection]

# Dependency graph
requires:
  - phase: 12-02
    provides: Logzio overview and logs tools with parallel aggregations
  - phase: 06-01
    provides: Drain algorithm and TemplateStore in internal/logprocessing/
provides:
  - Pattern mining MCP tool for Logzio with VictoriaLogs parity
  - Novelty detection via time window comparison
  - TemplateStore integration for namespace-scoped pattern storage
  - Complete progressive disclosure: overview → logs → patterns
affects: [logzio-integration-tests, mcp-client-usage, future-backend-integrations]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "VictoriaLogs parity: exact parameter and response type matching across backends"
    - "Shared pattern mining infrastructure via internal/logprocessing/"
    - "Sampling multiplier: targetSamples * 20 with 500-5000 range"
    - "Metadata collection during template mining (sample_log, pods, containers)"

key-files:
  created:
    - internal/integration/logzio/tools_patterns.go
  modified:
    - internal/integration/logzio/logzio.go

key-decisions:
  - "Exact VictoriaLogs parity for consistent AI experience across backends"
  - "ONLY log fetching adapted for Logzio Elasticsearch API - all else identical"
  - "Default limit 50, sampling multiplier targetSamples * 20 (500-5000 range)"
  - "Previous window failure handled gracefully - all patterns marked novel"

patterns-established:
  - "Backend parity pattern: clone reference implementation, adapt only data layer"
  - "TemplateStore lifecycle: initialize in Start(), pass to tool via ToolContext"
  - "Novelty detection via CompareTimeWindows (current vs previous duration)"
  - "Pattern tool as third step in progressive disclosure (overview → logs → patterns)"

# Metrics
duration: 3min
completed: 2026-01-22
---

# Phase 13 Plan 01: MCP Tools - Patterns Summary

**Logzio pattern mining with VictoriaLogs parity, Drain algorithm reuse, and novelty detection via time window comparison**

## Performance

- **Duration:** 2 min 44 sec
- **Started:** 2026-01-22T15:49:51Z
- **Completed:** 2026-01-22T15:52:35Z
- **Tasks:** 2
- **Files modified:** 2 (1 created, 1 modified)

## Accomplishments
- Pattern mining tool returns log templates with occurrence counts and novelty flags
- Exact VictoriaLogs parity: PatternsParams, PatternsResponse, PatternTemplate types match exactly
- Reuses existing Drain algorithm and TemplateStore from internal/logprocessing/
- Novelty detection compares current time window to previous window of same duration
- Complete progressive disclosure: overview → logs → patterns

## Task Commits

Each task was committed atomically:

1. **Task 1: Create patterns tool with VictoriaLogs parity** - `a2462fb` (feat)
   - Clone VictoriaLogs patterns tool structure
   - PatternsParams, PatternsResponse, PatternTemplate types match exactly
   - fetchLogsWithSampling adapted for Logzio Elasticsearch API
   - Uses GetErrorPattern/GetWarningPattern for severity filtering
   - Sampling multiplier: targetSamples * 20 with 500-5000 range
   - Metadata collection includes sample_log, pods, containers
   - Novelty detection via templateStore.CompareTimeWindows

2. **Task 2: Wire patterns tool into integration and initialize templateStore** - `4cf1af0` (feat)
   - Add templateStore field to LogzioIntegration struct
   - Initialize templateStore in Start() with DefaultDrainConfig()
   - Instantiate PatternsTool with templateStore reference
   - Register patterns tool as logzio_{name}_patterns
   - Tool schema matches VictoriaLogs (namespace required, severity/time/limit optional)
   - Update final log message to show 3 MCP tools

## Files Created/Modified

### Created
- **internal/integration/logzio/tools_patterns.go** (278 lines)
  - PatternsTool with Execute method
  - PatternsParams, PatternsResponse, PatternTemplate types (exact VictoriaLogs match)
  - fetchLogsWithSampling using Logzio QueryParams with severity patterns
  - mineTemplatesWithMetadata and mineTemplates helpers
  - extractMessage and setToSlice utilities
  - Novelty detection via CompareTimeWindows

### Modified
- **internal/integration/logzio/logzio.go**
  - Import internal/logprocessing for TemplateStore
  - Add templateStore field to LogzioIntegration struct
  - Initialize templateStore in Start() with DefaultDrainConfig()
  - Instantiate PatternsTool with templateStore in RegisterTools
  - Register patterns tool with schema (47 lines added)
  - Update tool count message from "2 MCP tools" to "3 MCP tools"

## Decisions Made

**1. VictoriaLogs exact parity enforced**
- **Rationale:** AI assistants learn one patterns tool API and apply across all backends. Consistency is critical for usability.
- **Impact:** ONLY log fetching mechanism adapted - all parameters, response fields, defaults, limits identical

**2. Shared Drain infrastructure reused**
- **Rationale:** Phase 6 extracted Drain to internal/logprocessing/ specifically for multi-backend reuse
- **Impact:** No duplicate pattern mining code, single source of truth for algorithm

**3. Sampling multiplier: targetSamples * 20**
- **Rationale:** Copied from VictoriaLogs for consistency, provides good sample size (50 * 20 = 1000 logs)
- **Impact:** Balances pattern diversity vs memory/performance

**4. Previous window failure handled gracefully**
- **Rationale:** If previous window fetch fails, mark all patterns as novel rather than failing entirely
- **Impact:** Novelty detection degrades gracefully, tool remains functional

## Deviations from Plan

None - plan executed exactly as written.

All implementation matched plan specifications:
- PatternsParams, PatternsResponse, PatternTemplate types match VictoriaLogs exactly
- fetchLogsWithSampling uses Logzio QueryParams with GetErrorPattern/GetWarningPattern
- Default limit is 50, max logs range is 500-5000
- Metadata collection includes sample_log, pods, containers
- Novelty detection via CompareTimeWindows
- TemplateStore initialized in Start() with DefaultDrainConfig()
- Patterns tool registered as logzio_{name}_patterns

## Issues Encountered

None - implementation proceeded smoothly. All code compiled on first attempt, all tests passed.

## Backend Parity Verification

**Type structure comparison:**
- PatternsParams: ✓ Exact match (TimeRangeParams, namespace, severity, limit)
- PatternsResponse: ✓ Exact match (time_range, namespace, templates, total_logs, novel_count)
- PatternTemplate: ✓ Exact match (pattern, count, is_novel, sample_log, pods, containers)

**Behavior parity:**
- Default limit: ✓ 50 (matches VictoriaLogs)
- Sampling multiplier: ✓ targetSamples * 20 (matches VictoriaLogs)
- Max logs range: ✓ 500-5000 (matches VictoriaLogs)
- Novelty detection: ✓ CompareTimeWindows (matches VictoriaLogs)
- Previous window: ✓ Same duration before current (matches VictoriaLogs)
- Metadata collection: ✓ sample_log, pods, containers (matches VictoriaLogs)

**Logzio-specific adaptations:**
- Log fetching: Uses QueryParams with RegexMatch for severity filtering (Elasticsearch DSL)
- Severity patterns: GetErrorPattern() / GetWarningPattern() from severity.go
- Time range handling: Uses Logzio TimeRange struct (identical to VictoriaLogs)
- Log entry structure: LogEntry with Message field instead of VictoriaLogs _msg

## User Setup Required

None - no external service configuration required.

Pattern mining tool is automatically registered when Logzio integration is configured. See Phase 11 (Secret File Management) for Kubernetes Secret setup if using apiTokenRef.

## Next Phase Readiness

**Logzio integration complete:**
- 3 MCP tools registered: overview, logs, patterns
- Progressive disclosure workflow fully implemented
- Template storage namespace-scoped and operational
- Pattern mining reuses proven Drain algorithm

**Ready for testing:**
- Integration tests can verify all 3 tools
- End-to-end testing of progressive disclosure workflow
- Novelty detection can be validated with time-shifted queries

**VictoriaLogs parity achieved:**
- Future backends can follow same pattern: clone reference, adapt data layer only
- AI assistants have consistent tool API across Logzio and VictoriaLogs

**No blockers.**

---
*Phase: 13-mcp-tools-patterns*
*Completed: 2026-01-22*
