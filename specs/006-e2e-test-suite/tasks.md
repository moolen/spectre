# Tasks: End-to-End Test Suite for Kubernetes Event Monitor

**Feature Branch**: `006-e2e-test-suite`
**Feature Specification**: [spec.md](spec.md)
**Implementation Plan**: [plan.md](plan.md)
**Generated**: 2025-11-26

**Task Scope**: ~210 tasks across 7 implementation phases
**Organized By**: User story priority (P1, P1, P2) + infrastructure/polish
**Test Coverage**: Integration tests for each scenario
**MVP Scope**: User Story 1 (default resource event capture)

---

## Dependency Map & Parallel Opportunities

```
Phase 1 (Setup)
    ↓
Phase 2 (Foundational Infrastructure)
    ├─→ Phase 3 (User Story 1: P1 - Default Resources) ← MVP Scope
    │
    ├─→ Phase 4 (User Story 2: P1 - Pod Restart)
    │       └─ Depends on Phase 2 only (can run parallel with US1)
    │
    └─→ Phase 5 (User Story 3: P2 - Dynamic Config)
            └─ Depends on Phase 2 only (can run parallel with US1 & US2)

Phase 6 (Integration & Cross-Story Testing)
    ├─ All scenarios complete → integration testing
    └─ Depends on Phase 3, 4, 5

Phase 7 (Polish & Documentation)
    └─ Final cleanup, performance tuning, docs
```

**Parallel Execution Strategy**:
- **Timeline Optimal**: US1 (4-5 days) → US2+US3 parallel (3-4 days) → Integration (2 days) → Polish (1 day)
- **Parallel Start**: All 3 user stories can start after Phase 2 (helpers complete)
- **Recommended MVP**: Focus on Phase 3 (US1) first for quick value delivery

---

## Phase 1: Project Setup & Infrastructure

### Setup Project Structure and Dependencies

- [ ] T001 Create tests/e2e/ directory structure with helpers, scenarios, fixtures subdirectories
- [ ] T002 Initialize Go module for test suite if needed (go mod init)
- [ ] T003 Add dependencies to go.mod: kind, kubernetes/client-go, Helm Go client, testify/assert
- [ ] T004 Create tests/e2e/main_test.go with package declaration and imports
- [ ] T005 Create tests/e2e/fixtures/ with watch-config-default.yaml, watch-config-extended.yaml, nginx-deployment.yaml
- [ ] T006 Create go.sum with resolved dependency versions
- [ ] T007 [P] Create tests/e2e/.gitignore for kubeconfig and cluster artifacts
- [ ] T008 Verify Go 1.21+ available and test compilation works (go test -compile)

---

## Phase 2: Foundational Helpers & Infrastructure (Blocking Prerequisites)

### Cluster Lifecycle Management

- [ ] T009 Implement tests/e2e/helpers/cluster.go with ClusterManager struct
- [ ] T010 Implement CreateCluster() function with Kind cluster provisioning (30-60s startup)
- [ ] T011 Implement cluster kubeconfig generation and context setup
- [ ] T012 Implement WaitForClusterReady() with node and API server health checks
- [ ] T013 Implement DeleteCluster() for cleanup with error handling
- [ ] T014 Implement GetKubeConfig() to return path to kubeconfig file
- [ ] T015 Add retry logic for cluster creation (max 3 attempts)
- [ ] T016 Add logging for cluster lifecycle events (creation, ready, deletion)

### Kubernetes API Automation (client-go Integration)

- [ ] T017 [P] Implement tests/e2e/helpers/deployment.go with DeploymentManager struct
- [ ] T018 [P] Create Kubernetes clientset from kubeconfig in DeploymentManager
- [ ] T019 [P] Implement CreateNamespace() for test isolation
- [ ] T020 [P] Implement CreateDeployment() to deploy nginx test image
- [ ] T021 [P] Implement DeleteDeployment() with proper cleanup
- [ ] T022 [P] Implement ListDeployments() with namespace filtering
- [ ] T023 [P] Implement GetDeployment() to retrieve single deployment details
- [ ] T024 [P] Add retry logic for deployment creation (eventual consistency)

