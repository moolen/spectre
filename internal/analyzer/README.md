# Analyzer Package

The `analyzer` package provides Kubernetes resource state analysis including status inference, error message extraction, and container issue detection.

## Overview

This package contains pure analysis functions that inspect Kubernetes resource JSON payloads and infer:
- **Status**: Ready, Warning, Error, Terminating, or Unknown
- **Error Messages**: Specific diagnostic information about what's wrong
- **Container Issues**: Container-level problems in Pods (CrashLoopBackOff, OOMKilled, etc.)

These functions are **stateless** and have no dependencies on storage, databases, or external services. They operate purely on JSON data.

## Package Contents

### Status Inference (`status.go`)

Determines the overall health status of Kubernetes resources.

**Main Function:**
```go
func InferStatusFromResource(kind string, data json.RawMessage, eventType string) string
```

**Supported Resource Types:**
- **Pod**: Analyzes container states, phase, and conditions
- **Deployment**: Checks replica counts and availability conditions
- **StatefulSet**: Monitors replica readiness
- **DaemonSet**: Verifies pod distribution across nodes
- **ReplicaSet**: Checks replica availability
- **Node**: Examines Ready condition and pressure states
- **Job**: Monitors completion and failure states
- **PersistentVolumeClaim**: Checks volume binding status
- **Generic**: Falls back to condition-based inference for unknown types

**Status Values:**
- `Ready`: Resource is healthy and functioning normally
- `Warning`: Resource has non-critical issues
- `Error`: Resource has critical problems
- `Terminating`: Resource is being deleted
- `Unknown`: Unable to determine status

**Example:**
```go
import "github.com/moolen/spectre/internal/analyzer"

podJSON := json.RawMessage(`{"status": {"phase": "Running", ...}}`)
status := analyzer.InferStatusFromResource("Pod", podJSON, "UPDATE")
// Returns: "Ready", "Warning", or "Error"
```

### Error Message Extraction (`errors.go`)

Extracts detailed, human-readable error messages from resource data.

**Main Function:**
```go
func InferErrorMessages(kind string, data json.RawMessage, status string) []string
```

**Returns:**
- Slice of error message strings (can be multiple errors per resource)
- Empty slice if no errors found or status is Ready

**Error Message Examples:**
- Pod: `"CrashLoopBackOff (container: app, restarts: 15)"`
- Deployment: `"Insufficient replicas (1/3 ready); Not available: MinimumReplicasUnavailable"`
- Node: `"NotReady: KubeletNotReady - container runtime network not ready"`
- Job: `"Job failed: BackoffLimitExceeded; 3 failed pods"`

**Resource-Specific Analysis:**

#### Pods
- Container states: CrashLoopBackOff, ImagePullBackOff, OOMKilled
- Restart count issues (high/very high)
- Scheduling failures (Unschedulable, node affinity)
- Pod phase issues (Failed, Unknown, Pending)
- Readiness conditions

#### Deployments
- Replica count mismatches (desired vs ready)
- Unavailable replicas
- Availability condition failures
- Progressing condition issues

#### StatefulSets
- Replica readiness issues
- Current vs desired replica counts

#### DaemonSets
- Pod readiness across nodes
- Unavailable pods
- Misscheduled pods

#### ReplicaSets
- Replica availability problems
- Ready vs desired counts

#### Nodes
- NotReady conditions with reasons
- NetworkUnavailable status
- Pressure conditions (MemoryPressure, DiskPressure, PIDPressure)

#### Jobs
- Failed conditions with reasons
- Failed pod counts
- Completion timeout issues

#### PersistentVolumeClaims
- Pending provisioning with reasons
- Lost volumes

**Example:**
```go
import "github.com/moolen/spectre/internal/analyzer"

deploymentJSON := json.RawMessage(`{"status": {...}}`)
errors := analyzer.InferErrorMessages("Deployment", deploymentJSON, "Error")
// Returns: ["Insufficient replicas (1/3 ready)", "2 unavailable replicas", ...]

errorMessage := strings.Join(errors, "; ")
// "Insufficient replicas (1/3 ready); 2 unavailable replicas"
```

### Container Analysis (`containers.go`)

Analyzes Pod container states to detect specific issues.

**Main Function:**
```go
func InspectContainerStates(obj *resourceData) []ContainerIssue
```

**Helper Function:**
```go
func GetContainerIssuesFromJSON(data json.RawMessage) ([]ContainerIssue, error)
```

**Container Issue Types:**
- `CrashLoopBackOff`: Container repeatedly crashing
- `ImagePullBackOff`: Unable to pull container image
- `ErrImagePull`: Initial image pull failure
- `OOMKilled`: Container killed due to out-of-memory
- `HighRestartCount`: Container has restarted 5-10 times
- `VeryHighRestartCount`: Container has restarted >10 times

**ContainerIssue Structure:**
```go
type ContainerIssue struct {
    ContainerName string
    IssueType     string  // CrashLoopBackOff, ImagePullBackOff, etc.
    RestartCount  int32
    Message       string
    Reason        string
    ExitCode      int32
    ImpactScore   float64 // 0.0 to 1.0
}
```

