---
phase: 11-secret-file-management
plan: 02
subsystem: integration
tags: [victorialogs, kubernetes, secrets, config, validation]

# Dependency graph
requires:
  - phase: 11-secret-file-management
    provides: Phase context and research on secret management approach
provides:
  - SecretRef type for referencing Kubernetes Secrets
  - Config struct with URL and optional APITokenRef
  - Validation logic for mutually exclusive authentication methods
  - Helper methods for secret-based config detection
affects: [11-03, 11-04, 10-logzio-integration]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "SecretRef pattern for Kubernetes Secret references"
    - "Config.Validate() for mutual exclusivity checks"
    - "Pointer types for optional fields (APITokenRef)"

key-files:
  created: []
  modified:
    - internal/integration/victorialogs/types.go
    - internal/integration/victorialogs/types_test.go

key-decisions:
  - "SecretRef omits namespace field - secrets always in same namespace as Spectre"
  - "APITokenRef is pointer type (*SecretRef) for optional/backward compatibility"
  - "Validation checks for URL-embedded credentials via @ pattern detection"
  - "UsesSecretRef() helper enables clean conditional logic for auth method"

patterns-established:
  - "SecretRef struct pattern: secretName + key fields for K8s Secret references"
  - "Config.Validate() pattern: check required fields, then mutual exclusivity, then conditional validation"
  - "Test structure: table-driven tests with name/config/wantErr/errContains"

# Metrics
duration: 2min
completed: 2026-01-22
---

# Phase 11 Plan 02: Config Type Extensions Summary

**VictoriaLogs Config struct with SecretRef support and validation for mutually exclusive authentication methods**

## Performance

- **Duration:** 2 minutes 3 seconds
- **Started:** 2026-01-22T12:16:33Z
- **Completed:** 2026-01-22T12:18:36Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- Added SecretRef type for Kubernetes Secret references with secretName and key fields
- Created Config struct with URL and optional APITokenRef for secret-based authentication
- Implemented Validate() method enforcing mutual exclusivity between URL-embedded credentials and SecretRef
- Added UsesSecretRef() helper for clean conditional logic
- Comprehensive test coverage with 11 test cases covering all validation scenarios

## Task Commits

Each task was committed atomically:

1. **Task 1: Add SecretRef to Config types** - `71eb77c` (feat)
2. **Task 2: Write unit tests for Config validation** - `b600791` (test)

## Files Created/Modified
- `internal/integration/victorialogs/types.go` - Added SecretRef struct, Config struct with URL and APITokenRef, Validate() and UsesSecretRef() methods
- `internal/integration/victorialogs/types_test.go` - Added TestConfig_Validate (7 cases) and TestConfig_UsesSecretRef (4 cases)

## Decisions Made
- **SecretRef omits namespace field:** Secrets are always assumed to be in the same namespace as Spectre deployment (from 11-CONTEXT.md decision). This simplifies configuration and follows Kubernetes best practices for co-located resources.
- **APITokenRef is pointer type:** Using `*SecretRef` makes the field optional and enables backward compatibility with existing configs that only have URL.
- **URL @ pattern for credential detection:** Validation checks for `@` character in URL to detect URL-embedded credentials (basic auth pattern like `http://user:pass@host`). This is defensive - VictoriaLogs might support basic auth.
- **UsesSecretRef() helper:** Provides clean boolean check for secret-based config, encapsulating the logic of "non-nil APITokenRef with non-empty SecretName".

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - implementation proceeded smoothly. The existing `secret_watcher.go` file (from future work) initially caused build issues due to missing Kubernetes dependencies, but `go mod tidy` resolved this automatically as the dependencies were already present in go.mod.

## User Setup Required

None - no external service configuration required. This is pure type definition and validation logic.

## Next Phase Readiness

**Ready for next phase (11-03: VictoriaLogs Factory Updates)**

The Config struct is now ready to be used in the VictoriaLogs integration factory. Next steps:
- Update `NewVictoriaLogsIntegration` to use Config struct instead of raw map
- Add config parsing and validation during integration initialization
- Handle both static URL configs and secret-based configs

**No blockers.**

The validation logic is comprehensive and tested. The mutual exclusivity check prevents misconfiguration. The pattern is ready to be replicated for Logz.io integration in Phase 10.

---
*Phase: 11-secret-file-management*
*Completed: 2026-01-22*
