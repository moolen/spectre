---
phase: 04-log-template-mining
plan: 03
subsystem: log-processing
tags: [drain, template-storage, persistence, json, namespace-scoping, concurrency]

# Dependency graph
requires:
  - phase: 04-01
    provides: DrainProcessor wrapper and Template types with SHA-256 hashing
  - phase: 04-02
    provides: PreProcess, AggressiveMask, and Kubernetes name masking functions
provides:
  - Namespace-scoped template storage (TemplateStore)
  - Per-namespace Drain instances for multi-tenant isolation
  - Periodic JSON snapshots with atomic writes (5-minute interval)
  - Template persistence and restoration on startup
affects:
  - 04-04 (template lifecycle management will use this storage)
  - Phase 5 (MCP tools will query templates via TemplateStore interface)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Namespace-scoped storage with per-namespace Drain instances
    - Double-checked locking for thread-safe lazy initialization
    - Atomic writes using temp-file-then-rename (POSIX atomicity)
    - Pattern normalization for stable template IDs (<IP>, <UUID> → <VAR>)
    - Periodic snapshot loop with graceful shutdown and final snapshot

key-files:
  created:
    - internal/logprocessing/store.go
    - internal/logprocessing/store_test.go
    - internal/logprocessing/persistence.go
    - internal/logprocessing/persistence_test.go
  modified: []

key-decisions:
  - "Normalize all placeholders (<IP>, <UUID>, etc.) to <VAR> for template ID generation while preserving semantic patterns for display"
  - "Pattern normalization ensures consistent template IDs regardless of when Drain learns pattern (first literal vs subsequent wildcards)"
  - "Deep copy templates on Get/List to prevent external mutation"
  - "Load errors don't crash server - start with empty state if snapshot corrupted"
  - "Failed snapshots logged but don't stop periodic loop (lose max 5 min on crash)"

patterns-established:
  - "getOrCreateNamespace uses double-checked locking: fast read path, slow write path with recheck"
  - "PersistenceManager Start() blocks until context cancel or Stop(), performs final snapshot"
  - "Snapshot serialization: lock store.mu for read → lock each namespace for read → build snapshot → marshal → atomic write"

# Metrics
duration: 8min
completed: 2026-01-21
---

# Phase 4 Plan 3: Template Storage & Persistence Summary

**Namespace-scoped template storage with per-namespace Drain instances and periodic JSON snapshots using atomic writes**

## Performance

- **Duration:** 8 min 19 sec
- **Started:** 2026-01-21T14:14:55Z
- **Completed:** 2026-01-21T14:23:14Z
- **Tasks:** 2
- **Files modified:** 4 (all created)

## Accomplishments

- TemplateStore integrates PreProcess → Drain → AggressiveMask → normalization pipeline
- Pattern normalization ensures stable template IDs across Drain learning phases
- Periodic persistence with 5-minute snapshots prevents data loss on crashes
- Atomic writes (temp + rename) prevent snapshot corruption
- Comprehensive test coverage: 30+ tests including concurrency and roundtrip serialization

## Task Commits

Each task was committed atomically:

1. **Task 1: Create namespace-scoped template storage** - `ac786b0` (feat)
   - TemplateStore with per-namespace DrainProcessor instances
   - Process() integrates full pipeline: PreProcess → Train → AggressiveMask → normalize → hash
   - GetTemplate, ListTemplates, GetNamespaces accessors
   - Thread-safe with RWMutex for concurrent access
   - 11 tests including concurrency, JSON logs, namespace scoping

2. **Task 2: Create periodic persistence with atomic writes** - `d870b38` (feat)
   - PersistenceManager with Start/Stop lifecycle methods
   - Snapshot() creates JSON with atomic temp-file-then-rename
   - Load() restores templates from JSON on startup
   - Schema versioning (version=1) for future migrations
   - 11 tests including corrupted JSON, version checks, periodic snapshots

## Files Created/Modified

- `internal/logprocessing/store.go` - TemplateStore with namespace scoping and Process() integration
- `internal/logprocessing/store_test.go` - 11 tests for storage, namespace isolation, concurrency
- `internal/logprocessing/persistence.go` - PersistenceManager with periodic snapshots and atomic writes
- `internal/logprocessing/persistence_test.go` - 11 tests for snapshot/load, atomicity, lifecycle

## Decisions Made

**Pattern normalization for stable template IDs:**
- Issue: First log gets masked to "connected to <IP>", but once Drain learns pattern, subsequent logs return "connected to <*>", causing different template IDs
- Solution: Normalize ALL placeholders (<*>, <IP>, <UUID>, <NUM>, etc.) to canonical <VAR> for ID generation
- Rationale: Ensures consistent template IDs regardless of when Drain learns the pattern
- Implementation: Generate ID from normalized pattern, but store semantic masked pattern for display and tokens
- Impact: Templates have stable IDs across server restarts and Drain evolution

**Load errors don't crash server:**
- Corrupted snapshots return error but server continues with empty state
- User decision: "Start empty on first run" - missing snapshot is acceptable
- Rationale: One corrupted snapshot shouldn't prevent server startup
- Pattern: Same as integration config loading - resilience over strict validation

**Deep copy on template retrieval:**
- GetTemplate and ListTemplates return deep copies of templates
- Prevents external code from mutating internal template state
- Follows defensive programming pattern for shared state

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Pattern extraction from Drain cluster output**
- **Found during:** Task 1 (store.go implementation)
- **Issue:** cluster.String() returns format "id={X} : size={Y} : [pattern]" not just pattern
- **Fix:** Added extractPattern() helper to extract pattern after last " : " separator
- **Files modified:** internal/logprocessing/store.go
- **Verification:** Test passed showing pattern "connected to <IP>" not full cluster string
- **Committed in:** ac786b0 (Task 1 commit)

**2. [Rule 1 - Bug] Pattern normalization for consistent template IDs**
- **Found during:** Task 1 testing (TestProcessSameTemplateTwice)
- **Issue:** First log masked to "<IP>", second to "<VAR>" (Drain's <*>), causing different template IDs
- **Fix:** Added normalizeDrainWildcards() to normalize ALL placeholders to <VAR> for ID generation
- **Files modified:** internal/logprocessing/store.go
- **Verification:** TestProcessSameTemplateTwice passed - both logs map to same template with count=2
- **Committed in:** ac786b0 (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (2 bugs)
**Impact on plan:** Both bugs discovered during testing. Pattern extraction fixed Drain API mismatch. Normalization fixed fundamental inconsistency in template ID generation. Both essential for correctness.

## Issues Encountered

None - tests passed after auto-fixes.

## Next Phase Readiness

**Ready for Plan 04-04 (Template Lifecycle Management):**
- Template storage complete with stable IDs and occurrence tracking
- Persistence ensures templates survive restarts
- Count tracking ready for pruning low-frequency templates
- Pattern tokens ready for similarity-based auto-merge

**Ready for Phase 5 (MCP Tools):**
- TemplateStore provides clean interface: GetTemplate, ListTemplates, GetNamespaces
- Namespace scoping supports multi-tenant queries
- Thread-safe for concurrent MCP tool requests

**No blockers or concerns.**

---
*Phase: 04-log-template-mining*
*Completed: 2026-01-21*
