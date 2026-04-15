# MCP Health Checks Implementation Plan

## Overview
This plan outlines the implementation of missing health checks and error/warning detection logic for the Spectre MCP server. The goal is to comprehensively detect all critical Kubernetes issues that could lead to incidents.

## Current State Analysis

### Already Implemented ✅

The status inference logic (`internal/storage/status_inference.go`) currently detects:

1. **Pod Status**
   - Phase detection (Running/Pending/Failed/Unknown)
   - Ready condition evaluation
   - Container state checking

2. **Workload Status**
   - Deployment replica mismatches
   - StatefulSet readiness
   - DaemonSet unavailable pods
   - Job completion/failure

3. **Node Health**
   - Ready condition
   - Basic pressure conditions (Memory/Disk/PID/Network)

4. **Storage**
   - PVC phase (Bound/Pending/Lost)

5. **Generic Condition Checking**
   - Ready, Healthy, Degraded, Failing, Failed, Stalled conditions
   - Error keyword detection in messages

### Missing Checks ❌

Based on the requirements list, the following are **NOT** yet implemented:

1. **Pod Container States**
   - CrashLoopBackOff detection (partial - needs explicit check)
   - ImagePullBackOff detection
   - OOMKilled detection
   - Restart count tracking (>0 frequently)

2. **Pod Readiness/Liveness Probes**
   - Readiness probe failures
   - Liveness probe failures
   - Startup probe failures

3. **Pod Scheduling Issues**
   - FailedScheduling events
   - Unschedulable due to node selector mismatch
   - Unschedulable due to affinity/anti-affinity conflicts
   - Insufficient resources (CPU/Memory)
   - Taints/tolerations mismatch

4. **Resource Limits & Pressure**
   - Container hitting CPU limits
   - Container hitting memory limits
   - Node resource exhaustion

5. **Pod Lifecycle Events**
   - Pod eviction events
   - Node drain events
   - Preemption events

6. **Service & Networking**
   - Service endpoint absence (no backing pods)
   - DNS failures (pod event messages)
   - DNS pod restart counts

7. **Bulk Operations**
   - Namespace deletion detection (partially via DELETE events)
   - CRD deletion detection
   - Bulk resource deletion patterns

8. **Node Conditions**
   - Enhanced pressure detection (severity levels)
   - NotReady nodes
   - NetworkUnavailable condition

## Implementation Strategy

### Approach

We'll enhance the existing status inference system with:
1. **Container state inspection** - Parse pod container statuses for specific issues
2. **Event pattern matching** - Detect specific event reasons and messages
3. **Probe failure detection** - Check container state for probe failures
4. **Scheduling failure analysis** - Parse FailedScheduling event messages
5. **Resource limit detection** - Check container state for resource limits
6. **Bulk operation detection** - Identify patterns in timeline data
7. **Enhanced impact scoring** - Weight new failure modes appropriately

### File Changes Required

1. **`internal/storage/status_inference.go`**
   - Enhance `inferPodStatus()` function
   - Add helper functions for container state inspection
   - Add event reason pattern matching

2. **`internal/mcp/tools/resource_changes.go`**
   - Update `calculateImpactScore()` to weight new failure modes
   - Add detection for bulk operations

3. **`internal/mcp/tools/cluster_health.go`**
   - Add specific issue categorization (CrashLoop, OOMKill, etc.)
   - Enhance top issues reporting with issue types

4. **New file: `internal/storage/container_states.go`**
   - Container state inspection utilities
   - Restart count analysis
   - Termination reason detection

5. **New file: `internal/storage/event_patterns.go`**
   - Event reason/message pattern matching
   - Scheduling failure parsing
   - DNS failure detection

## Detailed Implementation Plan

### 1. Pod Container State Detection

**File**: `internal/storage/container_states.go` (new)