**Critical Issues Detection:**
```go
func HasCriticalContainerIssues(issues []ContainerIssue) bool
```
Returns true if any container has OOMKilled or CrashLoopBackOff issues.

**Impact Scoring:**
```go
func GetHighestImpactScore(issues []ContainerIssue) float64
```
Returns the highest impact score among all container issues.

**Example:**
```go
import "github.com/moolen/spectre/internal/analyzer"

podJSON := json.RawMessage(`{"status": {"containerStatuses": [...]}}`)
issues, err := analyzer.GetContainerIssuesFromJSON(podJSON)
if err != nil {
    log.Error(err)
}

for _, issue := range issues {
    fmt.Printf("Container %s: %s (restarts: %d)\n", 
        issue.ContainerName, issue.IssueType, issue.RestartCount)
}

if analyzer.HasCriticalContainerIssues(issues) {
    fmt.Println("CRITICAL: Pod has critical container issues!")
}
```

## Usage in Project

### Storage Layer
The `resource_builder` uses status inference when building status segments:
```go
import "github.com/moolen/spectre/internal/analyzer"

segment := models.StatusSegment{
    Status: analyzer.InferStatusFromResource(kind, data, eventType),
    // ...
}
```

### MCP Tools
Tools use error inference and container analysis for diagnostics:

#### cluster_health.go
```go
import "github.com/moolen/spectre/internal/analyzer"

errorMessages := analyzer.InferErrorMessages(
    resource.Kind, 
    lastSegment.ResourceData, 
    lastSegment.Status,
)
errorMessage := strings.Join(errorMessages, "; ")
```

#### resource_changes.go
```go
import "github.com/moolen/spectre/internal/analyzer"

issues, err := analyzer.GetContainerIssuesFromJSON(segment.ResourceData)
if err == nil {
    summary.ContainerIssues = issues
}
```

## Design Principles

1. **Stateless**: All functions are pure - same input always produces same output
2. **No Side Effects**: No file I/O, no network calls, no external dependencies
3. **Multiple Errors**: Returns all applicable errors, not just the first one
4. **Graceful Degradation**: Returns empty results on parse errors, never panics
5. **Specificity**: Provides detailed diagnostic information with context
6. **Reusability**: Can be used by any component needing resource analysis
7. **Extensibility**: Easy to add support for new resource types

## Testing

Comprehensive test coverage in:
- `status_test.go` - Basic status inference tests
- `status_comprehensive_test.go` - Extensive scenarios for all resource types
- `errors_test.go` - Error message extraction tests
- `containers_test.go` - Container issue detection tests

**Run tests:**
```bash
# All analyzer tests
go test ./internal/analyzer -v

# Specific test
go test ./internal/analyzer -run TestInferErrorMessages -v

# With coverage
go test ./internal/analyzer -cover
```

## Extension Guide

### Adding Support for a New Resource Type

1. **Add to status inference** (`status.go`):
```go
func inferResourceSpecificStatus(kind string, obj *resourceData) string {
    switch kind {
    // ... existing cases ...
    case "customresource":
        return inferCustomResourceStatus(obj)
    }
}

func inferCustomResourceStatus(obj *resourceData) string {
    // Implement status logic
    if obj.condition("Ready").isTrue() {
        return resourceStatusReady
    }
    return resourceStatusError
}
```

2. **Add to error inference** (`errors.go`):
```go
func inferResourceSpecificErrors(kind string, obj *resourceData, status string) []string {
    switch kind {
    // ... existing cases ...
    case "customresource":
        return inferCustomResourceErrors(obj)
    }
}

func inferCustomResourceErrors(obj *resourceData) []string {
    errors := make([]string, 0)
    // Extract specific error details
    if cond := obj.condition("Healthy"); cond != nil && cond.isFalse() {
        errors = append(errors, fmt.Sprintf("Not healthy: %s", cond.Reason))
    }
    return errors
}
```

3. **Add tests**:
```go
func TestInferErrorMessages_CustomResource(t *testing.T) {
    resourceJSON := `{"status": {...}}`
    errors := InferErrorMessages("CustomResource", json.RawMessage(resourceJSON), "Error")
    
    if len(errors) == 0 {
        t.Fatal("Expected errors, got none")
    }
    // Assert specific error messages
}
```

## Performance Considerations

- **JSON Parsing**: Each function call parses JSON once
- **Caching**: If calling multiple functions on same resource, consider parsing once and reusing `resourceData`
- **Impact**: Typical analysis takes <1ms per resource
- **Optimization**: For high-volume scenarios, consider pre-computing during segment creation

## Dependencies

**Internal:**
- None - this package has no internal dependencies

**External:**
- `encoding/json` - JSON parsing
- `strings` - String manipulation
- `fmt` - String formatting

**No dependencies on:**
- Storage layer
- Database
- External services
- File system

## Related Documentation

- **Storage Package**: Uses analyzer for building status segments
- **MCP Tools**: Consumers of analyzer functions
- **Models Package**: Defines structures that contain analyzed data

## History

Originally part of `internal/storage`, refactored into separate package in December 2025 to:
- Separate concerns (storage vs analysis)
- Improve reusability
- Clarify package responsibilities
- Enable independent testing

## Maintainers

See main project MAINTAINERS file.
