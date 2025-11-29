# Implementation Plan: End-to-End Test Suite for Kubernetes Event Monitor

**Branch**: `006-e2e-test-suite` | **Date**: 2025-11-26 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/006-e2e-test-suite/spec.md`

**Note**: This plan defines the implementation approach for the e2e test suite covering cluster setup, deployment, and multi-scenario testing of KEM.

## Summary

Create a comprehensive e2e test suite that validates the Kubernetes Event Monitor's core functionality by programmatically provisioning a Kind cluster, building and deploying KEM via Helm, then executing three test scenarios: default resource event capture with namespace filtering, event persistence through Pod restarts, and dynamic configuration reloading. The test suite must use eventual consistency patterns with retries due to async event propagation and configuration reload delays.

## Technical Context

**Language/Version**: Go 1.21+ (test suite written in Go using testify assertions)
**Primary Dependencies**:
- Kind for cluster creation
- Kubernetes client-go for kubectl operations
- Helm Go client for Helm deployments
- testify/assert for assertions with Eventually support
- Docker SDK for image building

**Storage**: N/A (KEM uses internal block-based storage, tests verify accessibility only)
**Testing**: Go's testing package with testify/assert/require
**Target Platform**: Linux/macOS (where Kind and Docker are available)
**Project Type**: Single test application
**Performance Goals**:
- Setup completes in under 5 minutes
- API queries respond within 5 seconds
- 95th percentile response time under 10 seconds
**Constraints**:
- Requires Docker daemon running
- Requires Kind and kubectl installed
- Tests must be idempotent and re-runnable
- Assumes ~2-5 second event propagation delay
- Assumes ~30 second config reload window
**Scale/Scope**: Single test suite with 3 test scenarios covering ~50-100 test cases total

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**No project constitution defined yet.** This project follows standard Go testing practices:
- ✅ Test-first approach: Tests define requirements, implementation follows
- ✅ Integration testing focus: Tests verify real cluster behavior, not mocks
- ✅ Clear contracts: API contracts defined via Kubernetes resource specs
- ✅ Documentation: Quickstart and test scenarios fully documented
- ⚠️ No specific gates identified; follows Go community standards

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
tests/e2e/
├── main_test.go              # Main test entry point
├── helpers/
│   ├── cluster.go            # Kind cluster lifecycle management
│   ├── deployment.go         # Helm chart deployment
│   ├── api.go                # API client with port-forward
│   └── assertions.go         # Eventually assertions helpers
├── scenarios/
│   ├── scenario_default_resources_test.go     # Default resource event capture (P1)
│   ├── scenario_pod_restart_test.go           # Pod restart durability (P1)
│   └── scenario_dynamic_config_test.go        # Dynamic config reload (P2)
└── fixtures/
    ├── watch-config-default.yaml              # Default watch configuration
    ├── watch-config-extended.yaml             # Extended watch configuration
    └── nginx-deployment.yaml                  # Test nginx deployment
```

**Structure Decision**: Single test application organized by scenario, with reusable helpers for cluster operations and API access. Each scenario is independently runnable and focuses on one user story. Helpers encapsulate infrastructure concerns (Kind, Helm, port-forwarding), keeping tests clean and focused on behavior verification.

## Complexity Tracking

No constitution violations. Standard Go testing practices applied.

---

## Phase 0: Outline & Research

### Research Tasks

**Task 0.1**: Best practices for Kind cluster management in Go tests
- How to programmatically create/destroy Kind clusters
- Port forwarding from tests
- Cleanup and error handling

**Task 0.2**: Kubernetes client-go patterns for test automation
- Creating Deployments and reading events
- Managing namespaces in tests
- Port-forward mechanics

**Task 0.3**: Helm Go client integration for test deployments
- Programmatic Helm chart deployment
- Values overrides from Go code
- Chart validation

**Task 0.4**: testify/assert.Eventually patterns for async operations
- Retry logic configuration
- Timeout handling
- Error message reporting

### Research Output: research.md

Generated documentation covering:
1. **Decision**: Use Kind + kubernetes client-go + Helm Go client + testify
2. **Rationale**: Industry-standard for Kubernetes testing, native Go integration, maintained libraries
3. **Alternatives Considered**:
   - Kubeadm: More complex, overkill for local testing
   - Minikube: Slower, less flexible than Kind
   - Manual kubectl: Would require bash scripts, harder to test
4. **Implementation Patterns**: Code examples for each major operation

---

## Phase 1: Design & Contracts

### Data Model: data-model.md

**Key Entities**:

1. **TestCluster**
   - Name: string (unique identifier)
   - KubeConfig: string (path to config file)
   - Context: string (Kind context name)
   - Status: enum {Creating, Ready, Deleting, Failed}
   - CreatedAt: time.Time
   - Cleanup func: deletion handler

