# MCP Server Test Strategy Plan

## Overview
This plan outlines a comprehensive testing strategy for the Spectre MCP server, which provides Kubernetes audit log analysis tools for post-mortem analysis and live incident response.

## Current Implementation Context

The MCP server exposes 4 primary tools:
- `cluster_health` - Overall cluster health with resource status breakdown
- `resource_changes` - Resource change tracking with impact scoring
- `investigate` - Deep-dive RCA with detailed evidence
- `resource_explorer` - Resource discovery and filtering

Health detection uses multi-layered status inference:
- Resource-specific logic (Pods, Deployments, Nodes, etc.)
- Condition-based inference (Ready, Healthy, Degraded, etc.)
- Event-based fallback
- Impact scoring (0-1.0) based on error/warning events and status transitions

## Test Strategy Objectives

1. **Verify defect detection** - Ensure all critical Kubernetes issues are correctly identified
2. **Validate error/warning classification** - Confirm proper status assignment (Ready/Warning/Error)
3. **Test impact scoring accuracy** - Validate that high-impact changes score appropriately
4. **Ensure protocol compliance** - Verify JSON-RPC 2.0 and MCP protocol adherence
5. **Test edge cases** - Handle malformed input, missing data, time ranges
6. **Performance validation** - Ensure acceptable response times for large datasets

## Test Approach

### 1. Unit Testing Strategy

#### 1.1 Status Inference Tests (`internal/storage/status_inference_test.go`)

**Test Coverage:**
- Individual resource kind status detection
  - Deployment: replica mismatches, Available condition, Progressing failures
  - Pod: all phases (Running/Pending/Failed/Unknown), Ready condition
  - Node: Ready condition, pressure conditions (Memory/Disk/PID/Network)
  - StatefulSet: replica readiness
  - DaemonSet: scheduling issues, unavailable pods
  - Job: completion/failure conditions
  - PVC: phase transitions (Bound/Pending/Lost)

- Condition-based inference
  - Ready=True → Ready status
  - Ready=False + error keywords → Error status
  - Ready=False (no errors) → Warning status
  - Degraded/Failing/Failed conditions → Error status
  - Stalled condition → Warning status

- Event-based fallback
  - CREATE/UPDATE events → Ready
  - DELETE events → Terminating
  - Unknown events → Unknown

**Test Structure:**
```go
func TestPodStatusInference(t *testing.T) {
    tests := []struct {
        name           string
        pod            *corev1.Pod
        expectedStatus storage.ResourceStatus
        expectedMsg    string
    }{
        {
            name: "running pod with ready condition",
            pod:  createPod(corev1.PodRunning, conditionReady()),
            expectedStatus: storage.StatusReady,
        },
        {
            name: "crashloopbackoff pod",
            pod:  createPod(corev1.PodRunning, conditionNotReady("CrashLoopBackOff")),
            expectedStatus: storage.StatusError,
            expectedMsg:    "CrashLoopBackOff",
        },
        // ... more cases
    }
}
```

**Key Test Cases:**
- Pod in CrashLoopBackOff (Ready=False, reason contains "crash")
- Pod in ImagePullBackOff (Ready=False, reason contains "ImagePullBackOff")
- Pod with failed readiness probe (Ready=False with probe failure message)
- Pod pending due to node selector mismatch
- Pod with high restart count
- Pod hitting resource limits (OOMKilled)
- Deployment with unavailable replicas
- Node with MemoryPressure=True
- DaemonSet with misscheduled pods

#### 1.2 Impact Scoring Tests (`internal/mcp/tools/resource_changes_test.go`)

**Test Coverage:**
- Score calculation accuracy
  - Error events → +0.3
  - Warning events → +0.15
  - Status transition to Error → +0.3
  - Status transition to Warning → +0.15
  - High event count (>10) → +0.1
  - Score capping at 1.0

- Change categorization
  - Status transitions vs. event-only changes
  - Multiple transitions on same resource
  - Bulk changes affecting many resources

**Test Structure:**
```go
func TestImpactScoreCalculation(t *testing.T) {
    tests := []struct {
        name          string
        errorCount    int
        warningCount  int
        transitions   []storage.StatusTransition
        eventCount    int
        expectedScore float64
    }{
        {
            name:          "single error event",
            errorCount:    1,
            expectedScore: 0.3,
        },
        {
            name:          "error + warning + transition to error",
            errorCount:    1,
            warningCount:  1,
            transitions:   []storage.StatusTransition{{ToStatus: "Error"}},
            expectedScore: 0.75, // 0.3 + 0.15 + 0.3
        },
        {
            name:          "score capped at 1.0",
            errorCount:    5,
            transitions:   []storage.StatusTransition{
                {ToStatus: "Error"},
                {ToStatus: "Error"},
                {ToStatus: "Error"},
            },
            expectedScore: 1.0,
        },
        // ... more cases
    }
}
```