### Helm Integration

- [ ] T025 Implement tests/e2e/helpers/deployment.go HelmManager functions (or separate file)
- [ ] T026 Initialize Helm action configuration with kubeconfig
- [ ] T027 Implement DeployKEMChart() function with Helm chart installation
- [ ] T028 Implement custom values override (image tag, replica count for tests)
- [ ] T029 Implement WaitForKEMReady() with Pod readiness checks
- [ ] T030 Implement UninstallKEMChart() for cleanup
- [ ] T031 Add retry logic for Helm operations (timeout handling)
- [ ] T032 [P] Add validation that chart has required endpoints (/v1/search, etc.)

### Port-Forward & API Client

- [ ] T033 Implement tests/e2e/helpers/api.go with APIClient struct
- [ ] T034 Implement port-forward to KEM service using kubectl
- [ ] T035 Implement HTTP client with configurable timeout (default 30s)
- [ ] T036 Implement Query() method for generic API requests
- [ ] T037 Implement GetSearch() for /v1/search endpoint
- [ ] T038 Implement GetMetadata() for /v1/metadata endpoint
- [ ] T039 Implement GetResource() for /v1/resources/{id} endpoint
- [ ] T040 Implement GetSegments() for /v1/resources/{id}/segments endpoint
- [ ] T041 Implement GetEvents() for /v1/resources/{id}/events endpoint
- [ ] T042 [P] Add JSON unmarshaling for API responses
- [ ] T043 [P] Add error handling for API failures (timeout, connection refused, etc.)
- [ ] T044 Implement port-forward cleanup via context cancellation

### Eventually Assertion Helpers

- [ ] T045 Implement tests/e2e/helpers/assertions.go with Eventually wrapper functions
- [ ] T046 Implement EventuallyEventCount() to poll for expected event count
- [ ] T047 Implement EventuallyEventInNamespace() to verify namespace filtering
- [ ] T048 Implement EventuallyEventNotInNamespace() to verify cross-namespace isolation
- [ ] T049 Implement EventuallyResourceReady() to wait for resource status
- [ ] T050 [P] Add configurable timeout and retry interval (default: 10s timeout, 500ms interval)
- [ ] T051 [P] Add detailed assertion failure messages with context (query params, expected vs actual)
- [ ] T052 Implement EventuallyConfigReloaded() with 30s timeout for dynamic config testing

### Test Fixtures & Configuration

- [ ] T053 Create tests/e2e/fixtures/nginx-deployment.yaml with replicas: 1
- [ ] T054 Create tests/e2e/fixtures/watch-config-default.yaml (Deployments, Pods)
- [ ] T055 Create tests/e2e/fixtures/watch-config-extended.yaml (adds StatefulSet)
- [ ] T056 [P] Create tests/e2e/fixtures/helm-values-test.yaml (test-optimized settings)
- [ ] T057 Implement LoadFixture() helper function in assertions.go
- [ ] T058 Implement ApplyYAML() to apply Kubernetes manifests from fixtures
- [ ] T059 Implement UpdateConfigMap() for dynamic configuration updates

---

## Phase 3: User Story 1 - Default Resource Event Capture (P1)

**Goal**: Verify that KEM captures events for default watched resources (Deployments, Pods) and supports namespace filtering.

**Independent Test Criteria**:
- Deployment created → events appear in API within 5s
- Namespace filter works correctly
- Unfiltered queries return all events
- Cross-namespace filters work correctly
- Can be tested independently without US2 or US3

### Test Scenario Implementation

- [ ] T060 [US1] Create tests/e2e/scenarios/scenario_default_resources_test.go
- [ ] T061 [US1] Implement TestScenarioDefaultResources() test function
- [ ] T062 [US1] Initialize test cluster and deploy KEM with default config
- [ ] T063 [US1] Create test namespace for resource isolation
- [ ] T064 [US1] Deploy nginx test Deployment from fixture

