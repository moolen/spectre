# Implementation Tasks: Graceful Component Initialization and Shutdown

**Feature**: 003-graceful-initialization
**Branch**: `003-graceful-initialization`
**Created**: 2025-11-25
**Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md) | **Data Model**: [data-model.md](data-model.md)

## Overview

This document contains the implementation task checklist for graceful component initialization and shutdown. Tasks are organized by user story to enable independent development and testing.

**Total Tasks**: 28
**Phases**: 5 (Setup + Foundational + 3 User Stories + Polish)
**Recommended MVP**: Complete Phase 3 (User Story 1: Application Startup)

## Dependency Graph

```
Phase 1: Setup (P: parallelizable)
    ↓
Phase 2: Foundational (Component Interface & Manager)
    ↓
Phase 3: [US1] Application Startup
    ├─ Independent: Can test with test components
    ├─ Blocks: US2, US3, US4
    └─ MVP completion point
    ↓
Phase 4: [US2] Graceful Shutdown on SIGINT
    ├─ Depends on: US1
    ├─ Independent: Can test shutdown in isolation
    └─ Adds: Signal handling + shutdown logic
    ↓
Phase 5: [US3] Shutdown Timeout Handling
    ├─ Depends on: US2
    └─ Adds: Timeout protection and forced termination
    ↓
Phase 6: [US4] Component Lifecycle Coordination
    ├─ Depends on: US1, US2, US3
    └─ Adds: Multi-component validation tests
    ↓
Phase 7: Polish & Integration
    ├─ Cross-cutting concerns
    └─ End-to-end integration tests

## Parallel Execution Opportunities

**Phase 1 (Setup)**: All tasks parallelizable (different directories)
- [ ] T001 and T002 can run in parallel (no dependencies)

**Phase 2 (Foundational)**: All tasks parallelizable (different packages)
- [ ] T003, T004, T005, T006 can run in parallel (independent implementations)

**Phase 3 (US1)**: Some tasks parallelizable
- [ ] T007, T009, T011 can run in parallel (different components)
- [ ] T008, T010, T012 depend on their respective component's test/interface

**Phase 4-6**: Sequential (each phase depends on previous)

## Implementation Strategy

**MVP Scope**: Phase 3 (User Story 1: Application Startup)
- Delivers: Core component lifecycle system with dependency ordering
- Value: Can initialize components in correct order
- Testing: Can verify startup sequence with test doubles
- Time: ~3-4 tasks (~2-3 hours)

**Phase 1 MVP Extension**: Add Phase 4 (Graceful Shutdown)
- Adds: Signal handling and graceful shutdown
- Value: Full startup + shutdown cycle works
- Testing: Can test graceful shutdown scenarios
- Time: ~4 more tasks (~2-3 hours)

---

## Phase 1: Setup & Project Structure

### Phase Goal
Initialize project structure and create interface definitions that all components will implement.

### Independent Test Criteria
✅ Files created with correct structure
✅ No runtime testing needed for this phase

---

- [x] T001 Create lifecycle package structure with `internal/lifecycle/` directory
- [x] T002 Create test integration directory at `tests/integration/` for lifecycle tests

---

## Phase 2: Foundational - Component Interface & Manager Implementation

### Phase Goal
Implement the core component lifecycle interface and manager that orchestrates startup/shutdown.

### Independent Test Criteria
✅ Component interface can be implemented by test stubs
✅ Manager can register, start, and stop test components
✅ Dependency validation works (detects circular dependencies)

---

- [x] T003 Implement Component interface in `internal/lifecycle/component.go` with Start, Stop, Name methods
- [x] T004 Create Manager struct in `internal/lifecycle/manager.go` with fields for components and dependencies
- [x] T005 Implement Manager.Register() method in `internal/lifecycle/manager.go` with dependency validation
- [x] T006 Implement Manager.Start() method in `internal/lifecycle/manager.go` with dependency-ordered startup and rollback on failure

---

## Phase 3: [US1] Application Startup

### User Story
When the application starts, all required components must be initialized in correct dependency order.

### Acceptance Scenarios
1. All components (watchers, storage, API server) initialized in dependency order
2. Each successful component initialization is logged with timestamp
3. Component failure during startup triggers cleanup and non-zero exit

### Independent Test Criteria
✅ Can register and start mock components
✅ Can verify startup order through logs
✅ Can test failure handling with failing mock components
✅ Can run independently of shutdown functionality

---

- [x] T007 [P] [US1] Implement Storage component to meet Component interface in `internal/storage/storage.go`
- [ ] T008 [US1] Add unit test for Storage.Start() in `tests/unit/storage_lifecycle_test.go`
- [x] T009 [P] [US1] Implement Watcher component to meet Component interface in `internal/watcher/watcher.go`
- [ ] T010 [US1] Add unit test for Watcher.Start() in `tests/unit/watcher_lifecycle_test.go`
- [x] T011 [P] [US1] Implement API Server component to meet Component interface in `internal/api/server.go`
- [ ] T012 [US1] Add unit test for APIServer.Start() in `tests/unit/api_lifecycle_test.go`
- [ ] T013 [US1] Create integration test for startup sequence in `tests/integration/startup_test.go` (tests dependency order and logging)
- [x] T014 [US1] Update cmd/main.go to register components and call manager.Start() with proper error handling and exit codes
- [ ] T015 [US1] Add integration test for startup failure scenario in `tests/integration/startup_test.go` (validates rollback and cleanup)

---

## Phase 4: [US2] Graceful Shutdown on SIGINT

### User Story
When SIGINT is received, all components must gracefully stop in reverse dependency order with proper logging.

### Acceptance Scenarios
1. SIGINT signal triggers shutdown of all components
2. Components shutdown in reverse dependency order (API → Watchers → Storage)
3. Each component shutdown is logged with timestamp
4. Process exits with status code 0

### Independent Test Criteria
✅ Can send SIGINT to running application
✅ Can verify shutdown order through logs
✅ Can verify process exit code
✅ Can test with blocked components (don't actually complete)

---

- [ ] T016 Implement Manager.Stop() method in `internal/lifecycle/manager.go` with reverse-order shutdown and logging
- [ ] T017 Implement Manager.IsRunning() method in `internal/lifecycle/manager.go` to check component status
- [ ] T018 [P] [US2] Implement Storage.Stop() in `internal/storage/storage.go` with graceful shutdown and context deadline respect
- [ ] T019 [US2] Add unit test for Storage.Stop() in `tests/unit/storage_lifecycle_test.go`
- [ ] T020 [P] [US2] Implement Watcher.Stop() in `internal/watcher/watcher.go` with graceful shutdown
- [ ] T021 [US2] Add unit test for Watcher.Stop() in `tests/unit/watcher_lifecycle_test.go`
- [ ] T022 [P] [US2] Implement APIServer.Stop() in `internal/api/server.go` with graceful shutdown
- [ ] T023 [US2] Add unit test for APIServer.Stop() in `tests/unit/api_lifecycle_test.go`
- [ ] T024 [US2] Install signal handler in cmd/main.go to catch SIGINT/SIGTERM and call manager.Stop()
- [ ] T025 [US2] Create integration test for graceful shutdown in `tests/integration/shutdown_test.go` (verifies order and logging)
- [ ] T026 [US2] Create integration test for signal handling in `tests/integration/signal_handling_test.go` (sends SIGINT and verifies shutdown)

---

## Phase 5: [US3] Shutdown Timeout Handling

### User Story
Components that exceed the 30-second grace period are forcefully terminated with timeout warnings logged.

### Acceptance Scenarios
1. Component shutdown taking longer than grace period is forcefully terminated
2. Timeout event is logged as a warning with component name
3. Shutdown continues with remaining components despite timeout

### Independent Test Criteria
✅ Can simulate slow component with deliberate delays
✅ Can verify timeout occurs and log warnings appear
✅ Can verify other components still shutdown after timeout

---

- [ ] T027 [US3] Add timeout handling to Manager.Stop() in `internal/lifecycle/manager.go` using context.WithTimeout per component
- [ ] T028 [US3] Create integration test for timeout handling in `tests/integration/shutdown_test.go` (tests slow component scenario)

---

## Phase 6: [US4] Component Lifecycle Coordination

### User Story
Multiple components with dependencies initialize and shutdown in correct order without race conditions.

### Acceptance Scenarios
1. Dependencies initialize before dependents
2. Dependents shutdown before dependencies
3. All dependencies remain active until dependents fully shutdown

### Independent Test Criteria
✅ Can register multiple components with dependencies
✅ Can verify startup order matches declared dependencies
✅ Can verify shutdown order is reverse of startup
✅ Runs after US1, US2, US3 are complete

---

- [ ] T029 [US4] Create integration test for multi-component coordination in `tests/integration/startup_test.go` (tests full dependency graph)
- [ ] T030 [US4] Create integration test for dependency validation in `tests/integration/startup_test.go` (rejects circular dependencies)

---

## Phase 7: Polish & Cross-Cutting Concerns

### Phase Goal
Final validation, performance testing, and documentation updates.

### Independent Test Criteria
✅ All acceptance scenarios pass
✅ Startup completes within 5 seconds (SC-001)
✅ Shutdown completes within 30 seconds (SC-002)
✅ All events logged (SC-004)
✅ Exit codes correct (SC-005)

---

- [ ] T031 Run full integration test suite for all components (`go test ./tests/integration/...`)
- [ ] T032 Verify startup performance meets SC-001 (< 5 seconds) with test application
- [ ] T033 Verify shutdown performance meets SC-002 (< 30 seconds) with test application
- [ ] T034 Verify all lifecycle events are logged correctly (grep test output for required log messages)
- [ ] T035 Verify exit codes: 0 for success, 1 for startup failure
- [ ] T036 Add example usage comments to cmd/main.go showing component registration pattern
- [ ] T037 Run `go build ./cmd/main.go` and verify binary builds without errors
- [ ] T038 Manual smoke test: Start application, verify startup logs, send SIGINT, verify shutdown logs and exit code

---

## Testing Summary

### Unit Tests (by component)
- Storage: Start/Stop with valid/invalid context
- Watcher: Start/Stop with valid/invalid context
- API Server: Start/Stop with valid/invalid context

### Integration Tests
- **startup_test.go**: Startup order, dependency validation, failure rollback
- **shutdown_test.go**: Shutdown order, timeout handling, slow components
- **signal_handling_test.go**: SIGINT reception, cleanup verification

### Performance Tests
- Startup timing (target: < 5 seconds)
- Shutdown timing (target: < 30 seconds)
- Grace period enforcement (target: ± 100ms)

### Acceptance Test Scenarios
All 11 acceptance scenarios from spec (3+4+2+2) covered by tests

---

## Task Checklist Progress

### Phase 1 Completion
```
[Startup only] T001, T002
```

### Phase 2 Completion
```
[Foundational] T003, T004, T005, T006
```

### Phase 3 Completion (MVP)
```
[US1: Startup] T007-T015
Deliverable: Can start and initialize components in correct order
```

### Phase 4 Completion
```
[US2: Shutdown] T016-T026
Deliverable: Can shutdown gracefully on SIGINT
```

### Phase 5 Completion
```
[US3: Timeout] T027-T028
Deliverable: Timeout protection prevents hangs
```

### Phase 6 Completion
```
[US4: Coordination] T029-T030
Deliverable: Multi-component coordination validated
```

### Phase 7 Completion
```
[Polish] T031-T038
Deliverable: All acceptance scenarios pass, performance validated
```

---

## Acceptance Criteria Per Task

### T007-T012: Component Implementations
**Done When**:
- [ ] Component implements required interface (Start, Stop, Name)
- [ ] Can be created with zero arguments
- [ ] Start returns error only on actual failure, nil on success
- [ ] Stop returns error only on actual failure, nil on success
- [ ] Unit tests pass (90%+ code coverage)

### T008, T010, T012: Component Unit Tests
**Done When**:
- [ ] All Start() scenarios tested (success, context cancellation, etc.)
- [ ] All Stop() scenarios tested (success, timeout, context cancellation)
- [ ] Name() returns correct component name
- [ ] Tests are independent and can run in any order

### T013, T015: Startup Integration Tests
**Done When**:
- [ ] Startup order verified (Storage first, then Watchers/API in parallel)
- [ ] Failure scenarios tested (component fails to start)
- [ ] Rollback tested (started components cleaned up on failure)
- [ ] Logs contain all required startup events with timestamps

### T016-T017: Manager Shutdown & Status Methods
**Done When**:
- [ ] Stop() shuts down components in reverse dependency order
- [ ] IsRunning() correctly reflects component state
- [ ] Methods handle components that fail to stop gracefully
- [ ] Unit tests pass for all code paths

### T018-T023: Component Stop Implementations
**Done When**:
- [ ] Stop() respects context deadline (doesn't block forever)
- [ ] Gracefully waits for in-flight operations
- [ ] Returns error if shutdown fails
- [ ] Unit tests verify graceful shutdown behavior

### T024-T026: Signal Handling & Shutdown Tests
**Done When**:
- [ ] Signal handler catches SIGINT and SIGTERM
- [ ] manager.Stop() is called on signal
- [ ] Process exits with status code 0
- [ ] Integration test sends actual SIGINT signal and verifies behavior
- [ ] All shutdown events logged in correct order

### T027-T028: Timeout Handling
**Done When**:
- [ ] Manager.Stop() uses context.WithTimeout (30 second default)
- [ ] Components exceeding timeout are forcefully terminated
- [ ] Timeout events logged as warnings
- [ ] Integration test simulates slow component and verifies timeout

### T029-T030: Multi-Component & Dependency Validation
**Done When**:
- [ ] Startup respects full dependency graph
- [ ] Shutdown respects full reverse dependency graph
- [ ] Circular dependencies detected and rejected
- [ ] Tests cover realistic scenarios (Storage → Watchers, Storage → API)

### T031-T038: Final Validation
**Done When**:
- [ ] All integration tests pass (`go test ./tests/integration/... -v`)
- [ ] Startup timing < 5 seconds
- [ ] Shutdown timing < 30 seconds
- [ ] Exit codes correct (0 for success, 1 for startup failure)
- [ ] Smoke test completes without errors

---

## Notes for Implementation

### Important Patterns

1. **Context Deadline Checking**
   ```go
   select {
   case <-ctx.Done():
       return ctx.Err()
   default:
   }
   ```

2. **Graceful Shutdown Pattern**
   ```go
   done := make(chan error, 1)
   go func() { /* do work */ done <- nil }()
   select {
   case <-done:
       return nil
   case <-ctx.Done():
       return ctx.Err()
   }
   ```

3. **Dependency Order Registration**
   ```go
   manager.Register(storage)
   manager.Register(watcher, storage)
   manager.Register(apiServer, storage)
   ```

### Performance Targets

- **Startup**: Each component should initialize in < 1.5 seconds (3 components × 1.5s = 4.5s total with sequential ops, faster with parallel)
- **Shutdown**: Each component should shutdown in < 10 seconds (3 × 10s = 30s total)
- **Grace Period**: 30 seconds per component (context deadline enforcement)

### Logging Requirements

Every task that implements a component method must include logging:
- `logger.Info("Starting [ComponentName]")`
- `logger.Info("[ComponentName] started successfully")`
- `logger.Warn("[ComponentName] shutdown timeout")`
- Include timestamps (logging package should do this automatically)

### Testing Requirements

- Unit tests for component Start/Stop (both success and failure paths)
- Integration tests for manager startup/shutdown sequence
- Signal handling test that sends actual OS signal
- Performance tests to validate timing targets

---

## References

- Feature Specification: [spec.md](spec.md)
- Implementation Plan: [plan.md](plan.md)
- Data Model: [data-model.md](data-model.md)
- Quickstart Guide: [quickstart.md](quickstart.md)
- API Contracts: [contracts/](contracts/)