**Functionality**:
```go
// ContainerIssue represents a specific container-level problem
type ContainerIssue struct {
    ContainerName string
    IssueType     string // "CrashLoopBackOff", "ImagePullBackOff", "OOMKilled", etc.
    Reason        string
    Message       string
    RestartCount  int32
    ExitCode      int32
}

// InspectContainerStates analyzes pod container statuses for issues
func InspectContainerStates(pod *corev1.Pod) []ContainerIssue

// Helper functions
func detectCrashLoopBackOff(state corev1.ContainerState, status corev1.ContainerStatus) bool
func detectImagePullBackOff(state corev1.ContainerState) bool
func detectOOMKilled(state corev1.ContainerState) bool
func detectResourceLimitHit(state corev1.ContainerState) bool
func isHighRestartCount(restartCount int32, podAge time.Duration) bool
```

**Detection Logic**:

1. **CrashLoopBackOff**
   - Check `containerStatus.State.Waiting.Reason == "CrashLoopBackOff"`
   - OR check `containerStatus.LastTerminationState.Terminated.Reason` contains "Error"
   - RestartCount > 0

2. **ImagePullBackOff**
   - Check `containerStatus.State.Waiting.Reason == "ImagePullBackOff"` or `"ErrImagePull"`
   - Container not ready

3. **OOMKilled**
   - Check `containerStatus.LastTerminationState.Terminated.Reason == "OOMKilled"`
   - OR `containerStatus.LastTerminationState.Terminated.ExitCode == 137`

4. **Resource Limits**
   - ExitCode 137 (SIGKILL - often memory limit)
   - Message contains "memory limit" or "cpu throttling"

5. **High Restart Count**
   - RestartCount > 5 in last hour
   - RestartCount > 10 in last 24 hours
   - Increasing trend (requires historical data)

**Integration**:
- Modify `inferPodStatus()` in `status_inference.go` to call `InspectContainerStates()`
- If any critical issues found, return `StatusError` with detailed message
- If any warning-level issues, return `StatusWarning`

**Changes to `internal/storage/status_inference.go`**:
```go
func inferPodStatus(obj runtime.Object) (ResourceStatus, string) {
    pod, ok := obj.(*corev1.Pod)
    if !ok {
        return StatusUnknown, "not a pod"
    }

    // NEW: Inspect container states first
    containerIssues := InspectContainerStates(pod)
    if len(containerIssues) > 0 {
        // Categorize by severity
        for _, issue := range containerIssues {
            switch issue.IssueType {
            case "CrashLoopBackOff", "OOMKilled":
                return StatusError, fmt.Sprintf("Container %s: %s", issue.ContainerName, issue.IssueType)
            case "ImagePullBackOff":
                return StatusError, fmt.Sprintf("Container %s: %s", issue.ContainerName, issue.Message)
            }
        }
    }

    // ... existing pod phase and condition logic ...
}
```

### 2. Probe Failure Detection

**File**: `internal/storage/container_states.go` (extend)

**Functionality**:
```go
type ProbeFailure struct {
    ContainerName string
    ProbeType     string // "readiness", "liveness", "startup"
    FailureCount  int
    Reason        string
    Message       string
}

func DetectProbeFailures(pod *corev1.Pod, events []Event) []ProbeFailure
```

**Detection Logic**:

1. **Readiness Probe Failure**
   - Events with reason "Unhealthy" and message contains "Readiness probe failed"
   - Container status: `Ready == false`
   - Check `containerStatus.State.Running` (pod is running but not ready)

2. **Liveness Probe Failure**
   - Events with reason "Unhealthy" and message contains "Liveness probe failed"
   - May be followed by container restart

3. **Startup Probe Failure**
   - Events with message contains "Startup probe failed"
   - Container never reaches Running state

**Integration**:
- Requires event correlation (events passed to status inference)
- Modify signature: `func inferPodStatus(obj runtime.Object, events []Event) (ResourceStatus, string)`
- Return `StatusWarning` for readiness failures (after upgrade)
- Return `StatusError` for persistent liveness failures

**Impact**: Requires refactoring status inference to accept events as parameter

### 3. Scheduling Failure Detection

**File**: `internal/storage/event_patterns.go` (new)

**Functionality**:
```go
type SchedulingIssue struct {
    Reason      string // "FailedScheduling"
    Constraints []string // ["Insufficient cpu", "node selector mismatch"]
    NodeCount   int // Total nodes evaluated
    Message     string
}

func ParseSchedulingFailure(eventMessage string) *SchedulingIssue
```

