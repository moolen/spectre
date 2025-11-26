# Data Model: E2E Test Suite Entities

**Date**: 2025-11-26
**Feature**: 006-e2e-test-suite

## Core Entities

### TestCluster

Represents a Kind cluster instance used during test execution.

**Fields**:
- `Name`: string - Unique identifier for cluster (e.g., "kem-e2e-abc123")
- `KubeConfig`: string - Path to kubeconfig file
- `Context`: string - Kubernetes context name (Kind sets this automatically)
- `Status`: enum {Creating, Ready, Deleting, Failed}
- `CreatedAt`: time.Time - Cluster creation timestamp
- `Container`: string - Docker container ID (Kind-specific)

**Relationships**:
- Has-many: Namespaces (default, test-ns-1, test-ns-2, etc.)
- Has-one: KEM Deployment
- Has-many: Test Deployments (nginx instances)

**Lifecycle**:
1. **Creating**: Kind cluster creation in progress (typically 30-60s)
2. **Ready**: Cluster ready for tests
3. **Deleting**: Cleanup phase (typically 10-20s)
4. **Failed**: Error state, retry needed

**Validation Rules**:
- Name must be unique during test run
- KubeConfig file must be readable
- Context must exist in kubeconfig
- Status transitions must follow: Creating → Ready → Deleting (or → Failed)

**State Transitions**:
```
Creating ──ok──> Ready ──ok──> Deleted
   │              │
   └──error──> Failed
```

---

### Namespace

Kubernetes namespace for resource isolation.

**Fields**:
- `Name`: string - Namespace identifier (e.g., "test-ns-1", "kube-system")
- `CreatedAt`: time.Time
- `Status`: enum {Active, Terminating}

**Standard Namespaces**:
- `default` - KEM deployment target (implicit)
- `kube-system` - Kubernetes system components
- `test-default` - Primary test scenario namespace
- `test-alternate` - For cross-namespace filtering tests

**Validation Rules**:
- Name must be DNS-1123 compliant (lowercase, hyphens only)
- Name must not exceed 253 characters

---

### AuditEvent

Represents a Kubernetes API audit event captured by KEM.

**Fields**:
- `ID`: string - Unique event identifier (UUID)
- `Timestamp`: time.Time - Event creation time (Unix nanoseconds in storage, converted for API)
- `Verb`: enum {create, update, patch, delete, get, list} - API action performed
- `User`: string - User performing action
- `Message`: string - Human-readable description
- `Details`: string - Additional context (optional)
- `ResourceRef`: ResourceReference - What was acted upon
  - `Kind`: string (Deployment, Pod, ConfigMap, etc.)
  - `APIVersion`: string (v1, apps/v1, etc.)
  - `UID`: string - Resource UUID
  - `Name`: string - Resource name
  - `Namespace`: string - Resource namespace

**Time Range Semantics**:
- Events are stored with nanosecond precision internally
- API returns Unix seconds (divide by 1e9)
- Tests convert to time.Time by multiplying by 1000 (to milliseconds for Go)

**Sorting**:
- Primary: Timestamp (ascending)
- Secondary: Verb (for deterministic ordering of simultaneous events)

**Filtering Behavior**:
- **By namespace**: Only events where ResourceRef.Namespace matches
- **By kind**: Only events where ResourceRef.Kind matches
- **By time range**: Timestamp >= StartTime AND Timestamp <= EndTime
- **By verb**: Only events with matching Verb

---

### K8sResource

Aggregated view of a Kubernetes resource with its timeline.

**Fields**:
- `ID`: string - Resource UID
- `Name`: string - Resource name
- `Kind`: string - Resource kind
- `APIVersion`: string - API version group
- `Namespace`: string - Namespace
- `CreatedAt`: time.Time - Resource creation timestamp (first event)
- `StatusSegments`: []StatusSegment - Timeline of status changes
- `Events`: []AuditEvent - All audit events for this resource

**Relationships**:
- Created-by: AuditEvent (initial create event)
- Composed-of: StatusSegment (aggregated from events)
- Composed-of: AuditEvent (filtered subset)

---

### StatusSegment