### Event Capture Validation

- [ ] T065 [US1] Query API for Deployment events within time window using EventuallyEventCount()
- [ ] T066 [US1] Verify event verb is "create" (or "get" if pre-existing)
- [ ] T067 [US1] Verify event timestamp is within expected range
- [ ] T068 [US1] Verify event has correct resource metadata (kind, name, namespace)
- [ ] T069 [US1] Test minimum 1 event captured (EventuallyEventCount(>=1))

### Namespace Filtering

- [ ] T070 [US1] Query API with namespace filter matching test namespace
- [ ] T071 [US1] Verify filtered results include events from test namespace using EventuallyEventInNamespace()
- [ ] T072 [US1] Verify filtered count matches or exceeds expected count
- [ ] T073 [US1] Test multiple events from same resource all appear

### Unfiltered Query

- [ ] T074 [US1] Query API without namespace filter (all namespaces)
- [ ] T075 [US1] Verify results include events from default, kube-system, and test namespaces
- [ ] T076 [US1] Verify count includes events from all sources

### Cross-Namespace Filtering

- [ ] T077 [US1] Create second test namespace (test-alternate)
- [ ] T078 [US1] Create Deployment in second namespace
- [ ] T079 [US1] Query API with filter for first namespace only
- [ ] T080 [US1] Verify results exclude events from second namespace using EventuallyEventNotInNamespace()
- [ ] T081 [US1] Verify same events are returned when filtering for second namespace

### Metadata Consistency

- [ ] T082 [US1] Query /v1/metadata endpoint
- [ ] T083 [US1] Verify returned namespaces includes both test namespaces
- [ ] T084 [US1] Verify resource kinds includes "Deployment"
- [ ] T085 [US1] Verify resource counts are > 0 for Deployment

### Cleanup & Teardown

- [ ] T086 [US1] Delete test namespaces
- [ ] T087 [US1] Delete KEM cluster
- [ ] T088 [US1] Verify no orphaned resources using defer cleanup handlers

---

## Phase 4: User Story 2 - Pod Restart Durability (P1)

**Goal**: Verify that captured events remain accessible after Pod restarts, ensuring data persistence.

**Independent Test Criteria**:
- Events captured before restart are accessible after restart
- Event metadata integrity preserved
- Pod regains connectivity to storage backend
- Can be tested independently without US1 or US3

### Test Scenario Implementation

- [ ] T089 [US2] Create tests/e2e/scenarios/scenario_pod_restart_test.go
- [ ] T090 [US2] Implement TestScenarioPodRestart() test function
- [ ] T091 [US2] Initialize test cluster and deploy KEM with default config
- [ ] T092 [US2] Create test namespace for resource isolation
- [ ] T093 [US2] Deploy nginx test Deployment

### Pre-Restart Event Capture

- [ ] T094 [US2] Wait for events to appear in API (EventuallyEventCount(>=1))
- [ ] T095 [US2] Query /v1/search to get baseline event count
- [ ] T096 [US2] Record event IDs and timestamps
- [ ] T097 [US2] Query /v1/metadata to verify resource is tracked

### Pod Restart Operation

- [ ] T098 [US2] Get KEM Pod using client-go
- [ ] T099 [US2] Delete KEM Pod (forces restart via Kubernetes)
- [ ] T100 [US2] Wait for new Pod to reach Running state (WaitForKEMReady())
- [ ] T101 [US2] Verify Pod has new container ID but same service
- [ ] T102 [US2] Verify new Pod regains connectivity to storage backend

### Post-Restart Event Verification

- [ ] T103 [US2] Query API for previously captured events (same event IDs)
- [ ] T104 [US2] Verify event count includes all pre-restart events
- [ ] T105 [US2] Verify event timestamps are unchanged
- [ ] T106 [US2] Verify event metadata (verb, user, message) is unchanged
- [ ] T107 [US2] Verify no data corruption using EventuallyEventCount()