**Detection Logic**:

Parse event messages like:
- `"0/5 nodes are available: 3 Insufficient cpu, 2 node(s) didn't match node selector."`
- `"0/10 nodes are available: 5 node(s) had taint {key: value}, 5 Insufficient memory."`
- `"no nodes available to schedule pods"`

Extract:
- Total nodes evaluated
- Individual constraints preventing scheduling
- Categorize by constraint type (resources, selectors, taints, affinity)

**Integration**:
- Called from `inferPodStatus()` when pod is in Pending phase
- Check events for "FailedScheduling" reason
- Return `StatusError` if unschedulable for >5 minutes
- Return `StatusWarning` if recently unschedulable

**Changes to `internal/storage/status_inference.go`**:
```go
func inferPodStatus(obj runtime.Object, events []Event) (ResourceStatus, string) {
    // ... existing code ...

    if pod.Status.Phase == corev1.PodPending {
        // NEW: Check for scheduling failures
        for _, event := range events {
            if event.Reason == "FailedScheduling" {
                issue := ParseSchedulingFailure(event.Message)
                if issue != nil {
                    // Check age
                    if time.Since(event.FirstTimestamp) > 5*time.Minute {
                        return StatusError, fmt.Sprintf("Unschedulable: %s", event.Message)
                    }
                    return StatusWarning, fmt.Sprintf("Scheduling delayed: %s", event.Message)
                }
            }
        }
    }

    // ... rest of logic ...
}
```

### 4. Node Pressure & Resource Exhaustion

**File**: `internal/storage/status_inference.go` (enhance existing)

**Current Implementation**:
```go
func inferNodeStatus(obj runtime.Object) (ResourceStatus, string) {
    // ... existing code checks for MemoryPressure, DiskPressure, etc. ...
}
```

**Enhancements**:

1. **Severity Levels**
   - Pressure=True → Warning
   - Out of resources (allocatable == 0) → Error

2. **Additional Conditions**
   - `Ready=False` → Error (node not healthy)
   - `NetworkUnavailable=True` → Error
   - `PIDPressure=True` → Warning

3. **Capacity vs. Allocatable**
   - Check if allocatable CPU/Memory approaching 0
   - Warn if >90% allocated

**Changes**:
```go
func inferNodeStatus(obj runtime.Object) (ResourceStatus, string) {
    node, ok := obj.(*corev1.Node)
    if !ok {
        return StatusUnknown, "not a node"
    }

    // Check if node is ready
    for _, cond := range node.Status.Conditions {
        if cond.Type == corev1.NodeReady {
            if cond.Status != corev1.ConditionTrue {
                return StatusError, fmt.Sprintf("Node not ready: %s", cond.Reason)
            }
        }
    }

    // Enhanced pressure detection
    pressureIssues := []string{}
    for _, cond := range node.Status.Conditions {
        if cond.Status == corev1.ConditionTrue {
            switch cond.Type {
            case corev1.NodeMemoryPressure:
                pressureIssues = append(pressureIssues, "MemoryPressure")
            case corev1.NodeDiskPressure:
                pressureIssues = append(pressureIssues, "DiskPressure")
            case corev1.NodePIDPressure:
                pressureIssues = append(pressureIssues, "PIDPressure")
            case corev1.NodeNetworkUnavailable:
                return StatusError, "Network unavailable"
            }
        }
    }

    if len(pressureIssues) > 0 {
        return StatusWarning, fmt.Sprintf("Node pressure: %s", strings.Join(pressureIssues, ", "))
    }

    // NEW: Check resource exhaustion
    cpu := node.Status.Allocatable[corev1.ResourceCPU]
    memory := node.Status.Allocatable[corev1.ResourceMemory]
    if cpu.IsZero() || memory.IsZero() {
        return StatusError, "Node has no allocatable resources"
    }

    return StatusReady, "Node healthy"
}
```

### 5. Service Endpoint Detection

**File**: `internal/storage/status_inference.go` (new function)

**Functionality**:
```go
func inferEndpointsStatus(obj runtime.Object) (ResourceStatus, string)
```

**Detection Logic**:

For Endpoints resources:
- Check if `endpoints.Subsets` is empty or all have 0 ready addresses
- Cross-reference with Service (if available)
- Return Warning if no endpoints but Service exists

For EndpointSlice (newer API):
- Check if all slices have 0 ready endpoints

**Integration**:
- Add case in main status inference switch
- May require timeline correlation (check related Pods)

**Limitations**: This requires expanding timeline storage to include Endpoints/EndpointSlice resources

### 6. DNS Failure Detection

**File**: `internal/storage/event_patterns.go` (extend)

**Functionality**:
```go
func DetectDNSIssues(events []Event) []DNSIssue

type DNSIssue struct {
    PodName   string
    Reason    string
    Message   string
    Timestamp time.Time
}
```

**Detection Logic**:

Parse event messages for:
- `"dial tcp: lookup <hostname> on <dns-ip>: no such host"`
- `"nslookup: can't resolve"`
- `"Temporary failure in name resolution"`

Also check:
- CoreDNS/kube-dns pod health
- DNS pod restart counts
- DNS pod readiness failures

**Integration**:
- Call from cluster_health to identify DNS-related issues
- Correlate events across pods to detect cluster-wide DNS problems

### 7. Bulk Operation Detection

**File**: `internal/mcp/tools/resource_changes.go` (enhance)

**Functionality**:
```go
func DetectBulkOperations(changes []ResourceChange, timeline Timeline) []BulkOperation

type BulkOperation struct {
    OperationType string // "deletion", "creation", "scaling"
    ResourceKind  string
    Namespace     string
    Count         int
    Timespan      time.Duration
    ImpactScore   float64
}
```

**Detection Logic**:

1. **Namespace Deletion**
   - Multiple resources in same namespace with DELETE events
   - All within short timespan (<1 minute)
   - Parent namespace has deletionTimestamp

2. **CRD Deletion**
   - CustomResourceDefinition DELETE event
   - Followed by deletion of all instances

3. **Bulk Deletion**
   - >10 resources of same kind deleted within 5 minutes
   - Not explained by namespace/CRD deletion

4. **Bulk Scaling**
   - Multiple Deployments/StatefulSets scaled (up or down) simultaneously
   - Replica count changes detected

**Integration**:
- Run detection in `resource_changes` tool
- Return bulk operations as separate field in response
- Boost impact scores for resources involved in bulk operations

**Changes to `internal/mcp/tools/resource_changes.go`**:
```go
func (t *ResourceChangesTool) Execute(params map[string]interface{}, timeline *storage.Timeline) (interface{}, error) {
    // ... existing code to get changes ...

    // NEW: Detect bulk operations
    bulkOps := DetectBulkOperations(changes, timeline)

    // Boost impact scores for resources in bulk operations
    for i := range changes {
        for _, bulkOp := range bulkOps {
            if changes[i].Namespace == bulkOp.Namespace && changes[i].Kind == bulkOp.ResourceKind {
                changes[i].ImpactScore = math.Min(changes[i].ImpactScore + 0.2, 1.0)
            }
        }
    }

    return map[string]interface{}{
        "changes":         filteredChanges,
        "bulk_operations": bulkOps,
        "total_changes":   len(changes),
    }, nil
}
```

### 8. Pod Eviction & Preemption Detection

**File**: `internal/storage/event_patterns.go` (extend)

**Functionality**:
```go
func DetectEvictionEvents(events []Event) []EvictionEvent

type EvictionEvent struct {
    PodName   string
    Reason    string // "Evicted", "Preempted"
    Cause     string // "OutOfMemory", "DiskPressure", "HigherPriorityPod"
    NodeName  string
    Message   string
    Timestamp time.Time
}
```

**Detection Logic**:

Event reasons to detect:
- `"Evicted"` with message parsing:
  - `"The node was low on resource: memory"`
  - `"The node had condition: [DiskPressure]"`
- `"Preempted"` with message: `"Preempted by <namespace>/<pod> on node <node>"`

**Integration**:
- Call from `inferPodStatus()` when pod is in Failed phase
- Categorize evictions:
  - Resource pressure → Warning (node issue, not pod issue)
  - Pod resource limits → Error (pod issue)

### 9. Node Drain Detection

**File**: `internal/storage/event_patterns.go` (extend)