Represents a period of consistent status for a resource.

**Fields**:
- `ID`: string - Unique segment identifier
- `ResourceID`: string - Reference to parent resource
- `Status`: enum {Ready, Warning, Error, Terminating, Unknown}
- `StartTime`: time.Time - Segment start (Unix seconds from API)
- `EndTime`: time.Time - Segment end (Unix seconds from API)
- `Message`: string - Status description
- `Config`: map[string]interface{} - Configuration snapshot

**Status Values**:
- **Ready**: Resource operational, ready for traffic
- **Warning**: Degraded but functional state
- **Error**: Non-functional state
- **Terminating**: Deletion in progress
- **Unknown**: State cannot be determined

**Derivation Rules**:
- Segments are computed from event sequence
- Event CREATE → Ready segment starts
- Event UPDATE → status may change
- Event DELETE → Terminating segment starts

**Time Range Query**:
```
Query: TimeRange{Start: T1, End: T2}
Return: Segments where (segment.StartTime <= T2 AND segment.EndTime >= T1)
```

---

### APIQuery

Encapsulates a query to the KEM API.

**Fields**:
- `Endpoint`: string - API path (/v1/search, /v1/metadata, etc.)
- `Namespace`: string - Optional namespace filter
- `Kind`: string - Optional resource kind filter
- `StartTime`: time.Time - Optional time range start
- `EndTime`: time.Time - Optional time range end
- `Limit`: int - Optional result limit
- `RetryConfig`: RetryConfig - Retry behavior

**RetryConfig**:
- `MaxAttempts`: int (default: 10)
- `Interval`: time.Duration (default: 1 second)
- `Timeout`: time.Duration (default: 10 seconds total)

**Validation Rules**:
- Endpoint must be valid API path
- StartTime must be before EndTime if both specified
- Limit must be positive if specified
- RetryConfig fields must be positive

---

### TestFixture

Test data and configuration files.

**Deployments** (YAML fixtures):
- `nginx-deployment.yaml` - Standard nginx Deployment for testing
- `test-statefulset.yaml` - StatefulSet for dynamic config testing

**Watch Configurations**:
- `watch-config-default.yaml` - Default (Deployments, Pods)
- `watch-config-extended.yaml` - Extended (adds StatefulSet)

**Helm Values**:
- `helm-values-default.yaml` - Standard KEM values
- `helm-values-test.yaml` - Test-optimized values (reduced retention)

---

## State Machine: Test Lifecycle

```
Start
  ↓
CreateCluster (Creating → Ready)
  ↓
DeployKEM (Ready)
  ↓
WaitForReady (Ready → API accessible)
  ↓
RunScenario[1,2,3]
  ├─ CreateResources
  ├─ QueryAPI (with Eventually retries)
  ├─ VerifyAssertions
  └─ CleanupResources
  ↓
DeleteCluster (Deleting → gone)
  ↓
End
```

---

## Validation & Error Handling

**Cluster Validation**:
- Kubeconfig must be readable after Kind cluster creation
- Context must appear in kubeconfig
- Cluster connectivity must be verified before proceeding

**Event Validation**:
- Timestamps must be present and non-zero
- Verb must be one of {create, update, patch, delete, get, list}
- ResourceRef fields must be non-empty

**Query Validation**:
- Time ranges must be well-formed
- Filters must match known values
- RetryConfig values must be positive

**Assertion Failures**:
- Should include actual vs. expected values
- Should include query context (filters, time range)
- Should suggest debugging steps (check logs, verify resources)

---

## Testing Considerations

**Idempotency**:
- Each test creates unique namespace to avoid conflicts
- Cleanup must be thorough (all resources deleted)
- Re-running tests should not accumulate state

**Timing**:
- Account for ~2-5 second event propagation
- Account for ~30 second config reload
- Use Eventually with appropriate timeout windows

**Isolation**:
- Each test scenario gets its own cluster
- No shared state between scenarios
- Resource cleanup mandatory even on failure (use defer)

**Observability**:
- Log all cluster operations (creation, deployment, deletion)
- Log all API queries and responses
- Capture events on test failure for debugging
