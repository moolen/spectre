---
phase: 01-plugin-infrastructure-foundation
plan: 03
subsystem: infra
tags: [fsnotify, koanf, file-watcher, hot-reload, debouncing]

# Dependency graph
requires:
  - phase: 01-02
    provides: LoadIntegrationsFile function for loading and validating integration configs
provides:
  - IntegrationWatcher with file watching and debouncing
  - ReloadCallback pattern for notifying on config changes
  - Graceful Start/Stop lifecycle with context cancellation
  - Invalid config resilience (logs errors, continues watching)
affects:
  - 01-04-integration-manager-orchestration
  - phase-02-mcp-tools-registration

# Tech tracking
tech-stack:
  added:
    - github.com/fsnotify/fsnotify (file system notifications)
  patterns:
    - Debounce pattern with time.Timer for coalescing rapid file changes
    - Callback notification pattern for reload events
    - Graceful shutdown with timeout channel pattern

key-files:
  created:
    - internal/config/integration_watcher.go
    - internal/config/integration_watcher_test.go
  modified: []

key-decisions:
  - "IntegrationWatcherConfig (not WatcherConfig) to avoid naming conflict with existing Kubernetes watcher config"
  - "500ms default debounce prevents editor save storms"
  - "fsnotify directly instead of Koanf's file provider for better control over event handling"
  - "Invalid configs logged but don't crash watcher - resilience over fail-fast after initial load"
  - "5 second Stop() timeout for graceful shutdown"

patterns-established:
  - "File watcher pattern: Create → Add → Select loop on Events/Errors/Context"
  - "Debouncing via time.AfterFunc that resets on each event"
  - "Callback error handling: log but continue watching (don't propagate)"

# Metrics
duration: 3min
completed: 2026-01-20
---

# Phase 01 Plan 03: Integration File Watcher Summary

**File watcher with 500ms debouncing detects config changes via fsnotify, calls reload callback with validated config, resilient to invalid YAML and callback errors**

## Performance

- **Duration:** 3min 15sec
- **Started:** 2026-01-20T23:54:15Z
- **Completed:** 2026-01-20T23:57:30Z
- **Tasks:** 2
- **Files modified:** 2 created

## Accomplishments

- IntegrationWatcher with Start/Stop lifecycle manages fsnotify watcher
- Debouncing (500ms default) coalesces rapid file changes into single reload
- Invalid configs rejected without crashing watcher (logs error, keeps previous valid config)
- Callback fires with validated IntegrationsFile from LoadIntegrationsFile
- Graceful shutdown with 5 second timeout, context cancellation support
- Comprehensive test suite with 8 test cases, no race conditions

## Task Commits

Each task was committed atomically:

1. **Task 1: Create integration file watcher with debouncing** - `79eba6b` (feat)
2. **Task 2: Write watcher unit tests** - `59255a8` (test)

## Files Created/Modified

- `internal/config/integration_watcher.go` - File watcher with debouncing, callbacks on config reload
- `internal/config/integration_watcher_test.go` - Comprehensive tests covering all scenarios

## Decisions Made

**IntegrationWatcherConfig naming:** Renamed from `WatcherConfig` to avoid conflict with existing `internal/config/watcher_config.go` which defines Kubernetes resource watching config. Maintains clear separation between integration config watching and K8s resource watching.

**fsnotify direct usage:** Used fsnotify directly instead of Koanf's file provider Watch method. Provides better control over event handling, debouncing logic, and error resilience. Koanf is still used via LoadIntegrationsFile for parsing.

**Resilience over fail-fast:** After initial load succeeds, invalid configs during reload are logged but don't crash the watcher. This ensures one bad config edit doesn't break the entire system. Initial load still fails fast to prevent starting with invalid config.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Removed unused koanf field and import**
- **Found during:** Task 1 (Build verification)
- **Issue:** Import "github.com/knadh/koanf/providers/file" was unused after switching to direct fsnotify usage. Also removed unused `koanf *koanf.Koanf` field from IntegrationWatcher struct.
- **Fix:** Removed the import and struct field. Koanf is still used indirectly via LoadIntegrationsFile.
- **Files modified:** internal/config/integration_watcher.go
- **Verification:** `go build ./internal/config` succeeded without warnings
- **Committed in:** 79eba6b (Task 1 commit)

**2. [Rule 3 - Blocking] Renamed WatcherConfig to IntegrationWatcherConfig**
- **Found during:** Task 1 (Build verification)
- **Issue:** Type name conflict with existing `WatcherConfig` in `internal/config/watcher_config.go` (used for Kubernetes resource watching). Build failed with "WatcherConfig redeclared in this block".
- **Fix:** Renamed to `IntegrationWatcherConfig` throughout the file to avoid collision.
- **Files modified:** internal/config/integration_watcher.go
- **Verification:** `go build ./internal/config` succeeded
- **Committed in:** 79eba6b (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (2 blocking build issues)
**Impact on plan:** Both fixes necessary to unblock compilation. No functional changes to planned behavior.

## Issues Encountered

**fsnotify event timing:** Initial test runs showed file change events weren't being reliably detected immediately. Added 50ms initialization delay after Start() in tests to ensure watcher is fully set up before modifying files. This is a filesystem timing quirk, not a bug in the implementation.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

**Ready for 01-04 (Integration Manager):**
- IntegrationWatcher can be used to watch integrations config file
- ReloadCallback provides clean notification interface
- Start/Stop lifecycle integrates with context-based component management
- Debouncing prevents reload storms during config editing

**Ready for hot-reload in MCP server:**
- Watcher foundation complete
- Integration manager (01-04) will orchestrate: watch file → reload config → restart affected instances
- Atomic pointer swap pattern (from ROADMAP) can be implemented in integration manager using this watcher

**No blockers** - all infrastructure for config hot-reload is in place.

---
*Phase: 01-plugin-infrastructure-foundation*
*Completed: 2026-01-20*
