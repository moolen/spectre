---
phase: 11-secret-file-management
plan: 04
subsystem: infra
tags: [helm, kubernetes, rbac, secrets]

# Dependency graph
requires:
  - phase: 11-03
    provides: SecretWatcher implementation for hot-reload
provides:
  - Namespace-scoped RBAC (Role + RoleBinding) for Kubernetes Secret access
  - Helm chart configuration for secret-based authentication
  - Conditional RBAC rendering via values.yaml
affects:
  - 11-05 (will use these RBAC permissions for ConfigMap secret references)
  - 12-logzio (will use secret-based authentication)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Conditional Helm template rendering with .Values flags"
    - "Namespace-scoped RBAC for least privilege"

key-files:
  created:
    - chart/templates/role.yaml
    - chart/templates/rolebinding.yaml
  modified:
    - chart/values.yaml

key-decisions:
  - "Use namespace-scoped Role instead of ClusterRole for security"
  - "Default rbac.secretAccess.enabled to true for v1.2+"
  - "Conditional rendering allows opt-out for existing installations"

patterns-established:
  - "Pattern 1: RBAC templates conditionally rendered via .Values.rbac.* flags"
  - "Pattern 2: Secret access limited to Spectre's namespace only"

# Metrics
duration: 1m 42s
completed: 2026-01-22
---

# Phase 11 Plan 04: Helm RBAC Templates Summary

**Namespace-scoped Role and RoleBinding for Kubernetes Secret access with conditional rendering**

## Performance

- **Duration:** 1 min 42 sec
- **Started:** 2026-01-22T12:16:34Z
- **Completed:** 2026-01-22T12:18:16Z
- **Tasks:** 3
- **Files modified:** 3

## Accomplishments
- Created namespace-scoped Role granting get/watch/list on secrets
- Created RoleBinding connecting ServiceAccount to Role
- Added rbac.secretAccess.enabled configuration to values.yaml
- Enabled conditional rendering (default enabled for v1.2+)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create Role template for secret access** - `bf959bc` (feat)
2. **Task 2: Create RoleBinding template** - `3c75bc3` (feat)
3. **Task 3: Add values.yaml configuration for RBAC** - `ca9890b` (feat)

## Files Created/Modified
- `chart/templates/role.yaml` - Namespace-scoped Role for secret get/watch/list
- `chart/templates/rolebinding.yaml` - Connects ServiceAccount to secret-reader Role
- `chart/values.yaml` - Added rbac.secretAccess.enabled (default true)

## Decisions Made

**1. Namespace-scoped Role over ClusterRole**
- Follows principle of least privilege
- Prevents reading secrets from other namespaces
- More secure for multi-tenant clusters
- Simplifies RBAC setup (no cluster-admin required)

**2. Default enabled for v1.2+**
- v1.2 introduces secret-based authentication (Logz.io)
- "Just works" experience for secret rotation
- Can be disabled via --set rbac.secretAccess.enabled=false
- Existing installations: no impact if secrets unused

**3. Conditional rendering pattern**
- Uses .Values.rbac.secretAccess.enabled flag
- Both Role and RoleBinding conditionally rendered
- Consistent with existing Helm chart patterns

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - all tasks completed successfully on first attempt.

## Next Phase Readiness

**Ready for Phase 11-05 (ConfigMap Secret References):**
- RBAC permissions in place for SecretWatcher
- ServiceAccount has get/watch/list access to secrets
- Conditional rendering allows opt-in/opt-out
- Helm chart renders without errors

**No blockers or concerns.**

---
*Phase: 11-secret-file-management*
*Completed: 2026-01-22*