#### 1.3 Cluster Health Aggregation Tests (`internal/mcp/tools/cluster_health_test.go`)

**Test Coverage:**
- Status rollup (Critical/Degraded/Healthy)
- Top issues sorting (by error duration)
- Resource count aggregation
- Error rate calculation per kind

**Test Cases:**
- Mixed health cluster (some errors, some warnings, some healthy)
- All healthy cluster
- Critical cluster (all resources in error)
- Empty cluster
- Single resource kind vs. multiple kinds

#### 1.4 MCP Protocol Tests (`internal/mcp/handler_test.go`)

**Test Coverage:**
- JSON-RPC 2.0 compliance
  - Method not found errors
  - Invalid params errors
  - Parse errors
  - Internal errors

- MCP method handling
  - `initialize` - session setup
  - `tools/list` - tool enumeration
  - `tools/call` - parameter validation, execution
  - `logging/setLevel` - log level control

- Tool parameter validation
  - Required parameters
  - Time range validation (start < end)
  - Limit/threshold bounds checking
  - Unknown parameters handling

### 2. Integration Testing Strategy

#### 2.1 End-to-End MCP Flow Tests

**Approach:** Spin up test Spectre API server with pre-populated timeline data, exercise full MCP flow.

**Test Structure:**
```go
func TestMCPServerE2E(t *testing.T) {
    // Setup: Start test Spectre API with mock data
    apiServer := startTestSpectreAPI(t, testTimeline)
    defer apiServer.Close()

    // Setup: Start MCP server
    mcpServer := mcp.NewMCPServer(apiServer.URL)

    // Test: Execute cluster_health
    result := mcpServer.CallTool("cluster_health", params)

    // Verify: Correct status detection
    assert.Equal(t, "Critical", result.OverallStatus)
    assert.Greater(t, result.ErrorCount, 0)
}
```

**Test Scenarios:**
1. **Post-mortem analysis**
   - Query historical time range with known incidents
   - Verify resource_changes identifies the problematic resources
   - Verify investigate provides accurate timeline
   - Verify cluster_health shows degraded state during incident window

2. **Live incident response**
   - Query recent time range (last hour)
   - Verify real-time detection of ongoing issues
   - Verify investigation prompts are incident-focused

3. **Bulk deletion scenario**
   - Timeline with namespace deletion
   - Verify all child resources detected as Terminating
   - Verify high impact score due to volume

4. **Gradual degradation**
   - Timeline showing slow increase in errors (pod restarts increasing)
   - Verify impact scoring reflects escalation
   - Verify status transitions captured

#### 2.2 MCP Client Compatibility Tests

**Approach:** Test against actual MCP clients (Claude Desktop, other MCP clients).

**Manual Test Cases:**
1. Add Spectre MCP server to Claude Desktop config
2. Verify tool discovery (`tools/list`)
3. Execute each tool with various parameters
4. Verify response formatting is LLM-friendly
5. Test investigation prompt quality

**Automated Client Tests:**
```bash
# Using MCP inspector or test client
mcp-inspector --transport stdio --command "./spectre mcp --transport stdio"
```

### 3. Scenario-Based Testing Strategy

#### 3.1 Test Data Generation

**Approach:** Create synthetic Kubernetes timelines representing real-world scenarios.

**Implementation:**
- Use `internal/storage` to build in-memory timelines
- Populate with ResourceSnapshot and Event data
- Define scenarios as Go test fixtures

**Scenario Library:**

1. **CrashLoopBackOff Scenario**
   - Pod starts in Running state
   - Transitions to Error with "CrashLoopBackOff" message
   - Multiple events with increasing backoff
   - RestartCount incrementing

2. **ImagePullBackOff Scenario**
   - Pod stuck in Pending
   - Events: "Failed to pull image", "Back-off pulling image"
   - Condition Ready=False, reason="ImagePullBackOff"

3. **OOMKill Scenario**
   - Pod running, then transitions to Error
   - Events: "OOMKilled" with container name
   - RestartCount > 0
   - Status shows resource limit exceeded

4. **Readiness Probe Failure After Upgrade**
   - Deployment updates (image change detected)
   - New pods enter Running but Ready=False
   - Events: "Readiness probe failed"
   - Deployment shows unavailable replicas

5. **Node Pressure Scenario**
   - Node initially Ready
   - Transitions to Warning with MemoryPressure=True
   - Events: "NodeMemoryPressure"
   - Pods on that node start evicting