### Additional Resource Modification

- [ ] T108 [US2] Modify Deployment (e.g., scale replicas)
- [ ] T109 [US2] Verify new events appear in API
- [ ] T110 [US2] Verify all previous events still accessible
- [ ] T111 [US2] Verify event ordering is maintained (chronological)

### Cleanup & Teardown

- [ ] T112 [US2] Delete test namespace
- [ ] T113 [US2] Delete KEM cluster
- [ ] T114 [US2] Verify no orphaned resources

---

## Phase 5: User Story 3 - Dynamic Configuration Reload (P2)

**Goal**: Verify that watch configuration can be extended and reloaded without cluster restart.

**Independent Test Criteria**:
- Configuration persists across updates
- Pod annotation triggers reload
- New resource type is watched after reload
- Can be tested independently without US1 or US2

### Test Scenario Implementation

- [ ] T115 [US3] Create tests/e2e/scenarios/scenario_dynamic_config_test.go
- [ ] T116 [US3] Implement TestScenarioDynamicConfig() test function
- [ ] T117 [US3] Initialize test cluster and deploy KEM with default config
- [ ] T118 [US3] Create test namespace for resource isolation

### Initial State Verification

- [ ] T119 [US3] Verify default watch config watches Deployments and Pods
- [ ] T120 [US3] Create a StatefulSet (not in default config) and verify NO events appear
- [ ] T121 [US3] Wait 5 seconds to confirm events don't appear (negative test)
- [ ] T122 [US3] Delete test StatefulSet

### Configuration Update

- [ ] T123 [US3] Get current watch configuration ConfigMap from cluster
- [ ] T124 [US3] Update ConfigMap to add StatefulSet to watched resources
- [ ] T125 [US3] Apply updated ConfigMap using UpdateConfigMap() helper
- [ ] T126 [US3] Verify ConfigMap update persists

### Configuration Reload Trigger

- [ ] T127 [US3] Get KEM Pod reference
- [ ] T128 [US3] Annotate Pod with please-remount=${current_timestamp}
- [ ] T129 [US3] Verify annotation is applied using kubectl
- [ ] T130 [US3] Wait 15 seconds for configuration propagation (using EventuallyConfigReloaded())
- [ ] T131 [US3] Verify KEM Pod is still running (no restart, just reload)

### New Resource Type Validation

- [ ] T132 [US3] Create a StatefulSet in test namespace
- [ ] T133 [US3] Query API for StatefulSet events using EventuallyEventCount()
- [ ] T134 [US3] Verify events appear within 5 seconds
- [ ] T135 [US3] Verify event kind is "StatefulSet"
- [ ] T136 [US3] Verify event has correct metadata

### Multi-Resource Verification

- [ ] T137 [US3] Create Deployment (in original config)
- [ ] T138 [US3] Verify Deployment events appear (original functionality preserved)
- [ ] T139 [US3] Verify both Deployment and StatefulSet events coexist
- [ ] T140 [US3] Query /v1/metadata to verify both kinds are present

### Configuration Persistence

- [ ] T141 [US3] Restart KEM Pod (different from configuration reload)
- [ ] T142 [US3] Verify configuration persists (StatefulSet still watched)
- [ ] T143 [US3] Create another StatefulSet and verify events appear
- [ ] T144 [US3] Verify no configuration reset happened

### Cleanup & Teardown

- [ ] T145 [US3] Delete test namespace
- [ ] T146 [US3] Delete KEM cluster
- [ ] T147 [US3] Verify no orphaned resources

---

## Phase 6: Integration & Cross-Story Testing

### Multi-Scenario Sequence Testing

- [ ] T148 [P] Create tests/e2e/scenarios/scenario_integration_test.go
- [ ] T149 [P] Implement TestAllScenariosSequential() to run all 3 in order
- [ ] T150 [P] Verify US1 → US2 → US3 complete without interference

### Cross-Scenario State Verification

