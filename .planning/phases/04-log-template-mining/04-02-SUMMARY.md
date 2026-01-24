---
phase: 04-log-template-mining
plan: 02
subsystem: logprocessing
tags: [drain, normalization, masking, kubernetes, regex, json]

# Dependency graph
requires:
  - phase: 04-01
    provides: Drain algorithm wrapper and template types for clustering
provides:
  - JSON message extraction for structured log preprocessing
  - Case-insensitive normalization for consistent clustering
  - Aggressive variable masking with 11+ patterns (IPs, UUIDs, timestamps, etc.)
  - Kubernetes-specific pattern detection for pod/replicaset names
  - HTTP status code preservation for semantic distinction
affects: [04-03, 05-mcp-tools]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Two-phase processing: minimal preprocessing before Drain, aggressive masking after
    - Context-aware masking: HTTP status codes preserved based on surrounding tokens
    - Kubernetes naming pattern detection: deployment-replicaset-pod format

key-files:
  created:
    - internal/logprocessing/normalize.go
    - internal/logprocessing/masking.go
    - internal/logprocessing/kubernetes.go
  modified: []

key-decisions:
  - "JSON message field extraction with fallback order: message, msg, log, text, _raw, event"
  - "Masking happens AFTER Drain clustering to preserve structure detection"
  - "HTTP status codes preserved as literals (404 vs 500 stay distinct)"
  - "Kubernetes pod/replicaset names masked with <K8S_NAME> placeholder"
  - "File path regex without word boundaries to handle slash-separated paths"

patterns-established:
  - "ExtractMessage/PreProcess for Drain input preparation"
  - "AggressiveMask for post-clustering template cleanup"
  - "MaskKubernetesNames for K8s-specific pattern handling"

# Metrics
duration: 3.5min
completed: 2026-01-21
---

# Phase 4 Plan 2: Log Normalization & Variable Masking Summary

**JSON message extraction, case-insensitive normalization, and aggressive variable masking with Kubernetes-aware patterns for stable template generation**

## Performance

- **Duration:** 3.5 min
- **Started:** 2026-01-21T14:08:39Z
- **Completed:** 2026-01-21T14:12:07Z
- **Tasks:** 3
- **Files modified:** 6 (3 implementation + 3 test files)

## Accomplishments
- Complete JSON log preprocessing with fallback to plain text
- Aggressive variable masking pipeline with 11+ regex patterns
- Kubernetes-specific pattern detection for dynamic resource names
- HTTP status code preservation for semantic log distinction
- Comprehensive test coverage with 60+ test cases across all functions

## Task Commits

Each task was committed atomically:

1. **Task 1: Create normalization logic for Drain preprocessing** - `0e1554f` (feat)
2. **Task 2: Create aggressive variable masking for post-clustering** - `81dd264` (feat)
3. **Task 3: Create Kubernetes-specific pattern masking** - `7b4ab14` (feat)

## Files Created/Modified
- `internal/logprocessing/normalize.go` - JSON message extraction and case normalization for Drain input
- `internal/logprocessing/normalize_test.go` - Test coverage for ExtractMessage and PreProcess functions
- `internal/logprocessing/masking.go` - Aggressive variable masking with 11+ patterns and status code preservation
- `internal/logprocessing/masking_test.go` - Test coverage for IP, UUID, timestamp, path, URL, email masking
- `internal/logprocessing/kubernetes.go` - K8s pod and replicaset name pattern detection
- `internal/logprocessing/kubernetes_test.go` - Test coverage for K8s naming pattern masking

## Decisions Made

**1. JSON message field extraction order**
- Try common field names in priority: message, msg, log, text, _raw, event
- Fallback to full rawLog if no message field found (structured event logs)
- Rationale: Covers most logging frameworks while allowing flexibility for event logs

**2. Two-phase processing pattern**
- PreProcess: Minimal normalization (lowercase, trim) - NO masking
- AggressiveMask: Post-clustering variable masking
- Rationale: User decision from CONTEXT.md - preserves Drain's structure detection

**3. Context-aware status code preservation**
- Check 3-token window around numbers for: status, code, http, returned, response
- Preserve number if context matches, mask otherwise
- Rationale: "returned 404" vs "returned 500" must stay distinct per user decision

**4. File path regex fix**
- Removed word boundaries (\b) from file path patterns
- Rationale: Word boundaries don't work with slash separators, causing partial matches

**5. Kubernetes pattern specificity**
- Apply pod pattern first (more specific), then replicaset pattern
- Rationale: Pod pattern is superset of replicaset pattern - order prevents partial masking

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed file path regex partial matching**
- **Found during:** Task 2 (masking_test.go failing)
- **Issue:** File path pattern `/var/log/app.log` was matching as `/var` and `/log/app.log` separately due to word boundaries
- **Fix:** Removed `\b` word boundaries from filePathPattern and windowsPathPattern regexes
- **Files modified:** internal/logprocessing/masking.go
- **Verification:** TestAggressiveMask_Paths now passes for Unix and Windows paths
- **Committed in:** 81dd264 (Task 2 commit - included in fix before final commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Bug fix necessary for correct path masking. No scope creep.

## Issues Encountered

None - all tests passed after file path regex fix.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

**Ready for next plan (04-03):**
- Complete preprocessing pipeline: JSON extraction → normalization → Drain clustering → masking
- All masking patterns implemented: IPs, UUIDs, timestamps, hex, paths, URLs, emails, K8s names
- HTTP status codes preserved for semantic distinction
- Test coverage ensures patterns work correctly

**For integration:**
- PreProcess function ready for Drain input preparation
- AggressiveMask function ready for post-clustering template cleanup
- Functions are stateless and can be called from any context

**No blockers:**
- All planned functionality complete
- Package compiles cleanly
- Comprehensive test coverage (60+ test cases)

---
*Phase: 04-log-template-mining*
*Completed: 2026-01-21*