6. **Unschedulable Pod Scenario**
   - Pod remains in Pending
   - Events: "FailedScheduling: 0/5 nodes available, 3 Insufficient cpu, 2 node selector mismatch"
   - Condition Ready=False

7. **Service Endpoint Absence**
   - Service exists (Ready)
   - Endpoints object shows 0 ready addresses
   - Related pods in Error/Warning state

8. **Namespace Deletion Cascade**
   - Namespace gets deletionTimestamp
   - All resources transition to Terminating
   - High volume of DELETE events

9. **DaemonSet Scheduling Issues**
   - DaemonSet shows desiredNumberScheduled > currentNumberScheduled
   - Events: "FailedScheduling" on specific nodes (taints, selectors)
   - Status: numberUnavailable > 0

10. **PVC Stuck in Pending**
    - PVC phase = Pending
    - Events: "FailedBinding: no persistent volumes available"
    - Pod waiting on volume mount remains Pending

#### 3.2 Defect Detection Validation

For each scenario above, create tests that verify:

```go
func TestDetectCrashLoopBackOff(t *testing.T) {
    // Given: Timeline with CrashLoopBackOff scenario
    timeline := createCrashLoopBackOffScenario()

    // When: Run cluster_health
    health := tools.ClusterHealth(timeline, params)

    // Then: Verify detection
    assert.Equal(t, "Critical", health.OverallStatus)
    assert.Greater(t, health.Breakdown["Error"], 0)

    // When: Run resource_changes
    changes := tools.ResourceChanges(timeline, params)

    // Then: Verify impact scoring
    podChange := findChange(changes, "Pod", "crashloop-pod")
    assert.Greater(t, podChange.ImpactScore, 0.5)
    assert.Contains(t, podChange.StatusTransitions, "Error")

    // When: Run investigate
    evidence := tools.Investigate(timeline, "Pod", "crashloop-pod", params)

    // Then: Verify evidence collection
    assert.Contains(t, evidence.StatusTimeline, StatusSegment{Status: "Error"})
    assert.Contains(t, evidence.Events, Event{Reason: "BackOff"})
}
```

**Test Matrix:**
| Scenario | cluster_health Status | Impact Score Range | Key Evidence |
|----------|----------------------|-------------------|--------------|
| CrashLoopBackOff | Critical | 0.6-1.0 | Error transition, BackOff events |
| ImagePullBackOff | Critical | 0.5-0.8 | Error status, ImagePull events |
| OOMKill | Critical | 0.7-1.0 | OOMKilled events, restarts |
| Readiness Probe Fail | Degraded/Critical | 0.4-0.7 | Probe failed events |
| Node Pressure | Degraded | 0.3-0.6 | NodePressure condition |
| Unschedulable Pod | Degraded/Critical | 0.4-0.7 | FailedScheduling events |
| Service No Endpoints | Warning | 0.2-0.5 | Endpoint count = 0 |
| Namespace Deletion | Critical | 0.8-1.0 | Bulk DELETE events |
| DaemonSet Issues | Degraded | 0.3-0.6 | Unavailable count |
| PVC Pending | Warning/Critical | 0.3-0.6 | FailedBinding events |

### 4. Property-Based Testing

**Approach:** Use property-based testing (e.g., `go-fuzz`, `gopter`) to generate random timelines and verify invariants.

**Properties to Test:**
1. **Impact score bounds**: ∀ resource changes, 0 ≤ impact_score ≤ 1.0
2. **Status monotonicity**: If resource has Error events, status cannot be Ready
3. **Transition consistency**: Status transitions must have valid from/to states
4. **Time ordering**: All timestamps must be within query range and ordered
5. **Count consistency**: Breakdown counts must sum to total resource count

**Example:**
```go
func TestImpactScoreAlwaysBounded(t *testing.T) {
    properties := gopter.NewProperties(nil)

    properties.Property("impact score is always between 0 and 1", prop.ForAll(
        func(errorCount, warningCount, eventCount int) bool {
            score := calculateImpactScore(errorCount, warningCount, eventCount)
            return score >= 0.0 && score <= 1.0
        },
        gen.IntRange(0, 100),
        gen.IntRange(0, 100),
        gen.IntRange(0, 1000),
    ))

    properties.TestingRun(t)
}
```

### 5. Performance Testing

#### 5.1 Load Testing

**Test Cases:**
- Large cluster (10,000+ resources)
- Long time range (7 days)
- High churn (1000+ events/minute)
- Pagination stress (max_resources limits)

**Metrics:**
- Response time (p50, p95, p99)
- Memory usage
- CPU utilization
- Timeout handling