- [ ] T151 [P] Run US1 and US2 in parallel (same cluster instance)
- [ ] T152 [P] Verify event counts from US1 don't interfere with US2
- [ ] T153 [P] Run US3 after US1+US2 complete
- [ ] T154 [P] Verify dynamic config doesn't affect previous events

### Edge Case Testing

- [ ] T155 [P] Test API query with empty time window (no events in range)
- [ ] T156 [P] Test API query for non-existent namespace (empty results)
- [ ] T157 [P] Test configuration reload during active resource modifications
- [ ] T158 [P] Test rapid successive configuration changes (no errors)
- [ ] T159 [P] Test Pod restart during event propagation window
- [ ] T160 [P] Test API timeout handling (deliberately delayed responses)

### Performance Validation

- [ ] T161 [P] Measure API response times across all scenarios (p50, p95, p99)
- [ ] T162 [P] Verify setup time < 5 minutes
- [ ] T163 [P] Verify no memory leaks in cluster or test process
- [ ] T164 [P] Verify no orphaned goroutines

### Logging & Observability Testing

- [ ] T165 [P] Verify all cluster operations are logged
- [ ] T166 [P] Verify all API calls include request/response logging
- [ ] T167 [P] Verify test failures include sufficient debugging context
- [ ] T168 [P] Verify event audit trail is complete

---

## Phase 7: Polish, Documentation & Final Validation

### Test Reliability & Robustness

- [ ] T169 Run all tests 5 times sequentially to verify idempotency
- [ ] T170 Verify no flaky tests (all 100% pass rate across runs)
- [ ] T171 Add recovery logic for partial failures (e.g., cleanup on timeout)
- [ ] T172 Verify cluster deletion always completes (even on errors)
- [ ] T173 Add explicit timeout to all Eventually assertions (no hanging tests)

### Documentation & Quickstart

- [ ] T174 [P] Review and update quickstart.md with actual test commands
- [ ] T175 [P] Add troubleshooting guide for common test failures
- [ ] T176 [P] Document expected output format for each test scenario
- [ ] T177 [P] Create CI/CD integration guide (GitHub Actions example)
- [ ] T178 [P] Add performance baseline documentation

### Code Quality & Organization

- [ ] T179 [P] Review all helper functions for clarity and documentation
- [ ] T180 [P] Verify consistent error handling across all helpers
- [ ] T181 [P] Add package-level comments to all test files
- [ ] T182 [P] Verify test names follow Go conventions (TestXxx)
- [ ] T183 [P] Run go fmt and go vet on all test code

### Test Coverage & Completeness

- [ ] T184 Verify all FR requirements from spec.md are covered by tests
- [ ] T185 Verify all acceptance scenarios from user stories are tested
- [ ] T186 Verify edge cases from spec.md are addressed
- [ ] T187 Document any acceptance scenarios not yet implemented

### Final Validation & Sign-Off

- [ ] T188 Run complete test suite start to finish
- [ ] T189 Verify all 3 user story tests pass independently
- [ ] T190 Verify all 3 user stories pass sequentially
- [ ] T191 [P] Verify parallel execution of US1, US2, US3 works
- [ ] T192 Verify cleanup removes all artifacts (no orphaned clusters)
- [ ] T193 Verify no DNS pollution or service port conflicts

### Performance & Scalability Validation

- [ ] T194 Measure and document full test suite execution time
- [ ] T195 Verify API response times meet SC-003 (< 5s p50, < 10s p95)
- [ ] T196 Verify setup time meets SC-001 (< 5 minutes)
- [ ] T197 Verify configuration reload meets SC-004 (< 30 seconds)
- [ ] T198 Document system resource requirements (CPU, memory, disk)

### CI/CD Integration

- [ ] T199 [P] Create GitHub Actions workflow (if applicable)
- [ ] T200 [P] Verify test output is CI-friendly (structured, no interactive prompts)
- [ ] T201 [P] Add test result reporting (pass/fail, duration, error messages)
- [ ] T202 [P] Test on Linux and macOS (both common for Go development)