**Functionality**:
```go
func DetectNodeDrainEvents(events []Event, nodes []Node) []NodeDrainEvent

type NodeDrainEvent struct {
    NodeName      string
    Reason        string // "Draining", "Cordoned"
    PodsEvicted   int
    Timestamp     time.Time
}
```

**Detection Logic**:

Detect:
- Node becomes unschedulable (`node.Spec.Unschedulable == true`)
- Followed by pod eviction events on that node
- Event reason `"NodeDrain"` or `"EvictPod"`

**Integration**:
- Correlate node status changes with pod evictions
- Return Warning for planned drains
- Include in cluster_health top issues

### 10. Enhanced Impact Scoring

**File**: `internal/mcp/tools/resource_changes.go` (enhance)

**Current Scoring**:
```go
score := 0.0
if errorCount > 0 { score += 0.3 }
if warningCount > 0 { score += 0.15 }
// ... transition scoring ...
if eventCount > 10 { score += 0.1 }
return min(score, 1.0)
```

**Enhanced Scoring**:
```go
func calculateEnhancedImpactScore(change ResourceChange, containerIssues []ContainerIssue, events []Event) float64 {
    score := 0.0

    // Existing scoring
    if change.ErrorCount > 0 { score += 0.3 }
    if change.WarningCount > 0 { score += 0.15 }
    for _, trans := range change.StatusTransitions {
        if trans.ToStatus == "Error" { score += 0.3 }
        if trans.ToStatus == "Warning" { score += 0.15 }
    }

    // NEW: Container issue severity
    for _, issue := range containerIssues {
        switch issue.IssueType {
        case "OOMKilled":
            score += 0.4 // High impact - resource issue
        case "CrashLoopBackOff":
            score += 0.35 // High impact - app issue
        case "ImagePullBackOff":
            score += 0.25 // Medium impact - config issue
        }

        // Frequent restarts
        if issue.RestartCount > 10 {
            score += 0.2
        }
    }

    // NEW: Scheduling issues
    hasSchedulingIssue := false
    for _, event := range events {
        if event.Reason == "FailedScheduling" {
            hasSchedulingIssue = true
            score += 0.3
            break
        }
    }

    // NEW: Eviction/Preemption
    for _, event := range events {
        if event.Reason == "Evicted" {
            score += 0.35
        }
        if event.Reason == "Preempted" {
            score += 0.25
        }
    }

    // NEW: Probe failures
    probeFailureCount := 0
    for _, event := range events {
        if event.Reason == "Unhealthy" && strings.Contains(event.Message, "probe failed") {
            probeFailureCount++
        }
    }
    if probeFailureCount > 5 {
        score += 0.2
    }

    // High event volume
    if len(events) > 10 { score += 0.1 }
    if len(events) > 50 { score += 0.2 } // Very high churn

    return min(score, 1.0)
}
```

## Implementation Phases

### Phase 1: Container State Detection (Priority: High)
**Estimated Effort**: 2-3 days

Tasks:
1. Create `internal/storage/container_states.go`
2. Implement container issue detection functions
3. Modify `inferPodStatus()` to use container inspection
4. Add unit tests for each container issue type
5. Update impact scoring to include container issues

**Files Changed**:
- `internal/storage/container_states.go` (new)
- `internal/storage/status_inference.go` (modified)
- `internal/mcp/tools/resource_changes.go` (modified)

**Deliverables**:
- Detects: CrashLoopBackOff, ImagePullBackOff, OOMKilled, high restart counts
- Enhanced impact scoring

### Phase 2: Event Pattern Matching (Priority: High)
**Estimated Effort**: 3-4 days

Tasks:
1. Create `internal/storage/event_patterns.go`
2. Implement scheduling failure parser
3. Implement eviction/preemption detection
4. Implement DNS failure detection
5. Refactor status inference to accept events
6. Add unit tests for pattern matching

**Files Changed**:
- `internal/storage/event_patterns.go` (new)
- `internal/storage/status_inference.go` (modified - signature changes)
- All callsites of status inference functions (modified)

**Deliverables**:
- Detects: FailedScheduling, Evictions, Preemptions, DNS issues
- Event-aware status inference