**Acceptance Criteria:**
- cluster_health: < 2s for 10k resources
- resource_changes: < 3s for 1k changed resources
- investigate: < 5s for single resource with 1k events
- Memory: < 512MB for typical workload

#### 5.2 Benchmark Tests

```go
func BenchmarkClusterHealth(b *testing.B) {
    timeline := createLargeTimeline(10000)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        tools.ClusterHealth(timeline, params)
    }
}
```

### 6. Error Handling & Edge Case Testing

**Test Cases:**
1. **Invalid time ranges**
   - start > end
   - Future timestamps
   - Negative timestamps

2. **Missing/malformed data**
   - Resource snapshots missing status field
   - Events without reason/message
   - Null values in JSON

3. **Boundary conditions**
   - Empty timeline (no resources)
   - Single resource
   - Exact time range match
   - Zero-duration status segments

4. **Parameter validation**
   - Negative limits
   - Limits exceeding max
   - Invalid impact_threshold (< 0 or > 1)
   - Unknown resource kinds

5. **Concurrency**
   - Multiple simultaneous tool calls
   - Concurrent client sessions
   - Race condition detection

### 7. Continuous Integration Strategy

**CI Pipeline:**
```yaml
name: MCP Server Tests

on: [push, pull_request]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - run: go test -v -race -coverprofile=coverage.txt ./internal/mcp/...
      - run: go test -v -race ./internal/storage/...

  integration-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - run: make test-mcp-integration

  scenario-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - run: make test-scenarios

  benchmarks:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - run: go test -bench=. -benchmem ./internal/mcp/tools/
```

**Test Organization:**
```
tests/
├── unit/
│   ├── status_inference_test.go
│   ├── impact_scoring_test.go
│   └── protocol_test.go
├── integration/
│   ├── e2e_test.go
│   └── client_test.go
├── scenarios/
│   ├── fixtures.go
│   ├── crashloop_test.go
│   ├── oomkill_test.go
│   └── ... (one file per scenario)
└── performance/
    └── benchmarks_test.go
```

## Test Execution Plan

### Phase 1: Unit Test Foundation (Week 1)
- Implement status inference tests for all resource kinds
- Implement impact scoring calculation tests
- Implement MCP protocol handler tests
- Target: 80% code coverage

### Phase 2: Scenario Tests (Week 2)
- Create test fixtures for 10 core scenarios
- Implement defect detection validation for each
- Verify impact scoring accuracy
- Target: All scenarios passing

### Phase 3: Integration Tests (Week 3)
- Implement E2E tests with mock Spectre API
- Test against real MCP clients (manual)
- Implement property-based tests
- Target: Full workflow coverage

### Phase 4: Performance & CI (Week 4)
- Implement benchmarks and load tests
- Set up CI pipeline
- Document performance baselines
- Target: All tests automated

## Success Criteria

1. ✅ **Defect Detection**: All 10+ scenarios correctly detected with appropriate status/impact
2. ✅ **Protocol Compliance**: All MCP protocol tests passing
3. ✅ **Performance**: Response times within acceptance criteria
4. ✅ **Coverage**: >80% code coverage for MCP and storage packages
5. ✅ **CI/CD**: All tests automated and running on every commit
6. ✅ **Documentation**: Test cases documented, runnable by new contributors

## Maintenance Strategy

- **Regression testing**: Add test case for every bug fix
- **Scenario expansion**: Add new scenarios as real-world incidents occur
- **Performance monitoring**: Track benchmark trends over time
- **Client compatibility**: Test against new MCP client versions quarterly

## Tools & Dependencies

- Go testing framework (`testing` package)
- Test assertion library: `github.com/stretchr/testify`
- Property-based testing: `github.com/leanovate/gopter`
- HTTP testing: `net/http/httptest`
- MCP client testing: MCP inspector tool
- CI: GitHub Actions
- Coverage: `go tool cover`

## Open Questions

1. Should we test against a real Kubernetes cluster or rely on synthetic data?
   - **Recommendation**: Synthetic for unit/scenario tests, optional real cluster for integration tests

2. How to test investigate prompts quality?
   - **Recommendation**: Manual review + LLM-based evaluation (if applicable)

3. Should we test backward compatibility with older Spectre API versions?
   - **Recommendation**: Test against latest API version, document minimum version

4. What's the acceptable performance baseline for large clusters?
   - **Recommendation**: Start with proposed criteria, adjust based on real-world usage

## Next Steps

1. Review and approve this test strategy
2. Create GitHub issues for each testing phase
3. Implement Phase 1 (unit tests) first
4. Iterate and refine based on findings