### Maintenance & Extensibility

- [ ] T203 Document how to add new test scenarios
- [ ] T204 Document how to extend watch configuration testing
- [ ] T205 Document how to add new API endpoints to testing
- [ ] T206 Create template for new scenario test files

### Release Readiness

- [ ] T207 Review all test output for sensitive information (no credentials logged)
- [ ] T208 Verify test suite can be run by developers without special permissions
- [ ] T209 Create test suite README with overview and usage
- [ ] T210 Tag release and update CHANGELOG

---

## Acceptance Criteria Summary

### Per User Story

**User Story 1 (Default Resource Capture)**:
- ✓ Deployment events captured within time window
- ✓ Namespace filter returns only matching events
- ✓ Unfiltered query returns all events
- ✓ Cross-namespace filtering works correctly
- ✓ Metadata endpoint includes captured resources

**User Story 2 (Pod Restart Durability)**:
- ✓ Events accessible before and after Pod restart
- ✓ No data loss on restart
- ✓ Event metadata preserved
- ✓ Pod reconnects to storage backend successfully

**User Story 3 (Dynamic Configuration Reload)**:
- ✓ Configuration persists after update
- ✓ Annotation trigger reloads configuration
- ✓ New resource type is watched after reload
- ✓ Original watched resources still work after reload

### Overall Success Metrics

- ✓ All tests pass 100% consistently
- ✓ Setup < 5 minutes
- ✓ API response < 5s p50, < 10s p95
- ✓ Config reload < 30 seconds
- ✓ No orphaned resources
- ✓ Comprehensive documentation provided

---

## Implementation Recommendations

### MVP Scope (Recommended Start)

Focus first on **User Story 1** (Phase 3, T060-T088):
- Core functionality test
- 4-5 days to complete
- Provides quick value and learning
- Others can start in parallel after Phase 2

### Recommended Execution Order

1. **Days 1-2**: Phase 1 & 2 (setup, helpers) - **CRITICAL PATH**
2. **Days 3-5**: Phase 3 (US1) - **MVP**
3. **Days 4-5 (parallel)**: Phase 4 (US2) + Phase 5 (US3)
4. **Days 6-7**: Phase 6 (integration) + Phase 7 (polish)

### Testing Approach

- **TDD**: Tests already defined in user stories
- **Integration Focus**: Real Kind cluster, not mocks
- **Async-Aware**: Built-in retry/eventually patterns
- **Observability**: Comprehensive logging for debugging

### Parallel Opportunities

- **Helpers**: All Phase 2 tasks can be parallelized after T001-T008
- **Scenarios**: US1, US2, US3 can be implemented in parallel after Phase 2
- **Testing**: Each scenario has independent test validation
- **Documentation**: Can be written in parallel with implementation

---

## Task Execution Guidance

**For Each Task**:
1. Read the task description and acceptance criteria
2. Check dependencies (tasks blocking this one)
3. Implement with accompanying tests
4. Verify it works in isolation
5. Update progress and move to next task

**Test Verification**:
```bash
# Run specific user story tests
go test -v -run TestScenarioDefaultResources ./scenarios

# Run all tests
go test -v ./...

# Run with logging
go test -v -run TestScenario -count=1 ./scenarios
```

**Common Patterns**:
- All tests use `defer cleanup()` for resource cleanup
- All async operations use `assert.Eventually()` or `EventuallyXxx()` helpers
- All fixtures are in `tests/e2e/fixtures/`
- All helpers are imported from `tests/e2e/helpers`

---

## Success: This Tasks List is Complete When

- [ ] All 210 tasks are implemented
- [ ] All 3 user story tests pass independently
- [ ] All user stories pass sequentially
- [ ] Parallel execution works
- [ ] Full documentation provided
- [ ] Performance benchmarks validated
- [ ] No orphaned resources on cleanup
- [ ] Code review approved