### Phase 3: Probe Failure Detection (Priority: Medium)
**Estimated Effort**: 2 days

Tasks:
1. Extend container state detection for probes
2. Implement probe failure detection function
3. Correlate events with container states
4. Add to status inference logic
5. Add unit tests

**Files Changed**:
- `internal/storage/container_states.go` (extended)
- `internal/storage/status_inference.go` (modified)

**Deliverables**:
- Detects: Readiness, Liveness, Startup probe failures

### Phase 4: Enhanced Node Detection (Priority: Medium)
**Estimated Effort**: 1-2 days

Tasks:
1. Enhance `inferNodeStatus()` function
2. Add severity levels for pressure conditions
3. Add resource exhaustion detection
4. Add NotReady node detection
5. Add unit tests

**Files Changed**:
- `internal/storage/status_inference.go` (modified)

**Deliverables**:
- Improved node health detection
- Severity-based status assignment

### Phase 5: Bulk Operation Detection (Priority: Medium)
**Estimated Effort**: 2-3 days

Tasks:
1. Implement bulk operation detection in resource_changes
2. Add namespace deletion cascade detection
3. Add CRD deletion detection
4. Boost impact scores for bulk operations
5. Add to cluster_health reporting
6. Add unit tests

**Files Changed**:
- `internal/mcp/tools/resource_changes.go` (modified)
- `internal/mcp/tools/cluster_health.go` (modified)

**Deliverables**:
- Detects: Namespace deletions, CRD deletions, bulk deletions
- Enhanced reporting

### Phase 6: Service & Networking (Priority: Low)
**Estimated Effort**: 2-3 days

Tasks:
1. Add Endpoints/EndpointSlice to timeline storage
2. Implement endpoint status inference
3. Add DNS issue correlation
4. Add service health checks
5. Add unit tests

**Files Changed**:
- `internal/storage/types.go` (extend to include Endpoints)
- `internal/storage/status_inference.go` (new inference function)
- Data collection (may need changes)

**Deliverables**:
- Detects: Service endpoint absence, DNS issues
- Requires storage schema changes

### Phase 7: Node Drain Detection (Priority: Low)
**Estimated Effort**: 1-2 days

Tasks:
1. Implement node drain event detection
2. Correlate node unschedulable with pod evictions
3. Add to cluster_health reporting
4. Add unit tests

**Files Changed**:
- `internal/storage/event_patterns.go` (extended)
- `internal/mcp/tools/cluster_health.go` (modified)

**Deliverables**:
- Detects: Node drains with pod evictions

## Testing Strategy for Each Phase

For each phase:
1. **Unit Tests**: Test each new detection function with positive/negative cases
2. **Integration Tests**: Test end-to-end with synthetic timeline data
3. **Scenario Tests**: Create specific scenario fixtures for new issue types
4. **Impact Scoring Tests**: Verify impact scores are appropriate

Example test structure:
```go
func TestDetectCrashLoopBackOff(t *testing.T) {
    pod := &corev1.Pod{
        Status: corev1.PodStatus{
            ContainerStatuses: []corev1.ContainerStatus{
                {
                    Name:         "app",
                    RestartCount: 5,
                    State: corev1.ContainerState{
                        Waiting: &corev1.ContainerStateWaiting{
                            Reason: "CrashLoopBackOff",
                        },
                    },
                },
            },
        },
    }

    issues := InspectContainerStates(pod)

    assert.Len(t, issues, 1)
    assert.Equal(t, "CrashLoopBackOff", issues[0].IssueType)
    assert.Equal(t, int32(5), issues[0].RestartCount)
}
```

## Migration & Backward Compatibility

### Breaking Changes

**Phase 2** introduces breaking changes:
- Status inference function signatures change to accept events
- Requires all callsites to be updated

**Mitigation**:
1. Create new functions with `V2` suffix during transition
2. Migrate callsites incrementally
3. Remove old functions once migration complete

### Data Schema Changes

**Phase 6** requires extending timeline storage to include Endpoints/EndpointSlice resources.

**Mitigation**:
1. Make field additions optional
2. Ensure backward compatibility with existing data
3. Document storage version requirements

## Validation & Rollout

