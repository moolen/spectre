# MCP Failure Scenarios E2E Tests

## Overview

This test suite provides comprehensive end-to-end testing of the MCP (Model Context Protocol) server's failure detection and analysis capabilities. Each test scenario deploys a specific Kubernetes failure condition and validates that all four MCP tools correctly identify, analyze, and report the issue.

## Test Architecture

### Test Structure
- **Stage-based Testing**: Uses the Given-When-Then pattern for readable test scenarios
- **HTTP Transport**: All tests use MCP HTTP transport (not stdio)
- **Isolated Namespaces**: Each test runs in its own namespace for isolation
- **Real Failures**: Tests deploy actual failing Kubernetes resources

### Files
```
tests/e2e/
├── mcp_failure_scenarios_test.go           # Main test scenarios (9 tests)
├── mcp_failure_scenarios_stage_test.go     # Stage helper methods
└── fixtures/                                # YAML fixtures for failures
    ├── crashloop-pod.yaml
    ├── imagepull-pod.yaml
    ├── healthy-deployment.yaml
    ├── crashloop-deployment.yaml
    ├── oom-pod.yaml
    ├── unschedulable-pod.yaml
    ├── liveness-failure-pod.yaml
    ├── readiness-failure-pod.yaml
    └── failing-pvc.yaml
```

## Test Scenarios

### Scenario 1: Pod CrashLoopBackOff
**File**: `TestMCP_Scenario1_CrashLoopBackOff`  
**Failure**: Container that exits immediately with non-zero code  
**Validates**:
- cluster_health detects Error status
- Top issues include CrashLoopBackOff
- investigate provides RCA prompts
- resource_changes detects CrashLoopBackOff container issue (impact: 0.35)
- Impact score exceeds 0.30

### Scenario 2: Pod ImagePullBackOff
**File**: `TestMCP_Scenario2_ImagePullBackOff`  
**Failure**: Pod with non-existent image tag  
**Validates**:
- cluster_health detects Error status
- Top issues include ImagePullBackOff
- investigate shows relevant events
- resource_changes detects ImagePullBackOff container issue (impact: 0.25)
- Impact score exceeds 0.20

### Scenario 3: Deployment Configuration Change
**File**: `TestMCP_Scenario3_DeploymentConfigChange`  
**Failure**: Healthy deployment updated to crash on startup  
**Validates**:
- cluster_health detects transition from healthy to error
- investigate shows Ready → Error status transition
- Timeline documents the configuration change
- resource_changes tracks status transitions
- Impact score exceeds 0.30

### Scenario 4: Pod OOMKilled
**File**: `TestMCP_Scenario4_OOMKilled`  
**Failure**: Pod with memory limit too low for workload  
**Validates**:
- cluster_health detects Error with OOMKilled
- investigate shows exit code 137 in container termination
- resource_changes detects OOMKilled container issue (impact: 0.40 - highest)
- Impact score exceeds 0.35

### Scenario 5: Pod Scheduling Failure
**File**: `TestMCP_Scenario5_SchedulingFailure`  
**Failure**: Pod requesting impossible resources (1000 CPU cores)  
**Validates**:
- cluster_health detects Error/Warning
- investigate shows FailedScheduling events with constraint details
- resource_changes detects FailedScheduling event pattern (impact: 0.30)
- Impact score exceeds 0.25

### Scenario 7: Liveness Probe Failure
**File**: `TestMCP_Scenario7_LivenessProbeFailure`  
**Failure**: Pod with liveness probe that fails after initial success  
**Validates**:
- cluster_health detects increasing restart count
- investigate shows liveness probe failure events
- Timeline shows multiple restarts
- resource_changes detects LivenessProbe event pattern (impact: 0.35)
- Impact score exceeds 0.30

### Scenario 8: Readiness Probe Failure
**File**: `TestMCP_Scenario8_ReadinessProbeFailure`  
**Failure**: Pod that never becomes ready (failing readiness probe)  
**Validates**:
- cluster_health shows Warning status (Running but not Ready)
- investigate shows prolonged Warning state
- resource_changes detects ReadinessProbe event pattern (impact: 0.25)
- Impact score exceeds 0.20

### Scenario 9: PVC Provisioning Failure
**File**: `TestMCP_Scenario9_PVCProvisioningFailure`  
**Failure**: PVC requesting non-existent storage class  
**Validates**:
- cluster_health detects Error/Warning
- investigate shows provisioning failure events
- resource_changes shows Pending phase
- Events explain why provisioning failed

### Summary Test: All Scenarios Quick Check
**File**: `TestMCP_AllScenarios_Summary`  
**Purpose**: Quick validation that all tools work with basic failure scenarios  
**Runs**: CrashLoopBackOff, ImagePullBackOff, SchedulingFailure (basic checks only)

## MCP Tools Validated

Each test validates all four MCP tools:

### 1. cluster_health
- Overall cluster status (Healthy/Degraded/Critical)
- Resource counts by status (Ready, Warning, Error, Terminating)
- Top issues with error messages
- Error duration tracking

### 2. investigate
- Status timeline with transitions
- Event history
- Investigation prompts for RCA
- Current status and message
- Container-level details

### 3. resource_changes
- Container issues detection (CrashLoopBackOff, ImagePullBackOff, OOMKilled)
- Event pattern detection (FailedScheduling, ProbeFailure, Evicted)
- Impact scoring (0.0-1.0)
- Status transitions
- Change categorization

### 4. resource_explorer
- Resource discovery with filtering
- Status overview
- Issue counting
- Last status change tracking

## Running the Tests

### Run All Failure Scenario Tests
```bash
cd /home/moritz/dev/spectre
go test -v ./tests/e2e -run TestMCP_Scenario
```

