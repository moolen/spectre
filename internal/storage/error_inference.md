# Error Message Inference

## Overview

The error inference module (`error_inference.go`) extracts detailed, human-readable error messages from Kubernetes resource data. This provides specific diagnostic information beyond simple status classification (Ready/Warning/Error).

## Purpose

Previously, error messages were generic (e.g., "Resource updated"). Now, we extract specific details like:
- `CrashLoopBackOff (container: app, restarts: 15)`
- `ImagePullBackOff (container: nginx): Failed to pull image`
- `NotReady: KubeletNotReady - container runtime network not ready`
- `Insufficient replicas (1/3 ready); Not available: MinimumReplicasUnavailable`

## Main Function

```go
func InferErrorMessages(kind string, data json.RawMessage, status string) []string
```

**Parameters:**
- `kind`: Kubernetes resource kind (Pod, Deployment, Node, etc.)
- `data`: Raw JSON of the resource state
- `status`: Current status (Ready, Warning, Error, etc.)

**Returns:**
- Slice of error message strings describing what is wrong with the resource
- Empty slice if status is Ready or no issues found

## Supported Resource Types

### Pod
- Container states: CrashLoopBackOff, ImagePullBackOff, OOMKilled
- Restart count issues
- Scheduling failures
- Pod phase issues (Failed, Unknown)
- Readiness conditions

### Deployment
- Replica count mismatches
- Unavailable replicas
- Available condition failures
- Progressing condition failures

### StatefulSet
- Replica count issues
- Current vs desired replicas
- Generic condition errors

### DaemonSet
- Pod readiness across nodes
- Unavailable pods
- Misscheduled pods

### ReplicaSet
- Replica count issues
- Availability problems
- Generic condition errors

### Node
- NotReady conditions
- Network unavailability
- Pressure conditions (Memory, Disk, PID)

### Job
- Failed conditions
- Failed pod counts
- Completion issues

### PersistentVolumeClaim
- Pending state with reasons
- Lost volumes

### Other Resources
- Generic condition-based error extraction

## Usage Examples

### In MCP Tools (cluster_health.go)

```go
if lastSegment.Status == statusError || lastSegment.Status == statusWarning {
    errorMessages := storage.InferErrorMessages(
        resource.Kind, 
        lastSegment.ResourceData, 
        lastSegment.Status,
    )
    if len(errorMessages) > 0 {
        errorMessage = strings.Join(errorMessages, "; ")
    } else {
        // Fallback to generic message
        errorMessage = lastSegment.Message
    }
}
```

### Direct Usage

```go
import "github.com/moolen/spectre/internal/storage"

// Extract errors from a Pod
podJSON := json.RawMessage(`{"status": {...}}`)
errors := storage.InferErrorMessages("Pod", podJSON, "Error")
// Returns: ["CrashLoopBackOff (container: app, restarts: 15)"]
```

## Design Principles

1. **Multiple Errors**: Returns all applicable errors, not just the first one
2. **Specificity**: Provides container names, counts, and specific reasons
3. **Graceful Degradation**: Returns empty slice on parse errors, never panics
4. **Reusability**: Resource-specific logic is modular and extensible
5. **Leverages Existing Code**: Uses `InspectContainerStates()` for Pod analysis

## Testing

Comprehensive tests in `error_inference_test.go` cover:
- All resource types
- Multiple error conditions
- Edge cases (empty data, invalid JSON)
- Fallback scenarios

Run tests:
```bash
go test ./internal/storage -run TestInferErrorMessages -v
```

## Extension

To add support for a new resource kind:

1. Add case in `inferResourceSpecificErrors()`
2. Implement `inferXXXErrors(obj *resourceData) []string`
3. Extract relevant fields from resource status/spec
4. Add tests in `error_inference_test.go`

Example:
```go
func inferCustomResourceErrors(obj *resourceData) []string {
    errors := make([]string, 0)
    
    // Check custom conditions
    if cond := obj.condition("Healthy"); cond != nil && cond.isFalse() {
        errors = append(errors, fmt.Sprintf("Not healthy: %s", cond.Reason))
    }
    
    return errors
}
```

## Related Files

- `status_inference.go`: Determines overall status (Ready/Warning/Error)
- `container_states.go`: Analyzes Pod container issues
- `cluster_health.go`: Primary consumer of error messages
- `investigate.go`: Potential future consumer
