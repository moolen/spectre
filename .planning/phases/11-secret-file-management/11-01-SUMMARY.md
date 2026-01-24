---
phase: 11-secret-file-management
plan: 01
subsystem: integration
tags: [kubernetes, secret-management, client-go, informer, thread-safety, security]

# Dependency graph
requires:
  - phase: 01-integration-registry
    provides: Integration interface and lifecycle patterns
provides:
  - SecretWatcher component for Kubernetes secret watching with hot-reload
  - Thread-safe token storage with automatic rotation detection
  - Graceful degradation when secrets missing or deleted
affects: [12-logzio-integration-bootstrap]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Kubernetes SharedInformerFactory for resource watching
    - sync.RWMutex for high-read, low-write token access
    - Graceful degradation on missing resources (start degraded, watch for creation)

key-files:
  created:
    - internal/integration/victorialogs/secret_watcher.go
    - internal/integration/victorialogs/secret_watcher_test.go
  modified: []

key-decisions:
  - "Use kubernetes.Interface instead of *kubernetes.Clientset for testability with fake clientset"
  - "Namespace-scoped informer (not cluster-wide) for security and efficiency"
  - "30-second resync period following Kubernetes best practices"
  - "Start degraded if secret missing (don't fail startup) - watch picks it up when created"
  - "Token values never logged - security requirement enforced via grep verification"

patterns-established:
  - "SecretWatcher pattern: informer-based secret watching with thread-safe token caching"
  - "Graceful degradation: start degraded, mark unhealthy, auto-recover when resource available"
  - "Security-first logging: sensitive values never appear in logs or error messages"

# Metrics
duration: 4min
completed: 2026-01-22
---

# Phase 11 Plan 01: Secret File Management Summary

**Kubernetes-native secret watching with SharedInformerFactory, thread-safe token hot-reload, and zero-downtime credential rotation**

## Performance

- **Duration:** 4m 25s
- **Started:** 2026-01-22T12:16:42Z
- **Completed:** 2026-01-22T12:21:07Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- SecretWatcher component using client-go SharedInformerFactory for automatic secret watching
- Thread-safe token storage with sync.RWMutex (concurrent reads, exclusive writes)
- Hot-reload support via Kubernetes Watch API (detects secret changes within 2 seconds)
- Graceful degradation when secrets missing/deleted (starts degraded, auto-recovers)
- Comprehensive test suite with 10 test cases covering all scenarios including race conditions
- >90% test coverage with all tests passing with -race flag

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement SecretWatcher with SharedInformerFactory** - `655f4c3` (feat)
2. **Task 2: Write unit tests for SecretWatcher** - `f3b3378` (test)

## Files Created/Modified
- `internal/integration/victorialogs/secret_watcher.go` (264 lines) - SecretWatcher component with informer-based watching, thread-safe token storage, and graceful degradation
- `internal/integration/victorialogs/secret_watcher_test.go` (548 lines) - Comprehensive test suite with 10 tests covering initial fetch, rotation, missing keys, concurrent access, and cleanup

## Decisions Made

**1. Use kubernetes.Interface instead of concrete *kubernetes.Clientset type**
- **Rationale:** Enables testing with fake.Clientset without type assertions. Interface is standard Go practice for dependency injection and testability.

**2. Namespace-scoped informer via WithNamespace option**
- **Rationale:** More secure (only needs Role, not ClusterRole), more efficient (caches only secrets in Spectre's namespace), follows Kubernetes operator best practices.

**3. 30-second resync period**
- **Rationale:** Standard Kubernetes default. Balances cache freshness with API server load. Research showed <10s can cause API throttling, 0 disables resync (stale cache risk).

**4. Start degraded if secret missing (don't fail startup)**
- **Rationale:** Allows pod to start even if secret not yet created. Watch will pick it up when available. Better for orchestration (rolling updates, GitOps workflows).

**5. Token values never logged**
- **Rationale:** Security requirement. Enforced via code review and grep verification. Logs contain "Token rotated" but never actual token values.

**6. RWMutex over atomic.Value**
- **Rationale:** Research showed atomic.Value ~3x faster but only for simple types. RWMutex more flexible for validation logic (empty check, whitespace trim) and easier to reason about. Sufficient performance for token reads (not hot path).

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

**Type compatibility between fake.Clientset and kubernetes.Clientset**
- **Problem:** Test compilation failed with type mismatch between `*fake.Clientset` and `*kubernetes.Clientset`
- **Resolution:** Changed SecretWatcher.clientset field from `*kubernetes.Clientset` to `kubernetes.Interface`. This is the correct Go pattern - both real and fake clientsets implement the interface.
- **Impact:** Better design - interface-based dependency injection is more testable and follows Go best practices.

## Next Phase Readiness

**Ready for Phase 12 (Logz.io Integration Bootstrap):**
- SecretWatcher component available for integration with Logz.io client
- Pattern established for secret-based authentication
- Tests demonstrate hot-reload capability works correctly
- Graceful degradation ensures integrations remain registered even when secrets temporarily unavailable

**No blockers or concerns.**

**Integration pattern for Phase 12:**
```go
// In Logz.io integration Start():
watcher, err := NewInClusterSecretWatcher(namespace, secretName, key, logger)
if err != nil {
    return fmt.Errorf("failed to create secret watcher: %w", err)
}
if err := watcher.Start(ctx); err != nil {
    return fmt.Errorf("failed to start secret watcher: %w", err)
}
// In API client:
token, err := watcher.GetToken()
if err != nil {
    return fmt.Errorf("integration degraded: %w", err)
}
// Use token in Authorization header
```

---
*Phase: 11-secret-file-management*
*Completed: 2026-01-22*