### Validation Steps

For each phase:
1. ✅ Unit tests passing (>80% coverage for new code)
2. ✅ Integration tests passing
3. ✅ Manual testing with real Kubernetes clusters
4. ✅ Performance benchmarks acceptable (no regression)
5. ✅ Documentation updated

### Rollout Strategy

1. **Internal testing**: Deploy to test environment with synthetic data
2. **Alpha testing**: Deploy to staging environment with real cluster data
3. **Beta testing**: Deploy to subset of production users
4. **GA**: Full rollout after validation

## Success Metrics

After implementation, we should be able to:

1. ✅ Detect 100% of issue types in the requirements list
2. ✅ Assign appropriate status (Ready/Warning/Error) for each issue
3. ✅ Calculate accurate impact scores (0-1.0) weighted by severity
4. ✅ Provide detailed error messages for root cause analysis
5. ✅ Maintain response time <3s for typical queries
6. ✅ Pass all unit, integration, and scenario tests

## Documentation Requirements

For each phase:
1. Update API documentation with new detection capabilities
2. Document error/warning conditions and their status mappings
3. Update impact scoring algorithm documentation
4. Provide example queries and expected responses
5. Add troubleshooting guide for common issues

## Dependencies & Prerequisites

### Code Dependencies
- Kubernetes client-go library (already present)
- Go testing framework
- Mock/fixture generation utilities

### Knowledge Dependencies
- Kubernetes resource schemas
- Common failure patterns in production clusters
- Event reason and message formats

### Data Dependencies
- Access to Kubernetes audit logs
- Timeline storage with sufficient resource types
- Event data for correlation

## Risk Assessment & Mitigation

### Risks

1. **Performance degradation** from additional checks
   - **Mitigation**: Benchmark each phase, optimize hot paths

2. **False positives** from overly aggressive detection
   - **Mitigation**: Tune thresholds based on real-world data, add configuration

3. **Breaking changes** in status inference signatures
   - **Mitigation**: Phased migration with V2 functions

4. **Storage schema changes** requiring data migration
   - **Mitigation**: Make changes backward compatible, version schema

5. **Complexity increase** making maintenance harder
   - **Mitigation**: Comprehensive tests, clear documentation, modular design

## Future Enhancements (Out of Scope)

These are valuable but not in the current plan:

1. **Machine learning-based anomaly detection**
2. **Predictive failure analysis**
3. **Resource optimization recommendations**
4. **Automatic remediation suggestions**
5. **Custom health check plugins**
6. **Multi-cluster correlation**

## Appendix: Issue Detection Reference

| Issue Type | Status | Impact Score | Detection Method |
|------------|--------|--------------|------------------|
| CrashLoopBackOff | Error | 0.65 | Container state waiting reason |
| ImagePullBackOff | Error | 0.55 | Container state waiting reason |
| OOMKilled | Error | 0.70 | Termination state reason + exit code 137 |
| High Restart Count | Warning/Error | 0.35-0.55 | RestartCount >5 recent |
| Readiness Probe Fail | Warning | 0.35 | Event + container not ready |
| Liveness Probe Fail | Error | 0.50 | Event + container restart |
| FailedScheduling | Error | 0.60 | Event reason + pod pending >5min |
| Evicted | Warning | 0.35 | Event reason + pod failed |
| Preempted | Warning | 0.25 | Event reason |
| Node Pressure | Warning | 0.30 | Node condition |
| Node NotReady | Error | 0.70 | Node Ready=False |
| Service No Endpoints | Warning | 0.40 | Endpoints count = 0 |
| DNS Failure | Warning | 0.45 | Event message pattern |
| Namespace Deletion | Error | 0.85 | Bulk DELETE events |
| CRD Deletion | Error | 0.90 | CRD DELETE + cascade |
| Bulk Deletion | Error | 0.80 | >10 deletes in 5min |
| Node Drain | Warning | 0.30 | Node unschedulable + evictions |

## Next Steps

1. Review and approve this implementation plan
2. Create GitHub issues for each phase
3. Prioritize phases based on user needs
4. Begin implementation with Phase 1 (Container State Detection)
5. Iterate and refine based on testing and real-world usage