### Run Specific Scenario
```bash
go test -v ./tests/e2e -run TestMCP_Scenario2_ImagePullBackOff
```

### Run Summary Test (Quick Validation)
```bash
go test -v ./tests/e2e -run TestMCP_AllScenarios_Summary
```

### Skip in Short Mode
```bash
go test -short ./tests/e2e  # Skips all e2e tests
```

## Test Timing

Typical timing per scenario:
- **CrashLoopBackOff**: ~60-80 seconds
- **ImagePullBackOff**: ~60-80 seconds  
- **Configuration Change**: ~120-150 seconds (deploys twice)
- **OOMKilled**: ~60-70 seconds
- **Scheduling Failure**: ~40-50 seconds
- **Liveness Probe**: ~90-100 seconds
- **Readiness Probe**: ~60-70 seconds
- **PVC Failure**: ~40-50 seconds

**Total suite**: ~10-15 minutes for all scenarios

## Implementation Details

### Stage Methods

#### Setup Stages
- `a_test_environment()` - Creates isolated test namespace
- `mcp_server_is_deployed()` - Enables MCP in Helm deployment
- `mcp_client_is_connected()` - Establishes HTTP client connection

#### Deployment Stages
- `failure_scenario_is_deployed(fixture)` - Deploys failure from YAML
- `deployment_is_updated(from, to)` - Updates deployment config
- `wait_for_condition(duration)` - Waits for failure to manifest
- `failure_condition_is_observed(duration)` - Sets query time window

#### Tool Invocation Stages
- `cluster_health_tool_is_called()` - Calls cluster_health tool
- `investigate_tool_is_called_for_resource(kind, name)` - Calls investigate
- `resource_changes_tool_is_called()` - Calls resource_changes
- `resource_explorer_tool_is_called()` - Calls resource_explorer

#### Assertion Stages
- `cluster_health_detects_error()` - Validates error detection
- `cluster_health_shows_expected_issue(type)` - Checks top issues
- `investigate_shows_status_transition(from, to)` - Validates timeline
- `investigate_provides_rca_prompts()` - Checks RCA prompts exist
- `investigate_event_count_exceeds(count)` - Validates events
- `resource_changes_has_container_issue(type)` - Checks container issues
- `resource_changes_has_event_pattern(type)` - Checks event patterns
- `resource_changes_impact_score_exceeds(threshold)` - Validates impact
- `resource_explorer_shows_error_status()` - Checks discovery

### Assertion Strategy

Tests use **lenient assertions** to handle timing variations:
- Some assertions log warnings instead of failing (e.g., resource_explorer timing)
- Top issues may not always populate immediately
- Resource indexing has inherent delays

This approach ensures tests validate **capability** rather than **perfect timing**.

## Known Limitations

1. **Timing Sensitivity**: Resource indexing and event capture have inherent delays
2. **Node Pressure**: Scenario 6 (Node Pressure) not implemented (requires cluster-wide pressure simulation)
3. **Cascading Failures**: Scenario 10 (Multi-Resource Cascading) not yet implemented
4. **Shared Cluster**: Tests run on shared Kind cluster, may have cross-test pollution

## Future Enhancements

### Phase 1: Complete Coverage
- [ ] Add Scenario 6: Node Pressure testing
- [ ] Add Scenario 10: Cascading failure across multiple resources
- [ ] Add tests for DaemonSet failures
- [ ] Add tests for StatefulSet failures

### Phase 2: Enhanced Validation
- [ ] Add strict cross-tool consistency checks
- [ ] Validate timestamps align across tools
- [ ] Check that error counts match across tools
- [ ] Validate resource identification consistency

### Phase 3: Performance Testing
- [ ] Add assertions for query performance (< 5s per tool)
- [ ] Test with high resource counts (1000+ resources)
- [ ] Test with high event volumes
- [ ] Load testing for concurrent queries

### Phase 4: Advanced Scenarios
- [ ] Network policy violations
- [ ] Certificate expiration
- [ ] Resource quota exhaustion
- [ ] Service mesh failures
- [ ] Ingress configuration errors

## Production Readiness Checklist

- [x] All container issues detected (CrashLoopBackOff, ImagePullBackOff, OOMKilled)
- [x] All major event patterns detected (FailedScheduling, ProbeFailures)
- [x] All four MCP tools tested for each scenario
- [x] Impact scoring validated for accuracy
- [x] RCA prompts generated appropriately
- [x] Timeline tracking works correctly
- [x] HTTP transport validated
- [ ] Performance benchmarks established
- [ ] Cross-tool consistency fully validated
- [ ] Documentation complete

## Contributing

When adding new scenarios:

1. Create YAML fixture in `fixtures/` directory
2. Add test function in `mcp_failure_scenarios_test.go`
3. Use existing stage methods or add new ones as needed
4. Validate all four tools in the test
5. Update this README with scenario details
6. Ensure proper cleanup (automatic via namespace deletion)

## Troubleshooting

### Test Fails with "resources should be present"
- **Cause**: Resource not indexed yet by Spectre
- **Solution**: Increase wait times in test or check Spectre logs

### Test Fails with "top_issues not present"
- **Cause**: Timing - issue hasn't been categorized yet
- **Solution**: Lenient assertions already in place; check if error status detected

### Test Times Out
- **Cause**: MCP server not responding or resource stuck
- **Solution**: Check MCP server logs, increase timeouts

### Port Forward Failures
- **Cause**: Previous test didn't clean up properly
- **Solution**: Tests auto-cleanup; check for orphaned port-forwards

## References

- [MCP Tools Implementation](../../internal/mcp/tools/)
- [Analyzer Package](../../internal/analyzer/)
- [Event Patterns](../../internal/storage/event_patterns.go)
- [Implementation Plan](../../MCP_E2E_FAILURE_SCENARIOS_PLAN.md)
