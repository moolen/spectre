---
phase: 08-cleanup-helm-update
plan: 01
subsystem: infra
tags: [cli, commands, cleanup, go, cobra]

# Dependency graph
requires:
  - phase: 07-service-layer-extraction
    provides: HTTP client removed, service-only architecture
provides:
  - Clean CLI with only server and debug commands
  - Removed 14,676 lines of dead code (74 files)
  - No standalone MCP/agent/mock commands
affects: [08-02-helm-chart-update, deployment]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Consolidated server CLI pattern - single spectre server command"

key-files:
  created: []
  modified:
    - cmd/spectre/commands/root.go
  deleted:
    - cmd/spectre/commands/mcp.go
    - cmd/spectre/commands/mcp_health_test.go
    - cmd/spectre/commands/agent.go
    - cmd/spectre/commands/mock.go
    - internal/agent/ (entire package, 70 files)

key-decisions:
  - "Complete deletion approach - no TODO comments, no deprecation stubs, clean removal"
  - "Debug command kept even though it has no subcommands (for future debug utilities)"

patterns-established:
  - "Clean deletion pattern: rm files, remove registrations, verify build, commit atomically"

# Metrics
duration: 191s
completed: 2026-01-21
---

# Phase 08 Plan 01: Remove Standalone Commands Summary

**Deleted 14,676 lines of dead code including standalone MCP/agent/mock commands and entire internal/agent package after Phase 7 HTTP client removal**

## Performance

- **Duration:** 3 min 11 sec
- **Started:** 2026-01-21T20:36:39Z
- **Completed:** 2026-01-21T20:39:50Z
- **Tasks:** 3
- **Files deleted:** 74

## Accomplishments
- Removed standalone `spectre mcp` command (disabled in Phase 7)
- Removed `spectre agent` command (disabled in Phase 7)
- Removed `spectre mock` command (build-disabled, imported agent package)
- Deleted entire internal/agent package (70 files, all build-disabled)
- Cleaned root.go command registration
- Verified binary builds successfully with only server and debug commands

## Task Commits

Each task was committed atomically:

1. **Task 1: Delete standalone command files and agent package** - `15f7370` (chore)
   - Deleted 74 files totaling 14,676 lines
   - Commands: mcp.go, mcp_health_test.go, agent.go, mock.go
   - Package: entire internal/agent/ directory

2. **Task 2: Remove command registrations from root.go** - `8b3938e` (chore)
   - Removed rootCmd.AddCommand(mcpCmd) from init()
   - Only serverCmd and debugCmd remain

3. **Task 3: Verify Go build succeeds** - *(no commit - verification only)*
   - Build completed successfully
   - Binary shows only server command in Available Commands
   - Debug command in Additional Help Topics (has no subcommands)
   - Unknown command handling works correctly

## Files Created/Modified
- `cmd/spectre/commands/root.go` - Removed mcpCmd registration
- **Deleted:**
  - `cmd/spectre/commands/mcp.go` - Standalone MCP server command
  - `cmd/spectre/commands/mcp_health_test.go` - MCP command tests
  - `cmd/spectre/commands/agent.go` - Interactive AI agent command
  - `cmd/spectre/commands/mock.go` - Mock LLM command (imported agent package)
  - `internal/agent/` - Entire package (70 files: audit, commands, incident, model, multiagent, provider, runner, tools, tui)

## Decisions Made
- **Complete deletion approach**: No TODO comments or deprecation stubs added, per Phase 8 context decision for clean removal
- **Debug command kept**: Even though debugCmd has no subcommands currently, kept it registered for future debug utilities (appears in "Additional help topics")
- **Verified Cobra handling**: Confirmed Cobra's automatic unknown command error messages work correctly for deleted commands

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - all deletions and verification completed without issues.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

**Ready for Helm chart updates:**
- CLI surface now matches consolidated server architecture
- Only `spectre server` command needed in Helm deployment
- Standalone MCP/agent deployment manifests can be removed
- Binary is smaller (14,676 lines removed) and cleaner

**No blockers or concerns.**

---
*Phase: 08-cleanup-helm-update*
*Completed: 2026-01-21*