2. **KubeAPIQuery**
   - Endpoint: string (/v1/search, /v1/metadata, etc.)
   - Filters: map[string]string (namespace, kind, etc.)
   - TimeRange: {Start, End time.Time}
   - RetryConfig: {MaxAttempts int, Interval time.Duration}

3. **AuditEventAssertion**
   - ExpectedCount: int
   - ExpectedNamespace: string (optional)
   - MinimumCreatedAt: time.Time
   - MaximumCreatedAt: time.Time
   - ResourceKind: string (Deployment, Pod, etc.)

### API Contracts: /contracts/

**File**: k8s-event-monitor-api.openapi.yaml
- GET /v1/search: Returns SearchResponse with events array
- GET /v1/metadata: Returns MetadataResponse with namespaces/kinds
- GET /v1/resources/{id}: Returns Resource with segments
- GET /v1/resources/{id}/segments: Returns segments with time-range filtering
- GET /v1/resources/{id}/events: Returns events with pagination

**Kubernetes Resources** (managed by tests):
- Deployment (nginx test): Standard K8s spec
- ConfigMap (watch-config): YAML configuration for dynamic reload testing
- Pod (various): Created/deleted during test execution

### Quickstart: quickstart.md

```bash
# Prerequisites
kind --version      # >= 0.17
helm --version      # >= 3.0
kubectl --version   # >= 1.24
docker ps           # daemon running

# Run the e2e test suite
cd tests/e2e
go test -v ./...

# Run specific scenario
go test -v -run TestScenarioDefaultResources ./scenarios

# View cluster state during test (in separate terminal)
kubectl --context kind-kem-e2e get all -A
kubectl --context kind-kem-e2e logs -f -l app=kem

# Manually inspect API
kubectl --context kind-kem-e2e port-forward svc/kem-api 8080:8080
curl http://localhost:8080/v1/search
```

### Agent Context Update

Run `.specify/scripts/bash/update-agent-context.sh claude` to register:
- Go 1.21+ environment
- Kind, Helm, kubernetes client-go, testify dependencies
- tests/e2e/ as primary source location
- Test-first development approach

---

## Phase 2: Task Breakdown (by scenario priority)

### User Story 1: Default Resource Event Capture (P1)

**T1.1**: Create cluster lifecycle helpers (Kind creation/deletion)
**T1.2**: Create API client wrapper with port-forward management
**T1.3**: Create nginx Deployment fixture
**T1.4**: Implement TestScenarioDefaultResources test structure
**T1.5**: Test event capture within time window
**T1.6**: Test namespace filter returns matching events
**T1.7**: Test unfiltered queries return all events
**T1.8**: Test filtered non-existent namespace returns empty

### User Story 2: Pod Restart Durability (P1)

**T2.1**: Enhance cluster helpers with Pod restart capability
**T2.2**: Implement TestScenarioPodRestart test structure
**T2.3**: Test events accessible before restart
**T2.4**: Restart Pod and verify reconnection
**T2.5**: Test same events still accessible after restart
**T2.6**: Test metadata integrity preserved

### User Story 3: Dynamic Configuration Reload (P2)

**T3.1**: Create Helm values fixtures for extended config
**T3.2**: Create extended watch configuration
**T3.3**: Implement TestScenarioDynamicConfig test structure
**T3.4**: Test initial default resource watching
**T3.5**: Update watch config in cluster
**T3.6**: Annotate Pod with please-remount trigger
**T3.7**: Wait 15 seconds for reload
**T3.8**: Create new resource type
**T3.9**: Verify events appear in API

### Infrastructure Tasks

**T4.1**: Setup/teardown helpers with proper error handling
**T4.2**: Eventually assertion wrapper functions
**T4.3**: Logging and debugging output
**T4.4**: Test fixtures organization
**T4.5**: Documentation and README
**T4.6**: CI/CD integration considerations

---

## Artifacts Generated

✅ **Phase 0**: research.md (to be created during implementation planning)
✅ **Phase 1**:
- data-model.md (entity definitions and state model)
- contracts/k8s-event-monitor-api.openapi.yaml (API contract)
- quickstart.md (getting started guide)
- Agent context updated with Go + test tools

✅ **Phase 2**: tasks.md (detailed task breakdown, generated by /speckit.tasks)

---

## Success Metrics

- All 3 test scenarios pass with 100% consistency
- Setup time < 5 minutes on standard hardware
- API response times < 5s p50, < 10s p95
- Tests are repeatable and idempotent
- Clear error messages on failure
- Full documentation and quickstart available
